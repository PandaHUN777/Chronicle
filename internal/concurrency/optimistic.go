// Package concurrency provides optimistic-concurrency primitives shared
// across plugins. Callers send their last-known UpdatedAt timestamp on a
// mutation request; the service compares against the row's current
// timestamp and rejects with 409 Conflict if the row has been modified
// since.
//
// The wire shape is a JSON request body field `expected_updated_at` of
// type `*time.Time` (nullable — omitting it falls back to last-writer-wins).
package concurrency

import (
	"time"

	"github.com/keyxmakerx/chronicle/internal/apperror"
)

// Check rejects a mutation when the caller's expected UpdatedAt is older
// than the row's current UpdatedAt. Returns nil when the caller didn't
// supply an expected timestamp (last-writer-wins fallback) or when the
// timestamps match.
//
// label is the human-readable resource name embedded in the conflict
// message (e.g. "marker", "drawing"). Keep it short and lowercase.
//
// Comparison uses After rather than !Equal so that submitting the exact
// row timestamp is treated as "still current" — the row's tick can only
// move forward, so any forward movement is a conflict and any equal
// reading is fine.
func Check(current time.Time, expected *time.Time, label string) error {
	if expected == nil {
		return nil
	}
	if current.After(*expected) {
		return apperror.NewConflict(label + " was modified by another user; refresh and retry")
	}
	return nil
}
