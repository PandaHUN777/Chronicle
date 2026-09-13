package media

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"github.com/keyxmakerx/chronicle/internal/apperror"
	"github.com/keyxmakerx/chronicle/internal/middleware"
	"github.com/keyxmakerx/chronicle/internal/plugins/auth"
	"github.com/keyxmakerx/chronicle/internal/plugins/campaigns"
)

// MemberChecker verifies campaign membership and role without importing the
// full campaigns service. Implemented via an adapter in app/routes.go.
type MemberChecker interface {
	IsCampaignMember(campaignID, userID string) bool

	// MemberRole returns the caller's membership role in campaignID as an
	// int on campaigns.Role's own scale (RoleNone=0, RolePlayer=1,
	// RoleScribe=2, RoleOwner=3), or RoleNone if the user is not a member.
	// Used to gate campaign-scoped media writes (Upload) at the same
	// Scribe+ threshold every other campaign media write route already
	// enforces (routes.go: CampaignDeleteMedia is Owner, the media-picker
	// list and entity image endpoints are Scribe) — finding 1 of
	// .ai/designs/2026-09-12-security-audit-findings.md.
	MemberRole(campaignID, userID string) int

	// IsUserDmGranted reports whether the campaign Owner has granted the
	// user dm_only/co-DM visibility (mirrors campaigns.CampaignService.
	// IsUserDmGranted). ADR-058 decision on entity-scoped media access
	// requires the PROMOTED role (campaigns.CampaignContext.VisibilityRole()'s
	// formula: DM-granted → RoleOwner, else the raw MemberRole) wherever a
	// campaign context exists — a co-DM sees what the DM sees, operator
	// ruling `4df13033`. There is no *campaigns.CampaignContext on the
	// unscoped /media/:id route, so checkMediaAccess recomposes the same
	// formula from MemberRole + this call rather than re-deriving DM-grant
	// membership itself (that predicate lives in campaigns; this is a
	// signal, not a policy).
	IsUserDmGranted(campaignID, userID string) bool
}

// EntityVisibilityFilter narrows a set of entity IDs down to the ones a
// given viewer (role + userID) may see. Implemented by an adapter over the
// entities plugin's canonical EntityService.FilterViewableEntityIDs — the
// SAME method sessions, the relations widget, armory and npcs already call.
// Media does NOT get its own copy of the visibility predicate (that bug
// shipped three times already); it only ever forwards to this seam.
//
// ADR-058: a file at least one entity references is readable when at least
// one of those entities is visible to the viewer. Nil is a valid (if
// unwired) value — checkMediaAccess fails closed when it is nil, never
// silently skips the check.
type EntityVisibilityFilter interface {
	FilterViewableEntityIDs(ctx context.Context, campaignID string, entityIDs []string, role int, userID string) (map[string]bool, error)
}

// SecurityEventLogger records security events for the admin security dashboard.
// Implemented by the admin security service; wired after both are initialized.
type SecurityEventLogger interface {
	LogEvent(ctx context.Context, eventType, userID, actorID, ip, userAgent string, details map[string]any) error
}

// Handler handles HTTP requests for media operations.
type Handler struct {
	service          MediaService
	signer           *URLSigner
	memberChecker    MemberChecker
	securityLogger   SecurityEventLogger
	entityVisibility EntityVisibilityFilter
	// cache is optional (nil in tests and in any deploy without Redis
	// wired). Every read goes through a nil check and a miss/error falls
	// through to computing the decision fresh — ADR-058 Consequences:
	// "If the cache is unavailable the rule still applies; it gets
	// slower, not laxer."
	cache *redis.Client
}

// NewHandler creates a new media handler.
func NewHandler(service MediaService) *Handler {
	return &Handler{service: service}
}

// SetURLSigner sets the HMAC URL signer for signed URL generation and
// verification. Called during wiring in app/routes.go.
func (h *Handler) SetURLSigner(signer *URLSigner) {
	h.signer = signer
}

// SetMemberChecker sets the campaign membership checker for access control
// on private campaign media. Called during wiring in app/routes.go.
func (h *Handler) SetMemberChecker(checker MemberChecker) {
	h.memberChecker = checker
}

// SetEntityVisibilityFilter wires the entity-visibility seam (ADR-058
// decision 1). Called during wiring in app/routes.go, reusing the same
// entityVisibilityFilterAdapter sessions/npcs/armory already construct
// there — media does not get its own adapter type either.
func (h *Handler) SetEntityVisibilityFilter(f EntityVisibilityFilter) {
	h.entityVisibility = f
}

// SetCache wires the Redis client used to cache the ADR-058 entity-scoped
// access decision per (file, viewer). Called during wiring in app/routes.go
// with the same *redis.Client every other Redis-backed cache in this
// codebase uses (e.g. entities.Handler.SetCache) — nil is fine and simply
// disables caching (every request recomputes the decision, fail-safe).
func (h *Handler) SetCache(rdb *redis.Client) {
	h.cache = rdb
}

