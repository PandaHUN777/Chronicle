// ancestor_visibility_reachability_test.go is a REACHABILITY test for the
// 2026-09-12 security audit's UNTESTED finding #1
// (.ai/designs/2026-09-12-security-audit-findings.md):
//
//	"the entity breadcrumb prints the full ancestor chain with no visibility
//	filter. FindAncestors is a bare recursive CTE and GetAncestors is passed
//	no role or user, so a hidden PARENT's name and link may render to anyone
//	who can see the CHILD."
//
// This drives the REAL repository CTE (entityRepository.FindAncestors) and
// the REAL canonical visibility predicate (entityRepository.FilterViewableEntityIDs,
// i.e. visibilityFilter — see repository.go ~line 1177) against a real
// MariaDB, and asserts the CORRECT behaviour: an ancestor a viewer could not
// otherwise see must not appear in the ancestor chain handed to that viewer.
// It does not install a fake of either function — a fake would prove nothing
// about whether the leak is real (house test-honesty doctrine,
// internal/app/error_handler_api_type_test.go).
//
// Scratch-schema pattern (own throwaway schema, fully migrated, dropped on
// cleanup) mirrors sessions/dbtest_support_test.go and
// media/entity_visibility_access_integration_test.go — NOT the shared dev
// database, and safe under `make test-db-up` (whose CHRONICLE_TEST_DB_DSN has
// no schema name, which the existing openTestDB() in this package cannot
// handle — see that helper's DSN comment).
package entities

import (
	crand "crypto/rand"
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/keyxmakerx/chronicle/internal/database"
	"github.com/keyxmakerx/chronicle/internal/permissions"
)

