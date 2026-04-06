// Package systemd generates and installs systemd service and timer units for backup jobs.
package systemd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tofunmiadewuyi/dbq/internal/job"
	"github.com/tofunmiadewuyi/dbq/utils"
)

// unitDir returns the systemd unit directory for the current user.
// system-wide if root, user-level otherwise.
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

// serviceContent generates the .service unit file content.
// Type=oneshot means systemd considers the job done once the process exits
func serviceContent(j *job.Job, binaryPath string) string {
	return fmt.Sprintf(`[Unit]
Description=dbq backup — %s

[Service]
Type=oneshot
ExecStart=%s run %s
`, j.Name, binaryPath, j.ID)
}

// timerContent generates the .timer unit file content.
// Persistent=true means if the system was off when the timer should have fired,
// it will run immediately on next boot rather than waiting for the next interval.
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

// IsInstalled reports whether the timer unit file exists on disk for this job.
func IsInstalled(j *job.Job) bool {
	_, err := os.Stat(filepath.Join(unitDir(), timerFileName(j)))
	return err == nil
}

// Install writes the service and timer unit files and enables the timer.
func Install(j *job.Job) error {
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

	return systemctl(j, "enable", "--now")
}

// Uninstall stops and removes the timer and service for a job.
func Uninstall(j *job.Job) error {
	systemctl(j, "disable", "--now") // best effort — ignore error if not installed

	dir := unitDir()
	os.Remove(filepath.Join(dir, timerFileName(j)))
	os.Remove(filepath.Join(dir, serviceFileName(j)))

	return daemonReload()
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
