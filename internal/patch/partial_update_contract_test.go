// partial_update_contract_test.go — the structural ratchet for sweep R4's
// absent-means-preserve contract.
//
// WHY THIS SHAPE, and not the one the dispatch asked for first.
//
// The ask was "a guard that fails when a service's update input carries a
// VALUE-typed field the service assigns unguarded". The second half of that
// is not expressible with go/ast, and pretending otherwise would have
// produced a guard that passes by not looking — the exact failure sweep R3
// stage 12 spent a stage repairing. Deciding whether an assignment is
// "unguarded" needs data flow from an input field, through local variables
// and helper calls, into a model field, across package boundaries; and even
// with perfect flow analysis the answer is frequently "unguarded on purpose"
// (a required Name validated non-empty upstream, a Visibility defaulted from
// the stored row two lines above). A checker for that would be ~all false
// positives and would be allowlisted into meaninglessness inside a month.
//
// What IS expressible, and is the actual precondition for the bug: an update
// input can only preserve an absent key if its fields can REPRESENT absence.
// Every defect in this sweep — sessions, entities, timeline events, markers,
// calendar events — reduces to a field whose Go type has no absent state, so
// the service had nothing to guard on even if it wanted to. This guard pins
// representability, whole-tree, in two directions:
//
//  1. Every field of a CONTRACT-GOVERNED input must be able to say "absent":
//     patch.Field[T], a pointer, a map or a slice. A value-typed field cannot
//     land on one of these structs without a NAMED exception below.
//  2. The full inventory of Update*Input structs in internal/ is frozen. A
//     NEW one must be added to one list or the other, which forces its author
//     to decide — out loud — whether it is a partial update.
//
// It replaces nothing: each fixed endpoint also has its own three-direction
// regression test (absent preserves · present replaces · explicit null
// clears), and the convention is written down in .ai/conventions.md.
package patch_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// contractGoverned lists the update inputs converted by sweep R4. Every field
// of these must be presence-aware.
var contractGoverned = map[string]string{
	"sessions.UpdateSessionInput":       "PUT /campaigns/:id/sessions/:sid — Mark Complete wiped the schedule, summary, in-world date and recurrence",
	"entities.UpdateEntityInput":        "PUT /entities/:eid and the syncapi twin — every sync push un-parented the entity; {name} alone un-privated it",
	"timeline.UpdateTimelineEventInput": "PUT .../standalone-events/:eid — a rename cleared eight fields, including per-player visibility rules",
	"maps.UpdateMarkerInput":            "PUT .../markers/:mkid — an edit or a drag cleared pin_category, visibility_rules and the Foundry pairing key",
	// CALV5 SALVAGE: restored with the domain layer (2026-08-21). It is on the
	// GOVERNED list, not the allowlist, because the recovered type is already
	// presence-aware — the sweep that fixed it survived in the recovered file.
	"calendar.UpdateEventInput": "PUT .../calendar/events/:eid — Foundry's five-key push turned off recurrence, all-day and the entity link",

	// Original note, kept because the incident is the reason the entry exists:
	// "PUT .../calendar/events/:eid — Foundry's five-key push turned off
	// recurrence, all-day and the entity link".
	//
	// That is a REAL incident, not a hypothetical: the Foundry module sends a
	// narrow body on a rename, and a value-typed input read every absent field
	// as a zero and wrote it. V5's event update input must be presence-aware
	// from its first commit, and this line must come back with it — the module
	// still sends narrow bodies (its own contract pins them), so the defect is
	// waiting for any successor that forgets.

	// ADR-054 #1-#6 (2026-09-12 partial-update sweep, task #6). All six
	// reproduced against the shipped code with a temporary red test before
	// the fix; see the *_partial_update_test.go files next to each.
	"maps.UpdateTokenInput":        "PUT .../tokens/:tid (web + syncapi) — a drag PUT carrying only {x, y} zeroed IsHidden, IsLocked, both HP bars and every aura/light/vision field; a hidden ambush monster went visible on the next nudge",
	"maps.UpdateDrawingInput":      "PUT .../drawings/:did (web + syncapi) — shares UpdateTokenInput's shape; a reshape-only push wiped fill, text content, font size and rotation. No shipped caller trips it today, fixed anyway per ADR-054 #6",
	"maps.UpdateLayerInput":        "PUT .../layers/:lid (web + syncapi) — a SortOrder-only reorder push turned visibility and lock off for every layer. No shipped caller trips it today, fixed anyway per ADR-054 #6",
	"maps.UpdateMapInput":          "PUT /campaigns/:id/maps/:mid — a rename-only push unlinked the map's image and wiped its description; ImageID/Description were already *string and STILL blindly overwritten, because a plain pointer bound from JSON can't tell absent from null either",
	"timeline.UpdateTimelineInput": "PUT /campaigns/:id/timelines/:tid — fired on EVERY settings save, not just a narrow push: the request struct has no visibility_rules/description_html member at all, so both were unconditionally blanked and canUserView() treats an absent VisibilityRules as visible to everyone",
	"tags.UpdateTagInput":          "tagService.Update — the worst finding of the 2026-09-12 toggle-truth sweep (ADR-056): Color/DmOnly were plain value types, so ANY rename necessarily also sent DmOnly's zero value and turned a DM-only tag public",
	"tags.UpdateTagRequest":        "PUT /campaigns/:id/tags/:tagId — the wire-bound twin of UpdateTagInput above; same incident, same fix",
}

