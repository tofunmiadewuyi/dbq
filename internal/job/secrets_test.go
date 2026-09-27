package job

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tofunmiadewuyi/dbq/internal/secrets"
)

type stubSecretProvider struct {
	values secrets.JobSecrets
}

func (p *stubSecretProvider) Load(string) (secrets.JobSecrets, error) {
	return p.values, nil
}

func (p *stubSecretProvider) Save(_ string, values secrets.JobSecrets) error {
	p.values = values
	return nil
}

func (p *stubSecretProvider) Delete(string) error { return nil }

func TestJobTemplateNeverSerializesSecrets(t *testing.T) {
	j := Job{
		Name: "daily",
		ID:   "daily",
		Secrets: secrets.JobSecrets{
			DatabasePassword: "database-secret-value",
			StorageAccessKey: "storage-access-value",
			StorageSecretKey: "storage-secret-value",
		},
	}

	var out bytes.Buffer
	if err := jobTemplate.Execute(&out, j); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{
		j.Secrets.DatabasePassword,
		j.Secrets.StorageAccessKey,
		j.Secrets.StorageSecretKey,
	} {
		if strings.Contains(out.String(), secret) {
			t.Fatalf("serialized secret value %q", secret)
		}
	}
}

func TestHydrateSecretsLoadsSeparateBundle(t *testing.T) {
	want := secrets.JobSecrets{
		DatabasePassword: "database-password",
		StorageAccessKey: "access-key",
		StorageSecretKey: "secret-key",
	}
	provider := &stubSecretProvider{values: want}
	j := Job{ID: "daily"}

	if err := hydrateSecrets(&j, provider, legacyJobCredentials{}); err != nil {
		t.Fatal(err)
	}
	if j.Secrets != want {
		t.Fatalf("loaded secrets = %#v, want %#v", j.Secrets, want)
	}
	if j.provider != provider {
		t.Fatal("job did not retain its secret provider")
	}
}

func TestEngineRequestIsCompleteRuntimeSnapshot(t *testing.T) {
	j := Job{
		ID:          "daily",
		Name:        "Daily Backup",
		Destination: "/backups",
		Retention:   7,
		Secrets: secrets.JobSecrets{
			DatabasePassword: "database-password",
			StorageAccessKey: "access-key",
			StorageSecretKey: "secret-key",
		},
	}
	j.Database.Name = "app"
	j.Database.Host = "db.internal"
	j.Database.SSH.Required = true
	j.Database.SSH.Host = "server.internal"

	req := j.EngineRequest()
	if req.ID != j.ID || req.Name != j.Name || req.Database.Name != j.Database.Name || req.Database.Host != j.Database.Host {
		t.Fatalf("request did not preserve job identity/database: %#v", req)
	}
	if !req.SSH.Required || req.SSH.Host != j.Database.SSH.Host {
		t.Fatalf("request did not preserve SSH config: %#v", req.SSH)
	}
	if req.Secrets != j.Secrets || req.Retention != j.Retention || req.Destination != j.Destination {
		t.Fatalf("request did not preserve runtime values: %#v", req)
	}
}
