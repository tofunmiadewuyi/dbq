package reader

import (
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// dirEntry wraps fs.FileInfo to implement fs.DirEntry
type dirEntry struct {
	info fs.FileInfo
}

func (d *dirEntry) Name() string               { return d.info.Name() }
func (d *dirEntry) IsDir() bool                { return d.info.IsDir() }
func (d *dirEntry) Type() fs.FileMode          { return d.info.Mode().Type() }
func (d *dirEntry) Info() (fs.FileInfo, error) { return d.info, nil }

// NewSSHConnectionPool creates a pool of SSH connections
func NewSSHConnectionPool(host, user, keyPath string, port int, useSudo bool, poolSize int) (*SSHConnectionPool, error) {
	// read and parse private key once
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ssh key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ssh key: %w", err)
	}

	pool := &SSHConnectionPool{
		host:         host,
		user:         user,
		keyPath:      keyPath,
		port:         port,
		useSudo:      useSudo,
		pool:         make(chan *SSHFileReader, poolSize),
		poolSize:     poolSize,
		signer:       signer,
		currentCount: 1, // we create 1 connection upfront
	}

	// create just 1 connection upfront to test connectivity
	// remaining connections will be created on-demand
	conn, err := pool.createConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to create initial connection: %w", err)
	}
	pool.pool <- conn

	return pool, nil
}

func (p *SSHConnectionPool) createConnection() (*SSHFileReader, error) {
	config := &ssh.ClientConfig{
		User: p.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(p.signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%d", p.host, p.port)
	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial ssh: %w", err)
	}

	var sftpClient *sftp.Client
	if !p.useSudo {
		sftpClient, err = sftp.NewClient(sshClient)
		if err != nil {
			sshClient.Close()
			return nil, fmt.Errorf("failed to create sftp client: %w", err)
		}
	}

	return &SSHFileReader{
		sshClient:  sshClient,
		sftpClient: sftpClient,
		useSudo:    p.useSudo,
	}, nil
}

// get a connection from the pool
func (p *SSHConnectionPool) get() (*SSHFileReader, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, fmt.Errorf("connection pool is closed")
	}

	// try to get existing connection or create new one if under limit
	select {
	case conn := <-p.pool:
		p.mu.Unlock()
		return conn, nil
	default:
		// no available connection, create new one if under limit
		if p.currentCount < p.poolSize {
			p.currentCount++
			p.mu.Unlock()
			return p.createConnection()
		}
		p.mu.Unlock()
		// wait for available connection
		conn := <-p.pool
		return conn, nil
	}
}

// put a connection back to the pool
func (p *SSHConnectionPool) put(conn *SSHFileReader) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		conn.Close()
		return
	}
	p.mu.Unlock()

	select {
	case p.pool <- conn:
	default:
		conn.Close() // pool full or doesn't accept, just close it
	}

}

func (p *SSHConnectionPool) ReadDir(path string) ([]fs.DirEntry, error) {
	conn, err := p.get()
	if err != nil {
		return nil, err
	}
	defer p.put(conn)

	return conn.ReadDir(path)
}

func (p *SSHConnectionPool) ReadFile(path string) ([]byte, error) {
	conn, err := p.get()
	if err != nil {
		return nil, err
	}
	defer p.put(conn)

	return conn.ReadFile(path)
}

func (p *SSHConnectionPool) Stat(path string) (fs.FileInfo, error) {
	conn, err := p.get()
	if err != nil {
		return nil, err
	}
	defer p.put(conn)

	return conn.Stat(path)
}

func (p *SSHConnectionPool) ExecStream(cmd string, dst io.Writer) error {
	conn, err := p.get()
	if err != nil {
		return err
	}
	defer p.put(conn)
	return conn.ExecStream(cmd, dst)
}

func (p *SSHConnectionPool) Exec(cmd string) ([]byte, error) {
    conn, err := p.get()
    if err != nil {
        return nil, err
    }
    defer p.put(conn)
    return conn.Exec(cmd)
}

func (p *SSHConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true

	close(p.pool)
	for conn := range p.pool {
		conn.Close()
	}

	return nil
}
