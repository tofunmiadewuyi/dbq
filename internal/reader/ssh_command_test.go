package reader

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoteCommandPreservesArgumentsWithoutInjection(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "injected")
	value := "value'; touch " + marker + "; printf '"
	command, err := remoteCommand("printf", []string{"%s", value}, []string{"PGPASSWORD"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("sh", "-c", command).Output()
	if err != nil {
		t.Fatalf("execute generated command: %v\ncommand: %s", err, command)
	}
	if string(out) != value {
		t.Fatalf("output = %q, want %q", out, value)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("shell injection marker exists: %v", err)
	}
}

func TestRemoteCommandQuotesEnvironmentValues(t *testing.T) {
	value := "a b'c;$HOME"
	command, err := remoteCommand("sh", []string{"-c", `printf %s "$SAFE_VALUE"`}, []string{"SAFE_VALUE=" + value})
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("sh", "-c", command).Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != value {
		t.Fatalf("output = %q, want %q", out, value)
	}
}

func TestRemoteCommandRejectsInvalidEnvironmentName(t *testing.T) {
	_, err := remoteCommand("true", nil, []string{"BAD-NAME=value"})
	if err == nil || !strings.Contains(err.Error(), "invalid environment name") {
		t.Fatalf("unexpected error: %v", err)
	}
}