// governedFieldExceptions are value-typed fields deliberately left on a
// governed struct. Each needs a reason, and the reason has to be a fact.
var governedFieldExceptions = map[string]string{
	"entities.UpdateEntityInput.ImagePath": "INERT — entityService.Update never reads it. That is its own defect (campaign import believes it is applying image paths through this input and is not); booked in .ai/todo.md rather than fixed under a ruling that was about a different bug. It cannot clobber anything precisely because nothing reads it.",

	// ADR-054 #6: on each of these, Update only ever ASSIGNS Name when the
	// caller sends a non-empty value ("if input.Name != \"\" { … }"), so an
	// absent/blank name was already preserved before this sweep — Name was
	// never part of the blind-overwrite class these structs are fixed for.
	"maps.UpdateTokenInput.Name": "value-typed by choice: UpdateToken only assigns Name when it is non-empty, so an absent/blank name already preserved the stored one before this fix.",
	"maps.UpdateLayerInput.Name": "value-typed by choice: UpdateLayer only assigns Name when it is non-empty, so an absent/blank name already preserved the stored one before this fix.",
	// ADR-054 #4/#2/tags: on these three, Name is REQUIRED — Update validates
	// the merged name is non-empty and rejects the whole call with 400 when
	// it is blank, so an absent name fails LOUDLY instead of silently
	// overwriting. That is a different (and already-safe) failure mode from
	// the silent-clobber class the rest of each struct was fixed for.
	"maps.UpdateMapInput.Name":          "value-typed by choice: UpdateMap validates the merged name is non-empty and rejects the whole call with 400 when it is blank, so an absent name fails loudly rather than silently overwriting.",
	"timeline.UpdateTimelineInput.Name": "value-typed by choice: UpdateTimeline validates the merged name is non-empty and rejects the whole call with 400 when it is blank, so an absent name fails loudly rather than silently overwriting.",
	"tags.UpdateTagInput.Name":          "value-typed by choice: tagService.Update validates the merged name is non-empty and rejects the whole call with 400 when it is blank, so an absent name fails loudly rather than silently overwriting.",
	"tags.UpdateTagRequest.Name":        "value-typed by choice: the same required-name validation applies via UpdateTagInput.Name above — this is the wire-bound twin.",
}

