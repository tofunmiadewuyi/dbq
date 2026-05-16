package secrets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// fileManager is used on systems where the keyring/Secret Service is unavailable (e.g. headless Linux servers).
// Secrets are stored in a JSON file at ~/.config/dbq/.secrets with 0600 permissions.
type fileManager struct {
	path string
	mu   sync.Mutex
}

func newFileManager() *fileManager {
	var dir string
	if os.Getuid() == 0 {
		dir = "/etc/dbq"
	} else {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config", "dbq")
	}
	return &fileManager{path: filepath.Join(dir, ".secrets")}
}

func (m *fileManager) load() (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if os.IsNotExist(err) {
		return make(map[string]string), nil
	}
	if err != nil {
		return nil, err
	}
	var store map[string]string
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	return store, nil
}

func (m *fileManager) save(store map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(store)
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0600)
}

func (m *fileManager) Set(jobID, key, value string) error {
	store, err := m.load()
	if err != nil {
		return err
	}
	store[jobID+"/"+key] = value
	return m.save(store)
}

func (m *fileManager) Get(jobID, key string) (string, error) {
	store, err := m.load()
	if err != nil {
		return "", err
	}
	v, ok := store[jobID+"/"+key]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (m *fileManager) Delete(jobID string) error {
	store, err := m.load()
	if err != nil {
		return err
	}
	for _, k := range []string{KeyDBPassword, KeyStorageAKID, KeyStorageSAK} {
		delete(store, jobID+"/"+k)
	}
	return m.save(store)
}
