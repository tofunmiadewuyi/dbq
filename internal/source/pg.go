package source

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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

	remote, ok := r.(reader.RemoteFileCommandRunner)
	if !ok {
		return errors.New("reader does not support secure server-side command execution")
	}
	contents, err := pgpassLine(j)
	if err != nil {
		return err
	}
	credentialPath, err := remote.CreateCredentialFile("dbq-pgpass-", contents)
	if err != nil {
		return fmt.Errorf("create remote credential file: %w", err)
	}
	args := []string{"-Fc", "-w", "-h", j.Host, "-p", j.Port, "-U", j.Username, "-d", j.Name}
	runErr := remote.ExecRemoteCommandToFile("pg_dump", args, []string{"PGPASSWORD", "MYSQL_PWD", "PGPASSFILE=" + credentialPath}, remotePath)
	cleanupErr := remote.RemoveCredentialFile(credentialPath)
	if cleanupErr != nil {
		cleanupErr = fmt.Errorf("remove remote credential file: %w", cleanupErr)
	}
	if err := errors.Join(runErr, cleanupErr); err != nil {
		return fmt.Errorf("pg_dump: %w", err)
	}
	return nil
}

func (pg *Postgres) Test(j *SourceJob, r reader.FileReader) error {
	if err := checkPostgresTools(r); err != nil {
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
		return checkPostgresCompatibility(j, passfile, local.ExecCommand)
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
	runErr := checkPostgresCompatibility(j, passfile, remote.ExecRemoteCommand)
	cleanupErr := remote.RemoveCredentialFile(passfile)
	if runErr != nil {
		if cleanupErr != nil {
			return fmt.Errorf("%w (also failed to remove remote credential file: %v)", runErr, cleanupErr)
		}
		return runErr
	}
	if cleanupErr != nil {
		return fmt.Errorf("remove remote PostgreSQL credential file: %w", cleanupErr)
	}
	return nil
}

// PostgreSQL 10+ uses the first component as the major version; older
// releases use the first two. Compare the same numeric format the server uses.
var pgDumpVersionPattern = regexp.MustCompile(`^pg_dump \(PostgreSQL\) ([0-9]+)(?:\.([0-9]+))?`)

func checkPostgresCompatibility(j *SourceJob, passfile string, run func(string, []string, []string, io.Writer) error) error {
	var client bytes.Buffer
	if err := run("pg_dump", []string{"--version"}, nil, &client); err != nil {
		return fmt.Errorf("check pg_dump version: %w", err)
	}
	match := pgDumpVersionPattern.FindStringSubmatch(strings.TrimSpace(client.String()))
	if match == nil {
		return errors.New("could not determine pg_dump version")
	}
	major, err := strconv.Atoi(match[1])
	if err != nil || major < 1 {
		return errors.New("invalid pg_dump version")
	}
	clientMajor := major * 10000
	clientLabel := match[1]
	if major < 10 {
		minor, err := strconv.Atoi(match[2])
		if err != nil {
			return errors.New("invalid pg_dump version")
		}
		clientMajor += minor * 100
		clientLabel += "." + match[2]
	}
	var server bytes.Buffer
	if err := run("psql", postgresHealthArgs(j), postgresHealthEnv(passfile), &server); err != nil {
		return fmt.Errorf("check PostgreSQL server version: %w", err)
	}
	version, err := strconv.Atoi(strings.TrimSpace(server.String()))
	if err != nil || version < 10000 {
		return errors.New("could not determine PostgreSQL server version")
	}
	serverMajor := version / 10000 * 10000
	serverLabel := strconv.Itoa(version / 10000)
	if version < 100000 {
		serverMajor = version / 100 * 100
		serverLabel += "." + strconv.Itoa(version/100%100)
	}
	if clientMajor < serverMajor {
		return fmt.Errorf("pg_dump %s cannot back up PostgreSQL %s; install PostgreSQL client %s or newer on the backup host and ensure pg_dump in PATH uses it", clientLabel, serverLabel, serverLabel)
	}
	return nil
}

func postgresHealthArgs(j *SourceJob) []string {
	return []string{
		"--no-password", "--host", j.Host, "--port", j.Port,
		"--username", j.Username, "--dbname", j.Name,
		"--no-psqlrc", "--no-align", "--tuples-only", "--set", "ON_ERROR_STOP=1",
		"--command", "SHOW server_version_num",
	}
}

func postgresHealthEnv(passfile string) []string {
	return []string{
		"PGPASSWORD",
		"PGPASSFILE=" + passfile,
		"PGCONNECT_TIMEOUT=10",
		"PGOPTIONS=-c statement_timeout=10000",
	}
}

func checkPostgresTools(r reader.FileReader) error {
	if err := checkPgDump(r); err != nil {
		return err
	}
	if local, ok := r.(reader.CommandRunner); ok {
		if err := local.LookPath("psql"); err != nil {
			return fmt.Errorf("psql not found on target host — install postgresql-client")
		}
		return nil
	}
	if _, err := r.Exec("which psql"); err != nil {
		return fmt.Errorf("psql not found on target host — install postgresql-client")
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
