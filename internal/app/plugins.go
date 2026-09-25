// plugins.go declares the plugin registration model: a metadata-only
// registry that each plugin contributes to at App startup. Every field
// but Slug is optional, so a plugin registers only what it uses.

package app

import (
	"io/fs"

	"github.com/keyxmakerx/chronicle/internal/templates/layouts"
)

// PluginRegistration is the per-plugin entry in the App's registry.
// Each plugin contributes exactly one entry, populated inline from
// RegisterRoutes at the plugin's setup point.
type PluginRegistration struct {
	// Slug is the canonical identifier for this plugin. MUST match the
	// owning plugin's exported PluginSlug const so the lookup is
	// symmetric (slug → plugin code, plugin code → slug).
	Slug string

	// HealthCheck is an optional callback returning nil if the plugin is
	// operational, or an error if not. May be nil — not every plugin has
	// a schema or other failable health signal.
	HealthCheck func() error

	// StaticFS is an optional embedded filesystem of plugin-owned static
	// assets. When non-nil, App.mountPluginStatic() registers it with
	// Echo at /static/plugins/<Slug>/. Use echo.MustSubFS(<embed.FS>,
	// "static") at the registration site so the URL doesn't double the
	// "static" dir. nil = no static assets.
	StaticFS fs.FS
}

// registerPlugin appends a registration entry to the App's registry.
// Package-private — called only from RegisterRoutes at each plugin's
// setup point.
func (a *App) registerPlugin(p PluginRegistration) {
	a.registeredPlugins = append(a.registeredPlugins, p)
}

// RegisteredPlugins returns a copy of the App's registry slice, so
// callers can't mutate the App's internal slice.
func (a *App) RegisteredPlugins() []PluginRegistration {
	out := make([]PluginRegistration, len(a.registeredPlugins))
	copy(out, a.registeredPlugins)
	return out
}

// mountPluginStatic registers each registered plugin's StaticFS with Echo
// at /static/plugins/<slug>/. Called from RegisterRoutes after all plugin
// registrations have happened. Plugins with StaticFS == nil are skipped.

// pluginStaticPrefix is the single definition of where a plugin's embedded
// assets are served, so the mount and the host.embedded diagnostic that
// reports it cannot drift apart.
func pluginStaticPrefix(slug string) string { return "/static/plugins/" + slug }

func (a *App) mountPluginStatic() {
	for _, p := range a.registeredPlugins {
		if p.StaticFS == nil {
			continue
		}
		prefix := pluginStaticPrefix(p.Slug)
		// Register the embed FS so layouts.AssetURL can content-hash plugin
		// assets like on-disk ones, instead of busting all of them per deploy.
		layouts.RegisterAssetFS(prefix+"/", p.StaticFS)
		a.Echo.StaticFS(prefix, p.StaticFS)
	}
}
