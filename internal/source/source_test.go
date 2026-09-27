package source

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"reflect"
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
func (m *mockReader) ReadFile(path string) ([]byte, error)       { return nil, nil }
func (m *mockReader) Stat(path string) (fs.FileInfo, error)      { return nil, nil }
func (m *mockReader) Close() error                               { return nil }

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

type directCall struct {
	name              string
	args              []string
	env               []string
	credentialPath    string
	credentialContent string
	credentialMode    fs.FileMode
}

type directReader struct {
	mockReader
	call directCall
}

type remoteReader struct {
	mockReader
	credentialPath    string
	credentialContent string
	removed           bool
	name              string
	args              []string
	env               []string
	runErr            error
	removeErr         error
}

func (r *remoteReader) CreateCredentialFile(prefix, contents string) (string, error) {
	r.credentialPath = "/tmp/" + prefix + "test"
	r.credentialContent = contents
	return r.credentialPath, nil
}

func (r *remoteReader) RemoveCredentialFile(path string) error {
	if path == r.credentialPath {
		r.removed = true
	}
	return r.removeErr
}

func (r *remoteReader) ExecRemoteCommand(name string, args, env []string, _ io.Writer) error {
	r.name = name
	r.args = append([]string(nil), args...)
	r.env = append([]string(nil), env...)
	return r.runErr
}

func (r *directReader) LookPath(string) error { return nil }

func (r *directReader) ExecCommand(name string, args, env []string, _ io.Writer) error {
	r.call.name = name
	r.call.args = append([]string(nil), args...)
	r.call.env = append([]string(nil), env...)
	for _, value := range append(args, env...) {
		var path string
		switch {
		case strings.HasPrefix(value, "PGPASSFILE="):
			path = strings.TrimPrefix(value, "PGPASSFILE=")
		case strings.HasPrefix(value, "--defaults-extra-file="):
			path = strings.TrimPrefix(value, "--defaults-extra-file=")
		}
		if path == "" {
			continue
		}
		r.call.credentialPath = path
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		r.call.credentialContent = string(data)
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		r.call.credentialMode = info.Mode().Perm()
	}
	return nil
}

func TestPostgresLocalTestUsesArgvAndCredentialFile(t *testing.T) {
	r := &directReader{}
	job := &SourceJob{
		Host:     "db.example.com; touch /tmp/pwned",
		Port:     "5432",
		Username: "admin user",
		Password: `pa:ss\word'$(bad)`,
		Name:     "database name",
	}

	if err := (&Postgres{}).Test(job, r); err != nil {
		t.Fatal(err)
	}
	if r.call.name != "pg_dump" {
		t.Fatalf("command = %q", r.call.name)
	}
	wantArgs := []string{"--schema-only", "-w", "-h", job.Host, "-p", job.Port, "-U", job.Username, "-d", job.Name}
	if !reflect.DeepEqual(r.call.args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", r.call.args, wantArgs)
	}
	if strings.Contains(strings.Join(append(r.call.args, r.call.env...), " "), job.Password) {
		t.Fatal("password appeared in argv or environment")
	}
	if r.call.credentialMode != 0o600 {
		t.Fatalf("credential mode = %o", r.call.credentialMode)
	}
	if !strings.Contains(r.call.credentialContent, `pa\:ss\\word'$(bad)`) {
		t.Fatalf("unexpected pgpass content: %q", r.call.credentialContent)
	}
	if _, err := os.Stat(r.call.credentialPath); !os.IsNotExist(err) {
		t.Fatalf("credential file was not removed: %v", err)
	}
}

