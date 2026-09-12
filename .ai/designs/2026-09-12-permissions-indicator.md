# Permissions glance — build plan (ADR-057)

**Status:** ready for execution in slice order. **Render SIGNED by the operator
2026-09-12** — https://claude.ai/code/artifact/c84d0fb4-f4c5-4410-9657-ddd77bb20a20
— so every slice can start. Slices 1–2 never needed it.
**Executor model:** Sonnet (`go-dev`), with a `reviewer` pass before push.
**Owner-facing summary:** one icon beside the entity name tells the DM team
who can see it; hover shows the list; editing moves to edit mode; the odd icon
at the bottom goes away; Players see nothing.

## What exists (read these first)

- `internal/plugins/entities/visibility_badge.templ:10-32` — the header glance
  (closest to right; keep its seat).
- `internal/plugins/entities/visibility_glance.go:94-121` — the tooltip text
  builder incl. tag-widening. Reuse its logic in the popover.
- `internal/plugins/entities/entity_card.templ:78-100` — card badge.
- `internal/plugins/entities/show.templ:432-457` — `blockDetails`, the icon the
  operator means. **No role parameter.** Shown to Players.
- `internal/plugins/entities/show.templ:618-636` — `blockPermissions`; auto-
  appended by `service.go:2653-2699` `EnsurePermissionsBlockInDefaults` and
  `model.go:114-129,161-173` (`row-perm`).
- `static/js/widgets/permissions.js` — the editor. `:47-48` init state, `:170-174`
  `getMode()`, `:607-666` `load()`, `:234-264` trigger. Already theme-tokened.
- `internal/plugins/entities/form.templ:281-296` — edit-form mount (stays).
- `internal/plugins/campaigns/model.go:248-263` — `VisibilityRole()`.
- `internal/plugins/entities/service.go:2340-2375` — `CheckEntityAccess`.

## Census correction (2026-09-12, measured during build, Opus lead)

ADR-057 counted **four** implementations. A fresh grep for the teal shield
found **seven**, and the "shown to Players" defect is in **three** places,
not one. Slice 3 must replace all seven:

| # | Where | File | Role gate | States | Colour |
|---|---|---|---|---|---|
| 1 | Show header | `visibility_badge.templ:10-32` | `MemberRole >= RoleScribe` (caller, `show.templ:247`) | 3 + tag dot | `#0d9488` |
| 2 | List/grid card | `entity_card.templ:86-100` | `MemberRole >= RoleScribe` (caller, `:41`) | 3 | `#0d9488` |
| 3 | Details card | `show.templ:448-456` (`blockDetails`) | **NONE — Players see it** | 2 | `#0d9488` |
| 4 | Permissions row | `show.templ:627-636` (`blockPermissions`) | `MemberRole >= RoleOwner` | editor | tokens |
| 5 | Category table row | `category_dashboard.templ:382-393` | `MemberRole >= RoleScribe` | 2 | `#0d9488` + **amber** for private |
| 6 | Category tree node | `category_dashboard.templ:488-492` | **NONE — Players see it** | 2 | `#0d9488` |
| 7 | Child-entity list | `show.templ:719-723` | **NONE — Players see it** | 2 | `#0d9488` |

Three further notes for the executor:

- **Every gate is raw `MemberRole`, so no Co-DM sees any of them.** Slice 3's
  `VisibilityRole() >= RoleScribe` gate fixes that everywhere at once; that is
  the same defect slice 1 fixes on the access path, on the render path.
- **Rows 5-7 are two-state**: no globe for "everyone". Folding them into the
  one component gives them the third state. Check with the operator only if
  that changes a dense table's look; otherwise ship consistent.
- **Row 5 paints private amber**, a fourth colour vocabulary nobody else uses.
  It goes with the rest.
- **`#0d9488` also appears at `static/js/widgets/db_explorer.js:22`** as a
  calendar swatch colour, unrelated to visibility. The render-contract grep
  guard must be scoped to the entities templates, NOT a tree-wide ban, or it
  fails on an innocent line.

## The states, and what the glance shows

