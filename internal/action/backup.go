// Package action implements the backup and test operations that run against a job.
package action

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/tofunmiadewuyi/dbq/utils"
	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/job"
	"github.com/tofunmiadewuyi/dbq/internal/reader"
	"github.com/tofunmiadewuyi/dbq/internal/source"
	"github.com/tofunmiadewuyi/dbq/internal/storage"
)

func CreateBackup(j *job.Job) error {
	start := time.Now()

	err := runBackup(j)

	d := time.Since(start).Round(time.Millisecond)
	utils.AppendLog(j.ID, "backup", d, err)

	if err != nil {
		return err
	}

	fmt.Printf("✅ Backup completed in %s\n", d)
	return nil
}

func runBackup(j *job.Job) error {
	driver, err := source.NewDBDriver(&j.Database)
	if err != nil {
		return fmt.Errorf("failed to retrieve db driver: %w", err)
	}

	fileReader, err := reader.GetFileReader(&j.Database.SSH)
	if err != nil {
		return fmt.Errorf("failed to init file reader: %w", err)
	}
	defer fileReader.Close()

	dumpPath, err := driver.Dump(j, fileReader)
	if err != nil {
		return fmt.Errorf("failed to dump database: %w", err)
	}
	defer os.Remove(dumpPath)

	zipPath := dumpPath + ".zip"
	if err := ZipFile(dumpPath, zipPath); err != nil {
		return fmt.Errorf("failed to compress dump: %w", err)
	}
	defer os.Remove(zipPath)

	switch j.StorageType {
	case config.StorageCloud:
		return uploadToCloud(j, zipPath)
	case config.StorageDirectory:
		dest := filepath.Join(j.Destination, filepath.Base(zipPath))
		return copyFile(zipPath, dest)
	default:
		return fmt.Errorf("unknown storage type: %s", j.StorageType)
	}
}

func uploadToCloud(j *job.Job, zipPath string) error {
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

// ZipFile compresses a single file at srcPath into a zip archive at zipPath.
func ZipFile(srcPath, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.Base(srcPath)
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, src)
	return err
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
