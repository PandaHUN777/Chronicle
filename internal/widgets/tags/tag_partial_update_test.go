// tag_partial_update_test.go — sweep R4 / ADR-056, the tags half of the
// absent-means-preserve contract. This is the worst finding of the
// 2026-09-12 toggle-truth sweep: tagService.Update took plain `color
// string, dmOnly bool` parameters, so there was no way for a caller that
// only means to rename a tag to represent "leave DmOnly alone" — the best
// it could do was resend the value it already knew, and any caller that
// did not (a bare rename form, a syncapi client sending {name} only)
// necessarily sent Go's zero value and turned a DM-only tag public to
// every player.
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
