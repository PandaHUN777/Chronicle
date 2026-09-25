package media

import (
	"regexp"
	"testing"
)

// rfc4122v4 matches a lowercase, hyphenated version-4 UUID: the shape the
// media_files.id CHAR(36) column and every existing row expect.
var rfc4122v4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// TestGenerateUUID_FormatMatchesColumn pins the ID shape media file
// generation must keep producing. #712 replaces the hand-rolled
// crypto/rand implementation with uuid.NewString() (already a direct
// dependency, used in seven other packages) — this regression test
// protects the media_files.id CHAR(36) column, and every existing row's
// format, across that swap.
func TestGenerateUUID_FormatMatchesColumn(t *testing.T) {
	cases := []struct {
		name string
	}{
		{"first"},
		{"second"},
		{"third"},
	}

	seen := make(map[string]bool, len(cases))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := generateUUID()
			if len(id) != 36 {
				t.Fatalf("generateUUID() = %q, len = %d, want 36", id, len(id))
			}
			if !rfc4122v4.MatchString(id) {
				t.Fatalf("generateUUID() = %q, does not match RFC4122 v4 shape", id)
			}
			if seen[id] {
				t.Fatalf("generateUUID() returned a duplicate: %q", id)
			}
			seen[id] = true
		})
	}
}
