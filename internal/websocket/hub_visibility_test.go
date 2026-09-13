// hub_visibility_test.go — S1: the WebSocket fan-out must honor the same
// per-user visibility_rules (allowed_users / denied_users) the HTTP list
// path already enforces (maps/repository.go's ListMarkers non-owner
// branch, maps/drawing_repository.go's ListDrawings), not just the binary
// RequiresDM/dm_only gate.
//
// Before this fix: a marker or drawing explicitly denied to a specific
// player was still broadcast to that player's browser in full on every
// create/update, because the publisher (routes.go's
// mapEventPublisherAdapter) passed only IsDMOnly()/dm_only as the
// audience, and the hub's one gate (this file's package, hub.go) was
// RequiresDM alone. A "specific"-visibility marker isn't dm_only at all,
// so RequiresDM was false for it and it went to literally everyone.
//
// This test drives the REAL Hub: NewHub(), Run() started in a goroutine,
// real *Client values registered through the hub's own (unexported,
// in-package) registration channel — the same code path RegisterClient
// uses, minus the actual network conn, which the audience gate never
// touches — and Broadcast() pushed through the real broadcast channel.
// Nothing here is a fake or a fixture standing in for the hub: if hub.go's
// broadcast loop were reverted to check only RequiresDM, these tests fail
// with the denied/unlisted client's send channel holding a message it
// must never receive.
package websocket

import (
	"testing"
	"time"

	"github.com/keyxmakerx/chronicle/internal/permissions"
)

// newVisibilityTestHub starts a real Hub's event loop for the test to
// drive. The goroutine outlives the test (Hub has no shutdown hook) —
// harmless for a short-lived test binary.
func newVisibilityTestHub(t *testing.T) *Hub {
	t.Helper()
	h := NewHub()
	go h.Run()
	return h
}

