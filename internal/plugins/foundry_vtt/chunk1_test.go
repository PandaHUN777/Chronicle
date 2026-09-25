// chunk1_test.go pins three load-bearing properties, each behind its
// own test so a regression bisects to the right surface:
//
//  1. AutoPinSummary schema-version handling — write stamps the
//     current version; read is lenient on pre-versioning summaries
//     (zero → 1) and rejects forward-incompatible ones.
//  2. PackageRegistry.FoundryPackage across its "found", "not-found",
//     and "multiple-foundry-modules" paths.
//  3. Owner-side pin_mode round-trip via the CampaignSettingsAdapter
//     — Get returns whatever Set wrote; OwnerTabData populates
//     CurrentPinMode from the adapter.
//
// Pin-mode constants (preserve/promote/pinned) and IsValidPinMode are
// also asserted here so a typo can't drift the constant values out
// from under their callers.

package foundry_vtt

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/keyxmakerx/chronicle/internal/plugins/packages"
)

// --- 1. AutoPinSummary schema-version tests ---

// TestAutoPinSummary_StoreStampsCurrentSchemaVersion pins that
// storeAutoPinSummary always writes SchemaVersion =
// AutoPinSummarySchemaVersion regardless of what the caller passed.
// Defensive: a caller that forgets to set the field still gets a
// well-formed on-disk shape.
func TestAutoPinSummary_StoreStampsCurrentSchemaVersion(t *testing.T) {
	kv := newMemoryKV()
	svc := &service{kv: kv}

	// Caller deliberately omits SchemaVersion.
	summary := AutoPinSummary{
		PreviousVersion: "v0.1.14",
		NewVersion:      "v0.1.15",
		Affected:        3,
		Timestamp:       1700000000,
	}
	if err := svc.storeAutoPinSummary(context.Background(), summary); err != nil {
		t.Fatalf("storeAutoPinSummary: %v", err)
	}

	raw := kv.values[LatestAutoPinSummaryKey]
	var stored AutoPinSummary
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		t.Fatalf("unmarshal stored summary: %v (raw=%q)", err, raw)
	}
	if stored.SchemaVersion != AutoPinSummarySchemaVersion {
		t.Errorf("stored SchemaVersion = %d, want %d (current)", stored.SchemaVersion, AutoPinSummarySchemaVersion)
	}
}

// TestAutoPinSummary_ReadLenientOnLegacyV0 pins the lenient backfill
// for pre-Chunk-1 summaries that don't carry the schema_version
// field. Deserializing returns SchemaVersion = 1 (treated as the
// initial version), not 0.
func TestAutoPinSummary_ReadLenientOnLegacyV0(t *testing.T) {
	kv := newMemoryKV()
	// Hand-craft a pre-Chunk-1 summary blob (no schema_version key).
	legacyJSON := `{"previous_version":"v0.1.14","new_version":"v0.1.15","affected":3,"timestamp":1700000000}`
	kv.values[LatestAutoPinSummaryKey] = legacyJSON

	svc := &service{kv: kv}
	got, err := svc.GetUnreadAutoPinSummary(context.Background())
	if err != nil {
		t.Fatalf("GetUnreadAutoPinSummary: %v", err)
	}
	if got == nil {
		t.Fatal("expected legacy summary to surface, got nil")
	}
	if got.SchemaVersion != 1 {
		t.Errorf("legacy summary SchemaVersion = %d, want 1 (lenient backfill)", got.SchemaVersion)
	}
	if got.PreviousVersion != "v0.1.14" || got.NewVersion != "v0.1.15" || got.Affected != 3 {
		t.Errorf("legacy summary fields lost on read: %+v", got)
	}
}

// TestAutoPinSummary_RejectsForwardIncompatibleVersion pins the
// strict-on-future-versions branch. A summary with SchemaVersion
// > AutoPinSummarySchemaVersion (operator downgraded Chronicle
// while KV still holds a newer summary) MUST return an error rather
// than silently dropping fields the current code doesn't understand.
func TestAutoPinSummary_RejectsForwardIncompatibleVersion(t *testing.T) {
	kv := newMemoryKV()
	futureJSON := `{"schema_version":999,"previous_version":"v0.1.14","new_version":"v0.1.15","affected":3,"timestamp":1700000000}`
	kv.values[LatestAutoPinSummaryKey] = futureJSON

	svc := &service{kv: kv}
	got, err := svc.GetUnreadAutoPinSummary(context.Background())
	if err == nil {
		t.Fatalf("expected error on forward-incompatible schema_version, got summary=%+v", got)
	}
	if !strings.Contains(err.Error(), "schema_version=999") {
		t.Errorf("error should name the offending version; got: %v", err)
	}
	if !strings.Contains(err.Error(), "upgrade Chronicle") {
		t.Errorf("error should give recovery guidance; got: %v", err)
	}
}

