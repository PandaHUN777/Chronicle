// assets.go provides cache-busted static asset URLs.
//
// Echo's `e.Static` serves assets via `http.ServeContent`, which emits
// Last-Modified but no Cache-Control, so browsers can hold a build-old copy
// for hours without revalidating — a deploy that adds a CSS class can render
// against a stale stylesheet that has never heard of it.
//
// Every asset URL emitted by a template routes through AssetURL, which
// appends `?v=<digest>`:
//
//   - `<digest>` is the first 10 hex chars of the file's SHA-256 when the
//     asset resolves on disk (or through a registered plugin FS), so a
//     deploy only busts the files that actually changed.
//   - Assets that can't be resolved fall back to a per-build token, coarser
//     but still busting on every deploy.
//
// Digests are computed once per path and cached for the process lifetime.
//
// The other half is middleware.StaticCache (internal/middleware/
// static_cache.go), which turns the versioned URLs into a real caching
// policy: long-lived immutable caching for `?v=`-carrying requests, forced
// revalidation otherwise.
package layouts

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

// StaticURLPrefix is the URL prefix every Chronicle static asset lives under.
// Exported so the contract test and the caching middleware agree on one value.
const StaticURLPrefix = "/static/"

// assetResolvers holds the filesystems AssetURL will hash against, in
// registration order. The on-disk `static/` root is seeded by default (it
// mirrors `e.Static("/static", "static")` in internal/app/app.go); plugins that
// serve from an embed FS register theirs via RegisterAssetFS.
var (
	assetMu        sync.RWMutex
	assetResolvers = []assetResolver{{prefix: StaticURLPrefix, fsys: os.DirFS("static")}}
	assetDigests   sync.Map // url path -> digest string
)

// assetResolver maps a URL prefix onto a filesystem rooted at that prefix.
type assetResolver struct {
	prefix string
	fsys   fs.FS
}

// RegisterAssetFS teaches AssetURL to hash assets served from an embedded
// filesystem (plugin static mounts). urlPrefix is the full URL prefix the FS is
// mounted at, e.g. "/static/plugins/calendar/"; fsys is rooted at that prefix.
//
// Registration is optional: an unregistered mount just falls through to the
// per-build token, which still busts on deploy. Later registrations win over
// earlier ones for overlapping prefixes, and the on-disk root stays last so a
// specific plugin FS is always consulted first.
func RegisterAssetFS(urlPrefix string, fsys fs.FS) {
	if fsys == nil || !strings.HasPrefix(urlPrefix, StaticURLPrefix) {
		return
	}
	if !strings.HasSuffix(urlPrefix, "/") {
		urlPrefix += "/"
	}
	assetMu.Lock()
	defer assetMu.Unlock()
	// Prepend: most-specific-registered wins, on-disk root remains the tail.
	assetResolvers = append([]assetResolver{{prefix: urlPrefix, fsys: fsys}}, assetResolvers...)
	// A newly-registered FS can change what a path resolves to, so drop any
	// digests already cached from the fallback path.
	assetDigests.Range(func(k, _ any) bool {
		if key, ok := k.(string); ok && strings.HasPrefix(key, urlPrefix) {
			assetDigests.Delete(k)
		}
		return true
	})
}

// AssetURL returns urlPath with a cache-busting `?v=<digest>` appended.
// Non-"/static/" inputs (external CDNs, data: URIs, anything already carrying
// a query string) are returned untouched.
func AssetURL(urlPath string) string {
	if !strings.HasPrefix(urlPath, StaticURLPrefix) || strings.ContainsAny(urlPath, "?#") {
		return urlPath
	}
	return urlPath + "?v=" + assetDigest(urlPath)
}

// assetDigest resolves (and memoizes) one asset's version token.
func assetDigest(urlPath string) string {
	if cached, ok := assetDigests.Load(urlPath); ok {
		return cached.(string)
	}
	digest := hashAsset(urlPath)
	if digest == "" {
		digest = buildToken()
	}
	assetDigests.Store(urlPath, digest)
	return digest
}

// hashAsset walks the registered resolvers for the first one whose prefix
// matches and whose filesystem actually holds the file. Returns "" when no
// resolver can produce the bytes — the caller falls back to the build token.
func hashAsset(urlPath string) string {
	assetMu.RLock()
	resolvers := assetResolvers
	assetMu.RUnlock()

	for _, r := range resolvers {
		if !strings.HasPrefix(urlPath, r.prefix) {
			continue
		}
		// fs.FS paths are slash-separated and never rooted or dot-prefixed.
		rel := path.Clean(strings.TrimPrefix(urlPath, r.prefix))
		if rel == "." || rel == "/" || strings.HasPrefix(rel, "..") {
			continue
		}
		f, err := r.fsys.Open(rel)
		if err != nil {
			continue
		}
		sum := sha256.New()
		_, err = io.Copy(sum, f)
		_ = f.Close()
		if err != nil {
			continue
		}
		return hex.EncodeToString(sum.Sum(nil))[:10]
	}
	return ""
}

// buildToken is the per-build fallback version, derived from the running
// executable's size + modification time — not process-start time, since a
// plain restart of the same binary must not invalidate every client's cache,
// but a deploy (new binary, new mtime) must.
var buildToken = sync.OnceValue(func() string {
	exe, err := os.Executable()
	if err != nil {
		return "dev"
	}
	info, err := os.Stat(exe)
	if err != nil {
		return "dev"
	}
	sum := sha256.Sum256([]byte(strconv.FormatInt(info.Size(), 10) + "-" + strconv.FormatInt(info.ModTime().UnixNano(), 10)))
	return hex.EncodeToString(sum[:])[:10]
})

// BuildToken exposes the per-BUILD fallback version for diagnostics. A `?v=`
// token equal to this is not a content hash — it means AssetURL could not
// resolve the file through any registered resolver (wrong working directory,
// an unregistered plugin FS, a path typo), so an operator asking "is my new
// CSS being served?" can tell a fallback from a real content-based bust.
func BuildToken() string { return buildToken() }
