package scheduler

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/utils"
)

type SystemdScheduler struct{}

func (s *SystemdScheduler) IsInstalled(jobId string) bool {
	_, err := os.Stat(filepath.Join(unitDir(), timerFileName(jobId)))
	return err == nil
}

func (s *SystemdScheduler) Install(j *SchedulerJob) error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("scheduling requires systemd — not available on this system")
	}

	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine binary path: %w", err)
	}

	dir := unitDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd unit dir: %w", err)
	}

	service := serviceContent(j.Name, j.ID, binaryPath)
	if err := os.WriteFile(filepath.Join(dir, serviceFileName(j.ID)), []byte(service), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	timer, err := timerContent(j.Name, j.Frequency)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, timerFileName(j.ID)), []byte(timer), 0644); err != nil {
		return fmt.Errorf("failed to write timer file: %w", err)
	}

	if err := systemctl(j.ID, "enable", "--now"); err != nil {
		os.Remove(filepath.Join(dir, timerFileName(j.ID)))
		os.Remove(filepath.Join(dir, serviceFileName(j.ID)))
		return err
	}
	enableLinger()
	return nil
}

func (s *SystemdScheduler) Uninstall(jobId string) error {
	systemctl(jobId, "disable", "--now") //nolint:errcheck

	dir := unitDir()
	os.Remove(filepath.Join(dir, timerFileName(jobId)))
	os.Remove(filepath.Join(dir, serviceFileName(jobId)))

	return daemonReload()
}

func unitDir() string {
	if os.Getuid() == 0 {
		return "/etc/systemd/system"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user")
}

func serviceFileName(id string) string {
	return fmt.Sprintf("%s-%s.service", config.AppName, id)
}

func timerFileName(id string) string {
	return fmt.Sprintf("%s-%s.timer", config.AppName, id)
}

func serviceContent(jobName, jobId string, binaryPath string) string {
	return fmt.Sprintf(`[Unit]
Description=dbq backup — %s

[Service]
Type=oneshot
ExecStart=%s run %s
`, jobName, binaryPath, jobId)
}

func timerContent(jobName, jobFrequency string) (string, error) {
	onCalendar, err := utils.CronToOnCalendar(jobFrequency)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`[Unit]
Description=dbq backup timer — %s

[Timer]
OnCalendar=%s
Persistent=true

[Install]
WantedBy=timers.target
`, jobName, onCalendar), nil
}

func systemctl(jobId string, args ...string) error {
	base := []string{"systemctl"}
	if os.Getuid() != 0 {
		base = append(base, "--user")
	}
	base = append(base, args...)
	base = append(base, timerFileName(jobId))

	out, err := exec.Command(base[0], base[1:]...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return daemonReload()
}

func daemonReload() error {
	args := []string{"systemctl"}
	if os.Getuid() != 0 {
		args = append(args, "--user")
	}
	args = append(args, "daemon-reload")

	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("daemon-reload: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func enableLinger() {
	if os.Getuid() == 0 {
		return
	}
	u, err := user.Current()
	if err != nil {
		return
	}
	exec.Command("loginctl", "enable-linger", u.Username).Run() //nolint:errcheck
}
