package secrets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"time"
)

var jobIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// ValidateJobID ensures IDs remain a single safe filename and credential name.
func ValidateJobID(id string) error {
	if len(id) > 128 || !jobIDPattern.MatchString(id) {
		return fmt.Errorf("invalid job ID %q", id)
	}
	return nil
}

// Systemd stores authenticated ciphertext; plaintext only crosses subprocess
// pipes or is read from systemd's runtime credential directory.
type Systemd struct {
	dir            string
	binary         string
	user           bool
	credentialsDir string
}

var _ Store = (*Systemd)(nil)

func NewSystemd(dir string) (*Systemd, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("systemd secrets require Linux")
	}
	binary, err := exec.LookPath("systemd-creds")
	if err != nil {
		return nil, fmt.Errorf("systemd secrets require systemd-creds (systemd 250+, or 256+ for user services): %w", err)
	}
	p := &Systemd{dir: dir, binary: binary, user: os.Getuid() != 0, credentialsDir: os.Getenv("CREDENTIALS_DIRECTORY")}
	// A real round trip checks encryption, key access, and user-scope support.
	// Runtime services already have decrypted credentials and need no key access.
	if p.credentialsDir == "" {
		probe := []byte("dbq credential capability check")
		ciphertext, err := p.crypt("encrypt", "dbq-probe", probe)
		if err != nil {
			return nil, fmt.Errorf("systemd encryption unavailable; check systemd version and host key/TPM access: %w", err)
		}
		plain, err := p.crypt("decrypt", "dbq-probe", ciphertext)
		if err != nil || !bytes.Equal(plain, probe) {
			return nil, fmt.Errorf("systemd credential round-trip verification failed")
		}
	}
	return p, nil
}

// EncryptedCredential identifies the bundle to pass to a scheduled service.
func (p *Systemd) EncryptedCredential(id string) (name, path string, err error) {
	if err := ValidateJobID(id); err != nil {
		return "", "", err
	}
	name = "dbq-" + id
	return name, filepath.Join(p.dir, name+".cred"), nil
}

func (p *Systemd) crypt(operation, name string, input []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	args := []string{operation, "--name=" + name}
	if p.user {
		args = append(args, "--user")
	}
	if operation == "encrypt" {
		// auto fails if neither a suitable host key nor TPM is available. Never
		// use null/auto-initrd, which can provide no confidentiality.
		args = append(args, "--with-key=auto")
	}
	args = append(args, "-", "-")
	cmd := exec.CommandContext(ctx, p.binary, args...)
	cmd.Stdin = bytes.NewReader(input)
	output, err := cmd.Output()
	if err != nil {
		// Do not expose tool stderr or partial output: either may contain secrets.
		return nil, fmt.Errorf("systemd-creds %s failed: %w", operation, err)
	}
	return output, nil
}

func (p *Systemd) Load(id string) (JobSecrets, error) {
	name, path, err := p.EncryptedCredential(id)
	if err != nil {
		return JobSecrets{}, err
	}
	var plain []byte
	if p.credentialsDir != "" {
		// An explicitly supplied runtime credential is authoritative. A missing
		// or invalid bundle must fail, rather than try another source.
		plain, err = os.ReadFile(filepath.Join(p.credentialsDir, name))
	} else {
		var ciphertext []byte
		ciphertext, err = os.ReadFile(path)
		if os.IsNotExist(err) {
			return JobSecrets{}, nil
		}
		if err == nil {
			plain, err = p.crypt("decrypt", name, ciphertext)
		}
	}
	if err != nil {
		return JobSecrets{}, fmt.Errorf("load systemd credentials: %w", err)
	}
	var values JobSecrets
	if err := json.Unmarshal(plain, &values); err != nil {
		return JobSecrets{}, fmt.Errorf("invalid systemd credential bundle")
	}
	return values, nil
}

func (p *Systemd) Save(id string, values JobSecrets) error {
	name, path, err := p.EncryptedCredential(id)
	if err != nil {
		return err
	}
	plain, err := json.Marshal(values)
	if err != nil {
		return err
	}
	ciphertext, err := p.crypt("encrypt", name, plain)
	if err != nil {
		return err
	}
	verified, err := p.crypt("decrypt", name, ciphertext)
	if err != nil {
		return err
	}
	if !bytes.Equal(verified, plain) {
		return fmt.Errorf("systemd credential verification failed")
	}
	if err := os.MkdirAll(p.dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(p.dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(p.dir, ".credential-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(ciphertext); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	// Ensure the new directory entry is durable before migration may delete its
	// plaintext source after verification.
	dir, err := os.Open(p.dir)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (p *Systemd) Delete(id string) error {
	_, path, err := p.EncryptedCredential(id)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
