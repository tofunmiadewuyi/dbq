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

func TestRemoteFileCommandWritesPrivateOutputWithoutInjection(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "injected")
	outputPath := filepath.Join(dir, "dump'; touch injected; '")
	value := "database'; touch " + marker + "; printf '"
	command, err := remoteFileCommand("printf", []string{"%s", value}, nil, outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("sh", "-c", command).CombinedOutput(); err != nil {
		t.Fatalf("generated command failed: %v: %s", err, output)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil || string(data) != value {
		t.Fatal("remote file command changed output")
	}
	info, err := os.Stat(outputPath)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("remote output is not private")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("argument executed shell syntax")
	}
	// Existing backups must not be truncated or removed by a failed invocation.
	if err := exec.Command("sh", "-c", command).Run(); err == nil {
		t.Fatal("existing backup overwritten")
	}
	data, err = os.ReadFile(outputPath)
	if err != nil || string(data) != value {
		t.Fatal("existing backup changed")
	}
	failedPath := filepath.Join(dir, "failed.dump")
	command, err = remoteFileCommand("sh", []string{"-c", "printf partial; exit 7"}, nil, failedPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("sh", "-c", command).Run(); err == nil {
		t.Fatal("dump failure ignored")
	}
	if _, err := os.Stat(failedPath); !os.IsNotExist(err) {
		t.Fatal("partial dump not removed")
	}
}
