// Contract tests:
//
//  - ForcePinCampaign wraps the underlying error so the audit trail
//    shows "force-pin" was the failed path, while keeping the
//    original typed error unwrappable via errors.As / errors.Is.
//
//  - showAffectedCampaignsOnClick produces an IIFE that targets the
//    correct DOM ID (matches packages.sanitizeForID's rule of
//    dots-to-hyphens).
//
//  - sanitizeVersionForDOMID stays in lock-step with packages's
//    sanitizeForID — if either side drifts, the banner button
//    silently fails to find its target.
package foundry_vtt

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/keyxmakerx/chronicle/internal/plugins/packages"
)

// TestForcePinCampaign_ErrorWrapsAndUnwraps: when SetPinnedVersion
// returns a typed error, ForcePinCampaign wraps it for diagnostic
// context but must preserve the chain so errors.As(err, &fe) still
// extracts the typed Error — otherwise the handler's categorized
// JSON error response degrades to "internal" for every failure.
func TestForcePinCampaign_ErrorWrapsAndUnwraps(t *testing.T) {
	// Use a service whose pkgs reader returns nil package (triggers
	// ErrNoPackageRegistered inside SetPinnedVersion).
	svc := &service{
		pkgs:     &noFoundryPackageReader{},
		settings: &fakeSettings{},
	}
	err := svc.ForcePinCampaign(context.Background(),
		"camp-1", "v0.1.10", "admin-1", "1.2.3.4", "ua")
	if err == nil {
		t.Fatal("expected error when no package registered")
	}

	// Wrap visible in the error message.
	if !strings.Contains(err.Error(), "force-pin campaign") {
		t.Errorf("error message should include force-pin context, got: %v", err)
	}
	if !strings.Contains(err.Error(), "camp-1") {
		t.Errorf("error message should include campaign ID, got: %v", err)
	}
	if !strings.Contains(err.Error(), "v0.1.10") {
		t.Errorf("error message should include target version, got: %v", err)
	}

	// Original typed error unwrappable. errors.As walks the chain.
	var fe *Error
	if !errors.As(err, &fe) {
		t.Fatal("ForcePinCampaign should preserve the typed *Error chain so the handler can map to a categorized response")
	}
	if fe.Code != "no_package_registered" {
		t.Errorf("preserved error should be no_package_registered, got %q", fe.Code)
	}
}

// noFoundryPackageReader stubs PackageReader for the no-package
// code path. ListPackages returns empty (so FindFoundryPackage
// returns nil), which causes SetPinnedVersion → ForcePinCampaign
// to return ErrNoPackageRegistered. The other two methods aren't
// called in this code path; nil returns are safe.
type noFoundryPackageReader struct{}

func (noFoundryPackageReader) ListPackages(_ context.Context) ([]packages.Package, error) {
	return nil, nil
}
func (noFoundryPackageReader) GetPackage(_ context.Context, _ string) (*packages.Package, error) {
	return nil, nil
}
func (noFoundryPackageReader) InstallDirForVersion(_ packages.PackageType, _, _ string) string {
	return ""
}
func (noFoundryPackageReader) ListVersions(_ context.Context, _ string) ([]packages.PackageVersion, error) {
	return nil, nil
}

// TestShowAffectedCampaignsOnClick_TargetsCorrectID — the banner's
// "Show affected campaigns" button must construct a DOM selector
// that matches the ID set by packages.templ's VersionList. Both
// sides sanitize version strings via the same rule (dots →
// hyphens). If either side drifts, the button silently fails to
// find its target.
func TestShowAffectedCampaignsOnClick_TargetsCorrectID(t *testing.T) {
	got := showAffectedCampaignsOnClick("v0.1.10").Call
	// Match the ID packages.templ produces for "v0.1.10":
	// fvtt-campaigns-trigger-v0-1-10
	if !strings.Contains(got, "fvtt-campaigns-trigger-v0-1-10") {
		t.Errorf("IIFE should query the version-sanitized ID; got:\n%s", got)
	}
	// IIFE must also include the scrollIntoView + click sequence
	// so the operator's UX (auto-scroll + auto-expand) works.
	if !strings.Contains(got, "scrollIntoView") {
		t.Error("IIFE must scrollIntoView the target so admins see the expansion")
	}
	if !strings.Contains(got, ".click()") {
		t.Error("IIFE must programmatically click the Campaigns expand button")
	}
}

// TestSanitizeVersionForDOMID_MatchesPackagesSanitizer pins the
// behavior to packages.sanitizeForID's rule (dot/plus/slash →
// hyphen). If packages's sanitizer changes, this test must be
// updated in lockstep + so must onclick_handlers.go's helper.
func TestSanitizeVersionForDOMID_MatchesPackagesSanitizer(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"v0.1.10", "v0-1-10"},
		{"1.2.3+build", "1-2-3-build"},
		{"v0.1.0-rc.1", "v0-1-0-rc-1"},
		{"plain", "plain"},
		{"", ""},
	}
	for _, tc := range cases {
		got := sanitizeVersionForDOMID(tc.in)
		if got != tc.want {
			t.Errorf("sanitizeVersionForDOMID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
