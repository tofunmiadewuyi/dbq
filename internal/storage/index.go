// Package storage defines the storage client interface and provides implementations for uploading backups.
package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

// backupTimeLayout is the fixed-width timestamp dbq embeds in every backup
// filename. It is 15 characters wide, which ParseBackupTime relies on.
const backupTimeLayout = "20060102_150405"

// BackupFilename returns just the filename portion of a backup: job_db_20060102_150405.ext
// ext must include the dot, e.g. ".zip" or ".dump.gz".
func BackupFilename(jobName, dbName string, timestamp time.Time, ext string) string {
	safeDBName := filepath.Base(dbName)
	ts := timestamp.Format(backupTimeLayout)
	return fmt.Sprintf("%s_%s_%s%s", jobName, safeDBName, ts, ext)
}

// ParseBackupTime extracts the dump time dbq embedded in a backup's name. name
// may be a bare filename, a full object key, or an absolute path. It returns
// ok=false when the name doesn't belong to this job/db or doesn't carry a valid
// timestamp — callers treat those as un-prunable and leave them untouched,
// rather than guessing at their age.
func ParseBackupTime(jobName, dbName, name string) (t time.Time, ok bool) {
	base := filepath.Base(name)
	prefix := BackupFilePrefix(jobName, dbName)
	if !strings.HasPrefix(base, prefix) {
		return time.Time{}, false
	}
	rest := base[len(prefix):]
	if len(rest) < len(backupTimeLayout) {
		return time.Time{}, false
	}
	t, err := time.Parse(backupTimeLayout, rest[:len(backupTimeLayout)])
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// BackupFilePrefix returns the leading "{jobName}_{dbName}_" portion common to
// every backup filename for a job/db. Used to match a job's own files when
// pruning a shared local directory, so sibling jobs' backups are never touched.
func BackupFilePrefix(jobName, dbName string) string {
	return fmt.Sprintf("%s_%s_", jobName, filepath.Base(dbName))
}

// BackupPrefix returns the shared key prefix under which all of a job/db's
// backups live: backups/{jobName}/{dbName}/. It is extension-agnostic, so it
// matches both ".zip" (client-side) and ".dump.gz" (server-side) uploads.
func BackupPrefix(jobName, dbName string) string {
	safeDBName := filepath.Base(dbName)
	return fmt.Sprintf("backups/%s/%s/", jobName, safeDBName)
}

// BackupKey returns the full S3/R2 object key for a backup.
// ext must include the dot, e.g. ".zip" or ".dump.gz".
func BackupKey(jobName, dbName string, timestamp time.Time, ext string) string {
	return BackupPrefix(jobName, dbName) + BackupFilename(jobName, dbName, timestamp, ext)
}

// BackupObject identifies a single stored backup. Key is the full object key
// (or absolute path for directory storage). Timestamp is the dump time parsed
// out of the filename by ParseBackupTime — dbq stamps this at dump time, so it's
// the value we sort on for retention.
type BackupObject struct {
	Key       string
	Timestamp time.Time
}

// StorageClient is the shared interface every storage backend must satisfy.
type StorageClient interface {
	UploadBackup(ctx context.Context, timestamp time.Time, backupName, dbName, contentType string, r io.Reader) (string, error)
	TestConnection(ctx context.Context) error
	// PresignPutURL returns a short-lived presigned HTTP PUT URL for the given key.
	// the server can use this to upload directly without needing cloud credentials.
	PresignPutURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	// ListBackups returns every backup object stored for the given job/db.
	ListBackups(ctx context.Context, jobName, dbName string) ([]BackupObject, error)
	// DeleteBackup removes a single backup object by its full key.
	DeleteBackup(ctx context.Context, key string) error
}

// NewStorageClient returns the correct StorageClient for the job's configured provider.
func NewStorageClient(cfg *CloudStorage) (StorageClient, error) {
	switch cfg.Provider {
	case TypeS3:
		return NewS3Client(cfg)
	case TypeR2:
		return NewR2Client(cfg)
	default:
		return nil, fmt.Errorf("unsupported storage provider: %s", cfg.Provider)
	}
}
