package app

// map_audience_parity_test.go keeps three independent restatements of the
// per-user map visibility predicate in lockstep: maps' ListMarkers /
// ListDrawings SQL, maps.VisibilityRules.Allows, and
// websocket.Message.AudienceAllows (which can't call the Go restatement:
// internal/websocket must not import plugin types, and maps must not import
// the hub). Each case's expectation is asserted against both implementations
// independently, so two that drifted together would still agree with each
// other. A failure means one of the three copies moved — decide whether the
// SQL or the Go is right; do not edit the expectation to match.
//
// Contract: with only denied_users set, an unlisted user is INCLUDED
// (default-allow); once allowed_users is non-empty it's a strict allowlist
// and that user is EXCLUDED (default-deny), per ListMarkers' SQL.

import (
	"testing"

	"github.com/keyxmakerx/chronicle/internal/plugins/maps"
	"github.com/keyxmakerx/chronicle/internal/websocket"
)

func TestMapAudiencePredicateParity(t *testing.T) {
	const (
		alice = "user-alice"
		bob   = "user-bob"
		carol = "user-carol"
	)

	cases := []struct {
		name    string
		allowed []string
		denied  []string
		viewer  string
		want    bool
		why     string
	}{
		{
			name:   "no rules at all — everyone sees it",
			viewer: alice,
			want:   true,
			why:    "an absent rule set is the common case and must not hide anything",
		},
		{
			name:   "deny-only, viewer denied",
			denied: []string{bob},
			viewer: bob,
			want:   false,
			why:    "the whole point of S1: an explicit deny excludes",
		},
		{
			name:   "deny-only, viewer not named",
			denied: []string{bob},
			viewer: alice,
			want:   true,
			why:    "default-ALLOW while allowed_users is empty",
		},
		{
			name:   "deny-only, anonymous viewer",
			denied: []string{bob},
			viewer: "",
			want:   false,
			why:    "ADR-049: a non-empty deny list can't prove an anonymous visitor isn't the denied player",
		},
		{
			name:   "no deny list at all, anonymous viewer",
			viewer: "",
			want:   true,
			why:    "an absent deny list has nothing to exclude anonymous from",
		},
		{
			name:    "allowlist, viewer on it",
			allowed: []string{alice, bob},
			viewer:  alice,
			want:    true,
			why:     "named on a non-empty allowlist",
		},
		{
			name:    "allowlist, viewer not named",
			allowed: []string{alice, bob},
			viewer:  carol,
			want:    false,
			why:     "default-DENY once allowed_users is non-empty — the asymmetry",
		},
		{
			name:    "both lists, deny wins over allow",
			allowed: []string{alice, bob},
			denied:  []string{bob},
			viewer:  bob,
			want:    false,
			why:     "deny is checked first and is absolute; being on both must not grant access",
		},
		{
			name:    "both lists, allowed and not denied",
			allowed: []string{alice, bob},
			denied:  []string{carol},
			viewer:  alice,
			want:    true,
			why:     "the ordinary allowed case with a deny list also present",
		},
		{
			name:    "both lists, neither names the viewer",
			allowed: []string{alice},
			denied:  []string{bob},
			viewer:  carol,
			want:    false,
			why:     "a non-empty allowlist excludes, even though the deny list is silent",
		},
		{
			name:    "empty-but-present lists behave as absent",
			allowed: []string{},
			denied:  []string{},
			viewer:  alice,
			want:    true,
			why:     "a marker saved with empty rules must not become invisible",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rules := &maps.VisibilityRules{AllowedUsers: tc.allowed, DeniedUsers: tc.denied}
			msg := &websocket.Message{AllowedUsers: tc.allowed, DeniedUsers: tc.denied}

			httpSide := rules.Allows(tc.viewer)
			socketSide := msg.AudienceAllows(tc.viewer)

			if httpSide != socketSide {
				t.Errorf("THE TWO PREDICATES DISAGREE: maps.VisibilityRules.Allows=%v, "+
					"websocket.Message.AudienceAllows=%v. A marker is now more or less visible "+
					"over the socket than the HTTP list shows, which is the S1 leak reopened. "+
					"Find which copy moved; do not edit this expectation.", httpSide, socketSide)
			}
			if httpSide != tc.want {
				t.Errorf("maps.VisibilityRules.Allows = %v, want %v — %s", httpSide, tc.want, tc.why)
			}
			if socketSide != tc.want {
				t.Errorf("websocket.Message.AudienceAllows = %v, want %v — %s", socketSide, tc.want, tc.why)
			}
		})
	}
}

// TestMapAudiencePredicateParity_NilRules covers the one shape the two types
// cannot express identically: maps.VisibilityRules is a pointer that is nil
// when a marker has no rules stored, while a Message simply carries two nil
// slices. Both must mean "everyone", and a nil receiver must not panic —
// ListMarkers hands Allows a nil for the overwhelming majority of markers.
func TestMapAudiencePredicateParity_NilRules(t *testing.T) {
	var rules *maps.VisibilityRules
	if !rules.Allows("user-alice") {
		t.Error("a nil VisibilityRules must allow everyone; a marker with no rules is public to the campaign")
	}
	if !(&websocket.Message{}).AudienceAllows("user-alice") {
		t.Error("a Message with no audience fields must allow everyone")
	}
}
