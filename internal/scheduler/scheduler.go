// Package scheduler provides a platform-agnostic interface for scheduling backup jobs.
package scheduler

import (
	"runtime"
)

type SchedulerJob struct {
	Name      string
	ID        string
	Frequency string
}

// Scheduler manages the lifecycle of a scheduled backup job on the host OS.
type Scheduler interface {
	Install(j *SchedulerJob) error
	Uninstall(jobId string) error
	IsInstalled(jobId string) bool
}

// New returns the appropriate Scheduler implementation for the current OS.
func New() Scheduler {
	switch runtime.GOOS {
	case "darwin":
		return &LaunchdScheduler{}
	default:
		return &SystemdScheduler{}
	}
}

var defaultScheduler Scheduler

func init() { defaultScheduler = New() }

func Install(j *SchedulerJob) error { return defaultScheduler.Install(j) }
func Uninstall(jobId string) error  { return defaultScheduler.Uninstall(jobId) }
func IsInstalled(jobId string) bool { return defaultScheduler.IsInstalled(jobId) }