// notYetSwept freezes the rest of the inventory. Nothing here was audited by
// sweep R4 — being on this list is a statement about what was LOOKED AT, not
// a claim that the struct is safe. Removing a name from here means the
// struct became contract-governed; adding one means a new update input
// shipped and its author decided it is not a partial update.
var notYetSwept = map[string]bool{
	"packages.UpdatePolicyInput":          true,
	"packages.UpdateRepoURLInput":         true,
	"bestiary.UpdatePublicationInput":     true,
	"timeline.UpdateEntityGroupInput":     true,
	"timeline.UpdateEventVisibilityInput": true,
	"addons.UpdateAddonInput":             true,
	"entities.UpdateLayoutPresetInput":    true,
	"entities.UpdateContentTemplateInput": true,
	"entities.UpdateEntityTypeInput":      true,
	"entities.UpdatePromptInput":          true,
	"maps.UpdateTokenPositionInput":       true,
	"campaigns.UpdateCampaignInput":       true,
	// CALV5 SALVAGE: these three came back with the recovered domain layer and
	// are listed again because they were never swept — pretending otherwise
	// would make this test lie. They are the ORIGINAL types recovered verbatim,
	// not V5 rewrites, so restoring the status quo is the honest entry.
	//
	// BUT NOTE THE OPPORTUNITY, because it will not come again this cheaply:
	// they currently have NO handler, NO service and NO repository — nothing
	// calls them. Sweeping them to patch.Field is therefore risk-free right
	// now, where doing it later means touching live write paths. V5 should
	// sweep them before giving them a handler, and delete these three lines.
	"calendar.UpdateEventVisibilityInput":    true,
	"calendar.UpdateCalendarVisibilityInput": true,
	"calendar.UpdateCalendarInput":           true,

	// The scanner widened from Update*Input to Update*Request on 2026-09-12
	// (ADR-056: "a guard that can see half the surface is a guard that
	// certifies the other half by silence" — the tags finding below is why).
	// These ten are what the wider scanner found on day one; NONE were
	// looked at by this sweep (task #6 fixed exactly the six named in
	// contractGoverned's ADR-054 block plus tags, and no others). Being on
	// this list is a statement about what was SEEN, not a claim of safety —
	// several of these (UpdateEntityRequest, UpdateEntityTypeRequest,
	// UpdateSMTPRequest) look exactly like the shape this ratchet exists to
	// catch and are good candidates for the next sweep.
	//
	// campaigns.* and entities.* entries here are additionally out of THIS
	// session's reach: campaigns/ is another agent's worktree right now, and
	// widening scope to entities/ mid-task was not what was asked.
	"campaigns.UpdateCampaignRequest":         true,
	"campaigns.UpdateRoleRequest":             true,
	"campaigns.UpdateSidebarConfigRequest":    true,
	"entities.UpdateEntityRequest":            true,
	"entities.UpdateEntityTypeRequest":        true,
	"entity_notes.UpdateNoteRequest":          true,
	"notes.UpdateNoteRequest":                 true,
	"posts.UpdatePostRequest":                 true,
	"relations.UpdateRelationMetadataRequest": true,
	"smtp.UpdateSMTPRequest":                  true,
}

type inputStruct struct {
	qualified string // "sessions.UpdateSessionInput"
	file      string
	fields    []inputField
}

type inputField struct {
	name       string
	typeString string
	line       int
}

// TestPartialUpdateContract_GovernedInputsCanRepresentAbsence is direction 1.
func TestPartialUpdateContract_GovernedInputsCanRepresentAbsence(t *testing.T) {
	found := scanUpdateInputs(t)
	for name, why := range contractGoverned {
		st, ok := found[name]
		if !ok {
			t.Errorf("%s is contract-governed but no longer exists in the tree (%s). If it was renamed, rename it here too; if it was deleted, delete this entry.", name, why)
			continue
		}
		for _, f := range st.fields {
			if presenceAware(f.typeString) {
				continue
			}
			key := name + "." + f.name
			if reason, exempt := governedFieldExceptions[key]; exempt {
				t.Logf("%s: value-typed by exception — %s", key, reason)
				continue
			}
			t.Errorf(
				"%s:%d — %s.%s is %s, a type with no ABSENT state.\n"+
					"  This struct is a PARTIAL update (%s), so a caller that omits this key must\n"+
					"  leave the stored value alone — and the service cannot do that if the field\n"+
					"  cannot tell 'absent' from the zero value. Use patch.Field[%s] (absent\n"+
					"  preserves, explicit null clears, a value replaces), or add a NAMED entry to\n"+
					"  governedFieldExceptions saying why this one is different.",
				st.file, f.line, name, f.name, f.typeString, why, f.typeString,
			)
		}
	}
}

