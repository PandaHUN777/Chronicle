// category_calendar_block_test.go pins that a calendar_preview block added
// to an entity type's category dashboard renders. It exercises the SERVER
// round-trip (stored layout JSON -> parse -> render) so a regression here
// localizes to the Go path, distinct from a client-side (layout editor)
// failure to persist the block.
package entities

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/keyxmakerx/chronicle/internal/plugins/campaigns"
)

func TestCategoryDashboard_CalendarBlockRenders(t *testing.T) {
	layout := `{"rows":[{"id":"r1","columns":[{"id":"c1","width":12,"blocks":[` +
		`{"id":"b1","type":"calendar_preview","config":{}}]}]}]}`
	et := &EntityType{
		ID: 7, CampaignID: "camp-1", Slug: "npc", Name: "NPC", NamePlural: "NPCs",
		DashboardLayout: &layout,
	}

	// 1) Parse: the stored layout must yield the calendar block.
	parsed := et.ParseCategoryDashboardLayout()
	if parsed == nil {
		t.Fatal("ParseCategoryDashboardLayout returned nil for a valid calendar layout")
	}
	if len(parsed.Rows) != 1 || len(parsed.Rows[0].Columns) != 1 || len(parsed.Rows[0].Columns[0].Blocks) != 1 {
		t.Fatalf("layout shape lost in parse: %+v", parsed)
	}
	if got := parsed.Rows[0].Columns[0].Blocks[0].Type; got != "calendar_preview" {
		t.Fatalf("block type = %q, want calendar_preview", got)
	}

	// 2) Render: the custom dashboard must show the calendar card.
	//
	// CALV5-PLACEHOLDER: while the calendar is rebuilt this asserts the
	// shared rebuild notice instead of the card's own header ("Upcoming
	// Events"); restore the header assertion when V5 restores the card.
	cc := &campaigns.CampaignContext{Campaign: &campaigns.Campaign{ID: "camp-1", Name: "C"}, MemberRole: campaigns.RoleOwner}
	var buf bytes.Buffer
	if err := CategoryDashboardContent(cc, et, nil, nil, 0, ListOptions{}, "", nil).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "is being rebuilt") {
		t.Errorf("calendar block did not render at all; custom dashboard likely fell back to default.\nHTML:\n%s", html)
	}
}