// SetSecurityLogger wires a security event logger for recording media events
// (uploads, deletes, quota failures). Called during wiring in app/routes.go.
func (h *Handler) SetSecurityLogger(logger SecurityEventLogger) {
	h.securityLogger = logger
}

// logSecurityEvent fires a security event if a logger is wired. Fire-and-forget
// so media operations are never blocked by logging failures.
func (h *Handler) logSecurityEvent(ctx context.Context, eventType, userID, actorID, ip, userAgent string, details map[string]any) {
	if h.securityLogger != nil {
		_ = h.securityLogger.LogEvent(ctx, eventType, userID, actorID, ip, userAgent, details)
	}
}

// Upload handles multipart file uploads (POST /media/upload).
func (h *Handler) Upload(c echo.Context) error {
	userID := auth.GetUserID(c)
	if userID == "" {
		return apperror.NewUnauthorized("authentication required")
	}

	// Authorize the write BEFORE touching the file: campaign_id is caller-
	// supplied form data, and until this check existed nothing verified the
	// caller belonged to that campaign at all — any authenticated user who
	// knew a campaign UUID could write into its media space, and the dedup
	// short-circuit in mediaService.Upload (FindByContentHash) would hand
	// back an EXISTING file's id/signed URL for that campaign on a byte
	// match (finding 1, .ai/designs/2026-09-12-security-audit-findings.md).
	// Scribe+ matches every other campaign-scoped media write route: the
	// picker list and entity image endpoints require Scribe, and
	// CampaignDeleteMedia requires Owner. A blank campaign_id (avatars,
	// backdrops — routed through /account/avatar and /campaigns/:id/backdrop
	// respectively, never this endpoint today) stays unscoped, as before;
	// this gate only applies once the caller names a campaign.
	if campaignID := c.FormValue("campaign_id"); campaignID != "" {
		if h.memberChecker == nil || h.memberChecker.MemberRole(campaignID, userID) < int(campaigns.RoleScribe) {
			slog.Warn("media upload: rejected, insufficient campaign role",
				slog.String("user_id", userID),
				slog.String("campaign_id", campaignID),
			)
			return apperror.NewForbidden("insufficient permissions to upload media to this campaign")
		}
	}

	file, err := c.FormFile("file")
	if err != nil {
		slog.Warn("media upload: FormFile error",
			slog.String("user_id", userID),
			slog.Any("error", err),
		)
		return apperror.NewBadRequest("no file provided")
	}

	src, err := file.Open()
	if err != nil {
		slog.Error("media upload: file open error",
			slog.String("user_id", userID),
			slog.Any("error", err),
		)
		return apperror.NewInternal(err)
	}
	defer func() { _ = src.Close() }()

	fileBytes, err := io.ReadAll(src)
	if err != nil {
		// MaxBytesReader returns a specific error when body exceeds limit.
		if err.Error() == "http: request body too large" {
			return apperror.NewBadRequest("file too large")
		}
		slog.Error("media upload: read error",
			slog.String("user_id", userID),
			slog.Any("error", err),
		)
		return apperror.NewInternal(err)
	}

	// Detect MIME type from file content if browser didn't provide one.
	declaredMime := file.Header.Get("Content-Type")
	if declaredMime == "" || declaredMime == "application/octet-stream" {
		declaredMime = http.DetectContentType(fileBytes)
	}

	input := UploadInput{
		CampaignID:   c.FormValue("campaign_id"),
		UploadedBy:   userID,
		OriginalName: file.Filename,
		MimeType:     declaredMime,
		FileSize:     int64(len(fileBytes)),
		UsageType:    c.FormValue("usage_type"),
		FileBytes:    fileBytes,
	}

	if input.UsageType == "" {
		input.UsageType = UsageAttachment
	}

	mediaFile, err := h.service.Upload(c.Request().Context(), input)
	if err != nil {
		// Log quota-exceeded errors as security events for admin visibility.
		if apperror.SafeCode(err) == http.StatusBadRequest &&
			(strings.Contains(err.Error(), "quota") || strings.Contains(err.Error(), "limit")) {
			h.logSecurityEvent(c.Request().Context(), "media.quota_exceeded",
				userID, "", c.RealIP(), c.Request().UserAgent(),
				map[string]any{
					"campaign_id": input.CampaignID,
					"mime_type":   input.MimeType,
					"size":        input.FileSize,
					"reason":      apperror.SafeMessage(err),
				})
		}
		return err
	}

	h.logSecurityEvent(c.Request().Context(), "media.uploaded",
		userID, "", c.RealIP(), c.Request().UserAgent(),
		map[string]any{
			"file_id":     mediaFile.ID,
			"campaign_id": input.CampaignID,
			"mime_type":   mediaFile.MimeType,
			"size":        mediaFile.FileSize,
		})

	// Return signed URLs if signer is available. Bound to the uploader's own
	// session (ADR-058 decision 6) -- userID is already known non-empty,
	// checked at the top of this handler.
	var url, thumbURL string
	if h.signer != nil {
		viewer := ViewerSession(userID)
		url = h.signer.Sign(mediaFile.ID, viewer, SignedURLTTL)
		if _, ok := mediaFile.ThumbnailPaths["300"]; ok {
			thumbURL = h.signer.SignThumb(mediaFile.ID, "300", viewer, SignedURLTTL)
		}
	} else {
		url = "/media/" + mediaFile.ID
		if _, ok := mediaFile.ThumbnailPaths["300"]; ok {
			thumbURL = "/media/" + mediaFile.ID + "/thumb/300"
		}
	}

	return c.JSON(http.StatusCreated, UploadResponse{
		ID:           mediaFile.ID,
		URL:          url,
		ThumbnailURL: thumbURL,
		MimeType:     mediaFile.MimeType,
		FileSize:     mediaFile.FileSize,
	})
}

