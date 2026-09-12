// codm_visibility_test.go pins the operator's 2026-09-12 ruling (.ai/todo.md,
// "RULED 2026-09-12 by the operator: YES, the co-DM promotion crosses plugin
// lines") against effectiveRole, the one role-selection function every
// timeline content-filtering call site (Index, Show, and friends) goes
// through.
//
// This one is NOT a plain swap: effectiveRole must keep its existing
// view-as-player branch returning RolePlayer unconditionally — an Owner
// deliberately previewing the player experience must still see the PLAYER
// view even though VisibilityRole() would otherwise promote them (Owners
// already sit at RoleOwner, and a co-DM Owner-preview case would promote
// right back if the preview branch didn't come first). Only the non-preview
// branch promotes a co-DM to cc.VisibilityRole().
//
// TEST HONESTY: effectiveRole is a pure function of (view-as-player flag,
// CampaignContext) with no I/O, so this test exercises the real function
// directly — not a mock standing in for it. It fully proves the role
// SELECTION logic; it does not by itself prove every timeline SQL/service
// path correctly narrows content for that role (that is service.go /
// repository.go's own coverage).
package timeline

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/keyxmakerx/chronicle/internal/permissions"
	"github.com/keyxmakerx/chronicle/internal/plugins/campaigns"
	"github.com/keyxmakerx/chronicle/internal/templates/layouts"
)

// newTimelineTestContext builds an echo.Context whose request context carries
// the view-as-player flag, matching how the real middleware/layouts wiring
// stores it (layouts.SetViewingAsPlayer).
func newTimelineTestContext(viewingAsPlayer bool) echo.Context {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/campaigns/camp-1/timelines", nil)
	ctx := layouts.SetViewingAsPlayer(context.Background(), viewingAsPlayer)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec)
}

// TestEffectiveRole_CoDMIsPromotedToOwnerForVisibility is the RED/GREEN case:
// outside view-as-player mode, a co-DM (MemberRole=Player, IsDmGranted=true)
// must resolve to Owner, matching cc.VisibilityRole() and every entity path.
// Before the fix, effectiveRole's non-preview branch returned the raw
// int(cc.MemberRole) == RolePlayer, so a co-DM's timeline view was narrowed
// exactly like a plain Player's and hid dm_only events.
func TestEffectiveRole_CoDMIsPromotedToOwnerForVisibility(t *testing.T) {
	cc := &campaigns.CampaignContext{
		Campaign:    &campaigns.Campaign{ID: "camp-1"},
		MemberRole:  campaigns.RolePlayer,
		IsDmGranted: true,
	}
	c := newTimelineTestContext(false)

	got := effectiveRole(c, cc)
	if got != int(permissions.RoleOwner) {
		t.Errorf("effectiveRole for a co-DM (not previewing) = %d, want %d (permissions.RoleOwner via cc.VisibilityRole()) — "+
			"a co-DM must see dm_only timeline events like an Owner does",
			got, int(permissions.RoleOwner))
	}
}

// TestEffectiveRole_OwnerViewAsPlayerStillReturnsPlayer is the regression
// that matters most here: an Owner (co-DM or not) previewing the player
// experience must STILL get RolePlayer, never promoted back up. This must
// pass BOTH before and after the fix — the fix must compose with, not
// override, the existing preview branch.
func TestEffectiveRole_OwnerViewAsPlayerStillReturnsPlayer(t *testing.T) {
	cc := &campaigns.CampaignContext{
		Campaign:   &campaigns.Campaign{ID: "camp-1"},
		MemberRole: campaigns.RoleOwner,
	}
	c := newTimelineTestContext(true)

	got := effectiveRole(c, cc)
	if got != int(campaigns.RolePlayer) {
		t.Errorf("effectiveRole for an Owner in view-as-player mode = %d, want %d (campaigns.RolePlayer) — "+
			"view-as-player must still show the PLAYER view",
			got, int(campaigns.RolePlayer))
	}
}

// TestEffectiveRole_CoDMViewAsPlayerStillReturnsPlayer covers the specific
// composition the task called out: a co-DM (who WOULD be promoted outside
// preview mode) previewing as a player must also see the PLAYER view, not
// their promoted Owner-for-visibility role.
func TestEffectiveRole_CoDMViewAsPlayerStillReturnsPlayer(t *testing.T) {
	cc := &campaigns.CampaignContext{
		Campaign:    &campaigns.Campaign{ID: "camp-1"},
		MemberRole:  campaigns.RolePlayer,
		IsDmGranted: true,
	}
	c := newTimelineTestContext(true)

	got := effectiveRole(c, cc)
	if got != int(campaigns.RolePlayer) {
		t.Errorf("effectiveRole for a co-DM in view-as-player mode = %d, want %d (campaigns.RolePlayer) — "+
			"the preview branch must win over the visibility promotion",
			got, int(campaigns.RolePlayer))
	}
}

// TestEffectiveRole_PlainPlayerIsNotPromoted is the negative control: a
// Player with no DM grant, not previewing, must still resolve to RolePlayer.
func TestEffectiveRole_PlainPlayerIsNotPromoted(t *testing.T) {
	cc := &campaigns.CampaignContext{
		Campaign:   &campaigns.Campaign{ID: "camp-1"},
		MemberRole: campaigns.RolePlayer,
	}
	c := newTimelineTestContext(false)

	got := effectiveRole(c, cc)
	if got != int(campaigns.RolePlayer) {
		t.Errorf("effectiveRole for a plain Player = %d, want %d (campaigns.RolePlayer, unpromoted)",
			got, int(campaigns.RolePlayer))
	}
}

// TestEffectiveRole_OwnerNotPreviewingIsUnaffected pins that an actual Owner
// (not previewing) is unaffected by the fix — cc.VisibilityRole() for an
// Owner is already RoleOwner, so this must pass both before and after.
func TestEffectiveRole_OwnerNotPreviewingIsUnaffected(t *testing.T) {
	cc := &campaigns.CampaignContext{
		Campaign:   &campaigns.Campaign{ID: "camp-1"},
		MemberRole: campaigns.RoleOwner,
	}
	c := newTimelineTestContext(false)

	got := effectiveRole(c, cc)
	if got != int(campaigns.RoleOwner) {
		t.Errorf("effectiveRole for an Owner (not previewing) = %d, want %d (campaigns.RoleOwner)",
			got, int(campaigns.RoleOwner))
	}
}
