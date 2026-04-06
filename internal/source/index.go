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
	// DumpRemote runs pg_dump on the remote host writing output to remotePath on that host.
	// used when useserver=true to avoid streaming the dump over home internet.
	DumpRemote(job *job.Job, r reader.FileReader, remotePath string) error
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