// Serve serves a media file (GET /media/:id).
// Enforces HMAC-signed URL verification for campaign media and access
// control for private campaigns. Files without a campaign (avatars,
// backdrops) are served without signing.
func (h *Handler) Serve(c echo.Context) error {
	fileID := c.Param("id")
	fileID = strings.TrimSuffix(fileID, "/")

	file, err := h.service.GetByID(c.Request().Context(), fileID)
	if err != nil {
		return err
	}

	// Enforce access control.
	if err := h.checkMediaAccess(c, file, false, ""); err != nil {
		return err
	}

	filePath := h.service.FilePath(file)
	h.setSecurityHeaders(c, file)
	c.Response().Header().Set("Content-Type", file.MimeType)

	return c.File(filePath)
}

// allowedThumbSizes restricts thumbnail size parameter to known values,
// preventing the size from being used as an arbitrary map key.
var allowedThumbSizes = map[string]bool{"300": true, "800": true}

// ServeThumbnail serves a thumbnail of a media file (GET /media/:id/thumb/:size).
func (h *Handler) ServeThumbnail(c echo.Context) error {
	fileID := c.Param("id")
	size := c.Param("size")

	if !allowedThumbSizes[size] {
		return apperror.NewBadRequest("invalid thumbnail size")
	}

	file, err := h.service.GetByID(c.Request().Context(), fileID)
	if err != nil {
		return err
	}

	// Enforce access control (includes size in signature check).
	if err := h.checkMediaAccess(c, file, true, size); err != nil {
		return err
	}

	thumbPath := h.service.ThumbnailPath(file, size)
	h.setSecurityHeaders(c, file)

	return c.File(thumbPath)
}

// currentViewerIdentity resolves the PRESENTED viewer identity for a
// request verifying a signed media URL (ADR-058 decision 6): the session's
// user id when a Chronicle session cookie is present, or ViewerAnonymous
// when it is not. A cross-origin <img> request (no cookie -- Foundry's
// flow) always presents as anonymous here by construction; Verify itself
// decides whether that may still satisfy a link minted for ViewerAPIKey.
func currentViewerIdentity(c echo.Context) string {
	if userID := auth.GetUserID(c); userID != "" {
		return ViewerSession(userID)
	}
	return ViewerAnonymous
}

