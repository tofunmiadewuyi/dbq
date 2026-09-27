package runner

import (
	"strings"
	"testing"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/storage"
)

func validRequest() Request {
	return Request{
		ID: "job-1", Name: "nightly", Retention: 7,
		Database: Database{Type: Postgres, Host: "db.internal", Port: 5432, Name: "app", Username: "backup"},
		Storage:  Storage{Type: Directory, Directory: "/var/backups"},
		Secrets:  Secrets{DatabasePassword: "database-secret-value"},
	}
}

func TestValidateLocalDirectoryRequest(t *testing.T) {
	if err := Validate(validRequest()); err != nil {
		t.Fatal(err)
	}
}

func TestValidateNeverEchoesSecrets(t *testing.T) {
	req := validRequest()
	req.Database.Port = 0
	err := Validate(req)
	if err == nil {
		t.Fatal("invalid request accepted")
	}
	if strings.Contains(err.Error(), req.Secrets.DatabasePassword) {
		t.Fatal("validation error exposed secret")
	}
}

func TestEngineRequestMapsPublicRuntimeValues(t *testing.T) {
	req := validRequest()
	req.Database.Type = MySQL
	req.Storage = Storage{Type: Cloud, Provider: R2, Endpoint: "https://account.r2.cloudflarestorage.com", Bucket: "backups"}
	req.Secrets.StorageAccessKey = "access"
	req.Secrets.StorageSecretKey = "secret"

	got := engineRequest(req)
	if got.Database.Type != config.MySQL || got.Database.Port != "5432" {
		t.Fatalf("database mapping = %#v", got.Database)
	}
	if got.StorageType != storage.TypeCloud || got.CloudStorage.Provider != storage.TypeR2 || got.CloudStorage.Endpoint != req.Storage.Endpoint {
		t.Fatalf("storage mapping = %#v", got.CloudStorage)
	}
	if got.Secrets.DatabasePassword != req.Secrets.DatabasePassword || got.Secrets.StorageSecretKey != "secret" {
		t.Fatal("runtime secrets were not mapped")
	}
}
