package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// keepLatest returns the subset of keys that should be DELETED to satisfy a
// "keep the N most recent" retention policy. Keys are expected to embed a
// lexicographically sortable timestamp (see BackupFilename), so newest sorts
// last.
//
// It never returns every key: whenever keep >= 1 the newest `keep` keys are
// retained, so the most recent backup can never be deleted. A keep of 0 (or
// negative) means "unlimited" and deletes nothing.
func keepLatest(keys []string, keep int) []string {
	if keep <= 0 {
		return nil
	}
	if len(keys) <= keep {
		return nil
	}

	sorted := make([]string, len(keys))
	copy(sorted, keys)
	sort.Sort(sort.Reverse(sort.StringSlice(sorted))) // newest first

	return sorted[keep:] // tail = oldest = delete
}

// PruneCloud enforces a keep-last-N policy on a cloud backend, deleting the
// oldest backups for the given job/db beyond the newest `keep`. It returns the
// keys it deleted. A keep of 0 is a no-op.
func PruneCloud(ctx context.Context, client StorageClient, jobName, dbName string, keep int) ([]string, error) {
	if keep <= 0 {
		return nil, nil
	}

	objects, err := client.ListBackups(ctx, jobName, dbName)
	if err != nil {
		return nil, err
	}

	keys := make([]string, len(objects))
	for i, obj := range objects {
		keys[i] = obj.Key
	}

	toDelete := keepLatest(keys, keep)
	deleted := make([]string, 0, len(toDelete))
	for _, key := range toDelete {
		if err := client.DeleteBackup(ctx, key); err != nil {
			return deleted, fmt.Errorf("failed to delete %s: %w", key, err)
		}
		deleted = append(deleted, key)
	}
	return deleted, nil
}

// PruneDirectory enforces a keep-last-N policy on a local directory, deleting
// the oldest backup files for the given job/db beyond the newest `keep`. Only
// files matching this job/db's filename prefix are considered, so backups from
// other jobs sharing the directory are never touched. It returns the paths it
// deleted. A keep of 0 is a no-op.
func PruneDirectory(dir, jobName, dbName string, keep int) ([]string, error) {
	if keep <= 0 {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	prefix := BackupFilePrefix(jobName, dbName)
	var names []string
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > len(prefix) && e.Name()[:len(prefix)] == prefix {
			names = append(names, e.Name())
		}
	}

	toDelete := keepLatest(names, keep)
	deleted := make([]string, 0, len(toDelete))
	for _, name := range toDelete {
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err != nil {
			return deleted, fmt.Errorf("failed to delete %s: %w", path, err)
		}
		deleted = append(deleted, path)
	}
	return deleted, nil
}