// checkMediaAccess enforces signed URL verification and private/public
// campaign access control. Returns nil if access is allowed, or an error to
// return to the client.
func (h *Handler) checkMediaAccess(c echo.Context, file *MediaFile, isThumb bool, thumbSize string) error {
	// Files without a campaign (avatars, backdrops) are public.
	if file.CampaignID == nil {
		return nil
	}

	fileID := file.ID
	expiresStr := c.QueryParam("expires")
	sig := c.QueryParam("sig")
	presentedViewer := currentViewerIdentity(c)

	// signatureValid is hoisted to the function scope so the defense-in-depth
	// block below can gate on it. A valid signed URL is itself proof of
	// authorization (HMAC-SHA256 over fileID:viewer:expires, with a
	// server-side secret) and the leak surface is time-bounded by the
	// `expires` claim the verifier enforces AND (ADR-058 decision 6) scoped
	// to the viewer it was minted for. Without this hoist, cross-origin
	// <img> requests from Foundry — which can't carry Chronicle session
	// cookies — would fail the defense-in-depth check, hit the framework's
	// 404 path, and redirect-loop to /login. C-MEDIA-SIGNED-URL-TRUST
	// (Bug #23).
	signatureValid := false

	// Check signed URL if signer is configured.
	if h.signer != nil {
		if expiresStr != "" && sig != "" {
			if isThumb {
				signatureValid = h.signer.VerifyThumb(fileID, thumbSize, presentedViewer, expiresStr, sig)
			} else {
				signatureValid = h.signer.Verify(fileID, presentedViewer, expiresStr, sig)
			}
		}

		if !signatureValid {
			// No valid signature. Fall back: allow if user is authenticated
			// and is a member of the file's campaign (graceful migration).
			if !h.allowUnsignedAccess(c, file) {
				return apperror.NewForbidden("signed URL required")
			}
		}
	}

	// Defense-in-depth: for private campaigns, ALSO require authenticated
	// campaign membership IF we did not validate a signed URL above. A
	// valid signed URL is itself proof of authorization — h.signer.Verify
	// rejects expired URLs and signatures minted for a different viewer,
	// and HMAC-SHA256 over a server-side secret means signatures cannot be
	// forged. Skipping the cookie+membership check when signatureValid lets
	// cross-origin <img> tags from Foundry resolve normally; without this,
	// the operator's maps redirect-loop to /login (Bug #23, 2026-05-19).
	if !signatureValid {
		userID := auth.GetUserID(c)

		// Site admins bypass every check below, on a public or private
		// campaign alike — unchanged from the pre-ADR-058 behaviour. This
		// ADR narrows what a campaign MEMBER (or, decision 7, an anonymous
		// visitor) may reach, not a site admin.
		if session := auth.GetSession(c); session != nil && session.IsAdmin {
			return nil
		}

		switch {
		case file.CampaignIsPublic != nil && !*file.CampaignIsPublic:
			// ADR-058 decisions 1-3: membership alone used to be enough for
			// a private campaign — ANY role, on ANY entity's page, however
			// hidden. checkEntityScopedAccess narrows that to "at least one
			// entity referencing this file is visible to this viewer"
			// whenever the file IS referenced by an entity, and falls back
			// to the untouched membership check when it is not (avatars,
			// backdrops, freshly uploaded files have no owning entity by
			// construction).
			//
			// A private campaign never reaches that check anonymously: an
			// anonymous caller is rejected here, before any entity-level
			// "visible to everyone" grant could apply. Campaign privacy
			// trumps an entity's own visibility setting — that setting is
			// meant for the campaign's members, not the open internet.
			if userID == "" {
				return apperror.NewNotFound("media file not found")
			}
			allowed, err := h.checkEntityScopedAccess(c.Request().Context(), file, userID)
			if err != nil {
				// FAIL CLOSED (ADR-058): a broken reference or visibility
				// lookup is never treated as "yes". Log the real reason,
				// return the same generic 404 as every other denial on this
				// path so a probing client can't tell "hidden" from "broken".
				slog.Error("media: entity-scoped access check failed; denying access",
					slog.String("file_id", file.ID),
					slog.String("campaign_id", *file.CampaignID),
					slog.Any("error", err),
				)
				return apperror.NewNotFound("media file not found")
			}
			if !allowed {
				return apperror.NewNotFound("media file not found")
			}

		case file.CampaignIsPublic != nil && *file.CampaignIsPublic:
			// ADR-058 decision 7: allowUnsignedAccess used to grant an
			// unsigned request to ANY file in a public campaign, so the
			// entire internet could read DM-only artwork given only an id.
			// Narrowed to decision 1's own predicate — reusing
			// checkEntityScopedAccess rather than a second copy of it,
			// exactly as the ADR calls for.
			//
			// userID may be "" here (an anonymous caller is allowed to
			// REACH this check on a public campaign, unlike the private
			// branch above): checkEntityScopedAccess's own
			// viewerVisibilityRole resolves an unknown/empty user to
			// RoleNone, which is "decision 1 with RoleNone" verbatim. An
			// authenticated member instead gets their real promoted role,
			// so a logged-in Player of a public campaign is not
			// artificially capped at anonymous. The no-references branch
			// (decision 3) asks the same membership question it always
			// has, which an anonymous caller always fails — the ADR's
			// consequence, not an oversight: an unreferenced file (avatar,
			// backdrop, fresh upload) is still openly reachable through a
			// freshly-minted, viewer-bound SIGNED url (decision 6 mints
			// one on every render, authenticated or not); this branch is
			// only the fallback for a request presenting no signature at
			// all.
			allowed, err := h.checkEntityScopedAccess(c.Request().Context(), file, userID)
			if err != nil {
				slog.Error("media: entity-scoped access check failed (public campaign); denying access",
					slog.String("file_id", file.ID),
					slog.String("campaign_id", *file.CampaignID),
					slog.Any("error", err),
				)
				return apperror.NewNotFound("media file not found")
			}
			if !allowed {
				return apperror.NewNotFound("media file not found")
			}
		}
	}

	return nil
}

