// tag_partial_update_test.go pins the absent-means-preserve contract
// (ADR-056): a partial update that omits color/dmOnly must leave the stored
// value unchanged, not silently reset it (e.g. turning a DM-only tag public).
package tags

import (
	"context"
	"testing"
)

func storedTag() *Tag {
	return &Tag{ID: 1, CampaignID: "camp-1", Name: "Secret Plot", Slug: "secret-plot", Color: "#8b5cf6", DmOnly: true}
}

// THE headline regression: a rename-only call (Color/DmOnly genuinely
// ABSENT, not resent) must not turn off DmOnly or change the color.
func TestRename_PreservesDmOnlyAndColor(t *testing.T) {
	repo := &mockTagRepo{
		findByIDFn: func(_ context.Context, id int) (*Tag, error) { return storedTag(), nil },
	}
	svc := newTestService(repo)

	result, err := svc.Update(context.Background(), 1, UpdateTagInput{Name: "Secret Plot (renamed)"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "Secret Plot (renamed)" {
		t.Errorf("Name = %q, want renamed", result.Name)
	}
	if !result.DmOnly {
		t.Error("DmOnly flipped to false: a rename must not turn off a DM-only tag")
	}
	if result.Color != "#8b5cf6" {
		t.Errorf("Color = %q, want preserved (#8b5cf6)", result.Color)
	}
}

// Color/DmOnly must still be settable — this isn't a one-way ratchet.
func TestUpdate_ColorAndDmOnly_CanStillBeSet(t *testing.T) {
	repo := &mockTagRepo{
		findByIDFn: func(_ context.Context, id int) (*Tag, error) { return storedTag(), nil },
	}
	svc := newTestService(repo)

	color, dmOnly := "#22c55e", false
	result, err := svc.Update(context.Background(), 1, UpdateTagInput{
		Name:   "Secret Plot",
		Color:  &color,
		DmOnly: &dmOnly,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Color != "#22c55e" {
		t.Errorf("Color = %q, want #22c55e", result.Color)
	}
	if result.DmOnly {
		t.Error("DmOnly stayed true, want explicitly-set false")
	}
}
