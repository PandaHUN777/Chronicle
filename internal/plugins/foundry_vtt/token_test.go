// Tests for the per-campaign token signer. Ported from
// foundry_modules/token_test.go with the domain-prefix change as
// the only assertion difference.
package foundry_vtt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
)

// TestTokenSigner_RoundTrip — sign a token, verify it, get the
// same (campaignID, tokenVersion) back. Pin the round-trip
// contract.
func TestTokenSigner_RoundTrip(t *testing.T) {
	signer := NewTokenSigner("test-secret")
	token := signer.Sign("camp-1", 7)

	cid, ver, err := signer.Verify(token)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if cid != "camp-1" {
		t.Errorf("expected campaignID 'camp-1', got %q", cid)
	}
	if ver != 7 {
		t.Errorf("expected tokenVersion 7, got %d", ver)
	}
}

// TestTokenSigner_DifferentSecrets — a token minted with one
// secret must NOT verify with another. The whole HMAC contract.
func TestTokenSigner_DifferentSecrets(t *testing.T) {
	signerA := NewTokenSigner("secret-a")
	signerB := NewTokenSigner("secret-b")

	token := signerA.Sign("camp-1", 1)
	_, _, err := signerB.Verify(token)
	if !errors.Is(err, errInvalidToken) {
		t.Errorf("expected errInvalidToken when verifying with different secret, got %v", err)
	}
}

// TestTokenSigner_TamperedSignature — flipping a bit in the
// signature must fail verification. Defense against any actor
// guessing at the token shape.
func TestTokenSigner_TamperedSignature(t *testing.T) {
	signer := NewTokenSigner("test-secret")
	token := signer.Sign("camp-1", 1)
	// Tamper: swap the last char (in base64-decoded form it's a
	// hex digit of the signature; even one bit changes the HMAC).
	tampered := token[:len(token)-1] + "0"
	if tampered == token {
		tampered = token[:len(token)-1] + "1"
	}
	_, _, err := signer.Verify(tampered)
	if !errors.Is(err, errInvalidToken) {
		t.Errorf("expected errInvalidToken on tampered token, got %v", err)
	}
}

// TestTokenSigner_CampaignIDWithDots — campaign IDs are UUIDs
// without dots in practice, but the wire format splits on dots so
// defensive: a hypothetical dotted ID must round-trip.
func TestTokenSigner_CampaignIDWithDots(t *testing.T) {
	signer := NewTokenSigner("test-secret")
	token := signer.Sign("a.b.c", 1)

	cid, ver, err := signer.Verify(token)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if cid != "a.b.c" {
		t.Errorf("expected campaignID 'a.b.c', got %q", cid)
	}
	if ver != 1 {
		t.Errorf("expected tokenVersion 1, got %d", ver)
	}
}

// TestTokenSigner_DomainSeparation: foundry_vtt's signer uses the
// "foundry-vtt:" domain prefix, so a token minted with a
// "foundry-module:" prefix must not verify here — this is what
// prevents cross-plugin token replay.
func TestTokenSigner_DomainSeparation(t *testing.T) {
	signer := NewTokenSigner("shared-secret")
	// Inline reimplementation of compute() with the other domain
	// prefix. If foundry-vtt's signer ever drops its domain prefix,
	// this would verify and we'd allow cross-plugin replay.
	mac := hmac.New(sha256.New, []byte("shared-secret"))
	_, _ = fmt.Fprintf(mac, "foundry-module:%s:%d", "camp-1", 1)
	otherDomainSig := fmt.Sprintf("%x", mac.Sum(nil))
	raw := fmt.Sprintf("%s.%d.%s", "camp-1", 1, otherDomainSig)
	otherDomainTok := base64.RawURLEncoding.EncodeToString([]byte(raw))

	_, _, err := signer.Verify(otherDomainTok)
	if !errors.Is(err, errInvalidToken) {
		t.Errorf("foundry-module-domain token MUST NOT verify with foundry-vtt signer (cross-plugin replay defense), got %v", err)
	}
}
