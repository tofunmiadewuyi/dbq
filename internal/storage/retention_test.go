package storage

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

func obj(key string, t time.Time) BackupObject { return BackupObject{Key: key, Timestamp: t} }

func day(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }

func keysOf(objs []BackupObject) []string {
	out := make([]string, len(objs))
	for i, o := range objs {
		out[i] = o.Key
	}
	return out
}

func TestKeepLatest(t *testing.T) {
	objs := []BackupObject{
		obj("a", day(1)),
		obj("c", day(3)),
		obj("b", day(2)),
		obj("e", day(5)),
		obj("d", day(4)),
	}

	tests := []struct {
		name string
		objs []BackupObject
		keep int
		want []string // keys expected to be deleted
	}{
		{"keep 0 is unlimited", objs, 0, nil},
		{"negative keep deletes nothing", objs, -1, nil},
		{"keep more than available", objs, 10, nil},
		{"keep exactly the count", objs, 5, nil},
		{"keep 2 deletes 3 oldest", objs, 2, []string{"c", "b", "a"}},
		{"keep 1 never deletes newest", objs, 1, []string{"d", "c", "b", "a"}},
		{"empty input", nil, 3, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keysOf(keepLatest(tt.objs, tt.keep))
			if len(got) == 0 {
				got = nil
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("keepLatest keep=%d = %v, want %v", tt.keep, got, tt.want)
			}
			if len(tt.objs) > 0 && tt.keep >= 1 && len(got) == len(tt.objs) {
				t.Errorf("keepLatest deleted every object — newest was not retained")
			}
		})
	}
}

// TestKeepLatestSortsByTimestampNotKey proves recency comes from the parsed
// timestamp, not the key string: the key order is deliberately the reverse of
// the timestamp order.
func TestKeepLatestSortsByTimestampNotKey(t *testing.T) {
	objs := []BackupObject{
		obj("zzz_newest", day(3)),
		obj("mmm_middle", day(2)),
		obj("aaa_oldest", day(1)),
	}
	got := keysOf(keepLatest(objs, 1))
	want := []string{"mmm_middle", "aaa_oldest"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v — timestamp should win over key ordering", got, want)
	}
}

// TestKeepLatestTiebreak: equal timestamps fall back to key ordering so results
// are deterministic.
func TestKeepLatestTiebreak(t *testing.T) {
	same := day(1)
	objs := []BackupObject{obj("a", same), obj("b", same), obj("c", same)}
	got := keysOf(keepLatest(objs, 1))
	want := []string{"b", "a"} // "c" is the highest key, retained
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseBackupTime(t *testing.T) {
	tests := []struct {
		name           string
		jobName, dbNam string
		input          string
		wantOK         bool
		want           time.Time
	}{
		{"bare filename", "nightly", "mydb", "nightly_mydb_20260524_143000.zip", true, time.Date(2026, 5, 24, 14, 30, 0, 0, time.UTC)},
		{"full key", "nightly", "mydb", "backups/nightly/mydb/nightly_mydb_20260524_143000.zip", true, time.Date(2026, 5, 24, 14, 30, 0, 0, time.UTC)},
		{"dump.gz extension", "nightly", "mydb", "nightly_mydb_20260524_143000.dump.gz", true, time.Date(2026, 5, 24, 14, 30, 0, 0, time.UTC)},
		{"wrong job prefix", "nightly", "mydb", "other_mydb_20260524_143000.zip", false, time.Time{}},
		{"wrong db prefix", "nightly", "mydb", "nightly_otherdb_20260524_143000.zip", false, time.Time{}},
		{"non-date where timestamp should be", "nightly", "mydb", "nightly_mydb_notadatehere.zip", false, time.Time{}},
		{"too short to hold a timestamp", "nightly", "mydb", "nightly_mydb_2026.zip", false, time.Time{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseBackupTime(tt.jobName, tt.dbNam, tt.input)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPruneDirectory(t *testing.T) {
	dir := t.TempDir()

	files := []string{
		// ours (oldest -> newest by embedded timestamp)
		"nightly_mydb_20260101_000000.zip",
		"nightly_mydb_20260102_000000.zip",
		"nightly_mydb_20260103_000000.zip",
		// decoys that must never be touched
		"other_mydb_20260101_000000.zip",      // different job
		"nightly_otherdb_20260101_000000.zip", // different db
		"nightly_mydb_tampered.zip",           // matches prefix but no valid timestamp
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := PruneDirectory(dir, "nightly", "mydb", 1)
	if err != nil {
		t.Fatalf("PruneDirectory: %v", err)
	}

	wantDeleted := []string{
		filepath.Join(dir, "nightly_mydb_20260102_000000.zip"),
		filepath.Join(dir, "nightly_mydb_20260101_000000.zip"),
	}
	if !reflect.DeepEqual(deleted, wantDeleted) {
		t.Errorf("deleted = %v, want %v", deleted, wantDeleted)
	}

	// Survivors: newest of ours + both decoys + the tampered file (un-parseable
	// names are never pruned).
	wantRemaining := []string{
		"nightly_mydb_20260103_000000.zip",
		"nightly_mydb_tampered.zip",
		"nightly_otherdb_20260101_000000.zip",
		"other_mydb_20260101_000000.zip",
	}
	if remaining := listNames(t, dir); !reflect.DeepEqual(remaining, wantRemaining) {
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
