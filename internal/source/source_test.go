package source

import (
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"
)

// mockReader implements reader.FileReader for testing.
// Commands are matched by prefix against the errors map; unmatched commands succeed.
type mockReader struct {
	calls  []string
	errors map[string]error
}

func (m *mockReader) Exec(cmd string) ([]byte, error) {
	m.calls = append(m.calls, cmd)
	for prefix, err := range m.errors {
		if strings.HasPrefix(cmd, prefix) {
			return nil, err
		}
	}
	return []byte("ok"), nil
}

func (m *mockReader) ExecStream(cmd string, dst io.Writer) error {
	m.calls = append(m.calls, cmd)
	for prefix, err := range m.errors {
		if strings.HasPrefix(cmd, prefix) {
			return err
		}
	}
	return nil
}

func (m *mockReader) ReadDir(path string) ([]fs.DirEntry, error) { return nil, nil }
func (m *mockReader) ReadFile(path string) ([]byte, error)        { return nil, nil }
func (m *mockReader) Stat(path string) (fs.FileInfo, error)       { return nil, nil }
func (m *mockReader) Close() error                                 { return nil }

// calledContaining returns the first recorded call that contains substr, or "".
func (m *mockReader) calledContaining(substr string) string {
	for _, c := range m.calls {
		if strings.Contains(c, substr) {
			return c
		}
	}
	return ""
}

// calledWith returns true if any recorded call starts with prefix.
func (m *mockReader) calledWith(prefix string) bool {
	for _, c := range m.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

// --- Postgres ---

func TestPostgresDumpRemote_ToolNotFound(t *testing.T) {
	r := &mockReader{errors: map[string]error{"which pg_dump": errors.New("not found")}}
	err := (&Postgres{}).DumpRemote(&SourceJob{}, r, "/tmp/test.dump")
	if err == nil {
		t.Fatal("expected error when pg_dump not found")
	}
	if !strings.Contains(err.Error(), "pg_dump not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPostgresDumpRemote_CommandContents(t *testing.T) {
	r := &mockReader{}
	job := &SourceJob{Host: "db.example.com", Port: "5432", Username: "admin", Password: "secret", Name: "mydb"}

	if err := (&Postgres{}).DumpRemote(job, r, "/var/tmp/dbq/test.dump"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cmd := r.calledContaining("pg_dump -Fc")
	if cmd == "" {
		t.Fatal("no pg_dump -Fc command issued")
	}
	for _, want := range []string{"-Fc", "db.example.com", "5432", "admin", "mydb", "/var/tmp/dbq/test.dump"} {
		if !strings.Contains(cmd, want) {
			t.Errorf("dump command missing %q in: %s", want, cmd)
		}
	}
}

func TestPostgresDumpRemote_CleansUpOnFailure(t *testing.T) {
	r := &mockReader{errors: map[string]error{"mkdir -p": errors.New("dump failed")}}
	(&Postgres{}).DumpRemote(&SourceJob{Name: "mydb"}, r, "/tmp/test.dump") //nolint:errcheck

	if !r.calledWith("rm -f '/tmp/test.dump'") {
		t.Error("expected cleanup (rm -f) after dump failure")
	}
}

// --- MySQL ---

func TestMySQLDumpRemote_ToolNotFound(t *testing.T) {
	r := &mockReader{errors: map[string]error{"which mysqldump": errors.New("not found")}}
	err := (&MySQL{}).DumpRemote(&SourceJob{}, r, "/tmp/test.sql")
	if err == nil {
		t.Fatal("expected error when mysqldump not found")
	}
	if !strings.Contains(err.Error(), "mysqldump not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestMySQLDumpRemote_CommandContents(t *testing.T) {
	r := &mockReader{}
	job := &SourceJob{Host: "db.example.com", Port: "3306", Username: "root", Password: "secret", Name: "mydb"}

	if err := (&MySQL{}).DumpRemote(job, r, "/var/tmp/dbq/test.sql"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cmd := r.calledContaining("mysqldump -h")
	if cmd == "" {
		t.Fatal("no mysqldump command issued")
	}
	for _, want := range []string{"mysqldump", "db.example.com", "3306", "root", "mydb", "/var/tmp/dbq/test.sql"} {
		if !strings.Contains(cmd, want) {
			t.Errorf("dump command missing %q in: %s", want, cmd)
		}
	}
}

func TestMySQLDumpRemote_CleansUpOnFailure(t *testing.T) {
	r := &mockReader{errors: map[string]error{"mkdir -p": errors.New("dump failed")}}
	(&MySQL{}).DumpRemote(&SourceJob{Name: "mydb"}, r, "/tmp/test.sql") //nolint:errcheck

	if !r.calledWith("rm -f '/tmp/test.sql'") {
		t.Error("expected cleanup (rm -f) after dump failure")
	}
}
