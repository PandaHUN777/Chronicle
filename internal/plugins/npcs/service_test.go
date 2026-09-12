package npcs

import (
	"context"
	"testing"

	"github.com/keyxmakerx/chronicle/internal/permissions"
)

// --- Mocks ---
//
// TEST HONESTY (see internal/app/error_handler_api_type_test.go's header,
// and internal/app/armory_npcs_visibility_leak_test.go): these mocks stub
// away the REPOSITORY's SQL and the entities plugin's real visibility
// predicate, so nothing here proves the real SQL enforces visibility — that
// is what the real-database test in internal/app proves, against the real
// entities repository. What IS legitimately unit-testable here, without a
// database, is the SERVICE's own orchestration: does it call the visibility
// filter for the right roles, does it fail closed with no filter wired, and
// do ListNPCs/CountNPCs agree with each other for the same inputs. These
// tests are the ones that would NOT have caught the original leak (a mock of
// the predicate is exactly as trustworthy as the mock says it is) — they
// only guard the code that decides WHEN to consult the real predicate.

type mockNPCRepo struct {
	listIDsFn    func(ctx context.Context, campaignID string, characterTypeID int, opts NPCListOptions) ([]string, error)
	getByIDsFn   func(ctx context.Context, campaignID string, ids []string) ([]NPCCard, error)
	lastByIDsArg []string // records the ids GetNPCCardsByIDs was called with, for pagination assertions
}

func (m *mockNPCRepo) ListRevealedIDs(ctx context.Context, campaignID string, characterTypeID int, opts NPCListOptions) ([]string, error) {
	if m.listIDsFn != nil {
		return m.listIDsFn(ctx, campaignID, characterTypeID, opts)
	}
	return nil, nil
}

func (m *mockNPCRepo) GetNPCCardsByIDs(ctx context.Context, campaignID string, ids []string) ([]NPCCard, error) {
	m.lastByIDsArg = ids
	if m.getByIDsFn != nil {
		return m.getByIDsFn(ctx, campaignID, ids)
	}
	cards := make([]NPCCard, len(ids))
	for i, id := range ids {
		cards[i] = NPCCard{ID: id, Name: id}
	}
	return cards, nil
}

type mockTypeFinder struct {
	typeID int
	err    error
}

func (m *mockTypeFinder) FindCharacterTypeID(_ context.Context, _ string) (int, error) {
	return m.typeID, m.err
}

// mockVisibilityFilter records every call it receives and returns a
// caller-supplied viewable set, so tests can assert exactly which ids were
// sent to it (never more than the candidate set) and control the outcome.
type mockVisibilityFilter struct {
	viewable   map[string]bool
	err        error
	calls      int
	lastRole   int
	lastUserID string
	lastEntIDs []string
}

func (m *mockVisibilityFilter) FilterViewableEntityIDs(_ context.Context, _ string, entityIDs []string, role int, userID string) (map[string]bool, error) {
	m.calls++
	m.lastRole = role
	m.lastUserID = userID
	m.lastEntIDs = entityIDs
	if m.err != nil {
		return nil, m.err
	}
	return m.viewable, nil
}

// --- Tests ---

func TestListNPCs_ReturnsCards(t *testing.T) {
	repo := &mockNPCRepo{
		listIDsFn: func(_ context.Context, _ string, typeID int, _ NPCListOptions) ([]string, error) {
			if typeID != 42 {
				t.Fatalf("expected typeID=42, got %d", typeID)
			}
			return []string{"npc-1", "npc-2"}, nil
		},
		getByIDsFn: func(_ context.Context, _ string, ids []string) ([]NPCCard, error) {
			names := map[string]string{"npc-1": "Gandalf", "npc-2": "Aragorn"}
			cards := make([]NPCCard, len(ids))
			for i, id := range ids {
				cards[i] = NPCCard{ID: id, Name: names[id]}
			}
			return cards, nil
		},
	}
	// role=1 (Player) — no restricted ids in this fixture, so a filter that
	// allows everything it's asked about behaves like "no filtering happened".
	vf := &mockVisibilityFilter{viewable: map[string]bool{"npc-1": true, "npc-2": true}}
	svc := NewNPCService(repo, &mockTypeFinder{typeID: 42}, vf)

	cards, total, err := svc.ListNPCs(context.Background(), "campaign-1", permissions.RolePlayer, "user-1", DefaultNPCListOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total=2, got %d", total)
	}
	if len(cards) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(cards))
	}
	if cards[0].Name != "Gandalf" {
		t.Fatalf("expected first card name=Gandalf, got %s", cards[0].Name)
	}
}

