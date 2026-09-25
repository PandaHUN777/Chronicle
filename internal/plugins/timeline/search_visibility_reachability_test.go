// search_visibility_reachability_test.go is a REACHABILITY test for the
// 2026-09-12 security audit's UNTESTED finding #2
// (.ai/designs/2026-09-12-security-audit-findings.md):
//
//	"cross-plugin timeline search applies only the SQL role narrowing and
//	returns results without passing them through the per-user filter that
//	List uses, so a restricted timeline entry may be named to an anonymous
//	viewer."
//
// This drives the REAL timelineRepo.Search (the SQL role-only narrowing) and
// the REAL timelineService.SearchTimelines (the caller the audit finding
// says was responsible for the missing per-user filter) against a real
// MariaDB, asserting the CORRECT behaviour: a timeline restricted to
// specific users by visibility_rules must not be named to a viewer that
// allow-list excludes, anonymous included.
//
// IMPORTANT — what this test actually found: repository.go's own doc
// comment on Search (and service.go's on SearchTimelines) already states
// this was fixed as a "2026-09-12 audit finding 4 follow-up" (ADR-058),
// with SearchTimelines now running repo.Search's results through the same
// filterTimelinesByUser List uses. That means the .ai/designs anchor at
// repository.go:251 has DRIFTED — it now lands mid-comment describing the
// historical bug, not live vulnerable code. This test is written to assert
// the CORRECT behaviour regardless of that history: if it FAILS, the leak
// is reachable; if it PASSES, the fix holds under a real database, which is
// itself the verdict (CHANGED, not CONFIRMED — see the audit report).
//
// Scratch-schema pattern mirrors sessions/dbtest_support_test.go: own
// throwaway schema, fully migrated (core + calendar + timeline plugin
// migrations — timelines.calendar_id FKs to calendars(id) even when NULL,
// so the calendar plugin's tables must exist first), dropped on cleanup.
package timeline

import (
	crand "crypto/rand"
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-sql-driver/mysql"

	"github.com/keyxmakerx/chronicle/internal/database"
)

// newSearchVisScratchDB creates a fresh, fully-migrated (core + calendar +
// timeline plugin) throwaway schema and drops it on cleanup. Skips (never
// fails) when no server answers.
func newSearchVisScratchDB(t *testing.T) *sql.DB {
	t.Helper()

	raw := os.Getenv("CHRONICLE_TEST_DB_DSN")
	if raw == "" {
		t.Skip("set CHRONICLE_TEST_DB_DSN (make test-db-up) to run the timeline-search-visibility reachability test")
	}
	cfg, err := mysql.ParseDSN(raw)
	if err != nil {
		t.Skipf("CHRONICLE_TEST_DB_DSN is not a valid DSN: %v", err)
	}
	cfg.ParseTime = true

	serverCfg := *cfg
	serverCfg.DBName = ""
	admin, err := sql.Open("mysql", serverCfg.FormatDSN())
	if err != nil {
		t.Skipf("no test DB (sql.Open: %v)", err)
	}
	t.Cleanup(func() { admin.Close() })
	if err := admin.Ping(); err != nil {
		t.Skipf("no test DB server reachable at %s: %v — run `make test-db-up`", cfg.Addr, err)
	}

	name := fmt.Sprintf("chronicle_tl_search_vis_%06d", rand.Intn(1000000)) //nolint:gosec // test schema name
	if _, err := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Skipf("cannot create scratch schema: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE IF EXISTS `" + name + "`") })

	scratchCfg := *cfg
	scratchCfg.DBName = name
	db, err := sql.Open("mysql", scratchCfg.FormatDSN())
	if err != nil {
		t.Fatalf("opening scratch schema: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	if err := database.RunMigrations(db, scratchCfg.FormatDSN(), filepath.Join(root, "db", "migrations")); err != nil {
		t.Skipf("core migrations did not apply: %v", err)
	}

	// timelines.calendar_id FKs to calendars(id) (nullable, but the FK
	// constraint still requires the table to exist at CREATE TABLE time), so
	// the calendar plugin's schema must be loaded before timeline's — same
	// dependency sessions/dbtest_support_test.go documents and loads off
	// disk (never by importing the package: internal/wire/
	// plugin_import_guard_test.go forbids a timeline→calendar edge).
	calDir := os.DirFS(filepath.Join(root, "internal", "plugins", "calendar", "migrations"))
	tlSub, err := fs.Sub(MigrationsFS, database.PluginMigrationsSubdir)
	if err != nil {
		t.Fatalf("sub-FS: %v", err)
	}
	for _, res := range database.RunPluginMigrations(db, []database.PluginSchema{
		{Slug: "calendar", MigrationsFS: calDir},
		{Slug: "timeline", MigrationsFS: tlSub},
	}) {
		if !res.Healthy {
			t.Skipf("%s plugin migrations did not apply: %v", res.Slug, res.Error)
		}
	}
	return db
}

