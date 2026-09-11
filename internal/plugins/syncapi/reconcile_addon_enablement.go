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
// way. It returns the number of campaigns it enabled.
//
// WHY IT EXISTS. RequireSyncAPIAddon and AuthenticateKeyForWS now enforce the
// toggle, and enforcement defaults to DENIED: addons.IsEnabledForCampaign
// returns false when no campaign_addons row exists, and the only routine
// writers of that row are the manual UI toggle and (as of this change)
// CreateKey. A one-time plugin migration —
// syncapi/migrations/003_autoenable_existing_keys.up.sql — backfilled
// enabled=1 for campaigns that had an api_keys row, but it ran once, years of
// keys ago. Any campaign that minted its first key after that migration
// applied sits at "no row" and would have been cut off the instant
// enforcement went live. On the deployment this ships to, that includes a
// live Foundry VTT sync.
//
// WHY A RECONCILER AND NOT A MIGRATION. CLAUDE.md: migrations are
// APPEND-ONLY and SCHEMA-ONLY; one-time data fixes are idempotent
// reconcilers. It also has to be re-runnable — a campaign can acquire its
// first key between two boots of an older build.
//
// THE RULE, AND THE TRAP IT AVOIDS. Enable only where NO ROW EXISTS:
//
//	keys + no row      → enable   (never configured; it was working before,
//	                               so keep it working)
//	keys + row(0)      → SKIP     (the owner switched it off on purpose)
//	keys + row(1)      → skip     (already on; nothing to do)
//	no keys            → skip     (not a Sync API campaign; don't opt it in)
//
// The migration this replaces used `ON DUPLICATE KEY UPDATE enabled = 1`,
// which was harmless as a one-shot back when "enabled" meant nothing. As a
// BOOT reconciler that same clause would re-enable every campaign with keys
// on every restart, so switching the toggle off would last exactly until the
// next deploy — it would hand back the decorative toggle this change is
// removing. Hence HasCampaignAddonRecord: "has anyone decided?", not "is it
// on?". IsEnabledForCampaign cannot tell "never configured" from "explicitly
// off" and is the wrong question here.
//
// Best-effort by contract: it reports its error to the caller, which logs and
// continues. A backfill must not be able to stop the server from booting.
func ReconcileAddonEnablement(ctx context.Context, keys CampaignKeyLister, store AddonEnablementStore) (int, error) {
	if keys == nil || store == nil {
		return 0, fmt.Errorf("syncapi.ReconcileAddonEnablement: nil dependency (keys=%v store=%v)",
			keys != nil, store != nil)
	}

	campaignIDs, err := keys.ListCampaignIDsWithKeys(ctx)
	if err != nil {
		return 0, fmt.Errorf("syncapi.ReconcileAddonEnablement: listing campaigns with api keys: %w. "+
			"Campaigns that already use the Sync API may be refused until this is resolved or an "+
			"owner enables Sync API in Settings › Extensions", err)
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
