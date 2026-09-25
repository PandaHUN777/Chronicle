// Package observability holds Chronicle's in-process self-observation: a small,
// bounded, in-memory record of recent server errors, readable through the admin
// diagnostics catalog without shelling into the container. It is distinct from
// the audit plugin, which records DB-backed user actions scoped to a campaign
// and user, not anonymous or infra-level failures.
//
// It is a LEAF (standard library only, no Chronicle imports) so the writers
// (internal/app, internal/middleware) and reader (internal/systems) cannot form
// an import cycle through it.
//
// It is NOT durable or complete: the ring lives in process memory, so a restart
// empties it and each replica has its own. It is a recent-errors window, not an
// audit trail.
package observability

import (
	"strings"
	"sync"
	"time"
)

// DefaultCapacity is how many entries the process-wide ring holds. Sized
// against the recording policy (see ShouldRecord), not traffic: only 5xx and
// explicitly unexpected errors are stored, so this is a deep history of real
// failures, not a few seconds of noise.
const DefaultCapacity = 256

// maxErrLen bounds a stored error string. A DB driver error can carry an entire
// failed statement including bound values, so this keeps the ring from holding
// secrets rather than relying solely on redaction at render time. Truncation is
// marked (see truncate) so a clipped message is never mistaken for a whole one.
const maxErrLen = 300

// maxPathLen bounds a stored path. A route TEMPLATE is always short, but the
// fallback concrete path (see PathFor) is raw, attacker-controlled request
// bytes up to net/http's request-line limit, so this must be bounded here
// rather than left to callers.
const maxPathLen = 200

// Kind classifies where an error came from, which is often more diagnostic
// than the status code: KindApp and KindRaw can both be a 500 on the wire but
// mean very different things to whoever fixes it.
type Kind string

const (
	// KindApp is an *apperror.AppError — a domain error the code raised on
	// purpose, carrying its own status and message.
	KindApp Kind = "app"
	// KindHTTP is an *echo.HTTPError — raised by the framework or by a
	// handler using Echo's own error type.
	KindHTTP Kind = "http"
	// KindRaw is an error that was NEITHER of the above. It reached the
	// central handler unwrapped, got the default 500, and is the class most
	// likely to be an actual bug rather than a handled condition.
	KindRaw Kind = "raw"
	// KindPanic is a recovered panic. It never reaches the central error
	// handler at all (see RecordPanic), so it is recorded separately.
	KindPanic Kind = "panic"
)

// Entry is one recorded error. It is deliberately a summary, not a transcript:
// no request body, headers, query string, client IP, or user id — nothing that
// would turn a diagnostic paste into a data disclosure.
type Entry struct {
	// Time is when the error was recorded, in the server's clock.
	Time time.Time
	// Status is the HTTP status that was (or will be) sent to the client.
	Status int
	// Method is the HTTP method. Cheap, non-identifying, and it separates a
	// failing GET from a failing POST on the same route.
	Method string
	// Path is a route TEMPLATE ("/campaigns/:id/entities/:slug") whenever the
	// router matched one — see PathFor for why the concrete path is not
	// stored. It is also what makes host.errors-summary able to collapse a
	// thousand failures on one route into one line.
	Path string
	// PathIsTemplate distinguishes a template from the fallback concrete path,
	// so the renderer can say which it is showing instead of leaving the
	// reader to guess whether ":id" is literal.
	PathIsTemplate bool
	// Kind is the error's provenance (see the Kind constants).
	Kind Kind
	// Err is the error's message, truncated to maxErrLen.
	Err string
}

// PathFor is a privacy decision kept in one reviewable place: Chronicle has
// routes whose path segments are live credentials (e.g. `/rsvp/:token`,
// `/join/:code`), so when the router matched a route, the TEMPLATE is stored,
// never the concrete path, which could leak a working token.
//
// The concrete path is used only when there is no template (router matched
// nothing) — a reachable, hostile path: it can be raw, unbounded,
// attacker-controlled request bytes. Ring.Record bounds it. The boolean tells
// callers which they got, and is also the signal that the value is untrusted.
func PathFor(routeTemplate, rawPath string) (string, bool) {
	if t := strings.TrimSpace(routeTemplate); t != "" {
		return t, true
	}
	return rawPath, false
}

// ShouldRecord is the recording policy: record 5xx and explicitly-unexpected
// errors, never ordinary 4xx. The ring is fixed-size and evicts oldest-first,
// so admitting routine 4xx noise (bots, stale bookmarks, expired CSRF tokens)
// would let it silently evict the 500s an operator actually needs. `unexpected`
// is an escape hatch for callers (e.g. panic recovery) that know something is
// anomalous regardless of status; nothing sets it from client input.
func ShouldRecord(status int, unexpected bool) bool {
	return unexpected || status >= 500
}

