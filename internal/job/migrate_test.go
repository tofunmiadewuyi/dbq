package job

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/tofunmiadewuyi/dbq/internal/secrets"
)

type migrationStore struct {
	values   map[string]secrets.JobSecrets
	saves    int
	failSave int
	corrupt  bool
	onSave   func()
}

func (p *migrationStore) Load(id string) (secrets.JobSecrets, error) { return p.values[id], nil }
func (p *migrationStore) Save(id string, values secrets.JobSecrets) error {
	p.saves++
	if p.saves == p.failSave {
		return errors.New("store unavailable")
	}
	if p.corrupt {
		values.DatabasePassword = "verification-mismatch"
	}
	p.values[id] = values
	if p.onSave != nil {
		p.onSave()
	}
	return nil
}
func (p *migrationStore) Delete(id string) error { delete(p.values, id); return nil }

func migrationFixture(t *testing.T, legacy, config string) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, ".secrets")
	jobsDir := filepath.Join(dir, "jobs")
	if err := os.Mkdir(jobsDir, 0700); err != nil {
		t.Fatal(err)
	}
	if legacy != "" {
		if err := os.WriteFile(legacyPath, []byte(legacy), 0600); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(jobsDir, "daily.toml")
	if config != "" {
		if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return legacyPath, jobsDir, configPath
}

func TestMigrateSecretsCleansBothSourcesAndPreservesConfiguration(t *testing.T) {
	legacyPath, jobsDir, configPath := migrationFixture(t,
		`{"daily/db_password":"database-password","orphan/storage_sak":"orphan-secret"}`,
		`id = "daily"
name = "Daily"
frequency = "0 2 * * *"
retention = 7
custom = "preserved"
[database]
name = "app"
password = "database-password"
[database.ssh]
required = true
sshhost = "server"
[storage]
access_key = "access-key"
secret_key = "storage-secret"
bucket = "backups"
`)
	store := &migrationStore{values: map[string]secrets.JobSecrets{"daily": {StorageAccessKey: "access-key"}, "orphan": {DatabasePassword: "destination-only-password"}}}
	if err := migrateSecrets(store, legacyPath, jobsDir); err != nil {
		t.Fatal(err)
	}
	want := secrets.JobSecrets{DatabasePassword: "database-password", StorageAccessKey: "access-key", StorageSecretKey: "storage-secret"}
	if store.values["daily"] != want || store.values["orphan"].StorageSecretKey != "orphan-secret" || store.values["orphan"].DatabasePassword != "destination-only-password" {
		t.Fatal("migration lost credential material")
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatal("legacy plaintext file remains")
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"database-password", "access-key", "storage-secret"} {
		if bytes.Contains(data, []byte(secret)) {
			t.Fatal("cleaned config contains plaintext credentials")
		}
	}
	var config struct {
		ID, Name, Frequency, Custom string
		Retention                   int
		Database                    struct {
			Name string
			SSH  struct {
				Required bool
				Host     string `toml:"sshhost"`
			}
		}
		Storage struct{ Bucket string }
	}
	if _, err := toml.Decode(string(data), &config); err != nil {
		t.Fatal(err)
	}
	if config.ID != "daily" || config.Name != "Daily" || config.Frequency != "0 2 * * *" || config.Retention != 7 || config.Custom != "preserved" || config.Database.Name != "app" || !config.Database.SSH.Required || config.Database.SSH.Host != "server" || config.Storage.Bucket != "backups" {
		t.Fatal("migration changed non-secret configuration")
	}
	info, err := os.Stat(configPath)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("cleaned config is not private")
	}
	// A retry must neither need the removed sources nor overwrite the store.
	previousSaves := store.saves
	if err := migrateSecrets(store, legacyPath, jobsDir); err != nil || store.saves != previousSaves {
		t.Fatalf("migration retry failed: %v", err)
	}
}

