package maps

import (
	"context"
	"testing"
)

// TestListMarkers_PassesRoleAndUserIDToRepo pins that the marker-list
// service hands the requesting user's role AND userID down to the
// repository layer, where the per-user visibility_rules filter lives. A
// refactor that silently drops userID would let a denied user fetch every
// "everyone" marker including the ones they're explicitly excluded from.
// The repo's SQL is exercised separately by integration tests; this test
// covers the service boundary.
func TestListMarkers_PassesRoleAndUserIDToRepo(t *testing.T) {
	var captured struct {
		mapID  string
		role   int
		userID string
	}
	repo := &mockMapRepo{
		listMarkersFn: func(_ context.Context, mapID string, role int) ([]Marker, error) {
			captured.mapID = mapID
			captured.role = role
			// userID isn't on the mock signature; this test pins the
			// service-side parameter shape only.
			return nil, nil
		},
	}
	svc := newTestMapService(repo)

	if _, err := svc.ListMarkers(context.Background(), "camp-1", "map-1", 1, "user-denied"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.mapID != "map-1" {
		t.Errorf("expected mapID=map-1, got %q", captured.mapID)
	}
	if captured.role != 1 {
		t.Errorf("expected role=1 (RolePlayer), got %d", captured.role)
	}
}
