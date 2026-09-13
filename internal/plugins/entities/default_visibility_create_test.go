// default_visibility_create_test.go — the campaign's DefaultVisibility
// setting has to reach EVERY entity-creation path, not just the web form.
//
// Before the fix, CampaignSettings.DefaultVisibility had exactly one
// consumer in the whole repo: Handler.Create, the HTML form. QuickCreateAPI —
// the JSON endpoint the shop inventory widget posts to — built its
// CreateEntityInput with {Name, EntityTypeID} and nothing else, so every item
// the shop widget created in a "DM Only" campaign was visible to every player
// the instant it existed.
//
// The three directions per path are the point. "Absent" and "explicit false"
// are DIFFERENT: the campaign default fills in the first and must not
// override the second, which is why the resolution takes a patch.Field.
package entities

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/keyxmakerx/chronicle/internal/plugins/campaigns"
)

// stubCreateCapturingSvc records the CreateEntityInput the handler builds.
// GetEntityTypeByID returns a type with no fields so parseFieldsFromForm is
// a no-op; GetEntityTypes answers the quick-create fallback lookup.
type stubCreateCapturingSvc struct {
	EntityService
	created []CreateEntityInput
}

func (s *stubCreateCapturingSvc) Create(_ context.Context, campaignID, _ string, input CreateEntityInput) (*Entity, error) {
	s.created = append(s.created, input)
	return &Entity{ID: "ent-new", CampaignID: campaignID, Name: input.Name, IsPrivate: input.IsPrivate}, nil
}

func (s *stubCreateCapturingSvc) GetEntityTypeByID(_ context.Context, id int) (*EntityType, error) {
	return &EntityType{ID: id, Name: "Item", Slug: "item"}, nil
}

func (s *stubCreateCapturingSvc) GetEntityTypes(_ context.Context, _ string) ([]EntityType, error) {
	return []EntityType{{ID: 7, Name: "Item", Slug: "item"}}, nil
}

// newCreateContext builds an echo context carrying a campaign whose settings
// JSON holds the given default_visibility value.
func newCreateContext(t *testing.T, defaultVis, body, contentType string) echo.Context {
	t.Helper()
	settings := "{}"
	if defaultVis != "" {
		settings = `{"default_visibility":"` + defaultVis + `"}`
	}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/campaigns/camp-1/entities", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", contentType)
	c := e.NewContext(req, httptest.NewRecorder())
	c.SetParamNames("id")
	c.SetParamValues("camp-1")
	c.Set("campaign_context", &campaigns.CampaignContext{
		Campaign:   &campaigns.Campaign{ID: "camp-1", Name: "Test", Settings: settings},
		MemberRole: campaigns.RoleOwner,
		IsMember:   true,
	})
	return c
}

// --- QuickCreateAPI (shop widget) ------------------------------------------

func TestQuickCreateAPI_HonoursCampaignDefaultVisibility(t *testing.T) {
	cases := []struct {
		name       string
		defaultVis string
		body       string
		want       bool
		why        string
	}{
		{
			name:       "absent is_private under dm_only default starts private",
			defaultVis: "dm_only",
			body:       `{"name":"Potion of Healing","entity_type_id":7}`,
			want:       true,
			why:        "the shop widget never sends is_private, so the campaign default is the only thing that can hide the item",
		},
		{
			name:       "absent is_private under private default starts private",
			defaultVis: "private",
			body:       `{"name":"Potion of Healing","entity_type_id":7}`,
			want:       true,
			why:        "\"private\" is the other value the settings page writes",
		},
		{
			name:       "explicit false under dm_only default stays public",
			defaultVis: "dm_only",
			body:       `{"name":"Town Notice","entity_type_id":7,"is_private":false}`,
			want:       false,
			why:        "a client that deliberately says public must not be overridden by the default",
		},
		{
			name:       "explicit true with no default is private",
			defaultVis: "",
			body:       `{"name":"Secret Ledger","entity_type_id":7,"is_private":true}`,
			want:       true,
			why:        "an explicit request for private is honoured with or without a campaign default",
		},
		{
			name:       "absent with no default stays public",
			defaultVis: "",
			body:       `{"name":"Rope, 50ft","entity_type_id":7}`,
			want:       false,
			why:        "no default set means today's behaviour is unchanged",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &stubCreateCapturingSvc{}
			h := &Handler{service: svc}
			c := newCreateContext(t, tc.defaultVis, tc.body, echo.MIMEApplicationJSON)

			if err := h.QuickCreateAPI(c); err != nil {
				t.Fatalf("QuickCreateAPI: %v", err)
			}
			if len(svc.created) != 1 {
				t.Fatalf("expected 1 Create call, got %d", len(svc.created))
			}
			if got := svc.created[0].IsPrivate; got != tc.want {
				t.Errorf("IsPrivate = %v, want %v — %s", got, tc.want, tc.why)
			}
		})
	}
}

// --- Handler.Create (the web form, which already honoured the setting) -----

// The form path is the ONE path that was already correct. This pins its
// behaviour across the refactor that moved the four-line inline block onto
// the shared CampaignSettings.ResolveNewEntityPrivacy, so "there is now one
// implementation" cannot quietly mean "the form path changed".
//
// An unchecked HTML checkbox submits NOTHING, so the form's value-typed
// false IS the absent case — the form has no way to express "explicitly
// public", and never had one.
func TestCreate_FormHonoursCampaignDefaultVisibility(t *testing.T) {
	cases := []struct {
		name       string
		defaultVis string
		form       url.Values
		want       bool
	}{
		{
			name:       "unchecked box under dm_only default starts private",
			defaultVis: "dm_only",
			form:       url.Values{"name": {"Hidden Lair"}, "entity_type_id": {"7"}},
			want:       true,
		},
		{
			name:       "unchecked box under private default starts private",
			defaultVis: "private",
			form:       url.Values{"name": {"Hidden Lair"}, "entity_type_id": {"7"}},
			want:       true,
		},
		{
			name:       "unchecked box with no default stays public",
			defaultVis: "",
			form:       url.Values{"name": {"Village Green"}, "entity_type_id": {"7"}},
			want:       false,
		},
		{
			name:       "checked box is private with no default",
			defaultVis: "",
			form:       url.Values{"name": {"Secret"}, "entity_type_id": {"7"}, "is_private": {"true"}},
			want:       true,
		},
		{
			name:       "checked box is private under dm_only default",
			defaultVis: "dm_only",
			form:       url.Values{"name": {"Secret"}, "entity_type_id": {"7"}, "is_private": {"true"}},
			want:       true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &stubCreateCapturingSvc{}
			h := &Handler{service: svc}
			c := newCreateContext(t, tc.defaultVis, tc.form.Encode(), echo.MIMEApplicationForm)

			if err := h.Create(c); err != nil && !strings.Contains(err.Error(), "redirect") {
				t.Fatalf("Create: %v", err)
			}
			if len(svc.created) != 1 {
				t.Fatalf("expected 1 Create call, got %d", len(svc.created))
			}
			if got := svc.created[0].IsPrivate; got != tc.want {
				t.Errorf("IsPrivate = %v, want %v", got, tc.want)
			}
		})
	}
}
