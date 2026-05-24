package utils

import (
	"testing"
)

func TestCronToOnCalendar(t *testing.T) {
	tests := []struct {
		cron    string
		want    string
		wantErr bool
	}{
		{"0 2 * * *", "*-*-* 02:00:00", false},
		{"30 3 * * 1", "Mon *-*-* 03:30:00", false},
		{"0 0 1 * *", "*-*-01 00:00:00", false},
		{"0 0 1 1 *", "*-01-01 00:00:00", false},
		{"* * * * *", "*-*-* *:*:00", false},
		// too few / too many fields
		{"* * * *", "", true},
		{"* * * * * *", "", true},
		// complex expressions rejected
		{"0 2-4 * * *", "", true},
		{"*/5 * * * *", "", true},
		{"0 2,4 * * *", "", true},
		// invalid day-of-week
		{"0 0 * * 9", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.cron, func(t *testing.T) {
			got, err := CronToOnCalendar(tt.cron)
			if (err != nil) != tt.wantErr {
				t.Errorf("CronToOnCalendar(%q) error = %v, wantErr %v", tt.cron, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("CronToOnCalendar(%q) = %q, want %q", tt.cron, got, tt.want)
			}
		})
	}
}

func TestCronToStartCalendarInterval(t *testing.T) {
	got, err := CronToStartCalendarInterval("30 2 * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["Minute"] != 30 {
		t.Errorf("Minute = %d, want 30", got["Minute"])
	}
	if got["Hour"] != 2 {
		t.Errorf("Hour = %d, want 2", got["Hour"])
	}
	if _, ok := got["Day"]; ok {
		t.Error("Day should be absent for wildcard field")
	}
	if _, ok := got["Month"]; ok {
		t.Error("Month should be absent for wildcard field")
	}

	// complex expressions rejected
	_, err = CronToStartCalendarInterval("*/5 * * * *")
	if err == nil {
		t.Error("expected error for step expression")
	}

	// wrong field count
	_, err = CronToStartCalendarInterval("* * * *")
	if err == nil {
		t.Error("expected error for 4-field expression")
	}
}

func TestStringToID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Job", "my-job"},
		{"nightly backup", "nightly-backup"},
		{"ALLCAPS", "allcaps"},
		{"already-kebab", "already-kebab"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := StringToID(tt.input)
			if got != tt.want {
				t.Errorf("StringToID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
