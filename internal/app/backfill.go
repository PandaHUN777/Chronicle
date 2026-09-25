// Package app wires together all application dependencies.
//
// This file holds one-time, idempotent startup backfills: data fix-ups that
// replay an addon's enable-effects for campaigns that enabled it before the
// effect existed. They run through the owning services, never hand-rolled
// SQL, so they stay safe to run on every boot.
package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/keyxmakerx/chronicle/internal/plugins/entities"
)

// pcBackfillAddons is the slice of the addon service the player-character-type
// backfill needs (narrowed for testability).
type pcBackfillAddons interface {
	ListCampaignsUsingAddon(ctx context.Context, addonSlug string) ([]string, error)
}

// pcBackfillEntities is the slice of the entity service the backfill needs.
type pcBackfillEntities interface {
	EnsurePlayerCharacterType(ctx context.Context, campaignID string) error
}

// backfillPlayerCharacterTypes ensures the claimable "Player Character"
// sub-type is present and nested under the default "Characters" category for
// every campaign with the Player Character Claiming addon enabled — heals
// campaigns that enabled the addon before ApplyAddonEnableEffects created
// this type, since that hook only fires on a fresh enable.
//
// EnsurePlayerCharacterType is idempotent (re-parents, creates, or no-ops as
// needed), so this is safe to run on every boot. Per-campaign failures are
// logged and skipped rather than aborting the sweep. Returns the number of
// campaigns processed without error.
func backfillPlayerCharacterTypes(ctx context.Context, addonSvc pcBackfillAddons, entitySvc pcBackfillEntities) (int, error) {
	campaignIDs, err := addonSvc.ListCampaignsUsingAddon(ctx, entities.AddonPlayerCharacterClaiming)
	if err != nil {
		return 0, fmt.Errorf("listing campaigns with the claiming addon: %w", err)
	}

	processed := 0
	for _, campaignID := range campaignIDs {
		if err := entitySvc.EnsurePlayerCharacterType(ctx, campaignID); err != nil {
			slog.Warn("player-character-type backfill: ensure failed for campaign",
				slog.String("campaign_id", campaignID),
				slog.Any("error", err),
			)
			continue
		}
		processed++
	}
	return processed, nil
}
