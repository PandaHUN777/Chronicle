// Package syncapi — sync_cursor.go encodes the pull cursor for
// POST /api/v1/campaigns/:id/sync, letting a client resume a paged entity
// walk past syncMaxPullPages instead of re-scanning from the top each time.
//
// The cursor carries the next internal page number and is opaque on the
// wire, so the server can later switch to a keyset cursor without breaking
// clients. Offset paging is safe only because every ORDER BY in the
// entities repository ends in e.id ASC, giving a total order; without that
// tiebreaker a page walk would duplicate and skip rows.
package syncapi

import (
	"encoding/base64"
	"strconv"
	"strings"

	"github.com/keyxmakerx/chronicle/internal/apperror"
)

// syncCursorPrefix versions the cursor payload so a future keyset cursor can
// be told apart from this one rather than silently misparsed.
const syncCursorPrefix = "sync-v1:"

// syncMaxCursorPage bounds the decoded page number. At syncPageSize=100 this
// is ten million entities, far past any real campaign, and it stops a hostile
// or corrupt cursor from asking the database for an absurd OFFSET.
const syncMaxCursorPage = 100000

// encodeSyncCursor renders the next internal page number as an opaque token.
func encodeSyncCursor(page int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(syncCursorPrefix + strconv.Itoa(page)))
}

// decodeSyncCursor reads a cursor back into an internal page number. An empty
// cursor means "start at the beginning" and yields page 1, so a client that
// has never paged does not have to send the field at all.
//
// A malformed cursor is a 400 rather than a silent reset to page 1: resetting
// would restart the walk from the top and look like it worked, which is the
// same class of quiet lie the cursor exists to fix.
func decodeSyncCursor(cursor string) (int, error) {
	if cursor == "" {
		return 1, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, apperror.NewBadRequest("invalid sync cursor; send back the next_cursor value from the previous response")
	}
	payload := string(raw)
	if !strings.HasPrefix(payload, syncCursorPrefix) {
		return 0, apperror.NewBadRequest("unrecognized sync cursor version")
	}
	page, err := strconv.Atoi(strings.TrimPrefix(payload, syncCursorPrefix))
	if err != nil || page < 1 {
		return 0, apperror.NewBadRequest("invalid sync cursor page")
	}
	if page > syncMaxCursorPage {
		return 0, apperror.NewBadRequest("sync cursor is out of range")
	}
	return page, nil
}
