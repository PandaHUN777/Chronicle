package layouts

// addon_paths.go exposes the sidebar's addon destinations to code that must
// report where the navigation actually points.
//
// Reads from `addonURLMap` (app.templ), the same map `sidebarAddonLink`
// renders hrefs from, rather than a copy — so a diagnostic like
// campaign.surfaces can't drift from what the sidebar actually links to.

// AddonSidebarPath returns the campaign-relative path the sidebar links an
// addon to (e.g. "calendar" → "/apps/calendar"), or "" for a slug with no
// sidebar destination. The caller prefixes "/campaigns/<id>", exactly as
// sidebarAddonLink does.
func AddonSidebarPath(slug string) string { return addonURLMap[slug] }
