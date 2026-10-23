package reader

import (
	"errors"
	"fmt"
	"strings"
)

var errDumpDiagnostic = errors.New("dump tool reported an error despite successful exit")

// mysqldump can report failures to read tablespaces or other metadata while
// exiting zero. A zero status alone must not certify a complete backup. GTID
// warnings (including their continuation lines) are advisory, not failures.
func dumpDiagnosticError(stderr string) error {
	for _, line := range strings.Split(stderr, "\n") {
		diagnostic := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(diagnostic, "mysqldump:") && !strings.Contains(diagnostic, "[warning]") && !strings.HasPrefix(diagnostic, "mysqldump: warning:") {
			return fmt.Errorf("%w: %s", errDumpDiagnostic, strings.TrimSpace(line))
		}
	}
	return nil
}
