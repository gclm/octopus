package balancer

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gclm/octopus/internal/model"
	"github.com/gclm/octopus/internal/op"
	"github.com/gclm/octopus/internal/utils/log"
)

var testSettingIntOverride func(model.SettingKey) (int, bool)

// CircuitState 熔断器状态
type CircuitState int

const (
	StateClosed   CircuitState = iota // 正常通行
	StateOpen                         // 熔断中，拒绝所有请求
	StateHalfOpen                     // 半开，仅允许单个试探请求
)

type FailureKind string

const (
	FailureUnknown           FailureKind = "unknown"
	FailureFirstTokenTimeout FailureKind = "first_token_timeout"
	FailureNetworkTimeout    FailureKind = "network_timeout"
	FailureTLSError          FailureKind = "tls_error"
	FailureAuthError         FailureKind = "auth_error"
	FailureModelNotFound     FailureKind = "model_not_found"
	FailureUpstream5xx       FailureKind = "upstream_5xx"
	FailureProtocolError     FailureKind = "protocol_error"
	FailureNetworkError      FailureKind = "network_error"
	FailureClientError       FailureKind = "client_error"
)

const (
	defaultHealthScoreMin        = -100
	defaultHealthScoreMax        = 100
	defaultHealthDecayStep       = 5
	defaultHealthDecayInterval   = 10 * time.Minute
	defaultHealthWarmupSuccesses = 3
)

// circuitEntry 单个熔断器条目
type circuitEntry struct {
	State               CircuitState
	ConsecutiveFailures int64
	LastFailureTime     time.Time
	LastSuccessTime     time.Time
	TripCount           int // 累计熔断触发次数（用于指数退避）
	HealthScore         int
	SuccessCount        int64
	LastFailureKind     FailureKind
	OpenUntil           time.Time
	mu                  sync.Mutex
}

// 全局熔断器存储
var globalBreaker sync.Map // key: string -> value: *circuitEntry

// circuitKey 生成熔断器键：channelID:channelKeyID:modelName
func circuitKey(channelID, keyID int, modelName string) string {
	return fmt.Sprintf("%d:%d:%s", channelID, keyID, modelName)
}

// getOrCreateEntry 获取或创建熔断器条目
func getOrCreateEntry(key string) *circuitEntry {
	if v, ok := globalBreaker.Load(key); ok {
		return v.(*circuitEntry)
	}
	entry := &circuitEntry{State: StateClosed}
	actual, _ := globalBreaker.LoadOrStore(key, entry)
	return actual.(*circuitEntry)
}

// getThreshold 获取熔断阈值配置
func getThreshold() int64 {
	if testSettingIntOverride != nil {
		if v, ok := testSettingIntOverride(model.SettingKeyCircuitBreakerThreshold); ok && v > 0 {
			return int64(v)
		}
	}
	v, err := op.SettingGetInt(model.SettingKeyCircuitBreakerThreshold)
	if err != nil || v <= 0 {
		return 5
	}
	return int64(v)
}

func getHealthScoreThreshold() int {
	if testSettingIntOverride != nil {
		if v, ok := testSettingIntOverride(model.SettingKeyCircuitBreakerHealthScoreThreshold); ok {
			if v >= 0 {
				return -50
			}
			return v
		}
	}
	v, err := op.SettingGetInt(model.SettingKeyCircuitBreakerHealthScoreThreshold)
	if err != nil || v >= 0 {
		return -50
	}
	return v
}

func getHealthScoreRange() (int, int) {
	if testSettingIntOverride != nil {
		minV, okMin := testSettingIntOverride(model.SettingKeyCircuitBreakerHealthScoreMin)
		maxV, okMax := testSettingIntOverride(model.SettingKeyCircuitBreakerHealthScoreMax)
		if okMin && okMax {
			if minV >= maxV {
				return defaultHealthScoreMin, defaultHealthScoreMax
			}
			return minV, maxV
		}
	}
	minV, err := op.SettingGetInt(model.SettingKeyCircuitBreakerHealthScoreMin)
	if err != nil {
		minV = defaultHealthScoreMin
	}
	maxV, err := op.SettingGetInt(model.SettingKeyCircuitBreakerHealthScoreMax)
	if err != nil {
		maxV = defaultHealthScoreMax
	}
	if minV >= maxV {
		return defaultHealthScoreMin, defaultHealthScoreMax
	}
	return minV, maxV
}

