package foundry_vtt

import (
	"strconv"
	"strings"
)

// semverLess reports whether version a is strictly older than version b.
// Used by the admin's "notify older-version campaigns" action to filter
// campaigns whose pin is older than a given target.
//
// Handles the Foundry module version dialect: optional leading "v",
// 3-segment dotted decimals ("0.1.5", "1.10.0"), and an optional
// pre-release after "-" ("0.2.0-beta.1" sorts < "0.2.0"). Non-numeric
// segments compare lexicographically; missing segments are treated as
// 0 ("1.0" == "1.0.0"). Permissive on purpose — this orders versions
// the way operators expect, not strict semver.
func semverLess(a, b string) bool {
	pa, pra := splitVersion(a)
	pb, prb := splitVersion(b)

	for i := 0; i < len(pa) || i < len(pb); i++ {
		var ai, bi int
		if i < len(pa) {
			ai, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			bi, _ = strconv.Atoi(pb[i])
		}
		if ai != bi {
			return ai < bi
		}
	}

	// Numeric parts equal — compare pre-release. Per semver, a
	// version with a pre-release sorts BEFORE the same version
	// without one. ("0.2.0-beta" < "0.2.0".)
	switch {
	case pra == "" && prb == "":
		return false
	case pra == "":
		return false // a is the release; release > pre-release
	case prb == "":
		return true // b is the release; pre-release < release
	default:
		return pra < prb
	}
}

// splitVersion drops a leading "v"/"V", splits on "-" to separate the
// core dotted-decimal from the pre-release tag, and returns the
// dotted segments + the (possibly empty) pre-release string.
func splitVersion(s string) ([]string, string) {
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	core, pre, _ := strings.Cut(s, "-")
	return strings.Split(core, "."), pre
}
