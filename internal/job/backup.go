package job

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/reader"
	"github.com/tofunmiadewuyi/dbq/internal/source"
	"github.com/tofunmiadewuyi/dbq/internal/storage"
	"github.com/tofunmiadewuyi/dbq/utils"
)

func CreateBackup(j *Job) error {
	start := time.Now()

	err := runBackup(j)

	d := time.Since(start).Round(time.Millisecond)
	AppendLog(j.ID, "backup", d, err)

	if err != nil {
		return err
	}

	// Prune old backups only after a successful run, so a failed backup never
	// triggers deletion of existing copies. A cleanup failure is non-fatal —
	// the backup itself succeeded — so it's logged rather than returned.
	if deleted, pruneErr := PruneBackups(j); pruneErr != nil {
		fmt.Printf("⚠️  Backup ok, but retention cleanup failed: %v\n", pruneErr)
		AppendLog(j.ID, "prune", 0, pruneErr)
	} else if deleted > 0 {
		fmt.Printf("🧹 Pruned %d old backup(s), keeping latest %d\n", deleted, j.Retention)
	}

	fmt.Printf("✅ Backup completed in %s\n", d)
	return nil
}

func runBackup(j *Job) error {
	driver, err := source.NewDBDriver(j.Database.Type)
	if err != nil {
		return fmt.Errorf("failed to retrieve db driver: %w", err)
	}

	fileReader, err := reader.GetFileReader(j.ReaderSSH())
	if err != nil {
		return fmt.Errorf("failed to init file reader: %w", err)
	}
	defer fileReader.Close()

	// server-side path: dump on the remote host and upload directly to cloud
	if j.Database.SSH.Required && j.Database.SSH.UseServer && j.StorageType == storage.TypeCloud {
		return runServerSideBackup(j, driver, fileReader)
	}

	dumpPath, err := driver.Dump(j.SourceJob(), fileReader)
	if err != nil {
		return fmt.Errorf("failed to dump database: %w", err)
	}
	defer os.Remove(dumpPath)

	zipPath := dumpPath + ".zip"
	if err := utils.ZipFile(dumpPath, zipPath); err != nil {
		return fmt.Errorf("failed to compress dump: %w", err)
	}
	defer os.Remove(zipPath)

	switch j.StorageType {
	case storage.TypeCloud:
		return uploadToCloud(j, zipPath)
	case storage.TypeDirectory:
		dest := filepath.Join(j.Destination, filepath.Base(zipPath))
		return utils.CopyFile(zipPath, dest)
	default:
		return fmt.Errorf("unknown storage type: %s", j.StorageType)
	}
}

// runServerSideBackup dumps the database to a temp file on the remote server,
// then uploads it directly from the server to cloud storage using a presigned URL.
// this avoids routing the dump through the home internet connection
// and no credentials leave your machine.
func runServerSideBackup(j *Job, driver source.DBDriver, r reader.FileReader) error {
	timestamp := time.Now()
	fileName := storage.BackupFilename(j.Name, j.Database.Name, timestamp, ".dump")
	remotePath := fmt.Sprintf("/var/tmp/%s/%s", config.AppName, fileName)

	if err := driver.DumpRemote(j.SourceJob(), r, remotePath); err != nil {
		return fmt.Errorf("server-side dump failed: %w", err)
	}

	if _, err := r.Exec("which gzip"); err != nil {
		return fmt.Errorf("gzip not found on remote host — install gzip")
	}
	if _, err := r.Exec(fmt.Sprintf("gzip '%s'", remotePath)); err != nil {
		return fmt.Errorf("failed to compress dump: %w", err)
	}
	gzPath := remotePath + ".gz"
	defer func() { r.Exec(fmt.Sprintf("rm -f '%s'", gzPath)) }() //nolint: errcheck

	client, err := storage.NewStorageClient(&j.Storage)
	if err != nil {
		return fmt.Errorf("failed to init storage client: %w", err)
	}

	key := storage.BackupKey(j.Name, j.Database.Name, timestamp, ".dump.gz")
	url, err := client.PresignPutURL(context.Background(), key, 2*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to generate upload URL: %w", err)
	}

	curlCmd := fmt.Sprintf("curl -s -f -X PUT -T '%s' '%s'", gzPath, url)
	if _, err := r.Exec(curlCmd); err != nil {
		return fmt.Errorf("server upload failed: %w", err)
	}

	fmt.Printf("✅ Backup uploaded → %s\n", key)
	return nil
}

func uploadToCloud(j *Job, zipPath string) error {
	client, err := storage.NewStorageClient(&j.Storage)
	if err != nil {
		return fmt.Errorf("failed to init storage client: %w", err)
	}

	f, err := os.Open(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip for upload: %w", err)
	}
	defer f.Close()

	key, err := client.UploadBackup(
		context.Background(),
		time.Now(),
		j.Name,
		j.Database.Name,
		"application/zip",
		f,
	)
	if err != nil {
		return err
	}

	fmt.Printf("✅ Backup uploaded → %s\n", key)
	return nil
}

