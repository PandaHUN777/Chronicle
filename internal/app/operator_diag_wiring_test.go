package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Pins that every `SetXProvider` setter internal/systems declares for its
// app-layer dependency injection is really wired from RegisterRoutes: since
// systems cannot import app, the compiler cannot catch a forgotten call, and
// an unwired provider builds and unit-tests fine while permanently reporting
// "provider not wired" in production. Source-level AST checks (not a running
// server, not a comment-blind regexp) so the guard actually fails when the
// wiring line is missing or commented out.

// setProviderRe matches the provider-setter naming convention and captures the
// middle: `SetSyncMappingProvider` → "SyncMapping".
var setProviderRe = regexp.MustCompile(`^Set([A-Za-z0-9_]+)Provider$`)

// TestEveryDiagnosticProviderIsWiredInAppSource asserts that every provider
// setter internal/systems declares is really called from non-test app source.
// It does not check the call site's location; TestDiagnosticProviderCallsAreOnTheBootPath does.
func TestEveryDiagnosticProviderIsWiredInAppSource(t *testing.T) {
	declared := declaredProviders(t, filepath.Join("..", "systems"))
	if len(declared) == 0 {
		t.Fatal("found no Set*Provider declarations in internal/systems — this test would pass vacuously")
	}

	called := calledProviders(t, ".")

	for name := range declared {
		if !called[name] {
			t.Errorf("internal/systems declares Set%sProvider but no non-test file in internal/app calls systems.Set%sProvider.\n"+
				"An unwired provider ships as a diagnostic that permanently reports 'provider not wired', which reads like a real answer.\n"+
				"Wire it in RegisterRoutes (or, if it is genuinely wired outside internal/app, widen this test's search to that package).",
				name, name)
		}
	}
}

// TestDiagnosticProviderCallsAreOnTheBootPath checks that each provider call
// is reached from RegisterRoutes (which cmd/server/main.go calls at startup),
// not merely present somewhere in app source in a helper nothing invokes.
func TestDiagnosticProviderCallsAreOnTheBootPath(t *testing.T) {
	fset := token.NewFileSet()
	files := parseAppFiles(t, fset, ".")

	// Methods called from RegisterRoutes' body, plus RegisterRoutes itself.
	// One level only: all wiring is inline in RegisterRoutes today.
	bootFuncs := map[string]bool{"RegisterRoutes": true}
	for _, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "RegisterRoutes" || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok {
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == "a" {
						bootFuncs[sel.Sel.Name] = true
					}
				}
				return true
			})
		}
	}
	if !bootFuncs["RegisterRoutes"] || len(bootFuncs) == 1 {
		t.Fatal("could not find RegisterRoutes in internal/app — this test's assumption about the boot path is stale, re-derive it before deleting")
	}

	// Where each provider call actually sits. `found` records calls in the body
	// of a named function; `inLiteral` records calls buried inside a function
	// literal, kept apart only so the failure can say which of the two shapes
	// it saw instead of claiming the call does not exist.
	found := map[string]string{}
	inLiteral := map[string]string{}
	for _, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				// Do not descend into a function literal: a provider call inside
				// one only runs if something invokes the literal, which the AST
				// cannot confirm. This does not hide the shipped wiring, which
				// passes literals as ARGUMENTS (route-table closure, package
				// lister) — ast.Inspect visits the CallExpr before its
				// arguments, so that call is still recorded.
				if lit, ok := n.(*ast.FuncLit); ok {
					ast.Inspect(lit.Body, func(m ast.Node) bool {
						if name, ok := systemsProviderCall(m); ok {
							inLiteral[name] = fn.Name.Name
						}
						return true
					})
					return false
				}
				if name, ok := systemsProviderCall(n); ok {
					found[name] = fn.Name.Name
				}
				return true
			})
		}
	}

	for name := range declaredProviders(t, filepath.Join("..", "systems")) {
		host, ok := found[name]
		if !ok {
			if lit, buried := inLiteral[name]; buried {
				t.Errorf("systems.Set%sProvider is only called from inside a function literal in %s. "+
					"Nothing here proves that literal is ever invoked, so the provider may never be set at runtime — "+
					"move the call into the function body itself.", name, lit)
				continue
			}
			t.Errorf("systems.Set%sProvider is never called from internal/app, so it is not on any boot path", name)
			continue
		}
		if !bootFuncs[host] {
			t.Errorf("systems.Set%sProvider is called from %s, which RegisterRoutes does not call — so it is not on the boot path cmd/server/main.go runs", name, host)
		}
	}
}

// systemsProviderCall reports whether n is a `systems.SetXProvider(...)` call,
// returning the captured X.
func systemsProviderCall(n ast.Node) (string, bool) {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "systems" {
		return "", false
	}
	m := setProviderRe.FindStringSubmatch(sel.Sel.Name)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// declaredProviders returns the provider names declared by func decls in dir.
func declaredProviders(t *testing.T, dir string) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	out := map[string]bool{}
	for _, f := range parseAppFiles(t, fset, dir) {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			if m := setProviderRe.FindStringSubmatch(fn.Name.Name); m != nil {
				out[m[1]] = true
			}
		}
	}
	return out
}

// calledProviders returns the provider names called as systems.SetXProvider in dir.
func calledProviders(t *testing.T, dir string) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	out := map[string]bool{}
	for _, f := range parseAppFiles(t, fset, dir) {
		ast.Inspect(f, func(n ast.Node) bool {
			if name, ok := systemsProviderCall(n); ok {
				out[name] = true
			}
			return true
		})
	}
	return out
}

// parseAppFiles parses the non-test .go files directly inside dir. Test files
// are excluded: a provider wired only from a test is the bug being hunted.
func parseAppFiles(t *testing.T, fset *token.FileSet, dir string) []*ast.File {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	var out []*ast.File
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		path := filepath.Join(dir, n)
		// No ParseComments: this walk must catch wiring disabled by commenting
		// out the call, so comments must stay invisible to it.
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		out = append(out, f)
	}
	return out
}
