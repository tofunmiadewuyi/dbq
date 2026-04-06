// Package source defines the database driver interface and implementations for dumping databases.
package source

import (
	"fmt"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/job"
	"github.com/tofunmiadewuyi/dbq/internal/reader"
)

type DBDriver interface {
	Dump(job *job.Job, r reader.FileReader) (string, error)
	Test(job *job.Job, r reader.FileReader) error
}

func NewDBDriver(db *job.DB) (DBDriver, error) {

	switch db.Type {
	case config.Postgres:
		return &Postgres{}, nil
	case config.MySQL:
		return nil, fmt.Errorf("unsupported database type: %v", db)
	default:
		return nil, fmt.Errorf("unsupported database type: %v", db)
	}
}
