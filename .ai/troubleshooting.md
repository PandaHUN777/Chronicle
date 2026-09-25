# Troubleshooting

<!-- ====================================================================== -->
<!-- Category: Semi-static                                                    -->
<!-- Purpose: Known gotchas and their solutions. Prevents re-debugging the    -->
<!--          same non-obvious issues across sessions.                        -->
<!-- Update: Whenever a non-obvious bug is encountered and solved.            -->
<!-- ====================================================================== -->

> This file will grow as the project progresses. Add entries whenever you
> encounter and solve a non-obvious issue.

---

## Templ Files Not Updating

**Symptom:** Changed a `.templ` file but browser shows old content.

**Cause:** Templ generates `_templ.go` files that must be regenerated.

**Fix:** Run `make templ` or ensure `air` is configured to watch `.templ` files.

---

## HTMX Requests Returning Full Page

**Symptom:** Clicking an HTMX element replaces the whole page instead of just
the target element.

**Cause:** Handler not checking the `HX-Request` header.

**Fix:** Add `isHTMX(c)` check in handler before rendering. See
`.ai/conventions.md` for the pattern.

---

## MariaDB "parseTime" Error

**Symptom:** `sql: Scan error on column 'created_at', converting driver.Value
type []uint8 ("2026-01-15 10:30:00") to a time.Time`

**Cause:** Missing `parseTime=true` in the MariaDB DSN.

**Fix:** Ensure DATABASE_URL includes `?parseTime=true`:
```
user:pass@tcp(localhost:3306)/chronicle?parseTime=true
```

---

## Migration "dirty database" Error

**Symptom:** `make migrate-up` fails with "dirty database version N".

**Cause:** A previous migration partially applied.

**Fix:**
1. Check what version is dirty: `SELECT * FROM schema_migrations;`
2. Fix the migration SQL
3. Force version: `migrate -path db/migrations -database "$DATABASE_URL" force N`
4. Re-run: `make migrate-up`

---

## UUID Ordering in MariaDB

**Symptom:** Queries with `ORDER BY created_at` are slow or indexes not used
well with CHAR(36) UUID primary keys.

**Cause:** Random UUIDs (v4) cause index fragmentation in B-trees.

**Fix:** Consider UUID v7 (time-ordered) for better index locality. The
`google/uuid` package supports this with `uuid.Must(uuid.NewV7())`. Decision
to switch should be an ADR.

---

## Redis Connection Refused in Dev

**Symptom:** App fails to start with "redis: connection refused".

**Cause:** Redis container not running.

**Fix:** `make docker-up` to start MariaDB + Redis containers.

---

## "There is no database here" (there usually is)

**Symptom:** `make docker-up` fails because there is no Docker daemon, and
integration tests silently SKIP.

**Cause:** Docker is not the only way to get MariaDB. The MariaDB 10.11 server
binary (`mariadbd`) is often installed and runs directly. Two things make it
look broken when it isn't:

- `mariadbd` refuses to start as root unless given `--user=root`. The error
  ("Please consult the Knowledge Base to find out how to run mysqld as root!")
  reads like a permissions wall, not a missing flag.
- A Unix socket path longer than about 107 characters fails with a truncated
  path in the error, so a socket inside a long scratch directory looks like the
  server never started.

**Fix:** `make test-db-up` (or `tools/start-test-db.sh`) starts a disposable
server on port **13306**, never 3306, then `make test-int-local`. Integration
tests SKIP rather than FAIL when no server answers, so a green run without a
database proves nothing about them.

---

## Browser probes pass locally but fail in CI (or the reverse)

**Symptom:** a Browser Probes test fails in CI with nothing changed in the repo.

**Cause:** it was driving a different Chromium. CI pins Playwright 1.56.1
(Chromium 141.0.7390.37) and exports `CHROMIUM_BIN`. Every probe finder checks
`CHROMIUM_BIN` before `PATH`, because the runner image ships its own
`/usr/bin/chromium` and it changes without notice.

**Fix:** run the probes against the pinned build. Set `CHROMIUM_BIN` locally
to the same Chromium 141 build.

---

## A plugin's CSS or JS "isn't in the container"

**Symptom:** `grep` or `ls` under `/app/static` finds no trace of a plugin's
front-end asset, which looks like missing code.

**Cause:** Chronicle serves front-end files from two places. The on-disk
static root (`/app/static` in the container) is one. Some plugins instead
`//go:embed` their assets, and those files exist only inside the binary, where
no shell command can see them.

**Fix:** an empty grep of `/app/static` for an embedded asset is the expected
result. Check the plugin's `embed.go` instead, or run the `host.embedded`
admin diagnostic, which lists what is inside the binary
(`docs/operator-diagnostics.md`).
