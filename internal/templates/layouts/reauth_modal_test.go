package layouts

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestReauthModalHasNoStaticHiddenClass pins #596: the modal's visibility must
// be driven only by Alpine's x-show="show". A static "hidden" Tailwind class on
// the same element conflicts with it — when Alpine sets show=true it clears its
// own inline display:none override (reverting style.display to ""), which lets
// the class-based `display:none` from "hidden" keep winning, so the modal never
// actually appears even though x-show believes it is shown. x-cloak (backed by
// the [x-cloak]{display:none!important} rule in base.templ) is what must hide
// the modal before Alpine initializes, not a static class.
func TestReauthModalHasNoStaticHiddenClass(t *testing.T) {
	var buf strings.Builder
	if err := ReauthModal().Render(context.Background(), &buf); err != nil {
		t.Fatalf("render ReauthModal: %v", err)
	}
	html := buf.String()

	at := strings.Index(html, `id="reauth-modal"`)
	if at < 0 {
		t.Fatal(`no id="reauth-modal" in the rendered output`)
	}
	// Scan back to the start of the opening tag to read its class attribute.
	tagStart := strings.LastIndex(html[:at], "<div")
	if tagStart < 0 {
		t.Fatal("could not find the opening <div for #reauth-modal")
	}
	tagEnd := strings.Index(html[tagStart:], ">")
	if tagEnd < 0 {
		t.Fatal("unterminated opening tag for #reauth-modal")
	}
	openTag := html[tagStart : tagStart+tagEnd]

	classStart := strings.Index(openTag, ` class="`)
	if classStart < 0 {
		t.Fatal("#reauth-modal has no class attribute")
	}
	rest := openTag[classStart+8:]
	classEnd := strings.Index(rest, `"`)
	if classEnd < 0 {
		t.Fatal("unterminated class attribute on #reauth-modal")
	}
	classes := strings.Fields(rest[:classEnd])

	for _, c := range classes {
		if c == "hidden" {
			t.Fatalf("#reauth-modal must not carry a static \"hidden\" class alongside x-show=\"show\" — "+
				"Alpine only toggles its own inline display style, so a permanent class-level "+
				"display:none keeps winning and the modal never appears on reauth-required; "+
				"got classes %q", classes)
		}
	}

	if !strings.Contains(openTag, "x-cloak") {
		t.Error("#reauth-modal must keep x-cloak to stay hidden before Alpine initializes, " +
			"since dropping the static \"hidden\" class removes the only other guard against a flash")
	}
}

// TestReauthModalXCloakCSSRuleExists confirms base.templ still carries the
// [x-cloak]{display:none!important} rule this modal now depends on exclusively
// (with the static "hidden" class gone) to avoid an unstyled flash before
// Alpine boots.
func TestReauthModalXCloakCSSRuleExists(t *testing.T) {
	// Base() needs a lot of context wiring; the CSS rule is a static string
	// literal in the template regardless, so reading the source directly is
	// the direct way to pin it without standing up the whole layout.
	src, err := os.ReadFile("base.templ")
	if err != nil {
		t.Fatalf("read base.templ: %v", err)
	}
	if !strings.Contains(string(src), "[x-cloak]{display:none!important}") {
		t.Fatal("base.templ no longer defines the [x-cloak]{display:none!important} rule — " +
			"#reauth-modal relies on it to stay hidden before Alpine initializes")
	}
}
