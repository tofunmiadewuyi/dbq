package reader

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"
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
	return nil
}

func (l *LocalFileReader) LookPath(name string) error {
	_, err := exec.LookPath(name)
	return err
}

// ExecCommand executes a program directly, without a shell. env entries with
// KEY=value override the inherited environment; a bare KEY removes it.
func (l *LocalFileReader) ExecCommand(name string, args, env []string, dst io.Writer) error {
	c := exec.Command(name, args...)
	c.Env = mergeEnv(os.Environ(), env)
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
	return nil
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
