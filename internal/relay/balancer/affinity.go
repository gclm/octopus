package balancer

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/bestruirui/octopus/internal/model"
	tmodel "github.com/bestruirui/octopus/internal/transformer/model"
)

const (
	// Keep affinity local to near-tie scored candidates so health score remains
	// the primary signal, while similar request prefixes still map stably.
	prefixAffinityScoreTieThreshold = 0.25
	affinityPrefixRuneLimit         = 128
)

type groupItemKey struct {
	ID        int
	ChannelID int
	ModelName string
	Priority  int
	Weight    int
}

type affinityMetadata struct {
	score    float64
	affinity uint64
}

func applyPrefixAffinityOrdering(items []model.GroupItem, request *tmodel.InternalLLMRequest, scoreFn func(model.GroupItem) float64) []model.GroupItem {
	if len(items) < 2 || request == nil || scoreFn == nil {
		return items
	}
	affinityKey := requestAffinityKey(request)
	if affinityKey == "" {
		return items
	}

	sorted := make([]model.GroupItem, len(items))
	copy(sorted, items)

	metadata := make(map[groupItemKey]affinityMetadata, len(sorted))
	for _, item := range sorted {
		metadata[toGroupItemKey(item)] = affinityMetadata{
			score:    scoreFn(item),
			affinity: affinityRank(affinityKey, item),
		}
	}

	sort.SliceStable(sorted, func(i, j int) bool {
		left := metadata[toGroupItemKey(sorted[i])]
		right := metadata[toGroupItemKey(sorted[j])]

		if sorted[i].Priority != sorted[j].Priority {
			if left.score == right.score {
				return sorted[i].Priority < sorted[j].Priority
			}
			return left.score > right.score
		}

		if math.Abs(left.score-right.score) > prefixAffinityScoreTieThreshold {
			return left.score > right.score
		}
		if left.affinity != right.affinity {
			return left.affinity < right.affinity
		}
		if left.score != right.score {
			return left.score > right.score
		}
		if sorted[i].ChannelID == sorted[j].ChannelID {
			return sorted[i].ModelName < sorted[j].ModelName
		}
		return sorted[i].ChannelID < sorted[j].ChannelID
	})

	return sorted
}

func requestAffinityKey(request *tmodel.InternalLLMRequest) string {
	if request == nil {
		return ""
	}

	var builder strings.Builder
	appendAffinityText(&builder, embeddingAffinityText(request.EmbeddingInput))
	for _, msg := range request.Messages {
		appendAffinityText(&builder, messageAffinityText(msg))
		if builder.Len() >= affinityPrefixRuneLimit {
			break
		}
	}
	return normalizeAffinityText(builder.String())
}

func appendAffinityText(builder *strings.Builder, text string) {
	if builder == nil || text == "" {
		return
	}
	if builder.Len() > 0 {
		builder.WriteByte('\n')
	}
	builder.WriteString(text)
}

func embeddingAffinityText(input *tmodel.EmbeddingInput) string {
	if input == nil {
		return ""
	}
	if input.Single != nil {
		return *input.Single
	}
	return strings.Join(input.Multiple, "\n")
}

func messageAffinityText(msg tmodel.Message) string {
	parts := make([]string, 0, 2+len(msg.Content.MultipleContent))
	if msg.Content.Content != nil {
		parts = append(parts, *msg.Content.Content)
	}
	for _, part := range msg.Content.MultipleContent {
		if part.Type == "text" && part.Text != nil {
			parts = append(parts, *part.Text)
		}
	}
	if reasoning := msg.GetReasoningContent(); reasoning != "" {
		parts = append(parts, reasoning)
	}
	return strings.Join(parts, "\n")
}

func normalizeAffinityText(raw string) string {
	if raw == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(raw))
	pendingSpace := false
	runeCount := 0
	for _, r := range strings.ToLower(raw) {
		if unicode.IsSpace(r) {
			pendingSpace = builder.Len() > 0
			continue
		}
		if pendingSpace {
			builder.WriteByte(' ')
			pendingSpace = false
		}
		builder.WriteRune(r)
		runeCount++
		if runeCount >= affinityPrefixRuneLimit {
			break
		}
	}
	return strings.TrimSpace(builder.String())
}

func affinityRank(affinityKey string, item model.GroupItem) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(affinityKey))
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(fmt.Sprintf("%d|%s", item.ChannelID, item.ModelName)))
	return h.Sum64()
}

func toGroupItemKey(item model.GroupItem) groupItemKey {
	return groupItemKey{
		ID:        item.ID,
		ChannelID: item.ChannelID,
		ModelName: item.ModelName,
		Priority:  item.Priority,
		Weight:    item.Weight,
	}
}
