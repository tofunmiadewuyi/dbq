// Package reader provides an interface for executing commands and reading files, either locally or over SSH.
package reader

import (
	"fmt"
	"io"
	"io/fs"

	"github.com/tofunmiadewuyi/dbq/internal/job"
)

// FileReader interface for reading files (local or remote)
type FileReader interface {
	ReadDir(path string) ([]fs.DirEntry, error)
	ReadFile(path string) ([]byte, error)
	Stat(path string) (fs.FileInfo, error)
	Exec(cmd string) ([]byte, error)
	// ExecStream runs cmd and writes stdout directly to dst in chunks,
	// avoiding buffering the entire output in memory.
	ExecStream(cmd string, dst io.Writer) error
	Close() error
}

// GetFileReader is a helper to decide which reader to use
func GetFileReader(ssh *job.SSHConn) (FileReader, error) {
	if ssh.Required {
		// use ssh
		if ssh.User == "" {
			return nil, fmt.Errorf("ssh-user required when connecting over ssh")
		}
		if ssh.Key == "" {
			return nil, fmt.Errorf("ssh-key required when using ssh-host")
		}

		return NewSSHConnectionPool(ssh.Host, ssh.User, ssh.Key, ssh.Port, false, 1)
	}

	// use local
	return &LocalFileReader{}, nil
}
