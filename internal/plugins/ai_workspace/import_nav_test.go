package ai_workspace

// import_nav_test.go pins that the campaign settings route, which renders a
// full page (campaigns.Settings has no IsHTMX fragment branch), is only ever
// reached by navigation (plain anchors under hx-boost) and never by an
// hx-get that would fragment-swap it into a host div, nesting the app
// inside itself.

import (
	"os"
	"regexp"
	"testing"
)

// TestSettingsRouteNeverFragmentTargeted asserts no templ source in this
// plugin issues an hx-get against the settings page. Plain href anchors are
// the sanctioned way back to the tab.
func TestSettingsRouteNeverFragmentTargeted(t *testing.T) {
	files := []string{"import_review.templ", "import_result.templ", "tab.templ", "modal.templ"}
	re := regexp.MustCompile(`hx-get=\{[^}]*settings\?tab=`)
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			continue // optional surfaces may not exist in older trees
		}
		if re.Match(src) {
			t.Errorf("%s: hx-get targets the full-page settings route — use a plain <a href> (boosted nav) instead; fragment-swapping it nests the app inside itself", f)
		}
	}
}
