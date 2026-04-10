package task

import (
	"context"
	"strings"
	"time"

	"github.com/bestruirui/octopus/internal/model"
	"github.com/bestruirui/octopus/internal/op"
	"github.com/bestruirui/octopus/internal/relay/balancer"
	"github.com/bestruirui/octopus/internal/utils/log"
)

// ChannelAutoPauseRecoveryTask 检查自动暂停的渠道，若健康分已恢复则重新启用。
func ChannelAutoPauseRecoveryTask() {
	interval, err := op.SettingGetInt(model.SettingKeyAutoPauseInterval)
	if err != nil || interval <= 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	const recoveryThreshold = -20 // healthScoreWarning

	var reEnabled []int
	for _, channelID := range op.GetAutoDisabledChannels() {
		ch, err := op.ChannelGet(channelID, ctx)
		if err != nil {
			log.Warnf("auto-pause recovery: channel %d not found, removing from tracking", channelID)
			op.RemoveAutoDisabled(channelID)
			continue
		}
		if ch.Enabled {
			op.RemoveAutoDisabled(channelID)
			op.ResetChannelFailure(channelID)
			continue
		}

		modelNames := collectChannelModelNames(ch)
		if len(modelNames) == 0 {
			log.Warnf("auto-pause recovery: channel %d has no models, skipping", channelID)
			continue
		}

		allRecovered := true
		for _, modelName := range modelNames {
			score := balancer.GetHealthScore(channelID, modelName)
			if score < recoveryThreshold {
				allRecovered = false
				break
			}
		}

		if allRecovered {
			if err := op.ChannelEnabled(channelID, true, ctx); err != nil {
				log.Errorf("auto-pause recovery: failed to re-enable channel %d: %v", channelID, err)
				continue
			}
			op.RemoveAutoDisabled(channelID)
			op.ResetChannelFailure(channelID)
			reEnabled = append(reEnabled, channelID)
			log.Infof("channel %d (%s) auto-resumed: health scores recovered", channelID, ch.Name)
		}
	}

	if len(reEnabled) > 0 {
		log.Infof("channel auto-pause recovery: re-enabled %d channels: %v", len(reEnabled), reEnabled)
	}
}

func collectChannelModelNames(ch *model.Channel) []string {
	if ch == nil {
		return nil
	}
	seen := make(map[string]bool)
	var names []string

	for _, n := range splitModelString(ch.Model) {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	for _, n := range splitModelString(ch.CustomModel) {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	return names
}

func splitModelString(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
