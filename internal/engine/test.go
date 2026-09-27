package engine

import (
	"context"
	"fmt"
	"time"
)

func (e *Engine) TestSource(_ context.Context, req Request) (result TestResult, err error) {
	started := e.now()
	defer func() { result.Duration = e.now().Sub(started).Round(time.Millisecond) }()
	driver, err := e.newDriver(req.Database.Type)
	if err != nil {
		return TestResult{}, fmt.Errorf("db driver error: %w", err)
	}
	fileReader, err := e.newReader(&req.SSH)
	if err != nil {
		return TestResult{}, fmt.Errorf("file reader error: %w", err)
	}
	defer closeReader(fileReader)
	err = driver.Test(e.sourceJob(req), fileReader)
	return result, err
}

func (e *Engine) TestStorage(ctx context.Context, req Request) (result TestResult, err error) {
	started := e.now()
	defer func() { result.Duration = e.now().Sub(started).Round(time.Millisecond) }()
	client, err := e.newStorage(&req.CloudStorage, e.storageCredentials(req))
	if err != nil {
		return TestResult{}, fmt.Errorf("failed to init storage client: %w", err)
	}
	err = client.TestConnection(ctx)
	return result, err
}
