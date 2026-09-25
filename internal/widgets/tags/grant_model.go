package tags

import "time"

// TagPermission is a visibility grant carried by a tag. An entity bearing a
// tag that has grants becomes visible to the grant's subjects even if it
// would otherwise be hidden (dm_only / custom-without-you). Grants are
// additive only — a tag can widen visibility, never narrow it. Untag the
// entity or revoke the grant and the entity re-hides.
type TagPermission struct {
	ID          int       `json:"id"`
	TagID       int       `json:"tagId"`
	SubjectType string    `json:"subjectType"`
	SubjectID   string    `json:"subjectId"`
	CreatedBy   string    `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Grant subject types. These mirror the entities plugin's entity_permissions
// subject vocabulary so the visibility filter treats both grant tables
// identically.
const (
	// SubjectRole grants the tag's entities to all members at or above a role
	// level (subject_id is the role int as text: "1"=Player, "2"=Scribe).
	SubjectRole = "role"
	// SubjectUser grants to a specific user (subject_id is the user UUID).
	SubjectUser = "user"
	// SubjectGroup grants to all members of a campaign group (subject_id is the
	// group int as text).
	SubjectGroup = "group"
	// SubjectPublic grants to EVERYONE, including logged-out/anonymous visitors
	// on a public campaign — strictly wider than SubjectRole "1" (Player =
	// authenticated members). subject_id is unused and normalized to "".
	SubjectPublic = "public"
)

// CreateTagPermissionRequest is the JSON body for granting a tag to a subject.
type CreateTagPermissionRequest struct {
	SubjectType string `json:"subjectType"`
	SubjectID   string `json:"subjectId"`
}

// EntityTagGrant is one tag-derived grant on a single entity, used by the
// effective-visibility glance. It carries the owning tag's display fields plus
// the resolved subject so the badge tooltip can say e.g. "Visible to Players
// via ‹revealed-act-1›". SubjectLabel is the human label (role name / member
// display name / group name); SubjectType/SubjectID are the raw grant subject.
type EntityTagGrant struct {
	TagID        int    `json:"tagId"`
	TagName      string `json:"tagName"`
	TagSlug      string `json:"tagSlug"`
	TagColor     string `json:"tagColor"`
	SubjectType  string `json:"subjectType"`
	SubjectID    string `json:"subjectId"`
	SubjectLabel string `json:"subjectLabel"`
}
