// Package foundry_vtt provides the Foundry VTT integration plugin.
//
// const.go centralizes the plugin's identifier constants. Per T-B2 (plugin
// isolation), references to these identifiers from outside this package
// should import these constants rather than carry the string literal.

package foundry_vtt

// ModuleSource is the WebSocket Source identifier the Foundry module
// self-reports on its WS upgrade URL (`?client=foundry-module`) and the
// Hub stores on Client.Source to drive Foundry-presence tracking.
//
// Distinct from packages.PackageTypeFoundryModule despite the shared
// string value — different roles, don't collapse them.
const ModuleSource = "foundry-module"
