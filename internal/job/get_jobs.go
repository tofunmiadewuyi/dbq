package job

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/tofunmiadewuyi/dbq/internal/secrets"
)

func GetJobs(sm secrets.Manager) ([]Job, error) {
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
		var j Job
		if _, err := toml.DecodeFile(filepath.Join(dir, entry.Name()), &j); err != nil {
			return nil, err
		}
		hydrateSecrets(&j, sm)
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// hydrateSecrets sets the secrets manager on the job and populates sensitive fields
// from the keychain. Falls back to any plaintext value in the TOML and migrates it.
func hydrateSecrets(j *Job, sm secrets.Manager) {
	j.sm = sm

	hadPlaintext := j.Database.Password != "" || j.Storage.AKID != "" || j.Storage.SAK != ""

	if pw, err := sm.Get(j.ID, secrets.KeyDBPassword); err == nil {
		j.Database.Password = pw
	} else if j.Database.Password != "" {
		if err := sm.Set(j.ID, secrets.KeyDBPassword, j.Database.Password); err != nil {
			fmt.Printf("warning: could not store db password in keychain: %v\n", err)
		}
	}

	if akid, err := sm.Get(j.ID, secrets.KeyStorageAKID); err == nil {
		j.Storage.AKID = akid
	} else if j.Storage.AKID != "" {
		if err := sm.Set(j.ID, secrets.KeyStorageAKID, j.Storage.AKID); err != nil {
			fmt.Printf("warning: could not store storage akid in keychain: %v\n", err)
		}
	}

	if sak, err := sm.Get(j.ID, secrets.KeyStorageSAK); err == nil {
		j.Storage.SAK = sak
	} else if j.Storage.SAK != "" {
		if err := sm.Set(j.ID, secrets.KeyStorageSAK, j.Storage.SAK); err != nil {
			fmt.Printf("warning: could not store storage sak in keychain: %v\n", err)
		}
	}

	if hadPlaintext {
		if err := j.WriteJob(); err != nil {
			fmt.Printf("warning: could not rewrite job config for %q: %v\n", j.Name, err)
		}
	}
}
