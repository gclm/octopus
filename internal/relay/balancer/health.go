package balancer

import (
	"fmt"
	"sync"
	"time"
)

const (
	healthScoreGood    = 10  // 健康池阈值
	healthScoreWarning = -20 // 观察池下限
	healthScoreBad     = -50 // 隔离池下限

	healthDecayInterval = 10 * time.Minute
	healthDecayStep     = 3
)

type healthEntry struct {
	mu           sync.Mutex
	Score        int
	AvgLatencyMs int64
	SuccessCount int64
	FailureCount int64
	LastUpdate   time.Time
}

var healthStats sync.Map // key: "channelID:modelName" -> *healthEntry

func healthKey(channelID int, modelName string) string {
	return fmt.Sprintf("%d:%s", channelID, modelName)
}

func getHealthEntry(channelID int, modelName string) *healthEntry {
	key := healthKey(channelID, modelName)
	if v, ok := healthStats.Load(key); ok {
		return v.(*healthEntry)
	}
	entry := &healthEntry{}
	actual, _ := healthStats.LoadOrStore(key, entry)
	return actual.(*healthEntry)
}

// RecordHealthSuccess 记录成功，更新健康分
func RecordHealthSuccess(channelID int, modelName string, latencyMs int64) {
	entry := getHealthEntry(channelID, modelName)
	entry.recordSuccess(latencyMs)
}

// RecordHealthFailure 记录失败，更新健康分
func RecordHealthFailure(channelID int, modelName string) {
	entry := getHealthEntry(channelID, modelName)
	entry.recordFailure()
}

// GetHealthScore 获取当前健康分
func GetHealthScore(channelID int, modelName string) int {
	entry := getHealthEntry(channelID, modelName)
	return entry.getScore()
}

// GetHealthAvgLatency 获取平均延迟
func GetHealthAvgLatency(channelID int, modelName string) int64 {
	entry := getHealthEntry(channelID, modelName)
	return entry.getAvgLatency()
}

func (e *healthEntry) getScore() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.decayLocked(time.Now())
	return e.Score
}

func (e *healthEntry) getAvgLatency() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.AvgLatencyMs
}

func (e *healthEntry) recordSuccess(latencyMs int64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	e.decayLocked(now)

	// 加分：健康+2，负分+5（恢复加速）
	inc := 2
	if e.Score < 0 {
		inc = 5
	}
	e.Score = min(e.Score+inc, 100)
	e.SuccessCount++

	// 更新延迟（EMA）
	if e.AvgLatencyMs == 0 {
		e.AvgLatencyMs = latencyMs
	} else {
		e.AvgLatencyMs = int64(float64(e.AvgLatencyMs)*0.7 + float64(latencyMs)*0.3)
	}
	e.LastUpdate = now
}

func (e *healthEntry) recordFailure() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	e.decayLocked(now)

	// 扣分：-25
	e.Score = max(e.Score-25, -100)
	e.FailureCount++
	e.LastUpdate = now
}

func (e *healthEntry) decayLocked(now time.Time) {
	if e.Score == 0 || e.LastUpdate.IsZero() {
		return
	}
	elapsed := now.Sub(e.LastUpdate)
	steps := int(elapsed / healthDecayInterval)
	if steps <= 0 {
		return
	}

	decay := steps * healthDecayStep
	if e.Score > 0 {
		e.Score = max(e.Score-decay, 0)
	} else {
		e.Score = min(e.Score+decay, 0)
	}
}

// HealthInfo 单个渠道+模型的健康信息
type HealthInfo struct {
	Score        int   `json:"score"`
	AvgLatencyMs int64 `json:"avg_latency_ms"`
	SuccessCount int64 `json:"success_count"`
	FailureCount int64 `json:"failure_count"`
}

// GetHealthInfos 批量获取健康信息
func GetHealthInfos(keys []struct {
	ChannelID int
	ModelName string
}) map[string]HealthInfo {
	result := make(map[string]HealthInfo, len(keys))
	for _, k := range keys {
		entry := getHealthEntry(k.ChannelID, k.ModelName)
		entry.mu.Lock()
		entry.decayLocked(time.Now())
		info := HealthInfo{
			Score:        entry.Score,
			AvgLatencyMs: entry.AvgLatencyMs,
			SuccessCount: entry.SuccessCount,
			FailureCount: entry.FailureCount,
		}
		entry.mu.Unlock()
		result[healthKey(k.ChannelID, k.ModelName)] = info
	}
	return result
}

// resetHealthStatsForTest clears all health stats; test-only helper.
func resetHealthStatsForTest() {
	healthStats = sync.Map{}
}