// mediaAccessCacheTTL bounds how long an ADR-058 entity-scoped access
// decision is trusted before being recomputed. Short on purpose: it caps
// how long a viewer keeps reading a file after an owner hides the last
// page that made it visible — the ADR's "someone will lose access to an
// image they can see today" consequence should bite within about a
// minute, well inside the up-to-1-hour window a signed URL already
// tolerates today (decision 6, link-binding, is a separate slice). 60s
// also matches the short-lived cache entities.Handler already uses for
// its own hover-card lookups (Cache-Control: private, max-age=60): long
// enough that one page load's worth of repeated <img>/thumbnail requests
// from the same viewer hits it, short enough that "reload in a minute"
// after a permissions change is a real fix, not a euphemism.
const mediaAccessCacheTTL = 60 * time.Second

// mediaAccessCacheAllow/Deny are the cached payloads for the ADR-058
// entity-scoped decision. Plain strings (not "true"/"false" or JSON) so an
// unrecognized value — a future format change, or a stray key collision —
// is trivially detected and treated as a miss rather than misread.
const (
	mediaAccessCacheAllow = "1"
	mediaAccessCacheDeny  = "0"
)

// mediaAccessCacheKey scopes the cache to one file and one viewer exactly
// as ADR-058's Consequences call for ("caching the decision per (file,
// viewer)"). Only ever written for the has-references branch of
// checkEntityScopedAccess — the no-references (decision 3) branch is
// deliberately left uncached, see that function's comment.
func mediaAccessCacheKey(fileID, userID string) string {
	return "media:access:" + fileID + ":" + userID
}

// checkEntityScopedAccess applies ADR-058 decisions 1-3, and (reused
// verbatim per decision 7) the has-references half of decision 7's public-
// campaign rule. Two callers, two userID shapes:
//
//   - The private-campaign branch of checkMediaAccess always passes a real,
//     non-empty, authenticated userID — it rejects anonymous callers and
//     bypasses site admins before ever reaching here.
//   - The public-campaign branch (decision 7) may pass userID == "" for an
//     anonymous caller. viewerVisibilityRole resolves that to RoleNone via
//     the SAME MemberChecker calls it always makes (an unwired or
//     no-match lookup already resolves to "not a member" / RoleNone), so
//     nothing below needs a special case for it — an anonymous viewer
//     simply gets the same answer a very-not-a-member authenticated
//     viewer would.
//
// FAILS CLOSED THROUGHOUT: an error from the reference lookup, or from the
// visibility filter, denies access — it is never treated as "unreferenced"
// (which would grant plain membership access) or as "visible". Over-hiding
// costs an owner one reload; publishing a hidden page's artwork cannot be
// taken back.
func (h *Handler) checkEntityScopedAccess(ctx context.Context, file *MediaFile, userID string) (bool, error) {
	campaignID := *file.CampaignID

	// Cache check FIRST: on a hit this skips BOTH the reference lookup
	// (repository.go's entry_html LIKE scan) and the visibility filter
	// call entirely — the actual "lookup per image request" cost ADR-058
	// Consequences calls out. A miss (including redis.Nil, a connection
	// error, or an unrecognized payload) simply falls through to a fresh
	// computation: the cache is a speed optimization, never a source of
	// truth, so its unavailability can only make this slower, never
	// laxer.
	cacheKey := mediaAccessCacheKey(file.ID, userID)
	if h.cache != nil {
		switch cached, cerr := h.cache.Get(ctx, cacheKey).Result(); {
		case cerr == nil && cached == mediaAccessCacheAllow:
			return true, nil
		case cerr == nil && cached == mediaAccessCacheDeny:
			return false, nil
		}
	}

	refs, err := h.service.FindReferences(ctx, campaignID, file.ID)
	if err != nil {
		return false, fmt.Errorf("finding media references for access check: %w", err)
	}

	if len(refs) == 0 {
		// Decision 3: no entity references this file (an avatar, a
		// campaign backdrop, a just-uploaded file, or a file genuinely
		// never used anywhere) — campaign membership decides, as before.
		// Deliberately NOT cached: this was already the cheap path before
		// ADR-058, and caching it would add a staleness window a revoked
		// campaign member could ride out that decision 3 never asked for.
		//
		// An ABSENT checker refuses rather than admits. The previous line
		// here read `h.memberChecker == nil || …`, so an unwired dependency
		// silently granted every caller. That is the defect class ADR-053
		// named when the Sync API gate was built — "a security control that
		// silently no-ops when its dependency is missing" — and it is worth
		// correcting even though routes.go wires this unconditionally
		// (SetMemberChecker, one call site), because the whole value of a
		// fail-closed default is that it holds on the day someone adds a
		// second construction path and forgets. The upload gate a few
		// hundred lines above already refuses on nil; these two now agree.
		if h.memberChecker == nil {
			return false, fmt.Errorf("media: member checker not configured")
		}
		return h.memberChecker.IsCampaignMember(campaignID, userID), nil
	}

	// Decision 1: at least one referencing entity must be visible to this
	// viewer.
	if h.entityVisibility == nil {
		// Misconfiguration, not a policy outcome — the caller logs this
		// loudly and returns the same generic 404 as a real denial.
		return false, fmt.Errorf("media: entity visibility filter not configured")
	}

	entityIDs := make([]string, 0, len(refs))
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if seen[ref.EntityID] {
			continue
		}
		seen[ref.EntityID] = true
		entityIDs = append(entityIDs, ref.EntityID)
	}

	role := h.viewerVisibilityRole(campaignID, userID)

	viewable, err := h.entityVisibility.FilterViewableEntityIDs(ctx, campaignID, entityIDs, role, userID)
	if err != nil {
		return false, fmt.Errorf("filtering viewable entities for access check: %w", err)
	}

	allowed := false
	for _, ok := range viewable {
		if ok {
			allowed = true
			break
		}
	}

	if h.cache != nil {
		val := mediaAccessCacheDeny
		if allowed {
			val = mediaAccessCacheAllow
		}
		if err := h.cache.Set(ctx, cacheKey, val, mediaAccessCacheTTL).Err(); err != nil {
			// Cache write failure never affects the decision just made —
			// only the NEXT request's cost. Log and move on.
			slog.Warn("media: failed to cache entity-scoped access decision",
				slog.String("file_id", file.ID), slog.Any("error", err))
		}
	}

	return allowed, nil
}

