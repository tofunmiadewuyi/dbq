// Package storage defines the storage client interface and provides implementations for uploading backups.
package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/job"
)

// StorageClient is the shared interface every storage backend must satisfy.
type StorageClient interface {
	UploadBackup(ctx context.Context, timestamp time.Time, backupName, dbName, contentType string, r io.Reader) (string, error)
	TestConnection(ctx context.Context) error
}

// NewStorageClient returns the correct StorageClient for the job's configured provider.
func NewStorageClient(cfg *job.CloudStorage) (StorageClient, error) {
	switch cfg.Provider {
	case config.S3:
		return NewS3Client(cfg)
	case config.R2:
		return NewR2Client(cfg)
	default:
		return nil, fmt.Errorf("unsupported storage provider: %s", cfg.Provider)
	}
}
