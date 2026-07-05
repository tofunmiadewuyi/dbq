package job

import (
	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/secrets"
	"github.com/tofunmiadewuyi/dbq/internal/storage"
)

type DB struct {
	Name     string              `toml:"name"`
	Type     config.DatabaseType `toml:"type"`
	Port     string        `toml:"port"`
	Host     string        `toml:"host"`
	Username string        `toml:"username"`
	Password string        `toml:"password"`
	SSH      SSHConn `toml:"ssh"`
}


type SSHConn struct {
	Required  bool   `toml:"required"`
	Port      int    `toml:"sshport"`
	Host      string `toml:"sshhost"`
	Key       string `toml:"sshkey"`
	User      string `toml:"sshuser"`
	UseServer bool   `toml:"useserver"`
}

type Job struct {
	Name        string       `toml:"name"`
	ID          string       `toml:"id"`
	StorageType storage.StorageType  `toml:"storage_type"`
	Destination string       `toml:"destination"` // path if directory
	Frequency   string       `toml:"frequency"`
	Retention   int          `toml:"retention"` // keep N most recent backups; 0 = unlimited
	Database    DB           `toml:"database"`
	Storage     storage.CloudStorage `toml:"storage"`
	sm          secrets.Manager
}

