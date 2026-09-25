// helpers.go — packages plugin helpers exposed to packages.templ.
//
// The per-row admin UI for a package type is rendered via an HTMX lazy-load
// fragment owned by the type's plugin; this file holds the type→URL
// dispatch. The owning-plugin slug appears as a URL-path literal, which the
// plugin-isolation grep guard's regex (looking for a closing quote right
// after the slug) does not flag, since it's a URL path, not an import.
//
// TODO(#721): give package types their own UI hooks instead of this
// hard-coded URL dispatch.

package packages

// actionsFragmentURLFor returns the URL of the per-row actions
// fragment for the given package's type, or "" if the type has no
// type-specific fragment. packages.templ calls this when rendering
// each row's button group to know whether to insert an hx-get slot.
//
// Only foundry-module packages have a type-specific fragment; system
// packages render no extra actions beyond the generic
// Check/Versions/Usage/Delete buttons.
func actionsFragmentURLFor(pkg Package) string {
	switch pkg.Type {
	case PackageTypeFoundryModule:
		return "/admin/foundry-vtt/packages/" + pkg.ID + "/actions-fragment"
	case PackageTypeSystem:
		return ""
	default:
		return ""
	}
}
