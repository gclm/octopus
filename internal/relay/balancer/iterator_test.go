package balancer

import (
	"sync"
	"testing"

	"github.com/bestruirui/octopus/internal/model"
)

func TestApplyHealthOrderWithinSamePriority(t *testing.T) {
	prepareCircuitTest(t)
	globalBreaker = sync.Map{}
	RecordFailure(10, 1, "model-a", FailureTLSError)
	RecordSuccess(11, 1, "model-b", 0)

	candidates := []model.GroupItem{
		{ID: 1, ChannelID: 10, ModelName: "model-a", Priority: 1},
		{ID: 2, ChannelID: 11, ModelName: "model-b", Priority: 1},
	}

	applyHealthOrder(candidates)
	if candidates[0].ChannelID != 11 {
		t.Fatalf("expected healthier candidate first, got channel %d", candidates[0].ChannelID)
	}
}

func TestApplyHealthOrderPreservesPriorityAcrossGroups(t *testing.T) {
	prepareCircuitTest(t)
	globalBreaker = sync.Map{}
	RecordSuccess(20, 1, "model-a", 0)
	RecordSuccess(21, 1, "model-b", 0)

	candidates := []model.GroupItem{
		{ID: 1, ChannelID: 20, ModelName: "model-a", Priority: 2},
		{ID: 2, ChannelID: 21, ModelName: "model-b", Priority: 1},
	}

	applyHealthOrder(candidates)
	if candidates[0].Priority != 1 {
		t.Fatalf("expected lower priority value to stay first, got priority %d", candidates[0].Priority)
	}
}

func TestApplyHealthOrderPenalizesVeryBadHealthAcrossPriorities(t *testing.T) {
	prepareCircuitTest(t)
	globalBreaker = sync.Map{}
	RecordFailure(30, 1, "model-a", FailureTLSError) // score -50, penalty 2
	RecordSuccess(31, 1, "model-b", 0)               // score +3, penalty 0

	candidates := []model.GroupItem{
		{ID: 1, ChannelID: 30, ModelName: "model-a", Priority: 1},
		{ID: 2, ChannelID: 31, ModelName: "model-b", Priority: 2},
	}

	applyHealthOrder(candidates)
	if candidates[0].ChannelID != 31 {
		t.Fatalf("expected healthier candidate to overtake after penalty, got channel %d", candidates[0].ChannelID)
	}
}

func TestApplyHealthOrderDoesNotPromoteWarmupCandidateTooEarly(t *testing.T) {
	prepareCircuitTest(t)
	globalBreaker = sync.Map{}
	RecordSuccess(40, 1, "model-a", 500) // one fast success should still be in warmup

	candidates := []model.GroupItem{
		{ID: 1, ChannelID: 41, ModelName: "model-a", Priority: 1},
		{ID: 2, ChannelID: 40, ModelName: "model-a", Priority: 1},
	}

	applyHealthOrder(candidates)
	if candidates[0].ChannelID != 41 {
		t.Fatalf("expected neutral candidate to remain first during warmup, got channel %d", candidates[0].ChannelID)
	}
}

func TestApplyHealthOrderPromotesCandidateAfterWarmup(t *testing.T) {
	prepareCircuitTest(t)
	globalBreaker = sync.Map{}
	for range 3 {
		RecordSuccess(50, 1, "model-a", 500)
	}

	candidates := []model.GroupItem{
		{ID: 1, ChannelID: 51, ModelName: "model-a", Priority: 1},
		{ID: 2, ChannelID: 50, ModelName: "model-a", Priority: 1},
	}

	applyHealthOrder(candidates)
	if candidates[0].ChannelID != 50 {
		t.Fatalf("expected warmed candidate to be promoted, got channel %d", candidates[0].ChannelID)
	}
}
