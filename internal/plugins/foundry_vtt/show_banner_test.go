// show_banner_test.go pins the banner's "Update pin" settings link
// against the canonical set of campaign-settings tab values.

package foundry_vtt

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// validSettingsTabs mirrors the five `tab = '<value>'` buttons in
// campaigns/settings.templ. If a tab is renamed or added there, update
// this set and re-confirm every ?tab=<x> URL repo-wide.
var validSettingsTabs = map[string]bool{
	"general":      true,
	"features":     true,
	"people":       true,
	"integrations": true,
	"activity":     true,
}

// TestCampaignShowFoundryBanner_LinksToValidSettingsTab pins the
// banner's "Update pin" link against validSettingsTabs. A link to a
// non-existent tab initializes Alpine's tab to a no-match value,
// hiding every tab section behind a blank page.
func TestCampaignShowFoundryBanner_LinksToValidSettingsTab(t *testing.T) {
	component := CampaignShowFoundryBanner("test-campaign-id", BannerStatus{
		HasUpdate:      true,
		LatestVersion:  "0.2.0",
		CurrentVersion: "0.1.0",
	})
	var buf bytes.Buffer
	if err := component.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render banner: %v", err)
	}
	html := buf.String()

	// Locate the settings link. The banner has exactly one
	// /campaigns/<id>/settings?tab=... href; extract its tab value.
	const prefix = "/campaigns/test-campaign-id/settings?tab="
	idx := strings.Index(html, prefix)
	if idx < 0 {
		t.Fatalf("banner does not contain expected settings link prefix %q\n"+
			"rendered HTML:\n%s", prefix, html)
	}
	rest := html[idx+len(prefix):]
	// The tab value runs until the next quote or non-tab char.
	end := strings.IndexAny(rest, "\"'& <")
	if end < 0 {
		t.Fatalf("could not find end of tab value in href: %q", rest[:minInt(50, len(rest))])
	}
	tab := rest[:end]

	if !validSettingsTabs[tab] {
		t.Errorf("CampaignShowFoundryBanner links to ?tab=%q — not a valid "+
			"settings tab.\n"+
			"Valid tabs (from campaigns/settings.templ buttons): general, "+
			"features, people, integrations, activity.\n"+
			"The foundry-vtt settings fragment lives inside the "+
			"'integrations' tab, so this banner should target ?tab=integrations.\n"+
			"\nThis is the cordinator Issue #16 regression. If a tab "+
			"rename made %q valid intentionally, update validSettingsTabs "+
			"in this test.",
			tab, tab)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
