package job

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/tofunmiadewuyi/dbq/internal/secrets"
)

// legacyJobCredentials exists only to read credentials written by older dbq
// versions. Job itself deliberately has no TOML fields for secret values.
type legacyJobCredentials struct {
	Database struct {
		Password string `toml:"password"`
	} `toml:"database"`
	Storage struct {
		AccessKey string `toml:"access_key"`
		SecretKey string `toml:"secret_key"`
	} `toml:"storage"`
}

func GetJobs(provider secrets.Provider) ([]Job, error) {
	dir := JobsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var jobs []Job
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		var j Job
		if _, err := toml.DecodeFile(path, &j); err != nil {
			return nil, err
		}
		var legacy legacyJobCredentials
		if _, err := toml.DecodeFile(path, &legacy); err != nil {
			return nil, err
		}
		if err := hydrateSecrets(&j, provider, legacy); err != nil {
			return nil, fmt.Errorf("load secrets for job %q: %w", j.ID, err)
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// hydrateSecrets loads the job's separate secret bundle. Credentials left in
// an old TOML file are migrated only when the keyring does not already contain
// that value, then the config is rewritten without any credential fields.
func hydrateSecrets(j *Job, provider secrets.Provider, legacy legacyJobCredentials) error {
	j.provider = provider
	values, err := provider.Load(j.ID)
	if err != nil {
		return err
	}

	hadPlaintext := legacy.Database.Password != "" ||
		legacy.Storage.AccessKey != "" || legacy.Storage.SecretKey != ""
	if values.DatabasePassword == "" {
		values.DatabasePassword = legacy.Database.Password
	}
	if values.StorageAccessKey == "" {
		values.StorageAccessKey = legacy.Storage.AccessKey
	}
	if values.StorageSecretKey == "" {
		values.StorageSecretKey = legacy.Storage.SecretKey
	}
	j.Secrets = values

	if hadPlaintext {
		if err := j.WriteJob(); err != nil {
			return fmt.Errorf("migrate credentials out of job config: %w", err)
		}
	}
	return nil
}
