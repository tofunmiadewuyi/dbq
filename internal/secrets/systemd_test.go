package secrets

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSystemdRuntimeCredentialIsAuthoritative(t *testing.T) {
	dir := t.TempDir()
	p := &Systemd{dir: t.TempDir(), credentialsDir: dir}
	if err := os.WriteFile(filepath.Join(dir, "dbq-daily"), []byte(`{"DatabasePassword":"runtime-password"}`), 0o400); err != nil {
		t.Fatal(err)
	}
	values, err := p.Load("daily")
	if err != nil || values.DatabasePassword != "runtime-password" {
		t.Fatalf("runtime credential not loaded: %v", err)
	}
	// A missing injected bundle must fail rather than consult local ciphertext.
	values, err = p.Load("other")
	if !errors.Is(err, os.ErrNotExist) || values != (JobSecrets{}) {
		t.Fatalf("missing runtime credential did not fail closed: %v", err)
	}
	if err := os.Chmod(filepath.Join(dir, "dbq-daily"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dbq-daily"), []byte(`{"DatabasePassword":"secret-fragment"`), 0o400); err != nil {
		t.Fatal(err)
	}
	values, err = p.Load("daily")
	if err == nil || strings.Contains(err.Error(), "secret-fragment") || values != (JobSecrets{}) {
		t.Fatal("malformed runtime bundle must fail without exposing content")
	}
}

func TestSystemdFailedSavePreservesCiphertextAndRedactsToolOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell")
	}
	dir := t.TempDir()
	tool := filepath.Join(dir, "failing-creds")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\ncat >&2\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	p := &Systemd{dir: dir, binary: tool}
	path := filepath.Join(dir, "dbq-daily.cred")
	previous := []byte("existing-ciphertext")
	if err := os.WriteFile(path, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	err := p.Save("daily", JobSecrets{DatabasePassword: "do-not-expose"})
	if err == nil || strings.Contains(err.Error(), "do-not-expose") {
		t.Fatal("failed encryption must report an error without tool output")
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, previous) {
		t.Fatal("failed save replaced the existing credential")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatal("failed encryption left an extra credential file")
	}
}

func TestSystemdRejectsIDsOutsideStore(t *testing.T) {
	p := &Systemd{dir: t.TempDir()}
	for _, id := range []string{"../outside", "daily/other", "", "daily\nOther=setting"} {
		if _, err := p.Load(id); err == nil {
			t.Fatalf("Load accepted invalid ID %q", id)
		}
		if err := p.Save(id, JobSecrets{}); err == nil {
			t.Fatalf("Save accepted invalid ID %q", id)
		}
		if err := p.Delete(id); err == nil {
			t.Fatalf("Delete accepted invalid ID %q", id)
		}
	}
}

// Opt-in because the real tools may create a systemd host credential key.
// Run inside a disposable Linux environment with systemd-creds installed.
func TestSystemdStoreIntegration(t *testing.T) {
	if os.Getenv("DBQ_TEST_SYSTEMD") != "1" {
		t.Skip("set DBQ_TEST_SYSTEMD=1 in a disposable Linux environment")
	}
	dir := t.TempDir()
	p, err := NewSystemd(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := JobSecrets{DatabasePassword: "integration-db-password", StorageAccessKey: "integration-access", StorageSecretKey: "integration-secret"}
	if err := p.Save("daily", want); err != nil {
		t.Fatal(err)
	}
	got, err := p.Load("daily")
	if err != nil || got != want {
		t.Fatalf("credential round trip failed: %v", err)
	}
	name, path, _ := p.EncryptedCredential("daily")
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{want.DatabasePassword, want.StorageAccessKey, want.StorageSecretKey} {
		if bytes.Contains(ciphertext, []byte(secret)) {
			t.Fatal("plaintext found in persisted credential")
		}
	}
	for _, item := range []struct {
		path string
		mode os.FileMode
	}{{dir, 0o700}, {path, 0o600}} {
		info, err := os.Stat(item.path)
		if err != nil || info.Mode().Perm() != item.mode {
			t.Fatalf("incorrect credential permissions: %v", err)
		}
	}
	// A ciphertext copied to another job must fail name authentication.
	_, otherPath, _ := p.EncryptedCredential("other")
	if err := os.WriteFile(otherPath, ciphertext, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Load("other"); err == nil {
		t.Fatal("renamed ciphertext decrypted for another job")
	}
	// Exercise the same bundle through the scheduled runtime path.
	plain, err := p.crypt("decrypt", name, ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	runtimeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runtimeDir, name), plain, 0o400); err != nil {
		t.Fatal(err)
	}
	p.credentialsDir = runtimeDir
	got, err = p.Load("daily")
	if err != nil || got != want {
		t.Fatalf("runtime load failed: %v", err)
	}
	p.credentialsDir = ""
	want.DatabasePassword = "rotated-db-password"
	if err := p.Save("daily", want); err != nil {
		t.Fatal(err)
	}
	got, err = p.Load("daily")
	if err != nil || got != want {
		t.Fatalf("rotation failed: %v", err)
	}
	if err := os.WriteFile(path, []byte("corrupt-ciphertext"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Load("daily"); err == nil {
		t.Fatal("corrupt ciphertext accepted")
	}
	if err := p.Delete("daily"); err != nil {
		t.Fatal(err)
	}
	if err := p.Delete("daily"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("Delete left ciphertext behind")
	}
}