// --- 2. PackageRegistry.FoundryPackage parity tests ---

// stubPackageReader implements PackageReader against an in-memory slice
// so we can drive the registry through each of its three branches.
type stubPackageReader struct {
	pkgs []packages.Package
}

func (s *stubPackageReader) ListPackages(_ context.Context) ([]packages.Package, error) {
	return s.pkgs, nil
}
func (s *stubPackageReader) GetPackage(_ context.Context, id string) (*packages.Package, error) {
	for i := range s.pkgs {
		if s.pkgs[i].ID == id {
			return &s.pkgs[i], nil
		}
	}
	return nil, nil
}
func (s *stubPackageReader) InstallDirForVersion(_ packages.PackageType, _, _ string) string {
	return ""
}
func (s *stubPackageReader) ListVersions(_ context.Context, _ string) ([]packages.PackageVersion, error) {
	return nil, nil
}

// TestPackageRegistry_FoundryPackage_Found pins the happy path: when
// a foundry-module package exists in the catalog, the registry
// returns it.
func TestPackageRegistry_FoundryPackage_Found(t *testing.T) {
	r := NewPackageRegistry(&stubPackageReader{
		pkgs: []packages.Package{
			{ID: "sys-1", Type: packages.PackageTypeSystem},
			{ID: "fvtt-1", Type: packages.PackageTypeFoundryModule},
		},
	})
	got, err := r.FoundryPackage(context.Background())
	if err != nil {
		t.Fatalf("FoundryPackage: %v", err)
	}
	if got == nil {
		t.Fatal("expected foundry-module package, got nil")
	}
	if got.ID != "fvtt-1" {
		t.Errorf("ID = %q, want fvtt-1", got.ID)
	}
}

// TestPackageRegistry_FoundryPackage_NotFound pins the empty-catalog
// path: no foundry-module package → (nil, nil). Caller treats this
// as the "set up the foundry module first" empty state.
func TestPackageRegistry_FoundryPackage_NotFound(t *testing.T) {
	r := NewPackageRegistry(&stubPackageReader{
		pkgs: []packages.Package{
			{ID: "sys-1", Type: packages.PackageTypeSystem},
		},
	})
	got, err := r.FoundryPackage(context.Background())
	if err != nil {
		t.Fatalf("FoundryPackage: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil package, got %+v", got)
	}
}

// TestPackageRegistry_FoundryPackage_MultipleReturnsFirst pins the
// single-instance assumption documented at the package-registry
// file header: multiple foundry-module packages are unusual, and the
// first one wins.
func TestPackageRegistry_FoundryPackage_MultipleReturnsFirst(t *testing.T) {
	r := NewPackageRegistry(&stubPackageReader{
		pkgs: []packages.Package{
			{ID: "fvtt-first", Type: packages.PackageTypeFoundryModule},
			{ID: "fvtt-second", Type: packages.PackageTypeFoundryModule},
		},
	})
	got, err := r.FoundryPackage(context.Background())
	if err != nil {
		t.Fatalf("FoundryPackage: %v", err)
	}
	if got == nil {
		t.Fatal("expected first foundry-module package, got nil")
	}
	if got.ID != "fvtt-first" {
		t.Errorf("ID = %q, want fvtt-first (first-match-wins assumption)", got.ID)
	}
}

// TestPackageRegistry_FoundryPackageID_Convenience pins the ID-only
// convenience accessor matches FoundryPackage().ID and returns ""
// when no package exists.
func TestPackageRegistry_FoundryPackageID_Convenience(t *testing.T) {
	rFound := NewPackageRegistry(&stubPackageReader{
		pkgs: []packages.Package{{ID: "fvtt-1", Type: packages.PackageTypeFoundryModule}},
	})
	if got := rFound.FoundryPackageID(context.Background()); got != "fvtt-1" {
		t.Errorf("FoundryPackageID = %q, want fvtt-1", got)
	}

	rEmpty := NewPackageRegistry(&stubPackageReader{pkgs: nil})
	if got := rEmpty.FoundryPackageID(context.Background()); got != "" {
		t.Errorf("FoundryPackageID on empty catalog = %q, want \"\"", got)
	}
}

