package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/action"
	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/job"
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

