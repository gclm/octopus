package balancer

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gclm/octopus/internal/model"
	"github.com/gclm/octopus/internal/op"
)

const defaultExplorationEvery = 6

var (
	explorationCounters       sync.Map // key: groupID -> *atomic.Uint64
	routeAttemptActivity      sync.Map // key: channelID:modelName -> time.Time
	keyAttemptActivity        sync.Map // key: channelID:keyID:modelName -> time.Time
	explorationEveryOverride  int
	keyExplorationEnabledTest *bool
	explorationNowFunc        = time.Now
)

func explorationEvery() int {
	if explorationEveryOverride > 0 {
		return explorationEveryOverride
	}
	v, err := op.SettingGetInt(model.SettingKeyCircuitBreakerExplorationEvery)
	if err != nil || v <= 0 {
		return defaultExplorationEvery
	}
	return v
}

func routeAttemptKey(channelID int, modelName string) string {
	return fmt.Sprintf("%d:%s", channelID, modelName)
}

func keyAttemptKey(channelID, keyID int, modelName string) string {
	return fmt.Sprintf("%d:%d:%s", channelID, keyID, modelName)
}

// RecordRouteAttempt tracks the last real outbound attempt time for a channel-model route.
func RecordRouteAttempt(channelID int, modelName string) {
	if channelID <= 0 || modelName == "" {
		return
	}
	routeAttemptActivity.Store(routeAttemptKey(channelID, modelName), explorationNowFunc())
}

func lastRouteAttempt(channelID int, modelName string) time.Time {
	v, ok := routeAttemptActivity.Load(routeAttemptKey(channelID, modelName))
	if !ok {
		return time.Time{}
	}
	last, _ := v.(time.Time)
	return last
}

// RecordKeyAttempt tracks the last real outbound attempt time for a channel-key-model route.
func RecordKeyAttempt(channelID, keyID int, modelName string) {
	if channelID <= 0 || keyID <= 0 || modelName == "" {
		return
	}
	keyAttemptActivity.Store(keyAttemptKey(channelID, keyID, modelName), explorationNowFunc())
}

func lastKeyAttempt(channelID, keyID int, modelName string) time.Time {
	v, ok := keyAttemptActivity.Load(keyAttemptKey(channelID, keyID, modelName))
	if !ok {
		return time.Time{}
	}
	last, _ := v.(time.Time)
	return last
}

func shouldExplore(group model.Group, candidates []model.GroupItem) bool {
	if len(candidates) < 2 {
		return false
	}
	if group.Mode != model.GroupModeFailover && group.Mode != model.GroupModeWeighted {
		return false
	}
	every := explorationEvery()
	if every <= 1 {
		return true
	}
	v, _ := explorationCounters.LoadOrStore(group.ID, &atomic.Uint64{})
	return v.(*atomic.Uint64).Add(1)%uint64(every) == 0
}

func maybePromoteExploration(group model.Group, candidates []model.GroupItem) ExplorationDecision {
	if !shouldExplore(group, candidates) {
		return ExplorationDecision{}
	}

	switch group.Mode {
	case model.GroupModeWeighted:
		return maybePromoteWeightedExploration(candidates)
	case model.GroupModeFailover:
		return maybePromoteFailoverExploration(candidates)
	default:
		return ExplorationDecision{}
	}
}

func maybePromoteFailoverExploration(candidates []model.GroupItem) ExplorationDecision {
	basePriority := candidates[0].Priority
	bestIdx := -1
	bestLastAttempt := time.Time{}

	for i := 1; i < len(candidates); i++ {
		candidate := candidates[i]
		if candidate.Priority != basePriority {
			continue
		}
		if OrderingHealthScore(candidate.ChannelID, 0, candidate.ModelName) < 0 {
			continue
		}

		lastAttempt := lastRouteAttempt(candidate.ChannelID, candidate.ModelName)
		if bestIdx == -1 || bestLastAttempt.IsZero() || (!lastAttempt.IsZero() && lastAttempt.Before(bestLastAttempt)) {
			bestIdx = i
			bestLastAttempt = lastAttempt
		}
	}

	if bestIdx <= 0 {
		return ExplorationDecision{}
	}

	promoted := candidates[bestIdx]
	copy(candidates[1:bestIdx+1], candidates[:bestIdx])
	candidates[0] = promoted
	return ExplorationDecision{Kind: "channel", CandidateID: promoted.ChannelID}
}