func searchVisDBID(t *testing.T) string {
	t.Helper()
	var b [16]byte
	if _, err := crand.Read(b[:]); err != nil {
		t.Fatalf("random id: %v", err)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func mustSearchVisExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// TestSearchTimelines_DoesNotNameAllowListRestrictedTimelineToAnonymous
// reproduces audit finding #2 end to end against real SQL:
//
//  1. Seed a campaign, an allowed user, and a timeline with
//     visibility='everyone' (passes repo.Search's coarse SQL role filter —
//     this is NOT a dm_only timeline) but visibility_rules restricting it to
//     a single allowed user by id — the "restricted timeline entry" shape
//     the finding describes.
//  2. Call the REAL timelineRepo.Search directly and confirm it returns the
//     row (this is DOCUMENTED, intentional behaviour of that function —
//     repo.Search only narrows dm_only; it is not itself the bug).
//  3. Call the REAL timelineService.SearchTimelines as an ANONYMOUS viewer
//     (role=Player, userID="") and assert the timeline's name is ABSENT —
//     the per-user allow-list must still apply even though the coarse SQL
//     filter let the row through.
//  4. Sanity-check the harness: SearchTimelines AS the allowed user must
//     still return the timeline, proving step 3's absence is the allow-list
//     working, not a search string/setup mistake.
//
// A failure at step 3 is the proof the leak is reachable. A pass says the
// fix recorded in repository.go/service.go's doc comments (ADR-058) holds
// against a real database — see this file's header for why that makes the
// verdict CHANGED rather than CONFIRMED.
func TestSearchTimelines_DoesNotNameAllowListRestrictedTimelineToAnonymous(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a database; skipped under -short")
	}

	db := newSearchVisScratchDB(t)
	ctx := context.Background()
	repo := NewTimelineRepository(db)
	svc := NewTimelineService(repo, nil, nil, nil)

	ownerID := searchVisDBID(t)
	allowedUserID := searchVisDBID(t)
	campaignID := searchVisDBID(t)
	mustSearchVisExec(t, db, `INSERT INTO users (id, email, display_name, password_hash) VALUES (?, ?, ?, ?)`,
		ownerID, "search-vis-owner-"+ownerID+"@example.test", "Search Vis Owner", "x")
	mustSearchVisExec(t, db, `INSERT INTO users (id, email, display_name, password_hash) VALUES (?, ?, ?, ?)`,
		allowedUserID, "search-vis-allowed-"+allowedUserID+"@example.test", "Search Vis Allowed", "x")
	mustSearchVisExec(t, db, `INSERT INTO campaigns (id, name, slug, created_by) VALUES (?, ?, ?, ?)`,
		campaignID, "Search Vis Test", "search-vis-"+campaignID[:8], ownerID)
	t.Cleanup(func() {
		mustSearchVisExec(t, db, `DELETE FROM campaigns WHERE id = ?`, campaignID)
		mustSearchVisExec(t, db, `DELETE FROM users WHERE id IN (?, ?)`, ownerID, allowedUserID)
	})

	// A timeline visible to 'everyone' at the coarse level, but restricted by
	// visibility_rules to allowedUserID only.
	timelineID := searchVisDBID(t)
	const timelineName = "Secret Faction Uprising Timeline"
	visRules := fmt.Sprintf(`{"allowed_users":["%s"]}`, allowedUserID)
	mustSearchVisExec(t, db, `INSERT INTO timelines
		(id, campaign_id, name, visibility, visibility_rules, created_by, created_at, updated_at)
		VALUES (?,?,?,?,?,?,NOW(),NOW())`,
		timelineID, campaignID, timelineName, "everyone", visRules, ownerID)

	const rolePlayer = 1 // permissions.RolePlayer — an anonymous public-campaign viewer's role.
	query := "Secret Faction"

	// Step 2: repo.Search's own documented behaviour — role-only narrowing,
	// no per-user filter. This is expected to return the row.
	rawResults, err := repo.Search(ctx, campaignID, query, rolePlayer)
	if err != nil {
		t.Fatalf("repo.Search: %v", err)
	}
	foundRaw := false
	for _, tl := range rawResults {
		if tl.ID == timelineID {
			foundRaw = true
		}
	}
	if !foundRaw {
		t.Fatalf("test setup sanity check failed: repo.Search did not return the seeded timeline at all")
	}

	// Step 3: the full service path, as anonymous.
	anonResults, err := svc.SearchTimelines(ctx, campaignID, query, rolePlayer, "")
	if err != nil {
		t.Fatalf("SearchTimelines (anonymous): %v", err)
	}
	for _, r := range anonResults {
		if r["name"] == timelineName {
			t.Errorf("SECURITY LEAK: SearchTimelines(role=Player, user=anonymous) named restricted timeline %q"+
				" (id=%s), whose visibility_rules allow only user %s. The per-user allow-list was not applied"+
				" after repo.Search's coarse SQL role filter.", timelineName, timelineID, allowedUserID)
		}
	}

	// Step 4: sanity check — the allowed user must still find it, proving
	// the absence above is the allow-list, not a broken query string.
	allowedResults, err := svc.SearchTimelines(ctx, campaignID, query, rolePlayer, allowedUserID)
	if err != nil {
		t.Fatalf("SearchTimelines (allowed user): %v", err)
	}
	foundAllowed := false
	for _, r := range allowedResults {
		if r["name"] == timelineName {
			foundAllowed = true
		}
	}
	if !foundAllowed {
		t.Fatalf("test setup sanity check failed: SearchTimelines did not return the timeline even for the allow-listed user")
	}
}
