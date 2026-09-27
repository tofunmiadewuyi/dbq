package source

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/reader"
)

type Postgres struct{}

func (pg *Postgres) Dump(j *SourceJob, r reader.FileReader) (string, error) {
	if err := checkPgDump(r); err != nil {
		return "", err
	}
	if local, ok := r.(reader.CommandRunner); ok {
		return pg.dumpLocal(j, local)
	}
	remote, ok := r.(reader.RemoteCommandRunner)
	if !ok {
		return "", errors.New("reader does not support secure remote command execution")
	}

	fileName := fmt.Sprintf("%s_%s_%s.dump", j.ID, j.Name, time.Now().Format("20060102_150405"))
	outPath := filepath.Join(config.TmpPath, config.AppName, fileName)
	f, err := createLocalDumpFile(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to create dump file: %w", err)
	}
	defer f.Close()

	line, err := pgpassLine(j)
	if err != nil {
		_ = os.Remove(outPath)
		return "", err
	}
	passfile, err := remote.CreateCredentialFile("dbq-pgpass-", line)
	if err != nil {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("create remote PostgreSQL credential file: %w", err)
	}
	args := []string{"-Fc", "-w", "-h", j.Host, "-p", j.Port, "-U", j.Username, "-d", j.Name}
	runErr := remote.ExecRemoteCommand("pg_dump", args, []string{"PGPASSWORD", "PGPASSFILE=" + passfile}, f)
	cleanupErr := remote.RemoveCredentialFile(passfile)
	if runErr != nil {
		_ = os.Remove(outPath)
		if cleanupErr != nil {
			return "", fmt.Errorf("pg_dump: %w (also failed to remove remote credential file: %v)", runErr, cleanupErr)
		}
		return "", fmt.Errorf("pg_dump: %w", runErr)
	}
	if cleanupErr != nil {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("remove remote PostgreSQL credential file: %w", cleanupErr)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("close PostgreSQL dump: %w", err)
	}
	return outPath, nil
}

func (pg *Postgres) dumpLocal(j *SourceJob, r reader.CommandRunner) (string, error) {
	fileName := fmt.Sprintf("%s_%s_%s.dump", j.ID, j.Name, time.Now().Format("20060102_150405"))
	outPath := filepath.Join(config.TmpPath, config.AppName, fileName)
	out, err := createLocalDumpFile(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to create dump file: %w", err)
	}
	defer out.Close()

	line, err := pgpassLine(j)
	if err != nil {
		_ = os.Remove(outPath)
		return "", err
	}
	passfile, cleanup, err := writeCredentialFile("dbq-pgpass-*", line)
	if err != nil {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("create PostgreSQL credential file: %w", err)
	}
	defer cleanup()

	args := []string{"-Fc", "-w", "-h", j.Host, "-p", j.Port, "-U", j.Username, "-d", j.Name}
	env := []string{"PGPASSWORD", "PGPASSFILE=" + passfile}
	if err := r.ExecCommand("pg_dump", args, env, out); err != nil {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("pg_dump: %w", err)
	}
	return outPath, nil
}

// DumpRemote runs pg_dump on the remote host and writes the output to remotePath on that host.
// this avoids streaming the dump over the home internet connection.
func (pg *Postgres) DumpRemote(j *SourceJob, r reader.FileReader, remotePath string) error {
	if err := checkPgDump(r); err != nil {
		return err
	}

	cmd := fmt.Sprintf(
		"mkdir -p '%s' && PGPASSWORD='%s' pg_dump -Fc -f '%s' -h %s -p %s -U %s -d %s",
		filepath.Dir(remotePath),
		j.Password, remotePath, j.Host, j.Port, j.Username, j.Name,
	)

	if _, err := r.Exec(cmd); err != nil {
		r.Exec(fmt.Sprintf("rm -f '%s'", remotePath)) //nolint:errcheck
		return fmt.Errorf("pg_dump: %w", err)
	}
	return nil
}

func (pg *Postgres) Test(j *SourceJob, r reader.FileReader) error {
	if err := checkPgDump(r); err != nil {
		return err
	}
	if local, ok := r.(reader.CommandRunner); ok {
		line, err := pgpassLine(j)
		if err != nil {
			return err
		}
		passfile, cleanup, err := writeCredentialFile("dbq-pgpass-*", line)
		if err != nil {
			return fmt.Errorf("create PostgreSQL credential file: %w", err)
		}
		defer cleanup()
		args := []string{"--schema-only", "-w", "-h", j.Host, "-p", j.Port, "-U", j.Username, "-d", j.Name}
		return local.ExecCommand("pg_dump", args, []string{"PGPASSWORD", "PGPASSFILE=" + passfile}, io.Discard)
	}
	remote, ok := r.(reader.RemoteCommandRunner)
	if !ok {
		return errors.New("reader does not support secure remote command execution")
	}
	line, err := pgpassLine(j)
	if err != nil {
		return err
	}
	passfile, err := remote.CreateCredentialFile("dbq-pgpass-", line)
	if err != nil {
		return fmt.Errorf("create remote PostgreSQL credential file: %w", err)
	}
	args := []string{"--schema-only", "-w", "-h", j.Host, "-p", j.Port, "-U", j.Username, "-d", j.Name}
	runErr := remote.ExecRemoteCommand("pg_dump", args, []string{"PGPASSWORD", "PGPASSFILE=" + passfile}, io.Discard)
	cleanupErr := remote.RemoveCredentialFile(passfile)
	if runErr != nil {
		if cleanupErr != nil {
			return fmt.Errorf("pg_dump: %w (also failed to remove remote credential file: %v)", runErr, cleanupErr)
		}
		return runErr
	}
	if cleanupErr != nil {
		return fmt.Errorf("remove remote PostgreSQL credential file: %w", cleanupErr)
	}
	return nil
}

// checkPgDump verifies pg_dump is available in whichever environment r targets.
func checkPgDump(r reader.FileReader) error {
	if local, ok := r.(reader.CommandRunner); ok {
		if err := local.LookPath("pg_dump"); err != nil {
			return fmt.Errorf("pg_dump not found on target host — install postgresql-client")
		}
		return nil
	}
	_, err := r.Exec("which pg_dump")
	if err != nil {
		return fmt.Errorf("pg_dump not found on target host — install postgresql-client")
	}
	return nil
}
