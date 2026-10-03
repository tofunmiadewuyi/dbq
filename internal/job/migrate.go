package job

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/tofunmiadewuyi/dbq/internal/secrets"
)

type plaintextSource struct {
	path     string
	original []byte
	clean    []byte
	remove   bool
}

// MigrateSecrets imports legacy plaintext into the selected standalone store.
// All inputs are validated and all destination values verified before cleanup.
// Normal job loading and the external agent never call this operation.
func MigrateSecrets(dst secrets.Store) error {
	if os.Getenv("CREDENTIALS_DIRECTORY") != "" {
		return fmt.Errorf("run secret migration outside a service credential context")
	}
	return migrateSecrets(dst, secrets.LegacySecretsPath(), JobsDir())
}

func migrateSecrets(dst secrets.Store, legacyPath, jobsDir string) error {
	bundles := make(map[string]secrets.JobSecrets)
	var sources []plaintextSource
	data, err := readMigrationSource(legacyPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		bundles, err = secrets.DecodeLegacy(data)
		if err != nil {
			return err
		}
		sources = append(sources, plaintextSource{path: legacyPath, original: data, remove: true})
	}
	entries, err := os.ReadDir(jobsDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		path := filepath.Join(jobsDir, entry.Name())
		data, err := readMigrationSource(path)
		if err != nil {
			return err
		}
		var document map[string]any
		// Parse errors may quote source text, which can contain credentials.
		if _, err := toml.Decode(string(data), &document); err != nil {
			return fmt.Errorf("invalid job TOML in %q", path)
		}
		id, ok := document["id"].(string)
		if !ok {
			return fmt.Errorf("missing job ID in %q", path)
		}
		if err := secrets.ValidateJobID(id); err != nil {
			return err
		}
		if entry.Name() != id+".toml" {
			return fmt.Errorf("job ID does not match filename %q", path)
		}
		var values secrets.JobSecrets
		found := false
		for _, field := range []struct {
			table, key string
			dst        *string
		}{
			{"database", "password", &values.DatabasePassword},
			{"storage", "access_key", &values.StorageAccessKey},
			{"storage", "secret_key", &values.StorageSecretKey},
		} {
			table, ok := document[field.table].(map[string]any)
			if !ok {
				continue
			}
			value, exists := table[field.key]
			if !exists {
				continue
			}
			text, ok := value.(string)
			if !ok {
				return fmt.Errorf("invalid credential type in job %q (%s.%s)", id, field.table, field.key)
			}
			*field.dst = text
			delete(table, field.key)
			found = true
		}
		if !found {
			continue
		}
		merged, err := secrets.Merge(bundles[id], values)
		if err != nil {
			return fmt.Errorf("legacy sources disagree for job %q: %w", id, err)
		}
		bundles[id] = merged
		var clean bytes.Buffer
		if err := toml.NewEncoder(&clean).Encode(document); err != nil {
			return fmt.Errorf("encode cleaned job %q", id)
		}
		sources = append(sources, plaintextSource{path: path, original: data, clean: clean.Bytes()})
	}
	if err := secrets.Import(dst, bundles); err != nil {
		return err
	}
	// Check every source before touching any, so edits made while importing
	// cause a retry instead of deletion of unimported credentials.
	for _, source := range sources {
		current, err := readMigrationSource(source.path)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, source.original) {
			return fmt.Errorf("migration source %q changed; retry migration", source.path)
		}
	}
	// Clean job configs first. The standalone legacy file remains retryable if
	// an earlier config rewrite fails; all values already exist in the store.
	for _, source := range sources {
		if source.remove {
			continue
		}
		if err := replaceMigratedConfig(source.path, source.clean); err != nil {
			return err
		}
	}
	for _, source := range sources {
		if source.remove {
			if err := os.Remove(source.path); err != nil {
				return fmt.Errorf("remove migrated plaintext store: %w", err)
			}
		}
	}
	return nil
}

func replaceMigratedConfig(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".migration-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// Symlinks would leave plaintext at their targets after unlink or replacement.
func readMigrationSource(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("migration source %q must be a regular file", path)
	}
	return os.ReadFile(path)
}
