// visibility_glance_guard_test.go scans the entities plugin's own .templ
// SOURCE files (not other plugins' templates, and not static/js/) so a
// future edit cannot quietly resurrect a hand-rolled copy of the visibility
// vocabulary visibilityGlance consolidates (ADR-057).
package entities

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// entitiesTemplDir resolves the directory this test file lives in — the
// entities plugin's own templates. Every .templ file directly inside it
// (there are no subdirectories of .templ sources in this plugin; see
// glanceGuardFiles) is scanned.
func entitiesTemplDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test file path")
	}
	return filepath.Dir(thisFile)
}

func glanceGuardFiles(t *testing.T) []string {
	t.Helper()
	dir := entitiesTemplDir(t)
	matches, err := filepath.Glob(filepath.Join(dir, "*.templ"))
	if err != nil {
		t.Fatalf("glob .templ files: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("found no .templ files to scan — the guard would pass vacuously")
	}
	return matches
}

// TestNoHardcodedTealVisibilityColour pins that #0d9488 is gone from the
// entities plugin's templates — visibilityGlance uses var(--color-accent)
// instead. Scoped to internal/plugins/entities/*.templ only:
// static/js/widgets/db_explorer.js carries the identical hex as an
// unrelated calendar swatch colour, and a tree-wide ban would fail on that
// innocent line.
func TestNoHardcodedTealVisibilityColour(t *testing.T) {
	for _, path := range glanceGuardFiles(t) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(b), "#0d9488") {
			t.Errorf("%s: hard-coded #0d9488 must not appear anywhere in the entities plugin's templates (ADR-057 decision 3) — use visibilityGlance / var(--color-accent) instead", filepath.Base(path))
		}
	}
}

// TestNoShieldIconOutsideVisibilityGlance pins that fa-shield-halved, the
// glyph unique to the "custom" visibility state, appears nowhere except
// inside visibility_glance.templ, the one component that owns it. This is
// deliberately narrower than a blanket fa-globe/fa-lock ban: both of those
// glyphs have other, unrelated, legitimate uses in this plugin
// (entity_types.templ's type-icon palette, index.templ's "All" tab icon,
// show.templ's per-block "DM Only" ribbon, category_dashboard.templ's
// decorative "Visibility" column header) that a same-string ban would
// false-positive on.
func TestNoShieldIconOutsideVisibilityGlance(t *testing.T) {
	const ownerFile = "visibility_glance.templ"
	for _, path := range glanceGuardFiles(t) {
		if filepath.Base(path) == ownerFile {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(b), "fa-shield-halved") {
			t.Errorf("%s: fa-shield-halved must only appear in %s (ADR-057 slice 3 — one shared visibilityGlance component)", filepath.Base(path), ownerFile)
		}
	}
}

// hardcodedVisibilityBadgeAttr matches a LITERAL data-visibility-badge
// attribute value, e.g. data-visibility-badge="custom". The shared
// component instead emits it as a dynamic templ expression,
// data-visibility-badge={ state }, which this pattern does not match.
var hardcodedVisibilityBadgeAttr = regexp.MustCompile(`data-visibility-badge="(everyone|dm_only|custom)"`)

// TestNoHardcodedVisibilityBadgeAttrOutsideVisibilityGlance guards against a
// resurrected hand-rolled copy: nothing outside visibilityGlance may
// hard-code the data-visibility-badge attribute value as a literal string
// rather than deriving it from baseVisibilityState.
func TestNoHardcodedVisibilityBadgeAttrOutsideVisibilityGlance(t *testing.T) {
	const ownerFile = "visibility_glance.templ"
	for _, path := range glanceGuardFiles(t) {
		if filepath.Base(path) == ownerFile {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if hardcodedVisibilityBadgeAttr.Match(b) {
			t.Errorf("%s: a literal data-visibility-badge=\"...\" attribute must only appear in %s", filepath.Base(path), ownerFile)
		}
	}
}
