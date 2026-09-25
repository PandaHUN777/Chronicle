// registration.go exposes the plugin's canonical slug for the App's
// lightweight plugin registry.
//
// No Registration() function is exported here: PluginRegistration lives in
// internal/app, which already imports this package, so a reciprocal import
// would cycle.

package smtp

// PluginSlug is the canonical identifier for the smtp plugin in the App's
// PluginRegistration registry. Its routes are mounted from inside
// admin.RegisterRoutes, not from here.
const PluginSlug = "smtp"

// PluginHealthKey is the identifier the database.PluginHealthRegistry uses
// for this plugin. Kept separate from PluginSlug so cross-package callers
// stay symmetric with plugins where the two values differ (e.g. foundry_vtt).
const PluginHealthKey = "smtp"
