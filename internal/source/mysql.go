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

type MySQL struct{}

func (m *MySQL) Dump(job *job.Job, r reader.FileReader) (string, error) {
	if err := checkMysqldump(r); err != nil {
		return "", err
	}

	fileName := fmt.Sprintf("%s_%s_%s.sql", job.ID, job.Database.Name, time.Now().Format("20060102_150405"))
	outPath := filepath.Join(config.TmpPath, config.AppName, fileName)

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to create dump file: %w", err)
	}
	defer f.Close()

	cmd := fmt.Sprintf("MYSQL_PWD=%s mysqldump -h %s -P %s -u %s %s",
		job.Database.Password,
		job.Database.Host,
		job.Database.Port,
		job.Database.Username,
		job.Database.Name,
	)

	if err := r.ExecStream(cmd, f); err != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("mysqldump: %w", err)
	}

	return outPath, nil
}

// DumpRemote runs mysqldump on the remote host and writes the output to remotePath on that host.
func (m *MySQL) DumpRemote(j *job.Job, r reader.FileReader, remotePath string) error {
	if err := checkMysqldump(r); err != nil {
		return err
	}

	cmd := fmt.Sprintf(
		"mkdir -p '%s' && MYSQL_PWD='%s' mysqldump -h %s -P %s -u %s %s > '%s'",
		filepath.Dir(remotePath),
		j.Database.Password,
		j.Database.Host,
		j.Database.Port,
		j.Database.Username,
		j.Database.Name,
		remotePath,
	)

	if _, err := r.Exec(cmd); err != nil {
		r.Exec(fmt.Sprintf("rm -f '%s'", remotePath)) //nolint:errcheck
		return fmt.Errorf("mysqldump: %w", err)
	}
	return nil
}

func (m *MySQL) Test(job *job.Job, r reader.FileReader) error {
	if err := checkMysqldump(r); err != nil {
		return err
	}

	cmd := fmt.Sprintf("MYSQL_PWD=%s mysqladmin -h %s -P %s -u %s ping",
		job.Database.Password,
		job.Database.Host,
		job.Database.Port,
		job.Database.Username,
	)

	return r.ExecStream(cmd, io.Discard)
}

// checkMysqldump verifies mysqldump is available in whichever environment r targets.
func checkMysqldump(r reader.FileReader) error {
	_, err := r.Exec("which mysqldump")
	if err != nil {
		return fmt.Errorf("mysqldump not found on target host — install mysql-client")
	}
	return nil
}
