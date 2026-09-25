// logging_test.go pins that sensitiveParams redacts `sig` and `expires`:
// internal/plugins/media's signed URLs put a live, directly-usable
// credential — an HMAC signature plus the expiry it is valid until — in the
// query string, and an omission here would write that credential to the
// request log in plaintext for the rest of its TTL. See ADR-058.
package middleware

import (
	"strings"
	"testing"
)

func TestRedactQuery_RedactsMediaSignatureParams(t *testing.T) {
	raw := "expires=1234567890&sig=deadbeefcafef00d"
	got := redactQuery(raw)

	if strings.Contains(got, "deadbeefcafef00d") {
		t.Errorf("redactQuery(%q) = %q, still contains the live signature", raw, got)
	}
	if strings.Contains(got, "1234567890") {
		t.Errorf("redactQuery(%q) = %q, still contains the raw expiry", raw, got)
	}
	// redactQuery re-encodes via url.Values.Encode, which percent-encodes
	// the brackets — assert on the encoded form rather than the literal.
	if !strings.Contains(got, "%5BREDACTED%5D") {
		t.Errorf("redactQuery(%q) = %q, want both params redacted", raw, got)
	}
}

// TestRedactQuery_CaseInsensitiveParamNames confirms sig/expires are
// matched the same case-insensitive way (strings.EqualFold) every other
// sensitive param already is.
func TestRedactQuery_CaseInsensitiveParamNames(t *testing.T) {
	got := redactQuery("Sig=abc123&Expires=999")
	if strings.Contains(got, "abc123") || strings.Contains(got, "999") {
		t.Errorf("redactQuery did not redact mixed-case Sig/Expires: %q", got)
	}
}

// TestRedactQuery_LeavesOrdinaryParamsAlone is a control: redaction must
// not touch unrelated query params (would mask a real regression as a
// false pass above).
func TestRedactQuery_LeavesOrdinaryParamsAlone(t *testing.T) {
	got := redactQuery("page=2&per_page=24")
	if got != "page=2&per_page=24" {
		t.Errorf("redactQuery(%q) = %q, want unchanged", "page=2&per_page=24", got)
	}
}
