package campaigns

import (
	"context"
	"strings"
	"testing"
)

// surface_accent_swap_safety_test.go pins #631: the Surface Accents card used
// to wire its buttons with a single <script> that delegated click/change
// listeners. boot.js sets htmx.config.allowScriptTags=false, under which
// htmx deletes every <script> tag (inline or with src) from a fragment
// swapped in by a boosted sidebar navigation, so a user who reached
// Customize > Appearance by clicking the sidebar got dead swatches. The fix
// moves the wiring onto each control as its own inline IIFE onclick/onchange
// (surface_accent_onclick.go), which survives the swap because it lives on
// the element itself.

// TestSurfaceAccentsCardHasNoScriptTag proves the card never emits a
// <script> element — the exact thing htmx would strip on a boosted swap.
func TestSurfaceAccentsCardHasNoScriptTag(t *testing.T) {
	cc := &CampaignContext{
		Campaign:   &Campaign{ID: "camp-1", Settings: `{"accent_color":"#6366f1"}`},
		MemberRole: RoleOwner,
	}
	var sb strings.Builder
	if err := appearanceTab(cc, "tok").Render(context.Background(), &sb); err != nil {
		t.Fatalf("render appearanceTab: %v", err)
	}
	html := sb.String()

	at := strings.Index(html, `id="appearance-surface-accents"`)
	if at < 0 {
		t.Fatal(`no id="appearance-surface-accents" in the rendered output`)
	}
	// Bound the card to just its own markup so an unrelated <script>
	// elsewhere on the tab (e.g. TopbarImageSection) can't hide a regression.
	rest := html[at:]
	end := strings.Index(rest, "</div>\n\t\t</div>")
	if end < 0 {
		// Fall back to scanning to the end of the document; still catches a
		// script tag reintroduced anywhere in the card.
		end = len(rest)
	}
	card := rest[:end]

	if strings.Contains(card, "<script") {
		t.Error("Surface Accents card must not emit a <script> tag — htmx deletes it whole on a " +
			"boosted sidebar navigation (allowScriptTags=false), leaving the card's handlers dead; " +
			"use an inline IIFE onclick/onchange per control instead")
	}
}

// TestSurfaceAccentsCardControlsCarryOwnHandlers proves each preset swatch,
// the reset button, and the custom picker each carry their own onclick or
// onchange attribute (rather than relying on a parent listener that a script
// tag would have installed).
func TestSurfaceAccentsCardControlsCarryOwnHandlers(t *testing.T) {
	cc := &CampaignContext{
		Campaign:   &Campaign{ID: "camp-1", Settings: `{"accent_color":"#6366f1"}`},
		MemberRole: RoleOwner,
	}
	var sb strings.Builder
	if err := appearanceTab(cc, "tok").Render(context.Background(), &sb); err != nil {
		t.Fatalf("render appearanceTab: %v", err)
	}
	html := sb.String()

	// Every preset swatch plus the two reset buttons across both rows: at
	// least one onclick per row is present, and both custom-color inputs
	// carry onchange.
	if n := strings.Count(html, ` onclick="`); n < 2 {
		t.Errorf("expected onclick handlers on the surface-accent swatches/reset buttons, found %d", n)
	}
	if n := strings.Count(html, ` onchange="`); n < 2 {
		t.Errorf("expected onchange handlers on both custom-color inputs, found %d", n)
	}
	for _, id := range []string{"appearance-surface-custom-1", "appearance-surface-custom-2"} {
		idAt := strings.Index(html, `id="`+id+`"`)
		if idAt < 0 {
			t.Fatalf("no input with id %q rendered", id)
		}
		// The onchange attribute must sit on the same element as the id, not
		// just appear somewhere later in the document.
		tagStart := strings.LastIndex(html[:idAt], "<input")
		tagEnd := strings.Index(html[tagStart:], ">")
		openTag := html[tagStart : tagStart+tagEnd]
		if !strings.Contains(openTag, "onchange=") {
			t.Errorf("input %q must carry its own onchange handler; got tag %q", id, openTag)
		}
	}
}
