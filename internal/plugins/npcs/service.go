// service.go contains business logic for the NPC gallery. Resolves the
// campaign's character entity type and delegates listing to the repository.
package npcs

import (
	"context"
	"errors"
	"fmt"

	"github.com/keyxmakerx/chronicle/internal/apperror"
	"github.com/keyxmakerx/chronicle/internal/permissions"
)

// EntityTypeFinder resolves entity types by slug for a campaign.
// Implemented by the entities.EntityService — injected to avoid circular imports.
type EntityTypeFinder interface {
	FindCharacterTypeID(ctx context.Context, campaignID string) (int, error)
}

// EntityVisibilityFilter resolves which of a set of entity IDs a viewer
// (role + userID) may see, applying the entities plugin's own canonical
// visibility policy (default is_private, custom per-subject grants, tag
// grants). Wraps entities.EntityService.FilterViewableEntityIDs — the SAME
// method sessions and the relations widget already use — so the NPC gallery
// never hand-rolls its own copy of that predicate. That hand-rolled copy
// (`role < 2 AND is_private = false`) is finding 2 in
// .ai/designs/2026-09-12-security-audit-findings.md: it never consulted
// entities.visibility or entity_permissions, so a visibility='custom' NPC
// (whose is_private is untouched by SetEntityPermissions) stayed listed to
// Players and to anonymous visitors.
type EntityVisibilityFilter interface {
	FilterViewableEntityIDs(ctx context.Context, campaignID string, entityIDs []string, role int, userID string) (map[string]bool, error)
}

// TagLister fetches tags for a set of entity IDs in batch.
// Implemented by tags.TagService — injected to decorate NPC cards with tags.
type TagLister interface {
	ListTagsForEntities(ctx context.Context, entityIDs []string) (map[string][]TagInfo, error)
}

// TagInfo holds tag display data returned by TagLister.
type TagInfo struct {
	ID    int
	Name  string
	Slug  string
	Color string
}

// NPCService handles business logic for the NPC gallery.
type NPCService interface {
	// ListNPCs returns revealed character entities for the NPC gallery.
	ListNPCs(ctx context.Context, campaignID string, role int, userID string, opts NPCListOptions) ([]NPCCard, int, error)

	// CountNPCs returns the number of visible NPCs for badge/nav display.
	CountNPCs(ctx context.Context, campaignID string, role int, userID string) (int, error)
}

// npcService implements NPCService.
type npcService struct {
	repo             NPCRepository
	typeFinder       EntityTypeFinder
	tagLister        TagLister
	entityVisibility EntityVisibilityFilter
}

// NewNPCService creates a new NPC service. entityVisibility is the canonical
// visibility gate (see EntityVisibilityFilter) — required so a Player/
// anonymous viewer's list can never fall back to "everything visible" by
// construction; see visibleNPCIDs's fail-closed branch.
func NewNPCService(repo NPCRepository, typeFinder EntityTypeFinder, entityVisibility EntityVisibilityFilter) NPCService {
	return &npcService{repo: repo, typeFinder: typeFinder, entityVisibility: entityVisibility}
}

// SetTagLister injects the tag batch fetcher for card decoration.
func (s *npcService) SetTagLister(tl TagLister) {
	s.tagLister = tl
}

