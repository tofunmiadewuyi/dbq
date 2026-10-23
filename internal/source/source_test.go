package source

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/tofunmiadewuyi/dbq/internal/reader"
)

// mockReader implements reader.FileReader for testing.
// Commands are matched by prefix against the errors map; unmatched commands succeed.
type mockReader struct {
	calls         []string
	errors        map[string]error
	clientVersion string
	serverVersion string
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
	outputPath        string
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

func (r *remoteReader) ExecRemoteCommand(name string, args, env []string, dst io.Writer) error {
	r.writePostgresOutput(name, dst)
	r.name = name
	r.args = append([]string(nil), args...)
	r.env = append([]string(nil), env...)
	return r.runErr
}

func (r *remoteReader) ExecRemoteCommandToFile(name string, args, env []string, outputPath string) error {
	r.outputPath = outputPath
	return r.ExecRemoteCommand(name, args, env, io.Discard)
}

func (r *directReader) LookPath(string) error { return nil }

func (r *directReader) ExecCommand(name string, args, env []string, dst io.Writer) error {
	r.writePostgresOutput(name, dst)
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
	if r.call.name != "psql" {
		t.Fatalf("command = %q", r.call.name)
	}
	wantArgs := postgresHealthArgs(job)
	if !reflect.DeepEqual(r.call.args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", r.call.args, wantArgs)
	}
	if !reflect.DeepEqual(r.call.env, postgresHealthEnv(r.call.credentialPath)) {
		t.Fatalf("env = %#v", r.call.env)
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
	wantTail := []string{"--connect-timeout=10", "-h", job.Host, "-P", job.Port, "-u", job.Username, "ping"}
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
	wantArgs := postgresHealthArgs(job)
	if r.name != "psql" || !reflect.DeepEqual(r.args, wantArgs) {
		t.Fatalf("command = %q %#v, want psql %#v", r.name, r.args, wantArgs)
	}
	if !reflect.DeepEqual(r.env, postgresHealthEnv(r.credentialPath)) {
		t.Fatalf("env = %#v", r.env)
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
	wantArgs := []string{"--defaults-extra-file=" + r.credentialPath, "--connect-timeout=10", "-h", job.Host, "-P", job.Port, "-u", job.Username, "ping"}
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

func TestServerSideDumpKeepsPasswordOutOfCommandAndCleansCredentials(t *testing.T) {
	for _, driver := range []struct {
		name   string
		driver DBDriver
	}{{"postgres", &Postgres{}}, {"mysql", &MySQL{}}} {
		t.Run(driver.name, func(t *testing.T) {
			job := &SourceJob{Host: "db.example.com", Port: "5432", Username: "backup", Password: "password'$(do-not-run);end", Name: "app"}
			for _, scenario := range []string{"success", "command failure", "cleanup failure", "both failures"} {
				t.Run(scenario, func(t *testing.T) {
					r := &remoteReader{}
					runErr := errors.New("dump failed")
					cleanupErr := errors.New("credential removal failed")
					if strings.Contains(scenario, "command") || scenario == "both failures" {
						r.runErr = runErr
					}
					if strings.Contains(scenario, "cleanup") || scenario == "both failures" {
						r.removeErr = cleanupErr
					}
					outputPath := "/var/tmp/dbq/app.dump"
					err := driver.driver.DumpRemote(job, r, outputPath)
					for _, command := range r.calls {
						if strings.Contains(command, job.Password) {
							t.Fatal("password embedded in shell command")
						}
					}
					if r.runErr == nil && r.removeErr == nil && err != nil {
						t.Fatal(err)
					}
					if r.runErr != nil && !errors.Is(err, runErr) {
						t.Fatal("dump failure lost")
					}
					if r.removeErr != nil && !errors.Is(err, cleanupErr) {
						t.Fatal("cleanup failure lost")
					}
					if driver.name == "mysql" {
						for _, option := range []string{"--single-transaction", "--quick", "--skip-lock-tables", "--routines", "--events", "--triggers"} {
							if !strings.Contains(" "+strings.Join(r.args, " ")+" ", " "+option+" ") {
								t.Fatalf("MySQL backup omits %s", option)
							}
						}
					}
					if r.outputPath != outputPath || r.name == "" || !r.removed {
						t.Fatal("server-side execution or credential cleanup missing")
					}
					if strings.Contains(strings.Join(append(r.args, r.env...), " "), job.Password) {
						t.Fatal("password reached command arguments or environment")
					}
					if !strings.Contains(r.credentialContent, job.Password) {
						t.Fatal("credential file missing password")
					}
					if !strings.Contains(strings.Join(append(r.args, r.env...), " "), r.credentialPath) {
						t.Fatal("dump does not use credential file")
					}
				})
			}
		})
	}
}

func (m *mockReader) writePostgresOutput(name string, dst io.Writer) {
	if name == "pg_dump" {
		version := m.clientVersion
		if version == "" {
			version = "pg_dump (PostgreSQL) 17.6"
		}
		fmt.Fprintln(dst, version)
	}
	if name == "psql" {
		version := m.serverVersion
		if version == "" {
			version = "170006"
		}
		fmt.Fprintln(dst, version)
	}
}

func TestPostgresHealthChecksDumpCompatibility(t *testing.T) {
	for _, tc := range []struct{ name, client, server, wantError string }{
		{"older client", "pg_dump (PostgreSQL) 16.9 (Ubuntu 16.9-0ubuntu0.24.10.1)", "170006", "pg_dump 16 cannot back up PostgreSQL 17"},
		{"same major", "pg_dump (PostgreSQL) 17.1", "170006", ""},
		{"newer client", "pg_dump (PostgreSQL) 18.0", "170006", ""},
		{"older pre-10 client", "pg_dump (PostgreSQL) 9.5.25", "90624", "cannot back up"},
		{"unknown client", "unexpected", "170006", "version"},
		{"unknown server", "pg_dump (PostgreSQL) 17.6", "unexpected", "version"},
	} {
		for _, remote := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/ssh=%t", tc.name, remote), func(t *testing.T) {
				m := mockReader{clientVersion: tc.client, serverVersion: tc.server}
				var r reader.FileReader = &directReader{mockReader: m}
				if remote {
					r = &remoteReader{mockReader: m}
				}
				err := (&Postgres{}).Test(&SourceJob{Host: "localhost", Port: "5432", Name: "app", Username: "backup", Password: "secret"}, r)
				if tc.wantError == "" {
					if err != nil {
						t.Fatal(err)
					}
				} else if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want %q", err, tc.wantError)
				}
				if rr, ok := r.(*remoteReader); ok && !rr.removed {
					t.Fatal("credential file was not removed")
				}
				if lr, ok := r.(*directReader); ok && lr.call.credentialPath != "" {
					if _, err := os.Stat(lr.call.credentialPath); !os.IsNotExist(err) {
						t.Fatalf("credential file was not removed: %v", err)
					}
				}
			})
		}
	}
}
