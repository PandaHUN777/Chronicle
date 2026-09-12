// dm_grant_dashboard_test.go — ADR-057 slice 2 (P1FIX dispatch, review
// finding on the slice-1 commit).
//
// Show and OwnerDashboard both fetch the dashboard's "recently updated
// entities" list via h.recentLister.ListRecentForDashboard(ctx, campaignID,
// int(cc.MemberRole), userID, 8) — the ONLY visibility gate on that list (it
// threads straight through to entities' repository visibilityFilter, the
// same rule CheckEntityAccess and GetChildren use). A Co-DM (Player + DM
// grant) already sees dm_only entities in the entities plugin's own list and
// category views (VisibilityRole()-gated), so the dashboard passing the raw
// MemberRole instead disagreed with itself: dm_only entities the Co-DM can
// open directly vanish from the "recently updated" dashboard widget.
//
// This test does not render the full dashboard page (CampaignShowPage pulls
// in sidebar/addon/system wiring unrelated to this fix and isn't the thing
// under test); it drives the real Show handler up to and including the
// ListRecentForDashboard call — the call this fix changes — and captures the
// role integer actually passed, recovering from any unrelated template panic
// so that capture still lands. this is not a stub of the rule under test:
// VisibilityRole()'s promotion is real production code (model.go), and the
// assertion is on the literal argument value the handler passed to an
// injected collaborator.
package campaigns

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/keyxmakerx/chronicle/internal/plugins/auth"
)

// dashboardStubCampaignService only needs GetPendingTransfer to not panic;
// Show's other CampaignService use (none, before the recentLister call) is
// never reached.
type dashboardStubCampaignService struct {
	CampaignService
}

func (dashboardStubCampaignService) GetPendingTransfer(_ context.Context, _ string) (*OwnershipTransfer, error) {
	return nil, nil
}

// roleCaptureRecentLister records the role argument ListRecentForDashboard
// was called with, so the test can assert on the actual value threaded
// through the handler rather than on behavior of a stub that encodes the
// promotion rule itself.
type roleCaptureRecentLister struct {
	called  bool
	gotRole int
}

func (r *roleCaptureRecentLister) ListRecentForDashboard(_ context.Context, _ string, role int, _ string, _ int) ([]RecentEntity, error) {
	r.called = true
	r.gotRole = role
	return nil, nil
}

func dashboardCoDmContext() *CampaignContext {
	return &CampaignContext{
		Campaign:    &Campaign{ID: "camp-1"},
		MemberRole:  RolePlayer,
		IsDmGranted: true,
		IsMember:    true,
	}
}

// callShowCapturingRole drives h.Show(c), recovering from any panic in the
// (unrelated) template render that follows the recentLister call, so the
// capture made before that render still stands.
func callShowCapturingRole(t *testing.T, h *Handler, c echo.Context) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Show panicked during unrelated template rendering (expected in this minimal fixture): %v", r)
		}
	}()
	_ = h.Show(c)
}

// TestShow_RecentEntities_UsesPromotedVisibilityRole pins the dashboard half
// of the ADR-057 review finding: the "recently updated" entity list must be
// gated on cc.VisibilityRole() (Owner=3 for a Co-DM), not the raw
// cc.MemberRole (Player=1), so it agrees with what the Co-DM can already open
// directly.
func TestShow_RecentEntities_UsesPromotedVisibilityRole(t *testing.T) {
	h := NewHandler(dashboardStubCampaignService{})
	capture := &roleCaptureRecentLister{}
	h.SetRecentEntityLister(capture)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/campaigns/camp-1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("camp-1")
	c.Set(contextKeyCampaign, dashboardCoDmContext())
	auth.SetSession(c, &auth.Session{UserID: "codm-1"})

	callShowCapturingRole(t, h, c)

	if !capture.called {
		t.Fatal("Show never called ListRecentForDashboard — cannot assert on the role it would have passed")
	}
	if capture.gotRole != int(RoleOwner) {
		t.Errorf("Show must call ListRecentForDashboard with the Co-DM's promoted VisibilityRole (Owner=%d), got role=%d (raw MemberRole=%d) — "+
			"a Co-DM would see dm_only entities vanish from the dashboard's recently-updated list even though they can open them directly",
			int(RoleOwner), capture.gotRole, int(RolePlayer))
	}
}