// viewerVisibilityRole computes the SAME promotion
// campaigns.CampaignContext.VisibilityRole() applies (DM-granted →
// RoleOwner, else the raw member role) using the MemberChecker seam,
// since the unscoped /media/:id route never resolves a real
// *campaigns.CampaignContext (there is no :campaignId in that route to
// hang campaigns.RequireCampaignAccess middleware off of). A co-DM must
// see what the DM sees here too, exactly as it does for sessions,
// relations, armory and npcs (operator ruling, `4df13033`).
//
// A nil memberChecker (or any error surfaced as a plain false from
// IsUserDmGranted) resolves to the raw, un-promoted role — never a
// guessed promotion. That is the safe direction: it can only make a
// dm_only/custom-restricted entity LESS visible to this viewer, never
// more, which is exactly this ADR's fail-closed posture.
func (h *Handler) viewerVisibilityRole(campaignID, userID string) int {
	if h.memberChecker == nil {
		return int(campaigns.RoleNone)
	}
	if h.memberChecker.IsUserDmGranted(campaignID, userID) {
		return int(campaigns.RoleOwner)
	}
	return h.memberChecker.MemberRole(campaignID, userID)
}

// allowUnsignedAccess is the COARSE fallback gate when no valid signed URL
// is present -- it decides only whether checkMediaAccess proceeds to its
// fine-grained defense-in-depth switch at all, not the final answer. That
// switch (ADR-058 decisions 1-3 for private campaigns, decision 7 for
// public ones) runs regardless of what this function returns true for, and
// can still deny. Allows access for authenticated campaign members so old
// unsigned URLs still work during the migration period.
func (h *Handler) allowUnsignedAccess(c echo.Context, file *MediaFile) bool {
	// Public campaigns: let the request through to the decision-7 check
	// below rather than answering "signed URL required" outright. This
	// function no longer has the final word for a public campaign — it
	// used to (ADR-058 finding: "allowUnsignedAccess returns true for ANY
	// file whose campaign is public"), which is exactly what decision 7
	// narrows.
	if file.CampaignIsPublic != nil && *file.CampaignIsPublic {
		return true
	}

	// Authenticated user who is a campaign member.
	userID := auth.GetUserID(c)
	if userID != "" && file.CampaignID != nil {
		if h.memberChecker != nil && h.memberChecker.IsCampaignMember(*file.CampaignID, userID) {
			return true
		}
		// Site admins always have access.
		if session := auth.GetSession(c); session != nil && session.IsAdmin {
			return true
		}
	}

	return false
}

