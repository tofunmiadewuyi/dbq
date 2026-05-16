// Package utils provides shared helper functions used across the application.
package utils

import "strings"

// TailLines returns the last n lines of s. If n <= 0 or n >= total lines, s is returned unchanged.
func TailLines(s string, n int) string {
	if n <= 0 {
		return s
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if n < len(lines) {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n") + "\n"
}
