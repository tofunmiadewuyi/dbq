package engine

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/storage"
)

type streamStorage struct {
	fakeStorage
	consume func(io.Reader) error
}

func (s *streamStorage) UploadBackup(_ context.Context, _ time.Time, _, _, ext, content string, r io.Reader) (string, error) {
	s.extension, s.content = ext, content
	if err := s.consume(r); err != nil {
		return "", err
	}
	return "backup-key", nil
}

// Run the real drivers and local process runner through the engine. Assert the
// stored artifact, producer failure propagation, and termination of a stalled
// child when storage fails or the request is cancelled.
func TestLocalCloudStreaming(t *testing.T) {
	for _, db := range []config.DatabaseType{config.MySQL, config.Postgres} {
		for _, scenario := range []string{"success", "dump failure", "upload failure", "cancel"} {
			t.Run(string(db)+"/"+scenario, func(t *testing.T) {
				dir := t.TempDir()
				program := "mysqldump"
				if db == config.Postgres {
					program = "pg_dump"
				}
				script := "#!/bin/sh\nprintf 'database contents'\n"
				if db == config.MySQL {
					script = "#!/bin/sh\n" + `for flag in --single-transaction --quick --skip-lock-tables; do
                        found=no
                        for arg in "$@"; do [ "$arg" != "$flag" ] || found=yes; done
                        [ "$found" = yes ] || { echo "missing snapshot option: $flag" >&2; exit 9; }
                    done
                    printf 'database contents'
                    `
				}
				if scenario == "dump failure" {
					script += "echo 'database disconnected' >&2\nexit 1\n"
				}
				if scenario == "upload failure" || scenario == "cancel" {
					script += "sleep 60\n"
				}
				if err := os.WriteFile(filepath.Join(dir, program), []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				store := &streamStorage{}
				store.consume = func(r io.Reader) error {
					if scenario == "upload failure" || scenario == "cancel" {
						// gzip emits its header before the dump exits; fail mid-production.
						header := make([]byte, 10)
						if _, err := io.ReadFull(r, header); err != nil {
							return err
						}
						if scenario == "cancel" {
							cancel()
							_, err := io.Copy(io.Discard, r)
							return err
						}
						return errors.New("storage unavailable")
					}
					data, err := io.ReadAll(r)
					if err != nil {
						return err
					}
					gz, err := gzip.NewReader(bytes.NewReader(data))
					if err != nil {
						return err
					}
					defer gz.Close()
					plain, err := io.ReadAll(gz)
					if err != nil {
						return err
					}
					if string(plain) != "database contents" {
						t.Errorf("dump content: %q", plain)
					}
					return nil
				}
				e := New()
				e.newStorage = func(*storage.CloudStorage, storage.Credentials) (storage.StorageClient, error) { return store, nil }
				req := cloudRequest()
				req.Database.Type = db
				req.Secrets.DatabasePassword = "private-password"
				key, _, err := e.runBackup(ctx, req, time.Now())
				if scenario == "success" {
					if err != nil || key != "backup-key" {
						t.Fatalf("key=%q err=%v", key, err)
					}
					ext := ".dump.gz"
					if db == config.MySQL {
						ext = ".sql.gz"
					}
					if store.extension != ext || store.content != "application/gzip" {
						t.Fatalf("artifact: %s %s", store.extension, store.content)
					}
				} else {
					if err == nil || key != "" {
						t.Fatalf("key=%q err=%v", key, err)
					}
					if scenario == "dump failure" && !strings.Contains(err.Error(), "database disconnected") {
						t.Fatal(err)
					}
					if errors.Is(ctx.Err(), context.DeadlineExceeded) {
						t.Fatal("pipeline failed to stop promptly")
					}
				}
			})
		}
	}
}
