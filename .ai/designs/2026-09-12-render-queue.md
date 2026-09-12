# Render queue — what the operator has signed and what they asked to see next

Renders are usage-intensive (operator, 2026-09-12). One canvas with several
boards, not one canvas per idea. Static mockups unless a clickable flow is asked
for. Match Chronicle's tokens exactly — the values are below so nobody re-reads
the stylesheet.

## Signed

**Permissions glance — SIGNED 2026-09-12** ("That looks good").
Canvas: https://claude.ai/code/artifact/c84d0fb4-f4c5-4410-9657-ddd77bb20a20
Boards: DM-team view with hover key open · the three states (+ dark strip) ·
what a Player sees (no icon). Working files were in this session's scratchpad
(`glance/*.dc.html`, `canvas.json`) — ephemeral; re-extract from the artifact
if they are gone. **Unblocks ADR-057 slices 3–4.** Slices 1–2 never needed it.

**Editors and chrome — ROUND 2 drawn 2026-09-12, awaiting picks.**
Round 1 "looked good" with four rulings and two open questions (below);
round 2 is on the same canvas. **Open picks:** (1) permissions editor —
Option A (in the edit page, restyled to the form's density) or Option B
(the widget's existing slide-in card opened from the glance icon) —
**PICKED B, 2026-09-12; ADR-057 amended.** (2) how the current page is
shown — A rail-and-tint, B sliding marker, C echo in the header, or a mix
— **recommended A + C**; operator leans C + A and asked for three more,
drawn round 3: D traced ring (theirs), E icon fills, F folder tab. Still
open. Also drawn round 3: the campaign switcher as a right-edge panel with
search and filters (ruled, see the header-and-nav addendum item 5).
Canvas: https://claude.ai/code/artifact/d554c339-512f-4f01-9ead-06cafd69df90
Two pages. *Permissions:* Owner editing (three modes, role / member / group
rows, owner row, read-only tag line) · Scribe (mode badge only; honest blank
on a failed load). *Chrome:* sidebar as built beside its edit mode (drag,
eye, pen/trash on custom items; the inline edit card is the one polish
proposed over three `prompt()` boxes) · top-left brand as built, in place per
D13/D16 with the switcher folded in, and the Appearance field it is edited on
today · top bar as built (links, then a quote on the gradient style) and
Chrome V2 re-baselined (brand card left, centre open, readout rail right;
sky deferred, calendar/weather slots declared and empty) · editing the top
bar: today's two Appearance cards, then the V2 Top Bar tab (five background
modes, live preview, ordered widget rows with settings and visibility, six
slots, one staged save). Working files: this session's scratchpad
`chrome-editors/` (`gen.mjs` + `perm.mjs` + `chrome.mjs` emit the six
`.dc.html`; `canvas.json`) — ephemeral; re-extract from the artifact if gone.
Decision-free on purpose: nothing new was ruled, every board draws a plan
that already exists (ADR-057, header-and-nav N0/H1–H3, Chrome V2 D13–D19).

## Requested next (operator, 2026-09-12, in their words) — drawn above

> "renders of what it looks like for someone to be able to change those
> settings, and another render for the Nav Bar, the top-left icon, and the top
> UI bar and how each could be edited"

Four boards, likely one canvas, two pages (Permissions / Chrome):

1. **The permissions editor** — edit mode, the inline widget
   (`static/js/widgets/permissions.js`, mount `entities/form.templ:281-296`).
   Owner editing: the three modes, adding a role tier / member / group grant,
   the tag-widening line shown read-only. Scribe: per ADR-057 slice 2, nothing
   is shown on a failed load — never a wrong default. Draw the Owner board and
   a small Scribe board.
2. **The sidebar (nav bar)** and its editor — `app.templ:131-687` as built;
   the pencil editor (`static/js/sidebar_editor.js`, role >= 3) that reorders,
   relabels, re-icons and adds custom links over the unified `items` model.
   Preserve `sidebar-nav-glow` (`static/css/input.css:344-438`) exactly. No
   structural redesign — plan: `.ai/designs/2026-09-12-header-and-nav.md`
   section N0. Mobile stays the off-canvas drawer.
3. **The top-left icon** — the brand logo/name in the sidebar header
   (`app.templ:158-209`, `GetBrandLogo`/`GetBrandName`) and in the topbar
   (`GetDisplayName`, `app.templ:1196,1299`). How it is edited: today a field
   on the Appearance tab (`branding.templ`); Chrome V2 rules brand-in-place
   editing and the campaign switcher folding into the name (H2 in the plan).
4. **The top UI bar** — `app.templ:1151-1481` as built (left name/switcher,
   centre links-or-quote, right search/notes/dark/view-as-player/bell/user)
   and how it is edited: the Appearance tab's Top Bar Style (solid/gradient/
   image) and Topbar Content cards (`branding.templ:713-788`), then the
   Chrome V2 widget host (H1/H3). **Follow the signed July mockup** —
   `Cordinator/mockups/campaign-chrome-v2.html` + `mockups/chrome-directions/
   e-final-mix.png` — re-baselined per the plan's Step 0, never reinvented.

## Tokens (lifted from `static/css/input.css` on 2026-09-12)

Font: Inter, system-ui, -apple-system, sans-serif.
Light — bg-primary #f9fafb · bg-secondary #ffffff · bg-tertiary #f3f4f6 ·
border #e5e7eb · border-light #f3f4f6 · text-primary #111827 · text-body
#374151 · text-secondary #6b7280 · text-muted #9ca3af · text-faint #d1d5db.
Dark — bg-primary #111827 · bg-secondary #1f2937 · bg-tertiary #374151 ·
border #374151 · text-primary #f9fafb · text-body #d1d5db · text-secondary
#9ca3af · text-muted #6b7280 · text-faint #4b5563.
Accent (both modes) #6366f1 · hover #4f46e5 · light #a5b4fc. Tag-widened dot
amber #fbbf24. Elevation resting `0 1px 2px 0 rgb(0 0 0 / 0.05)`; hover
`0 6px 16px -4px rgb(0 0 0 / 0.12)`. Radius 8px on cards, pill 9999px.
Entity title 30px/36px/700; star 18px; glance glyph 11px, ml 6px, gap 8px.

## How the last one was made (so the next takes fewer tokens)

`/design` skill → lift tokens (above, done) → one `.dc.html` per board +
`canvas.json` in a scratch dir → seed with the skill's helper → `--check` →
publish with `contract: "0.1.31"`, `capabilities: {self: {}, downloads: {}}`.
Icons are inline stroke SVG on a 24 grid, never emoji or font glyphs.
