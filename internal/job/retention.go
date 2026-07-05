package job

import (
	"context"
	"fmt"

	"github.com/tofunmiadewuyi/dbq/internal/storage"
)

// PruneBackups enforces the job's keep-last-N retention policy against its
// configured storage backend. It is a no-op when Retention is 0 (unlimited).
//
// The newest backup is always retained (see storage.keepLatest), so pruning can
// never remove a job's most recent backup. Returns the number of backups
// deleted.
func PruneBackups(j *Job) (int, error) {
	if j.Retention <= 0 {
		return 0, nil
	}

	switch j.StorageType {
	case storage.TypeCloud:
		client, err := storage.NewStorageClient(&j.Storage)
		if err != nil {
			return 0, fmt.Errorf("failed to init storage client: %w", err)
		}
		deleted, err := storage.PruneCloud(context.Background(), client, j.Name, j.Database.Name, j.Retention)
		return len(deleted), err
	case storage.TypeDirectory:
		deleted, err := storage.PruneDirectory(j.Destination, j.Name, j.Database.Name, j.Retention)
		return len(deleted), err
	default:
		return 0, fmt.Errorf("unknown storage type: %s", j.StorageType)
	}
}
