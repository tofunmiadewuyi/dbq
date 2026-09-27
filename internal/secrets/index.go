// Package secrets provides an interface for storing and retrieving job credentials
// from the OS keychain, keeping sensitive values off disk.
package secrets

import (
	"errors"
	"fmt"
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

// Provider loads and stores a job's secret bundle.
type Provider interface {
	Load(jobID string) (JobSecrets, error)
	Save(jobID string, values JobSecrets) error
	Delete(jobID string) error
}

// legacyStore is the key-level interface needed to migrate the removed
// plaintext store. It is intentionally not exposed to job configuration code.
type legacyStore interface {
	Set(jobID, key, value string) error
	Get(jobID, key string) (string, error)
	DeleteKey(jobID, key string) error
	DeleteAll(jobID string) error
}

// New returns a keyring-backed Provider. It never falls back to plaintext
// storage. If a legacy plaintext store exists, its values are copied to the
// keyring, verified, and only then is the legacy file removed.
func New() (Provider, error) {
	m := &keyringManager{}
	if err := m.Set(probeKey, probeKey, "1"); err != nil {
		return nil, fmt.Errorf("OS keyring unavailable: %w", err)
	}
	_ = m.DeleteKey(probeKey, probeKey)

	if err := migrateLegacyFile(legacySecretsPath(), m); err != nil {
		return nil, fmt.Errorf("migrate legacy plaintext secrets: %w", err)
	}
	return m, nil
}
