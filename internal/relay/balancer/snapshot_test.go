package balancer

import (
	"sync"
	"testing"
	"time"

	"github.com/gclm/octopus/internal/model"
)

func TestSnapshotChannelRuntimeHealthBuildsSummaryAndKeyRoutes(t *testing.T) {
	prepareCircuitTest(t)
	globalBreaker = sync.Map{}

	channel := model.Channel{
		ID: 70,
		Keys: []model.ChannelKey{
			{ID: 1, ChannelID: 70, Enabled: true, ChannelKey: "key-a"},
			{ID: 2, ChannelID: 70, Enabled: true, ChannelKey: "key-b"},
		},
	}

	RecordFailure(70, 1, "model-a", FailureTLSError)
	for range 3 {
		RecordSuccess(70, 2, "model-b", 500*time.Millisecond)
	}

	SnapshotChannelRuntimeHealth(&channel)

	if channel.HealthSummary == nil {
		t.Fatal("expected channel health summary")
	}
	if channel.HealthSummary.Status != "cooling" {
		t.Fatalf("expected cooling summary, got %s", channel.HealthSummary.Status)
	}
	if channel.HealthSummary.TrackedRoutes != 2 {
		t.Fatalf("expected 2 tracked routes, got %d", channel.HealthSummary.TrackedRoutes)
	}

	if channel.Keys[0].HealthSummary == nil {
		t.Fatal("expected key 1 summary")
	}
	if channel.Keys[0].HealthSummary.Status != "cooling" {
		t.Fatalf("expected key 1 cooling, got %s", channel.Keys[0].HealthSummary.Status)
	}
	if len(channel.Keys[0].HealthRoutes) != 1 {
		t.Fatalf("expected key 1 route details, got %d", len(channel.Keys[0].HealthRoutes))
	}

	if channel.Keys[1].HealthSummary == nil {
		t.Fatal("expected key 2 summary")
	}
	if channel.Keys[1].HealthSummary.Status != "healthy" {
		t.Fatalf("expected key 2 healthy, got %s", channel.Keys[1].HealthSummary.Status)
	}
	if channel.Keys[1].HealthRoutes[0].OrderingScore <= 0 {
		t.Fatalf("expected positive ordering score, got %d", channel.Keys[1].HealthRoutes[0].OrderingScore)
	}
}
