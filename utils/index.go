// Package utils provides shared display and formatting helpers used across the application.
package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tofunmiadewuyi/dbq/internal/config"
)

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

