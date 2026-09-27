package source

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func writeCredentialFile(prefix, contents string) (string, func(), error) {
	f, err := os.CreateTemp("", prefix)
	if err != nil {
		return "", nil, err
	}
	path := f.Name()
	cleanup := func() { _ = os.Remove(path) }
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, err
	}
	if _, err := f.WriteString(contents); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

func createLocalDumpFile(path string) (*os.File, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
}

func pgpassLine(j *SourceJob) (string, error) {
	fields := []string{j.Host, j.Port, j.Name, j.Username, j.Password}
	for i, field := range fields {
		if strings.ContainsAny(field, "\r\n") {
			return "", fmt.Errorf("pgpass field %d contains a newline", i+1)
		}
		fields[i] = strings.NewReplacer(`\`, `\\`, `:`, `\:`).Replace(field)
	}
	return strings.Join(fields, ":") + "\n", nil
}

func mysqlOptionFile(password string) string {
	escaped := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
	).Replace(password)
	return "[client]\npassword=\"" + escaped + "\"\n"
}
