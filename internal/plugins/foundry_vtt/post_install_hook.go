package foundry_vtt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/keyxmakerx/chronicle/internal/plugins/packages"
)

// PostInstallHook implements packages.PostInstallHook for
// PackageTypeFoundryModule. After the packages plugin extracts a
// foundry-module install, this hook loads chronicle-package.json (or
// applies defaults when absent, but fails loudly if present and
// invalid) and rewrites the version field in the on-disk module.json
// so the served manifest reflects the installed version, not the
// upstream GitHub release's stale version string.
//
// It does NOT rewrite the manifest/download URL fields at
// install-time — those are rewritten per-request at serve-time by
// BuildManifestForCampaign so the URL stays per-campaign and
// per-token-version. Registered at boot via
// packages.RegisterPostInstallHook.
type PostInstallHook struct{}

// NewPostInstallHook constructs the hook. Stateless; kept as a
// constructor for symmetry with the rest of the plugin.
func NewPostInstallHook() *PostInstallHook {
	return &PostInstallHook{}
}

// PackageType identifies which package type this hook handles.
func (h *PostInstallHook) PackageType() packages.PackageType {
	return packages.PackageTypeFoundryModule
}

// AfterInstall is called by the packages plugin after a foundry-
// module package's zip is extracted and the DB row updated. Errors
// here fail the install and cause cleanup of destDir, so the operator
// sees the failure immediately rather than a stale served version.
func (h *PostInstallHook) AfterInstall(ctx context.Context, pkg *packages.Package, version, previousVersion, destDir string) error {
	_ = previousVersion // not needed here; AutoPinHook consumes it.
	// A missing descriptor is the normal fallback path; a
	// present-but-invalid one is an upstream packaging bug we fail
	// loudly on. `desc` is populated regardless — loadDescriptor
	// returns defaultDescriptor() alongside errDescriptorNotFound.
	desc, err := loadDescriptor(destDir)
	if err != nil && !errors.Is(err, errDescriptorNotFound) {
		return err
	}

	manifestPath := filepath.Join(destDir, desc.Package.ModuleJSONPath)

	if err := rewriteModuleJSONVersion(manifestPath, version); err != nil {
		return ErrModuleJSONMissing(manifestPath, err)
	}
	return nil
}

// rewriteModuleJSONVersion reads module.json, sets its "version"
// field to the installed version string, and writes the file back,
// preserving every other field (round-trips via map[string]any).
// The upstream GitHub release zip ships a module.json with a stale
// version string baked in, so the served manifest would otherwise
// disagree with the DB-tracked version.
//
// Uses os.WriteFile (not rename): no concurrent reader holds the
// manifest, since the serve handler reads it, not this hook.
func rewriteModuleJSONVersion(path, version string) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(bytes, &m); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	m["version"] = version
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal rewritten manifest: %w", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
