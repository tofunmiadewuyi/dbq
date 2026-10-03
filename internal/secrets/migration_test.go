package secrets

import (
	"strings"
	"testing"
)

func TestDecodeLegacyRejectsAmbiguousOrUnsupportedMaterial(t *testing.T) {
	for _, input := range []string{
		`null`,
		`{"daily/db_password":"first","daily/db_password":"second"}`,
		`{"daily/unknown":"secret"}`,
		`{"daily/db_password":null}`,
		`{"missing-separator":"secret"}`,
		`{"../outside/db_password":"secret"}`,
		`{"daily/db_password":"secret"} {}`,
		`{"daily/db_password":"secret-fragment`,
	} {
		_, err := DecodeLegacy([]byte(input))
		if err == nil {
			t.Fatalf("accepted invalid legacy material %q", input)
		}
		if strings.Contains(err.Error(), "secret-fragment") {
			t.Fatal("parse error exposed credential content")
		}
	}
}
