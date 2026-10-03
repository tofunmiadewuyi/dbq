package secrets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LegacySecretsPath identifies the removed plaintext fallback store.
func LegacySecretsPath() string { return filepath.Join(configDir(), ".secrets") }

// RequireMigration prevents standalone commands from ignoring a legacy store.
// The explicit migration command opens its destination without this check.
func RequireMigration() error {
	_, err := os.Lstat(LegacySecretsPath())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("legacy plaintext secrets present; run dbq secrets migrate")
}

// DecodeLegacy reads the historical jobID/key JSON object. Unknown fields and
// duplicate accounts are rejected so cleanup cannot silently discard material.
func DecodeLegacy(data []byte) (map[string]JobSecrets, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("invalid legacy secrets JSON: expected an object")
	}
	bundles := make(map[string]JobSecrets)
	seen := make(map[string]bool)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("invalid legacy secrets JSON")
		}
		account, ok := token.(string)
		if !ok || seen[account] {
			return nil, fmt.Errorf("invalid or duplicate legacy secret account")
		}
		seen[account] = true
		id, key, ok := strings.Cut(account, "/")
		if !ok {
			return nil, fmt.Errorf("invalid legacy secret account %q", account)
		}
		if err := ValidateJobID(id); err != nil {
			return nil, err
		}
		var raw any
		if err := decoder.Decode(&raw); err != nil {
			return nil, fmt.Errorf("invalid legacy secret value for %q", account)
		}
		value, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("invalid legacy secret value for %q", account)
		}
		bundle := bundles[id]
		switch key {
		case KeyDBPassword:
			bundle.DatabasePassword = value
		case KeyStorageAKID:
			bundle.StorageAccessKey = value
		case KeyStorageSAK:
			bundle.StorageSecretKey = value
		default:
			return nil, fmt.Errorf("unknown legacy secret key %q", key)
		}
		bundles[id] = bundle
	}
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("invalid legacy secrets JSON")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("invalid legacy secrets JSON: trailing data")
	}
	return bundles, nil
}

// Merge preserves destination-only fields and rejects differing nonempty values.
// Neither credential value is included in a conflict error.
func Merge(destination, source JobSecrets) (JobSecrets, error) {
	fields := []struct {
		key string
		dst *string
		src string
	}{
		{KeyDBPassword, &destination.DatabasePassword, source.DatabasePassword},
		{KeyStorageAKID, &destination.StorageAccessKey, source.StorageAccessKey},
		{KeyStorageSAK, &destination.StorageSecretKey, source.StorageSecretKey},
	}
	for _, field := range fields {
		if field.src == "" {
			continue
		}
		if *field.dst != "" && *field.dst != field.src {
			return JobSecrets{}, fmt.Errorf("conflicting values for %s", field.key)
		}
		*field.dst = field.src
	}
	return destination, nil
}

// Import preflights every bundle before writing, then verifies all complete
// destination bundles. Callers retain plaintext sources until this succeeds.
// Partial writes are retryable: matching destination values are accepted.
func Import(dst Store, sources map[string]JobSecrets) error {
	ids := make([]string, 0, len(sources))
	for id := range sources {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	merged := make(map[string]JobSecrets, len(ids))
	changed := make(map[string]bool, len(ids))
	for _, id := range ids {
		if err := ValidateJobID(id); err != nil {
			return err
		}
		current, err := dst.Load(id)
		if err != nil {
			return fmt.Errorf("read destination for job %q: %w", id, err)
		}
		values, err := Merge(current, sources[id])
		if err != nil {
			return fmt.Errorf("job %q: %w", id, err)
		}
		merged[id] = values
		changed[id] = values != current
	}
	for _, id := range ids {
		if !changed[id] {
			continue
		}
		if err := dst.Save(id, merged[id]); err != nil {
			return fmt.Errorf("store credentials for job %q: %w", id, err)
		}
	}
	for _, id := range ids {
		got, err := dst.Load(id)
		if err != nil {
			return fmt.Errorf("verify credentials for job %q: %w", id, err)
		}
		if got != merged[id] {
			return fmt.Errorf("verify credentials for job %q: value mismatch", id)
		}
	}
	return nil
}
