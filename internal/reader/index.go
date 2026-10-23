// Package reader provides an interface for executing commands and reading files, either locally or over SSH.
package reader

import (
	"context"
	"fmt"
	"io"
	"io/fs"
)

type SSHConn struct {
	Required  bool
	Port      int
	Host      string
	Key       string
	User      string
	UseServer bool
}

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

// CommandRunner executes a local program directly with an argument vector.
// Remote readers intentionally do not implement it because SSH exec requests
// are interpreted by a remote shell.
type CommandRunner interface {
	LookPath(name string) error
	ExecCommand(name string, args, env []string, dst io.Writer) error
}

// RemoteCommandRunner provides the secure primitives needed to run a streamed
// command over SSH. Implementations may use a remote shell internally, but
// must quote argv and environment values rather than accepting a command
// string from callers.
type RemoteCommandRunner interface {
	CreateCredentialFile(prefix, contents string) (string, error)
	RemoveCredentialFile(path string) error
	ExecRemoteCommand(name string, args, env []string, dst io.Writer) error
}

// RemoteInputCommandRunner sends input through the SSH channel, never embedding
// it in the remote shell command or argument vector.
type RemoteInputCommandRunner interface {
	RemoteCommandRunner
	ExecRemoteCommandWithInput(name string, args, env []string, src io.Reader, dst io.Writer) error
}

// RemoteFileCommandRunner writes stdout on the target without routing a dump
// through dbq. Implementations must quote paths and keep new files private.
type RemoteFileCommandRunner interface {
	RemoteCommandRunner
	ExecRemoteCommandToFile(name string, args, env []string, path string) error
}

// GetFileReader is a helper to decide which reader to use
func GetFileReader(ssh *SSHConn) (FileReader, error) {
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

// ContextCommandRunner supports cancellation of local streaming commands.
type ContextCommandRunner interface {
	ExecCommandContext(context.Context, string, []string, []string, io.Writer) error
}
