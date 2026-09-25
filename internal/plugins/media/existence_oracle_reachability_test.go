// existence_oracle_reachability_test.go is a REACHABILITY test for the
// audit finding recorded at
// .ai/designs/2026-09-12-security-audit-findings.md (UNTESTED section):
// "internal/plugins/media/handler.go:272 — a private campaign's file
// answers 403 to an anonymous caller while an unknown id answers 404. Two
// distinguishable responses is an existence oracle." The doc also records
// a claimed mitigation — a defense-in-depth branch ~17 lines below the 403
// that deliberately returns not-found for exactly this case — but says
// that branch is UNREACHABLE whenever a signer is configured, which is
// always true in production (app/routes.go wires SetURLSigner whenever
// signingSecret != "", and the secret is auto-generated when unset — there
// is no deploy path that leaves h.signer nil).
//
// This test drives the REAL, exported Handler.Serve — the actual HTTP
// entrypoint GET /media/:id is routed to — never checkMediaAccess
// directly and never a reimplementation of the branch order. It asserts
// the CORRECT behaviour (an anonymous, unauthenticated caller must not be
// able to tell "this id exists but is private" from "no such id" via
// status code alone). A failing assertion here is proof the oracle is
// reachable in the shipped configuration, not a design opinion.
package media

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/keyxmakerx/chronicle/internal/apperror"
)

// --- Finding 1 companion: end-to-end Serve() check for the public-campaign
// carve-out (.ai/designs/2026-09-12-security-audit-findings.md, UNTESTED:
// "on a public campaign `allowUnsignedAccess` returns true for every
// file... Any media id in a public campaign is readable by the internet").
// The package already has unit coverage of this exact scenario at the
// checkMediaAccess level (viewer_binding_test.go:
// TestCheckMediaAccess_AnonymousPublicCampaign_DmOnlyPage_Denied) — this
// test does not duplicate that reasoning, it drives one level higher,
// through the real exported Handler.Serve, to confirm the fix holds at
// the actual HTTP entrypoint and not only in the internal method.

// idKeyedMediaService is a MediaService double whose GetByID resolves from
// a small map and falls back to the exact same 404 the real service
// returns for an unmapped id (service.go's GetByID -> repo.FindByID, see
// TestGetByID_NotFound in service_test.go). Everything else is inherited
// from fakeAccessMediaService (entity_visibility_access_test.go, same
// package) — Serve never reaches those methods for a request denied
// before c.File is called.
type idKeyedMediaService struct {
	fakeAccessMediaService
	files map[string]*MediaFile
}

func (f *idKeyedMediaService) GetByID(ctx context.Context, id string) (*MediaFile, error) {
	if mf, ok := f.files[id]; ok {
		return mf, nil
	}
	return nil, apperror.NewNotFound("media file not found")
}

// serveRequestAnonymous drives Handler.Serve for a completely anonymous
// caller — no session cookie, no query string at all (no `sig`/`expires`,
// the shape of a bare <img src="/media/ID"> request with no signed link).
// It returns the *apperror.AppError Serve hands back; app/app.go's real
// errorHandler maps AppError.Code straight onto the wire status
// (`code = appErr.Code`, pinned by error_handler_api_type_test.go), so
// Code here is exactly the HTTP status a real client would see.
func serveRequestAnonymous(t *testing.T, h *Handler, id string) *apperror.AppError {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/media/"+id, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(id)

	err := h.Serve(c)
	if err == nil {
		t.Fatalf("Serve(%q) with no session and no signature: expected a denial, got nil (would have served the file)", id)
	}
	appErr, ok := err.(*apperror.AppError)
	if !ok {
		t.Fatalf("Serve(%q): expected *apperror.AppError, got %T: %v", id, err, err)
	}
	return appErr
}