func TestMigrateSecretsPreflightsAllSourcesAndDestinations(t *testing.T) {
	for _, tc := range []struct {
		name, legacy, config string
		destination          map[string]secrets.JobSecrets
	}{
		{"destination conflict", `{"aaa/db_password":"first","daily/db_password":"source-password"}`, `id = "daily"`, map[string]secrets.JobSecrets{"daily": {DatabasePassword: "destination-password"}}},
		{"source conflict", `{"daily/db_password":"source-password"}`, "id = \"daily\"\n[database]\npassword = \"other-password\"", nil},
		{"malformed config", `{"daily/db_password":"source-password"}`, "id = \"daily\"\n[database]\npassword = \"secret-fragment", nil},
		{"invalid field type", `{"daily/db_password":"source-password"}`, "id = \"daily\"\n[database]\npassword = 123", nil},
		{"unknown legacy key", `{"daily/unknown":"source-password"}`, `id = "daily"`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			legacyPath, jobsDir, configPath := migrationFixture(t, tc.legacy, tc.config)
			store := &migrationStore{values: tc.destination}
			if store.values == nil {
				store.values = make(map[string]secrets.JobSecrets)
			}
			err := migrateSecrets(store, legacyPath, jobsDir)
			if err == nil || store.saves != 0 {
				t.Fatal("migration did not fail before writing")
			}
			for _, secret := range []string{"source-password", "destination-password", "other-password", "secret-fragment"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatal("migration error exposed a credential")
				}
			}
			for path, want := range map[string]string{legacyPath: tc.legacy, configPath: tc.config} {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != want {
					t.Fatal("failed preflight modified plaintext source")
				}
			}
		})
	}
}

func TestMigrateSecretsPartialFailureRetainsSourcesAndCanRetry(t *testing.T) {
	legacy := `{"aaa/db_password":"first","daily/db_password":"second"}`
	legacyPath, jobsDir, _ := migrationFixture(t, legacy, "")
	store := &migrationStore{values: make(map[string]secrets.JobSecrets), failSave: 2}
	if err := migrateSecrets(store, legacyPath, jobsDir); err == nil {
		t.Fatal("expected partial write failure")
	}
	if store.values["aaa"].DatabasePassword != "first" {
		t.Fatal("test did not reach partial write")
	}
	got, err := os.ReadFile(legacyPath)
	if err != nil || string(got) != legacy {
		t.Fatal("partial migration removed source")
	}
	store.failSave = 0
	if err := migrateSecrets(store, legacyPath, jobsDir); err != nil {
		t.Fatal(err)
	}
	if store.values["daily"].DatabasePassword != "second" {
		t.Fatal("retry did not complete migration")
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatal("retry left plaintext source")
	}
}

