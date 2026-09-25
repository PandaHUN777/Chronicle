package permissions

import "testing"

// TestDeniesAnonymous pins the ADR-049 amendment: a logged-out visitor can't
// be proven not to be the specific player a deny list names, so once a deny
// list is non-empty, an anonymous viewer is excluded by it too.
func TestDeniesAnonymous(t *testing.T) {
	cases := []struct {
		name   string
		denied []string
		userID string
		want   bool
	}{
		{"no deny list — anonymous unaffected", nil, "", false},
		{"empty deny list — anonymous unaffected", []string{}, "", false},
		{"non-empty deny list, anonymous viewer — denied", []string{"u-bryn"}, "", true},
		{"non-empty deny list, authenticated viewer — not this rule's concern", []string{"u-bryn"}, "u-alice", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DeniesAnonymous(tc.denied, tc.userID); got != tc.want {
				t.Errorf("DeniesAnonymous(%v, %q) = %v, want %v", tc.denied, tc.userID, got, tc.want)
			}
		})
	}
}
