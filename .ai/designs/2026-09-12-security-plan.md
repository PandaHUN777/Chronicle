# Security plan — what the 2026-09-12 research left open

**Executor model:** Sonnet `auditor` for the audits, Sonnet `go-dev` for the
fixes, Opus `reviewer` before every push. **Owner-facing summary:** one live
leak to close (real-time map updates ignore per-player rules), three areas
never audited to audit, one item that needs you (the Foundry key), and one
place where a decision record overstated what was built.

## S1 — Real-time updates leak what the page hides *(fix; confirmed)*
**What:** markers accept per-user `visibility_rules` (`allowed_users` /
`denied_users`). The HTTP list honours them (`maps/repository.go:216-232`).
The WebSocket publisher passes only the binary `IsDMOnly()` as audience
(`app/routes.go:991` markers, `:871` drawings), and the hub's one gate is
`RequiresDM` (`websocket/hub.go:167`). A marker hidden from playerX by rule
still reaches playerX's browser in full on create/update. Drawings: the
struct has no `VisibilityRules` field and `ListDrawings` never selects the
column (`drawing_repository.go:128-159`), so rules set on drawings do nothing
anywhere.
**Design (ADR-055 rule 3 applied to a channel):** the publisher computes the
audience — `dmOnly` plus the explicit allow/deny user sets — and the hub
filters per recipient. Do not broadcast-then-hope; do not send a redacted
stub (that is "evidence of hidden"). `Drawing` gains `VisibilityRules`;
`ListDrawings` selects it; the drawing read filter matches the marker one.
**Red first:** a hub test with two connected Players, one denied by rule; the
denied one receives nothing. Files: `internal/websocket/hub.go`,
`internal/app/routes.go` (publish sites), `internal/plugins/maps/{drawing.go,
drawing_repository.go}`.

## S2 — Media serving *(audit first; then decide)*
Signed URLs are HMAC over `fileID:expires` only (`media/signed_url.go`) —
**not bound to a user** — valid 1h (`media/handler.go:161,512-514`). Whoever
has the link has the image. Booked in May as "look over with a fine comb,"
never run; its stated 24h TTL is already wrong.
**Audit questions, in order:** (1) can a Player or visitor obtain a signed
URL for an image attached to an entity they cannot see — via any list, card,
gallery, export, or WS payload? (2) does `checkMediaAccess` (`:277-298`)
consult entity visibility or only campaign membership? (3) private campaign +
anonymous: 404 confirmed; public campaign + anonymous: what exactly is
served unsigned (`allowUnsignedAccess`, `:307-310`)? (4) is 1h the right TTL
for a bearer link? **Then** decide: bind to user (breaks nothing in-app if
the app always mints per-viewer), shorten TTL, or accept and document.

## S3 — AI workspace: what it can write once inside *(trace)*
Route gating is correct: all four routes Owner-only (`ai_workspace/routes.go:
33-38`). Untraced: whether `importer/committer.go` scopes every `Create` /
`Update` / `CreateEntityType` strictly to the campaign in the URL or trusts
client-supplied IDs. One read-only pass over the committer and the entity
service write methods it calls. If it trusts an ID, that is an IDOR in a
bulk-write tool and jumps to the top of this list.

## S4 — Public-campaign anonymous surface *(sweep)*
Well-commented after two incidents (#478 and its over-correction), never
systematically swept. ADR-055's sweep compared member roles, not anonymous.
One `auditor` pass: for each subsystem's list/read, does `RoleNone` on a
public campaign see only `everyone` content — entities, notes, timeline,
sessions (`sessions/routes.go:77` is public-capable), maps, media?

## S5 — Co-DM cannot open DM-only entities *(fixed under ADR-057 slice 1)*
Nine `CheckEntityAccess` callers pass raw `MemberRole`; the design says
`VisibilityRole()`. See `.ai/designs/2026-09-12-permissions-indicator.md`.

## S6 — A correction to ADR-055 §4
It says "session entity names **and the timeline count** go now." The session
fix is built (worktree `56ed8d91`, merging). The timeline `EventCount`
rules-delta (`timeline/repository.go:115-137`, self-documented) was **not**
built — the count runs in SQL and the per-user rules resolve in Go. Booked
here honestly; the fix is to count after the Go-side filter, or accept the
oracle for the rules-only case and say so on the ADR. Not silently dropped.

## S7 — ADR-054's held items *(needs the operator)*
Fog read/write, GM layers, map deletes stay coarser on the API than the web
until the operator reads **which user created the Foundry API key** on the
live instance. If it is the Owner account, role-from-creator is safe and
the three fixes ship as one PR with the ADR-054 contract test. Until then
they do not move.

## Order
S1 → S3 → S4 → S2 (S2 is the widest and least urgent: it needs a decision,
the others need a fix or a fact). S5 rides the permissions plan. S6 is a
one-line ADR amendment plus a booking. S7 waits on one fact from the operator.
