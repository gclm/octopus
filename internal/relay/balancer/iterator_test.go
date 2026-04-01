package balancer

import (
	"sync"
	"testing"
	"time"

	"github.com/gclm/octopus/internal/model"
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
	RecordFailure(30, 1, "model-a", FailureTLSError)
	RecordSuccess(31, 1, "model-b", 0)

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
	RecordSuccess(40, 1, "model-a", 500*time.Millisecond)

	candidates := []model.GroupItem{
		{ID: 1, ChannelID: 41, ModelName: "model-a", Priority: 1},
		{ID: 2, ChannelID: 40, ModelName: "model-a", Priority: 1},
	}

	applyHealthOrder(candidates)
	if candidates[0].ChannelID != 41 {
		t.Fatalf("expected neutral candidate to remain first during warmup, got channel %d", candidates[0].ChannelID)
	}
}

func TestApplyHealthOrderKeepsBaseOrderForHealthyCandidates(t *testing.T) {
	prepareCircuitTest(t)
	globalBreaker = sync.Map{}
	for range 3 {
		RecordSuccess(50, 1, "model-a", 500*time.Millisecond)
	}

	candidates := []model.GroupItem{
		{ID: 1, ChannelID: 51, ModelName: "model-a", Priority: 1},
		{ID: 2, ChannelID: 50, ModelName: "model-a", Priority: 1},
	}

	applyHealthOrder(candidates)
	if candidates[0].ChannelID != 51 {
		t.Fatalf("expected base order to remain first for healthy candidates, got channel %d", candidates[0].ChannelID)
	}
}

func TestNewIteratorRoundRobinIsScopedPerGroup(t *testing.T) {
	prepareCircuitTest(t)
	groupA := model.Group{
		ID:   1,
		Mode: model.GroupModeRoundRobin,
		Items: []model.GroupItem{
			{ID: 1, ChannelID: 101, ModelName: "model-a", Priority: 1},
			{ID: 2, ChannelID: 102, ModelName: "model-a", Priority: 1},
		},
	}
	groupB := model.Group{
		ID:   2,
		Mode: model.GroupModeRoundRobin,
		Items: []model.GroupItem{
			{ID: 3, ChannelID: 201, ModelName: "model-b", Priority: 1},
			{ID: 4, ChannelID: 202, ModelName: "model-b", Priority: 1},
		},
	}

	iterA1 := NewIterator(groupA, 0, "model-a")
	if !iterA1.Next() || iterA1.Item().ChannelID != 102 {
		t.Fatalf("expected first group A iterator to start from second item, got %+v", iterA1.Item())
	}

	iterB1 := NewIterator(groupB, 0, "model-b")
	if !iterB1.Next() || iterB1.Item().ChannelID != 202 {
		t.Fatalf("expected first group B iterator to start from second item independently, got %+v", iterB1.Item())
	}

	iterA2 := NewIterator(groupA, 0, "model-a")
	if !iterA2.Next() || iterA2.Item().ChannelID != 101 {
		t.Fatalf("expected second group A iterator to rotate back, got %+v", iterA2.Item())
	}
}

func TestNewIteratorFailoverExploresWithinSamePriority(t *testing.T) {
	prepareCircuitTest(t)
	explorationEveryOverride = 1
	now := time.Unix(1_700_000_000, 0)
	explorationNowFunc = func() time.Time { return now }
	RecordRouteAttempt(101, "model-a")
	RecordRouteAttempt(102, "model-a")
	RecordRouteAttempt(103, "model-a")
	now = now.Add(10 * time.Minute)
	RecordRouteAttempt(102, "model-a")
	now = now.Add(10 * time.Minute)
	RecordRouteAttempt(103, "model-a")

	group := model.Group{
		ID:   3,
		Mode: model.GroupModeFailover,
		Items: []model.GroupItem{
			{ID: 1, ChannelID: 101, ModelName: "model-a", Priority: 1},
			{ID: 2, ChannelID: 102, ModelName: "model-a", Priority: 1},
			{ID: 3, ChannelID: 103, ModelName: "model-a", Priority: 2},
		},
	}

	iter := NewIterator(group, 0, "model-a")
	if !iter.Next() || iter.Item().ChannelID != 102 {
		t.Fatalf("expected same-priority exploration candidate first, got %+v", iter.Item())
	}
	span := iter.StartAttempt(102, 1002, "ch-102", "")
	span.End(model.AttemptSuccess, 200, "")
	attempts := iter.Attempts()
	if len(attempts) == 0 || attempts[len(attempts)-1].Exploration != "channel" {
		t.Fatalf("expected channel exploration mark on attempt, got %+v", attempts)
	}
}