// ListNPCs resolves the character type, narrows the campaign's matching
// characters to what the viewer may see, and returns one page of cards.
// Returns an empty list if no character entity type exists for the campaign.
// The total is the size of that SAME narrowed set (see visibleNPCIDs), so a
// filtered list and its count can never disagree — the failure mode finding 2
// also covered (an inflated count is itself a leak under ADR-055 rule 3).
func (s *npcService) ListNPCs(ctx context.Context, campaignID string, role int, userID string, opts NPCListOptions) ([]NPCCard, int, error) {
	typeID, err := s.typeFinder.FindCharacterTypeID(ctx, campaignID)
	if err != nil {
		// No character entity type yet — return empty gallery instead of erroring.
		var appErr *apperror.AppError
		if errors.As(err, &appErr) && appErr.Code == 404 {
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("resolving character type: %w", err)
	}

	visibleIDs, err := s.visibleNPCIDs(ctx, campaignID, typeID, role, userID, opts)
	if err != nil {
		return nil, 0, err
	}
	total := len(visibleIDs)
	if total == 0 {
		return nil, 0, nil
	}

	pageIDs := paginateIDs(visibleIDs, opts.Offset(), opts.PerPage)
	cards, err := s.repo.GetNPCCardsByIDs(ctx, campaignID, pageIDs)
	if err != nil {
		return nil, 0, err
	}

	// Decorate with tags if available.
	if s.tagLister != nil && len(cards) > 0 {
		ids := make([]string, len(cards))
		for i := range cards {
			ids[i] = cards[i].ID
		}
		tagMap, err := s.tagLister.ListTagsForEntities(ctx, ids)
		if err == nil {
			for i := range cards {
				if infos, ok := tagMap[cards[i].ID]; ok {
					for _, t := range infos {
						cards[i].Tags = append(cards[i].Tags, NPCTagInfo(t))
					}
				}
			}
		}
	}

	return cards, total, nil
}

// CountNPCs resolves the character type and returns the revealed count,
// computed from the exact same visibleNPCIDs path ListNPCs uses — never a
// separate SQL COUNT that could drift from what the list actually shows.
// Returns 0 if no character entity type exists for the campaign.
func (s *npcService) CountNPCs(ctx context.Context, campaignID string, role int, userID string) (int, error) {
	typeID, err := s.typeFinder.FindCharacterTypeID(ctx, campaignID)
	if err != nil {
		var appErr *apperror.AppError
		if errors.As(err, &appErr) && appErr.Code == 404 {
			return 0, nil
		}
		return 0, fmt.Errorf("resolving character type: %w", err)
	}
	visibleIDs, err := s.visibleNPCIDs(ctx, campaignID, typeID, role, userID, NPCListOptions{})
	if err != nil {
		return 0, err
	}
	return len(visibleIDs), nil
}

// visibleNPCIDs returns the character entity IDs matching opts' search/tag
// filters for characterTypeID, narrowed to the viewer's visibility.
//
// Only an OWNER is unrestricted here (role >= permissions.RoleOwner) —
// that is the NPC gallery's existing behaviour, UNCHANGED by this fix. The
// gallery's own reveal mechanic (is_private toggle) and whether a co-DM
// should see unrevealed NPCs are separate, undecided product questions
// booked in .ai/todo.md ("does the co-DM promotion cross plugin lines?") and
// deliberately out of scope here.
//
// Everyone below Owner, Scribes included, is narrowed through the
// SAME canonical visibility policy the entities plugin itself applies
// (EntityVisibilityFilter), replacing the hand-rolled `is_private == false`
// check finding 2 flagged — so this gallery can no longer disagree with the
// entity page about what is hidden.
func (s *npcService) visibleNPCIDs(ctx context.Context, campaignID string, characterTypeID, role int, userID string, opts NPCListOptions) ([]string, error) {
	allIDs, err := s.repo.ListRevealedIDs(ctx, campaignID, characterTypeID, opts)
	if err != nil {
		return nil, err
	}
	// Bypass at OWNER, not Scribe, because that is where the canonical
	// policy bypasses: visibilityFilter (entities/repository.go) returns an
	// empty predicate only for role >= RoleOwner. For a Scribe it still
	// evaluates, and its custom branch requires an actual matching grant —
	// a visibility='custom' entity is NOT automatically visible to a
	// Scribe. Short-circuiting at Scribe here would have left this gallery
	// more permissive than the entity page for exactly the state finding 2
	// was about, which is the same disagreement in a narrower audience.
	if role >= permissions.RoleOwner || len(allIDs) == 0 {
		return allIDs, nil
	}

	if s.entityVisibility == nil {
		// Fail CLOSED: with no way to check visibility, a Player/anonymous
		// viewer must see nothing rather than everything.
		return nil, nil
	}
	viewable, err := s.entityVisibility.FilterViewableEntityIDs(ctx, campaignID, allIDs, role, userID)
	if err != nil {
		return nil, fmt.Errorf("filtering NPC visibility: %w", err)
	}
	filtered := make([]string, 0, len(allIDs))
	for _, id := range allIDs {
		if viewable[id] {
			filtered = append(filtered, id)
		}
	}
	return filtered, nil
}

// paginateIDs slices a sorted id list to the [offset, offset+perPage) window,
// clamped to the slice bounds. perPage <= 0 returns the whole remainder.
func paginateIDs(ids []string, offset, perPage int) []string {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(ids) {
		return nil
	}
	end := len(ids)
	if perPage > 0 && offset+perPage < end {
		end = offset + perPage
	}
	return ids[offset:end]
}
