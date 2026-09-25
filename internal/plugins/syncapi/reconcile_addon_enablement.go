package syncapi

import (
	"context"
	"fmt"
	"log/slog"
)

// reconcileEnabledBy is the campaign_addons.enabled_by value recorded for a
// row this reconciler creates. No human made this decision — the campaign was
// already using the Sync API before the toggle was enforced — and attributing
// it to a user id would be a fabricated audit trail.
const reconcileEnabledBy = ""

// CampaignKeyLister reads which campaigns own API keys. Satisfied by
// SyncAPIService; narrowed here so the reconciler states exactly what it uses.
type CampaignKeyLister interface {
	ListCampaignIDsWithKeys(ctx context.Context) ([]string, error)
}

// AddonEnablementStore is the addons-service surface the reconciler needs.
// Satisfied by addons.AddonService. Note HasCampaignAddonRecord, NOT
// IsEnabledForCampaign: the difference between the two is the whole safety
// argument below.
type AddonEnablementStore interface {
	HasCampaignAddonRecord(ctx context.Context, campaignID string, addonSlug string) (bool, error)
	EnableForCampaignBySlug(ctx context.Context, campaignID string, addonSlug string, userID string) error
}

// ReconcileAddonEnablement enables the "Sync API" addon for every campaign
// that already owns an API key but has never had the toggle recorded either
// way (enforcement defaults to DENIED when no campaign_addons row exists).
// It returns the number of campaigns it enabled. Runs as an idempotent
// reconciler, not a migration, so it is safe to re-run every boot.
//
// Enable only where NO ROW EXISTS:
//
//	keys + no row  → enable  (never configured; keep it working)
//	keys + row(0)  → skip    (owner switched it off on purpose)
//	keys + row(1)  → skip    (already on)
//	no keys        → skip    (not a Sync API campaign)
//
// Hence HasCampaignAddonRecord ("has anyone decided?"), never
// IsEnabledForCampaign, which can't distinguish "never configured" from
// "explicitly off".
//
// Best-effort by contract: it reports its error to the caller, which logs
// and continues. A backfill must not be able to stop the server from booting.
func ReconcileAddonEnablement(ctx context.Context, keys CampaignKeyLister, store AddonEnablementStore) (int, error) {
	if keys == nil || store == nil {
		return 0, fmt.Errorf("syncapi.ReconcileAddonEnablement: nil dependency (keys=%v store=%v)",
			keys != nil, store != nil)
	}

	campaignIDs, err := keys.ListCampaignIDsWithKeys(ctx)
	if err != nil {
		return 0, fmt.Errorf("syncapi.ReconcileAddonEnablement: listing campaigns with api keys: %w. "+
			"Campaigns that already use the Sync API may be refused until this is resolved or an "+
			"owner enables Sync API on the campaign's Extensions page (sidebar → Extensions)", err)
	}

	enabled := 0
	for _, campaignID := range campaignIDs {
		if campaignID == "" {
			continue
		}
		has, err := store.HasCampaignAddonRecord(ctx, campaignID, SyncAPIAddonSlug)
		if err != nil {
			return enabled, fmt.Errorf("syncapi.ReconcileAddonEnablement: reading addon record for campaign %s: %w",
				campaignID, err)
		}
		if has {
			// Either already enabled, or deliberately disabled. Both are
			// decisions; neither is ours to overwrite.
			continue
		}
		if err := store.EnableForCampaignBySlug(ctx, campaignID, SyncAPIAddonSlug, reconcileEnabledBy); err != nil {
			return enabled, fmt.Errorf("syncapi.ReconcileAddonEnablement: enabling sync-api for campaign %s: %w",
				campaignID, err)
		}
		enabled++
		slog.Info("sync-api addon enabled for a campaign that already had API keys "+
			"but no recorded toggle state",
			slog.String("campaign_id", campaignID),
		)
	}
	return enabled, nil
}
