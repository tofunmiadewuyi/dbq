package secrets

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const service = "dbq"

type keyringManager struct{}

func (m *keyringManager) Set(jobID, key, value string) error {
	return keyring.Set(service, jobID+"/"+key, value)
}

func (m *keyringManager) Get(jobID, key string) (string, error) {
	return keyring.Get(service, jobID+"/"+key)
}

// Delete removes all known secrets for the given job.
func (m *keyringManager) Delete(jobID string) error {
	keys := []string{KeyDBPassword, KeyStorageAKID, KeyStorageSAK}
	for _, k := range keys {
		if err := keyring.Delete(service, jobID+"/"+k); err != nil && err != keyring.ErrNotFound {
			return fmt.Errorf("failed to delete %s: %w", k, err)
		}
	}
	return nil
}