func getHealthDecayStep() int {
	if testSettingIntOverride != nil {
		if v, ok := testSettingIntOverride(model.SettingKeyCircuitBreakerHealthScoreDecayStep); ok && v > 0 {
			return v
		}
	}
	v, err := op.SettingGetInt(model.SettingKeyCircuitBreakerHealthScoreDecayStep)
	if err != nil || v <= 0 {
		return defaultHealthDecayStep
	}
	return v
}

func getHealthDecayInterval() time.Duration {
	if testSettingIntOverride != nil {
		if v, ok := testSettingIntOverride(model.SettingKeyCircuitBreakerHealthScoreDecayIntervalSeconds); ok && v > 0 {
			return time.Duration(v) * time.Second
		}
	}
	v, err := op.SettingGetInt(model.SettingKeyCircuitBreakerHealthScoreDecayIntervalSeconds)
	if err != nil || v <= 0 {
		return defaultHealthDecayInterval
	}
	return time.Duration(v) * time.Second
}

func getHealthWarmupSuccesses() int {
	if testSettingIntOverride != nil {
		if v, ok := testSettingIntOverride(model.SettingKeyCircuitBreakerHealthScoreWarmupSuccesses); ok {
			if v < 0 {
				return 0
			}
			return v
		}
	}
	v, err := op.SettingGetInt(model.SettingKeyCircuitBreakerHealthScoreWarmupSuccesses)
	if err != nil {
		return defaultHealthWarmupSuccesses
	}
	if v < 0 {
		return 0
	}
	return v
}

func decayHealthScore(score int, lastUpdated, now time.Time) int {
	if score == 0 || lastUpdated.IsZero() || !now.After(lastUpdated) {
		return score
	}
	interval := getHealthDecayInterval()
	if interval <= 0 {
		return score
	}
	step := getHealthDecayStep()
	steps := int(now.Sub(lastUpdated) / interval)
	if steps <= 0 {
		return score
	}
	decay := steps * step
	if score > 0 {
		score -= decay
		if score < 0 {
			score = 0
		}
		return score
	}
	score += decay
	if score > 0 {
		score = 0
	}
	return score
}

func (e *circuitEntry) applyDecay(now time.Time) {
	updatedAt := e.LastSuccessTime
	if e.LastFailureTime.After(updatedAt) {
		updatedAt = e.LastFailureTime
	}
	e.HealthScore = clampHealthScore(decayHealthScore(e.HealthScore, updatedAt, now))
}

func rankingHealthScore(score int, successCount int64) int {
	warmupSuccesses := getHealthWarmupSuccesses()
	if warmupSuccesses > 0 && score > 0 && successCount < int64(warmupSuccesses) {
		return 0
	}
	return score
}

