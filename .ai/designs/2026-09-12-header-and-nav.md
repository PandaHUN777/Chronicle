# Header customization and nav polish — build plan

**Ruling:** the customizable header was designed in July, signed by the
operator, and never built. **Revive it; do not redesign it.** The nav's
structural work (one item model, one reorder mechanic — C-NAV-V3) shipped and
matches the code; it needs a polish slice, not a rethink.
**Executor model:** Sonnet (`go-dev`); design questions go back to the lead,
never guessed. **Owner-facing summary:** the top bar becomes yours — your
name or logo, editable in place; a row of small live widgets you pick; the
campaign switcher folded into the name. Two pieces (a moving sky background,
a mini calendar) wait for the calendar rebuild.

## Where the design lives (read in this order — it is the spec)

1. `Cordinator/plans/2026-06-03-topbar-customization-vision.md` — the ask.
2. `Cordinator/plans/2026-07-07-campaign-chrome-v2-design.md` — decisions
   D1–D19. **Binding.**
3. `Cordinator/mockups/campaign-chrome-v2.html` + `mockups/chrome-directions/
   e-final-mix.png` — the signed layout and the operator's visual pick.
4. `Cordinator/dispatches/chronicle/C-CHROME-{P1,P2,P3,BG}.md` — the four
   slices as written in July. **Stale anchors; re-baseline before use.**

## Step 0 — re-baseline (one agent, read-only, before any code)
For each of D1–D19 and each dispatch: does the file/line it names still
exist, and does the design still apply? Three drifts are already known:
- **D-"living sky" background mode:** the sky engine was deleted with the
  calendar (2026-08-21). **Defer to the skypane standalone project.** Nothing
  in H1–H3 may depend on it.
- **Compact-calendar and weather widgets:** the calendar plugin is domain-
  only until V5. **Their registry slots are declared; their bodies wait.**
- **The accent-trio rename (D14):** already shipped under different names as
  Site / Action / App accent (`C-ACCENT-SLOTS`, PR #541,
  `campaigns/branding.templ:557-634`). **Done; do not redo.**
Output: a table D1–D19 → keep / done / defer, with fresh `file:line`.

## Slices

### N0 — nav polish (independent of Chrome V2; can go first)
1. **Hover ghosting.** `C-NAV-HOVER-ANIM-FIX` was dispatched for "~6 boxes
   flash mid-sweep" on `sidebar-nav-glow` (`static/css/input.css:344-438`);
   no completion report exists and the CSS carries no fix comment. **Verify
   in headless Chromium first** (the sandbox has it); fix only if reproduced.
   The glow itself is operator-approved and must survive identically.
2. **Retired-tab deep link.** A dashboard card links to the retired Settings
   "Features" tab (blank page). Find it (`grep -rn "tab=features"`), point it
   at `/campaigns/:id/extensions`.
3. **The live inline-script bug.** `campaigns/branding.templ:675-710` (the
   Surface Accents card) carries an inline `<script>`; `boot.js:212` strips
   it on a boosted swap, so reaching Customize → Appearance by clicking the
   sidebar likely lands with dead handlers. Convert to inline IIFE `onclick`
   per STANDING_ORDERS §3 and add the file to the ratchet
   (`tools/check-page-scripts.sh`, `.ai/todo.md` item H).
4. **One save model.** That same card saves by direct PUT per click while
   every other Appearance control uses the staged draft/save bar
   (`branding.templ:365-373`, `appearance_editor.js`). Fold it into the
   staged flow. Two save models on one page is how settings get lost.
Mobile: **keep the off-canvas drawer.** No bottom bar, no icon rail — nothing
in the books asks for one and inventing one is the trap.

### H1 — the widget host *(P1, re-baselined)*
Go-registered widget-type registry; `TopBarSnapshot` with a data budget;
per-widget error isolation; portal popovers. Ship with the widgets that need
no calendar: **search, quick-note, custom text, links, era pill** (era is
timeline data — confirm at re-baseline). Calendar/weather slots declared,
empty, labelled honestly ("after the calendar rebuild"). Extend
`CampaignSettings` (`campaigns/model.go:540-560`) with `TopBarWidgets`; do
NOT add a parallel settings model — the nav-v3 retrospective names "two data
models" as the root of three bugs.

### H2 — brand in place, switcher merged *(P2)*
Brand name editable inline for Owner (today: a field on the Appearance tab);
the campaign switcher (`app.templ:1190-1217`) folds into the brand name
per the mockup. Keep `GetDisplayName` (`data.go`) as the single source.

### H3 — the editor *(P3)*
The customization UI for H1/H2 per `campaign-chrome-v2.html`. Staged save,
same bar as Appearance.

### H-BG — living sky *(deferred; skypane project)*

## Constraints every slice inherits
- `isolate` on `<header>` (`app.templ:1152`) is load-bearing for the
  background layer; never remove. Popovers portal to `<body>`.
- No `templ script`, no inline `<script>` bodies in anything reachable by
  a boosted sidebar link. Inline IIFE `onclick` only.
- `sidebar-nav-glow` unchanged in effect.
- `SidebarItem`/`SidebarConfig` extended, never paralleled.

## Acceptance
`make verify` + `make test-js`; screenshots at desktop and 390px against the
signed mockup (headless Chromium); the `reviewer` pass. H1–H3 additionally:
the re-baseline table is linked in the PR and every D-number it keeps is
cited at the code that implements it.
