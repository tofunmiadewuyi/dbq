// Package secrets provides an interface for storing and retrieving job credentials
// from the OS keychain, keeping sensitive values off disk.
package secrets

import "errors"

const (
	KeyDBPassword  = "db_password"
	KeyStorageAKID = "storage_akid"
	KeyStorageSAK  = "storage_sak"
)

var ErrNotFound = errors.New("secret not found")

// allKeys is every secret a job can own; DeleteAll walks it.
var allKeys = []string{KeyDBPassword, KeyStorageAKID, KeyStorageSAK}

const probeKey = "__probe__"

// Manager stores and retrieves credentials for a job.
type Manager interface {
	Set(jobID, key, value string) error
	Get(jobID, key string) (string, error)
	Delete(jobID, key string) error
	DeleteAll(jobID string) error
}

// New returns a keyring-backed Manager if the OS keychain is available,
// otherwise falls back to a file-based store at ~/.config/dbq/.secrets (0600).
func New() Manager {
	m := &keyringManager{}
	if err := m.Set(probeKey, probeKey, "1"); err != nil {
		return newFileManager()
	}
	_ = m.Delete(probeKey, probeKey)
	return m
}
