package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tofunmiadewuyi/dbq/internal/secrets"
)

func TestExecutionErrorsRedactKnownSecretsAndPreserveDiagnostics(t *testing.T) {
	req := cloudRequest()
	req.Secrets = secrets.JobSecrets{DatabasePassword: "db-secret", StorageAccessKey: "access-secret", StorageSecretKey: "storage-secret"}
	cause := errors.New("connection refused on db.example:5432; password=db-secret; access=access-secret; secret=storage-secret; https://store.example/backup?X-Amz-Signature=signed-value&X-Amz-Security-Token=token-value&partNumber=2; unknown-password=keep-me")
	driver := &fakeDriver{testErr: cause}
	client := &fakeStorage{testErr: cause, listErr: cause}
	e, _ := configuredEngine(t, driver, client)
	checks := []struct {
		name string
		run  func() error
	}{
		{"backup", func() error { _, err := e.Run(context.Background(), req); return err }},
		{"source test", func() error { _, err := e.TestSource(context.Background(), req); return err }},
		{"storage test", func() error { _, err := e.TestStorage(context.Background(), req); return err }},
		{"retention", func() error { _, err := e.Prune(context.Background(), req); return err }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			err := check.run()
			if err == nil {
				t.Fatal("expected failure")
			}
			for _, value := range []string{"db-secret", "access-secret", "storage-secret", "signed-value", "token-value"} {
				if strings.Contains(err.Error(), value) {
					t.Fatalf("known sensitive value remains: %s", value)
				}
			}
			for _, detail := range []string{"connection refused on db.example:5432", "partNumber=2", "unknown-password=keep-me"} {
				if !strings.Contains(err.Error(), detail) {
					t.Fatalf("diagnostic detail removed: %s", detail)
				}
			}
			if !errors.Is(err, cause) {
				t.Fatal("error identity lost")
			}
		})
	}
	driver.testErr = errors.New("authentication failed for backup user; consult pg_hba.conf")
	_, err := e.TestSource(context.Background(), req)
	if err.Error() != driver.testErr.Error() {
		t.Fatalf("unmatched diagnostic changed: %s", err)
	}
}
