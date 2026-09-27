// Package engine executes backup operations from fully resolved runtime input.
// It does not load job files, access the keyring, prompt users, or write logs.
package engine

import (
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/reader"
	"github.com/tofunmiadewuyi/dbq/internal/secrets"
	"github.com/tofunmiadewuyi/dbq/internal/storage"
)

type Database struct {
	Type     config.DatabaseType
	Name     string
	Host     string
	Port     string
	Username string
}

// Request is a complete in-memory description of one backup operation.
// Secrets are resolved by the caller and are never persisted by the engine.
type Request struct {
	ID           string
	Name         string
	Database     Database
	SSH          reader.SSHConn
	StorageType  storage.StorageType
	Destination  string
	CloudStorage storage.CloudStorage
	Retention    int
	Secrets      secrets.JobSecrets
}

type Result struct {
	StartedAt      time.Time
	Duration       time.Duration
	StorageKey     string
	Destination    string
	PrunedBackups  int
	RetentionError error
}

type TestResult struct {
	Duration time.Duration
}