// TestPartialUpdateContract_InventoryIsFrozen is direction 2.
func TestPartialUpdateContract_InventoryIsFrozen(t *testing.T) {
	found := scanUpdateInputs(t)
	var unclassified []string
	for name, st := range found {
		if _, ok := contractGoverned[name]; ok {
			continue
		}
		if notYetSwept[name] {
			continue
		}
		valueTyped := 0
		for _, f := range st.fields {
			if !presenceAware(f.typeString) {
				valueTyped++
			}
		}
		unclassified = append(unclassified, name+" ("+st.file+", "+itoa(valueTyped)+" value-typed field(s) today)")
	}
	sort.Strings(unclassified)
	for _, u := range unclassified {
		t.Errorf(
			"new update input %s is in neither list.\n"+
				"  Decide out loud: if it is a PARTIAL update, make its fields presence-aware and add\n"+
				"  it to contractGoverned; if it is a full replace, add it to notYetSwept. The count\n"+
				"  above is what the guard measured, not a verdict — a value-typed field is only a\n"+
				"  bug if some caller omits the key.", u,
		)
	}

	for name := range notYetSwept {
		if _, ok := found[name]; !ok {
			t.Errorf("notYetSwept lists %s, which no longer exists. Stale allowlist entries are how a ratchet stops ratcheting.", name)
		}
	}
	for name := range notYetSwept {
		if _, dup := contractGoverned[name]; dup {
			t.Errorf("%s is in BOTH lists; one of them is wrong", name)
		}
	}
}

// presenceAware reports whether a Go type can represent "the caller did not
// send this". Pointers, maps and slices carry nil; patch.Field carries an
// explicit presence bit.
func presenceAware(typeString string) bool {
	switch {
	case strings.HasPrefix(typeString, "*"),
		strings.HasPrefix(typeString, "[]"),
		strings.HasPrefix(typeString, "map["),
		strings.HasPrefix(typeString, "patch.Field["):
		return true
	}
	return false
}

// scanUpdateInputs parses every non-test .go file under internal/ and returns
// the Update*Input struct declarations keyed by "package.TypeName".
func scanUpdateInputs(t *testing.T) map[string]inputStruct {
	t.Helper()
	root := repoRoot(t)
	out := map[string]inputStruct{}
	fset := token.NewFileSet()

	err := filepath.Walk(filepath.Join(root, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "_templ.go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil // unparseable files are not this guard's business
		}
		pkg := file.Name.Name
		rel, _ := filepath.Rel(root, path)
		ast.Inspect(file, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok || !strings.HasPrefix(ts.Name.Name, "Update") ||
				(!strings.HasSuffix(ts.Name.Name, "Input") && !strings.HasSuffix(ts.Name.Name, "Request")) {
				return true
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return true
			}
			rec := inputStruct{qualified: pkg + "." + ts.Name.Name, file: rel}
			for _, f := range st.Fields.List {
				typeStr := exprString(f.Type)
				for _, nm := range f.Names {
					rec.fields = append(rec.fields, inputField{
						name: nm.Name, typeString: typeStr, line: fset.Position(nm.Pos()).Line,
					})
				}
				if len(f.Names) == 0 { // embedded
					rec.fields = append(rec.fields, inputField{
						name: typeStr, typeString: typeStr, line: fset.Position(f.Pos()).Line,
					})
				}
			}
			out[rec.qualified] = rec
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking internal/: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("found no Update*Input structs at all — the scanner is broken, and a broken scanner is a guard that passes by not looking")
	}
	return out
}

// exprString renders a type expression the way the source spells it, which is
// all presenceAware needs.
func exprString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return "*" + exprString(v.X)
	case *ast.SelectorExpr:
		return exprString(v.X) + "." + v.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprString(v.Elt)
	case *ast.MapType:
		return "map[" + exprString(v.Key) + "]" + exprString(v.Value)
	case *ast.IndexExpr: // patch.Field[string]
		return exprString(v.X) + "[" + exprString(v.Index) + "]"
	case *ast.InterfaceType:
		return "any"
	case *ast.FuncType:
		return "func"
	default:
		return "?"
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not find the repo root (no go.mod above the test's working directory)")
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
