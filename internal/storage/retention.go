package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// keepLatest returns the subset of objects that should be DELETED to satisfy a
// "keep the N most recent" retention policy. Recency is decided by the parsed
// Timestamp (dbq's dump time), with the Key as a stable tiebreaker for backups
// sharing a timestamp.
//
// It never returns every object: whenever keep >= 1 the newest `keep` are
// retained, so the most recent backup can never be deleted. A keep of 0 (or
// negative) means "unlimited" and deletes nothing.
func keepLatest(objects []BackupObject, keep int) []BackupObject {
	if keep <= 0 {
		return nil
	}
	if len(objects) <= keep {
		return nil
	}

	sorted := make([]BackupObject, len(objects))
	copy(sorted, objects)
	sort.Slice(sorted, func(i, j int) bool {
		if !sorted[i].Timestamp.Equal(sorted[j].Timestamp) {
			return sorted[i].Timestamp.After(sorted[j].Timestamp) // newest first
		}
		return sorted[i].Key > sorted[j].Key
	})

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

	toDelete := keepLatest(objects, keep)
	deleted := make([]string, 0, len(toDelete))
	for _, obj := range toDelete {
		if err := client.DeleteBackup(ctx, obj.Key); err != nil {
			return deleted, fmt.Errorf("failed to delete %s: %w", obj.Key, err)
		}
		deleted = append(deleted, obj.Key)
	}
	return deleted, nil
}

// PruneDirectory enforces a keep-last-N policy on a local directory, deleting
// the oldest backup files for the given job/db beyond the newest `keep`. Only
// files carrying this job/db's valid backup name are considered, so backups
// from other jobs — and any unrelated or tampered files — are never touched. It
// returns the paths it deleted. A keep of 0 is a no-op.
func PruneDirectory(dir, jobName, dbName string, keep int) ([]string, error) {
	if keep <= 0 {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var objects []BackupObject
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ts, ok := ParseBackupTime(jobName, dbName, e.Name())
		if !ok {
			continue // not a recognizable dbq backup — leave it untouched
		}
		objects = append(objects, BackupObject{Key: filepath.Join(dir, e.Name()), Timestamp: ts})
	}

	toDelete := keepLatest(objects, keep)
	deleted := make([]string, 0, len(toDelete))
	for _, obj := range toDelete {
		if err := os.Remove(obj.Key); err != nil {
			return deleted, fmt.Errorf("failed to delete %s: %w", obj.Key, err)
		}
		deleted = append(deleted, obj.Key)
	}
	return deleted, nil
}
