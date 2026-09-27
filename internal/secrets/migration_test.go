package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type memoryManager struct {
	values   map[string]string
	setErr   error
	getErr   error
	mismatch bool
}

func newMemoryManager() *memoryManager {
	return &memoryManager{values: make(map[string]string)}
}

func (m *memoryManager) Set(jobID, key, value string) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.values[jobID+"/"+key] = value
	return nil
}

func (m *memoryManager) Get(jobID, key string) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}
	value, ok := m.values[jobID+"/"+key]
	if !ok {
		return "", ErrNotFound
	}
	if m.mismatch {
		return value + "-different", nil
	}
	return value, nil
}

func (m *memoryManager) DeleteKey(jobID, key string) error {
	delete(m.values, jobID+"/"+key)
	return nil
}

func (m *memoryManager) DeleteAll(jobID string) error {
	for _, key := range allKeys {
		delete(m.values, jobID+"/"+key)
	}
	return nil
}

func writeLegacyFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".secrets")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMigrateLegacyFileVerifiesThenRemovesSource(t *testing.T) {
	path := writeLegacyFile(t, `{"daily/db_password":"p@ss","daily/storage_sak":"secret"}`)
	dst := newMemoryManager()

	if err := migrateLegacyFile(path, dst); err != nil {
		t.Fatalf("migrateLegacyFile: %v", err)
	}
	if got := dst.values["daily/db_password"]; got != "p@ss" {
		t.Fatalf("database password = %q", got)
	}
	if got := dst.values["daily/storage_sak"]; got != "secret" {
		t.Fatalf("storage secret = %q", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("legacy file still exists or stat failed unexpectedly: %v", err)
	}
}

func TestMigrateLegacyFileKeepsSourceWhenKeyringWriteFails(t *testing.T) {
	path := writeLegacyFile(t, `{"daily/db_password":"p@ss"}`)
	dst := newMemoryManager()
	dst.setErr = errors.New("keyring locked")

	if err := migrateLegacyFile(path, dst); err == nil {
		t.Fatal("expected migration error")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("legacy file should remain: %v", err)
	}
}

func TestMigrateLegacyFileKeepsSourceWhenVerificationFails(t *testing.T) {
	path := writeLegacyFile(t, `{"daily/db_password":"p@ss"}`)
	dst := newMemoryManager()
	dst.mismatch = true

	if err := migrateLegacyFile(path, dst); err == nil {
		t.Fatal("expected migration error")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("legacy file should remain: %v", err)
	}
}

func TestMigrateLegacyFileRejectsMalformedAccount(t *testing.T) {
	path := writeLegacyFile(t, `{"missing-separator":"secret"}`)

	if err := migrateLegacyFile(path, newMemoryManager()); err == nil {
		t.Fatal("expected migration error")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("legacy file should remain: %v", err)
	}
}

func TestMigrateLegacyFileAllowsMissingSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".secrets")
	if err := migrateLegacyFile(path, newMemoryManager()); err != nil {
		t.Fatalf("missing source should be a no-op: %v", err)
	}
}
