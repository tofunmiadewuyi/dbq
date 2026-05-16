package job

import (
	"context"
	"fmt"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/reader"
	"github.com/tofunmiadewuyi/dbq/internal/source"
	"github.com/tofunmiadewuyi/dbq/internal/storage"
)

func TestDump(j *Job) error {
	start := time.Now()
	err := runTestDump(j)
	d := time.Since(start).Round(time.Millisecond)
	AppendLog(j.ID, "test dump", d, err)
	if err != nil {
		return err
	}
	fmt.Printf("✅ Dump test passed in %s\n", d)
	return nil
}

func runTestDump(j *Job) error {
	driver, err := source.NewDBDriver(j.Database.Type)
	if err != nil {
		return fmt.Errorf("db driver error: %w", err)
	}

	fileReader, err := reader.GetFileReader(j.ReaderSSH())
	if err != nil {
		return fmt.Errorf("file reader error: %w", err)
	}
	defer fileReader.Close()

	return driver.Test(j.SourceJob(), fileReader)
}

func TestStorage(j *Job) error {
	start := time.Now()
	err := runTestStorage(j)
	d := time.Since(start).Round(time.Millisecond)
	AppendLog(j.ID, "test storage", d, err)
	if err != nil {
		return err
	}
	fmt.Printf("✅ Storage test passed in %s\n", d)
	return nil
}

func runTestStorage(j *Job) error {
	client, err := storage.NewStorageClient(&j.Storage)
	if err != nil {
		return fmt.Errorf("failed to init storage client: %w", err)
	}
	return client.TestConnection(context.Background())
}
