package reader

import (
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
