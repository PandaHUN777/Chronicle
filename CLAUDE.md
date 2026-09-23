# Chronicle

Chronicle is a self-hosted TTRPG worldbuilding platform. Go backend with Echo v4
framework, Templ templates, HTMX for interactivity, MariaDB for persistence, Redis
for caching/sessions. Frontend uses a **three-tier extension architecture**:
Plugins (feature apps), Systems (game system content), and Widgets (reusable UI blocks).

## Quick Commands

```bash
make dev            # Start dev server with hot reload (air)
make build          # Production binary build
make templ          # Regenerate Templ .go files from .templ sources
make tailwind       # Regenerate Tailwind CSS
make tailwind-watch # Watch mode for Tailwind CSS
make test           # Run all tests
make test-unit      # Unit tests only
make test-int       # Integration tests (requires running DB)
make test-db-up     # Start a local MariaDB for tests WITHOUT Docker (port 13306)
make test-db-down   # Stop that local test MariaDB
make test-int-local # Integration tests against it (starts it if needed)
make lint           # Run golangci-lint
make migrate-up     # Apply all pending migrations
make migrate-down   # Rollback last migration
make migrate-create # Create new migration (NAME=description)
make seed           # Seed dev database with sample data
make docker-up      # Start MariaDB + Redis containers (needs a Docker daemon)
make docker-down    # Stop containers
make clean          # Remove built artifacts
```

## Architecture at a Glance

**Three-tier extension architecture.** Everything beyond core infrastructure is
a Plugin, System, or Widget:

| Tier | Location | What It Is | Examples |
|------|----------|-----------|---------|
| **Plugin** | `internal/plugins/<name>/` | Feature app with handler/service/repo/templates | auth, campaigns, entities, maps, sessions (calendar is mid-rebuild: domain layer + migrations only until V5; requirements in #741, re-wiring points tagged `CALV5-PLACEHOLDER:`) |
| **System** | External repos via package manager | Game system content pack (reference data, tooltips) | Installed via Admin > Packages |
| **Widget** | `internal/widgets/<name>/` | Reusable UI building block (mounts to DOM) | editor, title, tags, attributes, mentions |

**Request flow:**
Router -> Middleware -> Handler -> Service -> Repository -> MariaDB

**Templates:**
Handler calls Templ component -> returns full page OR HTMX fragment (via
`middleware.IsHTMX(c)`).

**Widgets:**
Self-contained JS modules that mount to a DOM element via `data-widget` attributes,
fetch their own data from the API, and render themselves. Auto-mounted by `boot.js`.

See `.ai/architecture.md` for the full architecture document.

## Code Conventions (Critical -- Read These)

- **Handlers are thin:** bind request, call service, render response. NO business logic.
- **Services own business logic.** Services NEVER import Echo types.
- **Repositories own SQL.** One repository per aggregate root. Hand-written SQL.
- **Templ components:** one file per visual component. Layouts in `internal/templates/layouts/`.
- **HTMX detection:** use `middleware.IsHTMX(c)` to return fragment vs full page (also checks `HX-Boosted`).
- **Errors:** use domain error types from `internal/apperror/`. Never return raw DB errors.
- **Tests:** table-driven tests. Interfaces for all service/repo boundaries.
- **Naming:** `snake_case.go` for files, `PascalCase` for exported Go types, `camelCase` for JSON.
- **Migrations are APPEND-ONLY and SCHEMA-ONLY:** sequential numbered SQL files in `db/migrations/`. **Never edit, delete, or renumber a migration any live DB may have applied** — a CI guard (`tools/check-migration-immutability.sh`) enforces it; deleting an applied migration crash-loops boot (the 000030 incident, ADR-044/045). New DDL must be idempotent (`ADD COLUMN IF NOT EXISTS` / `CREATE TABLE IF NOT EXISTS` / `DROP ... IF EXISTS`). One-time **data** fixes do NOT go in migrations — use an idempotent reconciler (an `EnsureX`/`MergeX` service method or a `SetupProvider`). Core migrations may only reference core tables — plugin tables (e.g. `api_keys`, `maps`, `calendars`) live in `internal/plugins/<slug>/migrations/` and run *after* core, so a core migration referencing them crashes on a fresh DB. Span-the-layers data fixes must be split (core part in core, plugin part in the plugin). See `.ai/conventions.md` §"Migration Safety Rules".
- **Comments:** every package, every exported type, every non-obvious block. WHY not WHAT.
- **Database:** MariaDB. Use `database/sql` + `go-sql-driver/mysql`. No ORM.

## Where things live

Work is tracked in **GitHub Issues**, not in markdown files. The files in this
repo describe how the system works *now*; git and pull requests remember what
happened. Before you write anything down, pick its home:

| What you have | Where it goes | Never |
|---|---|---|
| Something to do: a bug, feature, follow-up or tech debt | An issue in the repo whose code must change | a backlog file, a new `.md`, a TODO comment |
| Something only the operator can do or decide | An issue labelled `needs-operator`, titled `Decide: …`, `Check: …` or `Do: …`, with your recommendation in plain language | chat only, a handoff doc |
| A multi-step effort | A parent issue with sub-issues | a "master plan" doc |
| How the system works | `.ai/architecture.md`, `conventions.md`, `data-model.md`, each plugin's `.ai.md`, `docs/`, all in the present tense | dates, "recent work" sections |
| Why it is built this way | An ADR in `.ai/decisions.md` (append-only; code cites `ADR-NNN`) | an essay in a code comment |
| What happened | The commit message and the pull request description, with `Fixes #N` | status logs, dated docs, reports |
| Unfinished work at the end of a session | A comment on the issue or PR: what's done, what's next, gotchas | a handoff file |

Labels: `needs-operator` (waiting on the human), `security`, `priority: high`,
`blocked`, `calendar-v5`, `documentation`, `good first issue`. Issue *types*
(Bug / Feature / Task) say what kind of thing it is. An old tracking ID from
before September 2026 (`C-…`, `FM-…`) is listed in the issue that replaced it,
so searching the issues for the ID finds it.

**Unfixed security weaknesses never go in a public issue.** They are tracked
in the private Cordinator repo until fixed; the fixing PR can then say
"security fix". Cordinator is otherwise a frozen archive of the old planning
system, and many code comments still cite its paths.

## AI Documentation System

All AI context files live in `.ai/` at the project root. Read `.ai/README.md` for
the full index. Key files:

| File | When to Read |
|------|-------------|
| The open issues for your task | **Every session start** -- what to do, and what is waiting on the operator |
| `.ai/architecture.md` | When designing new features or systems |
| `.ai/conventions.md` | When writing any code -- patterns with examples |
| `.ai/decisions.md` | When questioning a design choice -- ADRs with rationale |
| `.ai/data-model.md` | When writing queries or migrations (incomplete, see #742: the migrations are the truth) |

Each plugin and widget has its own `.ai.md` in its directory (systems are external packages — see `.ai/README.md`).
`.ai/status.md` and `.ai/todo.md` are pointers to the issues now; do not add to them.

## IMPORTANT RULES

1. **ALWAYS** start from the issue for your task. Open an issue for anything you find and don't fix.
2. **NEVER** append dated entries, session logs or "recent work" to any file. Put what happened in the PR description and close the issue with `Fixes #N`. Edit a doc only when the behavior it describes changes.
3. **NEVER** add business logic to handlers.
4. **NEVER** import Echo types outside of handler files.
5. When creating a new plugin, copy structure from an existing one and create its `.ai.md`.
6. When making an architecture decision, record it in `.ai/decisions.md` as a short ADR (context, decision, consequences). Implementation detail belongs in the PR.
7. Add comments to every package, exported type, and non-obvious code block.
8. Plugins talk to each other via **service interfaces**, never direct repo access.
9. Systems are **read-only** -- they serve reference content but never modify campaign data.
10. Widgets communicate via **DOM events** and **API endpoints**.
