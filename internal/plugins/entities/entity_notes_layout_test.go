package entities

import (
	"context"
	"encoding/json"
	"testing"
)

// blockTypeOrder flattens a layout's block types in row/column/block order.
func blockTypeOrder(l EntityTypeLayout) []string {
	var out []string
	for _, r := range l.Rows {
		for _, c := range r.Columns {
			for _, b := range c.Blocks {
				out = append(out, b.Type)
			}
		}
	}
	return out
}

func indexOf(ss []string, want string) int {
	for i, s := range ss {
		if s == want {
			return i
		}
	}
	return -1
}

// TestDefaultLayout_IncludesEntityNotes pins the item-7 fix: new entity types
// must carry the Player Notes block. It used to also assert a trailing
// permissions block; ADR-057 decision 5 removed that from the generated
// default layout (visibility editing moved into edit mode only), so that half
// of the old assertion is gone — this is no longer testing a permissions
// position, just that entity_notes is present.
func TestDefaultLayout_IncludesEntityNotes(t *testing.T) {
	order := blockTypeOrder(DefaultLayout())
	if indexOf(order, "entity_notes") < 0 {
		t.Fatalf("DefaultLayout must include an entity_notes block, got %v", order)
	}
	if indexOf(order, "permissions") >= 0 {
		t.Errorf("DefaultLayout must not auto-append a permissions block (ADR-057 decision 5), got %v", order)
	}
}

// TestCharacterLayout_IncludesEntityNotes pins that PC types get Player Notes
// too, without disturbing the character_surface-first invariant that
// ensure_pc_type_test.go relies on. Also no longer carries a permissions
// block (ADR-057 decision 5).
func TestCharacterLayout_IncludesEntityNotes(t *testing.T) {
	l := CharacterLayout()
	order := blockTypeOrder(l)
	if indexOf(order, "entity_notes") < 0 {
		t.Fatalf("CharacterLayout must include an entity_notes block, got %v", order)
	}
	if l.Rows[0].Columns[0].Blocks[0].Type != "character_surface" {
		t.Errorf("character_surface must remain the first block, got %v", order)
	}
	if indexOf(order, "permissions") >= 0 {
		t.Errorf("CharacterLayout must not auto-append a permissions block (ADR-057 decision 5), got %v", order)
	}
}

// TestEnsureEntityNotesBlockInDefaults_BackfillsAndIsIdempotent pins the
// reconciler: a type whose layout lacks entity_notes gets it (inserted above
// permissions), a second run is a no-op, and a type that already has the
// block is left untouched.
func TestEnsureEntityNotesBlockInDefaults_BackfillsAndIsIdempotent(t *testing.T) {
	// Type 1 lacks entity_notes but has a permissions row; type 2 already
	// has entity_notes and must not be touched.
	legacy := EntityTypeLayout{Rows: []TemplateRow{
		{ID: "row-1", Columns: []TemplateColumn{{ID: "c1", Width: 8, Blocks: []TemplateBlock{{ID: "b-entry", Type: "entry"}}}}},
		{ID: "row-perm", Columns: []TemplateColumn{{ID: "c-perm", Width: 12, Blocks: []TemplateBlock{{ID: "b-perm", Type: "permissions"}}}}},
	}}
	already := DefaultLayout() // already contains entity_notes

	saved := map[int]string{}
	typeRepo := &mockEntityTypeRepo{
		listAllFn: func(_ context.Context) ([]EntityType, error) {
			return []EntityType{{ID: 1, Layout: legacy}, {ID: 2, Layout: already}}, nil
		},
		updateLayoutFn: func(_ context.Context, id int, layoutJSON string) error {
			saved[id] = layoutJSON
			return nil
		},
	}
	svc := newTestService(&mockEntityRepo{}, typeRepo).(*entityService)

	n, err := svc.EnsureEntityNotesBlockInDefaults(context.Background())
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if n != 1 {
		t.Fatalf("want exactly 1 type updated (type 1), got %d", n)
	}
	if _, touched := saved[2]; touched {
		t.Error("type 2 already had entity_notes and must not be rewritten")
	}

	// Type 1's new layout must contain entity_notes, positioned before permissions.
	var got EntityTypeLayout
	if err := json.Unmarshal([]byte(saved[1]), &got); err != nil {
		t.Fatalf("unmarshal saved layout: %v", err)
	}
	order := blockTypeOrder(got)
	notes, perm := indexOf(order, "entity_notes"), indexOf(order, "permissions")
	if notes < 0 || perm < 0 || notes > perm {
		t.Errorf("backfilled layout must have entity_notes before permissions, got %v", order)
	}

	// Idempotent: feed type 1's healed layout back; nothing should change.
	typeRepo.listAllFn = func(_ context.Context) ([]EntityType, error) {
		return []EntityType{{ID: 1, Layout: got}}, nil
	}
	n2, err := svc.EnsureEntityNotesBlockInDefaults(context.Background())
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if n2 != 0 {
		t.Errorf("second run must be a no-op, got %d updated", n2)
	}
}

// TestEnsureEntityNotesBlockInDefaults_AppendsWhenNoPermissionsRow pins the
// fallback: a layout with no permissions row gets entity_notes appended at
// the end rather than dropped.
func TestEnsureEntityNotesBlockInDefaults_AppendsWhenNoPermissionsRow(t *testing.T) {
	bare := EntityTypeLayout{Rows: []TemplateRow{
		{ID: "row-1", Columns: []TemplateColumn{{ID: "c1", Width: 12, Blocks: []TemplateBlock{{ID: "b-entry", Type: "entry"}}}}},
	}}
	var savedJSON string
	typeRepo := &mockEntityTypeRepo{
		listAllFn: func(_ context.Context) ([]EntityType, error) {
			return []EntityType{{ID: 7, Layout: bare}}, nil
		},
		updateLayoutFn: func(_ context.Context, _ int, layoutJSON string) error {
			savedJSON = layoutJSON
			return nil
		},
	}
	svc := newTestService(&mockEntityRepo{}, typeRepo).(*entityService)

	if _, err := svc.EnsureEntityNotesBlockInDefaults(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var got EntityTypeLayout
	if err := json.Unmarshal([]byte(savedJSON), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	order := blockTypeOrder(got)
	if indexOf(order, "entity_notes") != len(order)-1 {
		t.Errorf("entity_notes should be appended last when no permissions row exists, got %v", order)
	}
}
