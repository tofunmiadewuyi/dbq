package reader

import (
	"bytes"
	"os"
	"reflect"
	"testing"
)

func TestMergeEnvOverridesAndRemovesSensitiveValues(t *testing.T) {
	base := []string{
		"PATH=/usr/bin",
		"PGPASSWORD=inherited-postgres-secret",
		"MYSQL_PWD=inherited-mysql-secret",
	}
	overrides := []string{
		"PGPASSWORD",
		"MYSQL_PWD",
		"PGPASSFILE=/tmp/dbq-pgpass",
	}
	want := []string{"PATH=/usr/bin", "PGPASSFILE=/tmp/dbq-pgpass"}

	if got := mergeEnv(base, overrides); !reflect.DeepEqual(got, want) {
		t.Fatalf("mergeEnv() = %#v, want %#v", got, want)
	}
}

func TestLocalCommandsDoNotInheritRunnerCredentials(t *testing.T) {
	for _, key := range []string{"DBQ_DB_PASSWORD", "DBQ_STORAGE_ACCESS_KEY", "DBQ_STORAGE_SECRET_KEY", "PGPASSWORD", "MYSQL_PWD"} {
		t.Setenv(key, "synthetic-secret")
	}
	t.Setenv("DBQ_TEST_VISIBLE", "preserved")
	script := `for key in DBQ_DB_PASSWORD DBQ_STORAGE_ACCESS_KEY DBQ_STORAGE_SECRET_KEY PGPASSWORD MYSQL_PWD; do eval 'test -z "${'"$key"'+x}"' || exit 9; done; test "$DBQ_TEST_VISIBLE" = preserved || exit 10; printf clean`
	r := &LocalFileReader{}
	for _, mode := range []string{"direct", "shell", "stream"} {
		t.Run(mode, func(t *testing.T) {
			var out bytes.Buffer
			var err error
			switch mode {
			case "direct":
				err = r.ExecCommand("sh", []string{"-c", script}, nil, &out)
			case "shell":
				var data []byte
				data, err = r.Exec(script)
				out.Write(data)
			case "stream":
				err = r.ExecStream(script, &out)
			}
			if err != nil || out.String() != "clean" {
				t.Fatalf("child environment isolation failed: %v", err)
			}
		})
	}
	if os.Getenv("DBQ_DB_PASSWORD") != "synthetic-secret" {
		t.Fatal("parent credentials were removed")
	}
}
