package sessions

// Session pages must not leak the names of privately-visible linked entities
// to a Player. ADR-055 rule 3: hidden content is ABSENT for a non-Owner
// viewer — not greyed, not counted, not named.
//
// THE DEFECT: ListSessionEntities' repository join (`INNER JOIN entities e`)
// carries no privacy predicate and ShowSession rendered every row it got back
// unconditionally. A DM marking an NPC private and linking it to a session as
// "encountered" published its name to every player who opened the session
// page. See ADR-055 / ADR-056 (2026-09-12 visibility sweep).
//
// The fix filters the session's linked-entity list against the SAME
// visibility policy the entities plugin itself applies (via the new
// EntityVisibilityFilter service dependency), in ShowSession, before the list
// reaches the template. Owners and Scribes are unaffected.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/keyxmakerx/chronicle/internal/plugins/campaigns"
)

// stubEntityVisibility is a minimal EntityVisibilityFilter: every entity id
// listed in `hidden` is reported as not viewable, everything else is.
type stubEntityVisibility struct {
	hidden map[string]bool
}

func (s *stubEntityVisibility) FilterViewableEntityIDs(_ context.Context, _ string, entityIDs []string, _ int, _ string) (map[string]bool, error) {
	out := make(map[string]bool, len(entityIDs))
	for _, id := range entityIDs {
		out[id] = !s.hidden[id]
	}
	return out, nil
}

// showSessionHTML drives the real handler (repo -> service -> handler ->
// template) for a session with two linked entities, one of which the
// visibility filter reports as hidden, and returns the rendered body.
func showSessionHTML(t *testing.T, role campaigns.Role) string {
	t.Helper()
	repo := &mockSessionRepo{
		findByIDFn: func(_ context.Context, id string) (*Session, error) {
			return &Session{ID: id, CampaignID: "camp-1", Name: "Session One", Status: StatusPlanned}, nil
		},
		listSessionEntitiesFn: func(_ context.Context, _ string) ([]SessionEntity, error) {
			return []SessionEntity{
				{EntityID: "ent-secret", EntityName: "Secret Villain", EntitySlug: "secret-villain", Role: EntityRoleEncountered},
				{EntityID: "ent-public", EntityName: "Town Square", EntitySlug: "town-square", Role: EntityRoleMentioned},
			}, nil
		},
	}
	visFilter := &stubEntityVisibility{hidden: map[string]bool{"ent-secret": true}}
	svc := NewSessionService(repo, nil, visFilter)
	h := NewHandler(svc)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/campaigns/camp-1/sessions/s1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id", "sid")
	c.SetParamValues("camp-1", "s1")
	c.Set("campaign_context", &campaigns.CampaignContext{
		Campaign:   &campaigns.Campaign{ID: "camp-1"},
		MemberRole: role,
	})

	if err := h.ShowSession(c); err != nil {
		t.Fatalf("ShowSession returned error: %v", err)
	}
	return rec.Body.String()
}

// TestShowSession_PlayerDoesNotSeeHiddenEntityName is the RED test for the
// leak: a Player must not receive the name or slug of a session-linked entity
// the entities plugin's own visibility policy would hide from them.
func TestShowSession_PlayerDoesNotSeeHiddenEntityName(t *testing.T) {
	html := showSessionHTML(t, campaigns.RolePlayer)
	if strings.Contains(html, "Secret Villain") {
		t.Errorf("Player-role session page leaked a hidden entity's name\n%s", html)
	}
	if strings.Contains(html, "secret-villain") {
		t.Errorf("Player-role session page leaked a hidden entity's slug (in its link href)\n%s", html)
	}
	if !strings.Contains(html, "Town Square") {
		t.Errorf("Player-role session page hid a visible entity it should still show\n%s", html)
	}
}

// TestShowSession_ScribeAndOwnerSeeEverything pins ADR-055's "keep the
// Owner/Scribe view unchanged": the fix must not narrow what those roles
// already saw.
func TestShowSession_ScribeAndOwnerSeeEverything(t *testing.T) {
	for _, role := range []campaigns.Role{campaigns.RoleScribe, campaigns.RoleOwner} {
		html := showSessionHTML(t, role)
		if !strings.Contains(html, "Secret Villain") {
			t.Errorf("role %v lost visibility into a linked entity it could already see", role)
		}
		if !strings.Contains(html, "Town Square") {
			t.Errorf("role %v lost visibility into a visible linked entity", role)
		}
	}
}