// setSecurityHeaders applies defense-in-depth headers to media responses.
func (h *Handler) setSecurityHeaders(c echo.Context, file *MediaFile) {
	resp := c.Response()

	// Force browser to respect declared Content-Type (prevents MIME sniffing).
	resp.Header().Set("X-Content-Type-Options", "nosniff")

	// Safe filename for Content-Disposition. Serve images inline with a
	// sanitized filename to prevent header injection.
	resp.Header().Set("Content-Disposition",
		fmt.Sprintf(`inline; filename="%s"`, sanitizeFilename(file.OriginalName)))

	// Prevent media URLs from being embedded as iframes.
	resp.Header().Set("X-Frame-Options", "DENY")

	// Restrictive CSP on media responses: no scripts, no styles.
	resp.Header().Set("Content-Security-Policy",
		"default-src 'none'; img-src 'self'; style-src 'none'; script-src 'none'")

	// Prevent referrer leakage of signed URLs or UUIDs.
	resp.Header().Set("Referrer-Policy", "no-referrer")

	// Cache control based on campaign privacy.
	if file.CampaignIsPublic != nil && !*file.CampaignIsPublic {
		// Private campaign media must not be cached by shared proxies.
		resp.Header().Set("Cache-Control", "private, no-store, max-age=0")
	} else {
		// Public/orphan media: cache aggressively (UUID filenames are immutable).
		resp.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
}

// sanitizeFilename strips characters that could be used for header injection,
// allowing only safe characters for Content-Disposition filenames.
func sanitizeFilename(name string) string {
	safe := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) ||
			r == '-' || r == '_' || r == '.' || r == ' ' {
			return r
		}
		return '_'
	}, name)
	if safe == "" {
		safe = "file"
	}
	return safe
}

// Info returns metadata about a media file (GET /media/:fileID/info).
// Requires authentication. Only the uploader or a site admin can view info.
func (h *Handler) Info(c echo.Context) error {
	userID := auth.GetUserID(c)
	if userID == "" {
		return apperror.NewUnauthorized("authentication required")
	}

	fileID := c.Param("fileID")
	file, err := h.service.GetByID(c.Request().Context(), fileID)
	if err != nil {
		return err
	}

	// Ownership check: only uploader or admin can see file info.
	session := auth.GetSession(c)
	if file.UploadedBy != userID && (session == nil || !session.IsAdmin) {
		return apperror.NewNotFound("media file not found")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"id":            file.ID,
		"original_name": file.OriginalName,
		"mime_type":     file.MimeType,
		"file_size":     file.FileSize,
		"usage_type":    file.UsageType,
		"created_at":    file.CreatedAt.Format(time.RFC3339),
		"thumbnails":    file.ThumbnailPaths,
	})
}

