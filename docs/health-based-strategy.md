# HealthBased 负载均衡策略设计

## 背景

现有策略：
- **RoundRobin**：轮询，均匀分摊
- **Failover**：按优先级主备切换
- **Weighted**：按手动配置权重分配

问题：缺少基于**运行时健康状态**的智能选择策略。

## 设计目标

1. **健康优先**：优先选择历史表现好的渠道
2. **快速隔离**：不健康的渠道快速降级，减少重试次数
3. **最多 2-3 次重试**：排序后前 2-3 个候选必须是高概率成功的
4. **独立策略**：不与 Weighted 混淆，保持手动权重的原始语义

## 策略职责分离

| 策略 | 用户意图 | 排序依据 |
|------|---------|----------|
| RoundRobin | 均匀分摊 | 轮转位置 |
| Failover | 主备容灾 | Priority |
| Weighted | 按配置分配 | 手动 Weight |
| **HealthBased** | 智能择优 | 运行时健康分 |

## 核心设计

### 1. 健康分定义

```
范围：-100 ~ 100
  0  = 中性（新渠道/长时间未使用）
 >0  = 健康
 <0  = 不健康
```

### 2. 分数变化规则

**成功（缓慢加分）**：
- 健康渠道：+2
- 负分渠道：+5（恢复加速）

**失败（快速扣分）**：
- 一次失败：-25

**时间衰减**：
- 每 10 分钟向 0 靠拢 3 分

### 3. 分层候选池

```
健康池 (score >= 10)    → 优先使用
观察池 (-20 <= score < 10) → 降级使用
隔离池 (-50 <= score < -20) → 兜底使用
垃圾池 (score < -50)     → 最后手段
```

**效果**：
- 新渠道 score = 0，属于观察池
- 5 次成功 → score = 10 → 进入健康池
- 1 次失败 → score = -15 → 降级到观察池
- 2 次失败 → score = -40 → 降级到隔离池
- 3 次失败 → score = -65 → 排到最后

## 代码实现

### health.go

```go
package balancer

import (
	"fmt"
	"sync"
	"time"
)

const (
	healthScoreGood    = 10   // 健康池阈值
	healthScoreWarning = -20  // 观察池下限
	healthScoreBad     = -50  // 隔离池下限

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
```

### balancer.go 新增

```go
// HealthBased 健康分优先策略
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

	// 分层
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

	// 合并
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
```

### GetBalancer 更新

```go
func GetBalancer(mode model.GroupMode) Balancer {
	switch mode {
	case model.GroupModeRoundRobin:
		return &RoundRobin{}
	case model.GroupModeFailover:
		return &Failover{}
	case model.GroupModeWeighted:
		return &Weighted{}
	case model.GroupModeHealthBased: // 新增
		return &HealthBased{}
	default:
		return &RoundRobin{}
	}
}
```

### model/group.go 新增

```go
const (
	GroupModeRoundRobin   GroupMode = 1
	GroupModeRandom       GroupMode = 2 // 可删除
	GroupModeFailover     GroupMode = 3
	GroupModeWeighted     GroupMode = 4
	GroupModeHealthBased  GroupMode = 5 // 新增
)
```

## 与现有机制的配合

### 1. 粘性会话 (Sticky Session)

**现有机制**：同一个 API Key + Model 在 TTL 内优先使用同一个渠道

**配合方式**：
```
请求进入
    ↓
检查粘性会话 → 有 → 优先使用粘性渠道
    ↓ 无
HealthBased 排序
    ↓
遍历候选 + 熔断检查
    ↓
成功 → 更新健康分 + 设置粘性
失败 → 更新健康分 + 熔断
```

**结论**：粘性会话和健康分是**互补的**，不冲突。

### 2. 熔断器 (Circuit Breaker)

**现有机制**：连续失败触发熔断，强制跳过

**配合方式**：
- HealthBased 排序 → 优先选健康的
- 熔断器 → 极端情况强制保护

**结论**：熔断器是**兜底机制**，健康分是**主动优化**。

## 持久化策略

### 结论：不需要持久化

健康分采用**纯内存存储**，不需要落库到 SQLite。

### 原因分析

| 因素 | 说明 |
|------|------|
| **重建速度快** | 5 次成功进入健康池，2-3 次失败降级，几分钟内恢复准确排序 |
| **时效性强** | 健康分反映"近期"表现，重启前的数据可能已过时 |
| **时间衰减机制** | 即使持久化，长时间不用也会自动衰减到 0（中性） |
| **写开销大** | 每次请求都更新健康分，SQLite 写并发有限，会增加延迟 |
| **单实例部署** | SQLite 暗示单实例部署，无需跨实例共享状态 |

### 服务重启后的自愈过程

```
服务重启
    ↓
所有渠道 score = 0（观察池）
    ↓
第一次请求 → 选第一个（按原始顺序）
    ↓
成功 → score +2
失败 → score -25
    ↓
几分钟内自然形成正确的健康排序
    ↓
稳定渠道进入健康池，不稳定渠道掉入隔离池
```

### 数据存储对比

| 数据类型 | 存储方式 | 原因 |
|----------|----------|------|
| 熔断状态 | ❌ 内存 | 重建快，重启后重新计数 |
| 健康分 | ❌ 内存 | 重建快，时效性强 |
| 粘性会话 | ❌ 内存 | TTL 过期自动失效 |
| 统计数据 (Stats*) | ✅ SQLite | 已有表，需要持久化 |
| 日志 (RelayLog) | ✅ SQLite | 已有表，需要持久化 |

### 如果未来需要持久化

如果后续有多实例部署需求，可以考虑：

1. **Redis**：跨实例共享，读写快
2. **定期快照**：每分钟将健康分快照到 SQLite，重启后加载（非实时）

但目前单实例场景下，纯内存方案是最优解。

## 运行效果预估

假设 34 个渠道，其中 5 个稳定，29 个不稳定：

| 阶段 | 健康池 | 观察池 | 隔离池 | 平均重试次数 |
|------|--------|--------|--------|--------------|
| 初始 | 0 | 34 | 0 | 不确定（看运气） |
| 运行 10 分钟 | 5 | 10 | 19 | 1-2 |
| 某健康渠道挂了 | 4 | 10 | 20 | 1-2 |

**关键效果**：
1. 稳定的渠道快速进入健康池（5 次成功）
2. 不稳定的渠道快速掉入隔离池（2-3 次失败）
3. 前几名大概率是健康的，重试次数控制在 2-3 次内

## 待删除

如果采用此方案，建议删除：
- **Random 策略**：与 RoundRobin 功能重复
