package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/reader"
	"github.com/tofunmiadewuyi/dbq/internal/source"
	"github.com/tofunmiadewuyi/dbq/internal/storage"
)

func (e *Engine) Run(ctx context.Context, req Request) (Result, error) {
	started := e.now()
	result := Result{StartedAt: started}
	if err := e.Health(ctx, req).Error(); err != nil {
		result.Duration = e.now().Sub(started).Round(time.Millisecond)
		return result, fmt.Errorf("preflight failed: %w", err)
	}
	key, destination, err := e.runBackup(ctx, req, started)
	result.Duration = e.now().Sub(started).Round(time.Millisecond)
	result.StorageKey = key
	result.Destination = destination
	if err != nil {
		return result, err
	}

	result.PrunedBackups, result.RetentionError = e.Prune(ctx, req)
	return result, nil
}

func (e *Engine) runBackup(ctx context.Context, req Request, started time.Time) (string, string, error) {
	driver, err := e.newDriver(req.Database.Type)
	if err != nil {
		return "", "", fmt.Errorf("failed to retrieve db driver: %w", err)
	}
	fileReader, err := e.newReader(&req.SSH)
	if err != nil {
		return "", "", fmt.Errorf("failed to init file reader: %w", err)
	}
	defer closeReader(fileReader)

	if req.SSH.Required && req.SSH.UseServer && req.StorageType == storage.TypeCloud {
		key, err := e.runServerSideBackup(ctx, req, driver, fileReader)
		return key, "", err
	}

	dumpPath, err := driver.Dump(e.sourceJob(req), fileReader)
	if err != nil {
		return "", "", fmt.Errorf("failed to dump database: %w", err)
	}
	defer e.removeFile(dumpPath) //nolint:errcheck

	artifactPath, extension, contentType := dumpPath, ".dump", "application/octet-stream"
	if req.Database.Type == config.MySQL {
		artifactPath = dumpPath + ".zip"
		extension = ".sql.zip"
		contentType = "application/zip"
		if err := e.zipFile(dumpPath, artifactPath); err != nil {
			return "", "", fmt.Errorf("failed to compress dump: %w", err)
		}
		defer e.removeFile(artifactPath) //nolint:errcheck
	}

	switch req.StorageType {
	case storage.TypeCloud:
		key, err := e.uploadToCloud(ctx, req, started, artifactPath, extension, contentType)
		return key, "", err
	case storage.TypeDirectory:
		destination := filepath.Join(req.Destination, storage.BackupFilename(req.Name, req.Database.Name, started, extension))
		return "", destination, e.copyFile(artifactPath, destination)
	default:
		return "", "", fmt.Errorf("unknown storage type: %s", req.StorageType)
	}
}

func (e *Engine) uploadToCloud(ctx context.Context, req Request, started time.Time, artifactPath, extension, contentType string) (string, error) {
	client, err := e.newStorage(&req.CloudStorage, e.storageCredentials(req))
	if err != nil {
		return "", fmt.Errorf("failed to init storage client: %w", err)
	}
	f, err := e.openFile(artifactPath)
	if err != nil {
		return "", fmt.Errorf("failed to open zip for upload: %w", err)
	}
	defer f.Close()

	key, err := client.UploadBackup(ctx, started, req.Name, req.Database.Name, extension, contentType, f)
	if err != nil {
		return "", err
	}
	return key, nil
}

// runServerSideBackup preserves dbq's existing useserver behavior. It remains
// intentionally separate from the hardened streamed SSH path.
func (e *Engine) runServerSideBackup(ctx context.Context, req Request, driver source.DBDriver, r reader.FileReader) (string, error) {
	timestamp := e.now()
	fileName := storage.BackupFilename(req.Name, req.Database.Name, timestamp, ".dump")
	remotePath := fmt.Sprintf("/var/tmp/%s/%s", config.AppName, fileName)

	if err := driver.DumpRemote(e.sourceJob(req), r, remotePath); err != nil {
		return "", fmt.Errorf("server-side dump failed: %w", err)
	}
	if _, err := r.Exec("which gzip"); err != nil {
		return "", fmt.Errorf("gzip not found on remote host — install gzip")
	}
	if _, err := r.Exec(fmt.Sprintf("gzip '%s'", remotePath)); err != nil {
		return "", fmt.Errorf("failed to compress dump: %w", err)
	}
	gzPath := remotePath + ".gz"
	defer func() { _, _ = r.Exec(fmt.Sprintf("rm -f '%s'", gzPath)) }()

	client, err := e.newStorage(&req.CloudStorage, e.storageCredentials(req))
	if err != nil {
		return "", fmt.Errorf("failed to init storage client: %w", err)
	}
	key := storage.BackupKey(req.Name, req.Database.Name, timestamp, ".dump.gz")
	url, err := client.PresignPutURL(ctx, key, 2*time.Hour)
	if err != nil {
		return "", fmt.Errorf("failed to generate upload URL: %w", err)
	}
	if _, err := r.Exec(fmt.Sprintf("curl -s -f -X PUT -T '%s' '%s'", gzPath, url)); err != nil {
		return "", fmt.Errorf("server upload failed: %w", err)
	}
	return key, nil
}
