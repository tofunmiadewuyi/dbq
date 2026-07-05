package storage

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestKeepLatest(t *testing.T) {
	// deliberately unsorted input; keepLatest must sort internally.
	keys := []string{
		"job_db_20260101_000000.zip",
		"job_db_20260103_000000.zip",
		"job_db_20260102_000000.zip",
		"job_db_20260105_000000.zip",
		"job_db_20260104_000000.zip",
	}

	tests := []struct {
		name string
		keys []string
		keep int
		want []string // keys expected to be deleted
	}{
		{
			name: "keep 0 is unlimited, deletes nothing",
			keys: keys,
			keep: 0,
			want: nil,
		},
		{
			name: "negative keep deletes nothing",
			keys: keys,
			keep: -1,
			want: nil,
		},
		{
			name: "keep more than available deletes nothing",
			keys: keys,
			keep: 10,
			want: nil,
		},
		{
			name: "keep exactly the count deletes nothing",
			keys: keys,
			keep: 5,
			want: nil,
		},
		{
			name: "keep 2 deletes the 3 oldest",
			keys: keys,
			keep: 2,
			want: []string{
				"job_db_20260103_000000.zip",
				"job_db_20260102_000000.zip",
				"job_db_20260101_000000.zip",
			},
		},
		{
			name: "keep 1 never deletes the newest",
			keys: keys,
			keep: 1,
			want: []string{
				"job_db_20260104_000000.zip",
				"job_db_20260103_000000.zip",
				"job_db_20260102_000000.zip",
				"job_db_20260101_000000.zip",
			},
		},
		{
			name: "empty input",
			keys: nil,
			keep: 3,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keepLatest(tt.keys, tt.keep)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("keepLatest(%v, %d) = %v, want %v", tt.keys, tt.keep, got, tt.want)
			}
			// safety invariant: when there is at least one key and keep >= 1,
			// keepLatest must never return every key.
			if len(tt.keys) > 0 && tt.keep >= 1 && len(got) == len(tt.keys) {
				t.Errorf("keepLatest deleted every key — newest was not retained")
			}
		})
	}
}

func TestPruneDirectory(t *testing.T) {
	dir := t.TempDir()

	// three backups for our job/db (oldest -> newest by timestamp)
	ours := []string{
		"nightly_mydb_20260101_000000.zip",
		"nightly_mydb_20260102_000000.zip",
		"nightly_mydb_20260103_000000.zip",
	}
	// decoys that must never be touched: another job, another db, a subdir.
	decoys := []string{
		"other_mydb_20260101_000000.zip",   // different job
		"nightly_otherdb_20260101_000000.zip", // different db
	}

	for _, name := range append(append([]string{}, ours...), decoys...) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := PruneDirectory(dir, "nightly", "mydb", 1)
	if err != nil {
		t.Fatalf("PruneDirectory: %v", err)
	}

	// should have deleted the two oldest of ours, keeping the newest.
	wantDeleted := []string{
		filepath.Join(dir, "nightly_mydb_20260102_000000.zip"),
		filepath.Join(dir, "nightly_mydb_20260101_000000.zip"),
	}
	if !reflect.DeepEqual(deleted, wantDeleted) {
		t.Errorf("deleted = %v, want %v", deleted, wantDeleted)
	}

	// verify surviving files: newest of ours + both decoys.
	remaining := listNames(t, dir)
	wantRemaining := []string{
		"nightly_mydb_20260103_000000.zip",
		"nightly_otherdb_20260101_000000.zip",
		"other_mydb_20260101_000000.zip",
	}
	if !reflect.DeepEqual(remaining, wantRemaining) {
		t.Errorf("remaining = %v, want %v", remaining, wantRemaining)
	}
}

func TestPruneDirectoryUnlimitedIsNoop(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"job_db_20260101_000000.zip",
		"job_db_20260102_000000.zip",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := PruneDirectory(dir, "job", "db", 0)
	if err != nil {
		t.Fatalf("PruneDirectory: %v", err)
	}
	if len(deleted) != 0 {
		t.Errorf("keep=0 should delete nothing, deleted %v", deleted)
	}
	if got := len(listNames(t, dir)); got != 2 {
		t.Errorf("expected 2 files to remain, got %d", got)
	}
}

func listNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}
