package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/action"
	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/job"
	"github.com/tofunmiadewuyi/dbq/utils"
)

func cleanStaleTempFiles() {
	tmpDir := filepath.Join(config.TmpPath, config.AppName)
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if time.Since(info.ModTime()) > 24*time.Hour {
			os.Remove(path)
		}
		return nil
	})
}

func printLogs(id string, lines int) {
	path := filepath.Join(utils.LogsDir(), id+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "no logs found for job %q\n", id)
		} else {
			fmt.Fprintf(os.Stderr, "could not read logs: %v\n", err)
		}
		os.Exit(1)
	}
	fmt.Print(utils.TailLines(string(data), lines))
}

func deleteJob(id string) {
	path := filepath.Join(utils.JobsDir(), id+".toml")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "no job found with id %q\n", id)
		} else {
			fmt.Fprintf(os.Stderr, "could not delete job: %v\n", err)
		}
		os.Exit(1)
	}
	fmt.Printf("job %q deleted\n", id)
}

func printConfig(id string) {
	path := filepath.Join(utils.JobsDir(), id+".toml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "no config found for job %q\n", id)
		} else {
			fmt.Fprintf(os.Stderr, "could not read config: %v\n", err)
		}
		os.Exit(1)
	}
	fmt.Println(path)
	fmt.Println()
	fmt.Print(string(data))
}

// runJob is the non-interactive path called by the systemd service:
//
//	dbq run <job-id>
func runJob(id string) {
	jobs, err := job.GetJobs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load jobs: %v\n", err)
		os.Exit(1)
	}
	for _, j := range jobs {
		if j.ID == id {
			if err := action.CreateBackup(&j); err != nil {
				fmt.Fprintf(os.Stderr, "backup failed: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}
	fmt.Fprintf(os.Stderr, "job %q not found\n", id)
	os.Exit(1)
}

