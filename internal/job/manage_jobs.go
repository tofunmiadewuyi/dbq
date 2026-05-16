package job

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tofunmiadewuyi/dbq/internal/scheduler"
	"github.com/tofunmiadewuyi/dbq/internal/secrets"
	"github.com/tofunmiadewuyi/dbq/utils"
)


func ManageJobs(sm secrets.Manager, jobs []Job) error {
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
				if err := CreateBackup(j); err != nil {
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
						if err := TestDump(j); err != nil {
							fmt.Println("error:", err)
						}
					case "Test Storage":
						fmt.Printf("⌛ Testing storage for %s...\n", j.Name)
						if err := TestStorage(j); err != nil {
							fmt.Println("error:", err)
						}
					}
				}

			case "Logs":
				data, err := ReadLogs(j.ID)
				if err != nil {
					if os.IsNotExist(err) {
						fmt.Printf("no logs found for %s\n", j.Name)
					} else {
						fmt.Println("error reading logs:", err)
					}
				} else {
					fmt.Printf("\n— logs for %s —\n\n%s\n", j.Name, data)
				}

			case "Schedule":
				if err := scheduler.Install(&scheduler.SchedulerJob{
					Name: j.Name,
					ID: j.ID,
					Frequency: j.Frequency,
				}); err != nil {
					fmt.Println("error:", err)
				} else {
					fmt.Printf("✅ Timer scheduled — job will run on: %s\n", utils.CronToReadable(j.Frequency))
				}

			case "Unschedule":
				if err := scheduler.Uninstall(j.ID); err != nil {
					fmt.Println("error:", err)
				} else {
					fmt.Printf("✅ Timer removed for %s\n", j.Name)
				}

			case "Delete":
				path := filepath.Join(JobsDir(), j.ID+".toml")
				if err := os.Remove(path); err != nil {
					fmt.Println("error deleting job:", err)
				} else {
					if err := j.DeleteSecrets(); err != nil {
						fmt.Printf("warning: could not remove secrets from keychain: %v\n", err)
					}
					fmt.Printf("✅ Job %q deleted\n", j.Name)
					updated, err := GetJobs(sm)
					if err != nil {
						return err
					}
					jobs = updated
					continue jobList
				}

			case "Edit":
				if err := EditJob(j); err != nil {
					fmt.Println("error:", err)
				}

			case "< Back":
				continue jobList

			default:
				fmt.Println("not yet implemented")
			}
		}
	}
}
