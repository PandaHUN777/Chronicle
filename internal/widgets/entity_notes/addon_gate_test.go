// addon_gate_test.go — ADR-056: "Player Notes" is a FEATURE toggle (off means
// off for everyone), not the sync-api-style integration toggle. Its five
// routes (GET/POST list, GET/PUT/DELETE one) were registered with campaign
// membership as the only check — zero addon awareness anywhere in this
// package — so turning the campaign's "player-notes" addon off hid the
// dashboard panel (internal/plugins/entities/block_registry_core.go) but left
// the REST API wide open to any campaign member.
//
// These tests drive RegisterRoutes itself, end-to-end through the real
// middleware chain, rather than hand-checking that a gate function exists —
// the defect was a correct gate that was simply never mounted here, and a
// fixture that wired it by hand would pass just as happily with routes.go
// unchanged.
package entity_notes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/keyxmakerx/chronicle/internal/plugins/addons"
	"github.com/keyxmakerx/chronicle/internal/plugins/auth"
	"github.com/keyxmakerx/chronicle/internal/plugins/campaigns"
)

// sessionCookieName mirrors the unexported constant in internal/plugins/auth
// (handler.go) — the auth package doesn't export it, so tests outside that
// package reproduce the literal, same as other cross-package fixtures in this
// codebase reproduce "campaign_context" for GetCampaignContext.
const sessionCookieName = "chronicle_session"

// gateFakeAuthSvc treats any cookie-bearing request as a valid session for a
// fixed user. Embeds the interface so only the one method RequireAuth calls
// (ValidateSession) needs a body.
type gateFakeAuthSvc struct{ auth.AuthService }

func (gateFakeAuthSvc) ValidateSession(_ context.Context, _ string) (*auth.Session, error) {
	return &auth.Session{UserID: "user-1"}, nil
}

// gateFakeCampaignSvc resolves any campaign ID to a campaign the fixed user
// is a Player member of.
type gateFakeCampaignSvc struct{ campaigns.CampaignService }

func (gateFakeCampaignSvc) GetByID(_ context.Context, id string) (*campaigns.Campaign, error) {
	return &campaigns.Campaign{ID: id}, nil
}

func (gateFakeCampaignSvc) GetMember(_ context.Context, campaignID, userID string) (*campaigns.CampaignMember, error) {
	return &campaigns.CampaignMember{CampaignID: campaignID, UserID: userID, Role: campaigns.RolePlayer}, nil
}

// gateFakeAddonSvc reports a single fixed enabled/disabled answer for every
// campaign/slug — enough to drive the two cases this test needs.
type gateFakeAddonSvc struct {
	addons.AddonService
	enabled bool
}

func (f gateFakeAddonSvc) IsEnabledForCampaign(_ context.Context, _ string, _ string) (bool, error) {
	return f.enabled, nil
}

// gateFakeNotesSvc is a minimal Service that always succeeds, so a request
// that clears every middleware gate reaches a real 200 rather than failing
// for an unrelated reason.
type gateFakeNotesSvc struct{ Service }

func (gateFakeNotesSvc) List(_ context.Context, _ string, _ ViewerContext) ([]Note, error) {
	return []Note{}, nil
}

// newGateRouter builds a real Echo router via RegisterRoutes with the addon
// reporting `enabled`.
func newGateRouter(enabled bool) *echo.Echo {
	e := echo.New()
	h := NewHandler(gateFakeNotesSvc{})
	RegisterRoutes(e, h, gateFakeCampaignSvc{}, gateFakeAuthSvc{}, gateFakeAddonSvc{enabled: enabled})
	return e
}

// authedNotesRequest builds a GET to the notes list route carrying a session
// cookie, so it clears auth.RequireAuth regardless of the addon gate.
func authedNotesRequest() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/campaigns/camp-1/entities/ent-1/notes", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok"})
	return req
}

// TestEntityNotesRoutes_RefusedWhenAddonDisabled is the RED test for the
// leak: with the "player-notes" addon OFF, a campaign member hitting the
// notes API directly must be refused, not served.
func TestEntityNotesRoutes_RefusedWhenAddonDisabled(t *testing.T) {
	rec := httptest.NewRecorder()
	newGateRouter(false).ServeHTTP(rec, authedNotesRequest())

	if rec.Code == http.StatusOK {
		t.Fatalf("entity_notes route served a request with the player-notes addon disabled: got 200, body %q", rec.Body.String())
	}
}

// TestEntityNotesRoutes_ServedWhenAddonEnabled pins that the gate does not
// regress the normal case: a member request succeeds when the addon is on.
func TestEntityNotesRoutes_ServedWhenAddonEnabled(t *testing.T) {
	rec := httptest.NewRecorder()
	newGateRouter(true).ServeHTTP(rec, authedNotesRequest())

	if rec.Code != http.StatusOK {
		t.Fatalf("entity_notes route refused a normal request with the player-notes addon enabled: got %d, body %q", rec.Code, rec.Body.String())
	}
}
