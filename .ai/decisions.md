# Architecture Decision Records

<!-- ====================================================================== -->
<!-- Category: Semi-static (APPEND-ONLY)                                      -->
<!-- Purpose: Records WHY decisions were made. Prevents revisiting settled     -->
<!--          questions. ADR numbers and heading lines are never reused,       -->
<!--          renumbered or removed — code cites them. A record's body may     -->
<!--          be tightened for length, and its status line updated, but its    -->
<!--          decision and rules must not change meaning.                     -->
<!-- Status line: one line right under the heading — Accepted |               -->
<!--   Accepted; amended by ADR-NNN | Superseded by ADR-NNN | Moot (reason).   -->
<!-- Update: Append a new record when a significant decision is made.         -->
<!-- Template: See .ai/templates/decision-record.md.tmpl                      -->
<!-- ====================================================================== -->

---

## ADR-001: Three-Tier Extension Architecture (Plugins, Modules, Widgets)

**Status:** Accepted

**Context:** Chronicle needs complete compartmentalization. Every feature should
be its own self-contained unit. But there are fundamentally different kinds of
extensions: full feature apps, game system content packs, and reusable UI pieces.

**Decision:** Three tiers:
- **Plugins** (`internal/plugins/`): Feature apps with handler/service/repo/templates.
  Core plugins (auth, campaigns, entities) always enabled. Optional plugins
  (calendar, maps, timeline) enabled per-campaign.
- **Systems** (`internal/systems/`): Game system content packs (Draw Steel, D&D 5e,
  Pathfinder 2e). Reference data, tooltips, dedicated pages. Read-only.
  Installed via package manager.
- **Widgets** (`internal/widgets/`): Reusable UI building blocks (editor, title,
  tags, attributes, mentions). Mount to DOM, fetch own data.

**Alternatives Considered:**
- Flat structure for everything: conflates apps with UI components
  and content packs. Naming becomes ambiguous.
- Plugin-only: widgets and modules have fundamentally different structures.

**Consequences:**
- Clear separation of concerns per tier.
- Each tier has its own directory structure template.
- Cross-tier deps flow downward: Plugins may use Widgets. Modules may use
  Widgets. Widgets are self-contained.

---

## ADR-002: MariaDB Over PostgreSQL

**Status:** Accepted

**Context:** Original spec called for PostgreSQL, but deployment target (Cosmos
Cloud) and user infrastructure use MariaDB.

**Decision:** MariaDB with `database/sql` + `go-sql-driver/mysql`. No ORM.

**Alternatives Considered:**
- PostgreSQL: richer features (JSONB, tsvector) but doesn't match user infra.
- SQLite: doesn't support concurrent writes for multi-user web app.

**Consequences:**
- No JSONB -- use MariaDB `JSON` columns (validated on write).
- No `tsvector` -- use MariaDB `FULLTEXT` indexes.
- No `gen_random_uuid()` -- generate UUIDs in Go (`uuid.New()`).
- Use `?` placeholders instead of `$1` in SQL.

---

## ADR-003: Hand-Written SQL Over ORM or sqlc

**Status:** Accepted

**Context:** Need a SQL layer. Options: ORM (GORM), code generator (sqlc),
hand-written.

**Decision:** Hand-written SQL in repository files.

**Alternatives Considered:**
- GORM: magic behavior, N+1 queries, hard to optimize.
- sqlc: excellent for Postgres but MySQL support is immature.

**Consequences:**
- Full control over query performance.
- More verbose but explicit.
- Each repository is self-contained.

---

## ADR-004: HTMX + Templ Over SPA Framework

**Status:** Accepted

**Context:** Frontend needs interactivity without Node.js build chain.

**Decision:** Server-side rendering with Templ + HTMX. Alpine.js for
client-only interactions.

**Alternatives Considered:**
- React/Vue SPA: requires Node.js build pipeline.
- Go html/template: no type safety, no components.

**Consequences:**
- No JSON API needed for UI (HTMX speaks HTML).
- Simpler build pipeline.
- Every handler checks `HX-Request` for fragment vs full page.

---

## ADR-005: PASETO v4 Over JWT

**Status:** Accepted

**Context:** Need secure tokens for sessions and API auth.

**Decision:** PASETO v4 for all tokens.

**Alternatives Considered:**
- JWT: algorithm confusion attacks, `none` algorithm, key confusion.

**Consequences:**
- No algorithm confusion attacks (PASETO mandates algorithms per version).
- Less library support than JWT, but Go has solid PASETO libs.

---

## ADR-006: Go Binary Serves HTTP Directly (No Nginx)

**Status:** Accepted

**Context:** Cosmos Cloud provides its own reverse proxy.

**Decision:** Echo serves HTTP directly. No nginx/caddy in container. Cosmos
handles TLS, domain routing, DDoS.

**Consequences:**
- Single-process container (just Go binary).
- Simpler Dockerfile, faster startup.
- No exposed ports in docker-compose -- Cosmos routes internally.

---

## ADR-007: Configurable Entity Types with JSON Fields

**Status:** Accepted

**Context:** Kanka has fixed entity types. Users want custom types and fields.

**Decision:** Entity types stored in DB with `fields` JSON column defining
field definitions. Drives both edit forms and profile display dynamically.

**Consequences:**
- GMs can add/remove/reorder fields per entity type per campaign.
- New entity types without code changes.
- JSON queries less performant but entity type defs are small and cached.

---

## ADR-008: Game Systems as Read-Only Modules

**Status:** Accepted

**Context:** Users want D&D 5e, Pathfinder, Draw Steel reference content
available as tooltips and pages.

**Decision:** Game systems are "Modules" -- separate tier from Plugins.
Ship static data, provide tooltip API, render reference pages. Read-only.
Enabled/disabled per campaign.

**Alternatives Considered:**
- Embed in entities system: conflates user content with reference data.
- External API calls: adds latency and external deps for self-hosted.

**Consequences:**
- Reference data ships with Docker image.
- Simpler structure than plugins (no service/repo).
- @mentions can reference both campaign entities AND module content.
- Must only include SRD/OGL content (legal).

---

## ADR-009: Dual Permission Model (Action vs Content Visibility)

**Status:** Accepted

**Context:** Site admins need to manage campaigns (delete, force-transfer) without
necessarily seeing all campaign content. A site admin who is also a player in a
campaign shouldn't be spoiled by seeing GM-only content.

**Decision:** Two distinct permission concepts:
1. **Action permissions** -- "can this user perform admin actions?" Checks
   `users.is_admin` flag. Admin actions go through `/admin` routes.
2. **Content visibility** -- "what content can this user see?" Uses the actual
   `campaign_members.role` value. No admin bypass for content.

An admin joining as Player sees only Player-visible content. An admin who hasn't
joined has `MemberRole=RoleNone` (no content access) but can still perform admin
actions via the admin panel.

**Role levels:** Player (1) < Scribe (2) < Owner (3). Admin is site-wide, not a
campaign role. `RequireRole(min)` checks `MemberRole >= min`.

**Alternatives Considered:**
- Single permission model with admin override: admins would always see everything,
  ruining the player experience for admin-players.
- Separate admin accounts: inconvenient for small servers where the admin is also
  a player.

**Consequences:**
- Admins can enjoy campaigns as players without spoilers.
- Admin operations are cleanly separated into `/admin` routes.
- Campaign routes never check `is_admin` -- only membership role.
- Future entity permissions (is_private) will respect MemberRole, not admin flag.

---

## ADR-010: SMTP Password Encryption with AES-256-GCM

**Status:** Accepted

**Context:** SMTP settings include a password that must be stored securely. The
password must be encrypted at rest and NEVER returned to the UI.

**Decision:** AES-256-GCM encryption with key derived from `SHA-256(SECRET_KEY)`.
Nonce prepended to ciphertext. Password decrypted only at send time, never cached.
UI shows `HasPassword: bool` only.

Empty password on update = keep existing. SECRET_KEY rotation makes stored password
unrecoverable -- admin must re-enter.

**Alternatives Considered:**
- Bcrypt/argon2id hash: can't decrypt to use for SMTP auth.
- Environment variable only: less flexible for web-based management.
- Reversible encryption with separate key: unnecessary complexity.

**Consequences:**
- Password encrypted at rest using app's SECRET_KEY.
- No password recovery -- by design. Admin re-enters on key rotation.
- Single encryption key (SECRET_KEY) for simplicity.
- If SECRET_KEY leaked, SMTP password is compromised (acceptable tradeoff
  for self-hosted). Document key management best practices.

---

## ADR-011: Sidebar Customization via Campaign JSON Column

**Status:** Accepted

**Context:** Campaign owners want to reorder and hide entity types in the sidebar
to match their campaign's focus (e.g., hide "Events" if not used, promote
"Characters" to the top).

**Decision:** Store sidebar configuration as JSON in `campaigns.sidebar_config`
column (migration 000006). Config contains `entity_type_order` (ordered list
of type IDs) and `hidden_type_ids`. LayoutInjector applies the config before
rendering. Client-side drag-to-reorder widget with auto-save via PUT API.

**Alternatives Considered:**
- Separate `sidebar_order` table: more normalized but overkill for a simple
  ordered list. One campaign has at most ~20 entity types.
- Store order in `entity_types.sort_order`: sort_order is type-global, not
  per-campaign. Two campaigns sharing the same type definitions would conflict.

**Consequences:**
- Simple single-column storage, no joins needed.
- Config parsed on every page render (small JSON, negligible overhead).
- Graceful degradation: malformed JSON falls back to default sort_order.
- Owner-only access -- players cannot customize the sidebar.

---

## ADR-012: Entity Type Layout Builder with JSON Column

**Status:** Accepted

**Context:** Entity profile pages need customizable layouts -- different entity
types should display their sections in different arrangements (e.g., Characters
might want "Basics" fields in a left sidebar with the entry in the main column).

**Decision:** Store layout configuration as JSON in `entity_types.layout_json`
column (migration 000007). Layout defines sections with key/label/type/column
properties. "column" is either "left" (sidebar) or "right" (main). Section types
are "fields", "entry", or "posts". Client-side two-column drag-and-drop widget.

**Alternatives Considered:**
- Separate layout_sections table: over-normalized for what is always read as a
  unit. The JSON blob is never queried individually.
- Hardcoded layouts per entity type: inflexible, defeats the purpose.

**Consequences:**
- Layout config read with entity type, no additional query.
- Sections validated server-side (valid types, valid columns, unique keys).
- Default layout auto-generated from field definitions when empty.
- Entity show page reads layout_json to render the profile — wired via `BlockRegistry` + `DefaultLayout()`/`CharacterLayout()`, consumed by `show.templ`.

---

## ADR-013: Pessimistic Locking for Shared Notes

**Status:** Accepted

**Context:** Shared notes can be edited by any campaign member. Without
concurrency control, two users editing the same note simultaneously would
overwrite each other's changes (last-write-wins).

**Decision:** Pessimistic edit locking with 5-minute auto-expiry. When a user
starts editing a shared note, the client acquires a lock via `POST /lock`.
While held, the lock is kept alive with a 2-minute heartbeat interval. Stale
locks (older than 5 minutes without heartbeat) are automatically reclaimed
by the lock acquisition query. Campaign owners can force-unlock any note.

**Alternatives Considered:**
- Optimistic concurrency (version counter + conflict detection): more complex
  client-side merge resolution. Notes panel is a lightweight widget, not a
  full collaborative editor -- pessimistic locking is simpler and sufficient.
- Real-time collaborative editing (CRDT/OT): massive complexity for a notes
  sidebar. This is Google Docs-level infra; overkill for a notes widget.
- No locking: acceptable for private notes (single user), but shared notes
  need protection against concurrent edits.

**Consequences:**
- Only one user can edit a shared note at a time.
- Lock state stored in the notes table itself (locked_by, locked_at columns).
- Stale locks self-heal via age check in the acquisition query.
- Private (non-shared) notes skip locking entirely -- only the owner edits them.
- 5-minute timeout is generous enough for slow typists but prevents abandoned locks.

---

## ADR-014: Snapshot-on-Save Version History for Notes

**Status:** Accepted

**Context:** Users need to recover previous versions of notes, especially
when shared notes are edited by multiple people.

**Decision:** Create a version snapshot before every content-changing operation
(Update and RestoreVersion). Snapshots store title, content blocks, entry JSON,
and entry HTML. Maximum 50 versions per note, oldest auto-pruned. Version
creation errors are swallowed -- version tracking is non-critical.

**Alternatives Considered:**
- Changelog-style diffs: more storage-efficient but requires complex diff/merge
  to reconstruct a version. Snapshots are simpler and notes are small.
- Event sourcing: overkill. Notes are not high-frequency write targets.
- No version history: risky with shared editing. Users expect undo capability.

**Consequences:**
- Every update creates a version row -- storage grows linearly but is bounded at 50.
- Auto-pruning runs after every version creation (DELETE subquery).
- Restore is a two-step operation: snapshot current state, then apply old version.
- Version errors don't block the save operation (swallowed with `_ = err`).

---

## ADR-015: Maps with Percentage Coordinates and Leaflet CRS.Simple

**Status:** Accepted

**Context:** Maps plugin needs to display pin markers on uploaded background images.
Markers must be positioned relative to the image, independent of actual pixel resolution.

**Decision:** Store marker coordinates as percentages (0-100 for both X and Y).
Use Leaflet.js with CRS.Simple to create a non-geographic coordinate system where
the image is overlaid. Leaflet converts percentage coords to pixel space at render
time based on image dimensions stored on the map record.

Multiple maps per campaign (unlike calendar's 1:1). Maps are listed on an index page.

**Alternatives Considered:**
- Pixel coordinates: breaks when image is resized or replaced with different resolution.
- Geographic coordinates (lat/lng): adds complexity for fantasy maps with no real-world
  mapping. CRS.Simple avoids this entirely.
- Canvas-based rendering: more work, less accessible, no built-in panning/zooming.

**Consequences:**
- Markers are resolution-independent. Image can be swapped with different sizes.
- Leaflet loaded from CDN per-page (not globally) to avoid loading JS on non-map pages.
- Image dimensions (width/height) must be stored on the map record for coordinate space.
- Draggable markers use silent PUT on dragend -- no save button needed.

---

## ADR-016: Inline Secrets via TipTap Mark Extension

**Status:** Accepted

**Context:** GMs need to write inline secret text within entity entries that only
they and scribes can see. Players should never receive the secret content -- it must
be stripped server-side, not just hidden with CSS.

**Decision:** Create a TipTap `secret` mark that renders as
`<span data-secret="true" class="chronicle-secret">`. Since the vendored TipTap
bundle doesn't export the raw `Mark` class, extend `TipTap.Underline` (which IS a
Mark subclass) and override name, parseHTML, renderHTML, commands, and shortcuts.

Server-side stripping in `internal/sanitize/`:
- `StripSecretsHTML()` -- regex strips `<span data-secret>...</span>` from HTML.
- `StripSecretsJSON()` -- recursive tree walk removes text nodes with `secret` mark
  from ProseMirror JSON.

Applied in `GetEntry` handler when `role < RoleScribe`.

**Alternatives Considered:**
- CSS-only hiding: insecure -- HTML still sent to client, visible in DevTools.
- Separate "GM notes" field: less flexible than inline secrets mixed with regular text.
- Build custom TipTap bundle with Mark export: adds Node.js build step, breaks
  vendored-only constraint.

**Consequences:**
- Secret content never reaches players (server-stripped from both JSON and HTML).
- Mark extension uses Underline.extend() hack -- works but is coupled to Underline
  being present in the bundle.
- Bluemonday whitelist updated to allow `data-secret` on `<span>`.
- CSS shows amber background + eye-slash indicator for owners/scribes in edit mode.

## ADR-017: Add 'plugin' to Addon Category ENUM

**Status:** Accepted

**Context:** The `addons.category` ENUM had three values: `module`, `widget`,
`integration`. Calendar and Maps are architecturally Plugins (full feature apps with
handler/service/repo/templates), not Widgets. The original migration 000015 seed data
miscategorized them as `widget` because the Plugin tier hadn't been reflected in the
database schema. Migrations 000027 and 000029 attempted to INSERT with
`category='plugin'`, causing a MariaDB "Data truncated" error (Error 1265). A
secondary duplicate slug conflict also existed since the rows were already seeded.

**Decision:** Add `plugin` as a fourth ENUM value via ALTER TABLE in migration 000027.
Use UPDATE instead of INSERT to fix existing seed data rows. Add `CategoryPlugin`
constant to Go code and validation. Add migration SQL validation tests as a safeguard.