// GetCooldown 获取当前冷却时间（带指数退避与动态惩罚）
func GetCooldown(tripCount int, kind FailureKind, healthScore int) time.Duration {
	if testSettingIntOverride != nil {
		base, okBase := testSettingIntOverride(model.SettingKeyCircuitBreakerCooldown)
		maxCooldown, okMax := testSettingIntOverride(model.SettingKeyCircuitBreakerMaxCooldown)
		if okBase && okMax {
			if base <= 0 {
				base = 60
			}
			if maxCooldown <= 0 {
				maxCooldown = 600
			}
			cooldown := base
			if tripCount > 1 {
				shift := tripCount - 1
				if shift > 20 {
					shift = 20
				}
				cooldown = base << shift
			}
			switch kind {
			case FailureTLSError, FailureAuthError:
				if cooldown < 1800 {
					cooldown = 1800
				}
			case FailureModelNotFound:
				if cooldown < 3600 {
					cooldown = 3600
				}
			case FailureFirstTokenTimeout:
				if cooldown < 900 {
					cooldown = 900
				}
			}
			if healthScore <= getHealthScoreThreshold() && cooldown < 600 {
				cooldown = 600
			}
			if healthScore <= -80 && cooldown < 1800 {
				cooldown = 1800
			}
			if cooldown > maxCooldown {
				cooldown = maxCooldown
			}
			return time.Duration(cooldown) * time.Second
		}
	}
	base, err := op.SettingGetInt(model.SettingKeyCircuitBreakerCooldown)
	if err != nil || base <= 0 {
		base = 60
	}
	maxCooldown, err := op.SettingGetInt(model.SettingKeyCircuitBreakerMaxCooldown)
	if err != nil || maxCooldown <= 0 {
		maxCooldown = 600
	}

	// 指数退避：baseCooldown * 2^(tripCount-1)
	cooldown := base
	if tripCount > 1 {
		shift := tripCount - 1
		if shift > 20 { // 防止溢出
			shift = 20
		}
		cooldown = base << shift
	}

	// 对确定性错误给出更强惩罚。
	switch kind {
	case FailureTLSError, FailureAuthError:
		if cooldown < 1800 {
			cooldown = 1800
		}
	case FailureModelNotFound:
		if cooldown < 3600 {
			cooldown = 3600
		}
	case FailureFirstTokenTimeout:
		if cooldown < 900 {
			cooldown = 900
		}
	}

	// 长期健康分过低时，至少进入更长冷却。
	if healthScore <= getHealthScoreThreshold() {
		if cooldown < 600 {
			cooldown = 600
		}
	}
	if healthScore <= -80 {
		if cooldown < 1800 {
			cooldown = 1800
		}
	}

	if cooldown > maxCooldown {
		cooldown = maxCooldown
	}

	return time.Duration(cooldown) * time.Second
}

// IsTripped 检查通道是否处于熔断状态
// 返回 tripped=true 表示该通道应被跳过，remaining 为剩余冷却时间
func IsTripped(channelID, keyID int, modelName string) (tripped bool, remaining time.Duration) {
	key := circuitKey(channelID, keyID, modelName)
	v, ok := globalBreaker.Load(key)
	if !ok {
		return false, 0 // 无记录，视为 Closed
	}
	entry := v.(*circuitEntry)

	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.applyDecay(time.Now())

	switch entry.State {
	case StateClosed:
		return false, 0

	case StateOpen:
		cooldown := GetCooldown(entry.TripCount, entry.LastFailureKind, entry.HealthScore)
		remaining = time.Until(entry.OpenUntil)
		if entry.OpenUntil.IsZero() {
			remaining = cooldown - time.Since(entry.LastFailureTime)
		}
		if remaining <= 0 {
			entry.State = StateHalfOpen
			log.Infof("circuit breaker [%s] Open -> HalfOpen (cooldown %v elapsed, health=%d, kind=%s)", key, cooldown, entry.HealthScore, entry.LastFailureKind)
			return false, 0
		}
		return true, remaining

	case StateHalfOpen:
		// 已有试探请求在进行中，拒绝其他请求
		return true, 0

	default:
		return false, 0
	}
}

func clampHealthScore(score int) int {
	minScore, maxScore := getHealthScoreRange()
	if score < minScore {
		return minScore
	}
	if score > maxScore {
		return maxScore
	}
	return score
}

func healthDeltaForSuccess(firstToken time.Duration) int {
	if firstToken > 0 && firstToken <= 1500*time.Millisecond {
		return 5
	}
	return 3
}

func healthDeltaForFailure(kind FailureKind) int {
	switch kind {
	case FailureTLSError, FailureAuthError:
		return -50
	case FailureModelNotFound:
		return -30
	case FailureFirstTokenTimeout:
		return -20
	case FailureNetworkTimeout:
		return -15
	case FailureProtocolError:
		return -20
	case FailureUpstream5xx:
		return -10
	case FailureClientError:
		return -10
	case FailureNetworkError:
		return -15
	default:
		return -10
	}
}

