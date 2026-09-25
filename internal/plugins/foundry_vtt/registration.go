// registration.go exposes the plugin's canonical slug for the App's
// lightweight plugin registry.
//
// This file holds only the PluginSlug const; there is intentionally
// no Registration() function here because PluginRegistration lives in
// internal/app, which already imports this package, and a reciprocal
// import would create a cycle.

package foundry_vtt

// PluginSlug is the canonical EXTERNAL identifier for the foundry_vtt
// plugin in the App's PluginRegistration registry. Shares its string
// value with ModuleSource by coincidence — they are conceptually
// distinct and should not be collapsed.
const PluginSlug = "foundry-vtt"

// PluginHealthKey is the INTERNAL identifier the database.PluginHealthRegistry
// uses to track this plugin's schema health. The registry was wired with
// Go-package-name keys (underscore form) before the slug convention was
// formalized, so this differs from PluginSlug. Exported so cross-package
// callers (e.g. the App's HealthCheck closures registered against the
// PluginRegistration registry) don't have to interpolate the literal.
const PluginHealthKey = "foundry_vtt"
