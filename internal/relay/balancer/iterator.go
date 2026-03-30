package balancer

import (
	"fmt"
	"sort"
	"time"

	"github.com/bestruirui/octopus/internal/model"
)

// Iterator 统一的负载均衡迭代器
// 内部编排：策略排序 + 粘性优先 + 决策追踪
type Iterator struct {
	candidates []model.GroupItem
	index      int
	stickyIdx  int    // 粘性通道在 candidates 中的位置，-1 表示无
	modelName  string // 请求模型名（用于熔断检查）

	// 内嵌追踪
	attempts []model.ChannelAttempt
	count    int
	active   *AttemptSpan
}

// NewIterator 创建负载均衡迭代器
// 自动处理：策略排序 + 粘性通道提前
func NewIterator(group model.Group, apiKeyID int, requestModel string) *Iterator {
	b := GetBalancer(group.Mode)
	candidates := b.Candidates(group.Items)
	applyHealthOrder(candidates)

	stickyIdx := -1
	if group.SessionKeepTime > 0 {
		stickyTTL := time.Duration(group.SessionKeepTime) * time.Second
		if sticky := GetSticky(apiKeyID, requestModel, stickyTTL); sticky != nil {
			for i, item := range candidates {
				if item.ChannelID == sticky.ChannelID {
					if i > 0 {
						// 将粘性通道移到最前面
						stickyItem := candidates[i]
						copy(candidates[1:i+1], candidates[0:i])
						candidates[0] = stickyItem
					}
					stickyIdx = 0
					break
				}
			}
		}
	}

	return &Iterator{
		candidates: candidates,
		index:      -1,
		stickyIdx:  stickyIdx,
		modelName:  requestModel,
	}
}

func applyHealthOrder(candidates []model.GroupItem) {
	// 惩罚型全局排序：保留优先级语义，但允许极差健康分在全局范围内适度后移。
	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		leftScore := OrderingHealthScore(left.ChannelID, 0, left.ModelName)
		rightScore := OrderingHealthScore(right.ChannelID, 0, right.ModelName)
		leftRank := effectivePriority(left.Priority, leftScore)
		rightRank := effectivePriority(right.Priority, rightScore)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if leftScore == rightScore {
			return left.ID < right.ID
		}
		return leftScore > rightScore
	})
}

func effectivePriority(priority, score int) int {
	penalty := 0
	switch {
	case score <= -80:
		penalty = 3
	case score <= -50:
		penalty = 2
	case score <= -20:
		penalty = 1
	}
	return priority + penalty
}

func EffectivePriorityFor(channelID, keyID int, item model.GroupItem) int {
	score := OrderingHealthScore(channelID, keyID, item.ModelName)
	return effectivePriority(item.Priority, score)
}

// Next 移动到下一个候选，返回 false 表示遍历完成
func (it *Iterator) Next() bool {
	it.index++
	return it.index < len(it.candidates)
}

// Item 返回当前候选的 GroupItem
func (it *Iterator) Item() model.GroupItem {
	return it.candidates[it.index]
}

// IsSticky 当前候选是否为粘性通道
func (it *Iterator) IsSticky() bool {
	return it.stickyIdx >= 0 && it.index == it.stickyIdx
}

// Len 返回候选列表长度
func (it *Iterator) Len() int {
	return len(it.candidates)
}

// Index 返回当前迭代位置（0-based）
func (it *Iterator) Index() int {
	return it.index
}

// Skip 记录当前通道被跳过（通道禁用、无Key、类型不兼容等）
func (it *Iterator) Skip(channelID, channelKeyID int, channelName, msg string) {
	it.count++
	it.attempts = append(it.attempts, model.ChannelAttempt{
		ChannelID:    channelID,
		ChannelKeyID: channelKeyID,
		ChannelName:  channelName,
		ModelName:    it.candidates[it.index].ModelName,
		AttemptNum:   it.count,
		Status:       model.AttemptSkipped,
		Sticky:       it.IsSticky(),
		Msg:          msg,
	})
}

// SkipCircuitBreak 检查熔断状态，若已熔断自动记录（含剩余冷却时间）并返回 true
func (it *Iterator) SkipCircuitBreak(channelID, channelKeyID int, channelName string) bool {
	modelName := it.candidates[it.index].ModelName
	tripped, remaining := IsTripped(channelID, channelKeyID, modelName)
	if !tripped {
		return false
	}
	msg := "circuit breaker tripped"
	if remaining > 0 {
		health, kind, _, state, _ := HealthInfo(channelID, channelKeyID, modelName)
		effective := EffectivePriorityFor(channelID, channelKeyID, it.candidates[it.index])
		msg = fmt.Sprintf("circuit breaker tripped, state: %d, remaining cooldown: %ds, health: %d, failure_kind: %s, effective_priority: %d", int(state), int(remaining.Seconds()), health, kind, effective)
	}
	it.count++
	it.attempts = append(it.attempts, model.ChannelAttempt{
		ChannelID:    channelID,
		ChannelKeyID: channelKeyID,
		ChannelName:  channelName,
		ModelName:    modelName,
		AttemptNum:   it.count,
		Status:       model.AttemptCircuitBreak,
		Sticky:       it.IsSticky(),
		Msg:          msg,
	})
	return true
}

// StartAttempt 开始一次真实转发尝试，返回 Span 用于记录结果
func (it *Iterator) StartAttempt(channelID, channelKeyID int, channelName string) *AttemptSpan {
	it.count++
	span := &AttemptSpan{
		attempt: model.ChannelAttempt{
			ChannelID:    channelID,
			ChannelKeyID: channelKeyID,
			ChannelName:  channelName,
			ModelName:    it.candidates[it.index].ModelName,
			AttemptNum:   it.count,
			Sticky:       it.IsSticky(),
		},
		startTime: time.Now(),
		iter:      it,
	}
	it.active = span
	return span
}

// Attempts 返回所有决策记录（交给日志模块持久化）
func (it *Iterator) Attempts() []model.ChannelAttempt {
	return it.attempts
}

// AttemptSpan 管理单次通道尝试的生命周期（计时、状态、结果）
type AttemptSpan struct {
	attempt      model.ChannelAttempt
	startTime    time.Time
	firstTokenAt time.Time
	statusCode   int
	iter         *Iterator
	ended        bool
}

func (it *Iterator) CurrentAttempt() *AttemptSpan {
	return it.active
}

// End 结束尝试：设置状态，自动计算耗时，追加到 Iterator
func (s *AttemptSpan) End(status model.AttemptStatus, statusCode int, msg string) {
	if s.ended {
		return
	}
	s.ended = true
	s.statusCode = statusCode
	s.attempt.Status = status
	s.attempt.Duration = int(time.Since(s.startTime).Milliseconds())
	s.attempt.Msg = msg
	s.iter.attempts = append(s.iter.attempts, s.attempt)
	if s.iter.active == s {
		s.iter.active = nil
	}
}

func (s *AttemptSpan) MarkFirstToken() time.Duration {
	if s.firstTokenAt.IsZero() {
		s.firstTokenAt = time.Now()
	}
	return s.firstTokenAt.Sub(s.startTime)
}

func (s *AttemptSpan) FirstTokenDuration() time.Duration {
	if s.firstTokenAt.IsZero() {
		return 0
	}
	return s.firstTokenAt.Sub(s.startTime)
}

// Duration 返回从开始到现在的耗时
func (s *AttemptSpan) Duration() time.Duration {
	return time.Since(s.startTime)
}
