package reader

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestPooledSSHCommandTransmitsInputOutsideCommand(t *testing.T) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	commands := make(chan string, 1)
	serverErrors := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErrors <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		server, channels, requests, err := ssh.NewServerConn(conn, config)
		if err != nil {
			serverErrors <- err
			return
		}
		defer server.Close()
		go ssh.DiscardRequests(requests)
		for request := range channels {
			ch, requests, err := request.Accept()
			if err != nil {
				serverErrors <- err
				return
			}
			for request := range requests {
				if request.Type != "exec" {
					_ = request.Reply(false, nil)
					continue
				}
				var payload struct{ Command string }
				if err := ssh.Unmarshal(request.Payload, &payload); err != nil {
					serverErrors <- err
					return
				}
				commands <- payload.Command
				_ = request.Reply(true, nil)
				cmd := exec.Command("sh", "-c", payload.Command)
				cmd.Stdin = ch
				cmd.Stdout = ch
				cmd.Stderr = ch.Stderr()
				status := uint32(0)
				if err := cmd.Run(); err != nil {
					status = 1
				}
				_, err = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
				_ = ch.Close()
				serverErrors <- err
				return
			}
		}
	}()
	client, err := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{
		User: "test", HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	pool := &SSHConnectionPool{pool: make(chan *SSHFileReader, 1), poolSize: 1, currentCount: 1}
	pool.pool <- &SSHFileReader{sshClient: client}
	defer pool.Close()
	input := `url = "https://example.test/?X-Amz-Signature=synthetic-token"` + "\n"
	var out bytes.Buffer
	if err := pool.ExecRemoteCommandWithInput("cat", nil, nil, strings.NewReader(input), io.Writer(&out)); err != nil {
		t.Fatal(err)
	}
	if out.String() != input {
		t.Fatal("SSH stdin changed or was not transmitted")
	}
	if command := <-commands; strings.Contains(command, "synthetic-token") || strings.Contains(command, "https://") {
		t.Fatal("input embedded in SSH command")
	}
	if err := <-serverErrors; err != nil {
		t.Fatal(err)
	}
}
