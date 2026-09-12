// Package tags implements the tags widget for Chronicle. Tags are campaign-scoped
// labels that can be attached to entities for categorization and filtering.
// This widget provides CRUD operations for tags and the ability to associate
// tags with entities via a many-to-many join table (entity_tags).
//
// Tags are a Widget in Chronicle's three-tier extension architecture: they
// provide API endpoints for the frontend tag picker component and are mounted
// on entity profile pages.
package tags

import "time"

// Tag represents a campaign-scoped label that can be attached to entities.
// Tags have a unique slug within their campaign for URL-safe references.
// DmOnly tags are only visible to Scribes and Owners — Players cannot see them.
type Tag struct {
	ID         int       `json:"id"`
	CampaignID string    `json:"campaignId"`
	Name       string    `json:"name"`
	Slug       string    `json:"slug"`
	Color      string    `json:"color"`
	DmOnly     bool      `json:"dmOnly"`
	CreatedAt  time.Time `json:"createdAt"`
}

// EntityTag represents the many-to-many relationship between entities and tags.
// Each row in the entity_tags join table maps one entity to one tag.
type EntityTag struct {
	EntityID  string    `json:"entityId"`
	TagID     int       `json:"tagId"`
	CreatedAt time.Time `json:"createdAt"`
}

// --- Request DTOs (bound from HTTP requests) ---

// CreateTagRequest holds the data submitted when creating a new tag.
type CreateTagRequest struct {
	Name   string `json:"name" form:"name"`
	Color  string `json:"color" form:"color"`
	DmOnly bool   `json:"dmOnly" form:"dmOnly"`
}

// UpdateTagRequest holds the data submitted when updating an existing tag.
//
// PARTIAL update: absent preserves, present replaces (contract ruled
// 2026-08-07, sweep R4; ADR-056 names this struct the worst finding of the
// 2026-09-12 toggle-truth sweep). Before this, Color and DmOnly were plain
// value types with no way to represent "the caller did not send this key" —
// a rename request that only carried `name` still bound `color: ""` and
// `dmOnly: false` on decode, and Update wrote both unconditionally. That
// turned every DM-only tag public the next time anyone renamed it.
//
// Name stays a required plain string: Update rejects the whole call with
// 400 when the merged name is blank, so an absent name fails loudly
// instead of silently overwriting.
//
// Neither Color nor DmOnly has a distinct "clear" state — Color is NOT
// NULL with a programmatic default and DmOnly is a plain boolean column —
// so a bare pointer is enough: nil preserves, a value (including the zero
// value, explicitly sent) replaces.
type UpdateTagRequest struct {
	Name   string  `json:"name" form:"name"`
	Color  *string `json:"color" form:"color"`
	DmOnly *bool   `json:"dmOnly" form:"dmOnly"`
}

// UpdateTagInput is the service-layer form of UpdateTagRequest — identical
// shape, kept as a separate type so the service package boundary doesn't
// force callers to depend on a "Request"-named wire type. See
// UpdateTagRequest's doc comment for the partial-update contract.
type UpdateTagInput struct {
	Name   string
	Color  *string
	DmOnly *bool
}

// SetEntityTagsRequest holds the array of tag IDs to assign to an entity.
// This replaces all existing tags on the entity with the provided set.
type SetEntityTagsRequest struct {
	TagIDs []int `json:"tagIds"`
}
