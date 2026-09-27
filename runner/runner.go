// Package runner exposes dbq's non-interactive backup execution API.
//
// Callers provide a complete runtime request, including secrets held in memory.
// The runner does not read job files, access the OS keyring, schedule work, or
// persist credentials. This initial public API executes database tools locally;
// SSH execution remains outside the public contract for now.
package runner

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/engine"
	"github.com/tofunmiadewuyi/dbq/internal/secrets"
	"github.com/tofunmiadewuyi/dbq/internal/storage"
)

type DatabaseType string

const (
	Postgres DatabaseType = "postgres"
	MySQL    DatabaseType = "mysql"
)

type StorageType string

const (
	Directory StorageType = "directory"
	Cloud     StorageType = "cloud"
)

type CloudProvider string

const (
	S3 CloudProvider = "s3"
	R2 CloudProvider = "r2"
)

type Database struct {
	Type     DatabaseType
	Host     string
	Port     int
	Name     string
	Username string
}

type Storage struct {
	Type      StorageType
	Provider  CloudProvider
	Endpoint  string
	Region    string
	Bucket    string
	Directory string
}

type Secrets struct {
	DatabasePassword string
	StorageAccessKey string
	StorageSecretKey string
}

type Request struct {
	ID        string
	Name      string
	Retention int
	Database  Database
	Storage   Storage
	Secrets   Secrets
}

type Result struct {
	StartedAt      time.Time
	Duration       time.Duration
	StorageKey     string
	Destination    string
	PrunedBackups  int
	RetentionError error
}

type Runner struct {
	engine *engine.Engine
}

func New() *Runner {
	return &Runner{engine: engine.New()}
}

func (r *Runner) Run(ctx context.Context, req Request) (Result, error) {
	if err := Validate(req); err != nil {
		return Result{}, err
	}
	result, err := r.engine.Run(ctx, engineRequest(req))
	return Result{
		StartedAt: result.StartedAt, Duration: result.Duration,
		StorageKey: result.StorageKey, Destination: result.Destination,
		PrunedBackups: result.PrunedBackups, RetentionError: result.RetentionError,
	}, err
}

func Validate(req Request) error {
	if strings.TrimSpace(req.ID) == "" || strings.TrimSpace(req.Name) == "" {
		return errors.New("backup id and name are required")
	}
	if req.Database.Type != Postgres && req.Database.Type != MySQL {
		return errors.New("database type must be postgres or mysql")
	}
	if strings.TrimSpace(req.Database.Host) == "" || strings.TrimSpace(req.Database.Name) == "" || strings.TrimSpace(req.Database.Username) == "" {
		return errors.New("database host, name, and username are required")
	}
	if req.Database.Port < 1 || req.Database.Port > 65535 {
		return errors.New("database port must be between 1 and 65535")
	}
	if req.Secrets.DatabasePassword == "" {
		return errors.New("database password is required")
	}
	if req.Retention < 0 {
		return errors.New("retention cannot be negative")
	}
	switch req.Storage.Type {
	case Directory:
		if strings.TrimSpace(req.Storage.Directory) == "" {
			return errors.New("storage directory is required")
		}
	case Cloud:
		if strings.TrimSpace(req.Storage.Bucket) == "" {
			return errors.New("storage bucket is required")
		}
		if req.Secrets.StorageAccessKey == "" || req.Secrets.StorageSecretKey == "" {
			return errors.New("storage credentials are required")
		}
		switch req.Storage.Provider {
		case S3:
			if strings.TrimSpace(req.Storage.Region) == "" {
				return errors.New("storage region is required for s3")
			}
		case R2:
			endpoint, err := url.Parse(req.Storage.Endpoint)
			if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
				return errors.New("storage endpoint must be a valid URL for r2")
			}
		default:
			return errors.New("storage provider must be s3 or r2")
		}
	default:
		return errors.New("storage type must be directory or cloud")
	}
	return nil
}

func engineRequest(req Request) engine.Request {
	databaseType := config.Postgres
	if req.Database.Type == MySQL {
		databaseType = config.MySQL
	}
	storageType := storage.TypeDirectory
	provider := storage.Provider("")
	if req.Storage.Type == Cloud {
		storageType = storage.TypeCloud
		provider = storage.TypeS3
		if req.Storage.Provider == R2 {
			provider = storage.TypeR2
		}
	}
	return engine.Request{
		ID: req.ID, Name: req.Name, Retention: req.Retention,
		Database: engine.Database{
			Type: databaseType, Host: req.Database.Host, Port: strconv.Itoa(req.Database.Port),
			Name: req.Database.Name, Username: req.Database.Username,
		},
		StorageType: storageType, Destination: req.Storage.Directory,
		CloudStorage: storage.CloudStorage{
			Provider: provider, Endpoint: req.Storage.Endpoint,
			Region: req.Storage.Region, Bucket: req.Storage.Bucket,
		},
		Secrets: secrets.JobSecrets{
			DatabasePassword: req.Secrets.DatabasePassword,
			StorageAccessKey: req.Secrets.StorageAccessKey,
			StorageSecretKey: req.Secrets.StorageSecretKey,
		},
	}
}
