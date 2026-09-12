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


## Addendum — operator rulings 2026-09-12 (round 2 of the renders)

Canvas: https://claude.ai/code/artifact/d554c339-512f-4f01-9ead-06cafd69df90
(Chrome page). Four rulings, in the operator's words, then what they change:

1. "the chronicle in the top left … needs to be able to be changed by the
   owner as well" → the brand in the **sidebar header** (logo + name) is
   Owner-editable **in place**: pencil on hover opens the name as an input,
   Enter saves through `UpdateBranding` (40-char cap unchanged), Esc
   cancels; click the logo to change it. D13's in-place editing moves from
   the header to the sidebar header. The Appearance field stays as the
   fallback.
2. "Wouldn't it be better to have campaigns in the navbar, vs the header?"
   → **one way to switch, in the nav.** The switcher folds into the sidebar
   brand (menu: current ✓, others, All campaigns = the old list page, New).
   The header picker (`app.templ:1190-1217`) and the sidebar "Campaigns"
   link both retire. **D16's brand card in the header is superseded**; the
   header's left slot shows the current path instead (see direction C).
3. "Journal, NPCs, Armory, Characters, etc should be in an apps drawer,
   kinda like how the parent categories work" → **N1 (new slice): Apps
   drawer.** Zone 2 becomes Dashboard · Apps ▸ · Categories ▸ · My
   Characters. Apps opens the same slide-over the categories use (Back row,
   then the addon shortcuts in owner order). The sidebar editor edits drawer
   items inside the drawer (reorder, hide) over the same unified `items`
   model — no second model. This supersedes N0's "no structural redesign".
4. "the ability to just have a page customized for the header, like if they
   wanted an image" → **H3 becomes its own Customize → Header page**: brand
   (name, logo), background (solid / gradient / animated gradient / image
   still-or-animated ≤1.5 MB with scrim / sky-deferred), widget list, one
   staged save. Today's Top Bar Style + Topbar Content cards fold into it.

**Open (operator picks):** how the current page is shown — A rail-and-tint,
B sliding marker (200 ms, jumps under reduced motion), C echo in the header
(the freed left slot reads the path), or a mix. **Lead's recommendation:
A + C.** The row lights in the sidebar; the header slot reads the path,
because inside a drawer the sidebar cannot show the leaf page. B adds
motion with nowhere to glide once drawers exist. N0's hover-glow verify still
stands; the glow itself is no longer sacred if the pick replaces it.

5. (round 3) "Can the campaign selector actually be a slide out from the
   right side … a search bar and filters inside of it?" → **yes, ruled.**
   Clicking the brand opens the same right-edge card the permissions widget
   ships (420px, 280ms, backdrop): search box, role chips (All / Owner /
   Scribe / Player with counts), Active / Archived, current campaign first
   then by last played, New at the foot, Manage = the old list page. Search
   and filters render only past a handful of campaigns. This replaces the
   dropdown drawn in round 2. Reuse the card's shell; do not build a second
   slide-in mechanism.

**Where-am-I, round 3:** the operator leans C + A and asked for three more:
D traced ring (their idea: on click a 2px line runs the button's perimeter,
direction random, ~350ms, then settles into the lit row; reduced motion =
no trace), E the icon fills (ripple from the icon, icon stays solid), F
folder tab (the active row takes the page colour and joins it at the edge).
All drawn with C's header path alongside. **Still open.**
