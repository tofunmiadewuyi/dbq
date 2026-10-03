package engine

import (
	"regexp"
	"sort"
	"strings"

	"github.com/tofunmiadewuyi/dbq/internal/secrets"
)

var diagnosticURL = regexp.MustCompile(`https?://[^\s<>"']+`)
var signedURLParameter = regexp.MustCompile(`(?i)([?&](?:X-Amz-Signature|X-Amz-Credential|X-Amz-Security-Token|AWSAccessKeyId|Signature|SecurityToken)=)[^&#\s]*`)

type redactedError struct {
	cause error
	text  string
}

func (e *redactedError) Error() string { return e.text }
func (e *redactedError) Unwrap() error { return e.cause }

// redactError only replaces supplied secret values and recognized S3 URL
// authentication parameters. Unmatched diagnostics remain unchanged. The cause
// remains available for errors.Is/As; callers should display Error(), not unwrap
// and print the original diagnostic.
func redactError(err error, values secrets.JobSecrets) error {
	if err == nil {
		return nil
	}
	original := err.Error()
	text := diagnosticURL.ReplaceAllStringFunc(original, func(value string) string {
		return signedURLParameter.ReplaceAllString(value, "${1}[REDACTED]")
	})
	known := []string{values.DatabasePassword, values.StorageAccessKey, values.StorageSecretKey}
	// Replace longer secrets first so overlapping values are removed completely.
	sort.Slice(known, func(i, j int) bool { return len(known[i]) > len(known[j]) })
	pairs := make([]string, 0, len(known)*2)
	for _, value := range known {
		if value != "" {
			pairs = append(pairs, value, "[REDACTED]")
		}
	}
	if len(pairs) > 0 {
		text = strings.NewReplacer(pairs...).Replace(text)
	}
	if text == original {
		return err
	}
	return &redactedError{cause: err, text: text}
}