// Ring is a fixed-size, oldest-first-evicting buffer of Entry, safe for
// concurrent use. It is a plain mutex rather than a lock-free structure because
// writes only happen on the error path — a server writing here often enough for
// lock contention to matter has a much larger problem than this mutex.
type Ring struct {
	mu    sync.Mutex
	buf   []Entry
	next  int    // index of the next write
	full  bool   // whether the buffer has wrapped at least once
	total uint64 // every Record that passed the policy, INCLUDING evicted ones
}

// NewRing creates a ring holding capacity entries. capacity <= 0 is coerced to
// DefaultCapacity, since a zero-length ring would silently discard everything.
func NewRing(capacity int) *Ring {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	return &Ring{buf: make([]Entry, capacity)}
}

// Record stores an entry, evicting the oldest when the ring is full. It applies
// no policy of its own (callers filter with ShouldRecord) and never returns an
// error: recording must not be able to fail while already handling a failure.
func (r *Ring) Record(e Entry) {
	if r == nil {
		return // nil-safe: an unwired ring is a no-op, never a panic
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	e.Err = truncate(e.Err, maxErrLen)
	// Path can carry attacker-controlled request bytes (see PathFor), so it must
	// be bounded here rather than left to the caller.
	e.Path = truncate(e.Path, maxPathLen)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.next] = e
	r.next++
	if r.next == len(r.buf) {
		r.next = 0
		r.full = true
	}
	r.total++
}

// Snapshot is an immutable copy of the ring's state, newest first, plus the
// counters that distinguish "nothing has gone wrong" from "nothing is
// recording" from "so much went wrong that history was evicted".
type Snapshot struct {
	// Capacity is the ring's fixed size.
	Capacity int
	// Held is how many entries the ring currently contains (<= Capacity).
	Held int
	// Total is every error recorded since process start, including evicted
	// ones. Total > Held signals the shown window is incomplete.
	Total uint64
	// Entries is the requested slice of held entries, NEWEST FIRST.
	Entries []Entry
}

// Snapshot returns up to limit entries, newest first. limit <= 0 returns all
// held entries. It copies under the lock and returns owned memory, so a caller
// can render at its leisure while the server keeps recording.
func (r *Ring) Snapshot(limit int) Snapshot {
	if r == nil {
		return Snapshot{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	held := r.next
	if r.full {
		held = len(r.buf)
	}
	s := Snapshot{Capacity: len(r.buf), Held: held, Total: r.total}
	if limit <= 0 || limit > held {
		limit = held
	}
	s.Entries = make([]Entry, 0, limit)
	// Walk backwards from the most recently written slot. r.next points at the
	// NEXT write, so the newest entry is at next-1, wrapping into the tail of
	// the buffer once the ring has filled.
	for i := 0; i < limit; i++ {
		idx := (r.next - 1 - i + len(r.buf)*2) % len(r.buf)
		s.Entries = append(s.Entries, r.buf[idx])
	}
	return s
}

// defaultRing is the process-wide ring, allocated eagerly at package init so
// Record is safe from the very first request (boot errors matter most).
var defaultRing = NewRing(DefaultCapacity)

// RecordHTTPError records an error the central HTTP error handler is about to
// render, applying ShouldRecord. Returns whether it was stored, which is only
// used by tests — production callers deliberately ignore it so that recording
// can never influence what the handler does next.
func RecordHTTPError(status int, method, routeTemplate, rawPath string, kind Kind, err error) bool {
	if !ShouldRecord(status, kind == KindPanic) {
		return false
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	p, templated := PathFor(routeTemplate, rawPath)
	defaultRing.Record(Entry{
		Status:         status,
		Method:         method,
		Path:           p,
		PathIsTemplate: templated,
		Kind:           kind,
		Err:            msg,
	})
	return true
}

// RecordPanic records a recovered panic. It exists separately because a
// recovered panic never reaches the central error handler (internal/
// middleware/recovery.go writes its 500 directly and returns nil to Echo).
//
// Only the panic value is recorded, not the stack trace: the stack is already
// in the process log, and per-entry frames would blow the ring's memory budget.
func RecordPanic(method, routeTemplate, rawPath, panicValue string) {
	p, templated := PathFor(routeTemplate, rawPath)
	defaultRing.Record(Entry{
		Status:         500,
		Method:         method,
		Path:           p,
		PathIsTemplate: templated,
		Kind:           KindPanic,
		Err:            "panic: " + panicValue,
	})
}

// Recent returns a snapshot of the process-wide ring for the diagnostics.
func Recent(limit int) Snapshot { return defaultRing.Snapshot(limit) }

// truncate clips s to n bytes and marks that it did, so a clipped message is
// never mistaken for a complete one. ToValidUTF8 drops the partial rune a
// byte-slice cut can leave, so the result is always printable.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "") + "…[truncated]"
}
