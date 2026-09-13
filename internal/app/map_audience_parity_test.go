package app

// map_audience_parity_test.go — the per-user map visibility predicate exists
// in THREE places, and this test is the only thing that keeps them honest.
//
// Security item S1 closed a leak where the WebSocket publisher ignored a
// marker's allowed_users / denied_users and the hub delivered a marker to a
// player it was explicitly hidden from. The fix filters per recipient in the
// hub. But the rule itself now lives in:
//
//  1. SQL — maps' ListMarkers and ListDrawings non-owner branches, which is
//     what the HTTP list endpoints actually enforce.
//  2. Go — maps.VisibilityRules.Allows, the in-process restatement.
//  3. Go — websocket.Message.AudienceAllows, used by the hub's fan-out.
//
// (3) cannot call (2): internal/websocket is generic transport and must not
// import a plugin's types, and maps must not import the hub. So the
// duplication is deliberate and unavoidable — which makes drift the risk.
// If the socket's predicate and the list's predicate disagree, a marker
// becomes more or less visible over the wire than the page shows, and that
// is precisely the leak S1 existed to close, reopened from the other side.
//
// Both implementations carry comments saying they must stay in lockstep. A
// comment is not a guard. internal/app is the one package that imports both,
// so the guard lives here: one table of cases, run through both, asserting
// they agree AND that each returns the documented answer. The second half
// matters — two implementations that drift together would agree with each
// other and still be wrong, so the table states the contract independently.
//
// The contract has TWO defaults, which is the part that is easy to get
// backwards: with only denied_users set, a user named in neither list is
// INCLUDED (default-allow); once allowed_users is non-empty it is a strict
// allowlist and that same user is EXCLUDED (default-deny). Taken from
// ListMarkers' SQL, not invented here.
//
// If this test fails, do not "fix" it by editing the expectation. Find which
// of the three copies moved, and decide whether the SQL or the Go is right.

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
