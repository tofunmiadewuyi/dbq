package engine

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/reader"
	"github.com/tofunmiadewuyi/dbq/internal/source"
)

func (e *Engine) streamToCloud(ctx context.Context, req Request, started time.Time, driver source.StreamDriver, runner reader.ContextCommandRunner) (string, error) {
	client, err := e.newStorage(&req.CloudStorage, e.storageCredentials(req))
	if err != nil {
		return "", fmt.Errorf("failed to init storage client: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	input, output := io.Pipe()
	defer input.Close()
	// Closing both ends unblocks reads and writes, including while the producer
	// is waiting for a database response rather than emitting bytes.
	stop := context.AfterFunc(ctx, func() {
		_ = input.CloseWithError(ctx.Err())
		_ = output.CloseWithError(ctx.Err())
	})
	defer stop()
	producer := make(chan error, 1)
	go func() {
		compressed := gzip.NewWriter(output)
		err := driver.DumpStream(ctx, e.sourceJob(req), runner, compressed)
		if err != nil {
			err = fmt.Errorf("failed to dump database: %w", err)
		} else if closeErr := compressed.Close(); closeErr != nil {
			err = fmt.Errorf("failed to compress dump: %w", closeErr)
		}
		// Normal EOF is only published after a successful process exit and footer.
		_ = output.CloseWithError(err)
		producer <- err
	}()
	extension := ".dump.gz"
	if req.Database.Type == config.MySQL {
		extension = ".sql.gz"
	}
	key, uploadErr := client.UploadBackup(ctx, started, req.Name, req.Database.Name, extension, "application/gzip", input)
	if uploadErr != nil {
		cancel()
		_ = input.CloseWithError(uploadErr)
	}
	producerErr := <-producer
	if uploadErr != nil {
		return "", fmt.Errorf("upload backup stream: %w", uploadErr)
	}
	if producerErr != nil {
		return "", producerErr
	}
	return key, nil
}
