package input

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ValidateCron(fieldName, s string) error {
	if len(strings.Fields(s)) != 5 {
		return fmt.Errorf("%s must be a valid cron expression (5 fields)", fieldName)
	}
	return nil
}

func ValidateField(fieldName, s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("%s is required", fieldName)
	}
	return nil
}

func ValidateInt(fieldName, s string) error {
	_, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("%s must be a number", fieldName)
	}
	return nil
}

func ExpandPath(s string) (string, error) {
	if strings.HasPrefix(s, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not resolve home directory: %w", err)
		}
		return filepath.Join(home, s[2:]), nil
	}
	return s, nil
}

func ValidatePath(fieldName, s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("%s path is required", fieldName)
	}
	expanded, err := ExpandPath(s)
	if err != nil {
		return err
	}
	if _, err := os.Stat(expanded); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", s)
	}
	return nil
}

func ValidateURL(s string) error {
    if strings.TrimSpace(s) == "" {
        return fmt.Errorf("url is required")
    }
    u, err := url.Parse(s)
    if err != nil || u.Scheme == "" || u.Host == "" {
        return fmt.Errorf("invalid url: %s", s)
    }
    return nil
}
