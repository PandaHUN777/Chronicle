// dm_grant_visibility_test.go — ADR-057 slice 1 (C-CODM-VIS-PARITY).
//
// campaigns.CampaignContext.VisibilityRole() promotes a DM-granted member
// (IsDmGranted=true) to Owner for visibility purposes so they can see
// dm_only content — that is the documented design (campaigns/model.go). But
// every CheckEntityAccess caller in this file passed the raw cc.MemberRole
// instead, so a Co-DM (a Player with a DM grant) was 404'd opening exactly
// the dm_only entity they are meant to be able to see. BacklinksFragment did
// both in one function: it already used VisibilityRole() to build the
// backlinks list, then re-derived a plain MemberRole for the target's own
// access check a few lines later — so the list is populated but the page
// that would show it 404s.
//
// These tests drive Handler.Show and Handler.BacklinksFragment directly with
// a campaign context set the way gm_fields_handler_test.go / player_notes
// _idor_test.go do (c.Set("campaign_context", ...) by the middleware's known
// key), so no router/session middleware is needed. The stub CheckEntityAccess
// mirrors the real service's default-visibility "dm_only" rule (service.go:
// Scribe+ i.e. role>=2 required) so the test exercises the actual promotion
// threshold rather than an arbitrary sentinel.
package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/keyxmakerx/chronicle/internal/plugins/auth"
	"github.com/keyxmakerx/chronicle/internal/plugins/campaigns"
)

// dmGrantEntitySvc is a minimal EntityService stub for a single dm_only
// (IsPrivate=true, default visibility) entity, plus one backlink entry that
// mentions it. CheckEntityAccess reproduces the real service's legacy
// default-mode rule so the Scribe+ (role>=2) threshold is genuine, not just
// an assertion on which constant got passed.
type dmGrantEntitySvc struct {
	EntityService
	entity   *Entity
	etype    *EntityType
	children []Entity
}

func (s *dmGrantEntitySvc) GetByID(_ context.Context, _ string) (*Entity, error) {
	e := *s.entity
	return &e, nil
}

func (s *dmGrantEntitySvc) GetEntityTypeByID(_ context.Context, _ int) (*EntityType, error) {
	et := *s.etype
	return &et, nil
}

func (s *dmGrantEntitySvc) GetAncestors(_ context.Context, _ string, _ int, _ string) ([]Entity, error) {
	return nil, nil
}

// GetChildren mirrors the real repository's visibilityFilter default-mode
// rule (entities/repository.go: role>=RoleScribe sees dm_only; role>=RoleOwner
// is the same threshold with room to spare) so a test against this stub
// exercises the actual promotion threshold Show's GetChildren call site must
// clear for a Co-DM, not an arbitrary sentinel unconnected to production.
func (s *dmGrantEntitySvc) GetChildren(_ context.Context, _ string, role int, _ string) ([]Entity, error) {
	if role >= int(campaigns.RoleScribe) {
		return s.children, nil
	}
	visible := make([]Entity, 0, len(s.children))
	for _, ch := range s.children {
		if !ch.IsPrivate {
			visible = append(visible, ch)
		}
	}
	return visible, nil
}

// CheckEntityAccess mirrors entityService.CheckEntityAccess's legacy
// default-mode branch: a private (dm_only) entity needs role>=RoleScribe.
// Real Owners (role>=RoleOwner) short-circuit above that in the production
// code; this stub only needs the Scribe threshold since that's what
// VisibilityRole()'s Owner promotion must clear for a Co-DM.
func (s *dmGrantEntitySvc) CheckEntityAccess(_ context.Context, _ string, role int, _ string) (*EffectivePermission, error) {
	if s.entity.IsPrivate && role < int(campaigns.RoleScribe) {
		return &EffectivePermission{CanView: false}, nil
	}
	return &EffectivePermission{CanView: true, CanEdit: role >= int(campaigns.RoleScribe)}, nil
}

// GetBacklinksWithSnippets always returns one entry. In production this
// query is already scoped by VisibilityRole() (handler.go's BacklinksFragment
// computes `role` once, at the top, for exactly this call) — the bug this
// slice fixes is the SECOND, independent role derivation a few lines later
// for the target entity's own CheckEntityAccess gate, so the list side is
// deliberately not part of what's under test here.
func (s *dmGrantEntitySvc) GetBacklinksWithSnippets(_ context.Context, campaignID, _ string, _ int, _ string) ([]BacklinkEntry, error) {
	return []BacklinkEntry{{
		Entity:  Entity{ID: "mentioner-1", CampaignID: campaignID, Name: "Secret War Council Minutes"},
		Snippet: "...the Baron's plan hinges on the hidden garrison...",
	}}, nil
}

// dmOnlyFixture returns a dm_only (IsPrivate, default-visibility) entity and
// a bare entity type. The entity type's Layout is left zero-valued (no rows)
// so Show falls through to its default two-column layout instead of a
// custom template — the same path a freshly-seeded entity type takes.
func dmOnlyFixture() (*Entity, *EntityType) {
	ent := &Entity{
		ID: "e1", CampaignID: "c1", EntityTypeID: 7, Name: "The Baron's Real Plan",
		IsPrivate: true, Visibility: VisibilityDefault,
	}
	et := &EntityType{ID: 7, CampaignID: "c1", Slug: "npc", Name: "NPC", NamePlural: "NPCs"}
	return ent, et
}

