package app

// admin_pending_counter_wiring_test.go pins that admin.Handler.SetPendingCounter
// is actually invoked on the boot path. SetPendingCounter compiled and
// unit-tested fine with zero callers -- that is exactly how it shipped with
// the admin dashboard's "Pending" stat permanently reading 0: the interface
// existed, the setter existed, nothing ever called it. Mirrors
// operator_diag_wiring_test.go's approach: a source-level AST check (not a
// running server, not a comment-blind regexp) so a wiring line that goes
// missing or gets commented out actually fails this test.

import (
	"go/ast"
	"go/token"
	"testing"
)

// TestSetPendingCounterIsWiredInRegisterRoutes asserts that
// adminHandler.SetPendingCounter(...) is called, with a real argument, directly
// inside RegisterRoutes' body -- the function cmd/server/main.go calls at
// startup -- so the wiring is really on the boot path, not just present
// somewhere in app source that nothing invokes.
func TestSetPendingCounterIsWiredInRegisterRoutes(t *testing.T) {
	fset := token.NewFileSet()
	files := parseAppFiles(t, fset, ".")

	var registerRoutes *ast.FuncDecl
	for _, f := range files {
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "RegisterRoutes" && fn.Body != nil {
				registerRoutes = fn
			}
		}
	}
	if registerRoutes == nil {
		t.Fatal("could not find RegisterRoutes in internal/app -- this test's assumption about the boot path is stale, re-derive it before deleting")
	}

	found := false
	ast.Inspect(registerRoutes.Body, func(n ast.Node) bool {
		// Do not descend into a function literal: a call there only runs if
		// something invokes the literal, which the AST cannot confirm -- see
		// operator_diag_wiring_test.go's identical guard.
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "SetPendingCounter" {
			return true
		}
		recv, ok := sel.X.(*ast.Ident)
		if !ok || recv.Name != "adminHandler" {
			return true
		}
		// Reject SetPendingCounter(nil) -- that compiles and "counts" as
		// wired by a naive call-site search but still leaves the dashboard
		// reading 0.
		if len(call.Args) == 1 {
			if arg, ok := call.Args[0].(*ast.Ident); !ok || arg.Name != "nil" {
				found = true
			}
		}
		return true
	})

	if !found {
		t.Error("adminHandler.SetPendingCounter(...) is never called with a real argument from " +
			"RegisterRoutes' body, so the admin dashboard's pending-submissions count is wired " +
			"nowhere and always reads 0")
	}
}
