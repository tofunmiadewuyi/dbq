package engine

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Run the actual curl process with the same argv/stdin as the SSH transport.
// This exercises curl's config parser and HTTP upload, not a simulated upload.
type uploadProcess struct {
	args    []string
	invoked bool
}

func (*uploadProcess) CreateCredentialFile(string, string) (string, error) {
	return "", errors.New("unexpected credential file")
}
func (*uploadProcess) RemoveCredentialFile(string) error {
	return errors.New("unexpected credential cleanup")
}
func (r *uploadProcess) ExecRemoteCommand(name string, args, env []string, dst io.Writer) error {
	return r.ExecRemoteCommandWithInput(name, args, env, nil, dst)
}
func (r *uploadProcess) ExecRemoteCommandWithInput(name string, args, env []string, src io.Reader, dst io.Writer) error {
	r.invoked = true
	r.args = append([]string{name}, args...)
	cmd := exec.Command(name, args...)
	cmd.Stdin = src
	cmd.Stdout = dst
	cmd.Env = append(os.Environ(), env...)
	return cmd.Run()
}

func TestRemoteUploadUsesStdinAndPreservesSignedURL(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl unavailable")
	}
	type received struct{ method, uri, body string }
	requests := make(chan received, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests <- received{r.Method, r.RequestURI, string(body)}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "backup 'quoted'.gz")
	if err := os.WriteFile(path, []byte("synthetic backup bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	remote := &uploadProcess{}
	for _, suffix := range []string{
		`/backup?X-Amz-Signature=synthetic-token&partNumber=2`,
		// Quotes, backslashes, shell syntax and brackets must remain URL data.
		`/backup?X-Amz-Signature=synthetic-token&quote="&slash=\&shell=$(whoami)&list=[1]`,
	} {
		if err := uploadRemote(remote, path, server.URL+suffix); err != nil {
			t.Fatal(err)
		}
		got := <-requests
		if got.method != "PUT" || got.uri != suffix || got.body != "synthetic backup bytes" {
			t.Fatalf("upload changed: %#v", got)
		}
		argv := strings.Join(remote.args, " ")
		if strings.Contains(argv, server.URL) || strings.Contains(argv, "synthetic-token") {
			t.Fatal("URL exposed in argv")
		}
	}
	for _, bad := range []string{"\n", "\r", "\x00"} {
		remote.invoked = false
		if err := uploadRemote(remote, path, server.URL+bad+"output = leaked"); err == nil {
			t.Fatal("control character accepted")
		}
		if remote.invoked {
			t.Fatal("invalid config reached curl")
		}
	}
}
