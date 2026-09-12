package entity_notes

import (
	"github.com/labstack/echo/v4"

	"github.com/keyxmakerx/chronicle/internal/plugins/addons"
	"github.com/keyxmakerx/chronicle/internal/plugins/auth"
	"github.com/keyxmakerx/chronicle/internal/plugins/campaigns"
)

// RegisterRoutes mounts the entity_notes REST endpoints. All routes
// require campaign membership at minimum (RolePlayer); audience checks
// further restrict what each viewer can see/write inside the service.
//
// Reads (GET): RolePlayer — the audience filter handles per-row visibility.
// Writes (POST/PUT/DELETE): RolePlayer — the audience checks handle role
// gates per-write (e.g., players can author private/everyone/custom but
// not dm_only).
//
// Gated on the "player-notes" addon (ADR-056): this is a FEATURE toggle, not
// an integration one, so — unlike sync-api's session short-circuit — off
// means off for every caller, with no first-party exception. Before this,
// disabling the addon only hid the dashboard block
// (entities/block_registry_core.go); every route here stayed reachable to
// any campaign member.
func RegisterRoutes(e *echo.Echo, h *Handler, campaignSvc campaigns.CampaignService, authSvc auth.AuthService, addonSvc addons.AddonService) {
	g := e.Group("/campaigns/:id",
		auth.RequireAuth(authSvc),
		campaigns.RequireCampaignAccess(campaignSvc),
		addons.RequireAddon(addonSvc, "player-notes"),
		campaigns.RequireRole(campaigns.RolePlayer),
	)
	g.GET("/entities/:eid/notes", h.List)
	g.POST("/entities/:eid/notes", h.Create)
	g.GET("/entities/:eid/notes/:nid", h.Get)
	g.PUT("/entities/:eid/notes/:nid", h.Update)
	g.DELETE("/entities/:eid/notes/:nid", h.Delete)
}
