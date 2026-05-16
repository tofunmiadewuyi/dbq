package job

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/input"
	"github.com/tofunmiadewuyi/dbq/internal/scheduler"
	"github.com/tofunmiadewuyi/dbq/utils"
)

func DefaultDBPort(db config.DatabaseType) (string, error) {
	switch db {
	case config.Postgres:
		return "5432", nil
	case config.MySQL:
		return "3306", nil
	default:
		return "", fmt.Errorf("unsupported database type: %v", db)
	}
}

func JobsDir() string {
	if os.Getuid() == 0 {
		return filepath.Join("/etc", config.AppName, "jobs")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", config.AppName, "jobs")
}

func CheckRootAccess() bool {
	return os.Getuid() == 0
}




// PrintAvailableJobs renders the job list and returns the selected job.
// Returns nil if the user chooses "< Back".
func PrintAvailableJobs(jobs []Job) *Job {
	w := 68
	box := utils.NewDisplayBox(w)
	border := box.BoxBorder()

	var b strings.Builder
	fmt.Fprintf(&b, "\n┌%s┐\n", border)
	fmt.Fprintf(&b, "│%s│\n", box.BoxCenter("AVAILABLE JOB(s)"))
	fmt.Fprintf(&b, "├%s┤\n", border)
	for i, j := range jobs {
		b.WriteString(box.RowStr(fmt.Sprintf("%d)  ", i+1), j.Name))
	}
	b.WriteString(box.RowStr("0)  ", "< back"))
	fmt.Fprintf(&b, "└%s┘\n\n", border)

	content := b.String()
	fmt.Print(content)

	selection := input.AskValidInt("Select: ", func(n string) error {
		return input.ValidateInt("A selection", n)
	}, "")

	utils.DimPrevious(content, selection)

	if selection == 0 {
		return nil
	}
	return &jobs[selection-1]
}

// PrintJobOptions renders the options for a selected job and returns the chosen option.
func PrintJobOptions(j *Job) string {
	w := 68
	box := utils.NewDisplayBox(w)
	border := box.BoxBorder()

	scheduleLabel := "Schedule"
	if scheduler.IsInstalled(j.ID) {
		scheduleLabel = "Unschedule"
	}
	options := []string{"Run", "Test", scheduleLabel, "Logs", "Edit", "Delete"}

	var b strings.Builder
	fmt.Fprintf(&b, "\n┌%s┐\n", border)
	fmt.Fprintf(&b, "│%s│\n", box.BoxCenter(j.Name))
	fmt.Fprintf(&b, "├%s┤\n", border)
	for i, opt := range options {
		b.WriteString(box.RowStr(fmt.Sprintf("%d)  ", i+1), opt))
	}
	b.WriteString(box.RowStr("0)  ", "< back"))
	fmt.Fprintf(&b, "└%s┘\n\n", border)

	content := b.String()
	fmt.Print(content)

	selection := input.AskValidInt("Select: ", func(n string) error {
		return input.ValidateInt("A selection", n)
	}, "")

	utils.DimPrevious(content, selection)

	if selection == 0 {
		return "< Back"
	}
	return options[selection-1]
}

// PrintTestOptions renders the test sub-menu and returns the chosen option.
func PrintTestOptions(j *Job) string {
	w := 68
	box := utils.NewDisplayBox(w)
	border := box.BoxBorder()

	options := []string{"Test Dump", "Test Storage"}

	var b strings.Builder
	fmt.Fprintf(&b, "\n┌%s┐\n", border)
	fmt.Fprintf(&b, "│%s│\n", box.BoxCenter("TEST — "+j.Name))
	fmt.Fprintf(&b, "├%s┤\n", border)
	for i, opt := range options {
		b.WriteString(box.RowStr(fmt.Sprintf("%d)  ", i+1), opt))
	}
	b.WriteString(box.RowStr("0)  ", "< back"))
	fmt.Fprintf(&b, "└%s┘\n\n", border)

	content := b.String()
	fmt.Print(content)

	selection := input.AskValidInt("Select: ", func(n string) error {
		return input.ValidateInt("A selection", n)
	}, "")

	utils.DimPrevious(content, selection)

	if selection == 0 {
		return "< Back"
	}
	return options[selection-1]
}

