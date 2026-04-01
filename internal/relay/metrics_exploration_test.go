package relay

import (
	"testing"

	"github.com/gclm/octopus/internal/model"
)

func TestSummarizeExplorationCountsChannelAndKeySignals(t *testing.T) {
	attempts := []model.ChannelAttempt{
		{ChannelID: 1, ChannelName: "c1", Status: model.AttemptFailed},
		{ChannelID: 2, ChannelName: "c2", Status: model.AttemptFailed, Exploration: "channel"},
		{ChannelID: 3, ChannelName: "c3", Status: model.AttemptSuccess, Exploration: "channel,key"},
	}

	summary := summarizeExploration(attempts)
	if summary.Total != 2 || summary.Channel != 2 || summary.Key != 1 {
		t.Fatalf("unexpected exploration summary: %+v", summary)
	}
}
