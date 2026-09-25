package foundry_vtt

import (
	"context"

	"github.com/keyxmakerx/chronicle/internal/plugins/packages"
)

// AutoPinHook implements packages.PostInstallHook, pinning DB state
// (auto-tracking campaigns) alongside the separate on-disk
// version-rewrite hook. The two hooks are independent; either
// failing fails the install. Running order is registration order —
// see routes.go.
type AutoPinHook struct {
	svc Service
}

// NewAutoPinHook constructs the hook, taking the service that holds
// the repo, settings and events dependencies for AutoPinOnInstall.
func NewAutoPinHook(svc Service) *AutoPinHook {
	return &AutoPinHook{svc: svc}
}

// PackageType identifies which package type this hook fires on.
func (h *AutoPinHook) PackageType() packages.PackageType {
	return packages.PackageTypeFoundryModule
}

// AfterInstall pins every auto-tracking campaign to previousVersion so
// they stay on the version they were effectively running before this
// install changed InstalledVersion; the admin sees the version spread
// in the "Campaigns Using v0.X.Y" admin UI and can bump per campaign.
//
// Errors fail the install loudly: a post-install failure leaves the
// catalog state pristine (destDir cleaned by packages.InstallVersion)
// rather than producing a half-applied state.
//
// actor* are recorded in security_events for the audit trail but are
// currently always empty strings — the admin handler that triggers
// the install doesn't thread actor info through to the hook context.
func (h *AutoPinHook) AfterInstall(ctx context.Context, pkg *packages.Package, version, previousVersion, destDir string) error {
	_ = pkg     // not needed; service infers the foundry-module package itself
	_ = destDir // on-disk path is the PostInstallHook's domain
	_, err := h.svc.AutoPinOnInstall(ctx, previousVersion, version, "", "", "")
	return err
}
