// safeident.go provides safe SQL identifier (table / column / index name)
// interpolation for DDL statements.
//
// MySQL/MariaDB does NOT support `?` placeholders for identifiers; DDL
// statements like `DROP TABLE` MUST interpolate the identifier into the SQL
// string. SafeIdent validates the identifier against a conservative regex
// matching the shape of legitimate Chronicle table/column names and wraps
// the result in backtick-quotes. Callers that interpolate identifiers into
// DDL MUST pass through SafeIdent first; a non-nil error means the
// identifier must NOT be used.

package database

import (
	"fmt"
	"regexp"
)

// safeIdentRe matches identifiers safe to interpolate into MySQL DDL after
// backtick-quoting: a leading letter or underscore (no leading digit — MySQL
// allows it but it's a useful tripwire), then letters/digits/underscores.
// Deliberately more conservative than what MySQL actually allows in quoted
// identifiers, to keep the helper's guarantee meaningful.
var safeIdentRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// SafeIdent validates the given string as a SQL identifier and returns it
// backtick-quoted for safe interpolation into DDL strings. Returns an error
// if the identifier doesn't match the conservative shape — callers MUST NOT
// fall back to raw interpolation on error.
//
// Use:
//
//	quoted, err := database.SafeIdent(tableName)
//	if err != nil { return err }
//	_, err = db.ExecContext(ctx, "DROP TABLE IF EXISTS "+quoted)
//
// The helper intentionally returns an error rather than sanitizing-and-
// continuing so that an invalid identifier surfaces loudly rather than
// silently mutating what the caller thought they were operating on.
func SafeIdent(s string) (string, error) {
	if s == "" {
		return "", fmt.Errorf("safe identifier: empty input")
	}
	if !safeIdentRe.MatchString(s) {
		return "", fmt.Errorf("safe identifier: %q does not match %s", s, safeIdentRe.String())
	}
	return "`" + s + "`", nil
}
