// Package storage defines the storage client interface and provides implementations for uploading backups.
package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"
)

// BackupFilename returns just the filename portion of a backup: job_db_20060102_150405.ext
// ext must include the dot, e.g. ".zip" or ".dump.gz".
func BackupFilename(jobName, dbName string, timestamp time.Time, ext string) string {
	safeDBName := filepath.Base(dbName)
	ts := timestamp.Format("20060102_150405")
	return fmt.Sprintf("%s_%s_%s%s", jobName, safeDBName, ts, ext)
}

// BackupKey returns the full S3/R2 object key for a backup.
// ext must include the dot, e.g. ".zip" or ".dump.gz".
func BackupKey(jobName, dbName string, timestamp time.Time, ext string) string {
	safeDBName := filepath.Base(dbName)
	return fmt.Sprintf("backups/%s/%s/%s", jobName, safeDBName, BackupFilename(jobName, dbName, timestamp, ext))
}

// StorageClient is the shared interface every storage backend must satisfy.
type StorageClient interface {
	UploadBackup(ctx context.Context, timestamp time.Time, backupName, dbName, contentType string, r io.Reader) (string, error)
	TestConnection(ctx context.Context) error
	// PresignPutURL returns a short-lived presigned HTTP PUT URL for the given key.
	// the server can use this to upload directly without needing cloud credentials.
	PresignPutURL(ctx context.Context, key string, expiry time.Duration) (string, error)
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
