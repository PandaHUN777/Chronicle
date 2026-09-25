// Package permissions provides shared role constants and permission checks
// for code that cannot import campaigns (circular dependency). campaigns
// defines the canonical Role type; this package mirrors the levels as plain
// ints.
package permissions

// Role levels for campaign membership. These mirror the values in
// campaigns.Role but are plain ints to avoid circular imports.
const (
	RoleNone   = 0 // No membership (site admins viewing without joining)
	RolePlayer = 1 // Read access to permitted content
	RoleScribe = 2 // Create/edit access to notes, entities, events
	RoleOwner  = 3 // Full control, campaign ownership
)

// CanSeeDmOnly returns true if the role has permission to view dm_only content.
// Owners always can. Other roles can if they have been granted dm_only
// visibility via CampaignSettings.DmGrantIDs.
func CanSeeDmOnly(role int, dmGranted ...bool) bool {
	if role >= RoleOwner {
		return true
	}
	return len(dmGranted) > 0 && dmGranted[0]
}
