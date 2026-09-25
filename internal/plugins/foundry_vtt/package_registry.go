// package_registry.go is the central lookup for the "one
// foundry-module per Chronicle instance" assumption: the DB query
// (FoundryPackage) and the DOM constants packages.templ targets for
// the per-version "Campaigns" button and the auto-pin banner's
// "Versions" button trigger. packages.templ inlines the string
// values for templ-render reasons (cross-plugin imports point only
// foundry_vtt → packages) but cross-references the constants here.
// A future "support multiple foundry-module packages" change only
// needs to revise this file's lookup.
//
// The registry re-queries on every call; no cache yet.

package foundry_vtt

import (
	"context"
	"strings"

	"github.com/keyxmakerx/chronicle/internal/plugins/packages"
)

// FvttVersionsTriggerAttr is the data attribute packages.templ stamps
// on the foundry-module package's "Versions" button so the auto-pin
// banner's IIFE can locate it. Exported so packages.templ can
// reference the constant instead of duplicating the string.
const FvttVersionsTriggerAttr = "data-fvtt-versions-trigger"

// FvttCampaignsTriggerIDPrefix is the prefix packages.templ uses for
// the per-version "Campaigns" expand button. The full ID is
// `<prefix><sanitized-version>` — see FvttCampaignsTriggerID.
const FvttCampaignsTriggerIDPrefix = "fvtt-campaigns-trigger-"

// FvttCampaignsTriggerID returns the canonical DOM ID for the
// per-version "Campaigns" expand button. The sanitization rule (dots
// / plus / slash → hyphen) MUST match packages.sanitizeForID; if
// either drifts, the auto-pin banner's IIFE silently fails to find
// its target. Cross-tested in onclick_handlers_test.go.
func FvttCampaignsTriggerID(version string) string {
	return FvttCampaignsTriggerIDPrefix + sanitizeVersionForDOMID(version)
}

// PackageRegistry centralizes the "one foundry-module per Chronicle"
// assumption. All foundry_vtt code that needs to find the
// foundry-module package goes through FoundryPackage.
// FindFoundryPackage is a thin wrapper kept for existing call sites;
// new code should use the registry directly.
type PackageRegistry struct {
	pkgs PackageReader
}

// NewPackageRegistry constructs a registry backed by the given
// PackageReader, sharing the service's existing packages adapter.
func NewPackageRegistry(pkgs PackageReader) *PackageRegistry {
	return &PackageRegistry{pkgs: pkgs}
}

// FoundryPackage returns the first foundry-module-typed package the
// catalog returns; assumes a single foundry-module package per
// Chronicle instance (see file header). Returns (nil, nil) when none
// exists — callers treat that as the "no package registered" state.
func (r *PackageRegistry) FoundryPackage(ctx context.Context) (*packages.Package, error) {
	if r == nil || r.pkgs == nil {
		return nil, nil
	}
	all, err := r.pkgs.ListPackages(ctx)
	if err != nil {
		return nil, ErrInternal("list_packages", err)
	}
	for i := range all {
		if all[i].Type == packages.PackageTypeFoundryModule {
			return &all[i], nil
		}
	}
	return nil, nil
}

// FoundryPackageID returns just the package ID of the foundry-module
// package, or "" if none exists. Errors are swallowed to "" since
// templ code renders the "no package registered" empty state on an
// empty ID anyway.
func (r *PackageRegistry) FoundryPackageID(ctx context.Context) string {
	pkg, err := r.FoundryPackage(ctx)
	if err != nil || pkg == nil {
		return ""
	}
	return pkg.ID
}

// Invalidate is a no-op extension point: a future cache layer can
// wire this to a TTL or the packages plugin's post-install hook
// without requiring call sites to change.
func (r *PackageRegistry) Invalidate() {
	// Intentional no-op; placeholder for future cache layer.
}

// sanitizeVersionForDOMID lives in onclick_handlers.go; declared
// here as a documentation pointer. Both sides MUST stay in lock-step
// with packages.sanitizeForID — pinned by onclick_handlers_test.go.
var _ = sanitizeVersionForDOMID // ensure the helper stays linked even if all call sites move

var _ = strings.Builder{}
