// Package wire holds Chronicle's wire-contract integrity tests.
//
// This file is a cross-plugin import fence (T-B2 plugin isolation): a
// go/ast scan flags any new internal/plugins/A -> internal/plugins/B import
// edge not already in the grandfathered baseline. Allowlisted for any
// plugin: auth, campaigns, addons, audit, settings, smtp, media, plus a
// plugin's own subpackages and any .../api subpackage.
//
// Regenerate the baseline after an intentional change and commit it in the
// same PR:
//   UPDATE_PLUGIN_IMPORT_BASELINE=1 go test ./internal/wire/... -run TestPluginImportGuard
package wire

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// pluginImportBaseline is the grandfathered set of cross-plugin import edges,
// keyed "importer:importee" by plugin slug name. Only edges present in the
// live tree but absent here fail the test.
var pluginImportBaseline = map[string]bool{
	// admin imports (admin is a core plugin; these are grandfathered)
	"admin:campaigns": true,
	"admin:auth":      true,
	"admin:media":     true,
	"admin:settings":  true,
	"admin:smtp":      true,

	// packages imports
	"packages:auth": true,

	// syncapi imports (Finding 2: structural debt, post-launch refactor)
	"syncapi:calendar":  true,
	"syncapi:campaigns": true,
	"syncapi:entities":  true,
	"syncapi:maps":      true,
	"syncapi:media":     true,
	"syncapi:auth":      true,

	// ai_workspace imports (Finding 2: structural debt, post-launch refactor)
	"ai_workspace:calendar":   true,
	"ai_workspace:campaigns":  true,
	"ai_workspace:entities":   true,
	"ai_workspace:sessions":   true,
	"ai_workspace:timeline":   true,
	"ai_workspace:auth":       true,

	// maps imports
	"maps:campaigns":      true,
	"maps:auth":           true,
	"maps:widgetbindings": true,

	// widgetbindings imports
	"widgetbindings:campaigns": true,
	"widgetbindings:auth":      true,

	// campaigns imports
	"campaigns:auth": true,

	// calendar imports (Finding 2: structural debt, post-launch refactor)
	"calendar:addons":         true,
	"calendar:audit":          true,
	"calendar:auth":           true,
	"calendar:campaigns":      true,
	"calendar:widgetbindings": true,

	// foundry_vtt imports (Finding 5: packages URL coupling, post-launch)
	"foundry_vtt:packages": true,

	// timeline imports
	"timeline:widgetbindings": true,
}

// pluginSharedAllowlist lists plugins that are legitimately imported by any
// other plugin — they are "shared infrastructure" rather than optional features.
// Imports of these do NOT count as cross-plugin violations.
var pluginSharedAllowlist = map[string]bool{
	"auth":      true,
	"campaigns": true,
	"addons":    true,
	"audit":     true,
	"settings":  true,
	"smtp":      true,
	"media":     true,
}

// pluginImportEdge represents one cross-plugin import edge found in source.
type pluginImportEdge struct {
	Importer string // slug of the importing plugin
	Importee string // slug of the imported plugin
	File     string // repo-relative source file
}

// String formats the edge for human-readable output.
func (e pluginImportEdge) String() string {
	return fmt.Sprintf("%s → %s (in %s)", e.Importer, e.Importee, e.File)
}

// baselineKey returns the map key for this edge (without file context, since
// the baseline tracks slug pairs, not per-file occurrences).
func (e pluginImportEdge) baselineKey() string {
	return e.Importer + ":" + e.Importee
}

