# Session handoff — branch `claude/determined-davinci-5ut5f5`

<!-- ====================================================================== -->
<!-- Category: SNAPSHOT — written 2026-09-13 at HEAD bfed63e9                -->
<!-- Purpose: everything a COLD session needs to resume without the          -->
<!--          transcript. Read this, then .ai/designs/2026-09-12-build-      -->
<!--          order.md, then start.                                          -->
<!-- ====================================================================== -->

## 0. Read this first

Branch: **`claude/determined-davinci-5ut5f5`**, 48 commits ahead of
`origin/main`, pushed, working tree clean.

**PR: [#607](https://github.com/keyxmakerx/Chronicle/pull/607)** — "Permissions,
media and the audit backlog". Opened 2026-09-13, one PR for the whole branch
with the work grouped by arc in the body.

It should have been several. `.ai/designs/2026-09-12-build-order.md` set the
rule "one PR per slice, nothing merges until the demolition deploy is
confirmed"; the second half was honoured and the first half was silently
dropped, so 49 commits accumulated with no PR at all — and `.ai/status.md`
twice said "see the branch's own PR for review status", pointing at nothing.
The operator caught it; no guard and no review did. The operator then chose one
PR now over a retro-split, so that the security work gets CI and review
coverage immediately rather than after rework.

The merge hold below is a **separate thing** from the PR question: a PR can
exist, be reviewed, and sit unmerged. The hold was never a reason not to open
one, and treating them as one rule is exactly how the omission hid.

**STANDING HOLD — nothing on this branch merges to `main` yet.** The operator
must first deploy the calendar demolition and walk its verification steps.
Until they say so, this branch accumulates and does not merge. Do not open a
merge, do not ask again each session — it is their call and they know it is
outstanding.

The calendar demolition is
**[PR #595](https://github.com/keyxmakerx/Chronicle/pull/595)** — "Calendar V5
clean slate", merged to `main` by the operator on **2026-09-11**, +1,816
−180,619 across 461 files. It carries **three migrations that permanently
destroy production data** (`calendar/019_calv5_clean_slate`,
`timeline/002_calv5_clear_calendar_links`,
`sessions/006_calv5_clear_availability`); all three `.down.sql` files are
deliberately empty, so **there is no rollback**. CI ran them once against an
empty MariaDB; they have never run against four months of real rows. That is
the entire reason for the hold.

Its operator gate, in order, copied here so nobody has to re-open the PR:

1. Take and verify a database backup.
2. Deploy and watch the boot log for `019`, `002` and `006` applying.
3. Sidebar → Calendar shows "being rebuilt", not a 404.
4. A campaign dashboard and an entity page show the rebuild notice, not a
   stuck "Loading…".
5. In Foundry, the sync dashboard's Calendar tab says rebuilding — **not** "No
   calendar configured" — while maps, actors, items and notes keep syncing.

The operator is **not a programmer** (networking background, GitHub
`keyxmakerx`). Explain in plain terms, never assume they will read Go to work
out what you meant. They are **usage-conscious**: renders and large agent
fan-outs cost real budget, and they have said so more than once. Prefer one
careful pass over three speculative ones.

---

## 1. What shipped on this branch, in order

Grouped by arc rather than strictly chronologically; the commit order is the
`git log --oneline origin/main..HEAD --reverse` order and is preserved within
each group.

### 1a. Toolchain, agents, and pre-existing sweeps (start of branch)

| Commit | What |
|---|---|
| `5584198b` | Four committed agent roles for Chronicle work (`.claude/agents/`) |
| `3c2073e0` | Go toolchain to 1.27.1; unmuted `x/crypto`, `x/net`, `echo/v4` |
| `8f97d21a` | Kept the VCS-stamping comment true after the toolchain move |
| `3349269e` | Campaign default visibility honoured on **every** entity-creation path (it reached only 1 of 5) |
| `62c88164` | The campaign "Sync API" toggle actually refuses Bearer keys now |
| `407c5fa6` | Machine-readable error **type** carried to API clients |
| `d61eaf18` | ADR-054, ADR-055, ADR-056 recorded |
| `56ed8d91` | Session-entity privacy leak + ungated Player Notes routes; real addon descriptions through Settings/Extensions |
| `6a2b62bf` | Six partial-update structs fixed; `Update*Request` scanner widened (ADR-054 #6) |
| `476b75f6` | ADR-057 + four execution plans from the 2026-09-12 design pass |
| `93de5c6a`, `9a243138`, `27fbc9e2`, `65cbcf7a`, `b1b59229` | Worktree merge, isolation-guard revert, operator answers booked, two review blockers fixed, and the doc corrections `65cbcf7a` claimed but didn't carry |

### 1b. The render rounds (design only, no product code)

`4597e8c6`, `072e05ec`, `fddaf32c`, `aefd495c`, `3cfd6938`, `42afc9f3`,
`2da04816`, `1cd3d415`, `61e81ff8`, `ecf25c49`.

Six rounds of chrome/permissions renders driven by operator feedback, then
approval and the build order. **Every ruling is written down** — see §3. The
last of these (`ecf25c49`) is the census correction: the visibility icon had
**seven** copies, not four, and **three had no role gate at all**.

### 1c. ADR-057 — one visibility glance, and the co-DM promotion

| Commit | What |
|---|---|
| `c59cf778` | **Slice 2** — a failed permissions load never claims "Everyone" |
| `e5b9155b` | **Slice 1** — a co-DM can open what they are already shown |
| `d9095c7f` | Booked the cross-plugin co-DM question the P-1 review surfaced |
| `b5993038` | The permissions widget stops claiming a mode it does not know yet |
| `e7003a2a` | Finished the co-DM promotion on the paths the page actually uses |
| `8c581076` | P-1 recorded as shipped and reviewed, with the lesson slice 3 inherits |
| `4d852a85` | **Slice 3** — one `visibilityGlance` component replaces seven copies |
| `4b41ef1d` | The operator's ruling recorded: a co-DM sees what the DM sees |
| `4df13033` | Co-DM promotion carried into armory, characters and timeline |

Mechanism: `campaigns.CampaignContext.VisibilityRole()` promotes a DM-granted
member (`IsDmGranted`) to Owner **for visibility only** — never for editing,
never for ownership. `internal/plugins/entities/visibility_glance.templ` is now
the single component; its role gate `visibilityGlanceVisible(cc)` lives
**inside** the component so a new call site cannot forget it.
`visibility_glance_guard_test.go` bans the hand-rolled hex, the icon class
outside the component, and the badge data attribute.

### 1d. Security S1 — the WebSocket per-user leak

| Commit | What |
|---|---|
| `d7240126` | Real-time map updates stop leaking what the page hides |
| `e70e7437` | **Pinned the map audience predicate across its two Go copies** |

`internal/websocket/hub.go` now applies `msg.AudienceAllows(client.UserID)` to
every non-DM-equivalent client. The predicate is **deliberately duplicated**
(the transport package must not import a plugin) and the duplication is pinned
by `internal/app/map_audience_parity_test.go`: one 9-case table run through
**both** `maps.VisibilityRules.Allows` and `websocket.Message.AudienceAllows`,
asserting they agree *and* asserting the documented answer. That test was
mutation-verified — inverting the socket copy makes it fail.

### 1e. The security audit and its first fixes

| Commit | What |
|---|---|
| `d29bf3e6` | All 17 findings from the S2/S3/S4 audit recorded |
| `61af48c5` | Media uploads require membership; a Player no longer lists every file |
| `901df82b` | Armory and NPC galleries honour custom entity visibility (finding 2) |
| `7c103a09` | A dm_only timeline is no longer readable by anyone with the address |

`.ai/designs/2026-09-12-security-audit-findings.md` is the record. **Nine of
the 17 were reachability-tested — all nine were reachable and none was blocked
by an existing control.** Eight are explicitly marked UNTESTED and are still
open (§4).

### 1f. CI was silently dead for a day

| Commit | What |
|---|---|
| `03ca8ead` | **Repaired the workflow: the lint step had lost its `uses:` and `with:` lines** |
| `4c9c4d5f` | Booked a workflow-shape guard |

During the Go 1.27 pin bump the `golangci-lint` step was reduced to a single
`version:` key. GitHub could not compile the workflow, so for ~19 hours and ~30
commits **every push produced a zero-job run**. Nothing was red; CI was simply
absent. The repaired step is:

```yaml
- name: golangci-lint
  uses: golangci/golangci-lint-action@v7
  with:
    version: v2.13.2
```

**The lesson, which is the reason this has its own section:** a green checks
list is not proof CI ran. A run whose *name is the file path* rather than the
workflow's `name:` is the tell that it never compiled. The guard to write is
in `.ai/todo.md` — ten lines of Python over `.github/workflows/*.yml`, failing
any step with neither `uses:` nor `run:`.

### 1g. ADR-058 — a picture inherits the permissions of the pages that use it

| Commit | What |
|---|---|
| `9599907b` | **ADR-058 recorded** (7 decisions + Consequences + Rejected) |
| `48acf5f1` | Decisions 1-3: a picture inherits page permissions |
| `922fd3b1` | Decisions 6-7: picture links belong to a person; a public campaign stops serving everything |
| `5eedb80c` | The link-binding slice recorded; remaining backlog narrowed |
| `bfed63e9` | Decisions 4-5: a silent merge can no longer publish a hidden page's artwork |

This arc is complete. Its pieces:

- `media.FindReferences` gained a **`cover_image_path` branch**. Without it a
  cover-only image looked *unreferenced* and would have fallen through to the
  plain-membership path — the trap that made the whole rule unsafe.
- `checkEntityScopedAccess` resolves a file's referencing pages and asks the
  canonical entity filter whether the viewer may see any of them. Cached in
  Redis at `media:access:<fileID>:<userID>`, 60s TTL.
- **Signed URLs are bound to the viewer.** `computeSignature(fileID, viewer,
  expires)`; viewers are `ViewerSession` / `ViewerAnonymous` / `ViewerAPIKey`;
  `SignedURLTTL = 15 * time.Minute`. `Verify` accepts a `ViewerAPIKey` digest
  when the presented viewer is `ViewerAnonymous` — that is the **narrow Foundry
  carve-out** and it is deliberate.
- `sig` and `expires` were added to `sensitiveParams` in
  `internal/middleware/logging.go` so a signature never lands in a log line.
- Upload is gated at **Scribe**: a `campaign_id` form value requires
  `MemberRole(campaignID, userID) >= int(campaigns.RoleScribe)`.
- **A nil member checker fails closed**, not open:
  `return false, fmt.Errorf("media: member checker not configured")`.
- Dedup: `canMergeWithExisting` requires the uploader to see **ALL** pages
  using the existing file, not any. Both mitigations the operator approved
  ("Yes please to both") are in: show where a picture is used, and refuse the
  merge rather than silently reusing a hidden page's file.

---

## 2. Where things stand, by area

| Area | State |
|---|---|
| ADR-057 permissions (P-1 slices 1-3) | **Done**, reviewed, mutation-verified |
| Co-DM visibility promotion | **Done** on entities, armory, NPCs, timeline — per the operator's ruling |
| WebSocket S1 leak | **Done**, parity-pinned |
| ADR-058 media | **Done** (all 7 decisions), 2 residuals booked |
| CI | **Repaired and running** |
| Security audit findings | 5 of 17 fixed; 8 never tested; the rest open |
| Chrome build (N1/N2/T-A/T-B/H1-H3/T-C) | **Designed and approved; not started** |
| Map images under ADR-058 | **Gap — not covered** |

---

## 3. Operator rulings (do not re-litigate these)

1. **Permissions editing opens from the glance icon** — Option B, a slide-out
   from the icon rather than an inline block in the entity editor. ADR-057 was
   amended to say so (`aefd495c`).
2. **The campaign switcher is a nav flyout**, a right-edge panel with search
   and filters — *not* a header dropdown, and there are not two switchers.
3. **Journal / NPCs / Armory / Characters etc. go into an Apps drawer**,
   working like the parent categories do.
4. **The "Chronicle" brand in the top-left is Owner-editable**, including the
   ability to give the header its own customized page (an image, say).
5. **Nav highlight = the living ring.** The full glow border, three states
   (at-rest slow traces; hover = the two traces "pulling away", little
   movement; active = D but at a middling speed, sizes and directions varying,
   sometimes fully encircling, sometimes not). **Never fast** — the operator
   explicitly asked not to freak out ADHD users. Default recorded: **D living
   ring, Calm preset**, with **J** as the still fallback. **This must be a
   setting**, and the Owner's customization page must expose the options.
6. **A co-DM has essentially the same access as the DM/owner** — "keep in mind
   that title is determined by the system". The flag is system-assigned, so
   Chronicle must honour it everywhere. **"Essentially" is doing real work:
   this is SEEING, not OWNING.** Armory's `Purchase` and `CanUserActAsBuyer`
   stay on the raw role — they read `CanEdit` and are economic actions. If the
   operator wants those promoted too, that is a **second ruling** nobody has
   asked for.
   Precedent for the same line: the Foundry key — "the owner is always the one
   that would/could create that. I could see the Co-DM being able to refresh
   it as a troubleshooting step."
7. **Both media-dedup mitigations, approved** — show where a picture is used,
   and warn/refuse on merge. Both shipped in `bfed63e9`.
8. **Renders are expensive.** Do not produce a render round unasked.

### Still outstanding from the operator

- The nav-highlight style pick is **defaulted, not confirmed** (D/Calm, J
  fallback). Building against the default is fine; say that you did.
- Whether armory's `Purchase` / `CanUserActAsBuyer` should also promote.
- Whether media access should be re-architected further than ADR-058 goes.
- **RULED 2026-09-13: one PR for the whole branch, arcs in the body** — now
  [#607](https://github.com/keyxmakerx/Chronicle/pull/607). Splitting an arc
  back out later is still on the table if review finds one thread too coarse;
  the PR body says so explicitly.

---

## 4. What is open, in the order I would do it

### 4.1 Security audit residue (highest value, smallest pieces)

- **Finding 5 — cross-campaign image signing oracle.** An entity's image field
  accepts *any* media UUID, so an attacker can get a file from another campaign
  signed for themselves. This is the one I would do first: it partially defeats
  ADR-058, which is otherwise complete.
- **Findings 6-8 — the AI workspace committer.** It republishes hidden entities
  from front matter; it wipes `fields_data` and clears `type_label`; and one of
  its doc comments describes behaviour the code does not have.
- **The eight UNTESTED findings** in
  `.ai/designs/2026-09-12-security-audit-findings.md`: breadcrumb ancestors,
  timeline search (partly fixed by `7c103a09`), marker popup entity name,
  unsigned public-campaign media, the 403/404 existence oracle, and signatures
  in logs (fixed by `922fd3b1`). Each needs a reachability test before anyone
  writes a fix — the nine that *were* tested were all reachable, so do not
  assume these are theoretical.

### 4.2 ADR-058 residuals

- **Map images are not covered.** `maps.image_id` (FK to `media_files`) and
  `map_tokens.image_path` are reference columns `FindReferences` does not
  union. A map-only image therefore still takes the plain-membership path.
- **The timing side-channel on a refused merge.** The refused response is
  byte-identical to an ordinary upload's (pinned by a test comparing status and
  JSON key sets), but a refusal makes two extra DB round trips first. Closing
  it means a dummy visibility check on every true no-match. Judged not worth
  the complexity and **flagged rather than claimed closed** — that honesty is
  the point; do not let a later commit message claim otherwise.
- **Decision 4 has no Scribe-reachable UI door.** The "where is this used"
  endpoint is now reachable at promoted role >= Scribe and filters correctly,
  but the media browser page and its delete action are still Owner-only route
  gates, so only an Owner sees the button. The media picker is the obvious
  place for the new entry point; nobody has designed it.

### 4.3 The workflow-shape guard

`tools/check-workflow-steps.sh`. See §1f. Small, and it closes a class of
failure that is invisible by construction.

### 4.4 The chrome build

`.ai/designs/2026-09-12-build-order.md` has the seven ordered slices with
executor and model per slice. Summary of what is left:

- **N1** — Apps drawer + switcher flyout + brand in place.
- **N2** — nav highlight styles (build D/Calm, keep J as the still fallback,
  expose the setting).
- **T-A / T-B** — Customize page cleanup.
- **H1-H3** — the header.
- **T-C** — preview rebuild.

---

## 5. Environment gotchas (these cost real time last session)

- **`templ` is at `/root/go/bin/templ`**, not on `PATH` by default. So are
  `golangci-lint` and `govulncheck`.
- **Generated `*_templ.go` files are gitignored** (`.gitignore:30`). Editing a
  `.templ` without regenerating means your local build is stale and CI's is
  fine — or the reverse. Run `make templ`.
- **`tools/test-restore-drill.sh` needs Docker and fails in this sandbox.** It
  is not your change that broke it.
- **`tools/check-page-scripts.sh` passes now.** It was failing because of a
  stray 21 MB `.claude/worktrees/agent-ace3f015ffbdaa6ea` copy left behind by
  an agent, which the guard walked. I initially told the operator this was
  "pre-existing and fails on main too" — true, but the *cause* was the stray
  worktree, and I corrected that to them. **Delete agent worktrees when you are
  done with them.**
- **Commits must be identity-verified** or a stop hook rejects them:
  `git config user.email noreply@anthropic.com`, and
  `git commit --amend --reset-author` if you have already committed.
- **Playwright, if you need it for renders:** import the absolute path
  `/opt/node22/lib/node_modules/playwright/index.mjs`; the bare specifier does
  not resolve.
- **Two agents must never edit the same file concurrently.** The build broke
  mid-flight when `EnsurePermissionsBlockInDefaults` was deleted while
  `routes.go` still called it. The fix that worked: send the exact replacement
  text to the agent that *owns* the file rather than editing it yourself.

---

## 6. Lessons this branch paid for

These are not platitudes; each one is the cause of a real defect on this
branch.

1. **A claim measured against another repo's source is true only on the day it
   was measured.** (Inherited doctrine from the Foundry module's CLAUDE.md, and
   it bit again here.)
2. **An enumeration written from a grep of one function name is not a census.**
   ADR-057 said "every `CheckEntityAccess` caller" and listed nine. There were
   twelve, plus four more paths reaching the same rule through `GetChildren`,
   `filterByTargetVisibility`, `GetFilteredGraphData` and
   `ListRecentForDashboard`. Same error class as the seven-vs-four icon census.
3. **A test that installs its own fake and asserts the fake proves nothing.**
   (`internal/app/error_handler_api_type_test.go` is the canonical example in
   this tree.) **Mutation-verify**: revert the fix, re-run, confirm the test
   fails, restore byte-identical.
4. **Green CI is not evidence CI ran.** §1f.
5. **Agents confidently report things that are false.** Four caught on this
   branch: a claim that `show.templ:632` hard-codes `data-editable` for every
   viewer (the block is owner-gated); a tooltip unification onto wording that is
   false on private campaigns; a gallery bypassing at Scribe when the canonical
   filter bypasses only at Owner; and a media fix that preserved a nil
   fail-open. **Read the diff, do not read the summary.**
6. **A bound nobody is told about is the defect, not the bound.** (From the
   Foundry module's page-walk fix; the same shape recurs here.)
7. **Half a rule is not the rule.** The build order said "one PR per slice,
   nothing merges until the demolition deploy is confirmed." I honoured the
   restrictive half and dropped the other, then wrote "see the branch's own PR
   for review status" into `.ai/status.md` **twice** — pointing at a PR that
   never existed. Neither a guard nor a review caught it across 48 commits;
   the operator did, by reading the handoff. When a plan has two clauses,
   check both, and never write a cross-reference without confirming its target
   exists.

---

## 7. Where the rest of the record lives

| File | What |
|---|---|
| `.ai/decisions.md` | ADR-057 (amended) and **ADR-058** |
| `.ai/designs/2026-09-12-build-order.md` | **Start here to build** — 7 slices, executor/model, standing rules |
| `.ai/designs/2026-09-12-security-audit-findings.md` | All 17 findings, which were tested |
| `.ai/designs/2026-09-12-header-and-nav.md` | Chrome plan + the round 2-6 ruling addenda |
| `.ai/designs/2026-09-12-permissions-indicator.md` | The corrected seven-copy census |
| `.ai/designs/2026-09-12-render-queue.md` | Signed renders and the operator's render requests |
| `.ai/todo.md` | The live backlog, including everything in §4 |
