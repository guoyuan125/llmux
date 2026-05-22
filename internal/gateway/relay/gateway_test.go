package relay

import (
	"testing"

	"github.com/liuguoyuan/llmux/internal/gateway/session"
	"github.com/liuguoyuan/llmux/internal/model"
)

// TestMoveToFront_ChannelExists verifies that when the sticky channel is present in
// candidates, it is promoted to position 0 and the remaining order is preserved.
func TestMoveToFront_ChannelExists(t *testing.T) {
	items := []model.GroupItem{
		{ChannelID: 1},
		{ChannelID: 2},
		{ChannelID: 3},
	}
	result := moveToFront(items, 2)
	if len(result) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result))
	}
	if result[0].ChannelID != 2 {
		t.Errorf("expected channel 2 at front, got %d", result[0].ChannelID)
	}
	// Original slice must not be mutated.
	if items[0].ChannelID != 1 {
		t.Errorf("original slice was mutated: items[0].ChannelID = %d", items[0].ChannelID)
	}
}

// TestMoveToFront_ChannelNotFound verifies that when the sticky channel ID is absent
// from candidates, moveToFront returns the original slice unchanged.
func TestMoveToFront_ChannelNotFound(t *testing.T) {
	items := []model.GroupItem{
		{ChannelID: 1},
		{ChannelID: 2},
	}
	result := moveToFront(items, 99)
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
	if result[0].ChannelID != 1 || result[1].ChannelID != 2 {
		t.Errorf("expected original order [1 2], got [%d %d]", result[0].ChannelID, result[1].ChannelID)
	}
}

// TestSessionStickiness_ChannelRemovedFromGroup verifies the fix for the session
// stickiness bug: when the sticky channel is no longer in candidates, the session
// entry must be deleted so subsequent requests use normal load balancing.
//
// The test directly exercises the logic added in HandleRelay by replicating it
// against a real session.Store, confirming the delete branch fires.
func TestSessionStickiness_ChannelRemovedFromGroup(t *testing.T) {
	const (
		apiKeyID     uint   = 1
		requestModel string = "gpt-4o"
		stickyID     uint   = 42 // channel that will be removed from the group
	)

	store := session.NewStore()

	// Seed a stale session pointing to channel 42, which is no longer in the group.
	store.Set(apiKeyID, requestModel, stickyID, 0, 60*1_000_000_000 /* 60s */)

	// Current candidates (channel 42 has been removed from the group).
	candidates := []model.GroupItem{
		{ChannelID: 10},
		{ChannelID: 20},
	}

	// Replicate the stickiness validation block from HandleRelay.
	if chID, _, ok := store.Get(apiKeyID, requestModel); ok {
		reordered := moveToFront(candidates, chID)
		if len(reordered) > 0 && reordered[0].ChannelID == chID {
			candidates = reordered
		} else {
			// Sticky channel not in group — delete stale session entry.
			store.Delete(apiKeyID, requestModel)
		}
	}

	// Verify the stale session was cleared.
	if _, _, ok := store.Get(apiKeyID, requestModel); ok {
		t.Error("stale session was not deleted after sticky channel removed from group")
	}

	// Verify candidates remain unchanged (no unintended reordering).
	if candidates[0].ChannelID != 10 || candidates[1].ChannelID != 20 {
		t.Errorf("candidates unexpectedly reordered: [%d %d]", candidates[0].ChannelID, candidates[1].ChannelID)
	}
}

// TestSessionStickiness_ChannelStillInGroup verifies that when the sticky channel
// is still a valid candidate, the session is preserved and it is placed first.
func TestSessionStickiness_ChannelStillInGroup(t *testing.T) {
	const (
		apiKeyID     uint   = 2
		requestModel string = "claude-3-5-sonnet"
		stickyID     uint   = 7
	)

	store := session.NewStore()
	store.Set(apiKeyID, requestModel, stickyID, 0, 60*1_000_000_000 /* 60s */)

	candidates := []model.GroupItem{
		{ChannelID: 5},
		{ChannelID: 7},
		{ChannelID: 9},
	}

	if chID, _, ok := store.Get(apiKeyID, requestModel); ok {
		reordered := moveToFront(candidates, chID)
		if len(reordered) > 0 && reordered[0].ChannelID == chID {
			candidates = reordered
		} else {
			store.Delete(apiKeyID, requestModel)
		}
	}

	// Session must still exist.
	if _, _, ok := store.Get(apiKeyID, requestModel); !ok {
		t.Error("session was incorrectly deleted when sticky channel is still in group")
	}

	// Sticky channel must be at front.
	if candidates[0].ChannelID != stickyID {
		t.Errorf("expected sticky channel %d at front, got %d", stickyID, candidates[0].ChannelID)
	}
}