func maybePromoteWeightedExploration(candidates []model.GroupItem) ExplorationDecision {
	bestIdx := -1
	bestLastAttempt := time.Time{}

	for i := 1; i < len(candidates); i++ {
		candidate := candidates[i]
		if OrderingHealthScore(candidate.ChannelID, 0, candidate.ModelName) < 0 {
			continue
		}
		lastAttempt := lastRouteAttempt(candidate.ChannelID, candidate.ModelName)
		if shouldPreferWeightedExplorationCandidate(bestIdx, bestLastAttempt, lastAttempt) {
			bestIdx = i
			bestLastAttempt = lastAttempt
		}
	}

	if bestIdx <= 0 {
		return ExplorationDecision{}
	}

	promoted := candidates[bestIdx]
	copy(candidates[1:bestIdx+1], candidates[:bestIdx])
	candidates[0] = promoted
	return ExplorationDecision{Kind: "channel", CandidateID: promoted.ChannelID}
}

func shouldPreferWeightedExplorationCandidate(currentIdx int, currentLastAttempt time.Time, nextLastAttempt time.Time) bool {
	if currentIdx == -1 {
		return true
	}
	if currentLastAttempt.IsZero() != nextLastAttempt.IsZero() {
		return nextLastAttempt.IsZero()
	}
	if currentLastAttempt.IsZero() {
		return false
	}
	return nextLastAttempt.Before(currentLastAttempt)
}

func resetExplorationStateForTest() {
	explorationCounters = sync.Map{}
	routeAttemptActivity = sync.Map{}
	keyAttemptActivity = sync.Map{}
	explorationEveryOverride = 0
	keyExplorationEnabledTest = nil
	explorationNowFunc = time.Now
}

func keyExplorationEnabled() bool {
	if keyExplorationEnabledTest != nil {
		return *keyExplorationEnabledTest
	}
	v, err := op.SettingGetBool(model.SettingKeyCircuitBreakerKeyExplorationEnabled)
	if err != nil {
		return true
	}
	return v
}

func shouldExploreKeys(channel *model.Channel) bool {
	if channel == nil || len(channel.Keys) < 2 {
		return false
	}
	return keyExplorationEnabled()
}

type ExplorationDecision struct {
	Kind        string
	CandidateID int
}

func (d ExplorationDecision) Active() bool {
	return d.Kind != ""
}

func maybePromoteKeyExploration(channel *model.Channel, modelName string, candidates []keyCandidate) ExplorationDecision {
	if !shouldExploreKeys(channel) {
		return ExplorationDecision{}
	}
	if len(candidates) == 0 || candidates[0].tripped || healthPenalty(candidates[0].score) > 0 {
		return ExplorationDecision{}
	}
	every := explorationEvery()
	if every > 1 {
		v, _ := explorationCounters.LoadOrStore(fmt.Sprintf("key:%d:%s", channel.ID, modelName), &atomic.Uint64{})
		if v.(*atomic.Uint64).Add(1)%uint64(every) != 0 {
			return ExplorationDecision{}
		}
	}

	bestIdx := -1
	bestLastAttempt := time.Time{}
	for i := 1; i < len(candidates); i++ {
		candidate := candidates[i]
		if candidate.tripped || healthPenalty(candidate.score) > 0 {
			continue
		}
		lastAttempt := lastKeyAttempt(channel.ID, candidate.key.ID, modelName)
		if bestIdx == -1 || bestLastAttempt.IsZero() || (!lastAttempt.IsZero() && lastAttempt.Before(bestLastAttempt)) {
			bestIdx = i
			bestLastAttempt = lastAttempt
		}
	}
	if bestIdx <= 0 {
		return ExplorationDecision{}
	}
	promoted := candidates[bestIdx]
	copy(candidates[1:bestIdx+1], candidates[:bestIdx])
	candidates[0] = promoted
	return ExplorationDecision{Kind: "key", CandidateID: promoted.key.ID}
}
