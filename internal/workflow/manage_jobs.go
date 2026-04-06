// Package workflow provides the interactive CLI flows for managing, running, and testing backup jobs.
package workflow

import (
	"fmt"

	"github.com/tofunmiadewuyi/dbq/utils"
	"github.com/tofunmiadewuyi/dbq/internal/action"
	"github.com/tofunmiadewuyi/dbq/internal/input"
	"github.com/tofunmiadewuyi/dbq/internal/job"
)

// PrintAvailableJobs renders the job list and returns the selected job.
// Returns nil if the user chooses "< Back".
func PrintAvailableJobs(jobs []job.Job) *job.Job {
	w := 68
	box := utils.NewDisplayBox(w)
	border := box.BoxBorder()
	center := box.BoxCenter
	row := box.CreateRow

	fmt.Printf("\n┌%s┐\n", border)
	fmt.Printf("│%s│\n", center("AVAILABLE JOB(s)"))
	fmt.Printf("├%s┤\n", border)

	for i, j := range jobs {
		row(fmt.Sprintf("%d)  ", i+1), j.Name)
	}

	row("0)  ", "< back")
	fmt.Printf("└%s┘\n\n", border)

	selection := input.AskValidInt("Select: ", func(n string) error {
		return input.ValidateInt("A selection", n)
	}, "")

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
	center := box.BoxCenter
	row := box.CreateRow

	options := []string{"Run", "Test", "Edit", "Delete"}

	fmt.Printf("\n┌%s┐\n", border)
	fmt.Printf("│%s│\n", center(j.Name))
	fmt.Printf("├%s┤\n", border)

	for i, opt := range options {
		row(fmt.Sprintf("%d)  ", i+1), opt)
	}

	row("0)  ", "< back")
	fmt.Printf("└%s┘\n\n", border)

	selection := input.AskValidInt("Select: ", func(n string) error {
		return input.ValidateInt("A selection", n)
	}, "")

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
	center := box.BoxCenter
	row := box.CreateRow

	options := []string{"Test Dump", "Test Storage"}

	fmt.Printf("\n┌%s┐\n", border)
	fmt.Printf("│%s│\n", center("TEST — "+j.Name))
	fmt.Printf("├%s┤\n", border)

	for i, opt := range options {
		row(fmt.Sprintf("%d)  ", i+1), opt)
	}

	row("0)  ", "< back")
	fmt.Printf("└%s┘\n\n", border)

	selection := input.AskValidInt("Select: ", func(n string) error {
		return input.ValidateInt("A selection", n)
	}, "")

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

			case "< Back":
				continue jobList

			default:
				fmt.Println("not yet implemented")
			}
		}
	}
}
