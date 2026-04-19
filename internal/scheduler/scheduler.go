// Package scheduler provides a platform-agnostic interface for scheduling backup jobs.
package scheduler

import (
	"runtime"

	"github.com/tofunmiadewuyi/dbq/internal/job"
)

// Scheduler manages the lifecycle of a scheduled backup job on the host OS.
type Scheduler interface {
	Install(j *job.Job) error
	Uninstall(j *job.Job) error
	IsInstalled(j *job.Job) bool
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

var def Scheduler

func init() { def = New() }

func Install(j *job.Job) error    { return def.Install(j) }
func Uninstall(j *job.Job) error  { return def.Uninstall(j) }
func IsInstalled(j *job.Job) bool { return def.IsInstalled(j) }
