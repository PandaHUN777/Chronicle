// dm_only_leak_test.go — 2026-09-12 security audit finding 4 (HIGH).
//
// THE BUG THESE PIN. Show, TimelineDataAPI and EmbedTimeline gated only on
// requireTimelineInCampaign — which checks that a timeline id belongs to the
// campaign named in the URL and nothing else. None of the three ever applied
// the timeline's own visibility ('dm_only' base visibility, or its per-user
// visibility_rules) the way Index/ListTimelines already does via
// filterTimelinesByUser. So on a PUBLIC campaign a viewer with no account at
// all (permissions.RoleNone, empty user id) could open any timeline by id —
// dm_only included — through any of these three routes and read it in full.
//
// THE FIX pins here: GetTimelineForViewer (service.go) applies the exact same
// predicate ListTimelines' filterTimelinesByUser already applies —
// timelineVisibleToViewer, factored out so the two paths cannot drift the way
// List and Show did (ADR-058; see internal/app/map_audience_parity_test.go
// for the precedent this mirrors). A viewer who may not see the timeline gets
// NotFound, never Forbidden: a Forbidden on a public campaign would confirm
// the id exists (ADR-055 rule 3 — the same existence-oracle problem a body
// leak is, in a different costume).
//
// Every anonymous/Player assertion below is paired with an Owner and a co-DM
// control, and one Owner-view-as-player control: a "fix" that simply hides
// the timeline from everyone, or that regresses the 4df13033 co-DM
// promotion, would pass the first half and fail these.
package timeline

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/keyxmakerx/chronicle/internal/permissions"
	"github.com/keyxmakerx/chronicle/internal/plugins/campaigns"
	"github.com/keyxmakerx/chronicle/internal/templates/layouts"
)

// secretTimeline is the dm_only fixture every case in this file reads.
func secretTimeline() *Timeline {
	return &Timeline{
		ID:         "tl-secret",
		CampaignID: "camp-1",
		Name:       "The Real Plan Behind The Curtain",
		Visibility: "dm_only",
		Color:      "#112233",
		Icon:       "fa-timeline",
	}
}

// newDMOnlyRepo returns a mock repo serving only secretTimeline(), by id,
// scoped to camp-1 — every other repo call defaults to nil/empty (the
// mockTimelineRepo zero-value behavior), which is enough for Show and
// TimelineDataAPI to render without needing linked events, groups or
// connections to exist.
func newDMOnlyRepo() *mockTimelineRepo {
	return &mockTimelineRepo{
		getByIDFn: func(_ context.Context, id string) (*Timeline, error) {
			if id == "tl-secret" {
				return secretTimeline(), nil
			}
			return nil, nil
		},
	}
}