func TestListNPCs_EmptyWhenNoCharacterType(t *testing.T) {
	svc := NewNPCService(&mockNPCRepo{}, &mockTypeFinder{err: context.DeadlineExceeded}, nil)

	_, _, err := svc.ListNPCs(context.Background(), "campaign-1", permissions.RolePlayer, "user-1", DefaultNPCListOptions())
	if err == nil {
		t.Fatal("expected error when character type not found")
	}
}

func TestCountNPCs(t *testing.T) {
	repo := &mockNPCRepo{
		listIDsFn: func(_ context.Context, _ string, typeID int, _ NPCListOptions) ([]string, error) {
			ids := make([]string, 7)
			for i := range ids {
				ids[i] = string(rune('a' + i))
			}
			return ids, nil
		},
	}
	// role=3 (Owner) bypasses the filter entirely — unchanged pre-fix behaviour.
	svc := NewNPCService(repo, &mockTypeFinder{typeID: 10}, nil)

	count, err := svc.CountNPCs(context.Background(), "campaign-1", permissions.RoleOwner, "owner-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 7 {
		t.Fatalf("expected count=7, got %d", count)
	}
}

func TestNPCListOptions_Offset(t *testing.T) {
	tests := []struct {
		page    int
		perPage int
		want    int
	}{
		{1, 24, 0},
		{2, 24, 24},
		{3, 10, 20},
		{0, 24, 0}, // clamped to 0
	}
	for _, tt := range tests {
		opts := NPCListOptions{Page: tt.page, PerPage: tt.perPage}
		if got := opts.Offset(); got != tt.want {
			t.Errorf("page=%d perPage=%d: expected offset=%d, got %d", tt.page, tt.perPage, tt.want, got)
		}
	}
}

func TestNPCListOptions_OrderByClause(t *testing.T) {
	tests := []struct {
		sort string
		want string
	}{
		{"name", "ORDER BY e.name ASC"},
		{"updated", "ORDER BY e.updated_at DESC"},
		{"created", "ORDER BY e.created_at DESC"},
		{"invalid", "ORDER BY e.name ASC"},
	}
	for _, tt := range tests {
		opts := NPCListOptions{Sort: tt.sort}
		if got := opts.OrderByClause(); got != tt.want {
			t.Errorf("sort=%s: expected %q, got %q", tt.sort, tt.want, got)
		}
	}
}

func TestNPCCard_FieldString(t *testing.T) {
	card := &NPCCard{
		Fields: map[string]any{
			"race":  "Elf",
			"level": 5,
		},
	}

	if got := card.FieldString("race"); got != "Elf" {
		t.Errorf("expected race=Elf, got %s", got)
	}
	if got := card.FieldString("missing"); got != "" {
		t.Errorf("expected empty for missing key, got %s", got)
	}
	if got := card.FieldString("level"); got != "" {
		t.Errorf("expected empty for non-string value, got %s", got)
	}

	// Nil fields should return empty.
	nilCard := &NPCCard{}
	if got := nilCard.FieldString("anything"); got != "" {
		t.Errorf("expected empty for nil fields, got %s", got)
	}
}

func TestTogglePrivate_EntityService(t *testing.T) {
	// This tests that TogglePrivate is part of the service interface.
	// The actual implementation is tested in the entities package.
	// Here we just verify the mock VisibilityToggler interface works.
	toggler := &mockToggler{newPrivate: true}
	newPrivate, err := toggler.TogglePrivate(context.Background(), "entity-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !newPrivate {
		t.Fatal("expected newPrivate=true")
	}
}

type mockToggler struct {
	newPrivate bool
	err        error
}

func (m *mockToggler) TogglePrivate(_ context.Context, _ string) (bool, error) {
	return m.newPrivate, m.err
}

// --- Visibility orchestration tests (finding 2 regression coverage) ---

