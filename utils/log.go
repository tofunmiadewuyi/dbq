package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/config"
)

func LogsDir() string {
	if os.Getuid() == 0 {
		return filepath.Join("/etc", config.AppName, "logs")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", config.AppName, "logs")
}

// AppendLog writes a single run entry to the job's log file.
func AppendLog(jobID, operation string, d time.Duration, err error) {
	dir := LogsDir()
	if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
		return
	}

	path := filepath.Join(dir, jobID+".log")
	f, mkErr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if mkErr != nil {
		return
	}
	defer f.Close()

	ts := time.Now().Format("2006-01-02 15:04:05")
	if err != nil {
		fmt.Fprintf(f, "%s  %-14s  failed   %s\n", ts, operation, err.Error())
	} else {
		fmt.Fprintf(f, "%s  %-14s  ok       %s\n", ts, operation, d.Round(time.Millisecond))
	}
}
