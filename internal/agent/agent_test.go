package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/engine"
)

type fakeExecutor struct {
	request engine.Request
	result  engine.Result
	err     error
	called  bool
	health  engine.HealthResult
}

func (e *fakeExecutor) Run(_ context.Context, req engine.Request) (engine.Result, error) {
	e.called = true
	e.request = req
	return e.result, e.err
}

func (e *fakeExecutor) Health(_ context.Context, req engine.Request) engine.HealthResult {
	e.called = true
	e.request = req
	return e.health
}

func requestJSON(t *testing.T, req Request) string {
	t.Helper()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func validRequest() Request {
	return Request{
		Version: ContractVersion, ID: "job-1", Name: "nightly", Retention: 7,
		Database: Database{Type: "postgres", Host: "db.internal", Port: 5432, Name: "app", Username: "backup"},
		Storage:  Storage{Type: "directory", Directory: "/var/backups"},
	}
}

func env(values map[string]string) GetenvFunc {
	return func(key string) string { return values[key] }
}

func decodeResult(t *testing.T, out *bytes.Buffer) Result {
	t.Helper()
	var result Result
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, out.String())
	}
	return result
}

func TestRunExecutesOneRequestAndWritesOneResult(t *testing.T) {
	started := time.Date(2026, 9, 28, 2, 0, 0, 0, time.UTC)
	executor := &fakeExecutor{result: engine.Result{
		StartedAt: started, Duration: 8 * time.Second, Destination: "/var/backups/nightly.dump", PrunedBackups: 1,
	}}
	var out, diagnostic bytes.Buffer
	code := Run(context.Background(), strings.NewReader(requestJSON(t, validRequest())), &out, &diagnostic,
		env(map[string]string{DatabasePasswordEnv: "database-secret"}), executor)
	if code != 0 || !executor.called {
		t.Fatalf("code=%d called=%v diagnostic=%q", code, executor.called, diagnostic.String())
	}
	result := decodeResult(t, &out)
	if result.Status != "succeeded" || result.FinishedAt.Sub(result.StartedAt) != 8*time.Second || result.PrunedBackups != 1 {
		t.Fatalf("result = %#v", result)
	}
	if executor.request.Secrets.DatabasePassword != "database-secret" || executor.request.Destination != "/var/backups" {
		t.Fatal("runtime request was not resolved from stdin and environment")
	}
	if strings.Contains(out.String(), "database-secret") {
		t.Fatal("stdout exposed a secret")
	}
}

func TestRunReportsExecutionFailureAsJSON(t *testing.T) {
	executor := &fakeExecutor{result: engine.Result{StartedAt: time.Now(), Duration: time.Second}, err: errors.New("dump failed")}
	var out bytes.Buffer
	code := Run(context.Background(), strings.NewReader(requestJSON(t, validRequest())), &out, &bytes.Buffer{},
		env(map[string]string{DatabasePasswordEnv: "database-secret"}), executor)
	result := decodeResult(t, &out)
	if code != 1 || result.Status != "failed" || result.Error != "dump failed" {
		t.Fatalf("code=%d result=%#v", code, result)
	}
}

func TestHealthOperationReturnsComponentResults(t *testing.T) {
	req := validRequest()
	req.Operation = "health"
	executor := &fakeExecutor{health: engine.HealthResult{Checks: []engine.HealthCheck{
		{Name: "database", Duration: 12 * time.Millisecond},
		{Name: "storage", Duration: 3 * time.Millisecond, Error: errors.New("permission denied")},
	}}}
	var out bytes.Buffer
	code := Run(context.Background(), strings.NewReader(requestJSON(t, req)), &out, &bytes.Buffer{},
		env(map[string]string{DatabasePasswordEnv: "database-secret"}), executor)
	result := decodeResult(t, &out)
	if code != 1 || result.Status != "failed" || len(result.Checks) != 2 {
		t.Fatalf("code=%d result=%#v", code, result)
	}
	if result.Checks[0].Status != "succeeded" || result.Checks[1].Status != "failed" {
		t.Fatalf("checks = %#v", result.Checks)
	}
}

func TestRunRejectsInvalidOrMultipleDocumentsWithoutExecution(t *testing.T) {
	for _, input := range []string{
		`{"version":2}`,
		requestJSON(t, validRequest()) + ` {}`,
		`{"version":1,"unknown":true}`,
	} {
		executor := &fakeExecutor{}
		var out bytes.Buffer
		code := Run(context.Background(), strings.NewReader(input), &out, &bytes.Buffer{}, env(nil), executor)
		result := decodeResult(t, &out)
		if code != 2 || executor.called || result.Status != "failed" || result.Error == "" {
			t.Fatalf("input=%q code=%d called=%v result=%#v", input, code, executor.called, result)
		}
	}
}

func TestRunRequiresCloudSecretsWithoutEchoingThem(t *testing.T) {
	req := validRequest()
	req.Storage = Storage{Type: "cloud", Provider: "s3", Region: "eu-west-1", Bucket: "backups"}
	var out bytes.Buffer
	code := Run(context.Background(), strings.NewReader(requestJSON(t, req)), &out, &bytes.Buffer{},
		env(map[string]string{DatabasePasswordEnv: "database-secret", StorageAccessKeyEnv: "access-secret"}), &fakeExecutor{})
	result := decodeResult(t, &out)
	if code != 2 || !strings.Contains(result.Error, "storage credentials") {
		t.Fatalf("code=%d result=%#v", code, result)
	}
	for _, secret := range []string{"database-secret", "access-secret"} {
		if strings.Contains(out.String(), secret) {
			t.Fatalf("stdout exposed %q", secret)
		}
	}
}
