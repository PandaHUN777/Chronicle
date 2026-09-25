# S2/S3/S4 audit findings — 2026-09-12

Produced by a 12-agent read-only audit workflow against the tree at
`4d852a85`. **17 findings.** Nine were reachability-tested by a second agent
whose instruction was to construct the attack path or quote the guard that
blocks it; **all nine came back reachable and NONE was proven blocked.** The
other eight were not tested — the workflow capped verification at three per
area, which was my error in scoping the run, not a judgement about them. They
are recorded here with their claims intact and marked UNTESTED. An untested
finding is unknown, not absent.

Two of the plan's own notes were measured wrong and are corrected below: the
media TTL is 1 hour, not 24, and "private campaign + anonymous = 404" is
false in the shipped configuration.

Nothing here is fixed yet except where noted. Ordered by what I would fix
first, not by which audit found it.

---

## 1. HIGH — anyone signed in can write into any campaign's media, and read back a private file
`internal/plugins/media/handler.go:119` · tested: REACHABLE

`POST /media/upload` takes `campaign_id` from the form body. Neither the
handler nor `mediaService.Upload` checks that the caller is a member of that
campaign. So any authenticated Chronicle user who knows a campaign UUID —
an ex-member, say — can write files into it. Worse, the dedup path returns an
EXISTING file's ID and a signed URL when the upload hashes to a file already
stored: upload a file you already have a copy of and you are handed a
credential for the private original.

This is the only finding in the set that is a WRITE. It goes first.

## 2. HIGH — the armory and NPC galleries ignore custom visibility
`internal/plugins/armory/repository.go:66`, `internal/plugins/npcs/repository.go:47`
tested: REACHABLE (twice, from two different audits)

Both galleries hand-roll their filter as `role < 2 AND is_private = false`
instead of using the canonical `visibilityFilter`. Switching an entity to
`visibility='custom'` never clears `is_private`, so a custom-restricted
entity stays listed — to Players, and on a public campaign to anonymous
visitors, who also get a signed URL for its artwork and a count that includes
it.

Note this is NOT the cross-plugin co-DM question booked in `.ai/todo.md`.
That question is whether a co-DM should see MORE. This is these two galleries
showing everyone content that is restricted, and it needs no ruling.

## 3. HIGH — a Player can list every media file in the campaign, with signed URLs
`internal/plugins/syncapi/routes.go:223` · tested: REACHABLE

`GET /api/v1/campaigns/:id/media` is gated on read permission, which a
session-authenticated Player has. The handler resolves no role and applies no
visibility filter, so a Player receives the id, original filename, size and a
one-hour signed URL for every media row in the campaign. Compare
`ListEntities` in the same package, which does resolve the role and thread it
into the filter.

## 4. HIGH — a dm_only timeline is served in full to an anonymous viewer
`internal/plugins/timeline/handler.go:121` · tested: REACHABLE

`Show`, `TimelineDataAPI` and `EmbedTimeline` gate only on campaign scope and
never apply the timeline's own visibility or its per-user rules. On a public
campaign a viewer with no account opens any timeline by id.

## 5. MEDIUM — an entity's image field accepts any media UUID, making page render a signing oracle
`internal/plugins/entities/service.go:924` · tested: REACHABLE

`UpdateImage` and `UpdateCoverImage` validate against path traversal but
never that the referenced file belongs to this campaign. The show template
then signs whatever id is stored. A Scribe or Owner of their own campaign who
knows a media UUID from a private campaign gets a working signed URL for it.
Requires knowing the UUID, which is not guessable — but findings 1, 2 and 3
are all ways to learn one.

## 6. MEDIUM — the AI import can publish an existing hidden entity from untrusted markdown
`internal/plugins/ai_workspace/importer/committer.go:477` · tested: REACHABLE

Both update paths always send `IsPrivate`, computed from the `visibility:`
front matter of the pasted markdown. Committing an import row against an
existing entity therefore re-decides that entity's visibility from text the
model wrote. The route is Owner-only, so this is not privilege escalation; it
is a hidden page being published by a commit the Owner did not read closely.

## 7. MEDIUM — the same AI import wipes the target's field data
`internal/plugins/ai_workspace/importer/committer.go:473` · tested: REACHABLE

The update sends a non-nil empty `FieldsData`, which replaces the stored map,
and a present `TypeLabel` patch carrying `""` whenever the front matter omits
`subcategory:`. So a commit against an existing entity silently empties its
type fields and clears its descriptor. The same call site cites the
partial-update contract correctly for `ParentID` and then breaks it for these
two. Data loss, not a leak, and the most likely of these to be noticed by a
user as "the app ate my page".

