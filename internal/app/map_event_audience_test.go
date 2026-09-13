// map_event_audience_test.go — S1: pins that mapEventPublisherAdapter
// actually computes the WebSocket audience from a marker/drawing's
// VisibilityRules, not just its dm_only bit.
//
// This is the companion to internal/websocket/hub_visibility_test.go,
// which drives a REAL Hub to prove the broadcast loop enforces
// AllowedUsers/DeniedUsers once they're set on a ws.Message. That test
// constructs the Message by hand, so it says nothing about whether the
// production adapter in THIS file's package ever sets those fields
// correctly from a real *maps.Marker / *maps.Drawing. This test closes
// that gap: it drives the actual PublishMarkerEvent/PublishDrawingEvent
// methods and asserts what lands on the wire-bound Message.
//
// It reuses the captureBus fixture already established by
// TestPublishFogEvent_RoutesByEventType (routes_test.go) for the same
// reason that one is legitimate: the assertion here is on the adapter's
// OWN output (which fields did it set on the message), not on delivery —
// delivery is what the hub test proves, against the real Hub, not a
// fake. Together the two tests cover the full path without either one
// pretending to be the other.
//
// Honesty check: this test fails if routes.go's publishWithAudience (or
// its PublishMarkerEvent/PublishDrawingEvent callers) stops parsing
// VisibilityRules and populating AllowedUsers/DeniedUsers/RequiresDM —
// regardless of whether hub.go's gate is correct. It does NOT fail if
// hub.go's gate is broken; that half is hub_visibility_test.go's job.
package app

import (
	"testing"

	"github.com/keyxmakerx/chronicle/internal/plugins/maps"
)

func strPtr(s string) *string { return &s }

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPublishMarkerEvent_ComputesAudienceFromVisibilityRules(t *testing.T) {
	cases := []struct {
		name           string
		marker         *maps.Marker
		wantRequiresDM bool
		wantAllowed    []string
		wantDenied     []string
	}{
		{
			name:   "everyone, no rules — full audience, no restriction",
			marker: &maps.Marker{ID: "m1", Visibility: "everyone"},
		},
		{
			name:           "dm_only — RequiresDM set, rules irrelevant",
			marker:         &maps.Marker{ID: "m2", Visibility: "dm_only"},
			wantRequiresDM: true,
		},
		{
			name:       "everyone with an explicit deny",
			marker:     &maps.Marker{ID: "m3", Visibility: "everyone", VisibilityRules: strPtr(`{"denied_users":["user-denied"]}`)},
			wantDenied: []string{"user-denied"},
		},
		{
			name:        "specific with an allow-list",
			marker:      &maps.Marker{ID: "m4", Visibility: "specific", VisibilityRules: strPtr(`{"allowed_users":["user-a"]}`)},
			wantAllowed: []string{"user-a"},
		},
		{
			name:        "both lists set at once",
			marker:      &maps.Marker{ID: "m5", Visibility: "specific", VisibilityRules: strPtr(`{"allowed_users":["user-a"],"denied_users":["user-b"]}`)},
			wantAllowed: []string{"user-a"},
			wantDenied:  []string{"user-b"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bus := &captureBus{}
			a := &mapEventPublisherAdapter{bus: bus}
			a.PublishMarkerEvent("updated", "camp-1", tc.marker)

			if bus.last == nil {
				t.Fatal("expected Publish to be called")
			}
			if bus.last.RequiresDM != tc.wantRequiresDM {
				t.Errorf("RequiresDM = %v, want %v", bus.last.RequiresDM, tc.wantRequiresDM)
			}
			if !stringSlicesEqual(bus.last.AllowedUsers, tc.wantAllowed) {
				t.Errorf("AllowedUsers = %v, want %v", bus.last.AllowedUsers, tc.wantAllowed)
			}
			if !stringSlicesEqual(bus.last.DeniedUsers, tc.wantDenied) {
				t.Errorf("DeniedUsers = %v, want %v", bus.last.DeniedUsers, tc.wantDenied)
			}
		})
	}
}

// TestPublishDrawingEvent_ComputesAudienceFromVisibilityRules is the
// drawing-side twin, added once Drawing gained VisibilityRules
// (maps/drawing.go) — the second half of the S1 leak, where a drawing's
// rules previously had no field to live in at all, so nothing could ever
// reach this adapter to be parsed in the first place.
func TestPublishDrawingEvent_ComputesAudienceFromVisibilityRules(t *testing.T) {
	cases := []struct {
		name           string
		drawing        *maps.Drawing
		wantRequiresDM bool
		wantAllowed    []string
		wantDenied     []string
	}{
		{
			name:    "everyone, no rules — full audience, no restriction",
			drawing: &maps.Drawing{ID: "d1", Visibility: "everyone"},
		},
		{
			name:           "dm_only — RequiresDM set, rules irrelevant",
			drawing:        &maps.Drawing{ID: "d2", Visibility: "dm_only"},
			wantRequiresDM: true,
		},
		{
			name:       "everyone with an explicit deny",
			drawing:    &maps.Drawing{ID: "d3", Visibility: "everyone", VisibilityRules: strPtr(`{"denied_users":["user-denied"]}`)},
			wantDenied: []string{"user-denied"},
		},
		{
			name:        "specific with an allow-list",
			drawing:     &maps.Drawing{ID: "d4", Visibility: "specific", VisibilityRules: strPtr(`{"allowed_users":["user-a"]}`)},
			wantAllowed: []string{"user-a"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bus := &captureBus{}
			a := &mapEventPublisherAdapter{bus: bus}
			a.PublishDrawingEvent("updated", "camp-1", tc.drawing)

			if bus.last == nil {
				t.Fatal("expected Publish to be called")
			}
			if bus.last.RequiresDM != tc.wantRequiresDM {
				t.Errorf("RequiresDM = %v, want %v", bus.last.RequiresDM, tc.wantRequiresDM)
			}
			if !stringSlicesEqual(bus.last.AllowedUsers, tc.wantAllowed) {
				t.Errorf("AllowedUsers = %v, want %v", bus.last.AllowedUsers, tc.wantAllowed)
			}
			if !stringSlicesEqual(bus.last.DeniedUsers, tc.wantDenied) {
				t.Errorf("DeniedUsers = %v, want %v", bus.last.DeniedUsers, tc.wantDenied)
			}
		})
	}
}

// TestPublishTokenEvent_NeverCarriesAudienceLists guards the "rules is nil
// for source kinds that don't carry per-user overrides" branch of
// publishWithAudience: tokens have no VisibilityRules concept (only the
// binary IsHidden), so a token event must never end up with a spurious
// AllowedUsers/DeniedUsers list.
func TestPublishTokenEvent_NeverCarriesAudienceLists(t *testing.T) {
	bus := &captureBus{}
	a := &mapEventPublisherAdapter{bus: bus}
	a.PublishTokenEvent("updated", "camp-1", &maps.Token{ID: "t1", IsHidden: true})

	if bus.last == nil {
		t.Fatal("expected Publish to be called")
	}
	if !bus.last.RequiresDM {
		t.Error("hidden token should be RequiresDM=true")
	}
	if len(bus.last.AllowedUsers) != 0 || len(bus.last.DeniedUsers) != 0 {
		t.Errorf("token events should never carry audience lists; got allowed=%v denied=%v", bus.last.AllowedUsers, bus.last.DeniedUsers)
	}
}
