// Package media -- signed_url.go implements HMAC-SHA256 signed media URLs.
// Signed URLs prevent permanent, irrevocable access to media files by
// requiring a time-limited cryptographic token. This mirrors the approach
// used by AWS S3, Google Cloud Storage, and Cloudflare R2 presigned URLs.
package media

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// URLSigner generates and verifies HMAC-SHA256 signed media URLs.
// The signing secret must be kept confidential -- anyone who knows
// it can forge valid signed URLs for any media file.
type URLSigner struct {
	secret []byte
}

// NewURLSigner creates a signer with the given secret key.
// The secret should be at least 32 bytes for adequate security.
func NewURLSigner(secret string) *URLSigner {
	return &URLSigner{secret: []byte(secret)}
}

// SignedURLTTL bounds how long a signed media URL remains valid after
// minting. Shortened (2026-09-13, ADR-058 decision 6 audit item) from the
// previous 1 hour, which predated viewer binding: back when a signature was
// a bare bearer token, TTL was the ONLY thing bounding how long a copied
// link kept working for whoever held it, and an hour is a long time to
// leave that open. Viewer binding (this file's Sign/Verify now embed WHO
// the link is for, see ViewerSession/ViewerAPIKey/ViewerAnonymous below)
// removes most of that risk, but shortening the TTL anyway costs nothing
// real: a page's images -- thumbnails included -- finish loading within
// seconds of render, and even a slow connection or a large gallery is well
// inside 15 minutes. A tab left open past that window just needs a reload
// to re-mint, the same "reload fixes it" posture mediaAccessCacheTTL
// already assumes for the ADR-058 entity-visibility cache (60s).
const SignedURLTTL = 15 * time.Minute

// --- Viewer binding (ADR-058 decision 6) ---
//
// Before this, Sign/SignThumb produced an HMAC over "fileID:expires" (or
// "fileID:size:expires") alone -- a bearer token. Anyone holding the
// (expires, sig) query pair could use it, for anyone, until it expired.
// Folding a VIEWER identity into the signed payload means the signature
// Verify computes for a DIFFERENT presented viewer will not match, so a
// copied link is inert for anyone else. See Verify's doc comment for the
// one deliberate exception (the Foundry cross-origin flow) and exactly why
// it does not reopen the hole this closes.
//
// This is also why an old-format link (signed before this change) stops
// working immediately rather than riding out its remaining TTL: the old
// payload never included a viewer segment at all, so there is no way to
// recompute a matching digest for it under the new scheme -- Verify simply
// never finds a match. That is a deliberate choice, not an oversight: the
// alternative (accepting the old two-field payload as an "anyone" grant
// alongside the new one) would keep the exact bearer-token behavior this
// decision exists to remove alive for up to an hour after every deploy.
// Given the shortened TTL above, the cost is a handful of already-open
// browser tabs or in-flight requests getting a broken image right at
// deploy until they reload -- not a silent, hour-long window.
const (
	// ViewerAnonymous marks a link minted for (or presented by) a request
	// that carried no authenticated Chronicle session: an anonymous visitor
	// to a public campaign page, or any cross-origin/cookieless request --
	// a signed <img src> can never carry a session cookie, so every such
	// request PRESENTS as this, regardless of what it was minted as.
	ViewerAnonymous = "anon"

	// ViewerAPIKey marks a link minted by a Bearer-token-authenticated
	// syncapi caller (Foundry VTT, or any other REST integration) rather
	// than a browser session. It is a single fixed sentinel, not one value
	// per API key or campaign: the request that will eventually PRESENT
	// this link (Foundry's cross-origin <img> fetch) is cookieless by
	// construction, so it can never present anything more specific than
	// "anonymous" -- there is no header, cookie, or query param on an
	// <img> GET a browser lets Foundry attach that could prove which key
	// minted the link. Verify's fallback below exists for exactly this
	// sentinel and no other.
	ViewerAPIKey = "apikey"
)

// ViewerSession returns the viewer identity for a signed URL minted for an
// authenticated Chronicle browser session. Embedding the user id means the
// signature Verify computes for any OTHER user id -- or for an anonymous
// presentation -- will not match it.
func ViewerSession(userID string) string {
	return "session:" + userID
}

