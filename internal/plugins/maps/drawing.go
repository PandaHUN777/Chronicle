package maps

import (
	"encoding/json"
	"time"

	"github.com/keyxmakerx/chronicle/internal/patch"
)

// Drawing represents a freehand drawing, shape, or text annotation on a map.
// Coordinates use percentage-based positioning (0-100) for resolution independence.
type Drawing struct {
	ID          string          `json:"id"`
	MapID       string          `json:"map_id"`
	LayerID     *string         `json:"layer_id,omitempty"`
	DrawingType string          `json:"drawing_type"` // freehand, rectangle, ellipse, polygon, text
	Points      json.RawMessage `json:"points"`       // Array of {x, y} coordinate pairs.
	StrokeColor string          `json:"stroke_color"`
	StrokeWidth float64         `json:"stroke_width"`
	FillColor   *string         `json:"fill_color,omitempty"`
	FillAlpha   float64         `json:"fill_alpha"`
	TextContent *string         `json:"text_content,omitempty"`
	FontSize    *int            `json:"font_size,omitempty"`
	Rotation    float64         `json:"rotation"`
	Visibility  string          `json:"visibility"` // everyone, dm_only
	// VisibilityRules mirrors Marker.VisibilityRules — per-player
	// allow/deny overrides (S1). The map_drawings.visibility_rules
	// column has existed since migration 002 (added alongside markers'
	// in the same statement) but was never selected or enforced
	// anywhere: a rule set on a drawing did nothing, not on the HTTP
	// list and not over the WebSocket feed. ListDrawings now selects
	// and enforces it (drawing_repository.go, matching ListMarkers'
	// predicate) and the WS publisher now honors it too (routes.go's
	// mapEventPublisherAdapter).
	VisibilityRules *string   `json:"visibility_rules,omitempty"`
	CreatedBy       *string   `json:"created_by,omitempty"`
	FoundryID       *string   `json:"foundry_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CreateDrawingInput is the validated input for creating a drawing.
type CreateDrawingInput struct {
	MapID       string
	LayerID     *string
	DrawingType string
	Points      json.RawMessage
	StrokeColor string
	StrokeWidth float64
	FillColor   *string
	FillAlpha   float64
	TextContent *string
	FontSize    *int
	Rotation    float64
	Visibility  string
	CreatedBy   string
	FoundryID   *string
}

// UpdateDrawingInput is the validated input for updating a drawing.
// ExpectedUpdatedAt is the optimistic-concurrency token (optional) and is
// NOT a data field, so it stays a plain pointer.
//
// PARTIAL update: absent preserves, explicit null clears, present replaces
// (contract ruled 2026-08-07, sweep R4; ADR-054 #6 — this struct shares
// UpdateTokenInput's shape and its unaudited history exactly). Before this
// every field but Points/StrokeColor/StrokeWidth/Visibility was assigned
// unguarded (`d.FillColor = input.FillColor`, `d.Rotation = input.Rotation`,
// …), so a caller sending only {points} to reshape a freehand stroke wiped
// its fill, its text content, its font size and its rotation on every call.
// No shipped caller does this today, but the route is reachable via syncapi
// (a Foundry-side drawing edit) with nothing stopping it.
type UpdateDrawingInput struct {
	Points            patch.Field[json.RawMessage]
	StrokeColor       patch.Field[string]
	StrokeWidth       patch.Field[float64]
	FillColor         patch.Field[string]
	FillAlpha         patch.Field[float64]
	TextContent       patch.Field[string]
	FontSize          patch.Field[int]
	Rotation          patch.Field[float64]
	Visibility        patch.Field[string]
	ExpectedUpdatedAt *time.Time
}

// Token represents a character, NPC, or object placed on a map.
// Tokens optionally link to Chronicle entities for cross-referencing.
type Token struct {
	ID             string          `json:"id"`
	MapID          string          `json:"map_id"`
	LayerID        *string         `json:"layer_id,omitempty"`
	EntityID       *string         `json:"entity_id,omitempty"`
	Name           string          `json:"name"`
	ImagePath      *string         `json:"image_path,omitempty"`
	X              float64         `json:"x"` // Percentage 0-100.
	Y              float64         `json:"y"`
	Width          float64         `json:"width"` // Grid units.
	Height         float64         `json:"height"`
	Rotation       float64         `json:"rotation"`
	Scale          float64         `json:"scale"`
	IsHidden       bool            `json:"is_hidden"` // GM-only visibility.
	IsLocked       bool            `json:"is_locked"`
	Bar1Value      *int            `json:"bar1_value,omitempty"`
	Bar1Max        *int            `json:"bar1_max,omitempty"`
	Bar2Value      *int            `json:"bar2_value,omitempty"`
	Bar2Max        *int            `json:"bar2_max,omitempty"`
	AuraRadius     *float64        `json:"aura_radius,omitempty"`
	AuraColor      *string         `json:"aura_color,omitempty"`
	LightRadius    *float64        `json:"light_radius,omitempty"`
	LightDimRadius *float64        `json:"light_dim_radius,omitempty"`
	LightColor     *string         `json:"light_color,omitempty"`
	VisionEnabled  bool            `json:"vision_enabled"`
	VisionRange    *float64        `json:"vision_range,omitempty"`
	Elevation      int             `json:"elevation"`
	SortOrder      int             `json:"sort_order"`
	StatusEffects  json.RawMessage `json:"status_effects,omitempty"`
	Flags          json.RawMessage `json:"flags,omitempty"`
	FoundryID      *string         `json:"foundry_id,omitempty"`
	CreatedBy      *string         `json:"created_by,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// CreateTokenInput is the validated input for placing a token on a map.
type CreateTokenInput struct {
	MapID          string
	LayerID        *string
	EntityID       *string
	Name           string
	ImagePath      *string
	X              float64
	Y              float64
	Width          float64
	Height         float64
	Rotation       float64
	Scale          float64
	IsHidden       bool
	IsLocked       bool
	Bar1Value      *int
	Bar1Max        *int
	Bar2Value      *int
	Bar2Max        *int
	AuraRadius     *float64
	AuraColor      *string
	LightRadius    *float64
	LightDimRadius *float64
	LightColor     *string
	VisionEnabled  bool
	VisionRange    *float64
	Elevation      int
	StatusEffects  json.RawMessage
	Flags          json.RawMessage
	CreatedBy      string
	FoundryID      *string
}

// UpdateTokenInput is the validated input for updating a token.
// ExpectedUpdatedAt is the optimistic-concurrency token (optional) and is
// NOT a data field, so it stays a plain pointer.
//
// PARTIAL update: absent preserves, explicit null clears, present replaces
// (contract ruled 2026-08-07, sweep R4; ADR-054 #6). Before this every field
// but Name was assigned unguarded — including fields that were ALREADY a Go
// pointer (Bar1Value, AuraRadius, ImagePath, …): a plain *T bound from JSON
// cannot tell "the caller omitted this key" from "the caller sent null", so
// the pointer type alone never protected anything. A drag PUT carrying only
// {x, y} zeroed IsHidden, IsLocked, both HP bars, and every aura/light/
// vision field on every single move — a GM's hidden ambush monster went
// visible to every player the instant someone nudged it half a pixel.
//
// Name is the one field deliberately left a plain string: UpdateToken only
// ever assigns it when the caller sends a non-empty value ("if input.Name
// != \"\" { … }"), so an absent/blank name was already preserved before
// this fix — it was never part of the blind-overwrite class.
type UpdateTokenInput struct {
	Name              string
	ImagePath         patch.Field[string]
	X                 patch.Field[float64]
	Y                 patch.Field[float64]
	Width             patch.Field[float64]
	Height            patch.Field[float64]
	Rotation          patch.Field[float64]
	Scale             patch.Field[float64]
	IsHidden          patch.Field[bool]
	IsLocked          patch.Field[bool]
	Bar1Value         patch.Field[int]
	Bar1Max           patch.Field[int]
	Bar2Value         patch.Field[int]
	Bar2Max           patch.Field[int]
	AuraRadius        patch.Field[float64]
	AuraColor         patch.Field[string]
	LightRadius       patch.Field[float64]
	LightDimRadius    patch.Field[float64]
	LightColor        patch.Field[string]
	VisionEnabled     patch.Field[bool]
	VisionRange       patch.Field[float64]
	Elevation         patch.Field[int]
	StatusEffects     patch.Field[json.RawMessage]
	Flags             patch.Field[json.RawMessage]
	ExpectedUpdatedAt *time.Time
}

// UpdateTokenPositionInput is a lightweight update for token position only.
// ExpectedUpdatedAt is the optimistic-concurrency token (optional). Drag
// pipelines that fire many position updates per second can simply omit it
// and accept last-writer-wins; deliberate "drop here" actions can include
// it to detect cross-user collisions.
type UpdateTokenPositionInput struct {
	X                 float64
	Y                 float64
	ExpectedUpdatedAt *time.Time
}

// Layer organizes map content into z-ordered groups (background, drawing, token, gm, fog).
type Layer struct {
	ID        string    `json:"id"`
	MapID     string    `json:"map_id"`
	Name      string    `json:"name"`
	LayerType string    `json:"layer_type"` // background, drawing, token, gm, fog
	SortOrder int       `json:"sort_order"`
	IsVisible bool      `json:"is_visible"`
	Opacity   float64   `json:"opacity"`
	IsLocked  bool      `json:"is_locked"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateLayerInput is the validated input for creating a layer.
type CreateLayerInput struct {
	MapID     string
	Name      string
	LayerType string
	SortOrder int
	IsVisible bool
	Opacity   float64
	IsLocked  bool
}

// UpdateLayerInput is the validated input for updating a layer.
// ExpectedUpdatedAt is the optimistic-concurrency token (optional) and is
// NOT a data field, so it stays a plain pointer.
//
// PARTIAL update: absent preserves, explicit null clears, present replaces
// (contract ruled 2026-08-07, sweep R4; ADR-054 #6 — same shape as
// UpdateTokenInput). Before this, SortOrder/IsVisible/IsLocked/Opacity were
// assigned unguarded, so reordering the layer stack (a SortOrder-only PUT)
// silently turned every OTHER layer's visibility and lock state off. No
// shipped caller does this today, but the route is reachable via syncapi.
//
// Name is deliberately left a plain string: UpdateLayer only ever assigns
// it when the caller sends a non-empty value, so an absent/blank name was
// already preserved before this fix.
type UpdateLayerInput struct {
	Name              string
	SortOrder         patch.Field[int]
	IsVisible         patch.Field[bool]
	Opacity           patch.Field[float64]
	IsLocked          patch.Field[bool]
	ExpectedUpdatedAt *time.Time
}

// FogRegion represents a revealed/hidden area of fog of war.
type FogRegion struct {
	ID         string          `json:"id"`
	MapID      string          `json:"map_id"`
	Points     json.RawMessage `json:"points"` // Polygon vertices.
	IsExplored bool            `json:"is_explored"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// CreateFogInput is the validated input for creating a fog region.
type CreateFogInput struct {
	MapID      string
	Points     json.RawMessage
	IsExplored bool
}
