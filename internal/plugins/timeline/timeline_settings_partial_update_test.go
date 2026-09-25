// Pins the timeline-settings half of the absent-means-preserve contract
// (ADR-054): visibility_rules and description_html are always absent from
// the settings-save wire struct, so UpdateTimeline must preserve them on
// absence, never clear them — canUserView() treats an absent
// VisibilityRules as visible to everyone, so clearing it would silently
// republish a restricted timeline to the whole campaign.
package timeline

import (
	"context"
	"testing"

	"github.com/keyxmakerx/chronicle/internal/patch"
)

// storedTimeline is the fully-populated row every case starts from.
func storedTimeline() *Timeline {
	s := func(v string) *string { return &v }
	return &Timeline{
		ID:              "tl-1",
		CampaignID:      "camp-1",
		Name:            "Ages of Eldrin",
		Description:     s("plain text notes"),
		DescriptionHTML: s("<p>a long formatted write-up</p>"),
		Color:           "#3366ff",
		Icon:            "fa-hourglass",
		Visibility:      "dm_only",
		VisibilityRules: s(`{"allowed_users":["u-1","u-2","u-3"]}`),
		ZoomDefault:     ZoomYear,
	}
}

func runTimelineUpdate(t *testing.T, input UpdateTimelineInput) *Timeline {
	t.Helper()
	var written *Timeline
	repo := &mockTimelineRepo{
		getByIDFn: func(_ context.Context, _ string) (*Timeline, error) { return storedTimeline(), nil },
		updateFn:  func(_ context.Context, tl *Timeline) error { written = tl; return nil },
	}
	if err := newTestTimelineService(repo).UpdateTimeline(context.Background(), "tl-1", input); err != nil {
		t.Fatalf("UpdateTimeline: %v", err)
	}
	if written == nil {
		t.Fatal("nothing was written")
	}
	return written
}

// THE headline regression: a settings-form save carrying exactly what the
// wire request struct can carry (name/description/color/icon/visibility/
// zoom_default) must not wipe the two fields that struct cannot carry at
// all: description_html and visibility_rules.
func TestTimelineSettingsSave_PreservesDescriptionHTMLAndVisibilityRules(t *testing.T) {
	got := runTimelineUpdate(t, UpdateTimelineInput{
		Name:        "Ages of Eldrin (renamed)",
		Color:       patch.Of("#3366ff"),
		Icon:        patch.Of("fa-hourglass"),
		Visibility:  patch.Of("dm_only"),
		ZoomDefault: patch.Of(ZoomYear),
	})
	want := storedTimeline()

	if got.Name != "Ages of Eldrin (renamed)" {
		t.Errorf("Name = %q, want renamed", got.Name)
	}
	assertPtrEq(t, "DescriptionHTML", got.DescriptionHTML, want.DescriptionHTML)
	assertPtrEq(t, "VisibilityRules", got.VisibilityRules, want.VisibilityRules)
	assertPtrEq(t, "Description", got.Description, want.Description)
}

// Pins the service-level contract for VisibilityRules/DescriptionHTML:
// absent preserves, present replaces, explicit null clears — regardless of
// what the current web UI can reach, since the dedicated visibility
// endpoint owns VisibilityRules.
func TestTimeline_VisibilityRulesAndDescriptionHTML_ThreeDirections(t *testing.T) {
	strp := func(s string) *string { return &s }
	cases := []struct {
		name         string
		input        UpdateTimelineInput
		wantRules    *string
		wantDescHTML *string
	}{
		{
			"absent preserves both",
			UpdateTimelineInput{Name: "Ages of Eldrin", Visibility: patch.Of("dm_only"), ZoomDefault: patch.Of(ZoomYear)},
			strp(`{"allowed_users":["u-1","u-2","u-3"]}`), strp("<p>a long formatted write-up</p>"),
		},
		{
			"present replaces both",
			UpdateTimelineInput{
				Name:            "Ages of Eldrin",
				Visibility:      patch.Of("dm_only"),
				ZoomDefault:     patch.Of(ZoomYear),
				VisibilityRules: patch.Of(`{"denied_users":["u-9"]}`),
				DescriptionHTML: patch.Of("<p>new write-up</p>"),
			},
			strp(`{"denied_users":["u-9"]}`), strp("<p>new write-up</p>"),
		},
		{
			"explicit null clears both",
			UpdateTimelineInput{
				Name:            "Ages of Eldrin",
				Visibility:      patch.Of("dm_only"),
				ZoomDefault:     patch.Of(ZoomYear),
				VisibilityRules: patch.Null[string](),
				DescriptionHTML: patch.Null[string](),
			},
			nil, nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runTimelineUpdate(t, tc.input)
			assertPtrEq(t, "VisibilityRules", got.VisibilityRules, tc.wantRules)
			assertPtrEq(t, "DescriptionHTML", got.DescriptionHTML, tc.wantDescHTML)
		})
	}
}
