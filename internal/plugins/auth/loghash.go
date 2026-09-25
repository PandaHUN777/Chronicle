// loghash provides email hashing for debug-log correlation.
//
// Password-reset Debug logs must never carry a raw email address: a
// log-shipping operator could otherwise distinguish "known but rate-limited"
// from "unknown email" branches and enumerate registered emails. The hash is
// for debug correlation only — not a credential, not stored, not used for any
// security decision — so truncation to 16 hex characters is acceptable.
package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// hashEmail returns a 16-character SHA-256 hex prefix of the lowercased
// email address, so password-reset Debug logs stay differentiable without
// a raw email address reaching log shipment paths.
func hashEmail(email string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(h[:])[:16]
}