**Alternatives Considered:**
- Keep only three categories and map plugins to `widget`: semantically wrong. Plugins
  are full feature apps, not reusable UI blocks.
- Change the column from ENUM to VARCHAR: loses the schema-level validation benefit
  of ENUM. The four-value ENUM is small and stable.

**Consequences:**
- The category ENUM now has four values: `plugin`, `module`, `widget`, `integration`.
- All future plugin registrations should use `category='plugin'`.
- Down migration for 000027 must revert the ENUM (requires no rows use `plugin`).
- Migration validation test in `internal/database/migrate_test.go` catches invalid
  ENUM values at `make test` time.

---

## ADR-018: D3.js for Timeline Visualization

**Status:** Accepted
**Context:** The timeline plugin needs an interactive visualization with zoom/pan/drag,
time scales, and entity group swim-lanes. We already use Leaflet.js for the maps plugin.

**Decision:** Use D3.js v7 for the timeline visualization. Load from CDN per-page
(matching Leaflet pattern), not bundled globally. D3 provides SVG-based rendering,
`d3.zoom` for pan/drag, `d3.scaleLinear` for time axes, and transitions. Leaflet.js
is designed for geographic tile-based rendering and is unsuitable for time-axis layouts.

**Alternatives Considered:**
- Leaflet.js: Already in the project for maps, but fundamentally geographic. Would
  require fighting the library's coordinate system and tile-based assumptions.
- vis-timeline: Purpose-built timeline library, but opinionated about styling and
  harder to customize for swim-lanes, fantasy calendars, and Chronicle's dark theme.
- Canvas-based rendering: Better performance for very large datasets, but loses SVG's
  accessibility, CSS styling integration, and text rendering quality.

**Consequences:**
- D3 v7 (~90KB gzipped) loaded only on timeline detail pages, no impact on other pages.
- SVG rendering gives full CSS control, accessibility, and crisp text at all zoom levels.
- Swim-lanes, zoom levels, and entity grouping can be implemented incrementally.
- Fantasy calendar dates (arbitrary year/month/day systems) work naturally with
  `d3.scaleLinear` since we convert to fractional years for positioning.

---

## ADR-023: Sessions-Calendar Integration and RSVP Email System

**Status:** Accepted

**Context:** Sessions were a standalone plugin with their own sidebar link and
addon toggle. Users expected sessions to appear on the calendar (especially
real-life mode calendars) and wanted RSVP from the calendar UI. The separate
sidebar link was confusing — sessions are fundamentally a calendar feature.

**Decision:**
- **Sessions require the calendar addon** — no separate "sessions" addon toggle.
  The sidebar link for sessions is removed; sessions are accessed via the
  calendar's dice icon and Sessions button in the calendar header.
- **Sessions display on real-life calendar grids** as purple chips with a dice
  icon. Clicking opens an inline modal with RSVP controls (Going/Maybe/Can't).
- **Recurring sessions** supported: weekly, biweekly, monthly, and custom N-week
  intervals. Stored on the sessions table with recurrence_type/interval fields.
- **RSVP via email**: SMTP SendHTMLMail added for multipart/alternative emails.
  Each invitation generates single-use tokens (7-day expiry) for one-click
  accept/decline links without requiring login.
- **RequireAddon middleware**: Route-level addon gating via AddonService.IsEnabledForCampaign
  query. Applied to calendar, maps, sessions, timeline, and media-gallery route groups.
- **Date formatting**: Session dates use `FormatScheduledDate()` returning
  "Mon, Jan 2, 2006" instead of raw ISO 8601.

**Alternatives considered:**
- Merging sessions into the calendar_events table: Rejected because sessions have
  attendees, RSVP tracking, entity linking, and notes — fundamentally different
  from calendar events. Keeping separate tables is cleaner.
- JWT-based RSVP tokens: Rejected for simplicity. Random tokens with DB lookup
  are simpler, revocable, and auditable.

Open work: #672 (Discord bot integration — reaction-based RSVP over the same
`SessionService` interface the email flow uses).

**Consequences:**
- Sessions sidebar link removed — users navigate via calendar.
- Disabled addons now return 404/redirect at the route level, not just hidden sidebar links.
- SMTP service supports both plain text and HTML email variants.
- Session RSVP tokens stored in session_rsvp_tokens table with FK cascade.

---

## ADR-019: Manifest-Driven Module Framework

**Status:** Accepted

**Context:** The module system had a static hardcoded registry listing three
coming-soon modules with no runtime infrastructure. We need a framework that
supports auto-discovery, validation, and a sandboxed interface for modules
to implement without accessing the database or Echo router.

**Decision:** Replace the static registry with a manifest-driven framework:

1. **manifest.json** — Each system declares metadata in a JSON file: id, name,
   version, author, license, categories, API version, entity presets, etc.
2. **SystemLoader** — Scans `internal/systems/*/manifest.json` and package-installed
   systems at startup, validates required fields, logs warnings for invalid
   manifests without failing startup.
3. **Module interface** — Sandboxed: `Info() *ModuleManifest`,
   `DataProvider() DataProvider`, `TooltipRenderer() TooltipRenderer`.
   Modules can only serve data through these interfaces.
4. **DataProvider interface** — `List(category)`, `Get(category, id)`,
   `Search(query)`, `Categories()` returning `ReferenceItem` structs.
5. **Global Init()** — Called once at startup, populates the singleton registry.

**Alternatives Considered:**
- Database-stored manifests: adds unnecessary complexity for static content packs.
- Go struct registration (current approach): no separation of metadata from code,
  no validation, no path to external module loading.

**Consequences:**
- Modules are self-describing via manifest.json (human-readable, validatable).
- Auto-discovery eliminates manual registry maintenance.
- Sandboxed interfaces prevent modules from accessing infrastructure directly.
- Admin modules page shows manifest metadata (author, license, API version).
- Module slugs added to installedAddons for per-campaign enable/disable.
- ADR-020 builds HTTP handlers and DataProvider implementations on this foundation.

---

## ADR-020: JSON-File DataProvider with Factory Registry

**Status:** Accepted

**Context:** ADR-019 delivered the Module/DataProvider/TooltipRenderer interfaces
and auto-discovery. This ADR needs a concrete DataProvider implementation, the
first module (D&D 5e), and HTTP handlers. The challenge: module subpackages (dnd5e/)
import the parent modules package for interfaces, but the loader in modules/
cannot import subpackages without creating circular imports.

**Decision:** Three key design choices:

1. **JSONProvider** — Generic DataProvider implementation that loads `data/*.json`
   files from a module's directory. Filename stem becomes the category slug.
   Items loaded into memory at startup. Case-insensitive search across Name,
   Summary, and Tags.

2. **Factory Registry** — Modules register factory functions via
   `modules.RegisterFactory(id, fn)` in their package `init()` functions.
   The loader calls registered factories during DiscoverAll() for modules
   with status "available". This avoids circular imports: the parent package
   holds the factory map, subpackages register themselves, and `app/routes.go`
   uses blank imports (`_ "modules/dnd5e"`) to trigger init().

3. **Dynamic Addon Middleware** — Module routes use `/campaigns/:id/modules/:mod`
   with middleware that reads the `:mod` param and checks `addonSvc.IsEnabledForCampaign()`
   dynamically, rather than requiring a separate route group per module.

**Alternatives Considered:**
- Direct import of dnd5e in loader.go: creates circular import.
- Plugin-style registration in app/routes.go: too much wiring code, doesn't scale.
- Separate handler per module: unnecessary duplication since all modules share the
  same reference page structure.

