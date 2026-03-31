package balancer

import (
	"sort"
	"time"

	"github.com/bestruirui/octopus/internal/model"
)

type keyCandidate struct {
	key     model.ChannelKey
	score   int
	tripped bool
}

// SelectChannelKey picks a channel key with routing health first and cost second.
// The boolean reports whether the returned key can be used immediately.
func SelectChannelKey(channel *model.Channel, modelName string) (model.ChannelKey, bool, ExplorationDecision) {
	if channel == nil || len(channel.Keys) == 0 {
		return model.ChannelKey{}, false, ExplorationDecision{}
	}

	nowSec := time.Now().Unix()
	candidates := make([]keyCandidate, 0, len(channel.Keys))

	for _, k := range channel.Keys {
		if !k.Enabled || k.ChannelKey == "" {
			continue
		}
		if k.StatusCode == 429 && k.LastUseTimeStamp > 0 {
			if nowSec-k.LastUseTimeStamp < int64(5*time.Minute/time.Second) {
				continue
			}
		}

		tripped, _ := IsTripped(channel.ID, k.ID, modelName)
		candidates = append(candidates, keyCandidate{
			key:     k,
			score:   OrderingHealthScore(channel.ID, k.ID, modelName),
			tripped: tripped,
		})
	}

	if len(candidates) == 0 {
		return model.ChannelKey{}, false, ExplorationDecision{}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if left.tripped != right.tripped {
			return !left.tripped
		}
		leftPenalty := healthPenalty(left.score)
		rightPenalty := healthPenalty(right.score)
		if leftPenalty != rightPenalty {
			return leftPenalty < rightPenalty
		}
		if left.score != right.score {
			return left.score > right.score
		}
		if left.key.TotalCost != right.key.TotalCost {
			return left.key.TotalCost < right.key.TotalCost
		}
		return left.key.ID < right.key.ID
	})

	decision := maybePromoteKeyExploration(channel, modelName, candidates)
	return candidates[0].key, !candidates[0].tripped, decision
}
