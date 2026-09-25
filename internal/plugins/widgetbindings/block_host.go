// DOM-id helpers for the re-render seam: a host's widget block lives inside
// a stable wrapper element so a bind/unbind can target it for an in-place
// HTMX swap; the picker fragment loads into a separate slot inside that
// wrapper.
package widgetbindings

import "sync"

// BlockHostID is the stable DOM id of a host's widget block — the swap target a
// binding mutation replaces (outerHTML), so the block re-renders in place with
// no full reload. The id is derived from the widget type + host id, which is
// unique per rendered block (one widget type per host).
func BlockHostID(widgetType, hostID string) string {
	return "widget-block-" + widgetType + "-" + hostID
}

// pickerSlotID is the DOM id of the inline panel the "Change…" affordance loads
// the picker fragment into (innerHTML). Nested inside the block host, so a
// successful mutation's outerHTML swap of the host also clears the open picker.
func pickerSlotID(widgetType, hostID string) string {
	return "widget-picker-" + widgetType + "-" + hostID
}

// pickerHostType is the affordance's host_type fallback: an unknown or empty
// host type resolves to the entity host, so a caller that forgets to name
// one still gets a valid picker query rather than one that reads no binding.
func pickerHostType(hostType string) string {
	if IsValidHostType(hostType) {
		return hostType
	}
	return HostTypeEntity
}

// --- the container-query opt-in -------------------------------------------

// inlineSizeHosts is the set of widget types whose BlockHost wrapper carries
// `container-type: inline-size`. It is an opt-in, not an unconditional
// declaration, so only the one widget type actually sized by container
// queries takes on containment; the other host types are unaffected.
// `container-type: inline-size` does NOT imply `contain: layout` in
// practice (measured in Chromium; despite what the spec text suggests), so
// it does not trap fixed/absolutely-positioned descendants such as the maps
// block's modals — but containment semantics differ between engines, so
// this stays scoped to the widget type that needs it.
//
// Written once at registration (before any request renders a block) and
// read on every render, hence the mutex rather than a bare map.
var (
	inlineSizeMu    sync.RWMutex
	inlineSizeHosts = map[string]bool{}
)

// DeclareInlineSizeHost marks a widget type whose block sizes itself with CSS
// container queries, so its BlockHost wrapper becomes a measured inline-size
// container. Called from the widget type's constructor at registration.
//
// A block that declares this takes on style + inline-size containment for its
// whole subtree — see inlineSizeHosts for what was and was not measured.
func DeclareInlineSizeHost(widgetType string) {
	inlineSizeMu.Lock()
	defer inlineSizeMu.Unlock()
	inlineSizeHosts[widgetType] = true
}

// blockHostStyle is the inline style BlockHost carries for widgetType: the
// container declaration for a declared host, empty for everything else.
//
// Inline rather than a stylesheet rule because widgetbindings owns no
// stylesheet, and a rule keyed on a plugin slug (`[data-widget-host="calendar"]`)
// would put a quoted plugin slug outside the owning plugin — the exact line
// tools/check-plugin-isolation.sh exists to catch.
func blockHostStyle(widgetType string) string {
	inlineSizeMu.RLock()
	defer inlineSizeMu.RUnlock()
	if inlineSizeHosts[widgetType] {
		return "container-type:inline-size"
	}
	return ""
}
