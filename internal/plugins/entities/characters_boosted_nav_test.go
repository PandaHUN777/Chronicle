package entities

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/keyxmakerx/chronicle/internal/plugins/campaigns"
)

// characters_boosted_nav_test.go pins that the Characters page survives a
// boosted sidebar navigation: every sidebar link is hx-boost with
// hx-target="#main-content", and boot.js sets htmx.config.allowScriptTags=false,
// under which htmx REMOVES any <script> tag inside the swapped fragment rather
// than just declining to execute it. A page script must therefore ship via the
// plugin body-script registry (internal/app/routes.go ->
// layouts.SetPluginBodyScripts -> internal/templates/layouts/base.templ),
// which emits outside the swapped region. See tools/page-script-allowlist.txt.

// castSwappedRegion returns the substring htmx would keep on a boosted sidebar
// navigation: the contents of <main id="main-content">. Both bounds are checked
// before they are used, so a layout rename fails loudly here instead of
// silently reducing the assertion to a scan of the empty string.
func castSwappedRegion(t *testing.T, page string) string {
	t.Helper()
	const marker = `id="main-content"`
	at := strings.Index(page, marker)
	if at < 0 {
		t.Fatalf("no %s in the rendered page — the App layout's swap target was renamed and this "+
			"boosted-navigation assertion just stopped reading anything", marker)
	}
	open := strings.Index(page[at:], ">")
	if open < 0 {
		t.Fatal("unterminated <main> open tag in the rendered page")
	}
	rest := page[at+open+1:]
	end := strings.Index(rest, "</main>")
	if end < 0 {
		t.Fatal("no </main> in the rendered page")
	}
	return rest[:end]
}

// renderCharactersPage renders CharactersPage with an empty context, the way
// every other page-templ test in this tree does. The registry contributes
// nothing under an empty context by construction, which is why its contents are
// pinned separately below.
func renderCharactersPage(t *testing.T, view CastView) string {
	t.Helper()
	cc := &campaigns.CampaignContext{Campaign: &campaigns.Campaign{ID: "camp1", Name: "Test Campaign"}}
	var sb strings.Builder
	if err := CharactersPage(cc, view).Render(context.Background(), &sb); err != nil {
		t.Fatalf("render CharactersPage: %v", err)
	}
	return sb.String()
}

// TestCharactersPageMountsNoScriptInsideTheSwappedRegion checks the swapped
// region only (inside <main>), not the whole document: the shell's own script
// tags sit outside it and a boosted navigation is meant to keep those. Scripts
// inside #main-content must be zero, for a page with a party and for an empty
// one.
func TestCharactersPageMountsNoScriptInsideTheSwappedRegion(t *testing.T) {
	owner := func(s string) *string { return &s }
	populated := CastView{
		ShowPlayers: true,
		Party: []CastMember{
			{Entity: Entity{ID: "a", Name: "Aldric", OwnerUserID: owner("u1")}, OwnerName: "Alice"},
		},
	}
	for name, view := range map[string]CastView{
		"populated": populated,
		"empty":     {ShowPlayers: true},
		"no-party":  {ShowPlayers: false},
	} {
		swapped := castSwappedRegion(t, renderCharactersPage(t, view))
		if n := strings.Count(swapped, "<script"); n != 0 {
			t.Errorf("%s: the Characters page emits %d <script> tag(s) inside #main-content; htmx "+
				"DELETES every one of them on a boosted sidebar navigation "+
				"(allowScriptTags=false), so the cast cards' quick-look wires on a direct load and "+
				"silently does not through the sidebar. Contribute it to the plugin body-script "+
				"registry in internal/app/routes.go instead.", name, n)
		}
	}
}

// TestCharactersDriverShipsFromThePluginBodyScriptRegistry pins that
// characters.js is mounted via the plugin body-script registry, not a
// `<script src>` inside the page body; without this the previous test would
// pass trivially by deleting the tag and orphaning the script. It reads the
// source directly since pluginBodyScripts is a startup-wiring local with no
// exported value to assert against.
func TestCharactersDriverShipsFromThePluginBodyScriptRegistry(t *testing.T) {
	src, err := os.ReadFile("../../app/routes.go")
	if err != nil {
		t.Fatalf("read the plugin body-script registry: %v", err)
	}
	s := string(src)

	const head = "pluginBodyScripts := []string{"
	at := strings.Index(s, head)
	if at < 0 {
		t.Fatalf("no %q in internal/app/routes.go — the registry was renamed and this guard stopped "+
			"reading anything", head)
	}
	rest := s[at+len(head):]
	end := strings.Index(rest, "}")
	if end < 0 {
		t.Fatal("unterminated pluginBodyScripts slice literal in internal/app/routes.go")
	}
	slice := rest[:end]

	if !strings.Contains(slice, `/js/characters.js"`) {
		t.Error("the plugin body-script registry does not mount characters.js — a Characters page " +
			"reached through the sidebar would render its cast cards with the quick-look driver " +
			"stripped, and look identical while doing it")
	}
	if !strings.Contains(slice, "entities.PluginSlug") {
		t.Error("characters.js is not addressed through entities.PluginSlug — the registry path must " +
			"track the slug the plugin's static FS is actually mounted under")
	}
}