| Entity state | Glyph | Token | Popover text |
|---|---|---|---|
| `default`, `is_private=false` | `fa-globe` | `--color-fg-muted` | "Everyone in this campaign" (+ "and visitors" if the campaign is public) |
| `default`, `is_private=true` | `fa-lock` | `--color-fg-muted` | "DM team only — Owner, Scribes, co-DMs" |
| `custom` | `fa-shield-halved` | `--color-accent` | "Specific people:" then one line per grant — `Players` / `Scribes` (role tiers), each named member, each group — then the tag line from `visibility_glance.go` if any |

Glyph size and weight match the existing header badge. Popover: same surface
tokens as `permissions.js`'s card, portalled to `<body>` (the Chrome V2 design
already rules popovers portal; do not nest under the topbar's `isolate`).

## Slices

### Slice 1 — Co-DM can open what they are allowed to see *(no design; do first)*
- Every `CheckEntityAccess` caller passes `int(cc.VisibilityRole())`, not
  `int(cc.MemberRole)`: `handler.go:599,1714,1800,1866,2096,3171,3246,3486`,
  `syncapi/api_handler.go:374`.
- **Red first:** a member with `IsDmGranted=true`, `MemberRole=Player`, on a
  `dm_only` entity: `Show` returns 404 before, 200 after. Add the same for
  `BacklinksFragment` (list populated AND target reachable).
- Do not change `CheckEntityAccess` itself; the fix is at the callers, which
  is where the inconsistency is.

### Slice 2 — a Scribe is never shown a wrong glance *(no design; do first)*
- `permissions.js`: on a non-2xx `load()`, render nothing (or a muted "you
  can't change permissions here" if `editable` was requested), never the
  init defaults. `getMode()` must not be reachable with unloaded state.
- **Red first:** a JS test that stubs a 403 and asserts the trigger text is
  not "Everyone". `test/js/` has the harness pattern.

### Slice 3 — one component, four call sites *(after the render is signed)*
- New `visibilityGlance(entity, viewer)` templ component in the entities
  plugin; gate inside it on `viewer.VisibilityRole() >= RoleScribe`.
- Header: `visibility_badge.templ` becomes a thin call to it.
- Cards: `entity_card.templ:78-100` becomes a call to it.
- **Delete** the icon block in `blockDetails` (`show.templ:432-457`).
- **Delete** `blockPermissions` from the read page, the `"permissions"` block
  registration (`block_registry_core.go:140-145`), `EnsurePermissionsBlockInDefaults`
  and the `row-perm` entries. Existing stored layouts carrying `row-perm`
  must render without it — a reconciler-style tolerance, not a migration.
- **Delete** every `#0d9488`. Add a grep to the render-contract test so it
  cannot return.
- Tests: Player and anonymous viewers get no glance element at all (assert on
  the rendered HTML, not on a flag); Co-DM gets it; the three states render
  the three glyphs.

### Slice 4 — the hover key
- Popover component driven by the same data `visibility_glance.go` builds;
  extend it to enumerate custom grants (roles → labels, users → display
  names, groups → group names). Owner-only data is fine here: only the DM
  team ever renders it.
- Keyboard: focusable, `Escape` closes, `aria-describedby`.
- Inline IIFE handlers only — this renders inside HTMX-swapped fragments
  (STANDING_ORDERS §3; `boot.js:212` strips `<script>` on swap).

### Slice 5 — finish `public` as a grant subject *(later, own PR)*
- `entities/model.go:803-806` `ValidSubjectType` accepts `public`;
  `repository.go:2022-2033` `GetEffectivePermission` honours it;
  `permissions.js` offers "Everyone, including visitors" as a row, Owner-only,
  only when the campaign is public.
- Booked, not this pass: a seeded "Party" group.

## Acceptance (per slice, before push)
`make verify` + `make test-js`; the red-first evidence pasted in the PR body;
a `reviewer` pass. Slices 3–4 additionally: the render the operator signed is
linked, and the built page is screenshotted against it (headless Chromium is
in the sandbox).

## Do not touch
`internal/widgets/tags/` (tag grants are a separate mechanism the glance only
reports); the sessions plugin; anything in ADR-054's held list.
