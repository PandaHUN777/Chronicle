// default_visibility_test.go — the resolution table for
// CampaignSettings.DefaultVisibility.
//
// The setting existed for months with exactly ONE consumer: the web
// entity-creation form. Every other creation path in the repo — the shop
// widget's quick-create, the REST POST /entities, the batch-sync create and
// the bestiary creature import — built its CreateEntityInput without ever
// reading it, so a DM who set "DM Only" still got PUBLIC entities from
// Foundry sync, from the shop widget and from creature imports.
//
// This file pins the shared resolution those paths now share. The
// three-state parameter is load-bearing: collapsing "absent" into "false" is
// precisely how a private default got ignored, and collapsing "explicit
// false" into "absent" would take a client's deliberate is_private:false and
// override it.
package campaigns

import (
	"testing"

	"github.com/keyxmakerx/chronicle/internal/patch"
)

func TestCampaignSettings_DefaultsToPrivate(t *testing.T) {
	cases := []struct {
		name string
		vis  string
		want bool
	}{
		{"unset means everyone", "", false},
		{"dm_only starts hidden", "dm_only", true},
		{"private starts hidden", "private", true},
		{"explicit public value is not private", "public", false},
		{"unknown value is not private", "sideways", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := CampaignSettings{DefaultVisibility: tc.vis}
			if got := s.DefaultsToPrivate(); got != tc.want {
				t.Errorf("DefaultsToPrivate() = %v, want %v (DefaultVisibility=%q)", got, tc.want, tc.vis)
			}
		})
	}
}

func TestCampaignSettings_ResolveNewEntityPrivacy(t *testing.T) {
	cases := []struct {
		name      string
		vis       string
		requested patch.Field[bool]
		want      bool
	}{
		{
			name:      "absent under a dm_only default starts private",
			vis:       "dm_only",
			requested: patch.Absent[bool](),
			want:      true,
		},
		{
			name:      "absent under a private default starts private",
			vis:       "private",
			requested: patch.Absent[bool](),
			want:      true,
		},
		{
			name:      "absent under no default stays public",
			vis:       "",
			requested: patch.Absent[bool](),
			want:      false,
		},
		{
			// The distinction the whole three-state type exists for: a
			// client that deliberately says "public" is obeyed, and is
			// NOT quietly upgraded by the campaign default.
			name:      "explicit false under a dm_only default stays public",
			vis:       "dm_only",
			requested: patch.Of(false),
			want:      false,
		},
		{
			name:      "explicit true under no default is private",
			vis:       "",
			requested: patch.Of(true),
			want:      true,
		},
		{
			name:      "explicit true under a dm_only default is private",
			vis:       "dm_only",
			requested: patch.Of(true),
			want:      true,
		},
		{
			// Create has nothing stored to clear, so null has no "clear"
			// reading; it falls through to the campaign default rather
			// than silently meaning public.
			name:      "explicit null falls through to the campaign default",
			vis:       "dm_only",
			requested: patch.Null[bool](),
			want:      true,
		},
		{
			name:      "explicit null with no default is public",
			vis:       "",
			requested: patch.Null[bool](),
			want:      false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := CampaignSettings{DefaultVisibility: tc.vis}
			if got := s.ResolveNewEntityPrivacy(tc.requested); got != tc.want {
				t.Errorf("ResolveNewEntityPrivacy() = %v, want %v", got, tc.want)
			}
		})
	}
}
