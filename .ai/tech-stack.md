# Technology Stack

<!-- ====================================================================== -->
<!-- Category: STATIC                                                         -->
<!-- Purpose: Quick reference for exact versions and why each tech was chosen. -->
<!-- Update: Only when a technology is added, removed, or upgraded.            -->
<!-- ====================================================================== -->

## Core

| Technology | Version | Role | Why |
|-----------|---------|------|-----|
| Go | 1.27+ | Backend language | Fast, single binary, strong typing |
| Echo | v4 | HTTP framework | Mature middleware, validation, Templ-friendly |
| Templ | latest | HTML templating | Type-safe, compiles to Go, component model |
| HTMX | 2.x | Frontend interactivity | Server-driven partials, no SPA, no Node |
| Alpine.js | 3.x | Client-side reactivity | Dropdowns, modals, toggles |
| MariaDB | latest (Docker image tag) | Primary database | User infrastructure requirement |
| Redis | latest/alpine (Docker image tag) | Sessions & cache | Session storage, rate limiting, caching |
| Tailwind CSS | 3.x (standalone CLI) | CSS framework | Utility-first, no Node needed |

## Frontend (Vendored, No Node.js)

| Library | Version | Role |
|---------|---------|------|
| TipTap | 3.x | Rich text editor widget (bundled via esbuild, see `static/vendor/tiptap-bundle.src.js`) |
| Leaflet.js | 1.9.x | Interactive maps (CDN-loaded per-page) |
| Font Awesome | 6 Free | UI icons |
| RPG Awesome | latest | TTRPG-themed icons |
| Inter | latest | UI font (self-hosted) |

## Go Dependencies

| Package | Role |
|---------|------|
| `github.com/labstack/echo/v4` | HTTP framework |
| `github.com/a-h/templ` | Template engine |
| `github.com/go-sql-driver/mysql` | MariaDB driver |
| `github.com/redis/go-redis/v9` | Redis client |
| `github.com/google/uuid` | UUID generation |
| `github.com/golang-migrate/migrate/v4` | DB migrations |
| `golang.org/x/crypto/argon2` | Password hashing (argon2id) |
| `github.com/microcosm-cc/bluemonday` | HTML sanitization |
| `github.com/extism/go-sdk` + `github.com/tetratelabs/wazero` | WASM plugin runtime (Extism host SDK on the wazero engine) |

## Dev Tools

| Tool | Purpose |
|------|---------|
| `air` | Hot reload for Go dev server |
| `templ` | Generate Go from .templ files |
| `tailwindcss` | Generate CSS (standalone binary) |
| `golangci-lint` | Linting |
| `gosec` | Security static analysis |
| `migrate` | CLI for running migrations |

## Docker Services

| Service | Image | Role |
|---------|-------|------|
| `chronicle` | Custom multi-stage | Go binary serves HTTP directly |
| `chronicle-db` | `mariadb:latest` | Database (persistent volume) |
| `chronicle-redis` | `redis:alpine` | Cache/sessions (128MB, allkeys-lru) |

## Environment Variables

See `internal/config/config.go` (`Load()`) for the full, authoritative list.
The ones most likely to need setting:

| Variable | Default | Description |
|----------|---------|-------------|
| `ENV` | `development` | `development` or `production`. Production enforces `SECRET_KEY` (32+ chars) and refuses the default `DB_PASSWORD`. |
| `PORT` | `8080` | HTTP listen port |
| `BASE_URL` | `http://localhost:8080` | Public URL |
| `LOG_LEVEL` | `debug` | `debug`, `info`, `warn`, `error` |
| `DB_HOST` / `DB_USER` / `DB_PASSWORD` / `DB_NAME` | `localhost:3306` / `chronicle` / `chronicle` / `chronicle` | MariaDB connection pieces |
| `DATABASE_URL` | (unset) | Overrides the `DB_*` pieces with a full DSN when set |
| `REDIS_URL` | `redis://localhost:6379` | Redis connection |
| `SECRET_KEY` | dev-only fallback | App secret (32+ chars in production). Encrypts stored SMTP passwords at rest. |
| `SESSION_TTL` | `720h` | Session duration (30 days) |
| `MAX_UPLOAD_SIZE` | `10485760` (10MB) | Max file upload, in bytes |
| `MEDIA_PATH` | `./data/media` | Where uploaded media is stored on disk |
| `MEDIA_SIGNING_SECRET` / `MEDIA_SIGNING_SECRET_FILE` | (unset) / `./data/.signing-secret` | HMAC secret for signed media URLs (see `.ai/conventions.md` §"Signed URLs") |
| `EXTENSIONS_PATH` | `./extensions` | Where WASM extension bundles are loaded from |
| `BACKUP_DIR` | `/app/data/backups` | Pre-migration backup destination |
