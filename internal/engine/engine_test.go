package engine

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/reader"
	"github.com/tofunmiadewuyi/dbq/internal/source"
	"github.com/tofunmiadewuyi/dbq/internal/storage"
)

type fakeReader struct{ closed bool }

func (r *fakeReader) ReadDir(string) ([]fs.DirEntry, error) { return nil, nil }
func (r *fakeReader) ReadFile(string) ([]byte, error)       { return nil, nil }
func (r *fakeReader) Stat(string) (fs.FileInfo, error)      { return nil, nil }
func (r *fakeReader) Exec(string) ([]byte, error)           { return nil, nil }
func (r *fakeReader) ExecStream(string, io.Writer) error    { return nil }
func (r *fakeReader) Close() error                          { r.closed = true; return nil }

type fakeDriver struct {
	dumpPath string
	dumpErr  error
	testErr  error
	dumped   bool
	tested   bool
}

func (d *fakeDriver) Dump(*source.SourceJob, reader.FileReader) (string, error) {
	d.dumped = true
	return d.dumpPath, d.dumpErr
}
func (d *fakeDriver) DumpRemote(*source.SourceJob, reader.FileReader, string) error {
	return errors.New("unexpected server-side dump")
}
func (d *fakeDriver) Test(*source.SourceJob, reader.FileReader) error {
	d.tested = true
	return d.testErr
}

type fakeStorage struct {
	uploadKey string
	uploadErr error
	listErr   error
	testErr   error
	objects   []storage.BackupObject
	uploaded  bool
	extension string
	content   string
	listed    bool
	deleted   []string
	tested    bool
}

func (s *fakeStorage) UploadBackup(_ context.Context, _ time.Time, _, _, extension, contentType string, _ io.Reader) (string, error) {
	s.uploaded = true
	s.extension = extension
	s.content = contentType
	return s.uploadKey, s.uploadErr
}
func (s *fakeStorage) TestConnection(context.Context) error {
	s.tested = true
	return s.testErr
}
func (s *fakeStorage) PresignPutURL(context.Context, string, time.Duration) (string, error) {
	return "", errors.New("unexpected presign")
}
func (s *fakeStorage) ListBackups(context.Context, string, string) ([]storage.BackupObject, error) {
	s.listed = true
	return s.objects, s.listErr
}
func (s *fakeStorage) DeleteBackup(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return nil
}

func cloudRequest() Request {
	return Request{
		ID:   "daily",
		Name: "daily",
		Database: Database{
			Type: config.Postgres,
			Name: "app",
		},
		StorageType: storage.TypeCloud,
		Retention:   1,
	}
}

func configuredEngine(t *testing.T, driver source.DBDriver, client storage.StorageClient) (*Engine, *fakeReader) {
	t.Helper()
	r := &fakeReader{}
	e := New()
	e.newDriver = func(config.DatabaseType) (source.DBDriver, error) { return driver, nil }
	e.newReader = func(*reader.SSHConn) (reader.FileReader, error) { return r, nil }
	e.newStorage = func(*storage.CloudStorage, storage.Credentials) (storage.StorageClient, error) {
		return client, nil
	}
	e.zipFile = func(_, dst string) error { return os.WriteFile(dst, []byte("zip"), 0o600) }
	return e, r
}

func TestRunKeepsPostgresCustomFormatUncompressed(t *testing.T) {
	dumpPath := filepath.Join(t.TempDir(), "dump")
	if err := os.WriteFile(dumpPath, []byte("dump"), 0o600); err != nil {
		t.Fatal(err)
	}
	driver := &fakeDriver{dumpPath: dumpPath}
	client := &fakeStorage{
		uploadKey: "backups/daily/app/latest.dump",
		objects: []storage.BackupObject{
			{Key: "new", Timestamp: time.Unix(2, 0)},
			{Key: "old", Timestamp: time.Unix(1, 0)},
		},
	}
	e, r := configuredEngine(t, driver, client)
	base := time.Unix(100, 0)
	e.now = func() time.Time {
		base = base.Add(time.Second)
		return base
	}

	result, err := e.Run(context.Background(), cloudRequest())
	if err != nil {
		t.Fatal(err)
	}
	if !driver.dumped || !client.uploaded || !client.listed {
		t.Fatalf("dumped=%v uploaded=%v listed=%v", driver.dumped, client.uploaded, client.listed)
	}
	if result.StorageKey != client.uploadKey || result.PrunedBackups != 1 || result.RetentionError != nil {
		t.Fatalf("unexpected result: %#v", result)
	}
	if client.extension != ".dump" || client.content != "application/octet-stream" {
		t.Fatalf("postgres artifact = %q %q", client.extension, client.content)
	}
	if len(client.deleted) != 1 || client.deleted[0] != "old" {
		t.Fatalf("deleted = %#v", client.deleted)
	}
	if !r.closed {
		t.Fatal("reader was not closed")
	}
	if _, err := os.Stat(dumpPath); !os.IsNotExist(err) {
		t.Fatalf("temporary file %s still exists: %v", dumpPath, err)
	}
}