**Consequences:**
- Adding a new module requires: manifest.json, data/*.json, a Go file with init()
  factory registration, and a blank import in app/routes.go.
- Module reference pages are generic (same Templ templates for all modules).
- Module content appears in entity @mention search when the module addon is enabled.
- TooltipAPI returns module-specific HTML via the TooltipRenderer interface.

---

## ADR-021: Layered Third-Party Extension Strategy

**Status:** Accepted. All three layers are implemented in `internal/extensions/`
(manifest/applier + widget registration + `wasm_host.go` on Extism/wazero,
pinned in `go.mod`).

**Context:** Chronicle's three-tier architecture (Plugins, Modules, Widgets) is
currently internal-only — all extensions ship with the Go binary. Users and the
community want to create and share content packs, custom widgets, and eventually
custom backend logic without forking the codebase. Research was conducted across
WordPress, Grafana, Discourse, Obsidian, Foundry VTT, and Shopify, plus Go-specific
approaches (HashiCorp go-plugin, WASM/Extism/wazero, GopherLua).

**Key findings from research:**
- No mainstream self-hosted platform truly sandboxes plugins except Grafana
  (subprocess isolation via gRPC) and Shopify (restricted Liquid rendering).
- WordPress, Discourse, Obsidian, and Foundry VTT all run plugins in-process
  with full access — security relies entirely on trust and code review.
- WASM (via Extism + wazero) is the most promising approach for a Go backend
  wanting user-uploadable extensions with real sandboxing: memory-safe isolation,
  capability-based security, language-agnostic authoring, pure Go runtime.
- Foundry VTT's patterns are directly relevant as a TTRPG competitor: manifest
  format, Flags storage, hook-based events, manifest URL updates.

**Decision:** Three layers of third-party extensibility, implemented incrementally:

### Layer 1: Content Extensions (Manifest-Only, No Code)
Declarative content packs distributed as zip archives containing a `manifest.json`
plus static assets (JSON data files, images, CSS). No executable code. Examples:
monster packs, map tile sets, pre-built entity templates, custom field definitions,
calendar presets, theme variants.

- **Manifest**: JSON declaring id, name, version, author, compatibility, contents.
- **Installation**: Upload zip via admin UI or place in `extensions/` directory.
- **Storage**: Extension data stored in DB via a generic extension data table.
  Inspired by Foundry VTT's Flags system (namespaced key-value on documents).
- **Security**: No code execution. Manifest validated, file types allowlisted.
- **Covers**: ~60% of what TTRPG users actually want to share.

### Layer 2: Widget Extensions (Browser-Sandboxed JS)
Custom widgets that self-register via `Chronicle.registerWidget()` and mount to
DOM elements. They run in the browser, can only hit existing API endpoints, and
are naturally sandboxed by the browser same-origin policy.

- **Distribution**: Bundled in content extension zips (a JS file in the package).
- **API**: `Chronicle.registerWidget(name, { mount, unmount, config })`.
- **Security**: Browser sandbox. Widgets use Chronicle.apiFetch() which includes
  CSRF tokens. Cannot access server filesystem or database directly.
- **Covers**: Custom UI blocks, visualization widgets, interactive tools.

### Layer 3: Logic Extensions (WASM-Sandboxed Backend)
Custom backend logic compiled to WebAssembly and executed via Extism + wazero.
Plugins are `.wasm` files with capability-based security: no filesystem, no
network, no database unless the host explicitly grants it through defined
host functions.

- **Runtime**: wazero (pure Go, zero CGO) via Extism SDK.
- **Host functions**: Chronicle exposes specific APIs (read entity, list tags,
  create event) as host functions. Plugins can only call what's exposed.
- **Distribution**: `.wasm` files in extension packages, hash-verified.
- **Use cases**: Custom validation rules, automated entity generation,
  game-system-specific calculators, webhook processors.

**Alternatives Considered:**
- HashiCorp go-plugin (gRPC subprocess per plugin): Battle-tested by Terraform
  and Grafana but designed for operator-installed compiled binaries, not
  user-uploaded extensions. Per-process overhead is heavy for many small TTRPG
  extensions.
- GopherLua (embedded Lua VM): Lightweight and familiar to game/modding
  communities. Could serve as intermediate between Layers 2 and 3 for simple
  automation/macros. May be added as Layer 2.5 if demand warrants.
- No sandboxing (WordPress/Foundry model): Unacceptable for a self-hosted
  platform where users upload community content. Security-by-trust doesn't scale.
- Signing-only (Grafana model): Good defense-in-depth but insufficient alone.
  Chronicle should implement SHA-256 manifest signing regardless of sandbox choice.

**Consequences:**
- Content extensions cover the majority of community sharing needs without code.
- Widget extensions leverage the existing boot.js auto-mounter and apiFetch infrastructure.
- The WASM layer gives backend extensibility real sandboxing, not trust-by-review.
- Each layer can be shipped independently; later layers don't block earlier ones.
- Manifest format and extension installer are shared infrastructure across all layers.
- Extension signing (SHA-256 checksums in signed manifest, inspired by Grafana)
  should be implemented for all layers as defense-in-depth.

---

## ADR-022: WASM Runtime via Extism SDK + wazero

**Status:** Accepted

**Context:** Layer 3 of ADR-021 called for WASM-sandboxed backend logic. With
Layers 1 (content) and 2 (widgets) proven, we need to implement the WASM runtime
to allow community-authored backend logic (custom validation, calculators,
automation) without giving extensions direct access to the database or filesystem.

**Decision:** Use the Extism Go SDK (v1.7.1) with wazero (v1.9.0) as the WASM
runtime. Key design choices:

1. **Capability-based security** — Plugins declare required capabilities in their
   manifest (`contributes.wasm_plugins[].capabilities`). The PluginManager only
   exposes host functions matching declared capabilities. Five capability groups:
   `log`, `entity_read`, `calendar_read`, `tag_read`, `kv_store`.

2. **Capabilities gate reads and writes separately** — read-only host functions
   (get_entity, search_entities, list_entity_types, get_calendar, list_events,
   list_tags, kv_get/set/delete, chronicle_log) plus write functions
   (`update_entity_fields` under `entity_write`, a calendar write under
   `calendar_write`) that a manifest must declare separately to reach.

3. **Per-plugin KV store via extension_data** — Reuses the existing `extension_data`
   table with namespace "wasm_kv" instead of creating new tables. Each plugin's
   data is scoped by campaign_id + extension_id.

4. **Async hook dispatch** — WASM plugins register for events via manifest `hooks`
   field. Events are dispatched fire-and-forget in goroutines. Plugin failures
   never affect the originating operation.

5. **Resource limits** — Default 16 MB memory, 30s timeout per call. Manifests
   can override up to 256 MB memory and 300s timeout. Instruction-fuel metering
   is wired (`FuelLimit`) but unlimited by default.

6. **Adapter interfaces** — EntityReader, CalendarReader, TagReader interfaces
   decouple WASM host functions from concrete plugin implementations, following
   the existing adapter pattern used throughout Chronicle.

**Alternatives Considered:**
- Direct wazero without Extism: More control but requires reimplementing plugin
  manifest handling, host function registration, and memory management that Extism
  provides out of the box.
- GopherLua: Lighter weight but Lua-only. WASM supports Rust, Go/TinyGo, JS,
  Python, and any language with a WASM target.
- gRPC subprocess model (HashiCorp go-plugin): Better for operator-installed
  plugins but too heavy for user-uploaded community extensions.

**Consequences:**
- WASM plugins are truly sandboxed: no filesystem, no network, no database access
  except through explicitly declared host functions.
- Community can author plugins in any language that compiles to WASM.
- Plugin lifecycle (load/unload/reload) managed centrally by PluginManager.
- Hook system enables reactive plugins without polling.
- KV store provides durable per-plugin state without new database tables.

## ADR-024: Extension Migration System (Dynamic Schema)

**Status:** Accepted

**Context:** The current migration system (sequential numbered SQL files via
golang-migrate) works for core schema but cannot handle dynamic extensions. When
a user uploads an extension that needs its own tables, and later disables or
removes it, the core migration pipeline has no mechanism for this. Extensions
should not modify the core migration sequence.

**Decision:**
Extensions use a **separate, per-extension migration system** alongside core:

1. **Core migrations** — remain as-is (sequential `000NNN_*.sql` files). These
   define the platform schema and run on every startup.

2. **Extension migrations** — each extension's zip manifest declares a `migrations/`
   directory containing numbered SQL files scoped to that extension. When an
   extension is installed, its migrations run against a tracking table
   (`extension_schema_versions`) keyed by `(extension_id, version)`.

3. **Namespaced tables** — extension-created tables MUST be prefixed with `ext_`
   followed by the extension slug (e.g., `ext_knowledge_graph_nodes`). This
   prevents collisions with core tables and makes cleanup straightforward.

4. **Install/uninstall lifecycle**:
   - **Install**: Run extension's `up` migrations in order.
   - **Uninstall**: Run extension's `down` migrations in reverse, then delete
     tracking rows. All `ext_<slug>_*` tables are dropped.
   - **Disable**: Tables and data stay intact (campaign-level toggle only).
   - **Enable**: No migration action needed (data preserved).

5. **Campaign deletion**: When a campaign is deleted, extension data in
   `ext_*` tables is cleaned up via `ON DELETE CASCADE` foreign keys to
   `campaigns.id`. Extensions that create non-campaign-scoped data use the
   existing `extension_data` table (already cascaded).

6. **Validation**: Extension migrations are validated before execution:
   - Only `CREATE TABLE ext_<slug>_*` and `ALTER TABLE ext_<slug>_*` allowed
   - No `DROP TABLE` on core tables
   - No `ALTER TABLE` on core tables
   - SQL statements parsed and validated server-side

**Alternatives Considered:**
- Let extensions use only `extension_data` JSON blobs: simpler but doesn't
  support efficient queries, indexes, or foreign keys for complex extensions.
- Give extensions full migration access: too dangerous — a malicious extension
  could `DROP TABLE users`.
- Schema-per-extension (MySQL databases): MariaDB doesn't truly isolate, and
  cross-database JOINs are needed for host functions.

**Consequences:**
- Extensions can define proper relational schemas when JSON blobs aren't enough.
- Core migration system stays simple and predictable.
- Uninstalling an extension cleanly removes all its schema artifacts.
- The `ext_` prefix convention makes it trivial to audit what extensions own.

## ADR-025: Campaign Deletion Cascade and Cleanup

**Status:** Accepted

**Context:** When a campaign is deleted, database CASCADE handles most rows, but
several gaps exist: media files are orphaned on disk (SET NULL, not CASCADE),
API keys lack foreign key constraints entirely, and extension-provisioned content
(entities, tags created by extensions) remains even after provenance records
are cascaded.

**Decision:**
Campaign deletion becomes a **multi-step service operation** instead of a single
SQL DELETE:

1. **Media file cleanup** — Before the SQL DELETE, query all `media_files` where
   `campaign_id = ?`, delete physical files from disk (main + thumbnails), then
   delete the DB rows. This replaces the current `ON DELETE SET NULL` behavior
   for campaign-scoped media. Avatars and backdrops (campaign_id IS NULL) are
   unaffected.

2. **API key cascade** — Add proper `FOREIGN KEY (campaign_id) REFERENCES
   campaigns(id) ON DELETE CASCADE` to `api_keys`. API request logs get
   `ON DELETE SET NULL` (retain for audit trail, but disassociate from campaign).

3. **Extension content cleanup** — Before delete, query `extension_provenance`
   for the campaign to find extension-created records (entity types, entities,
   tags, etc.). These are already CASCADE'd through their own campaign_id FKs,
   so no extra work needed. The provenance records themselves cascade.

4. **Extension table cleanup** — For extensions with `ext_*` tables, rows with
   the campaign_id are cleaned up via CASCADE FKs (required by ADR-024).

5. **WASM plugin state** — `extension_data` rows are already CASCADE'd. WASM
   plugins with in-memory state receive a `campaign.deleted` hook event so they
   can clean up caches.

6. **Non-default uploaded extensions** — When a campaign is deleted, uploaded
   extensions that are ONLY enabled for that campaign are flagged for cleanup.
   If no other campaign uses the extension, the extension zip and its `ext_*`
   tables can be uninstalled. This is a background job, not synchronous.

**Consequences:**
- Campaign deletion is slightly slower (disk I/O for media) but leaves no
  orphaned data.
- API keys are properly invalidated on campaign delete (security fix).
- Extensions can trust that campaign deletion is thorough.
- The media `CleanupOrphans()` method becomes a safety net, not the primary
  cleanup mechanism.

## ADR-026: Admin Data Hygiene Dashboard

**Status:** Accepted

**Context:** Over time, the database accumulates orphaned data: media files
without campaigns, API keys pointing to deleted campaigns, extension records
with no parent, etc. Admins need visibility into this and tools to clean it up
safely — but also guardrails to prevent accidentally deleting data that active
campaigns still depend on.

**Decision:**
Add an admin "Data Hygiene" page at `/admin/data-hygiene` with read-only
diagnostics and guarded cleanup actions:

1. **Orphan detection queries** — Read-only scans that identify:
   - Media files with `campaign_id IS NULL` that aren't avatars/backdrops
     (orphaned by campaign deletion or SET NULL)
   - Media files on disk with no matching DB record (stale filesystem artifacts)
   - API keys referencing non-existent campaigns (pre-FK-fix orphans)
   - Extension provenance records pointing to deleted records
   - `ext_*` tables with no matching installed extension
   - Notes/note_versions for deleted campaigns (if any escaped CASCADE)
   - Users with no campaign memberships (not necessarily orphaned — could be new)

2. **Safety guardrails** — Cleanup actions are blocked when data is still
   referenced:
   - Cannot delete a media file that is referenced by any entity's `image_path`
     or `entry_html`
   - Cannot delete an extension that has campaigns with it enabled
   - Cannot purge API keys for campaigns that still exist
   - Each action shows a preview of what will be affected before confirming
   - All cleanup actions are logged to `security_events` for audit trail

3. **Cleanup actions** (admin-only, confirmation required):
   - "Purge orphaned media" — deletes files from disk + DB rows for
     campaign-less media not referenced by any entity
   - "Purge stale filesystem files" — deletes files on disk with no DB record
   - "Purge orphaned API keys" — deletes keys for non-existent campaigns
   - "Run media orphan scan" — invokes `CleanupOrphans()` with dry-run option

4. **Dashboard stats** — Summary cards showing:
   - Total disk usage vs DB-tracked usage (delta = stale files)
   - Orphaned media count + size
   - Orphaned API key count
   - Extension table count vs installed extension count

5. **No automated cleanup** — All actions are manual and admin-initiated.
   No cron jobs or background workers that silently delete data. The admin
   decides when to clean up and reviews what will be affected.

**Alternatives Considered:**
- Automated background cleanup on schedule: too risky — could delete data
  during a race condition (e.g., campaign being restored from backup).
- Per-campaign cleanup page: campaigns already cascade; the problem is
  cross-campaign orphans that only a site admin can see.

**Consequences:**
- Admins have full visibility into database/filesystem health.
- No data is ever deleted without explicit admin action + confirmation.
- Safety checks prevent accidental deletion of in-use data.
- Complements ADR-025 (campaign deletion cleanup) as a catch-all safety net.

---

## ADR-027: RequireAddon Middleware Fail-Open on DB Errors

**Status:** Accepted

**Context:** The `RequireAddon` middleware checks whether an addon (calendar,
maps, timeline, sessions, etc.) is enabled for a campaign before allowing access
to its routes. When the database query fails, the middleware must decide whether
to block (fail-closed) or allow (fail-open) the request.

**Decision:**
`RequireAddon` fails open on DB errors — if the addon-check query fails, the
request is allowed through. Rationale:

1. If the database is down, nothing downstream works anyway (service calls,
   repo queries all fail). Blocking at the middleware level just changes the
   error from a 500 to a redirect/404, which is less informative.
2. Fail-open matches the principle of least surprise for self-hosted instances:
   a transient DB blip doesn't lock users out of features they have enabled.
3. The companion `RequireAddonAPI` middleware (for API v1 routes) uses
   fail-closed because API callers are programmatic and can handle 503 retries.

This convention is also used by `Handler.isAddonEnabled()` in the entity search
endpoint, which skips addon-specific search results on DB errors rather than
failing the entire search.

**Alternatives Considered:**
- Fail-closed everywhere: too disruptive for a self-hosted app where DB might
  have brief connectivity issues during backups or maintenance.
- Cache addon state in Redis: adds complexity; the DB query is a single indexed
  row lookup that takes <1ms.

**Consequences:**
- During DB outages, disabled addons may briefly appear enabled (routes accessible).
- This is acceptable because the underlying service calls will fail anyway.
- API routes use stricter fail-closed behavior (ADR-025 batch 24).

---

## ADR-028: Plugin-Isolated Database Schema Architecture

**Status:** Accepted; amended by ADR-030

**Context:** Chronicle had 63 sequential migration files mixing core tables with
plugin tables. A bad migration in any plugin (e.g., Error 1553 from migration
000063) crashed the entire app and left the DB in a dirty state requiring manual
recovery. Bandaid solutions (migrate_preflight.go, lint tests) caught some issues
but couldn't prevent all classes of failures. The goal: plugin failures should
never break the app, and user-installable extensions need safe schema isolation.

**Decision:** Two-tier schema system:
- **Tier 1 (Core):** Single baseline migration (`db/migrations/000001_baseline`)
  with all core tables. Runs via golang-migrate. Failure is fatal.
- **Tier 2 (Plugins):** Each built-in plugin has its own `migrations/` directory
  (`internal/plugins/<name>/migrations/`). Runs via custom `RunPluginMigrations()`
  after core migrations. Failure disables that plugin; app continues serving.

Plugin health tracked in `PluginHealthRegistry` (thread-safe in-memory). Routes
are conditionally registered based on `IsHealthy()`. Degraded plugins show a
"Feature unavailable" banner via `plugin_unavailable.templ`.

Version tracking uses `plugin_schema_versions` table (separate from
`extension_schema_versions` used by user-installed extensions). SQL validation
is skipped for trusted built-in plugins but enforced for user extensions via
`ValidateExtensionSQL()` + `ext_<slug>_` prefix requirement.

**Alternatives considered:**
- Keep all migrations together + better preflight checks: still single point of
  failure, doesn't scale to user-installable extensions.
- Per-plugin databases: too complex, cross-plugin FKs become impossible.
- Wrap each migration in a savepoint: MariaDB doesn't support transactional DDL.

**Consequences:**
- Plugin schema failures degrade gracefully instead of crashing the app.
- Each plugin's schema is independently versioned and can evolve separately.
- Cross-plugin FK dependencies require ordered plugin migration execution
  (calendar before sessions/timeline).
- Removed migrate_preflight.go and bandaid lint tests from migrate_test.go.
- Fresh DB only — no backward compatibility with the old 63-migration sequence.

---

## ADR-029: Features Page Consolidation (Plugin Hub + Addon Settings → Single Page)

**Status:** Accepted

**Context:** Campaign feature management was split across two pages:
1. **Plugin Hub** (`/campaigns/:id/plugins`) — read-only card grid visible to all members.
2. **Addon Settings** (`/campaigns/:id/addons/settings`) — owner-only toggle list.

This created confusion: owners had two "features" pages with different layouts and
capabilities. Non-owners could see features but couldn't tell which were enabled.

**Decision:** Consolidate into a single Features page at `/campaigns/:id/plugins`.
- All members see the card grid with enable/disable status.
- Owners see inline toggle buttons on each card.
- The old `/addons/settings` route, handler, and full-page template are removed.
- The addons fragment route (`/addons/fragment`) remains for the Customization Hub.
- Toggle forms include `redirect_to=plugins` so the handler redirects back to the
  unified page after toggling.

**Alternatives considered:**
- Keep both pages with cross-links: still confusing, maintenance burden.
- Merge into the Customization Hub: too buried, features deserve top-level access.

**Consequences:**
- Single source of truth for feature management.
- Owners can manage features directly from the same page all members see.
- Future enhancements (per-addon entity usage, "offline" banners) have one target page.

---

## ADR-030: Embed Plugin Migrations via Go embed.FS

**Status:** Accepted; amends ADR-028

**Context:** ADR-028 introduced per-plugin migration directories at
`internal/plugins/<name>/migrations/`. The migration runner used `os.Stat` and
`os.ReadDir` with relative filesystem paths. This worked in development (CWD =
project root) but failed silently in Docker: the runtime image copies the binary
to `/app` but never copies plugin migration directories. Since `os.Stat` returned
`os.IsNotExist`, the runner treated each plugin as "healthy with 0 migrations"
— no tables were created, entity pages crashed, and the DB Explorer showed 0/0.

**Decision:** Embed plugin migration SQL files in the binary using Go's `embed.FS`:
- Each plugin package gets an `embed.go` that exports `MigrationsFS embed.FS`
  with `//go:embed migrations/*.sql`.
- `PluginSchema.MigrationsDir` (string) replaced with `MigrationsFS` (`fs.FS`).
- `parsePluginMigrations` and `LatestMigrationVersion` read from `fs.FS` instead
  of the real filesystem.
- `RegisteredPlugins()` moved from `database` package to `cmd/server/main.go`
  to avoid import cycles (database can't import plugin packages). Uses `fs.Sub`
  to strip the `migrations/` prefix from each embed.FS.
- `PluginSchemas` stored on `App` struct and passed to `DatabaseExplorer` for
  on-demand re-migration from the admin panel.

**Alternatives considered:**
- Copy plugin migration dirs to Docker runtime image: fragile, requires syncing
  Dockerfile whenever plugins are added/removed. Still fails if CWD changes.
- Centralise all plugin migrations in one directory: loses per-plugin isolation
  that ADR-028 established.

**Consequences:**
- Migrations work in any environment regardless of working directory.
- No Dockerfile changes needed when adding new plugins with migrations.
- Each plugin must have an `embed.go` exporting its `MigrationsFS`.
- `RegisteredPlugins()` now lives in `cmd/server/main.go` instead of `database`.

---

## ADR-031: Auto-Register Game Systems as Addons from Manifests

**Status:** Accepted

**Context:** Game system addon definitions were hardcoded in the
`builtinAddons` array in `addons/service.go`. Adding a new game system
required two code changes: (1) the system's manifest.json + data files,
and (2) a matching `addonDef` entry in the addons package. This coupling
prevented truly self-service system creation — you couldn't just drop a
folder or install via the package manager and have it appear in the addon UI.

**Decision:** Auto-register game systems from the systems registry:
- `systems.AddonInfos()` returns addon metadata for all discovered systems
  with status "available" (name, description, version, icon, author from manifest).
- `addons.RegisterSystemAddon()` appends to `builtinAddons` and marks the
  slug as installed in `installedAddons`.
- App wiring calls these after `systems.Init()` but before `SeedInstalledAddons()`.
- The three hardcoded system entries (dnd5e, pathfinder2e, drawsteel) are
  removed from `builtinAddons`.

**Alternatives considered:**
- Have `addons` import `systems` directly: creates package coupling. The
  wiring layer in `app/routes.go` already imports both.
- Auto-discover from database only: doesn't help with initial registration
  of new systems before they're in the DB.

**Consequences:**
- New game systems appear as addons automatically with zero code changes.
- Systems from the package manager, custom uploads, or `internal/systems/`
  all register the same way.
- The blank import for dnd5e in `main.go` is still needed for its custom
  tooltip renderer factory — pure-data systems need no import.
- Campaign settings page originally fell back to "Campaign extension." for
  system descriptions (previously had hardcoded per-system strings); the real
  description is threaded through instead as of ADR-056.

---

## ADR-032: Sidebar Navigation Overhaul — Pure Folders & Unified Items

**Status:** Accepted

**Context:** The sidebar navigation had several limitations:
1. Organizational folders were implemented as entities with `is_folder=TRUE`,
   which polluted entity search results and required filtering workarounds.
2. Addon links (Journal, NPCs) were hardcoded in `app.templ` — owners
   couldn't reorder them relative to categories or custom links.
3. No tag filtering, lazy loading, or bulk operations for large campaigns.
4. Favorites were localStorage-only (lost on device switch).

**Decision:**

**Pure folders.** New `sidebar_nodes` table for organizational
folders with zero entity records. Entities gain `parent_node_id` (FK to
sidebar_nodes) as a mutually exclusive alternative to `parent_id`. The
`is_folder` column is removed from entities. Migration 000013 handles
data migration from `is_folder` entities to `sidebar_nodes` rows.

**DB-backed favorites.** New `entity_favorites` table replaces
localStorage. Per-user, per-campaign. Toggle/list API endpoints. The
`favorites.js` widget uses API calls with in-memory cache for instant UI.

**Unified sidebar model.** `SidebarItem` type added to
`SidebarConfig` with an `Items` array. All sidebar content (dashboard,
addons, categories, sections, links) unified as items. When `Items` is
present, the template renders in owner-defined order. When absent,
falls back to legacy format. One sidebar layout editor replaced the
separate category-order and custom-links editors.

**Large campaign support.** Tag filtering via `?tags=` query
param with AND-logic SQL subquery. Lazy loading at 50 entities per page
with IntersectionObserver. Multi-select bulk move with floating action
bar. Collapsible Manage section with localStorage persistence.

**Alternatives considered:**
- Keep `is_folder` on entities with filtering: still creates entity records,
  confusing conceptually, requires ongoing filtering in every query.
- New `sidebar_folders` table with `parent_type` discriminator: adds
  complexity for parent resolution. The simpler `parent_node_id` on
  entities avoids ambiguous joins.
- Separate `sidebar_items` database table: over-engineering for what
  is effectively a JSON config. The `SidebarConfig.Items` array in the
  existing JSON column is simpler and backward-compatible.

**Consequences:**
- Folders are true organizational containers — no entity records, no search pollution.
- Owners can reorder the entire sidebar (addons, categories, links).
- Large campaigns (500+ entities) load incrementally with tag filtering.
- Favorites persist across devices.
- Dual-parent model (`parent_id` vs `parent_node_id`) requires care in
  queries and the reorder service to keep them mutually exclusive.

---

## ADR-033: Startup Health Check System

**Status:** Accepted

**Context:** Migration 000018 added `archived_at` and `join_code` columns to
campaigns, but wasn't applied in a dev environment. Repository queries already
referenced these columns, causing server errors on campaign pages. This revealed
that Chronicle had no proactive detection of schema drift, unapplied migrations,
or security misconfigurations at startup.

**Decision:** Comprehensive startup health check system in
`internal/database/healthcheck.go`. Runs after `RunMigrations()` but before
route registration. Five checks:

1. **Migration version** — Verifies DB is at expected version (currently v18).
   Detects dirty state and logs force-retry instructions.
2. **Critical columns** — Queries `information_schema.COLUMNS` for required
   table/column pairs. Catches schema drift from failed or skipped migrations.
3. **DB connectivity** — Pings with 5s timeout. Monitors connection pool
   utilization (warns at 80% capacity).
4. **Security audit** — Detects weak/default DB passwords in production, HTTP
   BaseURL (CSRF cookie vulnerability), overprivileged DB user grants (SUPER,
   FILE, PROCESS), and world-writable schema_migrations table.
5. **Pre-migration backup** — `PreMigrationBackup()` runs mysqldump with gzip
   before migrations. Auto-rotates old backups (configurable retention).
   Silently skips if mysqldump is unavailable.

Server exits with `os.Exit(1)` if any check fails. Configuration via
`HealthCheckConfig` struct in `cmd/server/main.go`.

**Alternatives Considered:**
- Runtime health endpoint: too late, server already accepts traffic with bad state.
- External monitoring (Prometheus/Grafana): doesn't prevent startup with broken schema.
- Manual `make migrate-up` before deploy: error-prone, forgotten in practice.

**Consequences:**
- Schema drift detected before first request is served.
- Database backed up before destructive migrations.
- Security baseline (strong password, HTTPS, least-privilege DB user) enforced
  on every start.
- Adds ~100ms to startup time (information_schema queries are fast).
- mysqldump dependency is optional — backup silently skipped if not installed.

---

## ADR-034: Asymmetric Corner Bleed CSS Effect System

**Status:** Accepted

**Context:** Chronicle needed a cohesive visual language for interactive elements
(buttons, navigation items) that feels distinctive and polished, consistent
across the entire UI.

**Decision:** CSS pseudo-element (`::after` for buttons, `::before` for sidebar
nav) with stacked `linear-gradient` backgrounds creating an asymmetric glow
effect. Design principles:

- **Right edge strongest** — Gradients from right side have highest opacity.
- **Bottom heavier than top** — Bottom edge thicker (8px) vs top (4px).
- **Bottom-right corner heaviest** — 50% opacity at 8px, vs top-left at 15-20%.
- **Click/active state** — All corners expand to 100% width (full wrap-around)
  with 0.2-0.3s transition.
- **`.btn-pressed` JS class** — Added on `mousedown`, removed 300ms after
  `mouseup` for a tactile linger effect (`boot.js`).

Applied consistently across six button variants (primary, ghost, secondary,
danger, warning, success) and sidebar navigation, in `static/css/input.css`.

Sidebar uses `::before` (not `::after`) to avoid conflicting with the icon-only
tooltip which uses `::after`. Glow is suppressed in icon-only mode (too narrow).

**Alternatives Considered:**
- `box-shadow`: symmetric only, can't create directional weighting.
- `border-image`: limited transition/animation support.
- SVG filters: performance overhead, harder to maintain.
- CSS `outline`: no gradient or directional control.

**Consequences:**
- Unified visual language across all interactive elements.
- Pure CSS except for the 300ms linger JavaScript (6 lines in boot.js).
- Two pseudo-elements needed per element (one for glow, tooltips need separate).
- Sidebar nav avoids `::after` conflict by using `::before` instead.

---

## ADR-035: Operator Backup as POSIX Shell Script + Make Target

**Status:** Accepted; partly reversed by ADR-036

**Context:** Chronicle 0.0.1 needed an operator-runnable backup mechanism
plus a deployment runbook. The Go codebase already had
`PreMigrationBackup` (`internal/database/healthcheck.go:305-350`) — a
boot-time safety net invoked before migrations — but no on-demand path
for operators, no media or Redis coverage, and no manifest pairing.
Worse, the in-process backup was silently disabled in production
because the runtime image didn't ship `mysqldump`.

The choice was: build the operator backup as a Go subcommand of the
`chronicle` binary, or as a shell script under `scripts/`.

**Decision:** Shell script (`scripts/backup.sh`, `scripts/restore.sh`)
invoked via Make targets (`make backup`, `make restore`,
`make backup-check`, `make backup-list`). Inside the chronicle container
via `docker compose exec` for the compose path; same script runs
standalone on bare-metal hosts.

POSIX `sh` (Alpine `/bin/sh` is `ash`); no bashisms. `set -eu`. Exit
codes: `0` success, `1` operator error, `2` precondition failure,
`3` backend tool failure. Manifest pairs DB + media + redis artifacts
with sha256 + chronicle version + migration version so `restore.sh` can
refuse mismatched sets.

**Alternatives considered:**
- Go subcommand (`chronicle backup`, `chronicle restore`): would require
  rebuilding the image to update backup logic, adds a Cobra-style CLI
  surface to maintain, and still has to shell out to `mysqldump`
  internally — net loss vs. a shell script.
- Sidecar `chronicle-backup` service in compose: extra image, extra
  cron surface, more state to keep in sync. Rejected; the existing
  chronicle container already has the credentials, the volume, and
  (after this change) `mariadb-client`.
- Host-only script: forces every operator to install `mariadb-client`
  outside the container. Same script supports this case via env
  variables, but in-container is the documented primary path.

**Consequences:**
- Operators can update backup logic without rebuilding the image.
- `make backup-check` is a cheap CI surface for verifying that env vars
  and tool availability are correct.
- The `Dockerfile` runtime stage now installs `mariadb-client` and
  `gzip`. ~+15MB; this also lets the existing `PreMigrationBackup`
  actually function in production.
- Two retention systems coexist: `BackupMaxAge` (hardcoded 7d) for the
  in-process pre-migration files, and `BACKUP_RETENTION_DAYS` (default
  7d) for the operator-script artifacts. Filename prefix
  (`chronicle_pre_migrate_*` vs `chronicle_db_*`/`chronicle_media_*`)
  cleanly partitions which rotator owns which file. Future cleanup PR
  may unify; not a 0.0.1 blocker.
- Restore is a sysadmin operation only — no admin-UI restore path,
  intentionally. Documented in `docs/deployment.md` §9.
  *(Reversed by ADR-036 below.)*
- Documentation lives in `docs/deployment.md`; `scripts/README.md`
  documents the script convention itself.

## ADR-036: Admin UI for Backup and Restore

**Status:** Accepted; partly reverses ADR-035's "restore is sysadmin-only,
no admin-UI path" consequence.

**Context:**
ADR-035 deferred a web UI for both backup and restore on the grounds
that backup is rare enough for `make backup` from the host, and
restore is destructive enough that gating it behind a shell session
adds useful friction. A user request now makes that compromise the
bottleneck: operators who deploy chronicle to a VPS or container host
don't always have direct host shell access (or the muscle memory for
`make` invocations under their orchestrator), and "log in to the host
to recover from a backup" turns recovery from "click a button" into
"find the runbook, get an SSH key, hope BACKUP_DIR is mounted where I
think." For users running their own host, the UI is the only realistic
path.

**Decision:**
Two new admin-only plugins:

- `internal/plugins/backup` — `/admin/backup` page lists artifacts in
  `BACKUP_DIR` and exposes "Run backup now" + "Download artifact"
  buttons. Shells out to `scripts/backup.sh` synchronously under a
  20-minute timeout.
- `internal/plugins/restore` — `/admin/restore` page lists parsed
  manifests, with a per-row form requiring the operator to type the
  literal word `RESTORE` into a text field before the request is
  accepted. Shells out to `scripts/restore.sh --manifest <path>
  --yes --force` under a 30-minute timeout.

Security guarantees on top of the existing `RequireSiteAdmin` and
`CSRF` middleware:

- **Single-flight lock**: in-process mutex serializes both backup and
  restore against themselves. Concurrent requests get HTTP 409 with a
  clear message rather than spawning a second mysqldump or restore.
- **Rate limit**: per-IP sliding window — backup `2/hour`, downloads
  `20/hour`, restore `1/hour`. Bounds attack surface even when
  CSRF-protected.
- **Process group kills**: every shell-out runs with `Setpgid: true`;
  cancel sends `SIGKILL` to the negative PID so any descendants
  (mysqldump, tar, gzip, mariadb) die together.
- **Output cap**: stdout and stderr go through 64 KB ring buffers so
  a runaway script can't OOM the chronicle process.
- **Path safety**: every filename parameter is validated against
  `BACKUP_DIR` with both basename and prefix checks. Restore
  additionally requires the file to match `chronicle_manifest_*.txt`.
- **Typed-string confirmation**: restore requires `confirm=RESTORE`
  in the request body. Mirrors the shell script's interactive prompt
  so muscle memory transfers between the two surfaces.
- **No silent coalescing**: if a backup or restore is in flight,
  concurrent requests get 409, never "you joined the running one".

**Consequences:**
- Admins can recover without shell access. Big UX win for anyone
  running chronicle as a managed service.
- Restore from the UI is now possible — `docs/deployment.md` §9 must
  document this as the recommended path for VPS deployments while
  keeping the `make restore` flow as the in-shell escape hatch.
- The two plugins ship as independent PRs (backup #257, restore here).
  They duplicate small helpers (`capBuf`, basename validation) — once
  both land we can extract a shared internal package without churning
  the public surface.
- The 1/hour rate limit on restore is deliberately tighter than
  backup. Restore is at most an emergency operation and even one
  call/hour is generous.
- The "run backup before restore" advice in the UI's red banner is
  human-only safety; the system does NOT auto-snapshot before
  restoring. A future enhancement may add an opt-in pre-restore
  backup, but for now the operator owns that step (and the existing
  in-process pre-migration backup gives some protection too).

## ADR-037: Pre-migration backup symmetry with operator backups

**Status:** Accepted; refines ADR-035 (operator backup) and ADR-036
(admin UI for backup and restore).

**Context:**
The original `PreMigrationBackup` (added in the ADR-035 era) captured
only the database — a single `chronicle_pre_migrate_<TS>.sql.gz` per
boot. Three gaps surfaced once the operator backup pipeline matured:

1. **No media or Redis snapshot.** A migration that changes how media
   IDs are encoded would leave the on-disk media tree out-of-sync
   with any restored DB. Same for Redis (sessions only, but
   recoverable).
2. **Fail-open on tool absence.** If `mysqldump` was missing from the
   image, the function logged a warning and returned. Migrations
   proceeded with no rollback. In production that's a silent
   data-loss risk hidden behind a green deploy.
3. **No version stamping.** The artifact filename was just a
   timestamp; the operator had to remember which schema version the
   DB was at when it was taken. Operator backups already embed
   `migration_version=<N>` in their manifest; pre-migration didn't.

**Decision:**
Extend `PreMigrationBackup` so its output is interchangeable with
`scripts/backup.sh` output:

- Same artifact prefixes
  (`chronicle_pre_migrate_db_*.sql.gz`,
  `chronicle_pre_migrate_media_*.tar.gz`,
  `chronicle_pre_migrate_redis_*.rdb`).
- Same manifest format (`chronicle_pre_migrate_manifest_*.txt`) with
  `chronicle_manifest_version=1`, `chronicle_version=`,
  `migration_version=`, plus per-artifact sha256 + size.
- One distinguishing line: `chronicle_pre_migrate=1` so restore
  tooling can label boot-time bundles separately from
  operator-triggered ones.

Add `BACKUP_REQUIRED=1` env var: when set, any artifact failure
aborts startup before migrations apply. Default remains fail-open
for backwards compatibility with development setups that lack
`mariadb-client`.

Three security/correctness defenses:

- **Atomic writes.** Each artifact written to `<file>.partial` and
  renamed only after sha256 + size verification. Half-written files
  never persist.
- **0600 file mode** on every artifact and the manifest. The dump
  contains all data; loose permissions on a multi-user host would
  leak it.
- **Zero-byte rejection.** Any artifact that ends up zero bytes is
  treated as a capture failure (covers silent `mysqldump` exit-zero
  on empty DB, `redis-cli` writing nothing on no-permission, etc.).

**Consequences:**

- Pre-migration snapshots become **first-class restorable artifacts**.
  `scripts/restore.sh --manifest chronicle_pre_migrate_manifest_<TS>.txt`
  works the same as it does for operator backups; the admin restore
  UI surfaces them in the same list.
- Production deployments can opt into fail-closed via
  `BACKUP_REQUIRED=1` for a real "no rollback story = no migration"
  guarantee. Existing deployments are unaffected (default still
  fail-open).
- The retention sweep was extended to glob the four new artifact
  families plus a backwards-compat pattern for legacy
  `chronicle_pre_migrate_<TS>.sql.gz` files (no `_db_` infix). 7-day
  retention applies to all five; legacy files time out and disappear
  on their own as the new format takes over.
- `redis-cli` is now a soft dependency: present → Redis snapshot
  included; absent → skipped with a debug log. Chronicle's only
  Redis state is sessions, so a missing snapshot only means "users
  get logged out on rollback" — not data loss. Production images
  should still include `redis-tools` for the safety net.
- Documentation: `docs/deployment.md` §5 gains `BACKUP_REQUIRED`,
  `BACKUP_SCRIPT_PATH`, `RESTORE_SCRIPT_PATH`, `CHRONICLE_VERSION`
  entries; §7 (Rollback / Scenario A) is rewritten to use
  `scripts/restore.sh --manifest` against pre-migration manifests.

## ADR-038: Widget bindings — polymorphic, FK-free association table

**Status:** Accepted

**Context.** The widget-binding framework needs to map a *host*
(entity / entity-type / dashboard) to a *data instance* (a calendar / map /
timeline …) per *widget type*. `entities.map_id` is the existing hardcoded
special case (one entity → one map). We need the generic table.

**Decision.** `widget_bindings(id, campaign_id, host_type, host_id,
widget_type, instance_id, …)` is **polymorphic and FK-free** on both `host_id`
and `instance_id`. `host_type`/`widget_type` are an immutable, append-only
namespace validated **in app code, not a DB enum**.

**Why not the integrity-preserving alternatives** (the ones a DBA would reach
for first):
- *Exclusive-arc / nullable-FK-per-type* (`calendar_id`, `map_id`, … columns,
  each FK'd, with a CHECK that exactly one is set) and *join-table-per-type*
  (`entity_calendar_bindings`, `entity_map_bindings`, …) both **buy real
  referential integrity** — but at the cost of **per-widget-type schema churn**,
  which is exactly the hardcoding this framework exists to abolish (the
  "dynamic, not hardcoded" requirement). A new widget type would mean a
  migration every time.
- More decisively, a hard FK is **impossible here**: `instance_id` references a
  *different* table depending on `widget_type` (calendars **or** maps **or**
  timelines), and those are **plugin-owned** tables. Per the migration-ordering
  rule (`.ai/conventions.md` §Migration Safety — core runs before plugins, and
  a binding table referencing plugin tables would crash a fresh DB), the FK we'd
  want can't be collected anyway.

**Consequence / mitigation (this is load-bearing, not optional).** FK-free
means the *application* is the only integrity backstop (MariaDB has no RLS).
Integrity is enforced as an **AND** of three mechanisms — not "or":
1. **Per-plugin delete hook** — `Service.OnInstanceDeleted` (owning plugins
   call it when an instance is deleted).
2. **Always-on render-time orphan guard** — `Resolve` validates every candidate
   via `WidgetType.InstanceExists` (which also enforces campaign scope) and
   skips/sweeps dead bindings, falling through to the default.
3. **Periodic campaign integrity sweep** — `Service.Sweep`.
Campaign scope is pushed down to the repository signature (an unscoped read is
unrepresentable) and checked on **both** `host_id` and the resolved
`instance_id`. The table lives in the `widgetbindings` plugin; being FK-free,
its migration order vs calendar/maps/timeline is irrelevant.

**References.** `reports/chronicle/2026-06-07-widget-binding-framework-prep-audit.md`
(§3), `reports/chronicle/2026-06-07-widget-binding-precedent-research.md`
(polymorphic-association / multi-tenant-scoping precedent; Foundry #9818
cascade-direction bug → directional cascade test).

---

## ADR-039: Player Character Claiming — Owner-Toggleable Addon + Per-Type Claimable Flag

**Status:** Accepted

**Context.** Chronicle needs bidirectional player-character binding for both
Foundry sync and internal campaign management. A GM must know which player owns
which character; a player must be able to claim an unclaimed character. The
feature must be optional (campaigns can opt-in) and extensible (not every
character-shaped type needs claiming).

**Decision.** Three-part design:

1. **Owner-toggleable addon** (`player-character-claiming`): GMs opt-in per
   campaign via the Addons panel. Creating a "Player Character" sub-type (a
   dedicated entity type with `preset_category == "player_character"`) is
   gated on the addon being enabled. UI surfaces (claim button, owner roster,
   claimable toggle) are all hidden when the addon is off.

2. **Per-type claimable flag** (`entity_types.claimable BOOLEAN NULL`): allows
   the Owner fine-grained control. When set (TRUE/FALSE), the Owner's choice
   is authoritative. When NULL (default/unset), the legacy heuristic applies
   (preset_category "character" or slug `*-character`). This allows existing
   campaigns to keep claiming on their "Character" type without manual
   re-configuration.

3. **"Player Character" sub-type + legacy fallback**: New campaigns can use
   the dedicated "Player Character" sub-type when the addon is on (explicit,
   separate from characters that might be NPCs). Existing campaigns keep
   claiming on their existing "Character" type (heuristic-based). Both paths
   are supported; neither overwrites the other.

**Claimable-by-default when addon is on:** When an Owner enables the addon
and creates a new type, the claimable flag defaults to true. This reflects
the mental model: "I turned on the feature" → "I want my character types to
be claimable." If the Owner wants a character-shaped type that is *not*
claimable (e.g., "NPC", "Companion"), they can toggle the flag to false.

**Why this design vs alternatives:**

- **Addon on/off (vs always-on):** Existing campaigns default off. Zero
  surprise. Opt-in ceremonies reduce feature cruft for campaigns that don't
  use the feature.

- **Per-type flag (vs all-or-nothing):** Not all character-shaped entities
  should be claimable. An NPC generator, a companion template, or an "Open
  Seat" character all have the same *shape* as a PC but aren't owned by
  players. Per-type control is finer-grained and avoids category-wide toggles.

- **Dedicated PC sub-type (vs hardcoding "Character"):** Separates the concerns.
  "Player Character" is a campaign-wide opt-in with the addon; "Character" is
  the general-purpose entity type, which may or may not be claimable. Foundry
  sync looks for the PC sub-type specifically and auto-claims player-owned
  actors into it.

- **Heuristic fallback (vs migration-time decision):** Existing campaigns don't
  need a migration. The `claimable` column defaults NULL, and the service falls
  back to the existing heuristic (preset_category "character"). Campaigns can
  opt-in to explicit control by setting claimable on their types. Zero
  disruption.

**Audit trail:** Distinct audit actions (`entity.claimed` and
`entity.owner_changed`) make claiming and reassignment visible in the activity
log. The claiming player and the character's real name are recorded, not
opaque IDs.

**Foundry sync:** When the Foundry sync module detects the addon is on, it
maps player-owned PC actors (by actor type + GM ownership) to the PC sub-type
and auto-claims them, without manual operator configuration
(Chronicle-Foundry-Module `scripts/actor-sync.mjs`).

**References.** `internal/plugins/entities/.ai.md` §"Player Character Claiming",
`entities/{service.go, handler.go}` (isPlayerCharacterType, isClaimableType,
ClaimEntity, AssignOwner), `entities/{claim_banner.templ, claim_overview_test.go}`,
migration 000029.

## ADR-040: Dynamic-surface frame — a system-agnostic Widget, not a hardcoded sheet

**Status:** Accepted

**Context.** The operator wants a dynamic UI: a
mini surface that promotes into a full-screen sheet with expandable boxes, action
overlays, and drill-downs — applied first to the character sheet, later the rulebook.

**Decision.** Build it as ONE reusable, **system-agnostic frame** in the Widget tier
(`Chronicle.surface`, `static/js/widgets/dynamic_surface.js`) — a motion-preset library,
an overlay stack, an expand/collapse box, a memoized data provider, a mini→full
`launch`, and a schema-driven `mount` — rather than a bespoke renderer per sheet. **The
frame owns motion + structure; a System supplies box BODIES** via `registerBox(name, fn)`.
A System never writes animation code; it names which preset fits each card.

**Why.** Chronicle is genre-agnostic; the same paradigm must serve any game system and
the rulebook. Separating the frame (Chronicle) from the content (System/plugin) keeps
"Chronicle owns the template; the system fills it." Built on existing motion tokens
(`--ease/-dur/-elev-*`) + a new `--surface-*` contract, so it stays theme-aware; all
presets collapse to a fade under `prefers-reduced-motion`. No new tables — surfaces ride
a declarative schema; per-user view-state rides localStorage.

**References.** `static/js/widgets/dynamic_surface.js` + `.ai.md`; the admin surface demo
(`/admin/design-lab`); Cordinator `plans/2026-06-21-dynamic-widget-ui-framework-design.md`.

## ADR-041: `character_surface` as a layout BLOCK + the default for player-character types

**Status:** Accepted

**Context.** Player characters should open the dynamic
"big widget" sheet by default, yet stay editable in the existing layout customizer.

**Decision.** Register the surface as a normal entity-page **layout block**
(`character_surface`, `Contexts:["template"]`, `Singleton`) whose renderer emits a
`data-widget="dynamic-surface"` container with the entity's data **seeded inline**. Make
`CharacterLayout()` (the block + permissions) the default layout for
`isPlayerCharacterType` types in `CreateEntityType`, instead of `DefaultLayout()`.

**Why.** Because it's a registry-driven block, it appears in the layout-editor palette and
owners compose/rearrange it like any block — no separate "sheet editor." The default
applies only to NEWLY created PC types (we never rewrite existing customized layouts).
**Security:** the description box mounts the same role-aware `editor` widget the standard
`entry` block uses (so GM-only secrets aren't leaked) rather than inlining `EntryHTML`.

**References.** `entities/{character_surface.go, character_surface_block.templ,
block_registry_core.go, model.go:CharacterLayout, service.go}`,
`static/js/widgets/character_surface.js`; `entities/.ai.md` §`character_surface`.

## ADR-042: Cross-plugin section injection — `NPCSectionProvider`

**Status:** Accepted

**Context.** The unified Characters page (in the core
`entities` plugin) must render an NPCs/Monsters section owned by the `npcs` addon, without
the core plugin importing the addon (rule 8) or duplicating NPC logic.

**Decision.** `entities` defines an `NPCSectionProvider` interface returning a
`templ.Component`; `npcs.Handler.NPCSection` **structurally** satisfies it and is injected
via `entityHandler.SetNPCSectionProvider(npcHandler)` at app wiring. The npcs plugin
renders its own section (featured tag-row + revealed list + reveal toggle, reusing
`NPCCardComponent`); entities just slots the component in when the `npcs` addon is on.

**Why.** Keeps domain ownership where it belongs (npcs owns NPC rendering), preserves the
dependency direction (npcs→entities, never the reverse — npcs needs no import of entities
since the interface is satisfied implicitly), and generalizes: any addon can contribute a
section to a core page this way. The standalone `/npcs` gallery page redirected into this.

**References.** `entities/handler.go` (NPCSectionProvider, Characters), `npcs/handler.go`
(NPCSection), `npcs/npc_section.templ`, `app/routes.go` (SetNPCSectionProvider).

---

## ADR-043: Extension Settings / Onboarding framework (`SetupProvider`)

**Status:** Accepted

**Context.** Enabling an addon (or a game system, which auto-registers as one per ADR-031)
fired SILENT lifecycle hooks (`ApplySystemPresets`, `ApplyAddonEnableEffects`). That hid
real decisions inside boot automation and produced the duplicate-player-character-category
artifact (see ADR-044). The owner wanted each extension to own a visible settings/onboarding
page — renderable as an integrated overlay — driven by a reusable framework, with a nudge on
enable.

**Decision.** A Go `SetupProvider` interface + slug-keyed registry lives in the `addons`
plugin (it already owns the toggle, the per-campaign list, and `config_json` persistence).
A provider supplies `RunChecks` (health/QOL findings with a severity), `Questions`
(onboarding inputs), and `Apply` (idempotent). A generic handler + three Templ components
(`extension_settings_page` / `_overlay` / `_fragment`) render ANY provider as a full page or
a modal overlay — so every extension gets a consistent settings surface for free. Concrete
providers live in the **app layer** (like `PresetApplier`) and are wired via
`addonService.RegisterSetupProvider(...)` in `app/routes.go`, so `addons` never imports
`entities`/`systems`. Per-campaign setup state (`{completed, dismissed, answers}`) persists
under **`campaign_addons.config_json["setup"]`** — no new table or migration. The Extensions
hub card carries a `NeedsSetup` flag (computed per-campaign in `addonListerAdapter`) that
shows a "Setup" badge + an "Open setup" button (`hx-get` the overlay into a page-level modal
container). On enable, the toggle handler co-emits a `chronicle:notify` toast alongside the
existing `extensions-hub-refresh` HX-Trigger.

**Alternatives considered.** (a) A declarative manifest-driven schema for checks — rejected
for now because the first provider's checks must inspect live campaign data (Go logic);
manifest-defined QOL notes can come later for external packs. (b) A new top-level settings
nav — rejected; the Extensions hub (ADR-029) is the established surface. (c) A new
`extension_setup` table — rejected; `config_json` already exists with `UpdateCampaignConfig`.

**Consequences.** Enabling stays safe (the idempotent `EnsurePlayerCharacterType` still runs)
while destructive/ambiguous choices move into the owner-driven wizard. New providers (e.g.
calendar, maps) register with zero template changes.

**References.** `addons/setup_provider.go` (interface + registry + state),
`addons/setup_handler.go` + `addons/routes.go` (3 owner-gated routes),
`addons/extension_settings_*.templ`, `campaigns/handler.go` (`PluginHubAddon.NeedsSetup/HasSetup`),
`app/routes.go` (`addonListerAdapter`, `RegisterSetupProvider`), `app/setup_pc.go`.

---

## ADR-044: PC duplicate reconciliation moves from boot migration → owner-triggered Apply

**Status:** Accepted; amended after a production incident (its original draft
proposed deleting migration `000030`, which crash-looped production and was
reverted — see the migration-safety rule below); extended by ADR-045 and
ADR-050.

**Context.** A campaign could end up with BOTH a generic "Player Characters" type (holding the
claimed character entities) and a game system's own character type (e.g. Draw Steel's empty
"Heroes") — an enable-ordering artifact the non-destructive boot path nests but never merges.
The one-time, guarded boot migration `000030_consolidate_player_character_duplicate` auto-merges
the unambiguous case (exactly one of each) on deploy.

**Decision.** KEEP migration `000030` permanently (it is applied in production DBs and is
idempotent/guarded). ADDITIONALLY provide an owner-triggered, single-campaign service method
`entities.MergeDuplicatePlayerCharacterType` (+ repo `MoveEntitiesAndDeleteType`, one
transaction), surfaced as a check on the player-character extension settings page (ADR-043),
for cases the one-time migration does NOT cover: ambiguous campaigns (more than one of either
category → a human-readable `apperror`) and duplicates
that arise AFTER the migration ran. It classifies the unambiguous (generic → system) pair by
`preset_category`/`slug`/`is_default` only (no system names), moves the generic's entities onto
the system type (claims follow via `entities(id)`), and deletes the emptied generic. "Heroes
wins": the system's own type survives. The owner additionally chooses the system name vs a
custom name in the same wizard. The two mechanisms are complementary, not exclusive.

**Migration safety (the lesson — a real incident).** The original draft proposed DELETING
`000030`. That is UNSAFE and crash-looped production: golang-migrate's `file://` source
(`internal/database/migrate.go`, `m.Up()`) must contain a migration file for EVERY version up
to the DB's current recorded version. A prod DB already at `version=30` with no `000030` file
on disk fails with `no migration found for version 30: read down for version 30: file does not
exist` — an unrecoverable boot loop (the runner only auto-recovers `ErrDirty`, not a
missing-version source; the health-floor check never even runs). **RULE: never delete or
renumber a migration that any live database has applied — keep it forever, even when later
superseded.** (Matches CLAUDE.md "Never edit an applied migration" — extend it to "never
delete" one.)

**Incident-response lesson (the fix that didn't land).** The `000030` restore was committed to
a feature branch after the PR that needed it had already been merged and closed at the
pre-restore commit — so the fix sat on the branch, never reaching `main`, and editing the
already-merged PR's body changed nothing in the tree. `main` stayed broken until a fresh
hotfix PR carried the restore in. **RULES:** (1) a post-merge fix needs a NEW PR — editing a
merged PR is inert; (2) after shipping any incident fix, VERIFY it is actually on `main`
(`git ls-tree origin/main -- db/migrations/` or check the merged SHA), don't assume the branch
state equals `main`; (3) prefer **squash-merge** so a "deleted-then-restored within the branch"
sequence can't merge at an intermediate broken commit.

**Consequences.** Existing prod duplicates are healed automatically by `000030` on deploy
(unambiguous case) AND can be reconciled by the owner from the settings page (any case, full
visibility). No migration is ever removed. The owner-merge is idempotent (once the generic is
gone, a re-run is a no-op success).

**References.** `entities/service.go` (`MergeDuplicatePlayerCharacterType`,
`PlayerCharacterSetupSnapshot`), `entities/repository.go` (`MoveEntitiesAndDeleteType`),
`app/setup_pc.go` (the provider), ADR-043, ADR-039 (PC claiming).

---

## ADR-045: Migration robustness — fail-safe boot, append-only guards, schema-only policy

**Status:** Accepted; extends ADR-044 (the durable fix for the `000030` incident).

**Context.** Deleting an applied migration crash-looped production. Root cause was THREE
things: (1) golang-migrate's `Up()` hard-errors when the DB version exceeds the on-disk
source's highest version — this fires on a deleted migration AND on a normal image rollback;
(2) `restart: unless-stopped` turns any fatal boot into a ~1/sec loop; (3) the pre-migration
backup ran unconditionally before every boot, so each loop iteration wrote a full dump (the
"6 backups/min" symptom). Audit also found a live `ExpectedMigrationVersion` drift (29 vs the
real max 30) and 15 historical migrations using non-idempotent `ADD COLUMN`.

**Decision — three layers.**

1. **Runtime (boot fails safe, never crash-loops).** `database.MigrateWithBackup`
   (`internal/database/migrate_state.go`) replaces the unconditional backup-then-migrate
   sequence. It reads the DB version + highest on-disk migration ONCE, then:
   - **DB ahead of the build** → log an actionable warning and **start anyway** (skip `Up()`).
     Migrations are additive, so an older binary runs fine on a newer schema; the startup
     health checks backstop a destructive rollback. Fixes the deletion case AND ordinary
     image rollbacks.
   - **up to date** → skip BOTH backup and `Up()` (ends the backup-on-every-restart storm).
   - **pending** → back up, then migrate.
   A **dirty** database now FAILS FAST with restore guidance (the old `Force(v-1)` auto-retry
   looped forever on non-idempotent migrations). `fatalBoot` (`cmd/server/main.go`) sleeps
   `BOOT_FAIL_BACKOFF` (default 45s) before exit so unrecoverable errors retry ~1/min.

2. **CI guards (prevent the mistake).** `tools/check-migration-immutability.sh` (CI step)
   fails any PR that deletes or edits a migration already on the base branch.
   `internal/database/migrate_test.go` gains: version-pin (`ExpectedCoreMigrationVersion ==
   max(core migration)`), idempotent-DDL lint (grandfathering the immutable historical files),
   gapless numbering, and plugin up/down-pair coverage.

3. **Visibility (admins see + act) — the unified Database page.** `/admin/database` is one
   tabbed control surface (Alpine `x-data` tabs, the `storage.templ` pattern) so an operator
   reasons about — and recovers from — the database from a page, not from crash logs:
   - **Migrations** — core schema version + dirty flag + pending count + a DB-ahead/downgrade
     banner (the runtime A3 state, made visible), plus the existing per-plugin grid + "Apply
     Pending" + history.
   - **Health** — the SAME `RunStartupHealthChecks` the boot path runs, rendered live with
     pass/warn/fail pills. The runner was split: `database.RunHealthChecks` returns a structured
     `HealthCheckResult` with no logging/exit, and `RunStartupHealthChecks` wraps it for boot.
     The check config was extracted to `app.StartupHealthCheckConfig(cfg)` so **boot and the
     admin tab share one definition and can never disagree.** `GET /admin/database/status`
     exposes core+plugin status as JSON for external monitoring.
   - **Backups** — the existing `backup`/`restore` plugins surfaced (no new engine): artifacts
     with an **Auto** (pre-migration) vs **Manual** badge, restorable snapshots with their
     Chronicle/schema versions, last-auto-backup recency, and create/download/restore actions.
   - **Schema** — the D3 diagram, lazily mounted on first tab activation so it reads a real
     container width instead of the hidden-tab zero-width fallback.

   **Cross-plugin wiring stays decoupled** (the established `DatabaseExplorer` / ADR-042
   `NPCSectionProvider` pattern): `admin` defines `HealthChecker` / `BackupLister` interfaces
   (`database_health.go`); the app layer injects adapters (`internal/app/admin_db_adapters.go`)
   over the boot health config and the backup/restore services, so `admin` imports neither.

**Policy — migrations are APPEND-ONLY and SCHEMA-ONLY.** Never delete, edit, or renumber a
migration that any live DB may have applied (the immutability guard enforces this). New DDL
must be idempotent (`IF [NOT] EXISTS`). One-time DATA corrections do NOT go in migrations —
use an idempotent reconciler (an `EnsureX`/`MergeX` service method run from a boot backfill,
an addon-enable hook, or an owner-triggered `SetupProvider`), as in `app/setup_pc.go` +
`entities.MergeDuplicatePlayerCharacterType`. Reconcilers are idempotent, handle cases that
arise later, and surface ambiguity to a human — none of which a one-shot data migration can do.

**Consequences.** Upgrades and rollbacks "just work" or fail with a clear message; the incident
class (delete/edit/renumber/gap/non-idempotent/version-drift) is blocked at PR time; admins
manage migration state from a page. The historical `000030` stays (it's applied; the
immutability guard enforces it can't be removed again).

**References.** `internal/database/migrate_state.go`, `internal/database/migrate.go` (dirty
fail-fast), `internal/database/healthcheck.go` (`RunHealthChecks` split), `cmd/server/main.go`
(`fatalBoot`), `internal/database/migrate_test.go` (guards), `tools/check-migration-immutability.sh`,
`internal/app/{health_config,admin_db_adapters}.go` (shared config + tab adapters),
`internal/plugins/admin/{database_service,database_health,handler,database.templ}`,
ADR-044, ADR-028/030 (plugin migrations), ADR-037 (pre-migration backup), ADR-042 (cross-plugin
injection pattern).


---

## ADR-047: World-state broadcasts are audience-SPLIT, not audience-filtered

**Status:** Moot. The code it governed was deleted in #595; the calendar is
being rebuilt, #741.

It fixed calendar world-state events (weather, celestial) that were silently
dropped before reaching any WebSocket client, by splitting each broadcast
into a player-safe payload and a separate DM-only payload instead of
omitting GM-only detail from one shared payload.

Full text: https://github.com/keyxmakerx/Chronicle/blob/dfc73c78/.ai/decisions.md

---

## ADR-046: Calendar events get first-class RSVPs, distinct from session attendance

**Status:** Moot. The code it governed was deleted in #595; the calendar is
being rebuilt, #741.

It gave calendar events (festivals, downtime windows, one-off scenes) their
own RSVP storage and service, separate from session attendance, gated by the
calendar's existing visibility check and kept out of campaign/AI export.

Full text: https://github.com/keyxmakerx/Chronicle/blob/dfc73c78/.ai/decisions.md

---

## ADR-048: calendar-v4 — the Block is the calendar, and its honesty states are load-bearing

**Status:** Moot. The code it governed was deleted in #595; the calendar is
being rebuilt, #741.

It replaced the month-grid calendar with a four-zone "Block" component and
ruled a set of "honesty states" — cases where the UI must show absence or
uncertainty rather than guess at missing data.

Sections 17 and 18 (the availability overlay's role vocabulary and
per-member timezone display) are still live; that guidance now lives in
`internal/plugins/sessions/.ai.md` under "Role and Zone Display Rules".

Full text: https://github.com/keyxmakerx/Chronicle/blob/dfc73c78/.ai/decisions.md

---

## ADR-049: "no authenticated user" and "trusted system caller" are two states, not one empty string

**Status:** Accepted

### Context

The calendar and timeline visibility filters short-circuited on
`permissions.CanSeeDmOnly(role) || userID == ""` (calendar) and
`!CanSeeDmOnly(role) && userID != ""` (timeline). The empty user id was
documented in both places as "the system context" — a trusted in-process
caller with no request behind it.

**An anonymous HTTP request carries exactly that value.** `auth.GetUserID(c)`
returns `""` when there is no session, and on a PUBLIC campaign
`AllowPublicCampaignAccess` + `RequireViewAccess` let that request reach the
service. So the most privileged branch in the filter was the branch logged-out
internet traffic took: `dm_only` calendars, `dm_only` timelines and
per-user-restricted events and event links were served to a visitor who never
logged in — content a logged-in Player on the same campaign is correctly
denied.

This was not a missing check at a call site. It was **one representation
standing for two different states**, so every call site was correct and the
system was still wrong.

### Decision

**The two states get two representations, and the trusted one is unforgeable
from request data.**

`internal/permissions/viewer.go` adds `Viewer`, with an **unexported** `system`
bool and exactly two constructors:

- `RequestViewer(role, userID)` — anything that came in over HTTP. An empty
  `userID` means ANONYMOUS: no user. It cannot produce a system viewer.
- `SystemViewer(role)` — a trusted in-process caller that has no request
  identity, stated at the call site.

Every visibility filter asks `Viewer.SkipsPerUserRules()` (`system ||
CanSeeDmOnly(role)`) instead of testing the user id itself. **An anonymous
viewer is neither**, so it falls to the least-privileged path by construction
rather than by each call site remembering.

An empty user id is an ABSENT per-user layer — never a sentinel, never a
lookup key, never substituted with a synthesised identity (no `"anonymous"`
user, no per-IP key).

### Consequences

- **Two trusted callers now say so.** `timeline_widget_type.go`'s
  create-or-pick picker (Scribe-gated at the route) and the campaign timeline
  export adapter pass `permissions.SystemViewer`. **Their shipped behaviour is
  unchanged** — the picker still lists allow-list-restricted timelines — which
  is the point: the trust was real, only its representation was shared with
  anonymous traffic.
- **Tests that had pinned the bug as intended were inverted, not deleted**,
  and each kept a row asserting that the SYSTEM path still bypasses, so the
  pair proves the distinction.
- `TimelineService.ListTimelines` / `ListTimelinesForCalendar` /
  `ListTimelineEvents` take a `permissions.Viewer` instead of `(role, userID)`;
  the calendar's own filters are package-private and take one too, with the
  exported service methods building a `RequestViewer` at their boundary.

### References

Pinned by `internal/plugins/timeline/anonymous_visibility_test.go` (the
calendar's twin was deleted with the calendar in #595).

---

## ADR-050: An immutable plugin migration is repaired by a reconciler, and a half-applied one resumes instead of replaying

**Status:** Accepted; extends ADR-044 / ADR-045 (migration robustness, the
`000030` incident) and ADR-028/030 (plugin migrations).

### Context

Two failures in the same runner, both terminal, both invisible to CI.

1. **`foundry_vtt` migration 001 crashed on every brand-new database.** It is a
   consolidation migration — `RENAME TABLE foundry_module_campaign_tokens TO
   foundry_vtt_campaign_tokens` — and the plugin that created the source table
   had already been deleted. A fresh install hit `Error 1146`, the plugin was
   marked DEGRADED, and it could never self-heal, because
   `runSinglePluginMigrations` returns on the first failed migration: no later
   migration for that plugin is reachable, so a fresh-DB-safe `002` would never
   run. `PreMigrationCheck` does not cover it — it refuses only when
   `foundry_module_versions` exists *and has rows*, and on a fresh database the
   table does not exist at all.
2. **A plugin migration that failed on its second statement was unrecoverable.**
   `execPluginMigration` splits on semicolons, runs statement by statement on a
   plain `*sql.DB`, and writes the `plugin_schema_versions` row only after the
   LAST statement succeeds. A mid-migration failure therefore leaves the earlier
   statements' effects in the database and *no record that anything happened*.
   The next boot replays from statement one and dies on "duplicate column name",
   because most plugin ALTERs are not idempotent — so the operator sees an
   artefact of the retry rather than the real cause, and fixing the real cause
   cannot help.

Both were invisible for the same reason: **nothing in CI ever migrated an empty
database.** `tools/restore-drill.sh` loads a dump of an already-migrated one and
every integration test assumes `make migrate-up` has run.

### Decision

**1. An immutable migration that cannot run is repaired by a Go-side reconciler
plus a new append-only migration — never by editing the old one.**
`foundry_vtt.ReconcileConsolidationState` records 001 as applied on any database
where its RENAME has no source table; new migration `002_ensure_campaign_tokens`
states the post-consolidation shape in idempotent DDL. 001 is untouched, so
`tools/check-migration-immutability.sh` and the `migrate_test.go:402` grandfather
stand exactly as they were, and a database that still HAS the predecessor table
is left alone — 001 runs there for real and carries its live token rows across.
Fresh install, completed upgrade, and an upgrade that died between 001's two
statements all converge on one schema.

This is the existing house rule ("one-time data fixes go in a reconciler, never a
migration") extended to its schema-bootstrap twin, and it deliberately rejects the
two alternatives: editing 001 is forbidden outright, and relaxing
`runSinglePluginMigrations` so an unapplied earlier version is not a hard stop
would change failure semantics for all nine registered plugins in order to fix
one.

**2. A plugin migration gets a pre-flight applicability check and partial-progress
recording. It does NOT get a transaction, and the errors say so.** MariaDB has no
transactional DDL: every CREATE / ALTER / DROP / RENAME commits implicitly and
cannot be rolled back. `internal/database/plugin_migration_safety.go` therefore
buys two specific things and claims nothing more:

- **Pre-flight.** Before the first statement runs, every statement is validated
  against the schema catalogue *as it will stand at that point in the migration*
  (the simulation moves forward, so the ordinary CREATE-then-ALTER shape is not
  falsely refused). If any statement cannot possibly succeed, the migration
  aborts having executed NOTHING — converting "half-applied and unrecoverable"
  into "nothing applied, actionable error", which is the closest thing to
  atomicity available here. Table granularity: it catches the failures that
  produce unrecoverable states, not every possible SQL error.
- **Partial-progress recording** in a new runtime table
  `plugin_migration_progress`. When a statement fails anyway, the number that DID
  apply is recorded first, so the next boot resumes after them. Keyed by a sha256
  of the migration text and honoured **only** on a byte-identical match —
  migrations are immutable so this should never diverge, but a resume that skipped
  the wrong statements would be far worse than the crash-loop it replaces, so it
  is checked rather than assumed.

**3. Every uncertain answer degrades to the pre-existing behaviour.** The
pre-flight **fails open** when `information_schema` cannot be read, and an
unmatched or missing progress row replays from zero. Refusing every plugin
migration over a metadata hiccup would be worse than the bug being guarded, and
a false abort of a real migration is the one outcome worse than the original
defect.

**4. The guard is a fresh-DB replay, and a SKIP is a failure.** `make test-freshdb`
/ `cmd/server/freshdb_migration_test.go` replay the real bootstrap against
genuinely empty MariaDB schemas — one from zero, one from the pre-consolidation
shape — wired into CI as its own `Fresh-DB Migration Replay` job. The job greps
for PASS on each named test, so a test that merely *skips* (the way this class
hid for as long as it did) fails the job. The three DB-backed
`TestPluginMigration_*` recovery regressions run in the same job and are named in
the same assertion. `TestFreshDatabase_EveryPluginSchemaApplies` continuing to
pass is what rules out the pre-flight falsely refusing a real migration.

### Consequences

- A plugin whose old migration is unrunnable now has a sanctioned repair that
  does not touch the immutability guard. The cost is that the canonical schema is
  stated in two places — the original migration and the idempotent `002` — which
  is the price of append-only.
- Boot-time recovery semantics changed for **all** plugins, not just
  `foundry_vtt`: a failing migration may now abort earlier (pre-flight) or resume
  later (progress). Both directions were chosen to be strictly safer than replay,
  and both fall back to replay when unsure.
- `plugin_migration_progress` is a runtime table created by the runner itself,
  not by a migration — it must exist before any plugin migration runs, so it
  cannot be one.
- The non-idempotent-DDL CI ratchet the original booking imagined was **not**
  built. All nine existing offenders are immutable, so a ratchet would have
  needed a grandfather allowlist and a house-law amendment; the pre-flight
  addresses the harm those offenders cause without requiring either.

### References

- `internal/database/plugin_migration_safety.go` (package doc states the
  non-transaction claim in full) · `internal/database/plugin_schema.go`
- `internal/plugins/foundry_vtt/reconcile_consolidation.go` +
  `migrations/002_ensure_campaign_tokens.{up,down}.sql`
- Pins: `cmd/server/freshdb_migration_test.go` ·
  `internal/database/plugin_migration_recovery_test.go` ·
  `internal/database/plugin_migration_safety_test.go` ·
  `internal/plugins/foundry_vtt/reconcile_consolidation_test.go`

---

## ADR-051: The server records its own recent errors in a bounded in-memory ring, not a table

**Status:** Accepted

### Context

Chronicle could fingerprint every installed system package down to the byte and
was **completely blind to itself**. Nothing reported the host binary, its
commit, its build time, its uptime, its served or embedded assets, or its recent
errors. Diagnosing a routine deploy took an hour of shell archaeology and
produced two confident wrong conclusions — a Docker image label read as the
identity of a running process, and an empty `grep /app/static` read as missing
code (the assets were `//go:embed`-ed into the binary).

The error half of that blindness had its own shape. An error that fired at 2am
left **no trace an admin could reach**: it went to `slog`, `slog` went to
stdout, stdout went to the container log driver. Answering "what broke
overnight?" required shell access, `docker logs`, and knowing which container —
i.e. exactly the dependency the whole `host.*` family exists to remove.

The audit plugin cannot serve this purpose. It records **user actions**, is
DB-backed, and is keyed by campaign AND user — so a 500 on `/healthz`, or any
anonymous request, has neither key it needs.

### Decision

A new leaf package `internal/observability` holds a **fixed 256-slot,
mutex-guarded ring** of recent server errors, allocated at package init so it
records from the first request rather than being nil until wiring runs. It is
read through the diagnostics catalog as `host.errors` and `host.errors-summary`.

**1. What is recorded — 5xx responses and recovered panics, and nothing else.**
The policy is a named function (`ShouldRecord`), not an inline condition, so it
can be read and argued with in one place. The reason is **eviction, not
volume**: the ring evicts oldest-first, so a 404 storm would silently evict the
one 500 that matters. Each entry holds time, status, method, route template,
a `Kind`, and the error string. `Kind` separates an `*apperror.AppError` raised
deliberately (`KindApp`) from a raw error that escaped a handler (`KindRaw`) —
identical on the wire, completely different to whoever fixes it.

**2. What is deliberately NOT recorded.** No headers, no bodies, no query
strings, no user or campaign id. Above all **not the requested path**: `PathFor`
stores the route **TEMPLATE**, because Chronicle really routes `/rsvp/:token`,
`/proposals/respond/:token` and `/join/:code`, and a
concrete path would put a live credential into a buffer whose entire purpose is
to be pasted into a chat window. This is also what lets the summary collapse a
thousand failures on one route into one line. The error string is capped at 300
bytes (truncation marked) because a wrapped driver error can carry a failed
statement including bound values; it additionally passes through the existing
`redactSecrets` at render. That is **defence in depth, not a guarantee** — the
stored error string remains the weakest privacy link and is bounded rather than
solved.

**3. Why in-memory rather than persisted.** A table would need a migration, a
retention policy, a write on the error path, and would fail exactly when the
database is the thing that is broken — the case an error diagnostic most needs
to survive. The ring is ~50 KB allocated once, cannot fail, and cannot slow the
error path. The cost is real and is **stated in the output rather than hidden**:
a restart empties it, and each replica keeps its own, so an operator running
more than one replica sees only the one that served their admin request.

**4. Three renders that must never look alike.** "Provider not wired", "wired
and holding zero", and "the ring wrapped, N evicted" are distinct outputs. An
unwired provider says so **and explicitly denies meaning "no errors have
occurred"** — a diagnostic that reports a fake clean bill of health is worse
than no diagnostic, because it looks like an answer.

**5. Two write hooks, because one is not enough.** `app.errorHandler` records
and then delegates to its existing behaviour completely unchanged. Separately,
`middleware.Recovery` records the panic value — a recovered panic **never
reaches the error handler**, because `recovery.go` writes its own 500 with
`c.String` and returns nil, so Echo sees no error at all.

### Consequences

- An admin can answer "what broke overnight?" from the admin UI, with no shell.
- **Errors outside the two HTTP hooks are still invisible here.** Anything logged
  with `slog.Error` from a service or a background goroutine (WebSocket pumps,
  the calendar back-catalog walk, migrations) reaches stdout and nothing else.
  A `slog.Handler` wrapper teeing records at `>= LevelError` into the same ring
  was considered and deliberately **not** built: it widens what is stored from a
  fixed six-field summary to arbitrary log attributes, which needs its own pass
  over what those attributes can carry.
- The ring is not an audit trail and must never be cited as one. It is a
  recent-errors window for an operator standing in front of a running server.
- `internal/observability` is a **leaf**: standard library only, no Chronicle
  imports. Writers are `internal/app` and `internal/middleware`; the reader is
  `internal/systems` via provider injection. No cycle is possible.

### References

- `internal/observability/errorlog.go` (package doc states the non-durability
  claim in full) · `internal/systems/operator_diag_errors.go`
- Write hooks: `internal/app/app.go` (`errorHandler`) ·
  `internal/middleware/recovery.go`
- Wiring: `systems.SetRecentErrorsProvider` in `App.RegisterRoutes`, pinned on
  the boot path by `internal/app/operator_diag_wiring_test.go`
- Pins: `internal/observability/errorlog_test.go` ·
  `internal/systems/operator_diag_errors_test.go` ·
  `internal/systems/operator_diag_catalog_test.go`
- Docs: `docs/operator-diagnostics.md` §"The `host.*` family"

---

## ADR-052: The per-day moon discs get their own container-query threshold, not the named-event one

**Status:** Moot. The code it governed was deleted in #595; the calendar is
being rebuilt, #741.

It gave the per-day moon-disc row its own, lower container-query width
threshold, separate from the named-event threshold, after a census showed
the discs were unreachable on phones and on any long in-world week.

Full text: https://github.com/keyxmakerx/Chronicle/blob/dfc73c78/.ai/decisions.md

---

## ADR-053: The Sync API toggle refuses the Bearer key, not the route — and enabling it is a decision only a human or a key-creation makes

**Status:** Accepted

### Context

A campaign has a "Sync API" addon toggle (slug `sync-api`, category
`integration`, seeded by `db/migrations/000001_baseline.up.sql`). Switching it
off did nothing at all.

`internal/plugins/syncapi/routes.go` mounted `v1 := e.Group("/api/v1",
RequireAuthOrAPIKey(...), RateLimit(...), RequireJSONContentType())` with no
addon check, and the campaign sub-group underneath it carries ~50 endpoints.
`RequireAuthOrAPIKey` → `RequireAPIKey` → `syncAPIService.AuthenticateKey`
validates prefix, bcrypt hash, `IsActive` and expiry and never reads
`campaign_addons`; `addons.IsEnabledForCampaign` is the only place "enabled" is
ever evaluated and it was not on this path. The WebSocket had the same hole:
`AuthenticateKeyForWS` delegates to the same `AuthenticateKey` and is wired at
`internal/websocket/auth.go:81,107`.

Only `calGroup` and `mapGroup` were gated, with `RequireAddonAPI(addonChecker,
"calendar" / "maps")`. The pattern existed; it had simply never been applied to
the addon that governs the API itself.

### Decision

**1. The toggle refuses REAL BEARER KEYS, and nothing else.**

`RequireSyncAPIAddon` is mounted on both `/api/v1` groups (`v1` and
`v1Multipart`), after the identity resolver and before the rate limiter. It
short-circuits for any caller whose resolved `APIKey.ID == synthKeySessionID`.

The rejected alternative was reusing `RequireAddonAPI`, which is one line. It
gates EVERY caller — and `/api/v1/*` is dual-auth: Chronicle's own browser
widgets authenticate there by session cookie
(`static/js/widgets/layout_editor.js` reads `/entity-types` and `/maps` that
way) and receive a synthetic key. `sync-api` is an **integration** toggle; an
owner switching it off means "no outside clients", not "stop rendering my
layout editor". `calendar` and `maps` are **feature** addons and gating their
web callers too is right for them and wrong here. Pinned by
`TestSyncAPIAddon_SessionCallerUnaffected`, which 404s under the naive fix.

**2. It answers 403 `sync_api_disabled`, not 404.**

Rejected 404 (what `RequireAddonAPI` returns) on evidence from the consumer:
Chronicle's Foundry module reads a 404 on an API route as "this Chronicle is
too old to have that endpoint" and takes its version-compatibility path, hiding
the real cause — the same trap that made the calendar blackout answer 503
rather than 404 on purpose. The key is authentic and the campaign is real; what
is absent is authorization, which is 403. The machine-readable `type` lets a
client name the condition instead of parsing prose.

**3. It is NOT folded into `AuthenticateKey`.**

Rejected: it would be one choke point covering REST and WS together, but every
REST refusal would surface as `RequireAPIKey`'s blanket 401 "invalid api key",
which is a lie — the key is valid. `AuthenticateKey` keeps answering one
question ("is this token a live key?") and each transport shapes its own
refusal: middleware for REST, `AuthenticateKeyForWS` for the socket.

**4. The WebSocket is enforced AT CONNECT, and in-flight sessions are not
dropped.**

Rejected dropping live sockets. The hub has no disconnect-by-campaign
mechanism, and adding one would make this toggle *stronger than key
revocation*: deactivating or deleting an API key — the established revocation
control — also only takes effect at reconnect, as does revoking a `dm_granted`
flag (`Client.IsDmGranted`, "revoking a grant requires the user to reconnect").
A toggle that outranks revocation would be incoherent, and building a
revocation-polling loop into the hub in the same change as the gate widens the
blast radius of the deploy for no gain over the control it would exceed. This
is a connect-time control, uniformly with every other authorization fact the
hub resolves.

**5. Enforcement defaults to DENIED, so `CreateKey` records the decision.**

`addons.IsEnabledForCampaign` returns false when no `campaign_addons` row
exists. `syncapi/migrations/003_autoenable_existing_keys.up.sql` backfilled
`enabled = 1` for campaigns with an `api_keys` row, but it ran once; any
campaign that minted its first key afterwards sits at "no row".

`syncAPIService.CreateKey` now calls `EnableForCampaignBySlug` after the key
row commits. Rejected leaving it to the boot reconciler alone: a campaign
minting its first key would get a token that is dead until the next server
restart — "restart the server to make your new key work" is a worse defect than
the one being closed. This does re-enable a toggle an owner may have switched
off; that is deliberate and logged, because the owner is at that moment on the
API keys screen asking for an external credential, and the `campaign_addons`
row is the only record this system keeps of that. Nothing else re-enables it.
Best-effort on failure: the key row is committed and its plaintext is shown
once, so returning an error would destroy an unrecoverable credential.

**6. The backfill is a reconciler keyed on "is there a row", not "is it on".**

`syncapi.ReconcileAddonEnablement` runs at boot from
`internal/app/routes.go`, alongside `backfillPlayerCharacterTypes`, and enables
`sync-api` only for campaigns that own an API key and have **no
`campaign_addons` row at all**. A row saying `enabled = 0` is an owner's
decision and is left alone.

This is the trap. Migration 003's `ON DUPLICATE KEY UPDATE enabled = 1` was
harmless as a one-shot when "enabled" meant nothing; as a BOOT reconciler the
same clause would re-enable every key-owning campaign on every restart, so
switching the toggle off would last exactly until the next deploy — handing
back the decorative toggle this ADR removes. Hence the new
`HasCampaignAddonRecord`: `IsEnabledForCampaign` cannot distinguish "never
configured" from "explicitly off" and is the wrong question for a backfill.
Pinned by `TestReconcileAddonEnablement`, whose "deliberately switched it OFF"
row fails under the migration's semantics.

Per CLAUDE.md a one-time data fix is a reconciler, never a migration; it also
has to be re-runnable, since a campaign can acquire its first key between two
boots of an older build.

**7. An unwired gate refuses.**

`SetAddonGate` is injected in `internal/app/routes.go`. If that line is ever
dropped, `AuthenticateKeyForWS` fails with a distinct internal error rather
than assuming permission. A security control that silently no-ops when its
dependency is missing is the defect class this ADR exists to remove.

### Deliberately out of scope

`/api/version` (unauthenticated by design, pre-dates auth) and
`/api/v1/campaigns/:cid/foundry-vtt/module.{json,zip}`
(`foundry_vtt.RegisterPublicRoutes`, its own per-campaign signed token, not a
Bearer key) stay ungated. The operator must be able to fetch and update the
module in order to reach the toggle at all.

### Where it lives

- `internal/plugins/syncapi/middleware.go` — `RequireSyncAPIAddon`,
  `syncAPIDisabledError`, `SyncAPIAddonSlug`
- `internal/plugins/syncapi/routes.go` — mounted on `v1` and `v1Multipart`
- `internal/plugins/syncapi/service.go` — `SyncAPIAddonGate`, `SetAddonGate`,
  the `AuthenticateKeyForWS` gate, `CreateKey`'s enable
- `internal/plugins/syncapi/reconcile_addon_enablement.go` — the boot backfill
- `internal/plugins/addons/{service,repository}.go` —
  `EnableForCampaignBySlug`, `HasCampaignAddonRecord`
- Tests: `internal/plugins/syncapi/addon_gate_test.go`,
  `internal/plugins/syncapi/reconcile_addon_enablement_test.go`

## ADR-054: The sync API is a second door to the same rooms, and it checks the same locks

**Status:** Accepted

### Context

Six features across three plugins have the same defect: the web route enforces
a role and the `/api/v1` twin enforces only a permission tier.

| Resource | Web | `/api/v1` |
|---|---|---|
| fog of war (read) | `RequireRole(RoleOwner)` | `RequirePermission(PermRead)` |
| fog of war (write/reset) | Owner | `PermWrite` — a Scribe |
| map layers incl. `gm` | role-checked | no filter at all |
| marker/drawing/token delete | Owner | `PermWrite` — a Scribe |
| Player Notes toggle | hides the panel | five routes, zero checks |
| Notes toggle | hides the notebook | journal page + API open |

The mechanism is one function. `RequireAuthOrAPIKey` synthesises an `APIKey`
from a browser session with `permissionsForCampaignRole(role)` — Owner gets
`[read, write, sync]`, Scribe `[read, write]`, Player `[read]` — and every
syncapi route then checks a *permission*. Any resource whose web rule is finer
than "can read" or "can write" loses that rule on the way through. A player's
ordinary session therefore reads fog of war that the web refuses them. Nobody
built this as a hole; the API was built as a second, coarser vocabulary and
routes were added to it one at a time without asking what the web twin required.

### Decision

**1. The invariant, stated once.** For every resource reachable through
`/api/v1`, the API route's authorisation is at least as strict as the web
route's for the same action. The API is not a different product with its own
rules; it is another door into the same rooms.

**2. The mechanism: routes declare a role floor, not only a permission.** Where
a web route says `RequireRole(RoleOwner)`, its syncapi twin says so too. The
role comes from:
- the session, for the synthetic key — it keeps today's live-membership role;
- **Owner, fixed**, for a real stored Bearer key. Keys are strictly
  Owner-minted (`POST /api-keys` is `RequireRole(Owner)`), and the WebSocket
  path already granted any valid key Owner role, so REST now agrees instead of
  disagreeing with it. This is deliberately **decoupled** from the creator's
  live membership rather than re-checked on every call: if the creating user
  later loses Owner access or leaves the campaign, the key keeps syncing
  rather than an external integration silently breaking, but
  `flagIfKeyOwnerLostAccess` emits a loud, operator-visible signal instead of
  degrading quietly (`api_handler.go`).

This is chosen over a fourth permission tier (`PermGM`) precisely because of
the operator's Foundry key: that key was created by the Owner, so it keeps
reading fog and layers **without being re-issued**. A new tier would have
required every existing integration key to be re-minted before map sync
worked again, which is an outage dressed as a security fix.

**3. The guard: each syncapi handler resolves one role and hands it to the
SAME service-layer filtering the web path uses**, rather than checking a
separate permission tier. `resolveRole` (per handler) is the single place a
Bearer key or session becomes a `campaigns.Role`, pinned by
`resolve_role_test.go`; that role then drives `ListMarkers` /
`ListDrawings` / etc. exactly as the web handlers do, so the two surfaces
cannot drift onto different predicates for the same resource.

### Rejected

- **Collapsing syncapi into the plugins' own handlers.** Right in the long run —
  one handler, one lock — and wrong now: it is a rewrite on the scale of the
  calendar, and the calendar is the lesson.
- **A `PermGM` tier.** Expresses one distinction, breaks every existing key
  (above).
- **Treating the six as six tickets.** They are one habit — "gate what you can
  see" — and one habit needs one guard.

Only Owners can mint an API key (`POST /api-keys` is `RequireRole(Owner)`), so
a stored key never has a Scribe-level creator to resolve down to.

---

## ADR-055: Visibility stays per-subsystem; the read-path rule is the one thing they share

**Status:** Accepted

### Context

Chronicle carries five visibility models because it carries five kinds of
content:

| Subsystem | Model |
|---|---|
| entities | `is_private` bool, plus `custom` mode with per-subject grants |
| markers, drawings, timeline | `visibility` enum + `visibility_rules` allow/deny |
| tokens | `is_hidden` |
| notes | owner / `is_shared` / `shared_with` — starts most-private |
| maps, sessions | none — shared by nature |

The campaign `DefaultVisibility` setting is scoped to **entities** in its doc
comment, in the settings copy (three times), and in the only code that reads
it. The sweep found no creation-path inconsistency elsewhere: every marker,
drawing, token and timeline path defaults to `"everyone"` identically, web and
API alike. The obvious "fix" — make the campaign default reach everything — was
considered and is the thing this ADR exists to refuse.

What the sweep did find were four **read-path** leaks: a session page names
linked entities without checking their privacy; fog of war and GM layers are
listed to any player through the API; timeline `EventCount` still leaks the
existence of events hidden by per-user rules.

### Decision

**1. The models stay separate.** A note is personal by nature; a marker on a
shared map is public by nature; an entity is the thing a DM curates. They have
different shapes because the content does. Unifying them is a rewrite, and the
calendar is what a rewrite costs here.

**2. The campaign default stays entity-only.** Recorded so nobody "fixes"
markers or timelines to honour it. If a DM wants a hidden marker they hide the
marker; that is one click on an object, not a campaign-wide policy.

**3. The one rule every subsystem shares is on the read side:** *any query that
returns content to a non-Owner filters by that subsystem's own visibility
model, server-side, so hidden content is absent — not greyed, not counted, not
named, not ordered around.* This was already the repo's stated principle. It
now has an ADR number so a leak can cite what it violates.

**4. The four leaks are fixed under rule 3**, each with its own model, none by
inventing a shared one: session entity names, the timeline count, and fog of
war and GM layers over the sync API (ADR-054).

### Rejected

- **A campaign-wide visibility default for everything.** Rejected above.
- **"Private" as a creator-only visibility mode** for entities. The setting
  promised it; the code never did; see ADR-056 for why the promise goes
  rather than the code arriving.

---

## ADR-056: A toggle says what it does — code where the label is a promise, copy where the label is a name

**Status:** Accepted

### Context

Two toggles were found wrong in two different ways: Sync API
promised to cut off access and cut off nothing (**overpromised**); Sessions was
assumed dead and gated exactly one dashboard block while its real routes sat
under Calendar (**misnamed**). The sweep then found the same two shapes across
the rest of the catalogue, plus a third defect that is neither: **7 of 14
addons show "Campaign extension." as their entire description on the page
where an owner decides whether to enable them** — the real descriptions exist
in `builtinAddons` and never reach the screen because `PluginHubAddon` has no
`Description` field.

Every one of these needed the same question answered before it could be fixed:
is this a bug in the gate, or a bug in the words?

### Decision

**The test.** *If an owner would flip this believing it protects something or
cuts something off, the gate must be real — code. If the label merely points
at the wrong thing, the label moves — copy.* A control that lies about
protection is worse than no control, because the owner stops worrying.

Applied:

| Toggle | Verdict | Fix |
|---|---|---|
| Sync API | overpromised | **code** — done, ADR-053 |
| Player Notes | overpromised (five routes ungated) | **code** — gate the routes on the addon for every caller; it is a feature toggle, not an integration one, so no session short-circuit |
| Notes (floating) | overpromised (journal page + API open) | **code**, lower priority — same treatment |
| Sessions | misnamed | **copy** — describe the widget it gates; say Sessions lives under Calendar |
| Co-DM "control the live world-state" | promises a capability no route can exercise | **copy** now — drop the clause while the calendar is mid-rebuild; **code** when V5 wires `RequireCapability` |
| Calendar card | honest gate, undisclosed rebuild | **copy** — the disclosure every other surface already carries |
| Media Gallery "upload" | names an action that is deliberately never gated | **copy** — drop the word |
| Default Visibility "Private" | promises creator-only; delivers DM-only | **copy — remove the option.** See below |
| 8 generic descriptions | information missing | **code** — thread `Description` through `PluginHubAddon` |
| `settingsFeaturesTab` chain (~900 lines) | dead, drifted duplicate of the hub | **code** — delete |

**On "Private".** Two options that do the same thing is one option with a lie
attached. The honest alternatives were to build creator-only visibility or to
remove the claim. Creator-only already exists — per entity, via `custom` mode
with a user-subject grant — and a campaign-wide creator-only default would hide
a DM's work from their own co-DM, which is a niche want dressed as a default.
The option goes. Campaigns already storing `"private"` keep behaving exactly as
they do today (it collapses to `is_private=true`); the constant stays for
compatibility; the radio button does not. Removing a lying option beats
building what it lied about.

**Also under this ADR:** the partial-update contract test only recognises
structs named `Update*Input`; every `Update*Request` is invisible to it, and
the worst finding of the sweep (`tags.UpdateTagRequest`, a rename turns off
DM-only) lives exactly there. The scanner widens to `*Request`. A guard that
can see half the surface is a guard that certifies the other half by silence.

### Rejected

- **Fixing the descriptions by hand-editing the switch statement.** The text
  already exists once, in `builtinAddons`; a second copy is the drift the sweep
  found in the dead template.
- **Gating Player Notes with the Sync-API-style session short-circuit.** That
  short-circuit exists because `sync-api` is an *integration* toggle and
  first-party widgets share its routes. Player Notes is a *feature* toggle; off
  means off for everyone, or the toggle is decorative again.

## ADR-057: One visibility glance, shown to those who can change it, edited only in edit mode

**Status:** Accepted; amended to Option B — editing opens from the glance icon
(click, Owner only) as the widget's existing right-edge slide-in card
(`permissions.js` default layout, 420px, 280ms, backdrop), rather than from a
hover popover. Hover stays the read-only key; the edit form's old inline
mount (`form.templ` `data-layout="inline"`) is retired.

**Context:** an operator ask for one visibility icon, near the entity name,
that the DM team can glance at and click through to edit — replacing a
mismatched icon shown even to Players at the bottom of the page.

### Context

There is not one visibility indicator; there are **four**, each its own
implementation of the same three-glyph vocabulary (`fa-globe` / `fa-lock` /
`fa-shield-halved`):

| Where | File | Who sees it | Colour |
|---|---|---|---|
| Header, beside the name | `entities/visibility_badge.templ:10-32` | Scribe+ (raw `MemberRole`) | `#0d9488` hard-coded |
| List-grid card | `entities/entity_card.templ:78-100` | Scribe+ | `#0d9488` hard-coded |
| **"Details" card, bottom right** | `entities/show.templ:432-457` `blockDetails` | **everyone who can open the page — Players included** | `#0d9488` hard-coded |
| "Permissions" row, last row of every page | `permissions.js` via `blockPermissions`, `show.templ:618-636`, auto-appended by `EnsurePermissionsBlockInDefaults` (`service.go:2653-2699`) | Owner only | theme tokens (the only one) |

The third row is the operator's "random icon at the bottom": no role parameter
at all, shown to Players, painted a colour no theme defines. The fourth is the
editor, bolted onto the read page.

Two defects were found alongside: **a Co-DM cannot open a DM-only entity.**
`VisibilityRole()` promotes a DM-granted member to Owner for list filtering
(`campaigns/model.go:248-263`) and its doc comment says that is the design —
but every one of the nine `CheckEntityAccess` call sites passes raw
`MemberRole` instead (`entities/handler.go:599,1714,1800,1866,2096,3171,3246,
3486`; `syncapi/api_handler.go:374`). `BacklinksFragment` does both in one
function, lines 3229 and 3246: the Co-DM sees the list and is 404'd on the
target. No test covers it. And **a Scribe editing a DM-only entity is shown
"Permissions · Everyone"**: the widget's `load()` fires a GET the Scribe cannot
make (route is Owner-only), swallows the 403, and renders its init defaults
(`permissions.js:47-48,170-174,607-666`). An actively wrong glance.

### Decision

1. **One component.** `visibilityGlance(state, viewer)` in the entities
   plugin replaces all four. Same three glyphs — they are learned. The header
   position (beside the name) is the one that stays; it is where the eye
   already goes for the name and it is what the operator asked for.
2. **Seen by the DM team, never by Players or visitors.** The gate is
   `VisibilityRole() >= RoleScribe`, so a Co-DM sees it. A Player never does:
   a badge saying "custom" on something they *can* see tells them others
   cannot, which is evidence of hidden structure — ADR-055 rule 3 forbids
   exactly that. The indicator is a tool for people who can change access,
   not a label for people subject to it.
3. **Colour from tokens.** `var(--color-accent)` for custom, `--color-fg-muted`
   for everyone and DM-only. `#0d9488` is deleted from the tree; the guard is a
   grep in the render-contract test.
4. **Hover is a key, not a tooltip.** A popover, portalled to `<body>`, that
   names who has access: "Everyone in the campaign" · "DM team only (Owner,
   Scribes, co-DMs)" · for custom, the actual grants — role tiers, named
   members, groups — plus the tag-widening line the existing tooltip already
   builds (`visibility_glance.go:94-121`). Native `title=` goes.
5. **Editing lives in edit mode only.** The auto-appended "Permissions" row
   and its heal goroutine are removed from the read page. The edit form's
   inline widget (`form.templ:281-296`) is the one editor. This is the half
   of the ask that is a subtraction, and it is the bigger improvement.
6. **The two defects are fixed under this ADR, first**, because they need no
   design: every `CheckEntityAccess` caller passes `VisibilityRole()`; the
   widget renders nothing on a failed load, never a default.
7. **`public` as a grant subject is finished as its own later slice.**
   Migration `000028` added it to the enum so an owner could reveal an entity
   to logged-out visitors; the list filter honours it; `ValidSubjectType`
   refuses to write it and `GetEffectivePermission` ignores it. Half-wired is
   worse than absent. It gets wired, not deleted — after the glance ships.
8. **No "the party" audience.** Groups are manual and unseeded; a `role:1`
   grant ("every Player") is the working equivalent. A seeded "Party" group
   is not built.

### Rejected

- **Keeping four implementations and fixing the colour.** The colour is the
  symptom; the fourth copy is the disease.
- **A reduced indicator for Players** ("you can see this"). Any Player-facing
  state is a leak of the other states.
- **Merging with tag grants.** Tags widen visibility additively through a
  separate table; the glance *reports* that in the popover and must not own it.

## ADR-058: A picture inherits the permissions of the pages that use it

**Status:** Accepted; operator-ruled.

### Context

A security audit found that `checkMediaAccess`
(`internal/plugins/media/handler.go`) decides from: the file's campaign,
whether that campaign is public, the signature pair, the caller's user id,
campaign membership, and site-admin. Membership is ROLE-BLIND
(`GetMember(...) != nil`). It never consults the visibility of the entity the
file hangs off, because `media_files` carries no reference to one.

So **any member of a campaign, of any role, can read any image in that
campaign** — DM-only pages, custom-restricted pages, GM map layers. Three
doors that handed out file ids wholesale were closed the same day
(`61af48c5`, `901df82b`), but closing doors is not the same as locking the
room: anyone who learns an id another way still reads the image.

Two further facts shape the answer, and the second is the one that is easy to
miss:

1. **One file, many pages.** A picture can be referenced by several entities —
   as the main image, as a cover, or inline in entry HTML.
2. **Chronicle MERGES identical uploads.** `service.go:189` looks up
   `FindByContentHash(ctx, campaignID, hash)` and reuses the existing row. So
   two uploads of the same bytes become one file, and a file can end up shared
   between pages nobody deliberately linked. The operator's question — "a
   picture can be used multiple places?" — is what surfaced this.

### Decision

1. **A picture is readable if AT LEAST ONE page using it is visible to the
   viewer.** Not "hidden if any page using it is hidden".
   The strict rule is worse, not safer: a picture on both a public page and a
   hidden one would break the public page with a dead image, and a dead image
   where a picture obviously belongs is itself evidence that something is being
   withheld — the exact shape ADR-055 rule 3 forbids. It is also futile: the
   picture is already on screen on a page the viewer may open.
2. **This protects the picture, not the fact of reuse.** Putting a hidden
   page's artwork onto a visible page publishes that artwork, and no access
   rule can undo it. That is true today; this decision makes it the explicit
   reason rather than an accident. Hence 4 and 5.
3. **Files no page references keep today's behaviour** — campaign membership
   at the existing threshold. Avatars, campaign backdrops and freshly uploaded
   files have no owning entity by construction, and failing them closed would
   break the app.
4. **The "where is this used" list becomes part of the permissions story.** It
   exists (`FindReferences`) and is shown only in one Owner-only fragment. An
   owner must be able to see, before publishing, what else a picture is on.
5. **Merging across different permission levels is refused, and the author is
   told.** A silent merge is how a secret map's artwork becomes reachable
   months later with nobody deciding anything. Both halves were ruled by the
   operator ("Yes please to both"): do not merge when the existing file's pages
   and the new upload's destination do not agree, and say so when it happens.
6. **A signed URL is bound to the viewer.** Today it is an HMAC over
   `fileID:expires` only, valid an hour, so it is a bearer token: whoever holds
   the link uses it. Binding the identity in means a copied link is inert for
   anyone else. This does not replace 1 — a member can still mint their own —
   it stops the link LEAVING.
7. **A public campaign stops serving every file unsigned.** `allowUnsignedAccess`
   returns true for any file whose campaign is public, so today the whole
   internet can read every image in a public campaign, DM-only artwork
   included. Unsigned anonymous access is narrowed to pictures used by pages an
   anonymous viewer may actually see, which is decision 1 with `RoleNone`.

### Consequences, including the unwelcome ones

- **A lookup per image request.** Mitigated by caching the decision per
  (file, viewer) — Chronicle already runs Redis — and by 3's cheap path for
  unreferenced files. If the cache is unavailable the rule still applies; it
  gets slower, not laxer.
- **`FindReferences` is incomplete and must be fixed FIRST.**
  `repository.go:444-456` unions `entities.image_path` and an `entry_html LIKE`
  — it does NOT look at `cover_image_path`. A cover image would therefore look
  unreferenced and fall through to decision 3, which is the whole rule leaking
  through a missing column. This is not optional and is not a later slice.
- **The `LIKE` on `entry_html` is a scan.** It is acceptable per-request only
  behind the cache. If it proves too slow, the answer is an explicit
  media-to-entity link table, not relaxing the rule.
- **Someone will lose access to an image they can see today.** That is the
  point. It is a behaviour change on a live system and belongs in release
  notes, not in a silent deploy.

### Rejected

- **Adding an `entity_id` to `media_files`.** A single owner is wrong: one
  file legitimately serves many pages, and a merge makes that common rather
  than exceptional. A join table is the honest schema and is the fallback if
  the query cost bites — deliberately not built first, since the rule can ship
  without a migration.
- **Relying on unguessable ids.** They are v4 UUIDs from crypto/rand, so
  enumeration is not viable — but "you cannot guess it" is not an access
  control, and every leak fixed today was a way of being handed one.
- **Shortening the TTL alone.** It narrows the window and changes nothing
  about who may read. Worth doing, not a substitute.

## ADR-059: Chronicle stays separate from Grimoire; they integrate at the login and the link

**Status:** Accepted; operator-ruled. Open work: #634 (single sign-on / OIDC
login, the first integration seam below).

### Context

The operator asked whether Chronicle should be merged into, attached to, or
made an app inside [hunter-read/grimoire](https://github.com/hunter-read/grimoire),
on the belief that it "does most of what I am trying to accomplish already."
Measured against a clone of grimoire `main` at `fa7082ef` (2026-09-16):

- **Grimoire is a file library manager.** Its core is PDFs (every page indexed
  in SQLite FTS5, rendered server-side for reading), a battlemap image gallery,
  tokens, audio with a soundboard, 3D models, OPDS feeds. Its campaign tracker
  is a **flat markdown wiki** — `[[links]]`, per-page visibility, `||secrets||`,
  session scheduling. Its data model has **no entity, relation or timeline
  concept**; checked directly in `backend/models/`.
- **Chronicle is a structured world.** Typed entities with attributes, a
  relations graph, a timeline, interactive maps with markers/fog/live sync,
  permissions down to tags and groups, bidirectional Foundry sync, an item
  economy. The overlap with grimoire is the campaign-tracker band only.
- **Stacks are disjoint:** Python/FastAPI/SQLite/React versus Go/Echo/MariaDB/
  HTMX. Chronicle is ~182K lines (120K Go, 30K templ, 32K JS); grimoire ~78K.
- **Grimoire's addon system is not an app host.** Declarative YAML metadata
  scrapers, plus optional scripts run in a subprocess with — their words — "no
  database handle and no access to Grimoire internals." Nothing to plug an
  application into.
- **Grimoire is a one-person project, five months old.** 1.0.0 on 2026-04-06;
  250 of its last 298 commits (84%) by one author, the next human contributor
  at six. Well-run for its age (semver, changelog, architecture docs, CI), and
  still a single point of dependency.
- **Licences are compatible** (GPL-3 ↔ AGPL-3, GPL-3 §13) and it does not
  matter: the languages differ, so anything "pulled" is a reimplemented idea,
  not copied code.

### Decision

**Keep them separate. Run both. Integrate at two seams, cheapest first.**

1. **One login.** Grimoire already speaks OIDC (authorization code + PKCE).
   Chronicle has none. Put both behind one open-source identity provider and
   they become one suite: one login, one user list. This is the only change
   that makes them *feel* integrated, and it is Chronicle's largest missing
   feature independent of grimoire.
2. **Cross-links, with a division of labour.** Grimoire owns the library — the
   PDF, the battlemap image, the audio track. Chronicle owns the world and
   links out: a monster entity to its stat block at a page, a location to its
   map image. Grimoire publishes OpenAPI, so a "Grimoire resource" link type
   with a preview is small.

**Consequence for ADR-058 and the media renovation design:** some of what
Chronicle stores as media — battlemaps, audio — is grimoire's job. Where both
run, Chronicle's media scope may narrow to "pictures attached to entities."
That simplifies `.ai/designs/2026-09-13-media-renovation.md`; it does not
invalidate it.

**Features worth reimplementing from grimoire, ranked:** OIDC; guest invite
codes (code-only accounts scoped to one campaign, convertible and mergeable —
Chronicle's "guest" today means only "not logged in"); a per-user revocable
`.ics` session feed; a **player-safe export** (each person gets exactly what
they can see — Chronicle's export is Owner-only and dumps everything, verified
in `campaigns/export.go`); LegendKeeper import; the mixed-selection
confirmation pattern; per-user themes with a WCAG-AAA option.

**Not worth reimplementing:** PDF indexing, audio, 3D models, the token and
UVTT editors — grimoire's core, and the reason to run it alongside.

**Not actually gaps:** inline `||secrets||` (Chronicle has `editor_secret.js`,
server-stripped at `entities/handler.go:1744`); "restricted content is hidden
outright — the title is the spoiler" is ADR-055 rule 3.

### Rejected

- **Chronicle as an app inside grimoire.** Requires rewriting ~182K lines into
  a different language, database and rendering model; there is no host to
  plug into; and it ties Chronicle's future to one maintainer's roadmap.
- **Grimoire's features rebuilt inside Chronicle.** The same rewrite in
  reverse, and PDF indexing / audio / 3D are large specialised systems
  Chronicle has no reason to own.
- **Deep API coupling** beyond links and login. Grimoire's API is versioned
  and documented, but at 84% single-author the right depth of dependency is
  "a URL and a preview," not shared state.
