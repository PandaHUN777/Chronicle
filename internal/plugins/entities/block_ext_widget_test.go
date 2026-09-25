package entities

import (
	"context"
	"strings"
	"testing"

	"github.com/keyxmakerx/chronicle/internal/plugins/campaigns"
)

// TestBlockExtWidget_EmitsDeclaredConfigAsDataAttrs pins #622: a placed
// extension widget must receive its manifest-declared config (e.g. the Draw
// Steel Bestiary Browser's "source") as data-* attributes, not just
// data-widget/data-campaign-id/data-entity-id. Without this, every placed
// widget runs with no config and silently falls back to its own default.
func TestBlockExtWidget_EmitsDeclaredConfigAsDataAttrs(t *testing.T) {
	cc := &campaigns.CampaignContext{Campaign: &campaigns.Campaign{ID: "camp-1"}}
	entity := &Entity{ID: "ent-1"}
	block := TemplateBlock{
		Type: "ext_widget",
		Config: map[string]any{
			"widget_slug": "bestiary-browser",
			"source":      "creatures",
		},
	}

	buf := &bytesBufferLike{}
	if err := blockExtWidget(cc, entity, block).Render(context.Background(), buf); err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := buf.String()

	for _, want := range []string{
		`data-widget="bestiary-browser"`,
		`data-source="creatures"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered HTML missing %q:\n%s", want, got)
		}
	}
}

// TestBlockExtWidget_ConfigAttrsAreEscaped pins that config values, which
// come from campaign-owner-editable layout JSON, go through templ's
// attribute escaping like every other data-* attribute here — never raw
// interpolation.
func TestBlockExtWidget_ConfigAttrsAreEscaped(t *testing.T) {
	cc := &campaigns.CampaignContext{Campaign: &campaigns.Campaign{ID: "camp-1"}}
	entity := &Entity{ID: "ent-1"}
	block := TemplateBlock{
		Type: "ext_widget",
		Config: map[string]any{
			"widget_slug": "bestiary-browser",
			"source":      `"><script>x</script>`,
		},
	}

	buf := &bytesBufferLike{}
	if err := blockExtWidget(cc, entity, block).Render(context.Background(), buf); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(buf.String(), "<script>") {
		t.Errorf("script tag leaked through escaping: %s", buf.String())
	}
}

// TestBlockExtWidget_WidgetSlugNeverDuplicatedAsDataAttr guards against
// double-emitting widget_slug as both data-widget (explicit) and
// data-widget_slug (via the generic config loop) — it must be consumed
// once, not echoed back as its own config attribute.
func TestBlockExtWidget_WidgetSlugNeverDuplicatedAsDataAttr(t *testing.T) {
	cc := &campaigns.CampaignContext{Campaign: &campaigns.Campaign{ID: "camp-1"}}
	entity := &Entity{ID: "ent-1"}
	block := TemplateBlock{
		Type:   "ext_widget",
		Config: map[string]any{"widget_slug": "bestiary-browser"},
	}

	buf := &bytesBufferLike{}
	if err := blockExtWidget(cc, entity, block).Render(context.Background(), buf); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(buf.String(), "data-widget_slug") {
		t.Errorf("widget_slug leaked as its own data attribute: %s", buf.String())
	}
}

// TestBlockExtWidget_DropsUnsafeConfigKeys pins that a config key which
// isn't a plain identifier never becomes an attribute name: templ escapes
// attribute values but not names, so a key with a space or '=' would
// otherwise inject its own attributes, such as an event handler.
func TestBlockExtWidget_DropsUnsafeConfigKeys(t *testing.T) {
	cc := &campaigns.CampaignContext{Campaign: &campaigns.Campaign{ID: "camp-1"}}
	entity := &Entity{ID: "ent-1"}
	block := TemplateBlock{
		Type: "ext_widget",
		Config: map[string]any{
			"widget_slug":                  "bestiary-browser",
			"x onfocus=alert(1) autofocus": "v",
			`q"uote`:                       "v",
			"tab\tkey":                     "v",
			"ok_key":                       "kept",
		},
	}

	buf := &bytesBufferLike{}
	if err := blockExtWidget(cc, entity, block).Render(context.Background(), buf); err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := buf.String()

	for _, bad := range []string{"onfocus", "autofocus", "uote", "tab"} {
		if strings.Contains(got, bad) {
			t.Errorf("rendered HTML contains %q from an unsafe config key:\n%s", bad, got)
		}
	}
	if !strings.Contains(got, `data-ok_key="kept"`) {
		t.Errorf("rendered HTML missing the safe key data-ok_key:\n%s", got)
	}
}

// TestBlockExtWidget_ConfigCannotRepeatReservedAttrs pins that config keys
// can't repeat the attributes the mount div sets itself, in any letter case.
func TestBlockExtWidget_ConfigCannotRepeatReservedAttrs(t *testing.T) {
	cc := &campaigns.CampaignContext{Campaign: &campaigns.Campaign{ID: "camp-1"}}
	entity := &Entity{ID: "ent-1"}
	block := TemplateBlock{
		Type: "ext_widget",
		Config: map[string]any{
			"widget_slug": "bestiary-browser",
			"widget":      "evil-widget",
			"Campaign-Id": "other-campaign",
			"entity-id":   "other-entity",
			"editable":    "true",
		},
	}

	buf := &bytesBufferLike{}
	if err := blockExtWidget(cc, entity, block).Render(context.Background(), buf); err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := strings.ToLower(buf.String())

	for _, bad := range []string{"evil-widget", "other-campaign", "other-entity", "data-editable"} {
		if strings.Contains(got, bad) {
			t.Errorf("rendered HTML contains %q from a reserved config key:\n%s", bad, got)
		}
	}
	for _, name := range []string{"data-widget=", "data-campaign-id=", "data-entity-id="} {
		if n := strings.Count(got, name); n != 1 {
			t.Errorf("%s appears %d times, want 1:\n%s", name, n, got)
		}
	}
}
