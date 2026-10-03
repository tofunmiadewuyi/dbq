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

func GetJobs(provider secrets.Store) ([]Job, error) {
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
		j, err := loadJob(path, provider)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// GetJob resolves only the requested job, so scheduled services need no access
// to any other job's credentials.
func GetJob(provider secrets.Store, id string) (*Job, error) {
	if err := secrets.ValidateJobID(id); err != nil {
		return nil, err
	}
	j, err := loadJob(filepath.Join(JobsDir(), id+".toml"), provider)
	if err != nil {
		return nil, err
	}
	if j.ID != id {
		return nil, fmt.Errorf("job ID does not match config filename")
	}
	return &j, nil
}

func loadJob(path string, provider secrets.Store) (Job, error) {
	var j Job
	if _, err := toml.DecodeFile(path, &j); err != nil {
		return Job{}, err
	}
	if err := secrets.ValidateJobID(j.ID); err != nil {
		return Job{}, err
	}
	var legacy legacyJobCredentials
	if _, err := toml.DecodeFile(path, &legacy); err != nil {
		return Job{}, err
	}
	if err := hydrateSecrets(&j, provider, legacy); err != nil {
		return Job{}, fmt.Errorf("load secrets for job %q: %w", j.ID, err)
	}
	return j, nil
}

// hydrateSecrets resolves credentials without modifying configuration or
// silently choosing between conflicting legacy and stored values.
func hydrateSecrets(j *Job, provider secrets.Store, legacy legacyJobCredentials) error {
	if legacy.Database.Password != "" || legacy.Storage.AccessKey != "" || legacy.Storage.SecretKey != "" {
		return fmt.Errorf("plaintext credentials present; run dbq secrets migrate")
	}
	j.provider = provider
	values, err := provider.Load(j.ID)
	if err != nil {
		return err
	}
	j.Secrets = values
	return nil
}
