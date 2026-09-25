package foundry_vtt

import (
	"context"
	"fmt"
	"log/slog"
)

// AutoPinMigrationSettingKey tracks whether the one-time auto-pin
// migration has completed. Value is the version string used as the
// pin target; present means the migration ran, absent means it hasn't.
const AutoPinMigrationSettingKey = "foundry_vtt.autopin_migration_completed_for_version"

// SettingsKVStore is the narrow contract AutoPinMigrate needs:
// Get/Set on string keys. Implemented by settings.SettingsRepository
// in production; kept local so the migration can be tested without
// importing settings.
type SettingsKVStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

// AutoPinMigrate runs a one-time migration: every campaign with an
// empty foundry_module_pin is pinned to the currently-installed
// foundry-module version, so the next install triggers the
// AutoPinHook flow (preserves state, notifies admin) instead of
// silently bumping them.
//
// Idempotent via AutoPinMigrationSettingKey; clearing that key
// manually re-runs the migration. Must run after plugin migrations
// (schema must exist) and before the HTTP server accepts traffic.
//
// Returns nil without setting the flag (retries next boot) if no
// foundry-module package is registered or it has no installed
// version. Returns an error (aborts startup) on settings or query
// failure. Per-campaign pin failures are logged and skipped; the
// flag is still set on completion since the migration is best-effort
// and missed campaigns can be re-pinned via the admin UI.
func AutoPinMigrate(ctx context.Context, svc Service, settings SettingsKVStore) error {
	// settings.Get returns "" both when the key is absent and on a
	// read error; either way we proceed rather than abort startup —
	// worst case the migration re-runs, which is a no-op since
	// CampaignsWithEmptyPin only returns un-pinned rows.
	existing, _ := settings.Get(ctx, AutoPinMigrationSettingKey)
	if existing != "" {
		slog.Info("foundry_vtt autopin migration: already completed",
			slog.String("pinned_to", existing))
		return nil
	}

	// No-op if the foundry-module package isn't registered yet or has
	// no installed version: there's nothing to pin campaigns to.
	pkg, err := svc.FindFoundryPackage(ctx)
	if err != nil {
		return fmt.Errorf("foundry_vtt.AutoPinMigrate: find foundry package: %w", err)
	}
	if pkg == nil {
		slog.Info("foundry_vtt autopin migration: no foundry-module package registered, skipping (will retry on next boot)")
		return nil
	}
	if pkg.InstalledVersion == "" {
		slog.Info("foundry_vtt autopin migration: foundry-module package has no installed version, skipping")
		return nil
	}

	// Unlike AutoPinOnInstall, this pins even when previous==new: the
	// migration's job is to make the effective pin explicit for every
	// campaign, and it logs under EventModuleAutoPinMigration so the
	// audit trail distinguishes it from a per-install auto-pin.
	count, err := svc.MigrateAutoPinToVersion(ctx, pkg.InstalledVersion)
	if err != nil {
		return fmt.Errorf("foundry_vtt.AutoPinMigrate: iterate campaigns: %w", err)
	}

	// Set the completion flag so subsequent boots skip the migration.
	if err := settings.Set(ctx, AutoPinMigrationSettingKey, pkg.InstalledVersion); err != nil {
		return fmt.Errorf("foundry_vtt.AutoPinMigrate: write completion flag: %w", err)
	}

	slog.Info("foundry_vtt autopin migration: completed",
		slog.Int("campaigns_pinned", count),
		slog.String("version", pkg.InstalledVersion))
	return nil
}