// registerTestClient builds a Client and pushes it through the hub's real
// registration channel, so Run()'s own registration branch (the map
// insert under h.mu) executes exactly as it would for a live connection.
// It bypasses RegisterClient only because that function asserts its conn
// argument to a concrete *gorilla websocket.Conn — a real network socket
// buys this test nothing, since the audience gate under test runs
// entirely before any bytes reach a conn.
func registerTestClient(t *testing.T, h *Hub, campaignID, userID string, role int, dmGranted bool) *Client {
	t.Helper()
	c := &Client{
		ID:          campaignID + ":" + userID,
		CampaignID:  campaignID,
		UserID:      userID,
		Source:      "browser",
		Role:        role,
		IsDmGranted: dmGranted,
		hub:         h,
		send:        make(chan []byte, 4),
		done:        make(chan struct{}),
	}
	h.register <- c

	// The channel send only guarantees Run() has received the client, not
	// that it has finished the map insert under h.mu — poll briefly for
	// the insert to land before the test proceeds to Broadcast().
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.RLock()
		_, ok := h.clients[campaignID][c.ID]
		h.mu.RUnlock()
		if ok {
			return c
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("client %s never appeared in the hub's registry", c.ID)
	return nil
}

// drainOrNil does a non-blocking read of a client's send buffer. Callers
// sleep first to let the hub's single-threaded broadcast loop finish
// fanning out the one message under test before checking every client —
// see the settleBroadcast comment.
func drainOrNil(c *Client) []byte {
	select {
	case data := <-c.send:
		return data
	default:
		return nil
	}
}

// settleBroadcast gives the hub's Run() goroutine time to finish the
// fan-out loop for one already-published message before the test reads
// every client's channel. The loop's per-message work (iterate <=3 map
// entries, a couple of channel sends) completes in microseconds; 100ms is
// a large, standard margin for this class of test, not a tight race.
func settleBroadcast() { time.Sleep(100 * time.Millisecond) }

// TestHubBroadcast_DeniedPlayerReceivesNothing is the RED-FIRST case named
// in the S1 task: two Players connected to the same campaign, one denied
// by an explicit visibility rule. The denied Player must receive NOTHING
// for the event — not the full payload (the pre-fix bug), not a redacted
// stub (ADR-055 rule 3 forbids that too, since a stub is itself evidence
// hidden content exists).
//
// The rule here carries only denied_users (no allowed_users): per
// ListMarkers' SQL, that is "everyone except the denied," so the third
// Player — named in neither list — gets the HTTP path's default for that
// mode: included. That default is asserted explicitly below via
// wantsDefaultIncluded, not assumed.
func TestHubBroadcast_DeniedPlayerReceivesNothing(t *testing.T) {
	h := newVisibilityTestHub(t)
	const campaignID = "camp-denylist"

	dm := registerTestClient(t, h, campaignID, "user-dm", permissions.RoleOwner, false)
	wantsDefaultIncluded := registerTestClient(t, h, campaignID, "user-neither-list", permissions.RolePlayer, false)
	denied := registerTestClient(t, h, campaignID, "user-denied", permissions.RolePlayer, false)

	// Mirrors what mapEventPublisherAdapter.PublishMarkerEvent computes
	// for a marker with visibility "everyone" and
	// visibility_rules {"denied_users":["user-denied"]}.
	msg := NewMessage(MsgMarkerUpdated, campaignID, "marker-1", map[string]string{"name": "Hidden Cache"})
	msg.DeniedUsers = []string{"user-denied"}

	h.Broadcast(msg)
	settleBroadcast()

	if data := drainOrNil(dm); data == nil {
		t.Error("DM/Owner must always receive the message — Owners bypass visibility_rules entirely, matching ListMarkers' owner branch; got nothing")
	}
	if data := drainOrNil(wantsDefaultIncluded); data == nil {
		t.Error("a Player named in neither allowed_users nor denied_users must receive the message when no allow-list is in play (deny-only mode defaults to included, matching ListMarkers' SQL); got nothing")
	}
	if data := drainOrNil(denied); data != nil {
		t.Errorf("denied Player must receive NOTHING for this event — no payload, no redacted stub; got: %s", data)
	}
}

// TestHubBroadcast_AllowListModeExcludesUnlistedPlayer covers the other
// shape of "the default": a NON-EMPTY allowed_users list is a strict
// allowlist under ListMarkers' SQL (JSON_LENGTH(...) = 0 OR
// JSON_CONTAINS(...)), so a Player named in neither list is EXCLUDED
// here — the opposite default from the deny-only case above. The socket
// must match this asymmetry rather than picking one friendly default for
// both shapes.
func TestHubBroadcast_AllowListModeExcludesUnlistedPlayer(t *testing.T) {
	h := newVisibilityTestHub(t)
	const campaignID = "camp-allowlist"

	dm := registerTestClient(t, h, campaignID, "user-dm", permissions.RoleOwner, false)
	allowedPlayer := registerTestClient(t, h, campaignID, "user-allowed", permissions.RolePlayer, false)
	unlistedPlayer := registerTestClient(t, h, campaignID, "user-unlisted", permissions.RolePlayer, false)

	// Mirrors a "specific"-visibility marker: visibility_rules
	// {"allowed_users":["user-allowed"]}, no denied_users.
	msg := NewMessage(MsgMarkerCreated, campaignID, "marker-2", map[string]string{"name": "Secret Door"})
	msg.AllowedUsers = []string{"user-allowed"}

	h.Broadcast(msg)
	settleBroadcast()

	if data := drainOrNil(dm); data == nil {
		t.Error("DM/Owner must always receive the message; got nothing")
	}
	if data := drainOrNil(allowedPlayer); data == nil {
		t.Error("Player explicitly named in a non-empty allowed_users must receive the message; got nothing")
	}
	if data := drainOrNil(unlistedPlayer); data != nil {
		t.Errorf("Player not on a non-empty allow list must receive NOTHING; got: %s", data)
	}
}

// TestHubBroadcast_DrawingRuleDeniesPlayer is the drawing-side twin of
// TestHubBroadcast_DeniedPlayerReceivesNothing, added once Drawing gained
// VisibilityRules (drawing.go) and drawing_repository.go's ListDrawings
// started enforcing it — the second half of the S1 leak, where drawings
// had no field at all and the column was never selected, so a rule set on
// a drawing did nothing anywhere. The hub's gate is domain-agnostic (it
// only looks at the Message, not at whether the source was a marker or a
// drawing), so this pins that a drawing's rule reaches the wire the same
// way a marker's does — see routes.go's PublishDrawingEvent for the
// production code that builds this shape from a *maps.Drawing.
func TestHubBroadcast_DrawingRuleDeniesPlayer(t *testing.T) {
	h := newVisibilityTestHub(t)
	const campaignID = "camp-drawing"

	dm := registerTestClient(t, h, campaignID, "user-dm", permissions.RoleOwner, false)
	allowed := registerTestClient(t, h, campaignID, "user-neither-list", permissions.RolePlayer, false)
	denied := registerTestClient(t, h, campaignID, "user-denied", permissions.RolePlayer, false)

	msg := NewMessage(MsgDrawingUpdated, campaignID, "drawing-1", map[string]string{"drawing_type": "polygon"})
	msg.DeniedUsers = []string{"user-denied"}

	h.Broadcast(msg)
	settleBroadcast()

	if data := drainOrNil(dm); data == nil {
		t.Error("DM/Owner must always receive the drawing event; got nothing")
	}
	if data := drainOrNil(allowed); data == nil {
		t.Error("Player named in neither list should receive the drawing event by default (deny-only mode); got nothing")
	}
	if data := drainOrNil(denied); data != nil {
		t.Errorf("denied Player must receive NOTHING for this drawing event; got: %s", data)
	}
}