func TestRunCompressesMySQLSQLArtifact(t *testing.T) {
	dumpPath := filepath.Join(t.TempDir(), "dump.sql")
	if err := os.WriteFile(dumpPath, []byte("sql"), 0o600); err != nil {
		t.Fatal(err)
	}
	driver := &fakeDriver{dumpPath: dumpPath}
	client := &fakeStorage{uploadKey: "backups/daily/app/latest.sql.zip"}
	e, _ := configuredEngine(t, driver, client)
	req := cloudRequest()
	req.Database.Type = config.MySQL
	if _, err := e.Run(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if client.extension != ".sql.zip" || client.content != "application/zip" {
		t.Fatalf("mysql artifact = %q %q", client.extension, client.content)
	}
	for _, path := range []string{dumpPath, dumpPath + ".zip"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("temporary file %s still exists: %v", path, err)
		}
	}
}

func TestRunDoesNotUploadOrPruneAfterDumpFailure(t *testing.T) {
	driver := &fakeDriver{dumpErr: errors.New("dump failed")}
	client := &fakeStorage{}
	e, _ := configuredEngine(t, driver, client)

	_, err := e.Run(context.Background(), cloudRequest())
	if err == nil {
		t.Fatal("expected dump error")
	}
	if client.uploaded || client.listed {
		t.Fatalf("uploaded=%v listed=%v", client.uploaded, client.listed)
	}
}

func TestRunDoesNotPruneAfterUploadFailure(t *testing.T) {
	dumpPath := filepath.Join(t.TempDir(), "dump")
	if err := os.WriteFile(dumpPath, []byte("dump"), 0o600); err != nil {
		t.Fatal(err)
	}
	driver := &fakeDriver{dumpPath: dumpPath}
	client := &fakeStorage{uploadErr: errors.New("upload failed")}
	e, _ := configuredEngine(t, driver, client)

	_, err := e.Run(context.Background(), cloudRequest())
	if err == nil {
		t.Fatal("expected upload error")
	}
	if client.listed {
		t.Fatal("retention ran after upload failure")
	}
}

func TestRunReportsRetentionFailureWithoutFailingBackup(t *testing.T) {
	dumpPath := filepath.Join(t.TempDir(), "dump")
	if err := os.WriteFile(dumpPath, []byte("dump"), 0o600); err != nil {
		t.Fatal(err)
	}
	driver := &fakeDriver{dumpPath: dumpPath}
	client := &fakeStorage{uploadKey: "key", listErr: errors.New("list failed")}
	e, _ := configuredEngine(t, driver, client)

	result, err := e.Run(context.Background(), cloudRequest())
	if err != nil {
		t.Fatalf("backup should remain successful: %v", err)
	}
	if result.RetentionError == nil {
		t.Fatal("expected separate retention error")
	}
}

func TestSourceAndStorageTestsUseEngineDependencies(t *testing.T) {
	driver := &fakeDriver{}
	client := &fakeStorage{}
	e, r := configuredEngine(t, driver, client)
	req := cloudRequest()

	if _, err := e.TestSource(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if _, err := e.TestStorage(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if !driver.tested || !client.tested || !r.closed {
		t.Fatalf("driver tested=%v storage tested=%v reader closed=%v", driver.tested, client.tested, r.closed)
	}
}

func TestHealthChecksDatabaseAndWritableDirectory(t *testing.T) {
	driver := &fakeDriver{}
	e, _ := configuredEngine(t, driver, &fakeStorage{})
	req := cloudRequest()
	req.StorageType = storage.TypeDirectory
	req.Destination = t.TempDir()

	result := e.Health(context.Background(), req)
	if err := result.Error(); err != nil {
		t.Fatal(err)
	}
	if len(result.Checks) != 2 || result.Checks[0].Name != "database" || result.Checks[1].Name != "storage" {
		t.Fatalf("checks = %#v", result.Checks)
	}
	if !driver.tested {
		t.Fatal("database health check did not run")
	}
}

func TestRunStopsBeforeDumpWhenDirectoryIsNotWritable(t *testing.T) {
	driver := &fakeDriver{}
	e, _ := configuredEngine(t, driver, &fakeStorage{})
	req := cloudRequest()
	req.StorageType = storage.TypeDirectory
	req.Destination = filepath.Join(t.TempDir(), "missing")

	_, err := e.Run(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "preflight failed") {
		t.Fatalf("error = %v", err)
	}
	if driver.dumped {
		t.Fatal("dump started after preflight failure")
	}
}