// TestVisibleNPCIDs_OnlyOwnerBypassesFilter pins the bypass at OWNER, and the
// history matters. The fix for finding 2 originally bypassed at Scribe, on the
// reading that Scribe behaviour was out of scope. It is not the same question:
// the out-of-scope item is whether a co-DM should be PROMOTED in these plugins
// (booked in .ai/todo.md), and a co-DM arrives here as RolePlayer, below both
// thresholds either way.
//
// Scribe must go through the filter because the canonical policy says so.
// visibilityFilter (entities/repository.go:1177) returns an empty predicate
// only for role >= RoleOwner; for a Scribe it still evaluates, and its custom
// branch requires a matching grant. So a visibility='custom' entity is NOT
// automatically visible to a Scribe. Bypassing here would have left the
// gallery showing a Scribe the name and artwork of a page they cannot open --
// the same disagreement finding 2 was about, in a narrower audience.
//
// A Scribe still sees every is_private page, because the filter's default
// branch admits role >= 2. Only custom-without-grant is withheld.
func TestVisibleNPCIDs_OnlyOwnerBypassesFilter(t *testing.T) {
	t.Run("owner bypasses entirely", func(t *testing.T) {
		repo := &mockNPCRepo{
			listIDsFn: func(_ context.Context, _ string, _ int, _ NPCListOptions) ([]string, error) {
				return []string{"public-npc", "restricted-npc"}, nil
			},
		}
		vf := &mockVisibilityFilter{viewable: map[string]bool{}} // would hide everything if consulted
		svc := &npcService{repo: repo, typeFinder: &mockTypeFinder{}, entityVisibility: vf}

		role := permissions.RoleOwner
		ids, err := svc.visibleNPCIDs(context.Background(), "camp-1", 1, role, "u1", NPCListOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vf.calls != 0 {
			t.Errorf("an Owner must not be filtered at all, got %d filter calls", vf.calls)
		}
		if len(ids) != 2 {
			t.Errorf("an Owner must see both ids, got %v", ids)
		}
	})

	t.Run("scribe is filtered, not bypassed", func(t *testing.T) {
		repo := &mockNPCRepo{
			listIDsFn: func(_ context.Context, _ string, _ int, _ NPCListOptions) ([]string, error) {
				return []string{"public-npc", "restricted-npc"}, nil
			},
		}
		// The canonical filter admits the public one and withholds the
		// custom-restricted one, which is what it does for a real Scribe with
		// no matching grant.
		vf := &mockVisibilityFilter{viewable: map[string]bool{"public-npc": true}}
		svc := &npcService{repo: repo, typeFinder: &mockTypeFinder{}, entityVisibility: vf}

		role := permissions.RoleScribe
		ids, err := svc.visibleNPCIDs(context.Background(), "camp-1", 1, role, "u1", NPCListOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vf.calls != 1 {
			t.Errorf("a Scribe must be run through the canonical filter exactly once, got %d calls", vf.calls)
		}
		if len(ids) != 1 || ids[0] != "public-npc" {
			t.Errorf("a Scribe must not see the custom-restricted id, got %v", ids)
		}
		if vf.lastRole != permissions.RoleScribe {
			t.Errorf("the Scribe's own role must reach the filter, got %d", vf.lastRole)
		}
	})
}

// TestVisibleNPCIDs_PlayerAndAnonymousUseCanonicalFilter proves the service
// consults EntityVisibilityFilter (not a hand-rolled is_private check) for
// Player and anonymous, and narrows to exactly what it reports viewable.
func TestVisibleNPCIDs_PlayerAndAnonymousUseCanonicalFilter(t *testing.T) {
	for _, tc := range []struct {
		role   int
		userID string
	}{
		{permissions.RolePlayer, "player-1"},
		{permissions.RoleNone, ""},
	} {
		repo := &mockNPCRepo{
			listIDsFn: func(_ context.Context, _ string, _ int, _ NPCListOptions) ([]string, error) {
				return []string{"public-npc", "restricted-npc"}, nil
			},
		}
		vf := &mockVisibilityFilter{viewable: map[string]bool{"public-npc": true}}
		svc := &npcService{repo: repo, typeFinder: &mockTypeFinder{}, entityVisibility: vf}

		ids, err := svc.visibleNPCIDs(context.Background(), "camp-1", 1, tc.role, tc.userID, NPCListOptions{})
		if err != nil {
			t.Fatalf("role %d: unexpected error: %v", tc.role, err)
		}
		if vf.calls != 1 {
			t.Fatalf("role %d: expected EntityVisibilityFilter to be consulted exactly once, got %d", tc.role, vf.calls)
		}
		if vf.lastRole != tc.role || vf.lastUserID != tc.userID {
			t.Errorf("role %d: filter called with role=%d userID=%q, want role=%d userID=%q", tc.role, vf.lastRole, vf.lastUserID, tc.role, tc.userID)
		}
		if len(ids) != 1 || ids[0] != "public-npc" {
			t.Errorf("role %d: expected only public-npc, got %v", tc.role, ids)
		}
	}
}

// TestVisibleNPCIDs_FailsClosedWithNoFilterWired is the defense-in-depth
// case: if the visibility gate is somehow not wired for a Player/anonymous
// viewer, the result must be EMPTY, never "everything" — the opposite of the
// pre-fix bug's failure direction.
func TestVisibleNPCIDs_FailsClosedWithNoFilterWired(t *testing.T) {
	repo := &mockNPCRepo{
		listIDsFn: func(_ context.Context, _ string, _ int, _ NPCListOptions) ([]string, error) {
			return []string{"npc-1"}, nil
		},
	}
	svc := &npcService{repo: repo, typeFinder: &mockTypeFinder{}, entityVisibility: nil}
	ids, err := svc.visibleNPCIDs(context.Background(), "camp-1", 1, permissions.RolePlayer, "player-1", NPCListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected fail-CLOSED (no ids) with no visibility filter wired, got %v", ids)
	}
}

// TestListAndCountNPCs_NeverDisagree drives ListNPCs and CountNPCs off the
// same fixture and requires their totals to match, for every role — the
// "an inflated count is itself a leak" half of finding 2.
func TestListAndCountNPCs_NeverDisagree(t *testing.T) {
	candidateIDs := []string{"public-npc", "restricted-npc"}

	for _, tc := range []struct {
		name string
		role int
	}{
		{"anonymous", permissions.RoleNone},
		{"player", permissions.RolePlayer},
		{"scribe", permissions.RoleScribe},
		{"owner", permissions.RoleOwner},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockNPCRepo{
				listIDsFn: func(_ context.Context, _ string, _ int, _ NPCListOptions) ([]string, error) {
					return candidateIDs, nil
				},
			}
			vf := &mockVisibilityFilter{viewable: map[string]bool{"public-npc": true}}
			svc := NewNPCService(repo, &mockTypeFinder{typeID: 1}, vf)

			_, total, err := svc.ListNPCs(context.Background(), "camp-1", tc.role, "u1", DefaultNPCListOptions())
			if err != nil {
				t.Fatalf("ListNPCs: %v", err)
			}
			count, err := svc.CountNPCs(context.Background(), "camp-1", tc.role, "u1")
			if err != nil {
				t.Fatalf("CountNPCs: %v", err)
			}
			if total != count {
				t.Errorf("ListNPCs total=%d, CountNPCs=%d — must never disagree", total, count)
			}
		})
	}
}

// TestListNPCs_PaginatesTheFilteredSet proves pagination is applied AFTER
// visibility narrowing, not before — the pre-fix pagination happened in SQL
// against the unfiltered set, which is exactly the shape that let a
// restricted row occupy a page slot a visible row should have had.
func TestListNPCs_PaginatesTheFilteredSet(t *testing.T) {
	repo := &mockNPCRepo{
		listIDsFn: func(_ context.Context, _ string, _ int, _ NPCListOptions) ([]string, error) {
			return []string{"a", "restricted", "b", "c"}, nil
		},
	}
	vf := &mockVisibilityFilter{viewable: map[string]bool{"a": true, "b": true, "c": true}}
	svc := NewNPCService(repo, &mockTypeFinder{typeID: 1}, vf)

	opts := NPCListOptions{Page: 1, PerPage: 2, Sort: "name"}
	cards, total, err := svc.ListNPCs(context.Background(), "camp-1", permissions.RolePlayer, "player-1", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3 (restricted must not count)", total)
	}
	if len(cards) != 2 || cards[0].ID != "a" || cards[1].ID != "b" {
		t.Fatalf("page 1 = %v, want [a b] (restricted skipped, no gap left in the page)", repo.lastByIDsArg)
	}
}
