package input

import (
	"os"
	"testing"
)

func TestValidateCron(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"* * * * *", false},
		{"0 2 * * *", false},
		{"30 14 1 * *", false},
		{"", true},
		{"* * * *", true},     // 4 fields
		{"* * * * * *", true}, // 6 fields
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateCron("schedule", tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCron(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateField(t *testing.T) {
	if err := ValidateField("name", "hello"); err != nil {
		t.Errorf("expected no error for non-empty value, got %v", err)
	}
	if err := ValidateField("name", ""); err == nil {
		t.Error("expected error for empty string")
	}
	if err := ValidateField("name", "   "); err == nil {
		t.Error("expected error for whitespace-only string")
	}
}

func TestValidateInt(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"5432", false},
		{"0", false},
		{"-1", false},
		{"abc", true},
		{"12.5", true},
		{"", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateInt("port", tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateInt(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"https://example.com", false},
		{"http://localhost:9000", false},
		{"s3://mybucket", false},
		{"", true},
		{"not-a-url", true},
		{"//no-scheme", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	got, err := ExpandPath("~/documents")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != home+"/documents" {
		t.Errorf("ExpandPath(~/documents) = %q, want %q", got, home+"/documents")
	}

	got, err = ExpandPath("/absolute/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/absolute/path" {
		t.Errorf("ExpandPath(/absolute/path) = %q, want %q", got, "/absolute/path")
	}
}

func TestValidatePath(t *testing.T) {
	if err := ValidatePath("dir", os.TempDir()); err != nil {
		t.Errorf("expected no error for existing path, got %v", err)
	}
	if err := ValidatePath("dir", "/this/path/should/not/exist/dbq-test"); err == nil {
		t.Error("expected error for non-existent path")
	}
	if err := ValidatePath("dir", ""); err == nil {
		t.Error("expected error for empty path")
	}
}