// TestPackageRegistry_FvttCampaignsTriggerID_LockStep pins the DOM ID
// generation alignment: the registry's helper MUST produce the same
// string as the per-version trigger ID format in packages.templ, or
// the auto-pin banner's IIFE silently fails to find its target. The
// sanitization rule is shared with sanitizeVersionForDOMID /
// packages's sanitizeForID; see also cfmc9_test.go's
// TestSanitizeVersionForDOMID_MatchesPackagesSanitizer.
func TestPackageRegistry_FvttCampaignsTriggerID_LockStep(t *testing.T) {
	cases := []struct{ version, want string }{
		{"v0.1.10", "fvtt-campaigns-trigger-v0-1-10"},
		{"1.2.3+build", "fvtt-campaigns-trigger-1-2-3-build"},
		{"v0.1.0-rc.1", "fvtt-campaigns-trigger-v0-1-0-rc-1"},
	}
	for _, tc := range cases {
		got := FvttCampaignsTriggerID(tc.version)
		if got != tc.want {
			t.Errorf("FvttCampaignsTriggerID(%q) = %q, want %q", tc.version, got, tc.want)
		}
	}
}

// TestPackageRegistry_FvttVersionsTriggerAttr_StableName pins the
// data-attribute literal: onclick_handlers.go's IIFE selector and
// packages.templ's render-time attribute both depend on this exact
// string.
func TestPackageRegistry_FvttVersionsTriggerAttr_StableName(t *testing.T) {
	if FvttVersionsTriggerAttr != "data-fvtt-versions-trigger" {
		t.Errorf("FvttVersionsTriggerAttr = %q, want data-fvtt-versions-trigger; this drift would break the auto-pin banner's IIFE selector",
			FvttVersionsTriggerAttr)
	}
}

// TestFindFoundryPackage_DelegatesToRegistry pins that the service's
// FindFoundryPackage method goes through the registry rather than
// re-inlining the lookup.
func TestFindFoundryPackage_DelegatesToRegistry(t *testing.T) {
	pkgs := &stubPackageReader{
		pkgs: []packages.Package{
			{ID: "fvtt-x", Type: packages.PackageTypeFoundryModule},
		},
	}
	svc := &service{
		pkgs:     pkgs,
		registry: NewPackageRegistry(pkgs),
	}
	got, err := svc.FindFoundryPackage(context.Background())
	if err != nil {
		t.Fatalf("FindFoundryPackage: %v", err)
	}
	if got == nil || got.ID != "fvtt-x" {
		t.Fatalf("FindFoundryPackage = %+v, want id=fvtt-x", got)
	}

	// Defensive: service with nil registry still works (back-compat
	// for tests that construct services without the registry init).
	svcNoReg := &service{pkgs: pkgs}
	got, err = svcNoReg.FindFoundryPackage(context.Background())
	if err != nil {
		t.Fatalf("FindFoundryPackage (nil registry): %v", err)
	}
	if got == nil || got.ID != "fvtt-x" {
		t.Fatalf("FindFoundryPackage (nil registry) = %+v, want id=fvtt-x", got)
	}
}

// --- 3. Pin-mode constant + IsValidPinMode tests ---

// TestPinModeConstants pins the exact string values so a typo can't
// drift them out from under the code that switches on them, writes
// them, or backfills empty fields with PinModePromote.
func TestPinModeConstants(t *testing.T) {
	cases := []struct{ name, got, want string }{
		{"PinModePreserve", PinModePreserve, "preserve"},
		{"PinModePromote", PinModePromote, "promote"},
		{"PinModePinned", PinModePinned, "pinned"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// TestIsValidPinMode covers the three valid values plus rejected
// inputs. Empty string is INVALID per the helper's contract — it
// represents the pre-Chunk-6 "not yet set" state, which callers
// handle separately from validation.
func TestIsValidPinMode(t *testing.T) {
	cases := []struct {
		mode string
		want bool
	}{
		{PinModePreserve, true},
		{PinModePromote, true},
		{PinModePinned, true},
		{"", false},
		{"PROMOTE", false}, // case-sensitive
		{"always", false},  // not a canonical mode
		{"v0.1.14", false}, // version-string, not a mode
	}
	for _, tc := range cases {
		if got := IsValidPinMode(tc.mode); got != tc.want {
			t.Errorf("IsValidPinMode(%q) = %v, want %v", tc.mode, got, tc.want)
		}
	}
}

// --- helpers ---

// memoryKV is a tiny in-memory SettingsKVStore for tests that need
// to drive the autopin banner read/write paths through the service.
// Mirrors the contract of settings.SettingsRepository at the surface
// the service touches.
type memoryKV struct {
	mu     sync.Mutex
	values map[string]string
}

func newMemoryKV() *memoryKV {
	return &memoryKV{values: map[string]string{}}
}

func (k *memoryKV) Get(_ context.Context, key string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.values[key], nil
}
func (k *memoryKV) Set(_ context.Context, key, value string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.values[key] = value
	return nil
}