// TestServe_ExistenceOracle_PrivateCampaign_AnonymousCaller is the
// reachability verdict for handler.go:272. It wires a real signer
// (h.signer != nil), matching production exactly, and compares the status
// Serve returns for:
//   - a real file that exists and belongs to a private campaign, and
//   - an id that does not exist at all,
//
// both requested unsigned, anonymously. They must be indistinguishable.
func TestServe_ExistenceOracle_PrivateCampaign_AnonymousCaller(t *testing.T) {
	h := &Handler{
		signer:        NewURLSigner("test-secret"),
		memberChecker: &stubMemberChecker{},
		service: &idKeyedMediaService{
			files: map[string]*MediaFile{
				"file-exists-private": privateMediaFile(),
			},
		},
		entityVisibility: &fakeEntityVisibilityFilter{},
	}

	knownPrivate := serveRequestAnonymous(t, h, "file-exists-private")
	unknown := serveRequestAnonymous(t, h, "totally-nonexistent-id")

	t.Logf("known private file, unsigned, anonymous -> %d %s (%q)",
		knownPrivate.Code, knownPrivate.Type, knownPrivate.Message)
	t.Logf("unknown id,         unsigned, anonymous -> %d %s (%q)",
		unknown.Code, unknown.Type, unknown.Message)

	if knownPrivate.Code != unknown.Code {
		t.Errorf("existence oracle reachable: a private campaign's real file answers %d %q while an "+
			"unknown id answers %d %q for the identical anonymous, unsigned request shape. An "+
			"unauthenticated caller can distinguish \"this id exists in a private campaign\" from "+
			"\"no such id anywhere\" with one request. Root cause: checkMediaAccess's "+
			"`if !h.allowUnsignedAccess(c, file) { return apperror.NewForbidden(\"signed URL "+
			"required\") }` (handler.go, inside `if h.signer != nil`) fires and returns BEFORE the "+
			"defense-in-depth branch further down that would otherwise answer not-found for an "+
			"anonymous caller on a private campaign — that branch is unreached here because "+
			"h.signer != nil, which is always true in production.",
			knownPrivate.Code, knownPrivate.Type, unknown.Code, unknown.Type)
	}
}

// TestServe_PublicCampaign_DmOnlyArtwork_AnonymousCaller_Denied is the
// finding-1 reachability check, driven through the real Handler.Serve HTTP
// entrypoint rather than checkMediaAccess directly. A public campaign, a
// file referenced only by a dm_only entity, and a fully anonymous,
// unsigned request (the exact shape the audit finding describes: "any
// media id in a public campaign... readable by the internet"). Correct
// (ADR-058 decision 7) behaviour is denial.
func TestServe_PublicCampaign_DmOnlyArtwork_AnonymousCaller_Denied(t *testing.T) {
	campaignID := "camp-public-e2e"
	dmOnlyArt := &MediaFile{
		ID:               "file-dm-only-art",
		CampaignID:       &campaignID,
		CampaignIsPublic: boolPtr(true),
	}
	h := &Handler{
		signer:        NewURLSigner("test-secret"),
		memberChecker: &stubMemberChecker{},
		service: &idKeyedMediaService{
			fakeAccessMediaService: fakeAccessMediaService{
				findReferencesFn: func(ctx context.Context, campaignID, mediaID string) ([]MediaRef, error) {
					return []MediaRef{{EntityID: "ent-dm-only", EntityName: "Secret Villain", RefType: "image"}}, nil
				},
			},
			files: map[string]*MediaFile{"file-dm-only-art": dmOnlyArt},
		},
		// No entity is viewable to an anonymous (RoleNone) visitor.
		entityVisibility: &fakeEntityVisibilityFilter{viewableIDs: map[string]bool{}},
	}

	// serveRequestAnonymous itself fails the test (via t.Fatalf) if Serve
	// returns nil, i.e. if the file were actually served — that is the
	// primary assertion. What remains is confirming the denial is a real
	// client error, not an accidental 5xx masking the question.
	got := serveRequestAnonymous(t, h, "file-dm-only-art")
	t.Logf("public campaign, dm-only-referenced file, unsigned, anonymous -> %d %s (%q)",
		got.Code, got.Type, got.Message)

	if got.Code < http.StatusBadRequest || got.Code >= http.StatusInternalServerError {
		t.Errorf("expected a 4xx denial (403 or 404), got %d %s", got.Code, got.Type)
	}
}