// scanPluginImportEdges walks internal/plugins and finds all cross-plugin
// Go import statements. Returns the full edge list (not de-duped by slug pair).
func scanPluginImportEdges(t *testing.T, repoRoot string) []pluginImportEdge {
	t.Helper()
	pluginsDir := filepath.Join(repoRoot, "internal", "plugins")
	modulePath := "github.com/keyxmakerx/chronicle"
	pluginImportPrefix := modulePath + "/internal/plugins/"

	var edges []pluginImportEdge

	err := filepath.Walk(pluginsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Skip test files — they legitimately cross plugin boundaries for fixtures.
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Determine the owning plugin slug from the file path.
		rel, _ := filepath.Rel(pluginsDir, path)
		// rel looks like "syncapi/handler.go" or "ai_workspace/aiexport/renderer.go"
		parts := strings.SplitN(rel, "/", 2)
		if len(parts) == 0 {
			return nil
		}
		ownerSlug := parts[0]

		// Parse the file for imports.
		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return nil // build errors caught elsewhere
		}

		repoRelPath, _ := filepath.Rel(repoRoot, path)

		for _, imp := range file.Imports {
			if imp.Path == nil {
				continue
			}
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}

			if !strings.HasPrefix(importPath, pluginImportPrefix) {
				continue
			}

			// Extract the imported plugin slug (first path component after internal/plugins/).
			after := strings.TrimPrefix(importPath, pluginImportPrefix)
			importedParts := strings.SplitN(after, "/", 2)
			if len(importedParts) == 0 {
				continue
			}
			importedSlug := importedParts[0]

			// Self-imports (same plugin, sub-package) are fine.
			if importedSlug == ownerSlug {
				continue
			}

			// Shared/infrastructure plugins are always allowed.
			if pluginSharedAllowlist[importedSlug] {
				continue
			}

			edges = append(edges, pluginImportEdge{
				Importer: ownerSlug,
				Importee: importedSlug,
				File:     repoRelPath,
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", pluginsDir, err)
	}

	sort.Slice(edges, func(i, j int) bool {
		ki := edges[i].Importer + ":" + edges[i].Importee + ":" + edges[i].File
		kj := edges[j].Importer + ":" + edges[j].Importee + ":" + edges[j].File
		return ki < kj
	})
	return edges
}

// TestPluginImportGuard enforces that no new cross-plugin import edges appear
// beyond the grandfathered baseline; only additions fail, not removals.
// TODO(#720): pay down the grandfathered cross-plugin imports.
func TestPluginImportGuard(t *testing.T) {
	root := repoRoot(t) // reuse from wire_contract_test.go
	edges := scanPluginImportEdges(t, root)

	// De-duplicate by slug pair for baseline comparison.
	seenPairs := map[string]bool{}
	var newEdges []pluginImportEdge
	for _, e := range edges {
		key := e.baselineKey()
		if seenPairs[key] {
			continue
		}
		seenPairs[key] = true
		if !pluginImportBaseline[key] {
			newEdges = append(newEdges, e)
		}
	}

	if os.Getenv("UPDATE_PLUGIN_IMPORT_BASELINE") != "" {
		// Print the full current edge set so the developer can paste it into the baseline.
		allKeys := make([]string, 0, len(seenPairs))
		for k := range seenPairs {
			allKeys = append(allKeys, k)
		}
		sort.Strings(allKeys)
		t.Logf("UPDATE_PLUGIN_IMPORT_BASELINE set — current cross-plugin edge set (%d pairs):", len(allKeys))
		for _, k := range allKeys {
			t.Logf("  %q: true,", k)
		}
		t.Log("Paste these into pluginImportBaseline in plugin_import_guard_test.go,")
		t.Log("then commit the updated file in the same PR.")
		return
	}

	if len(newEdges) == 0 {
		t.Logf("plugin-import-guard: OK — %d cross-plugin slug pairs, all within grandfathered baseline.", len(seenPairs))
		return
	}

	var msg strings.Builder
	fmt.Fprintf(&msg, "plugin-import-guard: %d NEW cross-plugin import edge(s) detected outside grandfathered baseline:\n\n", len(newEdges))
	for _, e := range newEdges {
		msg.WriteString("  " + e.String() + "\n")
	}
	msg.WriteString("\n")
	msg.WriteString("Cross-plugin imports of non-shared plugins violate T-B2 (plugin removability).\n")
	msg.WriteString("Per cordinator/decisions/2026-05-21-core-tenets.md §T-B2:\n")
	msg.WriteString("  - Use service interfaces + adapter wiring in internal/app/routes.go instead.\n")
	msg.WriteString("  - Or add the edge to the baseline with a note citing the dispatch/ADR\n")
	msg.WriteString("    that approved it (run UPDATE_PLUGIN_IMPORT_BASELINE=1 to regenerate).\n")
	msg.WriteString("\n")
	msg.WriteString("Shared plugins that are always allowed: auth, campaigns, addons, audit, settings, smtp, media.\n")

	t.Fatal(msg.String())
}