// Delete removes a media file (DELETE /media/:fileID).
// Only the uploader or a site admin can delete.
func (h *Handler) Delete(c echo.Context) error {
	userID := auth.GetUserID(c)
	if userID == "" {
		return apperror.NewUnauthorized("authentication required")
	}

	fileID := c.Param("fileID")
	file, err := h.service.GetByID(c.Request().Context(), fileID)
	if err != nil {
		return err
	}

	// Ownership check.
	session := auth.GetSession(c)
	if file.UploadedBy != userID && (session == nil || !session.IsAdmin) {
		return apperror.NewNotFound("media file not found")
	}

	if err := h.service.Delete(c.Request().Context(), fileID); err != nil {
		return err
	}

	var campaignID string
	if file.CampaignID != nil {
		campaignID = *file.CampaignID
	}
	h.logSecurityEvent(c.Request().Context(), "media.deleted",
		userID, "", c.RealIP(), c.Request().UserAgent(),
		map[string]any{
			"file_id":     fileID,
			"campaign_id": campaignID,
		})

	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Campaign-scoped media browser ---

// CampaignMediaPageData holds all data for rendering the campaign media browser.
type CampaignMediaPageData struct {
	Files     []MediaFile
	Stats     *CampaignMediaStats
	Total     int
	Page      int
	PerPage   int
	CSRFToken string
}

// CampaignMediaList returns paginated campaign media as JSON. Drives
// the media-picker widget (the slideout the operator opens via "Choose
// from campaign" next to file inputs). Distinct from CampaignMedia
// which renders the admin browser HTML page.
//
// Scribe+ at the route level. The handler also folds in URL strings
// (signed media URLs and thumbnails) so the widget doesn't need to
// know the signing scheme — keeps the picker decoupled from media-
// internal details.
//
// GET /campaigns/:id/media/list?page=N&perPage=M
func (h *Handler) CampaignMediaList(c echo.Context) error {
	cc := campaigns.GetCampaignContext(c)
	if cc == nil {
		return apperror.NewMissingContext()
	}
	ctx := c.Request().Context()

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.QueryParam("perPage"))
	if perPage <= 0 || perPage > 100 {
		perPage = 24
	}

	files, total, err := h.service.ListCampaignMedia(ctx, cc.Campaign.ID, page, perPage)
	if err != nil {
		return err
	}

	// Bind every URL this response mints to the caller's own session
	// (ADR-058 decision 6). This route requires auth.RequireAuth
	// (routes.go), so userID is expected non-empty; falling back to
	// ViewerAnonymous if it somehow is not only narrows who can use the
	// resulting link, never widens it.
	viewer := ViewerAnonymous
	if userID := auth.GetUserID(c); userID != "" {
		viewer = ViewerSession(userID)
	}

	type mediaListItem struct {
		ID           string    `json:"id"`
		OriginalName string    `json:"original_name"`
		MimeType     string    `json:"mime_type"`
		FileSize     int64     `json:"file_size"`
		URL          string    `json:"url"`
		ThumbnailURL string    `json:"thumbnail_url,omitempty"`
		CreatedAt    time.Time `json:"created_at"`
	}

	out := make([]mediaListItem, 0, len(files))
	for i := range files {
		f := &files[i]
		item := mediaListItem{
			ID:           f.ID,
			OriginalName: f.OriginalName,
			MimeType:     f.MimeType,
			FileSize:     f.FileSize,
			CreatedAt:    f.CreatedAt,
		}
		// Same signed-URL logic the upload handler uses — keep the
		// picker's URLs viewer-bound and short-lived (SignedURLTTL),
		// fall back to unsigned when no signer is configured (dev / tests).
		if h.signer != nil {
			item.URL = h.signer.Sign(f.ID, viewer, SignedURLTTL)
			if thumb := pickThumbnail(f); thumb != "" {
				item.ThumbnailURL = h.signer.SignThumb(f.ID, thumb, viewer, SignedURLTTL)
			}
		} else {
			item.URL = "/media/" + f.ID
			if thumb := pickThumbnail(f); thumb != "" {
				item.ThumbnailURL = "/media/" + f.ID + "/thumb/" + thumb
			}
		}
		out = append(out, item)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"items":    out,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// pickThumbnail picks the smallest available thumbnail size key for a
// file. Picker UIs prefer small thumbs to keep the slideout snappy.
func pickThumbnail(f *MediaFile) string {
	if !f.IsImage() || len(f.ThumbnailPaths) == 0 {
		return ""
	}
	// Prefer 300px if present; fall back to anything else.
	if _, ok := f.ThumbnailPaths["300"]; ok {
		return "300"
	}
	for size := range f.ThumbnailPaths {
		return size
	}
	return ""
}

// CampaignMedia renders the campaign media browser page (GET /campaigns/:id/media).
func (h *Handler) CampaignMedia(c echo.Context) error {
	cc := campaigns.GetCampaignContext(c)
	if cc == nil {
		return apperror.NewNotFound("campaign not found")
	}

	ctx := c.Request().Context()

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	perPage := 24

	files, total, err := h.service.ListCampaignMedia(ctx, cc.Campaign.ID, page, perPage)
	if err != nil {
		return err
	}

	stats, err := h.service.GetCampaignStats(ctx, cc.Campaign.ID)
	if err != nil {
		return err
	}

	data := CampaignMediaPageData{
		Files:     files,
		Stats:     stats,
		Total:     total,
		Page:      page,
		PerPage:   perPage,
		CSRFToken: middleware.GetCSRFToken(c),
	}

	if middleware.IsHTMX(c) {
		return middleware.Render(c, http.StatusOK, CampaignMediaFragment(cc, data))
	}
	return middleware.Render(c, http.StatusOK, CampaignMediaPage(cc, data))
}

// CampaignDeleteMedia handles deletion of a campaign media file
// (DELETE /campaigns/:id/media/:mid).
func (h *Handler) CampaignDeleteMedia(c echo.Context) error {
	cc := campaigns.GetCampaignContext(c)
	if cc == nil {
		return apperror.NewNotFound("campaign not found")
	}

	mediaID := c.Param("mid")
	if err := h.service.DeleteCampaignMedia(c.Request().Context(), cc.Campaign.ID, mediaID); err != nil {
		return err
	}

	h.logSecurityEvent(c.Request().Context(), "media.deleted",
		auth.GetUserID(c), "", c.RealIP(), c.Request().UserAgent(),
		map[string]any{
			"file_id":     mediaID,
			"campaign_id": cc.Campaign.ID,
		})

	// Redirect back to media page for HTMX and standard requests.
	c.Response().Header().Set("HX-Redirect", fmt.Sprintf("/campaigns/%s/media", cc.Campaign.ID))
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

// CampaignMediaRefs returns an HTMX fragment showing which entities reference
// a media file (GET /campaigns/:id/media/:mid/refs).
func (h *Handler) CampaignMediaRefs(c echo.Context) error {
	cc := campaigns.GetCampaignContext(c)
	if cc == nil {
		return apperror.NewNotFound("campaign not found")
	}

	mediaID := c.Param("mid")
	refs, err := h.service.FindReferences(c.Request().Context(), cc.Campaign.ID, mediaID)
	if err != nil {
		return err
	}

	return middleware.Render(c, http.StatusOK, MediaRefsFragment(cc, mediaID, refs))
}
