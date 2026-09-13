package syncapi

import (
	"context"
	stderrors "errors"
	"testing"
)

// fakeKeyLister reports which campaigns own API keys.
type fakeKeyLister struct {
	ids []string
	err error
}

func (f fakeKeyLister) ListCampaignIDsWithKeys(context.Context) ([]string, error) {
	return f.ids, f.err
}

// TestReconcileAddonEnablement is the safety net under turning enforcement
// on. Enforcement defaults to DENIED — addons.IsEnabledForCampaign returns
// false when no campaign_addons row exists — and the only one-time backfill
// that ever wrote those rows (syncapi migration 003) ran once, long ago. Any
// campaign that minted its first key afterwards sits at "no row" and would be
// cut off the moment the gate went live. On the deployment this ships to that
// includes a live Foundry VTT sync.
//
// The table encodes the rule, and row 2 is the one that matters most: the
// reconciler runs at EVERY boot, so re-enabling on "enabled = 0" would mean a
// switched-off toggle lasts exactly until the next restart — handing back the
// decorative toggle this whole change removes. Hence it keys off "is there a
// row at all", not "is it on".
func TestReconcileAddonEnablement(t *testing.T) {
	cases := []struct {
		name string
		// campaigns owning at least one api_keys row.
		withKeys []string
		// campaigns that already have a campaign_addons row for sync-api.
		records map[string]bool
		// campaigns whose row says enabled=1 (a subset of records).
		enabled map[string]bool

		wantEnabled     int
		wantEnableCalls []string
	}{
		{
			name:            "campaign with keys and no recorded decision is enabled",
			withKeys:        []string{"camp-keys"},
			records:         map[string]bool{},
			enabled:         map[string]bool{},
			wantEnabled:     1,
			wantEnableCalls: []string{"camp-keys/" + SyncAPIAddonSlug},
		},
		{
			name:     "campaign that deliberately switched it OFF is left alone",
			withKeys: []string{"camp-off"},
			// A row exists and says disabled. That is an owner's decision.
			records:         map[string]bool{"camp-off": true},
			enabled:         map[string]bool{},
			wantEnabled:     0,
			wantEnableCalls: nil,
		},
		{
			name:            "campaign already enabled is not re-enabled",
			withKeys:        []string{"camp-on"},
			records:         map[string]bool{"camp-on": true},
			enabled:         map[string]bool{"camp-on": true},
			wantEnabled:     0,
			wantEnableCalls: nil,
		},
		{
			name:            "campaign with no keys is never opted in",
			withKeys:        nil,
			records:         map[string]bool{},
			enabled:         map[string]bool{},
			wantEnabled:     0,
			wantEnableCalls: nil,
		},
		{
			name:            "mixed estate: only the undecided key-owning campaign changes",
			withKeys:        []string{"camp-new", "camp-off", "camp-on"},
			records:         map[string]bool{"camp-off": true, "camp-on": true},
			enabled:         map[string]bool{"camp-on": true},
			wantEnabled:     1,
			wantEnableCalls: []string{"camp-new/" + SyncAPIAddonSlug},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gate := newFakeAddonGate()
			for id, has := range tc.records {
				gate.records[id] = has
			}
			for id, on := range tc.enabled {
				gate.enabled[id] = on
			}

			got, err := ReconcileAddonEnablement(context.Background(),
				fakeKeyLister{ids: tc.withKeys}, gate)
			if err != nil {
				t.Fatalf("ReconcileAddonEnablement: %v", err)
			}
			if got != tc.wantEnabled {
				t.Errorf("enabled count = %d, want %d", got, tc.wantEnabled)
			}
			if !stringsEqual(gate.enableCalls, tc.wantEnableCalls) {
				t.Errorf("enable calls = %v, want %v", gate.enableCalls, tc.wantEnableCalls)
			}
			// The disabled campaign must still read as disabled afterwards.
			if tc.records["camp-off"] && gate.enabled["camp-off"] {
				t.Error("reconciler re-enabled a campaign whose owner had switched Sync API off")
			}
		})
	}
}

// TestReconcileAddonEnablement_SecondRunIsANoOp pins idempotence against the
// shape the reconciler actually runs in: it fires on every boot, against the
// same estate, forever. The first pass writes the row; the second must see a
// decision on record and do nothing.
func TestReconcileAddonEnablement_SecondRunIsANoOp(t *testing.T) {
	gate := newFakeAddonGate()
	keys := fakeKeyLister{ids: []string{"camp-a", "camp-b"}}

	first, err := ReconcileAddonEnablement(context.Background(), keys, gate)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if first != 2 {
		t.Fatalf("first run enabled %d campaigns, want 2", first)
	}

	second, err := ReconcileAddonEnablement(context.Background(), keys, gate)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if second != 0 {
		t.Errorf("second run enabled %d campaigns, want 0 (must be idempotent)", second)
	}
	if len(gate.enableCalls) != 2 {
		t.Errorf("total enable calls = %d (%v), want 2 — the second pass must not write again",
			len(gate.enableCalls), gate.enableCalls)
	}
}

// TestReconcileAddonEnablement_ReportsErrors: the caller logs and carries on,
// but it can only do that if the error actually reaches it. A backfill that
// swallowed its own failure would leave campaigns refused with nothing in the
// log saying why.
func TestReconcileAddonEnablement_ReportsErrors(t *testing.T) {
	t.Run("key listing failure", func(t *testing.T) {
		_, err := ReconcileAddonEnablement(context.Background(),
			fakeKeyLister{err: stderrors.New("api_keys unavailable")}, newFakeAddonGate())
		if err == nil {
			t.Fatal("expected an error when the key listing fails")
		}
	})

	t.Run("addon store failure", func(t *testing.T) {
		gate := newFakeAddonGate()
		gate.err = stderrors.New("campaign_addons unavailable")
		_, err := ReconcileAddonEnablement(context.Background(),
			fakeKeyLister{ids: []string{"camp-a"}}, gate)
		if err == nil {
			t.Fatal("expected an error when the addon store fails")
		}
	})

	t.Run("nil dependency", func(t *testing.T) {
		if _, err := ReconcileAddonEnablement(context.Background(), nil, newFakeAddonGate()); err == nil {
			t.Error("expected an error for a nil key lister")
		}
		if _, err := ReconcileAddonEnablement(context.Background(), fakeKeyLister{}, nil); err == nil {
			t.Error("expected an error for a nil addon store")
		}
	})
}

func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
