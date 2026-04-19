package scheduler

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/tofunmiadewuyi/dbq/internal/job"
	"github.com/tofunmiadewuyi/dbq/utils"
)

type SystemdScheduler struct{}

func (s *SystemdScheduler) IsInstalled(j *job.Job) bool {
	_, err := os.Stat(filepath.Join(unitDir(), timerFileName(j)))
	return err == nil
}

func (s *SystemdScheduler) Install(j *job.Job) error {
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

	service := serviceContent(j, binaryPath)
	if err := os.WriteFile(filepath.Join(dir, serviceFileName(j)), []byte(service), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	timer, err := timerContent(j)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, timerFileName(j)), []byte(timer), 0644); err != nil {
		return fmt.Errorf("failed to write timer file: %w", err)
	}

	if err := systemctl(j, "enable", "--now"); err != nil {
		os.Remove(filepath.Join(dir, timerFileName(j)))
		os.Remove(filepath.Join(dir, serviceFileName(j)))
		return err
	}
	enableLinger()
	return nil
}

func (s *SystemdScheduler) Uninstall(j *job.Job) error {
	systemctl(j, "disable", "--now") //nolint:errcheck

	dir := unitDir()
	os.Remove(filepath.Join(dir, timerFileName(j)))
	os.Remove(filepath.Join(dir, serviceFileName(j)))

	return daemonReload()
}

func unitDir() string {
	if os.Getuid() == 0 {
		return "/etc/systemd/system"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user")
}

func serviceFileName(j *job.Job) string {
	return fmt.Sprintf("dbq-%s.service", j.ID)
}

func timerFileName(j *job.Job) string {
	return fmt.Sprintf("dbq-%s.timer", j.ID)
}

func serviceContent(j *job.Job, binaryPath string) string {
	return fmt.Sprintf(`[Unit]
Description=dbq backup — %s

[Service]
Type=oneshot
ExecStart=%s run %s
`, j.Name, binaryPath, j.ID)
}

func timerContent(j *job.Job) (string, error) {
	onCalendar, err := utils.CronToOnCalendar(j.Frequency)
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
`, j.Name, onCalendar), nil
}

func systemctl(j *job.Job, args ...string) error {
	base := []string{"systemctl"}
	if os.Getuid() != 0 {
		base = append(base, "--user")
	}
	base = append(base, args...)
	base = append(base, timerFileName(j))

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
