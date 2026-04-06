package action

import (
	"context"
	"fmt"
	"time"

	"github.com/tofunmiadewuyi/dbq/utils"
	"github.com/tofunmiadewuyi/dbq/internal/job"
	"github.com/tofunmiadewuyi/dbq/internal/reader"
	"github.com/tofunmiadewuyi/dbq/internal/source"
	"github.com/tofunmiadewuyi/dbq/internal/storage"
)

func TestDump(j *job.Job) error {
	start := time.Now()
	err := runTestDump(j)
	d := time.Since(start).Round(time.Millisecond)
	utils.AppendLog(j.ID, "test dump", d, err)
	if err != nil {
		return err
	}
	fmt.Printf("✅ Dump test passed in %s\n", d)
	return nil
}

func runTestDump(j *job.Job) error {
	driver, err := source.NewDBDriver(&j.Database)
	if err != nil {
		return fmt.Errorf("db driver error: %w", err)
	}

	fileReader, err := reader.GetFileReader(&j.Database.SSH)
	if err != nil {
		return fmt.Errorf("file reader error: %w", err)
	}
	defer fileReader.Close()

	return driver.Test(j, fileReader)
}

func TestStorage(j *job.Job) error {
	start := time.Now()
	err := runTestStorage(j)
	d := time.Since(start).Round(time.Millisecond)
	utils.AppendLog(j.ID, "test storage", d, err)
	if err != nil {
		return err
	}
	fmt.Printf("✅ Storage test passed in %s\n", d)
	return nil
}

func runTestStorage(j *job.Job) error {
	client, err := storage.NewStorageClient(&j.Storage)
	if err != nil {
		return fmt.Errorf("failed to init storage client: %w", err)
	}
	return client.TestConnection(context.Background())
}