func TestMigrateSecretsKeepsSourcesOnVerificationFailureOrConcurrentEdit(t *testing.T) {
	for _, scenario := range []string{"verification failure", "source changed"} {
		t.Run(scenario, func(t *testing.T) {
			legacy := `{"daily/db_password":"database-password"}`
			config := "id = \"daily\"\n[database]\npassword = \"database-password\""
			legacyPath, jobsDir, configPath := migrationFixture(t, legacy, config)
			store := &migrationStore{values: make(map[string]secrets.JobSecrets)}
			expectedConfig := config
			if scenario == "verification failure" {
				store.corrupt = true
			} else {
				expectedConfig = config + "\nname = \"concurrent-edit\"\n"
				store.onSave = func() {
					if err := os.WriteFile(configPath, []byte(expectedConfig), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := migrateSecrets(store, legacyPath, jobsDir); err == nil {
				t.Fatal("expected failure before cleanup")
			}
			for path, want := range map[string]string{legacyPath: legacy, configPath: expectedConfig} {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != want {
					t.Fatal("migration deleted or rewrote an unverified source")
				}
			}
		})
	}
}

func TestHydrateSecretsRequiresExplicitMigration(t *testing.T) {
	var legacy legacyJobCredentials
	legacy.Database.Password = "plaintext-password"
	store := &migrationStore{values: make(map[string]secrets.JobSecrets)}
	err := hydrateSecrets(&Job{ID: "daily"}, store, legacy)
	if err == nil || !strings.Contains(err.Error(), "dbq secrets migrate") || store.saves != 0 {
		t.Fatal("job load silently migrated credentials")
	}
}

func TestMigrateSecretsRejectsRuntimeOnlyCredentialContext(t *testing.T) {
	t.Setenv("CREDENTIALS_DIRECTORY", t.TempDir())
	store := &migrationStore{values: make(map[string]secrets.JobSecrets)}
	if err := MigrateSecrets(store); err == nil || store.saves != 0 {
		t.Fatal("runtime-only credentials accepted as migration destination")
	}
}

func TestMigrateSecretsRefusesSymlinkedPlaintextSources(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "plaintext-target")
	original := []byte(`{"daily/db_password":"source-password"}`)
	if err := os.WriteFile(target, original, 0600); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, ".secrets")
	if err := os.Symlink(target, source); err != nil {
		t.Fatal(err)
	}
	store := &migrationStore{values: make(map[string]secrets.JobSecrets)}
	if err := migrateSecrets(store, source, filepath.Join(dir, "jobs")); err == nil || store.saves != 0 {
		t.Fatal("symlinked plaintext source accepted")
	}
	if got, err := os.ReadFile(source); err != nil || !bytes.Equal(got, original) {
		t.Fatal("symlink source changed")
	}
}

func TestMigrateSecretsSystemdIntegration(t *testing.T) {
	if os.Getenv("DBQ_TEST_SYSTEMD") != "1" {
		t.Skip("requires disposable Linux systemd environment")
	}
	legacyPath, jobsDir, configPath := migrationFixture(t,
		`{"daily/db_password":"integration-database-password"}`,
		`id = "daily"
[storage]
access_key = "integration-access-key"
secret_key = "integration-secret-key"`)
	store, err := secrets.NewSystemd(filepath.Join(t.TempDir(), "credentials"))
	if err != nil {
		t.Fatal(err)
	}
	if err := migrateSecrets(store, legacyPath, jobsDir); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load("daily")
	want := secrets.JobSecrets{DatabasePassword: "integration-database-password", StorageAccessKey: "integration-access-key", StorageSecretKey: "integration-secret-key"}
	if err != nil || got != want {
		t.Fatalf("migration to real systemd store failed: %v", err)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatal("plaintext legacy file remains")
	}
	data, err := os.ReadFile(configPath)
	if err != nil || bytes.Contains(data, []byte("integration-")) {
		t.Fatal("plaintext config credentials remain")
	}
}

func TestMigrateSecretsCleanupFailureKeepsSourcesForRetry(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("requires directory permissions to deny writes")
	}
	legacy := `{"daily/db_password":"database-password"}`
	config := "id = \"daily\"\n[database]\npassword = \"database-password\""
	legacyPath, jobsDir, configPath := migrationFixture(t, legacy, config)
	defer os.Chmod(jobsDir, 0700)
	store := &migrationStore{values: make(map[string]secrets.JobSecrets)}
	store.onSave = func() {
		if err := os.Chmod(jobsDir, 0500); err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateSecrets(store, legacyPath, jobsDir); err == nil {
		t.Fatal("expected atomic config replacement to fail")
	}
	if store.values["daily"].DatabasePassword != "database-password" {
		t.Fatal("cleanup failure occurred before import")
	}
	for path, want := range map[string]string{legacyPath: legacy, configPath: config} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatal("cleanup failure modified plaintext source")
		}
	}
	if err := os.Chmod(jobsDir, 0700); err != nil {
		t.Fatal(err)
	}
	store.onSave = nil
	if err := migrateSecrets(store, legacyPath, jobsDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatal("cleanup retry left legacy plaintext")
	}
}
