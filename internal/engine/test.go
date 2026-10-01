package engine

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/storage"
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

// Health runs the non-destructive checks required before a backup. Run calls
// this same method so explicit health checks and execution cannot drift apart.
func (e *Engine) Health(ctx context.Context, req Request) HealthResult {
	source, sourceErr := e.TestSource(ctx, req)
	checks := []HealthCheck{{Name: "database", Duration: source.Duration, Error: sourceErr}}

	started := e.now()
	var storageErr error
	if req.StorageType == storage.TypeDirectory {
		storageErr = checkDirectoryWritable(req.Destination)
	} else {
		_, storageErr = e.TestStorage(ctx, req)
	}
	checks = append(checks, HealthCheck{
		Name: "storage", Duration: e.now().Sub(started).Round(time.Millisecond), Error: storageErr,
	})
	return HealthResult{Checks: checks}
}

func checkDirectoryWritable(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("storage directory %q is unavailable: %w%s", dir, err, createHint(dir))
	}
	if !info.IsDir() {
		return fmt.Errorf("storage destination %q is not a directory", dir)
	}
	probe, err := os.CreateTemp(dir, ".dbq-health-*")
	if err != nil {
		return fmt.Errorf("storage directory %q is not writable by %s: %w%s", dir, runningUser(), err, grantHint(dir))
	}
	name := probe.Name()
	closeErr := probe.Close()
	removeErr := os.Remove(name)
	if closeErr != nil {
		return fmt.Errorf("close storage probe: %w", closeErr)
	}
	if removeErr != nil {
		return fmt.Errorf("remove storage probe: %w", removeErr)
	}
	return nil
}

// createHint tells the operator how to create the directory so the first dump
// does not land in a world-readable path.
func createHint(dir string) string {
	owner, ok := chownTarget()
	if !ok {
		return ""
	}
	return fmt.Sprintf("; create it with: sudo mkdir -p %q && sudo chown %s %q && sudo chmod 700 %q", dir, owner, dir, dir)
}

func grantHint(dir string) string {
	owner, ok := chownTarget()
	if !ok {
		return ""
	}
	return fmt.Sprintf("; grant access with: sudo chown %s %q && sudo chmod 700 %q", owner, dir, dir)
}

// chownTarget names the uid dbq runs as. Not ok on platforms without uids,
// where the shell hints would be meaningless.
func chownTarget() (string, bool) {
	uid := os.Getuid()
	if uid < 0 {
		return "", false
	}
	if u, err := user.LookupId(strconv.Itoa(uid)); err == nil && u.Username != "" {
		return u.Username, true
	}
	return strconv.Itoa(uid), true
}

func runningUser() string {
	owner, ok := chownTarget()
	if !ok {
		return "this process"
	}
	return fmt.Sprintf("uid %d (%s)", os.Getuid(), owner)
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
