package balancer

import (
	"math/rand"
	"sort"
	"sync/atomic"

	"github.com/bestruirui/octopus/internal/model"
)

var roundRobinCounter uint64

// Balancer 根据负载均衡模式选择通道
type Balancer interface {
	// Candidates 返回按策略排序的候选列表
	// 调用方在遍历候选列表时自行检查熔断状态
	Candidates(items []model.GroupItem) []model.GroupItem
}

// GetBalancer 根据模式返回对应的负载均衡器
func GetBalancer(mode model.GroupMode) Balancer {
	switch mode {
	case model.GroupModeRoundRobin:
		return &RoundRobin{}
	case model.GroupModeFailover:
		return &Failover{}
	case model.GroupModeWeighted:
		return &Weighted{}
	case model.GroupModeHealthBased:
		return &HealthBased{}
	default:
		return &RoundRobin{}
	}
}

// RoundRobin 轮询：从上次位置开始轮转排列
type RoundRobin struct{}

func (b *RoundRobin) Candidates(items []model.GroupItem) []model.GroupItem {
	n := len(items)
	if n == 0 {
		return nil
	}
	idx := int(atomic.AddUint64(&roundRobinCounter, 1) % uint64(n))
	result := make([]model.GroupItem, n)
	for i := 0; i < n; i++ {
		result[i] = items[(idx+i)%n]
	}
	return result
}

// Failover 故障转移：按优先级排序
type Failover struct{}

func (b *Failover) Candidates(items []model.GroupItem) []model.GroupItem {
	if len(items) == 0 {
		return nil
	}
	return sortByPriority(items)
}

// Weighted 加权分配：按权重概率排序
type Weighted struct{}

func (b *Weighted) Candidates(items []model.GroupItem) []model.GroupItem {
	n := len(items)
	if n == 0 {
		return nil
	}

	// 构建加权随机排序
	type weightedItem struct {
		item  model.GroupItem
		score float64
	}

	totalWeight := 0
	for _, item := range items {
		w := item.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}

	scored := make([]weightedItem, n)
	for i, item := range items {
		w := item.Weight
		if w <= 0 {
			w = 1
		}
		// 给每个 item 一个加权随机分数：weight/totalWeight 作为概率基础，加上随机扰动
		scored[i] = weightedItem{
			item:  item,
			score: rand.Float64() * float64(w) / float64(totalWeight),
		}
	}

	// 按分数降序排列（分数越高优先级越高）
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	result := make([]model.GroupItem, n)
	for i := range scored {
		result[i] = scored[i].item
	}
	return result
}

// HealthBased 健康分优先：按运行时健康分排序
type HealthBased struct{}

func (b *HealthBased) Candidates(items []model.GroupItem) []model.GroupItem {
	n := len(items)
	if n == 0 {
		return nil
	}

	type scoredItem struct {
		item    model.GroupItem
		score   int
		latency int64
	}

	// 分层：健康池 / 观察池 / 隔离池 / 垃圾池
	var good, warning, bad, garbage []scoredItem

	for _, item := range items {
		entry := getHealthEntry(item.ChannelID, item.ModelName)
		score := entry.getScore()
		latency := entry.getAvgLatency()

		si := scoredItem{item: item, score: score, latency: latency}

		switch {
		case score >= healthScoreGood:
			good = append(good, si)
		case score >= healthScoreWarning:
			warning = append(warning, si)
		case score >= healthScoreBad:
			bad = append(bad, si)
		default:
			garbage = append(garbage, si)
		}
	}

	// 每组内按延迟排序
	sortByLatency := func(items []scoredItem) {
		sort.Slice(items, func(i, j int) bool {
			return items[i].latency < items[j].latency
		})
	}
	sortByLatency(good)
	sortByLatency(warning)
	sortByLatency(bad)
	sortByLatency(garbage)

	// 合并：健康池 → 观察池 → 隔离池 → 垃圾池
	result := make([]model.GroupItem, 0, n)
	for _, si := range good {
		result = append(result, si.item)
	}
	for _, si := range warning {
		result = append(result, si.item)
	}
	for _, si := range bad {
		result = append(result, si.item)
	}
	for _, si := range garbage {
		result = append(result, si.item)
	}

	return result
}

func sortByPriority(items []model.GroupItem) []model.GroupItem {
	sorted := make([]model.GroupItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})
	return sorted
}