// RecordSuccess 记录成功，重置熔断器状态并恢复健康分。
func RecordSuccess(channelID, keyID int, modelName string, firstToken time.Duration) {
	key := circuitKey(channelID, keyID, modelName)
	entry := getOrCreateEntry(key)

	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.applyDecay(time.Now())

	if entry.State == StateHalfOpen {
		log.Infof("circuit breaker [%s] HalfOpen -> Closed (probe succeeded, health=%d)", key, entry.HealthScore)
	}

	entry.State = StateClosed
	entry.ConsecutiveFailures = 0
	entry.TripCount = 0
	entry.LastSuccessTime = time.Now()
	entry.SuccessCount++
	entry.LastFailureKind = FailureUnknown
	entry.OpenUntil = time.Time{}
	entry.HealthScore = clampHealthScore(entry.HealthScore + healthDeltaForSuccess(firstToken))
}

// RecordFailure 记录失败，结合错误类型、健康分和指数退避决定冷却时长。
func RecordFailure(channelID, keyID int, modelName string, kind FailureKind) {
	key := circuitKey(channelID, keyID, modelName)
	entry := getOrCreateEntry(key)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := time.Now()
	entry.applyDecay(now)
	entry.LastFailureTime = now
	entry.LastFailureKind = kind
	entry.HealthScore = clampHealthScore(entry.HealthScore + healthDeltaForFailure(kind))

	switch entry.State {
	case StateClosed:
		entry.ConsecutiveFailures++
		threshold := getThreshold()
		if entry.ConsecutiveFailures >= threshold || entry.HealthScore <= getHealthScoreThreshold() || isFatalFailure(kind) {
			entry.State = StateOpen
			entry.TripCount++
			cooldown := GetCooldown(entry.TripCount, kind, entry.HealthScore)
			entry.OpenUntil = now.Add(cooldown)
			log.Warnf("circuit breaker [%s] Closed -> Open (failures=%d threshold=%d tripCount=%d cooldown=%v kind=%s health=%d)",
				key, entry.ConsecutiveFailures, threshold, entry.TripCount, cooldown, kind, entry.HealthScore)
		}

	case StateHalfOpen:
		entry.State = StateOpen
		entry.TripCount++
		entry.ConsecutiveFailures = 0
		cooldown := GetCooldown(entry.TripCount, kind, entry.HealthScore)
		entry.OpenUntil = now.Add(cooldown)
		log.Warnf("circuit breaker [%s] HalfOpen -> Open (probe failed tripCount=%d cooldown=%v kind=%s health=%d)",
			key, entry.TripCount, cooldown, kind, entry.HealthScore)

	case StateOpen:
		cooldown := GetCooldown(entry.TripCount, kind, entry.HealthScore)
		entry.OpenUntil = now.Add(cooldown)
	}
}

func isFatalFailure(kind FailureKind) bool {
	switch kind {
	case FailureTLSError, FailureAuthError, FailureModelNotFound:
		return true
	default:
		return false
	}
}

