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

type MySQL struct{}

func (m *MySQL) Dump(j *SourceJob, r reader.FileReader) (string, error) {
	if err := checkMysqldump(r); err != nil {
		return "", err
	}
	if local, ok := r.(reader.CommandRunner); ok {
		return m.dumpLocal(j, local)
	}
	remote, ok := r.(reader.RemoteCommandRunner)
	if !ok {
		return "", errors.New("reader does not support secure remote command execution")
	}

	fileName := fmt.Sprintf("%s_%s_%s.sql", j.ID, j.Name, time.Now().Format("20060102_150405"))
	outPath := filepath.Join(config.TmpPath, config.AppName, fileName)
	f, err := createLocalDumpFile(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to create dump file: %w", err)
	}
	defer f.Close()

	defaultsFile, err := remote.CreateCredentialFile("dbq-mysql-", mysqlOptionFile(j.Password))
	if err != nil {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("create remote MySQL credential file: %w", err)
	}
	args := []string{"--defaults-extra-file=" + defaultsFile, "-h", j.Host, "-P", j.Port, "-u", j.Username, j.Name}
	runErr := remote.ExecRemoteCommand("mysqldump", args, []string{"MYSQL_PWD"}, f)
	cleanupErr := remote.RemoveCredentialFile(defaultsFile)
	if runErr != nil {
		_ = os.Remove(outPath)
		if cleanupErr != nil {
			return "", fmt.Errorf("mysqldump: %w (also failed to remove remote credential file: %v)", runErr, cleanupErr)
		}
		return "", fmt.Errorf("mysqldump: %w", runErr)
	}
	if cleanupErr != nil {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("remove remote MySQL credential file: %w", cleanupErr)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("close MySQL dump: %w", err)
	}
	return outPath, nil
}

func (m *MySQL) dumpLocal(j *SourceJob, r reader.CommandRunner) (string, error) {
	fileName := fmt.Sprintf("%s_%s_%s.sql", j.ID, j.Name, time.Now().Format("20060102_150405"))
	outPath := filepath.Join(config.TmpPath, config.AppName, fileName)
	out, err := createLocalDumpFile(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to create dump file: %w", err)
	}
	defer out.Close()

	defaultsFile, cleanup, err := writeCredentialFile("dbq-mysql-*", mysqlOptionFile(j.Password))
	if err != nil {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("create MySQL credential file: %w", err)
	}
	defer cleanup()

	args := []string{"--defaults-extra-file=" + defaultsFile, "-h", j.Host, "-P", j.Port, "-u", j.Username, j.Name}
	if err := r.ExecCommand("mysqldump", args, []string{"MYSQL_PWD"}, out); err != nil {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("mysqldump: %w", err)
	}
	return outPath, nil
}

// DumpRemote runs mysqldump on the remote host and writes the output to remotePath on that host.
func (m *MySQL) DumpRemote(j *SourceJob, r reader.FileReader, remotePath string) error {
	if err := checkMysqldump(r); err != nil {
		return err
	}

	cmd := fmt.Sprintf(
		"mkdir -p '%s' && MYSQL_PWD='%s' mysqldump -h %s -P %s -u %s %s > '%s'",
		filepath.Dir(remotePath),
		j.Password, j.Host, j.Port, j.Username, j.Name, remotePath,
	)

	if _, err := r.Exec(cmd); err != nil {
		r.Exec(fmt.Sprintf("rm -f '%s'", remotePath)) //nolint:errcheck
		return fmt.Errorf("mysqldump: %w", err)
	}
	return nil
}

func (m *MySQL) Test(j *SourceJob, r reader.FileReader) error {
	if err := checkMysqldump(r); err != nil {
		return err
	}
	if local, ok := r.(reader.CommandRunner); ok {
		if err := local.LookPath("mysqladmin"); err != nil {
			return fmt.Errorf("mysqladmin not found on target host — install mysql-client")
		}
		defaultsFile, cleanup, err := writeCredentialFile("dbq-mysql-*", mysqlOptionFile(j.Password))
		if err != nil {
			return fmt.Errorf("create MySQL credential file: %w", err)
		}
		defer cleanup()
		args := []string{"--defaults-extra-file=" + defaultsFile, "-h", j.Host, "-P", j.Port, "-u", j.Username, "ping"}
		return local.ExecCommand("mysqladmin", args, []string{"MYSQL_PWD"}, io.Discard)
	}
	remote, ok := r.(reader.RemoteCommandRunner)
	if !ok {
		return errors.New("reader does not support secure remote command execution")
	}
	defaultsFile, err := remote.CreateCredentialFile("dbq-mysql-", mysqlOptionFile(j.Password))
	if err != nil {
		return fmt.Errorf("create remote MySQL credential file: %w", err)
	}
	args := []string{"--defaults-extra-file=" + defaultsFile, "-h", j.Host, "-P", j.Port, "-u", j.Username, "ping"}
	runErr := remote.ExecRemoteCommand("mysqladmin", args, []string{"MYSQL_PWD"}, io.Discard)
	cleanupErr := remote.RemoveCredentialFile(defaultsFile)
	if runErr != nil {
		if cleanupErr != nil {
			return fmt.Errorf("mysqladmin: %w (also failed to remove remote credential file: %v)", runErr, cleanupErr)
		}
		return runErr
	}
	if cleanupErr != nil {
		return fmt.Errorf("remove remote MySQL credential file: %w", cleanupErr)
	}
	return nil
}

// checkMysqldump verifies mysqldump is available in whichever environment r targets.
func checkMysqldump(r reader.FileReader) error {
	if local, ok := r.(reader.CommandRunner); ok {
		if err := local.LookPath("mysqldump"); err != nil {
			return fmt.Errorf("mysqldump not found on target host — install mysql-client")
		}
		return nil
	}
	_, err := r.Exec("which mysqldump")
	if err != nil {
		return fmt.Errorf("mysqldump not found on target host — install mysql-client")
	}
	return nil
}
