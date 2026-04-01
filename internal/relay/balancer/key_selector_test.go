package balancer

import (
	"sync"
	"testing"
	"time"

	"github.com/gclm/octopus/internal/model"
)

func TestSelectChannelKeyPrefersHealthBeforeCost(t *testing.T) {
	prepareCircuitTest(t)
	globalBreaker = sync.Map{}

	channel := model.Channel{
		ID: 60,
		Keys: []model.ChannelKey{
			{ID: 1, ChannelID: 60, Enabled: true, ChannelKey: "cheap", TotalCost: 1},
			{ID: 2, ChannelID: 60, Enabled: true, ChannelKey: "healthy", TotalCost: 9},
		},
	}

	RecordFailure(60, 1, "model-a", FailureNetworkTimeout)
	for range 3 {
		RecordSuccess(60, 2, "model-a", 500*time.Millisecond)
	}

	selected, hasAvailable, _ := SelectChannelKey(&channel, "model-a")
	if !hasAvailable {
		t.Fatal("expected available key")
	}
	if selected.ID != 2 {
		t.Fatalf("expected healthier key to be selected, got %d", selected.ID)
	}
}

func TestSelectChannelKeySkipsTrippedKeyWhenAnotherIsAvailable(t *testing.T) {
	prepareCircuitTest(t)
	globalBreaker = sync.Map{}

	channel := model.Channel{
		ID: 61,
		Keys: []model.ChannelKey{
			{ID: 1, ChannelID: 61, Enabled: true, ChannelKey: "tripped", TotalCost: 1},
			{ID: 2, ChannelID: 61, Enabled: true, ChannelKey: "fallback", TotalCost: 5},
		},
	}

	RecordFailure(61, 1, "model-a", FailureTLSError)
	selected, hasAvailable, _ := SelectChannelKey(&channel, "model-a")
	if !hasAvailable {
		t.Fatal("expected available key")
	}
	if selected.ID != 2 {
		t.Fatalf("expected fallback key, got %d", selected.ID)
	}
}

func TestSelectChannelKeyReturnsBestTrippedKeyWhenAllKeysBlocked(t *testing.T) {
	prepareCircuitTest(t)
	globalBreaker = sync.Map{}

	channel := model.Channel{
		ID: 62,
		Keys: []model.ChannelKey{
			{ID: 1, ChannelID: 62, Enabled: true, ChannelKey: "tripped-a", TotalCost: 1},
			{ID: 2, ChannelID: 62, Enabled: true, ChannelKey: "tripped-b", TotalCost: 5},
		},
	}

	RecordFailure(62, 1, "model-a", FailureTLSError)
	RecordFailure(62, 2, "model-a", FailureAuthError)

	selected, hasAvailable, _ := SelectChannelKey(&channel, "model-a")
	if hasAvailable {
		t.Fatal("expected no immediately available key")
	}
	if selected.ID == 0 {
		t.Fatal("expected best blocked key for circuit-break logging")
	}
}

func TestSelectChannelKeyExploresStaleHealthyPeer(t *testing.T) {
	prepareCircuitTest(t)
	explorationEveryOverride = 1
	now := time.Unix(1_700_000_000, 0)
	explorationNowFunc = func() time.Time { return now }
	globalBreaker = sync.Map{}

	channel := model.Channel{
		ID: 63,
		Keys: []model.ChannelKey{
			{ID: 1, ChannelID: 63, Enabled: true, ChannelKey: "hot", TotalCost: 1},
			{ID: 2, ChannelID: 63, Enabled: true, ChannelKey: "stale", TotalCost: 9},
		},
	}

	for range 3 {
		RecordSuccess(63, 1, "model-a", 500*time.Millisecond)
		RecordSuccess(63, 2, "model-a", 500*time.Millisecond)
	}

	RecordKeyAttempt(63, 1, "model-a")
	now = now.Add(30 * time.Minute)
	RecordKeyAttempt(63, 1, "model-a")

	selected, hasAvailable, decision := SelectChannelKey(&channel, "model-a")
	if !hasAvailable {
		t.Fatal("expected available key")
	}
	if selected.ID != 2 {
		t.Fatalf("expected stale healthy peer to be explored, got %d", selected.ID)
	}
	if decision.Kind != "key" {
		t.Fatalf("expected key exploration decision, got %+v", decision)
	}
}

func TestSelectChannelKeyDoesNotExploreWhenKeyExplorationDisabled(t *testing.T) {
	prepareCircuitTest(t)
	explorationEveryOverride = 1
	disabled := false
	keyExplorationEnabledTest = &disabled
	now := time.Unix(1_700_000_000, 0)
	explorationNowFunc = func() time.Time { return now }
	globalBreaker = sync.Map{}

	channel := model.Channel{
		ID: 64,
		Keys: []model.ChannelKey{
			{ID: 1, ChannelID: 64, Enabled: true, ChannelKey: "hot", TotalCost: 1},
			{ID: 2, ChannelID: 64, Enabled: true, ChannelKey: "stale", TotalCost: 9},
		},
	}

	for range 3 {
		RecordSuccess(64, 1, "model-a", 500*time.Millisecond)
		RecordSuccess(64, 2, "model-a", 500*time.Millisecond)
	}

	RecordKeyAttempt(64, 1, "model-a")
	now = now.Add(30 * time.Minute)
	RecordKeyAttempt(64, 1, "model-a")

	selected, hasAvailable, decision := SelectChannelKey(&channel, "model-a")
	if !hasAvailable {
		t.Fatal("expected available key")
	}
	if selected.ID != 1 {
		t.Fatalf("expected hot key to remain first when key exploration disabled, got %d", selected.ID)
	}
	if decision.Kind != "" {
		t.Fatalf("expected no key exploration decision, got %+v", decision)
	}
}
