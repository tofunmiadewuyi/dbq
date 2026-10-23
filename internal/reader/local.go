package reader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/secrets"
)

// LocalFileReader reads from local filesystem
type LocalFileReader struct{}

func (l *LocalFileReader) ReadDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(path)
}

func (l *LocalFileReader) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (l *LocalFileReader) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

func (l *LocalFileReader) Exec(cmd string) ([]byte, error) {
	c := exec.Command("sh", "-c", cmd)
	c.Env = childEnv(nil)
	var stderr bytes.Buffer
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}
	return out, nil
}

func (l *LocalFileReader) ExecStream(cmd string, dst io.Writer) error {
	c := exec.Command("sh", "-c", cmd)
	c.Env = childEnv(nil)
	var stderr bytes.Buffer
	c.Stderr = &stderr
	c.Stdout = dst
	if err := c.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return dumpDiagnosticError(stderr.String())
}

func (l *LocalFileReader) LookPath(name string) error {
	_, err := exec.LookPath(name)
	return err
}

// ExecCommand executes a program directly, without a shell. env entries with
// KEY=value override the inherited environment; a bare KEY removes it.
// Runner credentials and password environment variables are always stripped.
func (l *LocalFileReader) ExecCommand(name string, args, env []string, dst io.Writer) error {
	c := exec.Command(name, args...)
	c.Env = childEnv(env)
	var stderr bytes.Buffer
	c.Stderr = &stderr
	c.Stdout = dst
	if err := c.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return dumpDiagnosticError(stderr.String())
}

// ExecCommandContext streams stdout and stops the process group on cancellation.
func (l *LocalFileReader) ExecCommandContext(ctx context.Context, name string, args, env []string, dst io.Writer) error {
	c := exec.CommandContext(ctx, name, args...)
	configureStreamProcess(c)
	c.WaitDelay = 5 * time.Second
	c.Env = childEnv(env)
	stderr := &limitedDiagnostic{}
	c.Stderr = stderr
	c.Stdout = dst
	if err := c.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return dumpDiagnosticError(stderr.String())
}

// Keep diagnostic output bounded even when a dump tool repeatedly warns.
type limitedDiagnostic struct{ bytes.Buffer }

func (b *limitedDiagnostic) Write(p []byte) (int, error) {
	n := len(p)
	if remaining := 64*1024 - b.Len(); remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.Buffer.Write(p)
	}
	return n, nil
}

// childEnv preserves operational settings while preventing runner credentials
// from crossing the subprocess boundary. Removals are applied after overrides.
func childEnv(overrides []string) []string {
	env := mergeEnv(os.Environ(), overrides)
	return mergeEnv(env, []string{secrets.DatabasePasswordEnv, secrets.StorageAccessKeyEnv, secrets.StorageSecretKeyEnv, "PGPASSWORD", "MYSQL_PWD"})
}

func mergeEnv(base, overrides []string) []string {
	values := make(map[string]string, len(base)+len(overrides))
	order := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, exists := values[key]; !exists {
			order = append(order, key)
		}
		values[key] = entry
	}
	for _, entry := range overrides {
		key, _, hasValue := strings.Cut(entry, "=")
		if key == "" {
			continue
		}
		if !hasValue {
			delete(values, key)
			continue
		}
		if _, exists := values[key]; !exists {
			order = append(order, key)
		}
		values[key] = entry
	}
	out := make([]string, 0, len(values))
	for _, key := range order {
		if entry, ok := values[key]; ok {
			out = append(out, entry)
		}
	}
	return out
}

func (l *LocalFileReader) Close() error {
	return nil
}
