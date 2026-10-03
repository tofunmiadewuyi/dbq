package scheduler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemdServiceCredentialBinding(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daily.cred")
	if err := os.WriteFile(path, []byte("ciphertext"), 0o600); err != nil {
		t.Fatal(err)
	}
	j := &SchedulerJob{ID: "daily", Name: "Daily", CredentialName: "dbq-daily", EncryptedCredentialPath: path}
	unit, err := serviceContent(j, "/opt/dbq tools/dbq")
	if err != nil {
		t.Fatal(err)
	}
	for _, directive := range []string{
		"Environment=DBQ_SECRETS_PROVIDER=systemd\n",
		`LoadCredentialEncrypted="dbq-daily:` + path + `"` + "\n",
		`ExecStart="/opt/dbq tools/dbq" run "daily"` + "\n",
	} {
		if !strings.Contains(unit, directive) {
			t.Fatalf("missing service directive %q", directive)
		}
	}
	j.CredentialName = "dbq-other"
	if _, err := serviceContent(j, "/bin/dbq"); err == nil {
		t.Fatal("cross-job binding accepted")
	}
	j.CredentialName = "dbq-daily"
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := serviceContent(j, "/bin/dbq"); err == nil {
		t.Fatal("missing encrypted credential accepted")
	}
}

func TestSystemdUnitIntegration(t *testing.T) {
	if os.Getenv("DBQ_TEST_SYSTEMD") != "1" {
		t.Skip("requires disposable Linux systemd environment")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "daily credential.cred")
	if err := os.WriteFile(path, []byte("ciphertext"), 0o600); err != nil {
		t.Fatal(err)
	}
	unit, err := serviceContent(&SchedulerJob{ID: "daily", Name: "Daily 100%", CredentialName: "dbq-daily", EncryptedCredentialPath: path}, "/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	unitPath := filepath.Join(dir, "dbq-daily.service")
	if err := os.WriteFile(unitPath, []byte(unit), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("systemd-analyze", "verify", unitPath).CombinedOutput(); err != nil {
		t.Fatalf("systemd rejected generated unit: %v\n%s", err, output)
	}
}