// tlTestReq builds an echo.Context carrying a CampaignContext, an optional
// authenticated user id, and the view-as-player flag — the same three
// ingredients effectiveRole and GetTimelineForViewer read in production,
// assembled the way this package's own tests already do
// (audit_log_test.go's newReqWithCC, codm_visibility_test.go's
// newTimelineTestContext) rather than a shape invented for this file alone.
func tlTestReq(path string, cc *campaigns.CampaignContext, userID string, viewAsPlayer bool, htmx bool) echo.Context {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	ctx := layouts.SetViewingAsPlayer(req.Context(), viewAsPlayer)
	req = req.WithContext(ctx)
	if htmx {
		req.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id", "tid")
	c.SetParamValues("camp-1", "tl-secret")
	c.Set("campaign_context", cc)
	if userID != "" {
		c.Set("auth_user_id", userID)
	}
	return c
}

// --- Viewer fixtures ---

func anonCampaignCtx() *campaigns.CampaignContext {
	return &campaigns.CampaignContext{
		Campaign:   &campaigns.Campaign{ID: "camp-1", Name: "Public Campaign", IsPublic: true},
		MemberRole: campaigns.RoleNone,
	}
}

func playerCampaignCtx() *campaigns.CampaignContext {
	return &campaigns.CampaignContext{
		Campaign:   &campaigns.Campaign{ID: "camp-1", Name: "Public Campaign", IsPublic: true},
		MemberRole: campaigns.RolePlayer,
	}
}

func ownerCampaignCtx() *campaigns.CampaignContext {
	return &campaigns.CampaignContext{
		Campaign:   &campaigns.Campaign{ID: "camp-1", Name: "Public Campaign", IsPublic: true},
		MemberRole: campaigns.RoleOwner,
	}
}

func coDMCampaignCtx() *campaigns.CampaignContext {
	return &campaigns.CampaignContext{
		Campaign:    &campaigns.Campaign{ID: "camp-1", Name: "Public Campaign", IsPublic: true},
		MemberRole:  campaigns.RolePlayer,
		IsDmGranted: true,
	}
}

// --- Show ---

func TestShow_Anonymous_DMOnlyTimeline_NotFound(t *testing.T) {
	h := NewHandler(newTestTimelineService(newDMOnlyRepo()))
	c := tlTestReq("/campaigns/camp-1/timelines/tl-secret", anonCampaignCtx(), "", false, false)

	err := h.Show(c)
	assertAppError(t, err, http.StatusNotFound)
}

func TestShow_PlainPlayer_DMOnlyTimeline_NotFound(t *testing.T) {
	h := NewHandler(newTestTimelineService(newDMOnlyRepo()))
	c := tlTestReq("/campaigns/camp-1/timelines/tl-secret", playerCampaignCtx(), "u-player", false, false)

	err := h.Show(c)
	assertAppError(t, err, http.StatusNotFound)
}

func TestShow_Owner_DMOnlyTimeline_Succeeds(t *testing.T) {
	h := NewHandler(newTestTimelineService(newDMOnlyRepo()))
	c := tlTestReq("/campaigns/camp-1/timelines/tl-secret", ownerCampaignCtx(), "u-owner", false, true)

	if err := h.Show(c); err != nil {
		t.Fatalf("Owner must be able to open a dm_only timeline: %v", err)
	}
	rec := c.Response().Writer.(*httptest.ResponseRecorder)
	if !strings.Contains(rec.Body.String(), secretTimeline().Name) {
		t.Error("Owner's rendered page did not contain the timeline name")
	}
}

func TestShow_CoDM_DMOnlyTimeline_Succeeds(t *testing.T) {
	h := NewHandler(newTestTimelineService(newDMOnlyRepo()))
	c := tlTestReq("/campaigns/camp-1/timelines/tl-secret", coDMCampaignCtx(), "u-codm", false, true)

	if err := h.Show(c); err != nil {
		t.Fatalf("a co-DM must see a dm_only timeline like an Owner does: %v", err)
	}
	rec := c.Response().Writer.(*httptest.ResponseRecorder)
	if !strings.Contains(rec.Body.String(), secretTimeline().Name) {
		t.Error("co-DM's rendered page did not contain the timeline name")
	}
}

// TestShow_OwnerViewAsPlayer_DMOnlyTimeline_NotFound is the composition the
// task called out explicitly: an Owner previewing the player experience must
// see exactly what a Player sees. The timeline itself is dm_only, so the
// PLAYER view of it is that it does not exist — same as
// TestShow_PlainPlayer_DMOnlyTimeline_NotFound. A "fix" that let
// view-as-player leak through the Owner's real role would fail this while
// passing the plain-anonymous/Player cases above.
func TestShow_OwnerViewAsPlayer_DMOnlyTimeline_NotFound(t *testing.T) {
	h := NewHandler(newTestTimelineService(newDMOnlyRepo()))
	c := tlTestReq("/campaigns/camp-1/timelines/tl-secret", ownerCampaignCtx(), "u-owner", true, false)

	err := h.Show(c)
	assertAppError(t, err, http.StatusNotFound)
}

// --- TimelineDataAPI ---

func TestTimelineDataAPI_Anonymous_DMOnlyTimeline_NotFound(t *testing.T) {
	h := NewHandler(newTestTimelineService(newDMOnlyRepo()))
	c := tlTestReq("/campaigns/camp-1/timelines/tl-secret/data", anonCampaignCtx(), "", false, false)

	err := h.TimelineDataAPI(c)
	assertAppError(t, err, http.StatusNotFound)
}

func TestTimelineDataAPI_PlainPlayer_DMOnlyTimeline_NotFound(t *testing.T) {
	h := NewHandler(newTestTimelineService(newDMOnlyRepo()))
	c := tlTestReq("/campaigns/camp-1/timelines/tl-secret/data", playerCampaignCtx(), "u-player", false, false)

	err := h.TimelineDataAPI(c)
	assertAppError(t, err, http.StatusNotFound)
}

func TestTimelineDataAPI_Owner_DMOnlyTimeline_Succeeds(t *testing.T) {
	h := NewHandler(newTestTimelineService(newDMOnlyRepo()))
	c := tlTestReq("/campaigns/camp-1/timelines/tl-secret/data", ownerCampaignCtx(), "u-owner", false, false)

	if err := h.TimelineDataAPI(c); err != nil {
		t.Fatalf("Owner must be able to fetch dm_only timeline data: %v", err)
	}
	rec := c.Response().Writer.(*httptest.ResponseRecorder)
	if !strings.Contains(rec.Body.String(), secretTimeline().Name) {
		t.Error("Owner's JSON response did not contain the timeline name")
	}
}

func TestTimelineDataAPI_CoDM_DMOnlyTimeline_Succeeds(t *testing.T) {
	h := NewHandler(newTestTimelineService(newDMOnlyRepo()))
	c := tlTestReq("/campaigns/camp-1/timelines/tl-secret/data", coDMCampaignCtx(), "u-codm", false, false)

	if err := h.TimelineDataAPI(c); err != nil {
		t.Fatalf("a co-DM must be able to fetch dm_only timeline data: %v", err)
	}
	rec := c.Response().Writer.(*httptest.ResponseRecorder)
	if !strings.Contains(rec.Body.String(), secretTimeline().Name) {
		t.Error("co-DM's JSON response did not contain the timeline name")
	}
}

// --- EmbedTimeline ---
//
// EmbedTimeline never returns an error for a bad/unauthorized id by design
// (its contract is to always render SOMETHING, falling back to the empty
// state) — so the assertion here is on rendered BODY CONTENT, not the error
// return: an unauthorized viewer must get the empty-state fragment, never
// TimelineEmbedFragment carrying the secret timeline's name.

func TestEmbedTimeline_Anonymous_DMOnlyTimeline_RendersEmpty(t *testing.T) {
	h := NewHandler(newTestTimelineService(newDMOnlyRepo()))
	c := tlTestReq("/campaigns/camp-1/timelines/embed?timeline_id=tl-secret", anonCampaignCtx(), "", false, false)

	if err := h.EmbedTimeline(c); err != nil {
		t.Fatalf("EmbedTimeline must not error, even for a hidden id: %v", err)
	}
	rec := c.Response().Writer.(*httptest.ResponseRecorder)
	body := rec.Body.String()
	if strings.Contains(body, secretTimeline().Name) {
		t.Errorf("anonymous viewer's embed fragment leaked the dm_only timeline's name: %s", body)
	}
}

func TestEmbedTimeline_PlainPlayer_DMOnlyTimeline_RendersEmpty(t *testing.T) {
	h := NewHandler(newTestTimelineService(newDMOnlyRepo()))
	c := tlTestReq("/campaigns/camp-1/timelines/embed?timeline_id=tl-secret", playerCampaignCtx(), "u-player", false, false)

	if err := h.EmbedTimeline(c); err != nil {
		t.Fatalf("EmbedTimeline must not error, even for a hidden id: %v", err)
	}
	rec := c.Response().Writer.(*httptest.ResponseRecorder)
	body := rec.Body.String()
	if strings.Contains(body, secretTimeline().Name) {
		t.Errorf("Player's embed fragment leaked the dm_only timeline's name: %s", body)
	}
}

func TestEmbedTimeline_Owner_DMOnlyTimeline_RendersFragment(t *testing.T) {
	h := NewHandler(newTestTimelineService(newDMOnlyRepo()))
	c := tlTestReq("/campaigns/camp-1/timelines/embed?timeline_id=tl-secret", ownerCampaignCtx(), "u-owner", false, false)

	if err := h.EmbedTimeline(c); err != nil {
		t.Fatalf("EmbedTimeline must not error: %v", err)
	}
	rec := c.Response().Writer.(*httptest.ResponseRecorder)
	body := rec.Body.String()
	if !strings.Contains(body, secretTimeline().Name) {
		t.Errorf("Owner's embed fragment should contain the timeline name, got: %s", body)
	}
}

func TestEmbedTimeline_CoDM_DMOnlyTimeline_RendersFragment(t *testing.T) {
	h := NewHandler(newTestTimelineService(newDMOnlyRepo()))
	c := tlTestReq("/campaigns/camp-1/timelines/embed?timeline_id=tl-secret", coDMCampaignCtx(), "u-codm", false, false)

	if err := h.EmbedTimeline(c); err != nil {
		t.Fatalf("EmbedTimeline must not error: %v", err)
	}
	rec := c.Response().Writer.(*httptest.ResponseRecorder)
	body := rec.Body.String()
	if !strings.Contains(body, secretTimeline().Name) {
		t.Errorf("co-DM's embed fragment should contain the timeline name, got: %s", body)
	}
}

// --- Search (repository.go:251 / service.go SearchTimelines follow-up finding) ---
//
// This is a SEPARATE leak from the Show/data/embed one above: a timeline
// with base visibility 'everyone' but a visibility_rules allow-list can still
// be NAMED by SearchTimelines to a viewer the allow-list excludes, because
// (before the fix) Search never ran the per-user filter List already runs.
// A dm_only timeline's NAME is also covered, redundantly with the SQL
// narrowing, by the same fixed code path.

func restrictedByRulesTimeline() Timeline {
	rules := `{"allowed_users":["u-1"]}`
	return Timeline{
		ID:              "tl-restricted",
		CampaignID:      "camp-1",
		Name:            "For u-1's Eyes Only",
		Visibility:      "everyone",
		VisibilityRules: &rules,
		Icon:            "fa-timeline",
		Color:           "#445566",
	}
}

// TestSearchTimelines_AnonymousViewer_DoesNotNameRestrictedTimeline was
// written RED-FIRST against SearchTimelines' pre-fix signature —
// (ctx, campaignID, query, role), no userID — since that is the interface
// that existed before the fix (see /tmp red evidence: this failed with a
// bare 4-arg call, repo.Search's fixture unfiltered). The fix added a userID
// parameter so the service can build the same permissions.Viewer
// ListTimelines already builds; this call site now carries the trailing ""
// anonymous user id. The assertion itself is unchanged: an anonymous viewer
// must never see this timeline's name.
func TestSearchTimelines_AnonymousViewer_DoesNotNameRestrictedTimeline(t *testing.T) {
	repo := &mockTimelineRepo{
		searchFn: func(_ context.Context, _ string, _ string, _ int) ([]Timeline, error) {
			return []Timeline{restrictedByRulesTimeline()}, nil
		},
	}
	svc := newTestTimelineService(repo)

	results, err := svc.SearchTimelines(context.Background(), "camp-1", "eyes", int(permissions.RoleNone), "")
	if err != nil {
		t.Fatalf("SearchTimelines: %v", err)
	}
	for _, r := range results {
		if r["name"] == restrictedByRulesTimeline().Name {
			t.Errorf("anonymous search results named a visibility_rules-restricted timeline: %+v", results)
		}
	}
}

// TestSearchTimelines_AllowedUser_StillSeesTheirRestrictedTimeline is the
// control proving the fix is a per-user FILTER, not a "hide everything
// dm_only-adjacent" sledgehammer: the user the allow-list actually names
// must still find their own timeline by name. Added alongside the fix
// (SearchTimelines gained the userID parameter this call needs, so it could
// not even compile beforehand) — there is no "before" state to red-first
// for a distinction the old signature had no way to express.
func TestSearchTimelines_AllowedUser_StillSeesTheirRestrictedTimeline(t *testing.T) {
	repo := &mockTimelineRepo{
		searchFn: func(_ context.Context, _ string, _ string, _ int) ([]Timeline, error) {
			return []Timeline{restrictedByRulesTimeline()}, nil
		},
	}
	svc := newTestTimelineService(repo)

	results, err := svc.SearchTimelines(context.Background(), "camp-1", "eyes", int(permissions.RolePlayer), "u-1")
	if err != nil {
		t.Fatalf("SearchTimelines: %v", err)
	}
	if len(results) != 1 || results[0]["name"] != restrictedByRulesTimeline().Name {
		t.Errorf("the allow-listed user must still see their own restricted timeline in search, got: %+v", results)
	}
}
