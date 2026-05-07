package balancer

import (
	"sort"
	"sync/atomic"

	"github.com/gclm/octopus/internal/model"
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
	case model.GroupModeLoadShare:
		return &LoadShare{}
	default:
		return &Fallback{}
	}
}

// Fallback 故障转移：按优先级排序，同优先级内轮询
type Fallback struct{}

func (b *Fallback) Candidates(items []model.GroupItem) []model.GroupItem {
	n := len(items)
	if n == 0 {
		return nil
	}

	// 按优先级分组
	type priorityGroup struct {
		priority int
		items    []model.GroupItem
	}

	groups := make(map[int]*priorityGroup)
	var priorities []int
	for _, item := range items {
		p := item.Priority
		if _, ok := groups[p]; !ok {
			groups[p] = &priorityGroup{priority: p}
			priorities = append(priorities, p)
		}
		groups[p].items = append(groups[p].items, item)
	}
	sort.Ints(priorities)

	// 同优先级内轮询
	result := make([]model.GroupItem, 0, n)
	for _, p := range priorities {
		g := groups[p]
		if len(g.items) == 1 {
			result = append(result, g.items[0])
			continue
		}
		// 多个同优先级：轮询偏移
		idx := int(atomic.AddUint64(&roundRobinCounter, 1) % uint64(len(g.items)))
		for i := 0; i < len(g.items); i++ {
			result = append(result, g.items[(idx+i)%len(g.items)])
		}
	}
	return result
}

// LoadShare 负载均衡：按权重分配，结合健康分降级
type LoadShare struct{}

func (b *LoadShare) Candidates(items []model.GroupItem) []model.GroupItem {
	n := len(items)
	if n == 0 {
		return nil
	}

	type scoredItem struct {
		item    model.GroupItem
		score   int
		latency int64
		weight  int
	}

	// 分层：健康池 / 观察池 / 隔离池 / 垃圾池
	var good, warning, bad, garbage []scoredItem

	for _, item := range items {
		entry := getHealthEntry(item.ChannelID, item.ModelName)
		score := entry.getScore()
		latency := entry.getAvgLatency()
		w := item.Weight
		if w <= 0 {
			w = 1
		}

		si := scoredItem{item: item, score: score, latency: latency, weight: w}

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

	// 每组内按权重×健康分综合排序
	sortByScore := func(items []scoredItem) {
		sort.Slice(items, func(i, j int) bool {
			scoreI := float64(items[i].weight) * float64(max(items[i].score, 1))
			scoreJ := float64(items[j].weight) * float64(max(items[j].score, 1))
			if scoreI != scoreJ {
				return scoreI > scoreJ
			}
			return items[i].latency < items[j].latency
		})
	}
	sortByScore(good)
	sortByScore(warning)
	sortByScore(bad)
	sortByScore(garbage)

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
