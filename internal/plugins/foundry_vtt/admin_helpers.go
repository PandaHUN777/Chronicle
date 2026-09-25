package foundry_vtt

import (
	"fmt"
	"time"
)

// relativeTime renders a coarse "Xm ago / Xh ago / Xd ago" string
// for the admin "Campaigns Using v0.1.5" panel's last-active column.
//
// Local helper, following the codebase pattern of per-plugin
// formatting helpers. Coarse buckets on purpose — admins want to see
// at a glance which campaigns are dormant, not compare 30m vs 35m.
func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
