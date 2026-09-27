package secrets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// legacySecretsPath is the location used by dbq's removed plaintext fallback.
// It remains here only so existing installations can be migrated once.
func legacySecretsPath() string {
	var dir string
	if os.Getuid() == 0 {
		dir = "/etc/dbq"
	} else {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config", "dbq")
	}
	return filepath.Join(dir, ".secrets")
}

// migrateLegacyFile copies a legacy plaintext store into dst, verifies every
// value can be read back, and removes the file only after full verification.
// A partial failure deliberately leaves the source file intact so a later run
// can retry without losing credentials.
func migrateLegacyFile(path string, dst legacyStore) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var store map[string]string
	if err := json.Unmarshal(data, &store); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	for account, value := range store {
		jobID, key, ok := strings.Cut(account, "/")
		if !ok || jobID == "" || key == "" {
			return fmt.Errorf("invalid legacy secret account %q", account)
		}
		if err := dst.Set(jobID, key, value); err != nil {
			return fmt.Errorf("store %q in keyring: %w", account, err)
		}
	}

	for account, want := range store {
		jobID, key, _ := strings.Cut(account, "/")
		got, err := dst.Get(jobID, key)
		if err != nil {
			return fmt.Errorf("verify %q in keyring: %w", account, err)
		}
		if got != want {
			return fmt.Errorf("verify %q in keyring: value mismatch", account)
		}
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove migrated plaintext file %s: %w", path, err)
	}
	return nil
}
