package source

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/job"
	"github.com/tofunmiadewuyi/dbq/internal/reader"
)


type Postgres struct{}

func (pg *Postgres) Dump(job *job.Job, r reader.FileReader) (string, error) {
	if err := checkPgDump(r); err != nil {
		return "", err
	}

	fileName := fmt.Sprintf("%s_%s_%s.dump", job.ID, job.Database.Name, time.Now().Format("20060102_150405"))
	outPath := filepath.Join(config.TmpPath, config.AppName, fileName)

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to create dump file: %w", err)
	}
	defer f.Close()

	cmd := fmt.Sprintf("PGPASSWORD=%s pg_dump -Fc -h %s -p %s -U %s -d %s",
		job.Database.Password,
		job.Database.Host,
		job.Database.Port,
		job.Database.Username,
		job.Database.Name,
	)

	if err := r.ExecStream(cmd, f); err != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("pg_dump: %w", err)
	}

	return outPath, nil
}

// DumpRemote runs pg_dump on the remote host and writes the output to remotePath on that host.
// this avoids streaming the dump over the home internet connection.
func (pg *Postgres) DumpRemote(j *job.Job, r reader.FileReader, remotePath string) error {
	if err := checkPgDump(r); err != nil {
		return err
	}

	cmd := fmt.Sprintf(
		"mkdir -p '%s' && PGPASSWORD='%s' pg_dump -Fc -f '%s' -h %s -p %s -U %s -d %s",
		filepath.Dir(remotePath),
		j.Database.Password,
		remotePath,
		j.Database.Host,
		j.Database.Port,
		j.Database.Username,
		j.Database.Name,
	)

	if _, err := r.Exec(cmd); err != nil {
		r.Exec(fmt.Sprintf("rm -f '%s'", remotePath)) //nolint: errcheck
		return fmt.Errorf("pg_dump: %w", err)
	}
	return nil
}

func (pg *Postgres) Test(job *job.Job, r reader.FileReader) error {
	if err := checkPgDump(r); err != nil {
		return err
	}

	cmd := fmt.Sprintf("PGPASSWORD=%s pg_dump --schema-only -h %s -p %s -U %s -d %s",
		job.Database.Password,
		job.Database.Host,
		job.Database.Port,
		job.Database.Username,
		job.Database.Name,
	)

	return r.ExecStream(cmd, io.Discard)
}

// checkPgDump verifies pg_dump is available in whichever environment r targets.
func checkPgDump(r reader.FileReader) error {
	_, err := r.Exec("which pg_dump")
	if err != nil {
		return fmt.Errorf("pg_dump not found on target host — install postgresql-client")
	}
	return nil
}