// HealthScore 返回当前候选健康分，用于排序惩罚。
func HealthScore(channelID, keyID int, modelName string) int {
	if keyID == 0 {
		best := 0
		found := false
		now := time.Now()
		globalBreaker.Range(func(k, v any) bool {
			entryKey, ok := k.(string)
			if !ok {
				return true
			}
			prefix := fmt.Sprintf("%d:", channelID)
			suffix := fmt.Sprintf(":%s", modelName)
			if !strings.HasPrefix(entryKey, prefix) || !strings.HasSuffix(entryKey, suffix) {
				return true
			}
			entry := v.(*circuitEntry)
			entry.mu.Lock()
			entry.applyDecay(now)
			score := entry.HealthScore
			entry.mu.Unlock()
			if !found || score > best {
				best = score
				found = true
			}
			return true
		})
		if found {
			return best
		}
	}

	key := circuitKey(channelID, keyID, modelName)
	v, ok := globalBreaker.Load(key)
	if !ok {
		return 0
	}
	entry := v.(*circuitEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.applyDecay(time.Now())
	return entry.HealthScore
}

// OrderingHealthScore returns the score used for routing decisions.
// Positive scores remain neutral until the candidate has collected enough
// successful samples to leave warm-up.
func OrderingHealthScore(channelID, keyID int, modelName string) int {
	if keyID == 0 {
		best := 0
		found := false
		now := time.Now()
		globalBreaker.Range(func(k, v any) bool {
			entryKey, ok := k.(string)
			if !ok {
				return true
			}
			prefix := fmt.Sprintf("%d:", channelID)
			suffix := fmt.Sprintf(":%s", modelName)
			if !strings.HasPrefix(entryKey, prefix) || !strings.HasSuffix(entryKey, suffix) {
				return true
			}
			entry := v.(*circuitEntry)
			entry.mu.Lock()
			entry.applyDecay(now)
			score := rankingHealthScore(entry.HealthScore, entry.SuccessCount)
			entry.mu.Unlock()
			if !found || score > best {
				best = score
				found = true
			}
			return true
		})
		if found {
			return best
		}
		return 0
	}

	key := circuitKey(channelID, keyID, modelName)
	v, ok := globalBreaker.Load(key)
	if !ok {
		return 0
	}
	entry := v.(*circuitEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.applyDecay(time.Now())
	return rankingHealthScore(entry.HealthScore, entry.SuccessCount)
}

func HealthInfo(channelID, keyID int, modelName string) (score int, kind FailureKind, remaining time.Duration, state CircuitState, ok bool) {
	key := circuitKey(channelID, keyID, modelName)
	v, exists := globalBreaker.Load(key)
	if !exists {
		return 0, FailureUnknown, 0, StateClosed, false
	}
	entry := v.(*circuitEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	now := time.Now()
	entry.applyDecay(now)
	remaining = time.Until(entry.OpenUntil)
	if remaining < 0 {
		remaining = 0
	}
	return entry.HealthScore, entry.LastFailureKind, remaining, entry.State, true
}

// ResetChannelState clears runtime breaker and sticky state for a channel.
// This is useful when channel configuration changes materially and old health
// samples no longer represent the current upstream quality.
func ResetChannelState(channelID int) {
	prefix := fmt.Sprintf("%d:", channelID)
	globalBreaker.Range(func(k, _ any) bool {
		entryKey, ok := k.(string)
		if ok && strings.HasPrefix(entryKey, prefix) {
			globalBreaker.Delete(entryKey)
		}
		return true
	})

	globalSession.Range(func(k, v any) bool {
		entry, ok := v.(*SessionEntry)
		if ok && entry.ChannelID == channelID {
			globalSession.Delete(k)
		}
		return true
	})
}

func ReadyForSticky(channelID, keyID int, modelName string) bool {
	warmupSuccesses := getHealthWarmupSuccesses()
	if warmupSuccesses == 0 {
		return true
	}

	key := circuitKey(channelID, keyID, modelName)
	v, ok := globalBreaker.Load(key)
	if !ok {
		return false
	}
	entry := v.(*circuitEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.applyDecay(time.Now())
	return entry.SuccessCount >= int64(warmupSuccesses)
}

func ClassifyFailure(err error, statusCode int) FailureKind {
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return FailureAuthError
	}
	if statusCode == http.StatusNotFound {
		return FailureModelNotFound
	}
	if statusCode >= 500 {
		return FailureUpstream5xx
	}
	if statusCode >= 400 {
		return FailureClientError
	}

	if err == nil {
		return FailureUnknown
	}

	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "first token timeout") {
		return FailureFirstTokenTimeout
	}
	if strings.Contains(msg, "non-sse content-type") || strings.Contains(msg, "transform stream") {
		return FailureProtocolError
	}
	if strings.Contains(msg, "context deadline exceeded") || strings.Contains(msg, "client.timeout exceeded") {
		return FailureNetworkTimeout
	}

	var certErr x509.UnknownAuthorityError
	if errors.As(err, &certErr) || strings.Contains(msg, "certificate signed by unknown authority") || strings.Contains(msg, "x509:") {
		return FailureTLSError
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return FailureNetworkTimeout
		}
		return FailureNetworkError
	}
	if strings.Contains(msg, "connection refused") || strings.Contains(msg, "no such host") || strings.Contains(msg, "tls:") {
		return FailureNetworkError
	}

	return FailureUnknown
}
