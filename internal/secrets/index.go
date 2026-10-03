// Package secrets provides an interface for storing and retrieving job credentials
// independently of the execution engine and persisted job configuration.
package secrets

import (
	"errors"
)

const (
	KeyDBPassword  = "db_password"
	KeyStorageAKID = "storage_akid"
	KeyStorageSAK  = "storage_sak"
)

var ErrNotFound = errors.New("secret not found")

// allKeys is every secret a job can own; DeleteAll walks it.
var allKeys = []string{KeyDBPassword, KeyStorageAKID, KeyStorageSAK}

const probeKey = "__probe__"

// JobSecrets is the complete secret material required by one backup job.
// It is deliberately separate from the persisted job configuration.
type JobSecrets struct {
	DatabasePassword string
	StorageAccessKey string
	StorageSecretKey string
}

// Provider resolves a job's secret bundle without requiring ownership of its storage.
// External orchestrators can supply read-only providers.
type Provider interface {
	Load(jobID string) (JobSecrets, error)
}

// Store is a provider that dbq can manage for standalone jobs.
// Keyring and future encrypted systemd stores implement this interface.
type Store interface {
	Provider
	Save(jobID string, values JobSecrets) error
	Delete(jobID string) error
}
