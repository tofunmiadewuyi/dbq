package storage

import (
	"strings"
	"testing"
	"time"
)

var fixedTime = time.Date(2026, 5, 24, 14, 30, 0, 0, time.UTC)

func TestBackupFilename(t *testing.T) {
	tests := []struct {
		name    string
		jobName string
		dbName  string
		ext     string
		want    string
	}{
		{
			name:    "basic zip",
			jobName: "myjob",
			dbName:  "mydb",
			ext:     ".zip",
			want:    "myjob_mydb_20260524_143000.zip",
		},
		{
			name:    "dump.gz extension",
			jobName: "nightly",
			dbName:  "production",
			ext:     ".dump.gz",
			want:    "nightly_production_20260524_143000.dump.gz",
		},
		{
			name:    "db name with path traversal is sanitized",
			jobName: "myjob",
			dbName:  "../../etc/passwd",
			ext:     ".zip",
			want:    "myjob_passwd_20260524_143000.zip",
		},
		{
			name:    "db name with leading slash is sanitized",
			jobName: "myjob",
			dbName:  "/var/lib/mydb",
			ext:     ".zip",
			want:    "myjob_mydb_20260524_143000.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BackupFilename(tt.jobName, tt.dbName, fixedTime, tt.ext)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBackupKey(t *testing.T) {
	tests := []struct {
		name    string
		jobName string
		dbName  string
		ext     string
		want    string
	}{
		{
			name:    "zip",
			jobName: "myjob",
			dbName:  "mydb",
			ext:     ".zip",
			want:    "backups/myjob/mydb/myjob_mydb_20260524_143000.zip",
		},
		{
			name:    "dump.gz",
			jobName: "nightly",
			dbName:  "production",
			ext:     ".dump.gz",
			want:    "backups/nightly/production/nightly_production_20260524_143000.dump.gz",
		},
		{
			name:    "db name sanitized in both path segment and filename",
			jobName: "myjob",
			dbName:  "../../etc/passwd",
			ext:     ".zip",
			want:    "backups/myjob/passwd/myjob_passwd_20260524_143000.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BackupKey(tt.jobName, tt.dbName, fixedTime, tt.ext)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBackupKeyLeafEqualsBackupFilename(t *testing.T) {
	filename := BackupFilename("job", "db", fixedTime, ".zip")
	key := BackupKey("job", "db", fixedTime, ".zip")
	if !strings.HasSuffix(key, "/"+filename) {
		t.Errorf("BackupKey %q does not end with BackupFilename %q", key, filename)
	}
}
