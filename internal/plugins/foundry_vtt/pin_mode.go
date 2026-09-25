// pin_mode.go — pin-mode domain constants and normalization helpers.
//
// Three modes distinguish what AutoPinOnInstall does to each campaign
// when an admin installs a new module version. The choice lives on the
// per-campaign settings JSON alongside the `foundry_module_pin` key —
// see CampaignSettings.FoundryModulePinMode in the campaigns plugin's
// model.go.

package foundry_vtt

// PinMode constants name the three valid values for the
// `foundry_module_pin_mode` settings key. Empty string means "not yet
// set"; AutoPinOnInstall treats it the same as PinModePromote.
const (
	// PinModePreserve: when admin installs a new module version,
	// AutoPinOnInstall sets the campaign's pin to the *previous*
	// version, so the campaign keeps serving what it was already
	// serving. Operator must consciously bump from there.
	PinModePreserve = "preserve"

	// PinModePromote: when admin installs a new module version,
	// AutoPinOnInstall sets the campaign's pin to the *new* version.
	// The default for new campaigns.
	PinModePromote = "promote"

	// PinModePinned is set implicitly when the campaign has a
	// non-empty `foundry_module_pin` (the version string). The mode
	// key may be stored as `"pinned"` for clarity in admin UIs, but
	// the source of truth is still the pin field: if pin != "",
	// the campaign is pinned regardless of pin_mode.
	PinModePinned = "pinned"
)

// IsValidPinMode reports whether the given string is one of the
// three canonical pin modes. Empty string ("not yet set") is NOT
// valid here; callers that handle it should check separately.
func IsValidPinMode(mode string) bool {
	switch mode {
	case PinModePreserve, PinModePromote, PinModePinned:
		return true
	default:
		return false
	}
}
