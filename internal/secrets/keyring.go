package secrets

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const service = "dbq"

type keyringManager struct{}

func (m *keyringManager) Set(jobID, key, value string) error {
	return keyring.Set(service, jobID+"/"+key, value)
}

func (m *keyringManager) Get(jobID, key string) (string, error) {
	v, err := keyring.Get(service, jobID+"/"+key)
	if err == keyring.ErrNotFound {
		return "", ErrNotFound
	}
	return v, err
}

func (m *keyringManager) Delete(jobID, key string) error {
	if err := keyring.Delete(service, jobID+"/"+key); err != nil && err != keyring.ErrNotFound {
		return err
	}
	return nil
}

// DeleteAll removes every known secret for the job, attempting each key even if an earlier one fails.
func (m *keyringManager) DeleteAll(jobID string) error {
	var errs []error
	for _, k := range allKeys {
		if err := m.Delete(jobID, k); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete %s: %w", k, err))
		}
	}
	return errors.Join(errs...)
}
