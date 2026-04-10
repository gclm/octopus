package op

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bestruirui/octopus/internal/utils/log"
)

type channelFailureState struct {
	mu            sync.Mutex
	Count         int
	LastFailureAt time.Time
}

var (
	channelFailures      sync.Map // channelID (int) -> *channelFailureState
	autoDisabledChannels sync.Map // channelID (int) -> struct{}
)

// RecordChannelFailure 记录渠道级失败，返回 true 表示达到阈值应触发自动暂停。
func RecordChannelFailure(channelID int, threshold int) (triggered bool) {
	v, _ := channelFailures.LoadOrStore(channelID, &channelFailureState{})
	state := v.(*channelFailureState)

	state.mu.Lock()
	defer state.mu.Unlock()

	state.Count++
	state.LastFailureAt = time.Now()

	if state.Count >= threshold {
		triggered = true
		state.Count = 0 // 重置，防止同一批次重复触发
	}
	return
}

// AutoDisableChannel 自动暂停渠道：DB + 缓存 + 标记追踪。
func AutoDisableChannel(channelID int, ctx context.Context) error {
	ch, ok := channelCache.Get(channelID)
	if !ok {
		return fmt.Errorf("channel %d not found", channelID)
	}
	if !ch.Enabled {
		return nil // 已经禁用，无需操作
	}

	if err := ChannelEnabled(channelID, false, ctx); err != nil {
		return fmt.Errorf("failed to auto-disable channel %d: %w", channelID, err)
	}

	autoDisabledChannels.Store(channelID, struct{}{})
	log.Warnf("channel %d (%s) auto-paused: reached failure threshold", channelID, ch.Name)
	return nil
}

// IsAutoDisabled 返回渠道是否被系统自动暂停。
func IsAutoDisabled(channelID int) bool {
	_, ok := autoDisabledChannels.Load(channelID)
	return ok
}

// GetAutoDisabledChannels 返回所有被自动暂停的渠道 ID。
func GetAutoDisabledChannels() []int {
	var ids []int
	autoDisabledChannels.Range(func(key, _ any) bool {
		ids = append(ids, key.(int))
		return true
	})
	return ids
}

// RemoveAutoDisabled 从自动暂停追踪中移除（用户手动启用或已恢复时调用）。
func RemoveAutoDisabled(channelID int) {
	autoDisabledChannels.Delete(channelID)
}

// ResetChannelFailure 重置渠道失败计数（用户手动启用时调用）。
func ResetChannelFailure(channelID int) {
	channelFailures.Delete(channelID)
}
