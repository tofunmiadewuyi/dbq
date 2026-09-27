// Package agent implements dbq's one-request stdio execution contract.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/engine"
	"github.com/tofunmiadewuyi/dbq/internal/secrets"
	"github.com/tofunmiadewuyi/dbq/internal/storage"
)

const (
	ContractVersion = 1
	MaxRequestBytes = 1 << 20

	DatabasePasswordEnv = "DBQ_DB_PASSWORD"
	StorageAccessKeyEnv = "DBQ_STORAGE_ACCESS_KEY"
	StorageSecretKeyEnv = "DBQ_STORAGE_SECRET_KEY"
)

type Database struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

type Storage struct {
	Type      string `json:"type"`
	Provider  string `json:"provider,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
	Region    string `json:"region,omitempty"`
	Bucket    string `json:"bucket,omitempty"`
	Directory string `json:"directory,omitempty"`
}

type Request struct {
	Version   int      `json:"version"`
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Retention int      `json:"retention"`
	Database  Database `json:"database"`
	Storage   Storage  `json:"storage"`
}

type Result struct {
	Version        int       `json:"version"`
	Status         string    `json:"status"`
	StartedAt      time.Time `json:"started_at,omitempty"`
	FinishedAt     time.Time `json:"finished_at,omitempty"`
	StorageKey     string    `json:"storage_key,omitempty"`
	Destination    string    `json:"destination,omitempty"`
	PrunedBackups  int       `json:"pruned_backups"`
	RetentionError string    `json:"retention_error,omitempty"`
	Error          string    `json:"error,omitempty"`
}

type Executor interface {
	Run(context.Context, engine.Request) (engine.Result, error)
}

type GetenvFunc func(string) string

// Run reads one request, executes it, writes one result, and returns the process
// exit code: 0 success, 1 execution failure, 2 invalid request.
func Run(ctx context.Context, in io.Reader, out, diagnostic io.Writer, getenv GetenvFunc, executor Executor) int {
	req, err := decodeRequest(in)
	if err != nil {
		writeResult(out, Result{Version: ContractVersion, Status: "failed", Error: err.Error()}, diagnostic)
		return 2
	}
	engineReq, err := resolveRequest(req, getenv)
	if err != nil {
		writeResult(out, Result{Version: ContractVersion, Status: "failed", Error: err.Error()}, diagnostic)
		return 2
	}
	result, runErr := executor.Run(ctx, engineReq)
	response := Result{
		Version: ContractVersion, Status: "succeeded",
		StartedAt: result.StartedAt, FinishedAt: result.StartedAt.Add(result.Duration),
		StorageKey: result.StorageKey, Destination: result.Destination,
		PrunedBackups: result.PrunedBackups,
	}
	if result.RetentionError != nil {
		response.RetentionError = result.RetentionError.Error()
	}
	if runErr != nil {
		response.Status = "failed"
		response.Error = runErr.Error()
	}
	if err := writeResult(out, response, diagnostic); err != nil {
		return 1
	}
	if runErr != nil {
		return 1
	}
	return 0
}

func decodeRequest(in io.Reader) (Request, error) {
	data, err := io.ReadAll(io.LimitReader(in, MaxRequestBytes+1))
	if err != nil {
		return Request{}, errors.New("could not read request")
	}
	if len(data) > MaxRequestBytes {
		return Request{}, errors.New("request exceeds size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var req Request
	if err := decoder.Decode(&req); err != nil {
		return Request{}, fmt.Errorf("invalid request: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Request{}, errors.New("invalid request: expected one JSON document")
	}
	return req, nil
}

func resolveRequest(req Request, getenv GetenvFunc) (engine.Request, error) {
	if req.Version != ContractVersion {
		return engine.Request{}, fmt.Errorf("unsupported contract version %d", req.Version)
	}
	if strings.TrimSpace(req.ID) == "" || strings.TrimSpace(req.Name) == "" {
		return engine.Request{}, errors.New("backup id and name are required")
	}
	if req.Retention < 0 {
		return engine.Request{}, errors.New("retention cannot be negative")
	}
	databaseType := config.DatabaseType(req.Database.Type)
	if databaseType != config.Postgres && databaseType != config.MySQL {
		return engine.Request{}, errors.New("database type must be postgres or mysql")
	}
	if strings.TrimSpace(req.Database.Host) == "" || strings.TrimSpace(req.Database.Name) == "" || strings.TrimSpace(req.Database.Username) == "" {
		return engine.Request{}, errors.New("database host, name, and username are required")
	}
	if req.Database.Port < 1 || req.Database.Port > 65535 {
		return engine.Request{}, errors.New("database port must be between 1 and 65535")
	}
	databasePassword := getenv(DatabasePasswordEnv)
	if databasePassword == "" {
		return engine.Request{}, errors.New("database password is required")
	}

	storageType := storage.StorageType(req.Storage.Type)
	engineReq := engine.Request{
		ID: req.ID, Name: req.Name, Retention: req.Retention,
		Database: engine.Database{
			Type: databaseType, Host: req.Database.Host, Port: strconv.Itoa(req.Database.Port),
			Name: req.Database.Name, Username: req.Database.Username,
		},
		StorageType: storageType,
		Secrets:     secrets.JobSecrets{DatabasePassword: databasePassword},
	}
	switch storageType {
	case storage.TypeDirectory:
		if strings.TrimSpace(req.Storage.Directory) == "" {
			return engine.Request{}, errors.New("storage directory is required")
		}
		engineReq.Destination = req.Storage.Directory
	case storage.TypeCloud:
		if strings.TrimSpace(req.Storage.Bucket) == "" {
			return engine.Request{}, errors.New("storage bucket is required")
		}
		engineReq.Secrets.StorageAccessKey = getenv(StorageAccessKeyEnv)
		engineReq.Secrets.StorageSecretKey = getenv(StorageSecretKeyEnv)
		if engineReq.Secrets.StorageAccessKey == "" || engineReq.Secrets.StorageSecretKey == "" {
			return engine.Request{}, errors.New("storage credentials are required")
		}
		switch req.Storage.Provider {
		case "s3":
			if strings.TrimSpace(req.Storage.Region) == "" {
				return engine.Request{}, errors.New("storage region is required for s3")
			}
			engineReq.CloudStorage.Provider = storage.TypeS3
		case "r2":
			endpoint, err := url.Parse(req.Storage.Endpoint)
			if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
				return engine.Request{}, errors.New("storage endpoint must be a valid URL for r2")
			}
			engineReq.CloudStorage.Provider = storage.TypeR2
		default:
			return engine.Request{}, errors.New("storage provider must be s3 or r2")
		}
		engineReq.CloudStorage.Endpoint = req.Storage.Endpoint
		engineReq.CloudStorage.Region = req.Storage.Region
		engineReq.CloudStorage.Bucket = req.Storage.Bucket
	default:
		return engine.Request{}, errors.New("storage type must be directory or cloud")
	}
	return engineReq, nil
}

func writeResult(out io.Writer, result Result, diagnostic io.Writer) error {
	if err := json.NewEncoder(out).Encode(result); err != nil {
		fmt.Fprintln(diagnostic, "could not write agent result")
		return err
	}
	return nil
}
