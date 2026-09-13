# Decision records — the items waiting on a call, and the call

One page each. These are **recommendations made so the operator's answer can
be one word**, not full plans. A full plan is written when an item's turn
comes; writing it earlier is how the 64,000 lines of dispatches happened.
Timeline V2 and the skypane widget are parked by the operator and are not
here.

## Notes / journal — "Obsidian built in"
**The question:** what that phrase means for Chronicle. **What exists:** a
floating notebook (`widgets/notes`), per-entity audience-scoped notes
(`widgets/entity_notes`), a journal page, entity backlinks, `{@…}` reference
markup. **The gap the operator feels is almost certainly that notes are not
pages** — they cannot link to each other, be linked to, sit in the sidebar
tree, or carry the permissions entities carry.
**Recommendation:** make a Note a first-class entity type (a system-provided
type, like Player Characters), so it inherits backlinks, mentions, the
permissions glance, sidebar placement and Foundry mirroring for free — and
retire the floating notebook once parity is reached. Do **not** build a
second wiki engine beside the entity engine.
**What confirms it:** one research pass on `widgets/notes` — how much of it
(checklists, locking, versions) has no entity equivalent. If more than a
little, the recommendation weakens. **Answer needed:** yes / no on the
direction.

## Rulebook
**The operator's words (2026-09-06):** "a behind-the-scenes massive index of
rules that other things can reference (hover-overs, cross-links), and a
highly interactive, easy-to-understand, well-documented rules page on top
of it." **Reality:** the index exists — Draw Steel's `data/` holds 519
abilities, 57 skills, 60 glossary terms, keywords, kits, ancestries — and
`{@category term}` + `reference-renderer.js` already give hover tooltips. The
rulebook *page* is hand-authored (8 entries), reads none of it, and hovers
resolve against only the 60 glossary terms.
**Recommendation:** the page becomes a generated view over `data/`: a table
of contents by category, each entry rendered by the reference renderer, and
hover resolution widened to all of `data/`, not the glossary alone. No new
index — the operator's "massive index" is the one already shipped; the work
is a seat and a renderer. **Design-first on a canvas**, per the operator's
own ruling. **Answer needed:** yes / no on "generated over data/, not
authored".

## Global search V2
**The question:** index strategy, permission filtering, ranking.
**Recommendation:** MariaDB FULLTEXT (the database you already run — no new
service), permission filtering applied *after* the query through the
existing `FilterViewableEntityIDs` pattern so search can never be a
visibility oracle, ranking by match then recency. The booked prerequisite
(a per-user filter gap in calendar search) is moot — the calendar is gone.
**Answer needed:** none; this is technical. It waits only on being next.

## Widget extensions (Phase Q) — **decided**
**The question:** a new `Chronicle.registerWidget` API or reuse
`Chronicle.register`. **The call:** reuse. System packages already ship
widgets on `Chronicle.register('slug', {init, destroy})` and it works; a
second API is a second contract to keep in step. An extension widget is the
same call plus a manifest declaration, and logic that needs sandboxing goes
through the existing WASM layer (Phase R, already built). Recorded here; an
ADR when the first extension widget lands.

## NPC presence (E4/E5)
No decision pending — the operator settled the signal source ("both") and
the order ("after the current queue, E4+E5 paired last") in June. Its turn
has not come. Nothing to answer.

## Theme rebuild (`C-THEME-V2`)
**Ready.** Pre-scoped to line numbers in May (`Cordinator/plans/BACKLOG.md:
930-973`): cleanup A/B, real expansion C, preview rebuild. Operator called
it back-burner then. **Answer needed:** a green light, and it slots after
the header slices, which share the Appearance surface.

## AI workspace destructive operations
**Ready to plan.** The operator's load-bearing constraint is recorded
(`BACKLOG.md:288-308`): an unmissable red warning band, per-row confirm,
audit-restore hooks. Sequence is already fixed — audit (S3 in the security
plan) → design conversation → implementation. **Answer needed:** none until
S3 reports.
