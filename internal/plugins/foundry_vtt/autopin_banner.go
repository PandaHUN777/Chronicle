package foundry_vtt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// Settings KV keys for the auto-pin banner.
//
// LatestAutoPinSummaryKey holds the JSON-serialized AutoPinSummary of
// the most recent install that auto-pinned campaigns; overwritten on
// every install (older summaries stay queryable via security_events).
//
// AutoPinBannerDismissedAtKey holds a Unix-second timestamp of the
// last dismissal. The banner shows iff the latest summary's
// timestamp is strictly greater than the dismissal timestamp.
const (
	LatestAutoPinSummaryKey     = "foundry_vtt.latest_autopin_summary"
	AutoPinBannerDismissedAtKey = "foundry_vtt.autopin_banner_dismissed_at"
)

// AutoPinSummarySchemaVersion is the current wire-shape version of
// AutoPinSummary's JSON serialization. Bump it whenever the struct
// gains a field callers can't safely ignore.
//
// Read path is lenient: SchemaVersion 0 (pre-versioning) is treated
// as 1; a version higher than this constant is rejected so a future
// Chronicle version can't silently drop data it doesn't understand.
const AutoPinSummarySchemaVersion = 1

// AutoPinSummary is the renderable bundle the admin banner displays.
// Populated by AutoPinOnInstall and serialized to the settings KV;
// read back by GetUnreadAutoPinSummary.
type AutoPinSummary struct {
	// SchemaVersion is the wire-shape version; see
	// AutoPinSummarySchemaVersion. `omitempty` so a zero-value summary
	// doesn't serialize as `"schema_version":0` and confuse the read path.
	SchemaVersion int `json:"schema_version,omitempty"`
	// PreviousVersion is the version campaigns were effectively
	// running before this install. The banner phrases it as the
	// version campaigns are now pinned TO.
	PreviousVersion string `json:"previous_version"`
	// NewVersion is the version that just got installed. The banner
	// says "you installed N; M campaigns were auto-pinned to <prev>
	// — bump them to <new> via..." linking back to admin actions.
	NewVersion string `json:"new_version"`
	// Affected is the count of campaigns auto-pinned by this install.
	Affected int `json:"affected"`
	// Timestamp is when the install fired the auto-pin (Unix seconds).
	// Drives the unread/dismissed comparison.
	Timestamp int64 `json:"timestamp"`
}

// storeAutoPinSummary serializes summary to the settings KV under
// LatestAutoPinSummaryKey, called by AutoPinOnInstall after the
// per-campaign fan-out completes. The summary is supplementary, so a
// nil kv or a write error doesn't abort the install.
//
// Always stamps SchemaVersion to the current value regardless of the
// caller's, so a struct built without setting it still serializes
// correctly.
func (s *service) storeAutoPinSummary(ctx context.Context, summary AutoPinSummary) error {
	if s.kv == nil {
		return nil // KV not wired (tests); skip silently
	}
	summary.SchemaVersion = AutoPinSummarySchemaVersion
	bytes, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshal autopin summary: %w", err)
	}
	return s.kv.Set(ctx, LatestAutoPinSummaryKey, string(bytes))
}

// GetUnreadAutoPinSummary returns the latest summary if unread, nil
// if no summary exists or the admin has already dismissed it.
// Read-only: doesn't touch the dismissal key.
func (s *service) GetUnreadAutoPinSummary(ctx context.Context) (*AutoPinSummary, error) {
	if s.kv == nil {
		return nil, nil
	}
	raw, err := s.kv.Get(ctx, LatestAutoPinSummaryKey)
	if err != nil || raw == "" {
		// The banner is supplementary, not load-bearing: treat a
		// missing key or any read error as "no summary to surface"
		// rather than aborting the page render.
		return nil, nil
	}
	var summary AutoPinSummary
	if err := json.Unmarshal([]byte(raw), &summary); err != nil {
		return nil, fmt.Errorf("parse stored autopin summary: %w", err)
	}

	// SchemaVersion 0 means a pre-versioning summary; treat as 1.
	// A version newer than we know is rejected so a downgraded
	// Chronicle binary can't silently drop data it doesn't understand.
	if summary.SchemaVersion == 0 {
		summary.SchemaVersion = 1
	}
	if summary.SchemaVersion > AutoPinSummarySchemaVersion {
		return nil, fmt.Errorf(
			"stored autopin summary schema_version=%d is newer than this Chronicle (max=%d); "+
				"upgrade Chronicle or clear the %q settings key to recover",
			summary.SchemaVersion, AutoPinSummarySchemaVersion, LatestAutoPinSummaryKey)
	}

	// A summary timestamp <= dismissed_at means the admin has already
	// acknowledged this install.
	dismissedRaw, _ := s.kv.Get(ctx, AutoPinBannerDismissedAtKey)
	if dismissedRaw != "" {
		dismissed, parseErr := strconv.ParseInt(dismissedRaw, 10, 64)
		if parseErr == nil && summary.Timestamp <= dismissed {
			return nil, nil
		}
	}
	return &summary, nil
}

// DismissAutoPinBanner stamps the current Unix timestamp into the
// dismissal key, so GetUnreadAutoPinSummary returns nil until a new
// install produces a summary with a fresher timestamp.
func (s *service) DismissAutoPinBanner(ctx context.Context) error {
	if s.kv == nil {
		return errors.New("settings KV not configured; banner state can't persist")
	}
	now := strconv.FormatInt(time.Now().Unix(), 10)
	if err := s.kv.Set(ctx, AutoPinBannerDismissedAtKey, now); err != nil {
		return fmt.Errorf("set dismissal timestamp: %w", err)
	}
	return nil
}
