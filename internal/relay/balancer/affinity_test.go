package balancer

import (
	"reflect"
	"testing"

	"github.com/gclm/octopus/internal/model"
	tmodel "github.com/gclm/octopus/internal/transformer/model"
)

func TestRequestAffinityKey_NormalizesSimilarPrefixes(t *testing.T) {
	requestA := &tmodel.InternalLLMRequest{
		Messages: []tmodel.Message{{
			Role: "user",
			Content: tmodel.MessageContent{
				Content: stringPtr("Hello   WORLD\n\nFrom  Octopus"),
			},
		}},
	}
	requestB := &tmodel.InternalLLMRequest{
		Messages: []tmodel.Message{{
			Role: "user",
			Content: tmodel.MessageContent{
				Content: stringPtr(" hello world from octopus "),
			},
		}},
	}

	gotA := requestAffinityKey(requestA)
	gotB := requestAffinityKey(requestB)
	if gotA == "" || gotB == "" {
		t.Fatalf("unexpected empty affinity keys: %q %q", gotA, gotB)
	}
	if gotA != gotB {
		t.Fatalf("affinity key mismatch: %q != %q", gotA, gotB)
	}
}

func TestApplyPrefixAffinityOrdering_IsStableForSamePrefix(t *testing.T) {
	request := &tmodel.InternalLLMRequest{
		Messages: []tmodel.Message{{
			Role: "user",
			Content: tmodel.MessageContent{
				Content: stringPtr("Summarize this changelog and keep headings stable"),
			},
		}},
	}
	items := []model.GroupItem{
		{ChannelID: 101, ModelName: "gpt-5", Priority: 1},
		{ChannelID: 102, ModelName: "gpt-5", Priority: 1},
		{ChannelID: 103, ModelName: "gpt-5", Priority: 1},
	}
	scoreFn := func(model.GroupItem) float64 { return 10 }

	first := applyPrefixAffinityOrdering(items, request, scoreFn)
	second := applyPrefixAffinityOrdering(items, request, scoreFn)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("ordering is not stable: first=%v second=%v", first, second)
	}
}

func TestApplyPrefixAffinityOrdering_RespectsLargeScoreGap(t *testing.T) {
	request := &tmodel.InternalLLMRequest{
		Messages: []tmodel.Message{{
			Role:    "user",
			Content: tmodel.MessageContent{Content: stringPtr("same prefix")},
		}},
	}
	items := []model.GroupItem{
		{ChannelID: 1, ModelName: "gpt-5", Priority: 1},
		{ChannelID: 2, ModelName: "gpt-5", Priority: 1},
	}
	scores := map[int]float64{1: 9, 2: 7}

	got := applyPrefixAffinityOrdering(items, request, func(item model.GroupItem) float64 {
		return scores[item.ChannelID]
	})
	if got[0].ChannelID != 1 {
		t.Fatalf("expected higher scored candidate to stay first, got %+v", got)
	}
}

func stringPtr(v string) *string { return &v }
