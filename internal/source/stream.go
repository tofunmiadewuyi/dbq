package source

import (
	"context"
	"fmt"
	"io"

	"github.com/tofunmiadewuyi/dbq/internal/reader"
)

// StreamDriver writes a dump without creating a local dump file. Success means
// the dump process has exited successfully.
type StreamDriver interface {
	DumpStream(context.Context, *SourceJob, reader.ContextCommandRunner, io.Writer) error
}

func (m *MySQL) DumpStream(ctx context.Context, j *SourceJob, r reader.ContextCommandRunner, dst io.Writer) error {
	path, cleanup, err := writeCredentialFile("dbq-mysql-*", mysqlOptionFile(j.Password))
	if err != nil {
		return fmt.Errorf("create MySQL credential file: %w", err)
	}
	defer cleanup()
	args := []string{"--defaults-extra-file=" + path, "--routines", "--events", "--triggers", "-h", j.Host, "-P", j.Port, "-u", j.Username, j.Name}
	if err := r.ExecCommandContext(ctx, "mysqldump", args, []string{"MYSQL_PWD"}, dst); err != nil {
		return fmt.Errorf("mysqldump: %w", err)
	}
	return nil
}

func (pg *Postgres) DumpStream(ctx context.Context, j *SourceJob, r reader.ContextCommandRunner, dst io.Writer) error {
	line, err := pgpassLine(j)
	if err != nil {
		return err
	}
	path, cleanup, err := writeCredentialFile("dbq-pgpass-*", line)
	if err != nil {
		return fmt.Errorf("create PostgreSQL credential file: %w", err)
	}
	defer cleanup()
	args := []string{"-Fc", "-w", "-h", j.Host, "-p", j.Port, "-U", j.Username, "-d", j.Name}
	if err := r.ExecCommandContext(ctx, "pg_dump", args, []string{"PGPASSWORD", "PGPASSFILE=" + path}, dst); err != nil {
		return fmt.Errorf("pg_dump: %w", err)
	}
	return nil
}
