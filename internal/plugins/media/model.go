// Package media manages file uploads, storage, and serving for Chronicle.
// Supports image uploads with automatic thumbnail generation at multiple sizes.
// Files are stored on the local filesystem in a date-based directory structure.
package media

import (
	"path/filepath"
	"strings"
	"time"
)

// MediaFile represents an uploaded file stored on disk.
type MediaFile struct {
	ID             string            `json:"id"`
	CampaignID     *string           `json:"campaign_id,omitempty"`
	UploadedBy     string            `json:"uploaded_by"`
	Filename       string            `json:"filename"`       // UUID-based filename on disk.
	OriginalName   string            `json:"original_name"`  // User's original filename.
	MimeType       string            `json:"mime_type"`
	FileSize       int64             `json:"file_size"`
	// ContentHash is the sha256 of the original file bytes (hex). Populated
	// at upload time (after MIME validation, before disk write) so a
	// later upload of identical bytes within the same campaign reuses
	// this row instead of duplicating storage. Nullable on the DB side
	// for legacy rows that pre-date the column — they get backfilled
	// at startup.
	ContentHash    string            `json:"content_hash,omitempty"`
	UsageType      string            `json:"usage_type"`     // attachment, entity_image, avatar, backdrop.
	ThumbnailPaths map[string]string `json:"thumbnail_paths"` // size -> filename (e.g., "300" -> "uuid_300.jpg").
	CreatedAt      time.Time         `json:"created_at"`

	// CampaignIsPublic is populated by FindByID via a LEFT JOIN on campaigns.
	// nil means the file has no campaign (avatars, backdrops). Used by the
	// serve handler to enforce access control on private campaign media.
	CampaignIsPublic *bool `json:"-"`

	// MatchedExisting and UsedBy are transient, ADR-058 decision 5 fields —
	// never persisted (Create/FindByID/etc. never populate them). Set ONLY
	// by mediaService.Upload's dedup path, and ONLY when the content-hash
	// match was safe to merge (canMergeWithExisting said yes): MatchedExisting
	// marks that this call reused an existing row instead of writing a new
	// one, and UsedBy names every page already using it — decision 4's
	// "where is this used" data, reused here.
	//
	// A REFUSED merge (the uploader can't see every referencing page) never
	// touches either field — the MediaFile it returns is a genuinely fresh
	// row built the same way an ordinary upload is, so both stay at their
	// zero value. That is load-bearing, not incidental: json:"-" keeps them
	// out of any accidental whole-struct serialization, but the real
	// guarantee is that a refused merge's response is byte-for-byte what an
	// ordinary upload's response would be. Telling the uploader "this
	// matched a file you can't see" would let them fingerprint a hidden
	// page's artwork with their own candidate images — see UploadResponse.
	MatchedExisting bool       `json:"-"`
	UsedBy          []MediaRef `json:"-"`
}

// UploadInput holds the validated input for creating a media file.
type UploadInput struct {
	CampaignID   string
	UploadedBy   string
	OriginalName string
	MimeType     string
	FileSize     int64
	UsageType    string
	FileBytes    []byte
}

// UploadResponse is the JSON response returned after a successful upload.
type UploadResponse struct {
	ID           string `json:"id"`
	URL          string `json:"url"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	MimeType     string `json:"mime_type"`
	FileSize     int64  `json:"file_size"`

	// Deduplicated and UsedBy surface ADR-058 decision 5's SAFE merge case:
	// this upload matched an existing file's content hash and the uploader
	// could already see every page using it, so it was merged rather than
	// stored again. UsedBy is decision 4's "where is this used" list, reused
	// here rather than rebuilt, and needs no separate filtering pass — a
	// safe merge already proves the uploader can see all of it.
	//
	// A REFUSED merge (the uploader can't see at least one referencing page)
	// leaves both fields at their zero value, `omitempty` drops them from
	// the JSON entirely, and the rest of this struct is populated exactly
	// like an ordinary upload's. That is deliberate, not an oversight: this
	// response must never let an uploader distinguish "matched a file you
	// can't see" from "no match at all" — the former would confirm a hidden
	// page's artwork exists by trying candidate images, which is the ADR-055
	// rule 3 leak this whole arc exists to close.
	Deduplicated bool       `json:"deduplicated,omitempty"`
	UsedBy       []MediaRef `json:"used_by,omitempty"`
}

// --- MIME Type Validation ---

// AllowedMimeTypes defines which MIME types are accepted for upload.
var AllowedMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
	// Audio types for note attachments.
	"audio/mpeg": true,
	"audio/ogg":  true,
	"audio/wav":  true,
	"audio/webm": true,
}

// MimeToExtension maps MIME types to file extensions.
var MimeToExtension = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
	"audio/mpeg": ".mp3",
	"audio/ogg":  ".ogg",
	"audio/wav":  ".wav",
	"audio/webm": ".webm",
}

// IsImage returns true if the file is an image based on MIME type.
func (f *MediaFile) IsImage() bool {
	return strings.HasPrefix(f.MimeType, "image/")
}

// Extension returns the file extension for this media file.
func (f *MediaFile) Extension() string {
	if ext, ok := MimeToExtension[f.MimeType]; ok {
		return ext
	}
	return filepath.Ext(f.OriginalName)
}

// Usage type constants.
const (
	UsageAttachment  = "attachment"
	UsageEntityImage = "entity_image"
	UsageAvatar      = "avatar"
	UsageBackdrop    = "backdrop"
)

// MediaRef is a lightweight reference from an entity to a media file.
// Used by the campaign media browser to show which entities use each file,
// AND (ADR-058) by checkMediaAccess to decide whether a file inherits an
// entity's visibility instead of falling back to plain campaign membership.
type MediaRef struct {
	EntityID   string `json:"entity_id"`
	EntityName string `json:"entity_name"`
	EntitySlug string `json:"entity_slug"`
	RefType    string `json:"ref_type"` // "image" (entity image_path or cover_image_path) or "content" (in editor HTML).
}

// CampaignMediaStats holds aggregate storage stats scoped to one campaign.
type CampaignMediaStats struct {
	TotalFiles int
	TotalBytes int64
}