## 8. LOW — the committer's own comment describes behaviour it does not have
`internal/plugins/ai_workspace/importer/committer.go:96` · tested: confirmed

It says `Visibility=dm_only` is preserved on the entity's Visibility field via
Update. `UpdateEntityInput` has no Visibility member, the service never
assigns it, and the UPDATE statement does not mention the column. A
documentation defect, but the kind that makes the next reader trust a
guarantee that is not there.

---

## TESTED 2026-09-13 — three real, three not. Verdicts below.

**This section used to say "UNTESTED — treat as unknown." It is no longer
unknown.** Two agents reachability-tested all six against real code (real
MariaDB, real repositories and services, no fakes installed for the function
under test). Result: **3 CONFIRMED, 2 already fixed by ADR-058, 1 not a bug.**

The count also drops from "eight" to six — the task list carried a stale
number that nobody had recounted against this list.

**Do not read a verdict here as permanent.** Each one is true against the
tree it was measured on (commit at time of test: `eb45947a`). The Foundry
repo learned this the expensive way: a claim measured against another repo's
source is only true on the day it is measured.



- **CONFIRMED — REAL, UNFIXED.** MEDIUM `internal/plugins/entities/repository.go:1619` — the entity
  breadcrumb prints the full ancestor chain with no visibility filter.
  `FindAncestors` is a bare recursive CTE and `GetAncestors` is passed no role
  or user at all, so a hidden parent's name and link render to anyone who can
  see the child. This is the textbook shape ADR-055 rule 3 forbids.
- **REFUTED — already fixed; this anchor has drifted into a doc comment.**
  ADR-058 closed it: `timeline/service.go:867` (`SearchTimelines`) runs
  `filterTimelinesByUser` over every result from `repo.Search`. Pinned by
  `timeline/search_visibility_reachability_test.go` (PASSES). Original text:
  MEDIUM `internal/plugins/timeline/repository.go:251` — cross-plugin
  timeline search applies only the SQL role narrowing and returns results
  without passing them through the per-user filter that `List` uses, so a
  restricted timeline can be named to an anonymous viewer.
- **CONFIRMED — REAL, UNFIXED.** MEDIUM `internal/plugins/maps/repository.go:124` — the marker popup
  joins entities and selects the linked entity's name with no visibility
  predicate on the joined row. The marker's own visibility is filtered; the
  entity it points at is not, so an "everyone" marker can name a private page.
- **NOT A BUG — operator-ruled and shipped.** ADR-058 decision 7 is this
  finding, quoted almost verbatim, and landed in `922fd3b1`. The public-campaign
  carve-out was never meant for all media, only for files on pages the viewer can
  already see; `checkMediaAccess` now routes public campaigns through
  `checkEntityScopedAccess` with `RoleNone` for anonymous. Original text:
  MEDIUM `internal/plugins/media/handler.go:308` — on a public campaign
  `allowUnsignedAccess` returns true for every file, and the access check
  consults only campaign membership, never the visibility of the entity the
  file hangs off. Any media id in a public campaign is readable by the
  internet. There is a test pinning this deliberately, so decide whether it
  is intended for ALL media or only for media on visible pages.
- **CONFIRMED — REAL, UNFIXED.** MEDIUM/LOW `internal/plugins/media/handler.go:272` — a private
  campaign's file answers 403 to an anonymous caller while an unknown id
  answers 404. Two distinguishable responses is an existence oracle. The
  function has a defence-in-depth branch 17 lines below that deliberately
  returns not-found, and it is unreachable whenever a signer is configured,
  which is always in production.
- **NOT A BUG — already fixed in the same ADR-058 sweep (`922fd3b1`).**
  `sensitiveParams` contains `sig` and `expires`; verified end-to-end through the
  real middleware by `middleware/logging_endtoend_test.go` (PASSES). Original text:
  LOW `internal/middleware/logging.go:18` — the sensitive-parameter list
  omits `sig` and `expires`, so the request logger writes a live,
  user-unbound bearer credential for a private image into the log in
  plaintext, where it stays valid for the rest of its hour.

## The structural finding behind several of these

`checkMediaAccess` never consults entity visibility. Its whole input set is
the file's campaign, whether that campaign is public, the signature pair, the
caller's user id, campaign membership and site-admin. Membership is
role-blind. So **any member of a campaign, of any role, can read any image in
that campaign** regardless of which entity it hangs off — DM-only pages, custom-
restricted pages, GM map layers.

That is structural, not a missed branch: `media_files` carries no reference to
the entity using it. The only reverse mapping is a query used by one
Owner-only fragment, and it omits the cover-image column. Fixing findings 1
through 3 closes the ways to LEARN a file id; it does not make the image
itself access-controlled. Deciding whether it should be is an architecture
question for the operator, not a patch.
