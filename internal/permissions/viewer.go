package permissions

// Viewer names WHO a visibility filter is deciding for. "No authenticated
// user" and "trusted in-process caller with no request behind it" (ADR-049)
// are separate, unforgeable representations: `system` is unexported, so only
// SystemViewer (in this package) can set it, and RequestViewer can never
// produce a system viewer no matter how empty its user id is. An anonymous
// request therefore falls to the least-privileged path by construction, not
// by every call site remembering to check. An empty user id always means NO
// USER — never a sentinel, lookup key, or synthesised identity.
type Viewer struct {
	role   int
	userID string
	system bool
}

// RequestViewer builds the viewer for a caller that arrived over HTTP.
//
// An empty userID means ANONYMOUS: no user. It does not mean "every user", it
// does not mean "trusted", and it must never be used as a lookup key. A
// request can never become a system viewer through this constructor — that is
// the whole point of it.
func RequestViewer(role int, userID string) Viewer {
	return Viewer{role: role, userID: userID}
}

// SystemViewer builds the viewer for a trusted in-process caller that has no
// request identity behind it — a campaign export walking its own rows, or a
// widget picker already authorized at its route. It bypasses the per-user
// visibility layer deliberately and says so at the call site, which is the
// only place that trust can honestly be declared.
func SystemViewer(role int) Viewer {
	return Viewer{role: role, system: true}
}

// Role is the viewer's campaign role level (see the Role* constants).
func (v Viewer) Role() int { return v.role }

// UserID is the viewer's user id, empty for anonymous viewers and for system
// callers. Never use it as a lookup key without checking IsAnonymous first.
func (v Viewer) UserID() string { return v.userID }

// IsSystem reports whether this is a trusted in-process caller. Only
// SystemViewer can make this true.
func (v Viewer) IsSystem() bool { return v.system }

// IsAnonymous reports whether there is no user behind this viewer — a
// logged-out visitor on a public campaign. It is deliberately NOT the same
// question as IsSystem.
func (v Viewer) IsAnonymous() bool { return !v.system && v.userID == "" }

// SkipsPerUserRules reports whether the per-user visibility layer is bypassed
// wholesale for this viewer: an Owner/co-DM sees everything by role, and a
// system caller is trusted by construction.
//
// AN ANONYMOUS VIEWER IS NEITHER. That sentence is the fix — every visibility
// filter asks this one question instead of testing `userID == ""` itself.
func (v Viewer) SkipsPerUserRules() bool {
	return v.system || CanSeeDmOnly(v.role)
}

// DeniesAnonymous reports whether a non-empty deny list must also exclude an
// anonymous (empty) userID (ADR-049 amendment): a logged-out visitor cannot
// be proven not to be the specific player the list names, so a deny list
// that isn't provably clear of them denies them too. Every per-user
// visibility predicate that reads a DeniedUsers/denied_users list — map
// markers and drawings (Go and SQL), the WebSocket hub's audience gate, and
// timeline's canUserView — calls this before testing membership, so the
// rule lives in one place instead of four.
func DeniesAnonymous(deniedUsers []string, userID string) bool {
	return userID == "" && len(deniedUsers) > 0
}
