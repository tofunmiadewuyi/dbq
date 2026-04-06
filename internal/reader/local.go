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

func (l *LocalFileReader) Close() error {
	return nil
}


