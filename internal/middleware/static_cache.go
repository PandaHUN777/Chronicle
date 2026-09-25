package middleware

import (
	"strings"

	"github.com/labstack/echo/v4"
)

// StaticCache sets an explicit Cache-Control policy on static asset
// responses. Echo's `e.Static` serves through `http.ServeContent`, which
// emits Last-Modified but no Cache-Control, so browsers fall back to
// heuristic freshness and can reuse a long-untouched file for hours without
// revalidating — the "site looks old after a deploy" bug. See
// layouts.AssetURL for the `?v=` cache-busting token this depends on.
//
// The `?v=` token picks between two policies:
//
//   - `?v=` present → the URL is content-addressed, so the bytes behind it
//     can never change: cache hard and forever (`immutable`).
//   - no `?v=` → a hand-typed URL, a legacy link, or a direct fetch. Allow
//     caching but force revalidation on every use, so a stale copy is at
//     worst one conditional request (304, no body) away from correct — an
//     asset whose template we forgot to convert degrades to "always
//     revalidated", never "silently stale".
//
// Applied as a DEFAULT before the handler runs: requests outside urlPrefix
// pass straight through, and any handler that sets its own Cache-Control
// afterwards wins.
//
// Registered globally rather than on a route group because static assets
// are mounted in two places — the on-disk `/static` root and each plugin's
// embed FS under `/static/plugins/<slug>` — and one prefix check covers both.
func StaticCache(urlPrefix string) echo.MiddlewareFunc {
	const (
		immutablePolicy  = "public, max-age=31536000, immutable"
		revalidatePolicy = "public, max-age=0, must-revalidate"
	)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !strings.HasPrefix(c.Request().URL.Path, urlPrefix) {
				return next(c)
			}
			h := c.Response().Header()
			if h.Get(echo.HeaderCacheControl) == "" {
				if c.QueryParam("v") != "" {
					h.Set(echo.HeaderCacheControl, immutablePolicy)
				} else {
					h.Set(echo.HeaderCacheControl, revalidatePolicy)
				}
			}
			return next(c)
		}
	}
}