// coDmContext is a Player MemberRole with an Owner-granted dm_only visibility
// grant — the Co-DM shape the defect hits. Real Owners already satisfy every
// CheckEntityAccess call without promotion, so they don't exercise this path.
func coDmContext() *campaigns.CampaignContext {
	return &campaigns.CampaignContext{
		Campaign:    &campaigns.Campaign{ID: "c1"},
		MemberRole:  campaigns.RolePlayer,
		IsDmGranted: true,
		IsMember:    true,
	}
}

// TestShow_CoDmCanOpenDmOnlyEntity pins the Show-handler half of the defect:
// a Co-DM (Player + DM grant) opening a dm_only entity must get the page, not
// a 404. Before the fix, Show passed int(cc.MemberRole) (Player, role=1) to
// CheckEntityAccess, which is below the Scribe threshold a dm_only entity
// requires — so this failed with a 404 AppError even though VisibilityRole()
// already promotes exactly this viewer to Owner for visibility purposes.
func TestShow_CoDmCanOpenDmOnlyEntity(t *testing.T) {
	ent, et := dmOnlyFixture()
	h := &Handler{service: &dmGrantEntitySvc{entity: ent, etype: et}}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/campaigns/c1/entities/e1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id", "eid")
	c.SetParamValues("c1", "e1")
	c.Set("campaign_context", coDmContext())
	auth.SetSession(c, &auth.Session{UserID: "codm-1"})

	err := h.Show(c)
	if err != nil {
		t.Fatalf("Co-DM opening a dm_only entity: Show returned %v, want nil (200) — "+
			"VisibilityRole() promotes IsDmGranted to Owner for visibility, but the "+
			"CheckEntityAccess call site is still gating on raw MemberRole", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("Co-DM opening a dm_only entity: got status %d, want 200", rec.Code)
	}
}

// TestBacklinksFragment_CoDmSeesListAndReachesTarget pins the two-halves-of-
// one-function defect called out in ADR-057: BacklinksFragment already builds
// its list with cc.VisibilityRole() (so a Co-DM's backlinks list is
// populated), but its OWN entity's CheckEntityAccess call a few lines later
// re-derived int(cc.MemberRole) — so the Co-DM saw evidence the entity has
// referencing content, then got 404'd trying to load that very fragment.
// This asserts both halves together: 200, and the populated list in the body.
func TestBacklinksFragment_CoDmSeesListAndReachesTarget(t *testing.T) {
	ent, et := dmOnlyFixture()
	h := &Handler{service: &dmGrantEntitySvc{entity: ent, etype: et}}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/campaigns/c1/entities/e1/backlinks", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id", "eid")
	c.SetParamValues("c1", "e1")
	c.Set("campaign_context", coDmContext())
	auth.SetSession(c, &auth.Session{UserID: "codm-1"})

	err := h.BacklinksFragment(c)
	if err != nil {
		t.Fatalf("Co-DM requesting backlinks of a dm_only entity: got %v, want nil (200)", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("Co-DM requesting backlinks of a dm_only entity: got status %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Secret War Council Minutes") {
		t.Errorf("backlinks list must be populated for the Co-DM viewer; body=%s", rec.Body)
	}
}

// TestShow_CoDmSeesDmOnlyChildren pins the entity page's OTHER half of the
// ADR-057 review finding: Show's own CheckEntityAccess call (a few lines above
// GetChildren) is already promoted via cc.VisibilityRole(), but the GetChildren
// call passed the raw cc.MemberRole -- so a Co-DM who is correctly let onto a
// dm_only PARENT page then saw that page's own dm_only children silently
// dropped from the Sub-pages section (line 604 and line 617 must agree, per
// the P1FIX dispatch). Before the fix this failed because GetChildren's stub
// (mirroring the real repository's visibilityFilter) excludes a dm_only child
// below the RoleScribe threshold, and int(cc.MemberRole) for this Co-DM
// (Player) is below it.
func TestShow_CoDmSeesDmOnlyChildren(t *testing.T) {
	ent, et := dmOnlyFixture()
	child := Entity{
		ID: "child-1", CampaignID: "c1", EntityTypeID: 7, Name: "The Hidden Vault",
		IsPrivate: true, Visibility: VisibilityDefault, TypeName: "NPC",
	}
	h := &Handler{service: &dmGrantEntitySvc{entity: ent, etype: et, children: []Entity{child}}}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/campaigns/c1/entities/e1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id", "eid")
	c.SetParamValues("c1", "e1")
	c.Set("campaign_context", coDmContext())
	auth.SetSession(c, &auth.Session{UserID: "codm-1"})

	err := h.Show(c)
	if err != nil {
		t.Fatalf("Co-DM opening a dm_only entity: Show returned %v, want nil (200)", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("Co-DM opening a dm_only entity: got status %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "The Hidden Vault") {
		t.Errorf("Co-DM must see the dm_only child in the Sub-pages list; body=%s", rec.Body)
	}
}