func TestNewIteratorDoesNotExploreRoundRobinGroups(t *testing.T) {
	prepareCircuitTest(t)
	explorationEveryOverride = 1
	now := time.Unix(1_700_000_000, 0)
	explorationNowFunc = func() time.Time { return now }
	RecordRouteAttempt(101, "model-a")
	now = now.Add(10 * time.Minute)
	RecordRouteAttempt(102, "model-a")

	group := model.Group{
		ID:   4,
		Mode: model.GroupModeRoundRobin,
		Items: []model.GroupItem{
			{ID: 1, ChannelID: 101, ModelName: "model-a", Priority: 1},
			{ID: 2, ChannelID: 102, ModelName: "model-a", Priority: 1},
		},
	}

	iter := NewIterator(group, 0, "model-a")
	if !iter.Next() || iter.Item().ChannelID != 102 {
		t.Fatalf("expected round robin order to stay intact without exploration, got %+v", iter.Item())
	}
}

func TestNewIteratorWeightedExploresColdStartCandidate(t *testing.T) {
	prepareCircuitTest(t)
	explorationEveryOverride = 1
	base := time.Unix(240*60, 0).UTC()
	smartNowFunc = func() time.Time { return base }
	explorationNowFunc = func() time.Time { return base }
	t.Cleanup(func() { smartNowFunc = time.Now })

	for i := 0; i < 30; i++ {
		recordSmartOutcome(1, "m", true)
	}
	RecordRouteAttempt(1, "m")
	base = base.Add(30 * time.Minute)
	RecordRouteAttempt(1, "m")

	group := model.Group{
		ID:   88,
		Mode: model.GroupModeWeighted,
		Items: []model.GroupItem{
			{ID: 1, ChannelID: 1, ModelName: "m", Weight: 80, Priority: 1},
			{ID: 2, ChannelID: 2, ModelName: "m", Weight: 20, Priority: 5},
		},
	}

	iter := NewIterator(group, 0, "m")
	if !iter.Next() || iter.Item().ChannelID != 2 {
		t.Fatalf("expected weighted exploration to give cold-start candidate a sample, got %+v", iter.Item())
	}
}

func TestNewIteratorWeightedKeepsPrimaryOrderWithoutExploration(t *testing.T) {
	prepareCircuitTest(t)
	explorationEveryOverride = 6
	base := time.Unix(240*60, 0).UTC()
	smartNowFunc = func() time.Time { return base }
	explorationNowFunc = func() time.Time { return base }
	t.Cleanup(func() { smartNowFunc = time.Now })

	for i := 0; i < 30; i++ {
		recordSmartOutcome(1, "m", true)
	}
	RecordFailure(2, 1, "m", FailureTLSError)

	group := model.Group{
		ID:   89,
		Mode: model.GroupModeWeighted,
		Items: []model.GroupItem{
			{ID: 1, ChannelID: 1, ModelName: "m", Weight: 80, Priority: 1},
			{ID: 2, ChannelID: 2, ModelName: "m", Weight: 20, Priority: 1},
		},
	}

	iter := NewIterator(group, 0, "m")
	if !iter.Next() || iter.Item().ChannelID != 1 {
		t.Fatalf("expected weighted primary order to stay intact without exploration, got %+v", iter.Item())
	}
}
