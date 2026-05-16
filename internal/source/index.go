// Package source defines the database driver interface and implementations for dumping databases.
package source

import (
	"fmt"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/reader"
)

type SourceJob struct {
	ID       string
	Name     string
	Host     string
	Port     string
	Username string
	Password string
}

type DBDriver interface {
	Dump(j *SourceJob, r reader.FileReader) (string, error)
	DumpRemote(j *SourceJob, r reader.FileReader, remotePath string) error
	Test(j *SourceJob, r reader.FileReader) error
}

func NewDBDriver(dbType config.DatabaseType) (DBDriver, error) {
	switch dbType {
	case config.Postgres:
		return &Postgres{}, nil
	case config.MySQL:
		return &MySQL{}, nil
	default:
		return nil, fmt.Errorf("unsupported database type: %v", dbType)
	}
}
