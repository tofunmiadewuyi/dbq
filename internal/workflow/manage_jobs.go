// Package workflow provides the interactive CLI flows for managing, running, and testing backup jobs.
package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tofunmiadewuyi/dbq/utils"
	"github.com/tofunmiadewuyi/dbq/internal/action"
	"github.com/tofunmiadewuyi/dbq/internal/input"
	"github.com/tofunmiadewuyi/dbq/internal/job"
	"github.com/tofunmiadewuyi/dbq/internal/systemd"
)

// PrintAvailableJobs renders the job list and returns the selected job.
// Returns nil if the user chooses "< Back".
func PrintAvailableJobs(jobs []job.Job) *job.Job {
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
func PrintJobOptions(j *job.Job) string {
	w := 68
	box := utils.NewDisplayBox(w)
	border := box.BoxBorder()

	scheduleLabel := "Schedule"
	if systemd.IsInstalled(j) {
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
func PrintTestOptions(j *job.Job) string {
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

func ManageJobs(jobs []job.Job) error {
	// Job selection loop — "< Back" here returns to the main menu.
jobList:
	for {
		j := PrintAvailableJobs(jobs)
		if j == nil {
			return nil
		}

		// Job options loop — "< Back" here returns to the job list.
		for {
			jobOption := PrintJobOptions(j)

			switch jobOption {
			case "Run":
				fmt.Printf("⌛ Running backup for %s...\n", j.Name)
				if err := action.CreateBackup(j); err != nil {
					fmt.Println("error:", err)
				}

			case "Test":
				// Test sub-menu loop — "< Back" here returns to job options.
				for {
					testOption := PrintTestOptions(j)
					if testOption == "< Back" {
						break
					}
					switch testOption {
					case "Test Dump":
						fmt.Printf("⌛ Testing dump for %s...\n", j.Name)
						if err := action.TestDump(j); err != nil {
							fmt.Println("error:", err)
						}
					case "Test Storage":
						fmt.Printf("⌛ Testing storage for %s...\n", j.Name)
						if err := action.TestStorage(j); err != nil {
							fmt.Println("error:", err)
						}
					}
				}

			case "Schedule":
				if err := systemd.Install(j); err != nil {
					fmt.Println("error:", err)
				} else {
					fmt.Printf("✅ Timer scheduled — job will run on: %s\n", utils.CronToReadable(j.Frequency))
				}

			case "Logs":
				data, err := utils.ReadLogs(j.ID)
				if err != nil {
					if os.IsNotExist(err) {
						fmt.Printf("no logs found for %s\n", j.Name)
					} else {
						fmt.Println("error reading logs:", err)
					}
				} else {
					fmt.Printf("\n— logs for %s —\n\n%s\n", j.Name, data)
				}

			case "Unschedule":
				if err := systemd.Uninstall(j); err != nil {
					fmt.Println("error:", err)
				} else {
					fmt.Printf("✅ Timer removed for %s\n", j.Name)
				}

			case "Delete":
				path := filepath.Join(utils.JobsDir(), j.ID+".toml")
				if err := os.Remove(path); err != nil {
					fmt.Println("error deleting job:", err)
				} else {
					fmt.Printf("✅ Job %q deleted\n", j.Name)
					updated, err := job.GetJobs()
					if err != nil {
						return err
					}
					jobs = updated
					continue jobList
				}

			case "< Back":
				continue jobList

			default:
				fmt.Println("not yet implemented")
			}
		}
	}
}
