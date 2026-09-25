# AI Documentation Index

Context for AI sessions working on Chronicle. Start at the root `CLAUDE.md`;
this page maps the rest.

**Open work is not in these files.** It lives in GitHub Issues (see "Where
things live" in `CLAUDE.md`). The files here describe how the system works now,
in the present tense. Edit one when the behavior it describes changes, and
never append dated entries or "recent work".

## How to use these files

1. **Every session:** start from the issue for your task. Issues labelled
   `needs-operator` are waiting on the human.
2. **When working on a plugin or widget:** read its `.ai.md` (index below).
3. **When coding:** read `conventions.md` for patterns with code examples.
4. **When making or questioning a design choice:** read `decisions.md`. Older
   cross-repo rulings live in the private Cordinator repo's `decisions/`, which
   code comments still cite; Cordinator is otherwise a frozen archive.

## Reference files

| File | What it covers |
|------|----------------|
| `architecture.md` | System design, the three-tier extension model, request flow, dependency graph |
| `conventions.md` | Code patterns with Go/Templ/SQL examples, CI guards, security rules, cross-plugin import discipline |
| `decisions.md` | Architecture Decision Records. Append-only; code cites ADR numbers, so never renumber |
| `tech-stack.md` | Technology versions, configs, and why each was chosen |
| `data-model.md` | Schema overview: every live table and what it holds. The migrations are the source of truth |
| `api-routes.md` | Where to find routes: `internal/wire/routes_snapshot.txt` lists every route (CI-guarded) and `docs/api/openapi.yaml` describes the sync API |
| `glossary.md` | TTRPG and Chronicle terminology |
| `troubleshooting.md` | Non-obvious problems and their fixes, including the test-environment ones |
| `plugin-development.md` | Building WASM extensions |
| `designs/` | Designs. `2026-09-12-build-order.md` and `2026-09-12-header-and-nav.md` are approved and unbuilt (#739). `2026-09-13-media-renovation.md` is the media plan, waiting on four decisions (#730, #733). |

`status.md` and `todo.md` are pointers to the issues now. Finished plans,
audits and old designs were deleted; git history keeps them.

## Per-plugin and per-widget docs

Every directory below has an `.ai.md` describing its purpose, files, routes,
business rules and footguns.

- **Plugins** (`internal/plugins/<name>/`): addons, admin, ai_workspace (and
  ai_workspace/aiexport), armory, audit, auth, backup, bestiary, campaigns,
  designlab, entities, foundry_vtt, maps, media, npcs, packages, restore,
  sessions, settings, smtp, syncapi, timeline, widgetbindings. The calendar
  plugin has none: it holds only a domain layer and migrations until V5 (#741).
- **Widgets** (`internal/widgets/<name>/`): attributes, editor, entity_notes,
  mentions, notes, posts, relations, tags, title.
- **Infrastructure:** `internal/database/`, `internal/extensions/`,
  `internal/systems/` (game systems are external packages installed through
  Admin → Packages, so there is one systems-infrastructure doc, not one per
  system), `internal/websocket/`.
- **Front-end scripts** (`static/js/`): `boot`, `sidebar_tag_filter`,
  `sidebar_tree`, and under `widgets/`: `dynamic_surface`, `entity_tooltip`,
  `image_upload`, `layout_editor`, `template_editor`, `timeline_viz`.
- **Examples:** `extensions/example-wasm-go/`, `extensions/example-wasm-rust/`.

## Templates

- `templates/module-ai.md.tmpl`: start a new plugin or widget `.ai.md` from this.
- `templates/decision-record.md.tmpl`: the ADR format.

## Other documentation

- `docs/`: operator-facing docs (deployment and upgrades, the restore drill,
  admin diagnostics, the OpenAPI spec, package authoring).
- `tools/`: the CI guard scripts.
- Root `README.md`: the project overview for humans.
