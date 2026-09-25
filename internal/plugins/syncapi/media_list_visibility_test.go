// media_list_visibility_test.go pins that GET /api/v1/campaigns/:id/media
// withholds its bulk listing (id, filename, size, signed URL for every
// media row) from a caller below Scribe: media_files carries no reference
// to the entity it illustrates, so there is no cheap "only what this
// caller can see" filter, and the same Scribe+ threshold the web app uses
// for campaign-wide media browsing applies here instead.
//
// These tests drive the REAL MediaAPIHandler.ListMedia — never a
// reimplementation of the filter — against a fake MediaService that
// returns a canned listing so the test can tell whether the handler
// actually withheld it.
package syncapi

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/keyxmakerx/chronicle/internal/plugins/campaigns"
	"github.com/keyxmakerx/chronicle/internal/plugins/media"
)

// fakeMediaSvcForList is a minimal media.MediaService double for ListMedia
// tests. Embeds the (nil) interface so unused methods panic loudly if the
// handler ever reaches them instead of silently returning zero values.
type fakeMediaSvcForList struct {
	media.MediaService
	listFn     func(ctx context.Context, campaignID string, page, perPage int) ([]media.MediaFile, int, error)
	listCalled bool
}

func (f *fakeMediaSvcForList) ListCampaignMedia(ctx context.Context, campaignID string, page, perPage int) ([]media.MediaFile, int, error) {
	f.listCalled = true
	if f.listFn != nil {
		return f.listFn(ctx, campaignID, page, perPage)
	}
	return nil, 0, nil
}

// oneSecretFile is what the campaign's media table actually holds: one
// file a Player should never learn the id or signed URL of in bulk.
func oneSecretFile() ([]media.MediaFile, int, error) {
	return []media.MediaFile{
		{ID: "dm-only-map-file", OriginalName: "dm-only-map.png", MimeType: "image/png"},
	}, 1, nil
}

// TestListMedia_SessionPlayerGetsNoRows is the red-first case: a Player's
// own session (the synthetic-key path RequireAuthOrAPIKey uses for in-app
// callers) must not receive the campaign's media listing.
func TestListMedia_SessionPlayerGetsNoRows(t *testing.T) {
	campSvc := &stubCampaignSvcForRole{getMemberFn: memberWithRole(campaigns.RolePlayer)}
	mediaSvc := &fakeMediaSvcForList{listFn: func(context.Context, string, int, int) ([]media.MediaFile, int, error) {
		return oneSecretFile()
	}}
	h := NewMediaAPIHandler(&stubSyncSvcForRole{}, mediaSvc)
	h.SetCampaignService(campSvc)

	key := &APIKey{ID: synthKeySessionID, CampaignID: "camp-1", UserID: "player-1"}
	c, rec := newRoleContext(key)

	if err := h.ListMedia(c); err != nil {
		t.Fatalf("ListMedia returned error: %v", err)
	}

	var body struct {
		Data  []map[string]any `json:"data"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 0 || body.Total != 0 {
		t.Errorf("a Player must receive no rows from the campaign media listing; got %d row(s), total=%d, body=%s",
			len(body.Data), body.Total, rec.Body.String())
	}
}

// TestListMedia_SessionScribeStillSeesListing is the positive control: a
// Scribe (the same threshold the web app's own media picker and browser
// already use for bulk campaign media access) must not be newly blocked.
func TestListMedia_SessionScribeStillSeesListing(t *testing.T) {
	campSvc := &stubCampaignSvcForRole{getMemberFn: memberWithRole(campaigns.RoleScribe)}
	mediaSvc := &fakeMediaSvcForList{listFn: func(context.Context, string, int, int) ([]media.MediaFile, int, error) {
		return oneSecretFile()
	}}
	h := NewMediaAPIHandler(&stubSyncSvcForRole{}, mediaSvc)
	h.SetCampaignService(campSvc)

	key := &APIKey{ID: synthKeySessionID, CampaignID: "camp-1", UserID: "scribe-1"}
	c, rec := newRoleContext(key)

	if err := h.ListMedia(c); err != nil {
		t.Fatalf("ListMedia returned error: %v", err)
	}
	if !mediaSvc.listCalled {
		t.Fatal("expected ListCampaignMedia to be called for a Scribe caller")
	}

	var body struct {
		Data  []map[string]any `json:"data"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 1 || body.Total != 1 {
		t.Errorf("expected the Scribe caller to see the 1 stubbed file, got %d row(s), total=%d", len(body.Data), body.Total)
	}
}

// TestListMedia_StoredBearerKeyStillSeesListing pins that no shipped
// integration (Foundry included) is affected: a real stored Bearer key
// always resolves to Owner (APIHandler.resolveRole's documented policy —
// keys are strictly Owner-minted), never a synthetic session key, so this
// fix cannot change behavior for any existing API-key integration.
func TestListMedia_StoredBearerKeyStillSeesListing(t *testing.T) {
	campSvc := &stubCampaignSvcForRole{getMemberFn: memberWithRole(campaigns.RolePlayer)} // even if the creator is now just a Player...
	mediaSvc := &fakeMediaSvcForList{listFn: func(context.Context, string, int, int) ([]media.MediaFile, int, error) {
		return oneSecretFile()
	}}
	h := NewMediaAPIHandler(&stubSyncSvcForRole{}, mediaSvc)
	h.SetCampaignService(campSvc)

	// ID != synthKeySessionID marks this as a real stored key.
	key := &APIKey{ID: 42, CampaignID: "camp-1", UserID: "foundry-key-owner"}
	c, rec := newRoleContext(key)

	if err := h.ListMedia(c); err != nil {
		t.Fatalf("ListMedia returned error: %v", err)
	}

	var body struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Total != 1 {
		t.Errorf("a stored Bearer key (Owner-resolved) must still see the listing; got total=%d", body.Total)
	}
}