// Sign generates a signed URL path for a media file with the given TTL,
// binding it to viewer (ADR-058 decision 6) -- one of ViewerAnonymous,
// ViewerAPIKey, or ViewerSession(userID). The returned path includes
// ?expires= and &sig= query parameters; the viewer is NOT itself present in
// the URL -- it is folded into the signature, and Verify's caller supplies
// the PRESENTED viewer for comparison rather than reading it back out.
func (s *URLSigner) Sign(fileID, viewer string, ttl time.Duration) string {
	expires := time.Now().Add(ttl).Unix()
	sig := s.computeSignature(fileID, viewer, expires)
	return fmt.Sprintf("/media/%s?expires=%d&sig=%s", fileID, expires, sig)
}

// SignThumb generates a signed URL path for a media thumbnail, viewer-bound
// exactly like Sign.
func (s *URLSigner) SignThumb(fileID, size, viewer string, ttl time.Duration) string {
	expires := time.Now().Add(ttl).Unix()
	// Include size in the signed payload to prevent size parameter tampering.
	sig := s.computeThumbSignature(fileID, size, viewer, expires)
	return fmt.Sprintf("/media/%s/thumb/%s?expires=%d&sig=%s", fileID, size, expires, sig)
}

// Verify checks that a signature is valid, not expired, and was minted for
// the PRESENTED viewer -- the caller's best determination of who is asking
// right now (see media.currentViewerIdentity: a session's user id, or
// ViewerAnonymous when the request carries none). Uses hmac.Equal for
// constant-time comparison to prevent timing attacks.
//
// The one deliberate exception: when the presented viewer is
// ViewerAnonymous (a cookieless request -- there is nothing else it could
// present), a signature minted for the fixed ViewerAPIKey sentinel ALSO
// verifies. That is precisely, and only, the operator's Foundry
// cross-origin <img> flow this must not break
// (signed_url_trust_test.go: TestCheckMediaAccess_ValidSignedURL_NoCookie_PrivateCampaign)
// -- syncapi.MediaAPIHandler.toAPIResponse mints every media URL it hands
// to a Bearer-token caller with ViewerAPIKey, and that link is then fetched
// by Foundry as a cross-origin <img>, which cannot carry Chronicle's
// session cookie. It does NOT let a link minted for a specific session
// (ViewerSession(userID)) be replayed cookielessly: that signature was
// computed over "session:<id>", never over ViewerAPIKey, so it will not
// match here either -- the fallback only ever accepts the ViewerAPIKey
// digest specifically.
func (s *URLSigner) Verify(fileID, viewer, expiresStr, signature string) bool {
	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > expires {
		return false
	}
	if hmac.Equal([]byte(signature), []byte(s.computeSignature(fileID, viewer, expires))) {
		return true
	}
	if viewer == ViewerAnonymous {
		return hmac.Equal([]byte(signature), []byte(s.computeSignature(fileID, ViewerAPIKey, expires)))
	}
	return false
}

// VerifyThumb checks a thumbnail signature including the size parameter,
// with the same viewer binding (and the same ViewerAPIKey/anonymous
// carve-out) as Verify.
func (s *URLSigner) VerifyThumb(fileID, size, viewer, expiresStr, signature string) bool {
	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > expires {
		return false
	}
	if hmac.Equal([]byte(signature), []byte(s.computeThumbSignature(fileID, size, viewer, expires))) {
		return true
	}
	if viewer == ViewerAnonymous {
		return hmac.Equal([]byte(signature), []byte(s.computeThumbSignature(fileID, size, ViewerAPIKey, expires)))
	}
	return false
}

// computeSignature creates an HMAC-SHA256 hex digest over
// "{fileID}:{viewer}:{expires}". Including viewer is decision 6's whole
// mechanism: two requests presenting different viewers for the same file
// and expiry produce different digests, so a link cannot be re-authorized
// for someone it was not minted for just by knowing fileID and expires.
func (s *URLSigner) computeSignature(fileID, viewer string, expires int64) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = fmt.Fprintf(mac, "%s:%s:%d", fileID, viewer, expires)
	return hex.EncodeToString(mac.Sum(nil))
}

