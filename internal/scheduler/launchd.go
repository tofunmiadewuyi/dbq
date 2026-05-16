package scheduler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/utils"
)

type LaunchdScheduler struct{}

func (l *LaunchdScheduler) IsInstalled(jobId string) bool {
	_, err := os.Stat(plistPath(jobId))
	return err == nil
}

func (l *LaunchdScheduler) Install(j *SchedulerJob) error {
	if _, err := exec.LookPath("launchctl"); err != nil {
		return fmt.Errorf("scheduling requires launchd — not available on this system")
	}

	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine binary path: %w", err)
	}

	interval, err := utils.CronToStartCalendarInterval(j.Frequency)
	if err != nil {
		return fmt.Errorf("could not convert schedule: %w", err)
	}

	dir := plistDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents dir: %w", err)
	}

	path := plistPath(j.ID)
	if err := os.WriteFile(path, []byte(plistContent(j.ID, binaryPath, interval)), 0644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	out, err := exec.Command("launchctl", "load", path).CombinedOutput()
	if err != nil {
		os.Remove(path)
		return fmt.Errorf("launchctl load: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (l *LaunchdScheduler) Uninstall(jobId string) error {
	path := plistPath(jobId)
	exec.Command("launchctl", "unload", path).Run() //nolint:errcheck
	os.Remove(path)
	return nil
}

func plistDir() string {
	if os.Getuid() == 0 {
		return "/Library/LaunchDaemons"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents")
}

func plistLabel(jobId string) string {
	return fmt.Sprintf("com.%s.%s", config.AppName, jobId)
}

func plistPath(jobId string) string {
	return filepath.Join(plistDir(), plistLabel(jobId)+".plist")
}

func plistContent(jobId string, binaryPath string, interval map[string]int) string {
	var intervalEntries strings.Builder
	for _, key := range []string{"Minute", "Hour", "Day", "Month", "Weekday"} {
		if v, ok := interval[key]; ok {
			fmt.Fprintf(&intervalEntries, "\t\t<key>%s</key><integer>%d</integer>\n", key, v)
		}
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>run</string>
		<string>%s</string>
	</array>
	<key>StartCalendarInterval</key>
	<dict>
%s	</dict>
</dict>
</plist>
`, plistLabel(jobId), binaryPath, jobId, intervalEntries.String())
}
