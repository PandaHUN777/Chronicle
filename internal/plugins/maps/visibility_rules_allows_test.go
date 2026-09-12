// visibility_rules_allows_test.go — pins VisibilityRules.Allows as the
// canonical Go-language statement of the SQL predicate in repository.go's
// ListMarkers and drawing_repository.go's ListDrawings (S1). Both the
// WebSocket hub's messageAudienceAllows (internal/websocket/hub.go) and
// this method must agree with the SQL; this test is what "verified
// against the code, not assumed" means for the "what's the default for a
// user named in neither list" question the S1 task asked to be settled.
package maps

import "testing"

func TestVisibilityRulesAllows(t *testing.T) {
	cases := []struct {
		name   string
		rules  *VisibilityRules
		userID string
		want   bool
	}{
		{"nil rules — no restriction at all", nil, "anyone", true},
		{"empty rules — no restriction at all", &VisibilityRules{}, "anyone", true},
		{
			name:   "deny-only mode: user on the deny list is excluded",
			rules:  &VisibilityRules{DeniedUsers: []string{"user-denied"}},
			userID: "user-denied",
			want:   false,
		},
		{
			name:   "deny-only mode: a user named in NEITHER list defaults to INCLUDED",
			rules:  &VisibilityRules{DeniedUsers: []string{"user-denied"}},
			userID: "user-neither-list",
			want:   true,
		},
		{
			name:   "allow-list mode: a listed user is included",
			rules:  &VisibilityRules{AllowedUsers: []string{"user-allowed"}},
			userID: "user-allowed",
			want:   true,
		},
		{
			name:   "allow-list mode: a user named in NEITHER list defaults to EXCLUDED",
			rules:  &VisibilityRules{AllowedUsers: []string{"user-allowed"}},
			userID: "user-neither-list",
			want:   false,
		},
		{
			name: "both lists set: deny always wins even if also allowed",
			rules: &VisibilityRules{
				AllowedUsers: []string{"user-x"},
				DeniedUsers:  []string{"user-x"},
			},
			userID: "user-x",
			want:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rules.Allows(tc.userID); got != tc.want {
				t.Errorf("Allows(%q) = %v, want %v", tc.userID, got, tc.want)
			}
		})
	}
}
