// extensions_card.go lets the campaigns plugin's top-level Extensions hub
// (`GET /campaigns/:id/extensions`) embed the per-campaign Content Packs
// list as a card inside its own chrome, via campaigns' ContentPacksCardRenderer
// interface — this inverts the import direction so extensions imports
// campaigns, never the reverse.
//
// Per-pack enable/disable POSTs at /campaigns/:id/extensions/:extID/
// {enable,disable} still render campaignExtensionListFragment for in-place
// HTMX swap.

package extensions

import (
	"context"

	"github.com/a-h/templ"

	"github.com/keyxmakerx/chronicle/internal/plugins/campaigns"
)

// RenderCampaignExtensionList loads the per-campaign installed Content
// Packs and returns the list-fragment templ component for the campaigns
// plugin to embed in its Extensions hub. Implements
// `campaigns.ContentPacksCardRenderer`. No packs installed renders an empty
// list card; an error bubbles up so the hub can degrade gracefully
// (rendering without the Content Packs card).
func (h *Handler) RenderCampaignExtensionList(ctx context.Context, cc *campaigns.CampaignContext) (templ.Component, error) {
	exts, err := h.svc.ListForCampaign(ctx, cc.Campaign.ID)
	if err != nil {
		return nil, err
	}
	if exts == nil {
		exts = []CampaignExtension{}
	}
	return campaignExtensionListFragment(cc, exts), nil
}
