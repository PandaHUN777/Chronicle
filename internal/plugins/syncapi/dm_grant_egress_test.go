// dm_grant_egress_test.go — ADR-057 slice 2 review finding (P1FIX dispatch).
//
// dm_grant_visibility_test.go's own header states the asymmetry by design:
// visibilityRoleFor promotes ONLY the CheckEntityAccess call inside GetEntity;
// `role` itself (h.resolveRole(c)'s return value) stays the caller's raw
// member role for everything that follows — GM-only/owner-only field
// stripping (stripEntitiesFieldsForEgress / FilterRestrictedFields) and
// inline-secret redaction (stripEntitySecretsForEgress). Nothing pinned that
// second half. A future "obvious cleanup" — e.g. reassigning
// `role = h.visibilityRoleFor(...)` once, right after CheckEntityAccess, so
// "the promoted role" is used consistently for the rest of the function —
// would silently ship GM notes and DM secret prose to every Co-DM, and every
// existing test in this package would stay green because they only assert
// the CanView/200 outcome (see TestGetEntity_CoDmCanReadDmOnlyEntity).
//
// This test drives the same Co-DM shape (Player + DM grant) through the real
// GetEntity handler and asserts the gm_only field value and the inline secret
// are both absent from the response. The stub CheckEntityAccess reproduces
// the real service's legacy default-mode rule (Scribe+ required for a
// private entity) exactly like dm_grant_visibility_test.go's stub — the
// promotion threshold under test is genuine, not an arbitrary sentinel — and
// neither stub encodes the stripping rule itself: FilterRestrictedFields and
// stripEntitySecretsForEgress are the real, unstubbed production functions.
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

// TestGetEntity_CoDmStillGetsGMFieldsAndSecretsStripped pins the second half
// of the ADR-057 asymmetry: a Co-DM (Player + DM grant) reading a dm_only
// entity is let in by the promoted CheckEntityAccess call, but the response
// must still have its gm_only field value and inline secret stripped, because
// `role` (Player) is deliberately left unpromoted for that decision.
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
