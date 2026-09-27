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

func (m *keyringManager) DeleteKey(jobID, key string) error {
	if err := keyring.Delete(service, jobID+"/"+key); err != nil && err != keyring.ErrNotFound {
		return err
	}
	return nil
}

// DeleteAll removes every known secret for the job, attempting each key even if an earlier one fails.
func (m *keyringManager) DeleteAll(jobID string) error {
	var errs []error
	for _, k := range allKeys {
		if err := m.DeleteKey(jobID, k); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete %s: %w", k, err))
		}
	}
	return errors.Join(errs...)
}

func (m *keyringManager) Load(jobID string) (JobSecrets, error) {
	var values JobSecrets
	fields := []struct {
		key string
		dst *string
	}{
		{KeyDBPassword, &values.DatabasePassword},
		{KeyStorageAKID, &values.StorageAccessKey},
		{KeyStorageSAK, &values.StorageSecretKey},
	}
	for _, field := range fields {
		value, err := m.Get(jobID, field.key)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return JobSecrets{}, fmt.Errorf("load %s: %w", field.key, err)
		}
		*field.dst = value
	}
	return values, nil
}

func (m *keyringManager) Save(jobID string, values JobSecrets) error {
	fields := []struct {
		key   string
		value string
	}{
		{KeyDBPassword, values.DatabasePassword},
		{KeyStorageAKID, values.StorageAccessKey},
		{KeyStorageSAK, values.StorageSecretKey},
	}
	var errs []error
	for _, field := range fields {
		var err error
		if field.value == "" {
			err = m.DeleteKey(jobID, field.key)
		} else {
			err = m.Set(jobID, field.key, field.value)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("save %s: %w", field.key, err))
		}
	}
	return errors.Join(errs...)
}

func (m *keyringManager) Delete(jobID string) error {
	return m.DeleteAll(jobID)
}
