// dm_grant_egress_test.go pins the second half of the ADR-057 asymmetry:
// visibilityRoleFor promotes only the CheckEntityAccess call in GetEntity, so
// a Co-DM (Player + DM grant) gets in, but `role` stays unpromoted for
// gm_only field stripping (FilterRestrictedFields) and inline-secret
// redaction (stripEntitySecretsForEgress) — those must still strip for them.
package syncapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/keyxmakerx/chronicle/internal/plugins/campaigns"
	"github.com/keyxmakerx/chronicle/internal/plugins/entities"
)

// stubEntitySvcForDmGrantEgress is stubEntitySvcForDmGrant (dm_grant_visibility
// _test.go) plus a real EntityType.Fields set, so FilterRestrictedFields has
// a gm_only field to strip.
type stubEntitySvcForDmGrantEgress struct {
	entities.EntityService
	entity *entities.Entity
	etype  *entities.EntityType
}

func (s *stubEntitySvcForDmGrantEgress) GetByID(_ context.Context, _ string) (*entities.Entity, error) {
	e := *s.entity
	return &e, nil
}

func (s *stubEntitySvcForDmGrantEgress) GetEntityTypeByID(_ context.Context, _ int) (*entities.EntityType, error) {
	et := *s.etype
	return &et, nil
}

func (s *stubEntitySvcForDmGrantEgress) CheckEntityAccess(_ context.Context, _ string, role int, _ string) (*entities.EffectivePermission, error) {
	if s.entity.IsPrivate && role < int(campaigns.RoleScribe) {
		return &entities.EffectivePermission{CanView: false}, nil
	}
	return &entities.EffectivePermission{CanView: true, CanEdit: role >= int(campaigns.RoleScribe)}, nil
}

// TestGetEntity_CoDmStillGetsGMFieldsAndSecretsStripped asserts a Co-DM
// reading a dm_only entity gets its gm_only field and inline secret stripped
// despite the promoted CanView check letting them read the entity.
func TestGetEntity_CoDmStillGetsGMFieldsAndSecretsStripped(t *testing.T) {
	secretHTML := `<p>The tavern is quiet.</p><span data-secret="true">The barkeep is a spy.</span>`
	ent := &entities.Entity{
		ID: "e1", CampaignID: "camp-1", EntityTypeID: 7, IsPrivate: true,
		EntryHTML: &secretHTML,
		FieldsData: map[string]any{
			"gm_notes":     "The BBEG is actually the mayor",
			"public_field": "known to all",
		},
	}
	et := &entities.EntityType{ID: 7, CampaignID: "camp-1", Fields: []entities.FieldDefinition{
		{Key: "gm_notes", GMOnly: true},
		{Key: "public_field"},
	}}

	h := NewAPIHandler(nil,
		&stubEntitySvcForDmGrantEgress{entity: ent, etype: et},
		&stubCampaignSvcForDmGrant{role: campaigns.RolePlayer, granted: true}, // Co-DM: Player + DM grant
		nil)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/campaigns/camp-1/entities/e1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id", "entityID")
	c.SetParamValues("camp-1", "e1")
	c.Set(apiKeyContextKey, &APIKey{ID: synthKeySessionID, CampaignID: "camp-1", UserID: "codm-1", IsActive: true})

	if err := h.GetEntity(c); err != nil {
		t.Fatalf("Co-DM reading dm_only entity: got %v, want nil (200) — CheckEntityAccess must still be promoted", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}

	body := rec.Body.String()
	if strings.Contains(body, "barkeep is a spy") {
		t.Errorf("inline GM secret leaked to a Co-DM (Player-tier secret stripping must not be skipped): body=%s", body)
	}

	var resp entities.Entity
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rec.Body)
	}
	if _, ok := resp.FieldsData["gm_notes"]; ok {
		t.Errorf("gm_only field value leaked to a Co-DM: fields_data=%v", resp.FieldsData)
	}
	if resp.FieldsData["public_field"] != "known to all" {
		t.Errorf("non-restricted field must survive stripping: fields_data=%v", resp.FieldsData)
	}
}
