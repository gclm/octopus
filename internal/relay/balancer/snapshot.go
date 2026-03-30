package balancer

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bestruirui/octopus/internal/model"
)

type keyHealthAggregate struct {
	summary model.ChannelKeyHealthSummary
	routes  []model.ChannelHealthRoute
}

// SnapshotChannelRuntimeHealth attaches runtime health summary information to a
// channel and its keys without changing persisted channel state.
func SnapshotChannelRuntimeHealth(channel *model.Channel) {
	if channel == nil {
		return
	}

	summary, keyAggregates := snapshotChannelHealth(channel.ID)
	channel.HealthSummary = &summary

	for i := range channel.Keys {
		channel.Keys[i].HealthSummary = nil
		channel.Keys[i].HealthRoutes = nil

		agg, ok := keyAggregates[channel.Keys[i].ID]
		if !ok {
			continue
		}

		sort.SliceStable(agg.routes, func(i, j int) bool {
			left := agg.routes[i]
			right := agg.routes[j]
			leftRank := routeSortRank(left)
			rightRank := routeSortRank(right)
			if leftRank != rightRank {
				return leftRank < rightRank
			}
			if left.CooldownRemainingMs != right.CooldownRemainingMs {
				return left.CooldownRemainingMs > right.CooldownRemainingMs
			}
			if left.RawScore != right.RawScore {
				return left.RawScore < right.RawScore
			}
			return left.ModelName < right.ModelName
		})

		summaryCopy := agg.summary
		channel.Keys[i].HealthSummary = &summaryCopy
		channel.Keys[i].HealthRoutes = agg.routes
	}
}

func snapshotChannelHealth(channelID int) (model.ChannelHealthSummary, map[int]*keyHealthAggregate) {
	summary := model.ChannelHealthSummary{
		Status:            "idle",
		BestOrderingScore: math.MinInt,
		WorstRawScore:     math.MaxInt,
	}
	keyAggregates := make(map[int]*keyHealthAggregate)
	trackedKeys := make(map[int]struct{})
	now := time.Now()

	globalBreaker.Range(func(k, v any) bool {
		entryKey, ok := k.(string)
		if !ok {
			return true
		}

		parts := strings.SplitN(entryKey, ":", 3)
		if len(parts) != 3 {
			return true
		}

		parsedChannelID, err := strconv.Atoi(parts[0])
		if err != nil || parsedChannelID != channelID {
			return true
		}

		keyID, err := strconv.Atoi(parts[1])
		if err != nil {
			return true
		}

		entry := v.(*circuitEntry)
		entry.mu.Lock()
		entry.applyDecay(now)
		rawScore := entry.HealthScore
		orderingScore := rankingHealthScore(rawScore, entry.SuccessCount)
		remaining := time.Until(entry.OpenUntil)
		if remaining < 0 {
			remaining = 0
		}
		lastFailureKind := string(entry.LastFailureKind)
		if lastFailureKind == "" {
			lastFailureKind = string(FailureUnknown)
		}
		route := model.ChannelHealthRoute{
			ModelName:           parts[2],
			ChannelKeyID:        keyID,
			State:               routeStateLabel(entry.State, remaining),
			RawScore:            rawScore,
			OrderingScore:       orderingScore,
			SuccessCount:        entry.SuccessCount,
			WarmupPending:       rawScore > orderingScore,
			CooldownRemainingMs: remaining.Milliseconds(),
			LastFailureKind:     lastFailureKind,
		}
		entry.mu.Unlock()

		agg := keyAggregates[keyID]
		if agg == nil {
			agg = &keyHealthAggregate{
				summary: model.ChannelKeyHealthSummary{
					Status:            "idle",
					BestOrderingScore: math.MinInt,
					WorstRawScore:     math.MaxInt,
				},
			}
			keyAggregates[keyID] = agg
		}
		agg.routes = append(agg.routes, route)
		updateKeySummary(&agg.summary, route)

		trackedKeys[keyID] = struct{}{}
		updateChannelSummary(&summary, route)
		return true
	})

	if summary.TrackedRoutes == 0 {
		summary.BestOrderingScore = 0
		summary.WorstRawScore = 0
		return summary, keyAggregates
	}

	summary.TrackedKeys = len(trackedKeys)
	summary.Status = deriveAggregateStatus(summary.CoolingRoutes, summary.WarmupRoutes, summary.WorstRawScore, summary.BestOrderingScore)

	for _, agg := range keyAggregates {
		if agg.summary.TrackedRoutes == 0 {
			agg.summary.BestOrderingScore = 0
			agg.summary.WorstRawScore = 0
			continue
		}
		agg.summary.Status = deriveAggregateStatus(agg.summary.CoolingRoutes, agg.summary.WarmupRoutes, agg.summary.WorstRawScore, agg.summary.BestOrderingScore)
	}

	return summary, keyAggregates
}

func updateChannelSummary(summary *model.ChannelHealthSummary, route model.ChannelHealthRoute) {
	if summary == nil {
		return
	}

	summary.TrackedRoutes++
	if route.State == "open" {
		summary.CoolingRoutes++
	}
	if route.WarmupPending {
		summary.WarmupRoutes++
	}
	if route.OrderingScore > summary.BestOrderingScore {
		summary.BestOrderingScore = route.OrderingScore
	}
	if route.RawScore < summary.WorstRawScore {
		summary.WorstRawScore = route.RawScore
	}
	if route.CooldownRemainingMs > summary.CooldownRemainingMs {
		summary.CooldownRemainingMs = route.CooldownRemainingMs
		summary.LastFailureKind = route.LastFailureKind
	}
	if summary.LastFailureKind == "" && route.LastFailureKind != string(FailureUnknown) {
		summary.LastFailureKind = route.LastFailureKind
	}
}

func updateKeySummary(summary *model.ChannelKeyHealthSummary, route model.ChannelHealthRoute) {
	if summary == nil {
		return
	}

	summary.TrackedRoutes++
	if route.State == "open" {
		summary.CoolingRoutes++
	}
	if route.WarmupPending {
		summary.WarmupRoutes++
	}
	if route.OrderingScore > summary.BestOrderingScore {
		summary.BestOrderingScore = route.OrderingScore
	}
	if route.RawScore < summary.WorstRawScore {
		summary.WorstRawScore = route.RawScore
	}
	if route.CooldownRemainingMs > summary.CooldownRemainingMs {
		summary.CooldownRemainingMs = route.CooldownRemainingMs
		summary.LastFailureKind = route.LastFailureKind
	}
	if summary.LastFailureKind == "" && route.LastFailureKind != string(FailureUnknown) {
		summary.LastFailureKind = route.LastFailureKind
	}
}

func routeStateLabel(state CircuitState, remaining time.Duration) string {
	switch state {
	case StateOpen:
		if remaining > 0 {
			return "open"
		}
		return "half_open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "closed"
	}
}

func deriveAggregateStatus(coolingRoutes, warmupRoutes, worstRawScore, bestOrderingScore int) string {
	switch {
	case coolingRoutes > 0:
		return "cooling"
	case warmupRoutes > 0:
		return "warming"
	case worstRawScore < 0:
		return "degraded"
	case bestOrderingScore > 0:
		return "healthy"
	default:
		return "neutral"
	}
}

func routeSortRank(route model.ChannelHealthRoute) int {
	switch {
	case route.State == "open":
		return 0
	case route.WarmupPending:
		return 1
	case route.RawScore < 0:
		return 2
	case route.OrderingScore > 0:
		return 3
	default:
		return 4
	}
}
