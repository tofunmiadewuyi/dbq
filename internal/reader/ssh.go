package reader

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"sync"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHFileReader reads from remote filesystem via SSH/SFTP
type SSHFileReader struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	useSudo    bool
}

// SSHConnectionPool manages multiple SSH connections for concurrent access
type SSHConnectionPool struct {
	host         string
	user         string
	keyPath      string
	port         int
	useSudo      bool
	pool         chan *SSHFileReader
	poolSize     int
	signer       ssh.Signer
	mu           sync.Mutex
	closed       bool
	currentCount int
}

func NewSSHFileReader(host, user, keyPath string, port int, useSudo bool) (*SSHFileReader, error) {
	// read private key
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ssh key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ssh key: %w", err)
	}

	// ssh client config
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: proper host key verification
	}

	// connect to ssh
	addr := fmt.Sprintf("%s:%d", host, port)
	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial ssh: %w", err)
	}

	// create sftp client (won't work with sudo, will use ssh commands instead)
	var sftpClient *sftp.Client
	if !useSudo {
		sftpClient, err = sftp.NewClient(sshClient)
		if err != nil {
			sshClient.Close()
			return nil, fmt.Errorf("failed to create sftp client: %w", err)
		}
	}

	return &SSHFileReader{
		sshClient:  sshClient,
		sftpClient: sftpClient,
		useSudo:    useSudo,
	}, nil
}

func (s *SSHFileReader) ReadDir(path string) ([]fs.DirEntry, error) {
	if s.useSudo {
		return s.readDirWithSudo(path)
	}

	entries, err := s.sftpClient.ReadDir(path)
	if err != nil {
		return nil, err
	}

	// convert []fs.FileInfo to []fs.DirEntry
	result := make([]fs.DirEntry, len(entries))
	for i, entry := range entries {
		result[i] = &dirEntry{entry}
	}
	return result, nil
}

func (s *SSHFileReader) ReadFile(path string) ([]byte, error) {
	if s.useSudo {
		return s.readFileWithSudo(path)
	}

	file, err := s.sftpClient.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return io.ReadAll(file)
}

func (s *SSHFileReader) Stat(path string) (fs.FileInfo, error) {
	if s.useSudo {
		return s.statWithSudo(path)
	}
	return s.sftpClient.Stat(path)
}

func (s *SSHFileReader) ExecStream(cmd string, dst io.Writer) error {
	session, err := s.sshClient.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	var stderr bytes.Buffer
	session.Stderr = &stderr
	session.Stdout = dst
	if err := session.Run(cmd); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

func (s *SSHFileReader) Exec(cmd string) ([]byte, error) {
	session, err := s.sshClient.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	var stderr bytes.Buffer
	session.Stderr = &stderr
	out, err := session.Output(cmd)
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}
	return out, nil
}

// sudo helpers using ssh exec
func (s *SSHFileReader) readFileWithSudo(path string) ([]byte, error) {
	session, err := s.sshClient.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	cmd := fmt.Sprintf("sudo cat %q", path)
	return session.Output(cmd)
}

func (s *SSHFileReader) readDirWithSudo(path string) ([]fs.DirEntry, error) {
	session, err := s.sshClient.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	// use ls with specific format
	cmd := fmt.Sprintf("sudo ls -1 %q", path)
	output, err := session.Output(cmd)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	entries := make([]fs.DirEntry, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		// create basic entry (we don't have full stat info without extra calls)
		entries = append(entries, &basicDirEntry{name: line})
	}
	return entries, nil
}

func (s *SSHFileReader) statWithSudo(path string) (fs.FileInfo, error) {
	// not needed for our use case, return nil
	return nil, fmt.Errorf("stat not implemented for sudo mode")
}

// basicDirEntry for sudo ls output
type basicDirEntry struct {
	name string
}

func (b *basicDirEntry) Name() string               { return b.name }
func (b *basicDirEntry) IsDir() bool                { return false } // assume files
func (b *basicDirEntry) Type() fs.FileMode          { return 0 }
func (b *basicDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

func (s *SSHFileReader) Close() error {
	if s.sftpClient != nil {
		s.sftpClient.Close()
	}
	if s.sshClient != nil {
		s.sshClient.Close()
	}
	return nil
}