// newAncestorScratchDB creates a fresh, fully-migrated (core migrations only —
// entities lives in db/migrations/000001_baseline.up.sql plus later core
// migrations that add owner_user_id/map_id) throwaway schema and drops it on
// cleanup. Skips (never fails) when no server answers, per house convention.
func newAncestorScratchDB(t *testing.T) *sql.DB {
	t.Helper()

	raw := os.Getenv("CHRONICLE_TEST_DB_DSN")
	if raw == "" {
		t.Skip("set CHRONICLE_TEST_DB_DSN (make test-db-up) to run the ancestor-visibility reachability test")
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

	name := fmt.Sprintf("chronicle_ancestor_vis_%06d", rand.Intn(1000000)) //nolint:gosec // test schema name
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
	return db
}

func ancestorDBID(t *testing.T) string {
	t.Helper()
	var b [16]byte
	if _, err := crand.Read(b[:]); err != nil {
		t.Fatalf("random id: %v", err)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func mustAncestorExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// TestFindAncestors_LeaksHiddenParentToViewerWhoCanOnlySeeChild pins the fix
// for audit finding #1 end to end against real SQL. Originally written
// against the pre-fix `FindAncestors(ctx, entityID)` (no role/userID at
// all), it failed with the hidden grandparent surviving in the result — see
// the failure message this test produced before the fix, preserved in the
// fix's commit/PR history. FindAncestors and GetAncestors now take
// role/userID and apply visibilityFilter, so this test drives:
//
//  1. Seed a campaign with a PRIVATE grandparent entity (is_private=true,
//     default visibility mode — invisible to a Player/anonymous viewer,
//     per visibilityFilter's `e.visibility = 'default' AND (? >= 2 OR
//     e.is_private = false)` branch) and a PUBLIC child entity parented
//     under it (is_private=false — visible to everyone, matching the entity
//     show handler's own access gate).
//  2. Call the REAL entityRepository.FindAncestors(child.ID, role, userID)
//     — first as an owner (bypass) to sanity-check the fixture, then as
//     entities/handler.go:626's `h.service.GetAncestors(ctx, entity.ID,
//     int(cc.VisibilityRole()), userID)` now does in Show(): as an
//     anonymous/Player viewer.
//  3. Assert the hidden grandparent is OMITTED from the Player-viewer
//     result entirely (ADR-055 rule 3 — absent, not named), and cross-check
//     against the REAL, canonical FilterViewableEntityIDs(role=RolePlayer,
//     "") — the predicate List, Search, and the mention graph already
//     enforce — for drift protection.
//
// A failure here means the hidden grandparent is once again reachable
// through the ancestor chain: the exact regression this test exists to catch.
func TestFindAncestors_LeaksHiddenParentToViewerWhoCanOnlySeeChild(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a database; skipped under -short")
	}

	db := newAncestorScratchDB(t)
	ctx := context.Background()
	repo := NewEntityRepository(db)

	ownerID := ancestorDBID(t)
	campaignID := ancestorDBID(t)
	mustAncestorExec(t, db, `INSERT INTO users (id, email, display_name, password_hash) VALUES (?, ?, ?, ?)`,
		ownerID, "ancestor-vis-"+ownerID+"@example.test", "Ancestor Vis Owner", "x")
	mustAncestorExec(t, db, `INSERT INTO campaigns (id, name, slug, created_by) VALUES (?, ?, ?, ?)`,
		campaignID, "Ancestor Vis Test", "ancestor-vis-"+campaignID[:8], ownerID)
	t.Cleanup(func() {
		mustAncestorExec(t, db, `DELETE FROM campaigns WHERE id = ?`, campaignID)
		mustAncestorExec(t, db, `DELETE FROM users WHERE id = ?`, ownerID)
	})

	// One entity type shared by both rows.
	res, err := db.Exec(`INSERT INTO entity_types (campaign_id, slug, name, name_plural) VALUES (?,?,?,?)`,
		campaignID, "ancestor-vis-npc", "NPC", "NPCs")
	if err != nil {
		t.Fatalf("seed entity type: %v", err)
	}
	entityTypeID64, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("entity type last insert id: %v", err)
	}
	entityTypeID := int(entityTypeID64)

	now := time.Now().UTC()

	// A HIDDEN grandparent: is_private=true, default visibility. Its name is
	// the secret ("Secret DM Cabal HQ") — nothing an anonymous/Player viewer
	// should ever see named or linked.
	grandparent := &Entity{
		ID: ancestorDBID(t), CampaignID: campaignID, EntityTypeID: entityTypeID,
		Name: "Secret DM Cabal HQ", Slug: "secret-dm-cabal-hq-" + ancestorDBID(t)[:8],
		IsPrivate: true, FieldsData: map[string]any{}, CreatedBy: ownerID,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Create(ctx, grandparent); err != nil {
		t.Fatalf("create grandparent: %v", err)
	}

	// A PUBLIC child parented under the hidden grandparent.
	gpID := grandparent.ID
	child := &Entity{
		ID: ancestorDBID(t), CampaignID: campaignID, EntityTypeID: entityTypeID,
		Name: "Public Town Square", Slug: "public-town-square-" + ancestorDBID(t)[:8],
		ParentID: &gpID, IsPrivate: false, FieldsData: map[string]any{}, CreatedBy: ownerID,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Create(ctx, child); err != nil {
		t.Fatalf("create child: %v", err)
	}

	// Step 2a: sanity-check the fixture with an OWNER call (visibilityFilter
	// bypasses entirely at RoleOwner) — confirms the grandparent really is
	// structurally the parent before we test what a lesser role may see of it.
	ownerView, err := repo.FindAncestors(ctx, child.ID, permissions.RoleOwner, "")
	if err != nil {
		t.Fatalf("FindAncestors (owner): %v", err)
	}
	if len(ownerView) != 1 || ownerView[0].ID != grandparent.ID {
		t.Fatalf("test setup sanity check failed: want exactly [grandparent] for an owner, got %+v", ownerView)
	}

	// Step 2b: the exact call entities/handler.go:626 makes for the
	// breadcrumb (post-fix: role/userID threaded through, matching the
	// GetChildren call directly below it), as an anonymous/Player viewer
	// (role=RolePlayer, no user id — the role a public-campaign visitor
	// gets, and the SAME role class the audit finding names).
	ancestors, err := repo.FindAncestors(ctx, child.ID, permissions.RolePlayer, "")
	if err != nil {
		t.Fatalf("FindAncestors (player): %v", err)
	}

	// CORRECT (post-fix) behaviour: a hidden ancestor is OMITTED from the
	// chain entirely (ADR-055 rule 3) — not merely cross-checked against
	// FilterViewableEntityIDs afterward, since a caller (the breadcrumb
	// template) that already has the row will render it regardless of what
	// a separate predicate says elsewhere. If this regresses — e.g. the
	// role/userID threading above is dropped, or the outer SELECT's
	// visibilityFilter application is removed — the hidden grandparent
	// reappears here.
	for _, a := range ancestors {
		if a.ID == grandparent.ID {
			t.Errorf("SECURITY LEAK: FindAncestors(child=%s, role=Player, userID=\"\") returned hidden ancestor"+
				" %q (id=%s, is_private=true). entities/handler.go:626's breadcrumb renders every entry in this"+
				" slice unconditionally (ADR-055 rule 3: hidden content must be absent, not named).",
				child.ID, a.Name, a.ID)
		}
	}
	if len(ancestors) != 0 {
		t.Errorf("expected the Player-visible ancestor chain to be empty (grandparent is the only ancestor and it's"+
			" private), got %d entries: %+v", len(ancestors), ancestors)
	}

	// Cross-check against the canonical predicate too: whatever IS returned
	// to this viewer must also be something FilterViewableEntityIDs (the
	// same predicate List/Search/the mention-graph trust) agrees they may see.
	ancestorIDs := make([]string, len(ancestors))
	for i, a := range ancestors {
		ancestorIDs[i] = a.ID
	}
	viewable, err := repo.FilterViewableEntityIDs(ctx, campaignID, ancestorIDs, permissions.RolePlayer, "")
	if err != nil {
		t.Fatalf("FilterViewableEntityIDs: %v", err)
	}
	for _, a := range ancestors {
		if !viewable[a.ID] {
			t.Errorf("FindAncestors(child=%s) returned ancestor %q (id=%s) which FilterViewableEntityIDs says"+
				" role=Player/anonymous may NOT view — the two visibility predicates have drifted apart.",
				child.ID, a.Name, a.ID)
		}
	}
}