// computeThumbSignature includes the size (to prevent size parameter
// tampering) and the viewer (decision 6), same rationale as
// computeSignature.
func (s *URLSigner) computeThumbSignature(fileID, size, viewer string, expires int64) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = fmt.Fprintf(mac, "%s:%s:%s:%d", fileID, size, viewer, expires)
	return hex.EncodeToString(mac.Sum(nil))
}

// GenerateSigningSecret creates a cryptographically random 32-byte hex string
// suitable for use as a MEDIA_SIGNING_SECRET. Called during first boot if
// no secret is configured.
func GenerateSigningSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating signing secret: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// SigningSecretSource describes where LoadOrInitSigningSecret got
// its secret, so the caller can log the right warning. The signer
// itself doesn't care about provenance, but the operator does — an
// in-memory secret means every restart silently invalidates every
// outstanding Foundry manifest token (since foundry_vtt's
// TokenSigner shares this secret with the media URLSigner).
type SigningSecretSource string

const (
	// SecretFromEnv — secret came from the env var (operator-managed).
	// Subsequent restarts will read the same env. No persistence
	// concern.
	SecretFromEnv SigningSecretSource = "env"

	// SecretFromFile — secret was previously generated and persisted;
	// this boot read the persisted value. Restart-stable.
	SecretFromFile SigningSecretSource = "file"

	// SecretGeneratedAndPersisted — generated this boot AND written
	// to disk. Restart-stable from now on. Operator should still
	// switch to env-managed for production hygiene.
	SecretGeneratedAndPersisted SigningSecretSource = "generated_and_persisted"

	// SecretGeneratedInMemory — generated this boot, persistence
	// failed (data dir not writable, or path empty). DANGER: every
	// restart will silently invalidate every Foundry manifest
	// token. This is the Issue #17 mode.
	SecretGeneratedInMemory SigningSecretSource = "generated_in_memory"
)

// LoadOrInitSigningSecret resolves the HMAC signing secret used by
// both the media URLSigner and the foundry_vtt TokenSigner.
// Priority:
//
//  1. envSecret if non-empty (operator set MEDIA_SIGNING_SECRET).
//  2. The persisted file at path if it exists.
//  3. A freshly generated secret, persisted to path. If persist
//     fails, the secret is still returned but flagged in-memory.
//
// Persistence is load-bearing: the foundry_vtt TokenSigner uses this
// secret as its HMAC key, and Foundry stores manifest URLs (which
// embed tokens signed with this secret) indefinitely. Without
// persistence, restart → new secret → every outstanding manifest
// token 403s — the symptom diagnosed in cordinator Issue #17.
//
// Pass path="" to disable persistence (test-only; production should
// always pass a path).
//
// Returns (secret, source, error). A non-nil error never means the
// secret is unusable — it's a soft signal that something prevented
// persistence (most often "data dir not writable"). The caller
// should log the source + error to the operator either way.
func LoadOrInitSigningSecret(envSecret, path string) (string, SigningSecretSource, error) {
	if envSecret != "" {
		return envSecret, SecretFromEnv, nil
	}

	if path != "" {
		if b, err := os.ReadFile(path); err == nil {
			secret := strings.TrimSpace(string(b))
			if secret != "" {
				return secret, SecretFromFile, nil
			}
			// Empty file — fall through to regenerate. The file
			// will be overwritten with the new secret.
		} else if !errors.Is(err, fs.ErrNotExist) {
			// Read error other than "doesn't exist" — return it so
			// the caller can log it, but proceed to generate.
			generated, genErr := GenerateSigningSecret()
			if genErr != nil {
				return "", SecretGeneratedInMemory, fmt.Errorf("read %s: %w; then generate: %v", path, err, genErr)
			}
			return generated, SecretGeneratedInMemory, fmt.Errorf("read %s: %w", path, err)
		}
	}

	generated, err := GenerateSigningSecret()
	if err != nil {
		return "", SecretGeneratedInMemory, err
	}

	if path == "" {
		return generated, SecretGeneratedInMemory, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return generated, SecretGeneratedInMemory, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(generated), 0o600); err != nil {
		return generated, SecretGeneratedInMemory, fmt.Errorf("write %s: %w", path, err)
	}
	return generated, SecretGeneratedAndPersisted, nil
}