func TestMySQLLocalTestUsesArgvAndCredentialFile(t *testing.T) {
	r := &directReader{}
	job := &SourceJob{
		Host:     "db.example.com; touch /tmp/pwned",
		Port:     "3306",
		Username: "root user",
		Password: "quote\" slash\\ newline\nvalue",
		Name:     "database name",
	}

	if err := (&MySQL{}).Test(job, r); err != nil {
		t.Fatal(err)
	}
	if r.call.name != "mysqladmin" {
		t.Fatalf("command = %q", r.call.name)
	}
	if len(r.call.args) == 0 || !strings.HasPrefix(r.call.args[0], "--defaults-extra-file=") {
		t.Fatalf("defaults file is not the first argument: %#v", r.call.args)
	}
	wantTail := []string{"-h", job.Host, "-P", job.Port, "-u", job.Username, "ping"}
	if !reflect.DeepEqual(r.call.args[1:], wantTail) {
		t.Fatalf("args = %#v, want defaults file followed by %#v", r.call.args, wantTail)
	}
	if strings.Contains(strings.Join(append(r.call.args, r.call.env...), " "), job.Password) {
		t.Fatal("password appeared in argv or environment")
	}
	if r.call.credentialMode != 0o600 {
		t.Fatalf("credential mode = %o", r.call.credentialMode)
	}
	if !strings.Contains(r.call.credentialContent, `password="quote\" slash\\ newline\nvalue"`) {
		t.Fatalf("unexpected option-file content: %q", r.call.credentialContent)
	}
	if _, err := os.Stat(r.call.credentialPath); !os.IsNotExist(err) {
		t.Fatalf("credential file was not removed: %v", err)
	}
}

func TestPostgresSSHTestKeepsPasswordOutOfCommand(t *testing.T) {
	r := &remoteReader{}
	job := &SourceJob{
		Host:     "db.example.com; touch /tmp/pwned",
		Port:     "5432",
		Username: "admin user",
		Password: `pa:ss\word'$(bad)`,
		Name:     "database name",
	}

	if err := (&Postgres{}).Test(job, r); err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"--schema-only", "-w", "-h", job.Host, "-p", job.Port, "-U", job.Username, "-d", job.Name}
	if r.name != "pg_dump" || !reflect.DeepEqual(r.args, wantArgs) {
		t.Fatalf("command = %q %#v, want pg_dump %#v", r.name, r.args, wantArgs)
	}
	if strings.Contains(strings.Join(append(r.args, r.env...), " "), job.Password) {
		t.Fatal("password appeared in remote command")
	}
	if !strings.Contains(r.credentialContent, `pa\:ss\\word'$(bad)`) {
		t.Fatalf("unexpected pgpass content: %q", r.credentialContent)
	}
	if !r.removed {
		t.Fatal("remote credential file was not removed")
	}
}

func TestMySQLSSHTestKeepsPasswordOutOfCommand(t *testing.T) {
	r := &remoteReader{}
	job := &SourceJob{
		Host:     "db.example.com; touch /tmp/pwned",
		Port:     "3306",
		Username: "root user",
		Password: "quote\" slash\\ newline\nvalue",
		Name:     "database name",
	}

	if err := (&MySQL{}).Test(job, r); err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"--defaults-extra-file=" + r.credentialPath, "-h", job.Host, "-P", job.Port, "-u", job.Username, "ping"}
	if r.name != "mysqladmin" || !reflect.DeepEqual(r.args, wantArgs) {
		t.Fatalf("command = %q %#v, want mysqladmin %#v", r.name, r.args, wantArgs)
	}
	if strings.Contains(strings.Join(append(r.args, r.env...), " "), job.Password) {
		t.Fatal("password appeared in remote command")
	}
	if !strings.Contains(r.credentialContent, `password="quote\" slash\\ newline\nvalue"`) {
		t.Fatalf("unexpected option-file content: %q", r.credentialContent)
	}
	if !r.removed {
		t.Fatal("remote credential file was not removed")
	}
}

func TestSSHTestRemovesCredentialAfterCommandFailure(t *testing.T) {
	r := &remoteReader{runErr: errors.New("dump failed")}
	err := (&Postgres{}).Test(&SourceJob{Host: "localhost", Port: "5432", Name: "db", Username: "user", Password: "secret"}, r)
	if err == nil {
		t.Fatal("expected command failure")
	}
	if !r.removed {
		t.Fatal("remote credential file was not removed after command failure")
	}
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
