package op

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gclm/octopus/internal/db"
	"github.com/gclm/octopus/internal/model"
	"github.com/gclm/octopus/internal/utils/log"
	"github.com/gclm/octopus/internal/utils/snowflake"
)

const relayLogMaxSize = 20
const relayLogMaxSizeNoDB = 100 // 当不保存到数据库时，允许更大的缓存用于实时查询

var relayLogCache = make([]model.RelayLog, 0, relayLogMaxSize)
var relayLogCacheLock sync.Mutex

var relayLogFlushLock sync.Mutex

var relayLogSubscribers = make(map[chan model.RelayLog]struct{})
var relayLogSubscribersLock sync.RWMutex

var relayLogStreamTokens = make(map[string]struct{})
var relayLogStreamTokensLock sync.RWMutex

func RelayLogStreamTokenCreate() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)

	relayLogStreamTokensLock.Lock()
	relayLogStreamTokens[token] = struct{}{}
	relayLogStreamTokensLock.Unlock()

	return token, nil
}

func RelayLogStreamTokenVerify(token string) bool {
	relayLogStreamTokensLock.RLock()
	_, ok := relayLogStreamTokens[token]
	relayLogStreamTokensLock.RUnlock()
	return ok
}

func RelayLogStreamTokenRevoke(token string) {
	relayLogStreamTokensLock.Lock()
	delete(relayLogStreamTokens, token)
	relayLogStreamTokensLock.Unlock()
}

func RelayLogSubscribe() chan model.RelayLog {
	ch := make(chan model.RelayLog, 10)
	relayLogSubscribersLock.Lock()
	relayLogSubscribers[ch] = struct{}{}
	relayLogSubscribersLock.Unlock()
	return ch
}

func RelayLogUnsubscribe(ch chan model.RelayLog) {
	relayLogSubscribersLock.Lock()
	delete(relayLogSubscribers, ch)
	relayLogSubscribersLock.Unlock()
	close(ch)
}

func notifySubscribers(relayLog model.RelayLog) {
	relayLogSubscribersLock.RLock()
	defer relayLogSubscribersLock.RUnlock()

	for ch := range relayLogSubscribers {
		select {
		case ch <- relayLog:
		default:
		}
	}
}

func relayLogFlushToDB(ctx context.Context) error {
	relayLogFlushLock.Lock()
	defer relayLogFlushLock.Unlock()

	relayLogCacheLock.Lock()
	if len(relayLogCache) == 0 {
		relayLogCacheLock.Unlock()
		return nil
	}
	batch := make([]model.RelayLog, len(relayLogCache))
	copy(batch, relayLogCache)
	flushedUpto := len(batch)
	relayLogCacheLock.Unlock()

	result := db.GetDB().WithContext(ctx).Create(&batch)
	if result.Error != nil {
		return result.Error
	}

	relayLogCacheLock.Lock()
	if len(relayLogCache) >= flushedUpto {
		relayLogCache = relayLogCache[flushedUpto:]
	} else {
		relayLogCache = relayLogCache[:0]
	}
	if len(relayLogCache) == 0 {
		relayLogCache = make([]model.RelayLog, 0, relayLogMaxSize)
	}
	relayLogCacheLock.Unlock()

	return nil
}

func RelayLogAdd(ctx context.Context, relayLog model.RelayLog) error {
	enabled, err := SettingGetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return err
	}
	relayLog.ID = snowflake.GenerateID()
	go notifySubscribers(relayLog)

	// 使用异步 Sink（如果已初始化）
	if enabled && logSink != nil {
		logSink.Send(relayLog)
		// 同时保留缓存用于实时查询
		relayLogCacheLock.Lock()
		relayLogCache = append(relayLogCache, relayLog)
		if len(relayLogCache) > relayLogMaxSizeNoDB {
			relayLogCache = relayLogCache[len(relayLogCache)-relayLogMaxSizeNoDB/2:]
		}
		relayLogCacheLock.Unlock()
		return nil
	}

	// Fallback：同步缓存逻辑
	maxSize := relayLogMaxSize
	if !enabled {
		maxSize = relayLogMaxSizeNoDB
	}

	relayLogCacheLock.Lock()
	relayLogCache = append(relayLogCache, relayLog)
	if len(relayLogCache) >= maxSize {
		if enabled {
			relayLogCacheLock.Unlock()
			return relayLogFlushToDB(ctx)
		}
		// 如果未启用日志保存，移除最旧的日志，保留最新的日志用于实时查询
		keepSize := maxSize / 2
		if len(relayLogCache) > keepSize {
			relayLogCache = relayLogCache[len(relayLogCache)-keepSize:]
		}
	}
	relayLogCacheLock.Unlock()
	return nil
}

func RelayLogSaveDBTask(ctx context.Context) error {
	log.Debugf("relay log save db task started")
	startTime := time.Now()
	defer func() {
		log.Debugf("relay log save db task finished, save time: %s", time.Since(startTime))
	}()
	enabled, err := SettingGetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return err
	}

	if enabled {
		if err := relayLogFlushToDB(ctx); err != nil {
			return err
		}
		return relayLogCleanup(ctx)
	}

	// 如果未启用日志保存，检查缓存大小，如果超过限制则清理旧日志
	relayLogCacheLock.Lock()
	if len(relayLogCache) > relayLogMaxSizeNoDB {
		keepSize := relayLogMaxSizeNoDB / 2
		relayLogCache = relayLogCache[len(relayLogCache)-keepSize:]
	}
	relayLogCacheLock.Unlock()

	return nil
}

func relayLogCleanup(ctx context.Context) error {
	keepPeriod, err := SettingGetInt(model.SettingKeyRelayLogKeepPeriod)
	if err != nil {
		return err
	}

	if keepPeriod <= 0 {
		return nil
	}

	cutoffTime := time.Now().Add(-time.Duration(keepPeriod) * 24 * time.Hour).Unix()
	return db.GetDB().WithContext(ctx).Where("time < ?", cutoffTime).Delete(&model.RelayLog{}).Error
}

// LogListQuery 日志列表查询参数
type LogListQuery struct {
	StartTime *int
	EndTime   *int
	Model     string
	ChannelID *int
	RequestID string
	Status    string // "success" or "error"
}

// RelayLogList 查询日志列表，支持多维度过滤
func RelayLogList(ctx context.Context, q *LogListQuery, page, pageSize int) ([]model.RelayLog, error) {
	enabled, err := SettingGetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return nil, err
	}
	hasTimeFilter := q.StartTime != nil && q.EndTime != nil

	// 构建数据库查询
	query := db.GetDB().WithContext(ctx)
	if hasTimeFilter {
		query = query.Where("time >= ? AND time <= ?", *q.StartTime, *q.EndTime)
	}
	if q.Model != "" {
		query = query.Where("request_model_name = ?", q.Model)
	}
	if q.ChannelID != nil {
		query = query.Where("channel_id = ?", *q.ChannelID)
	}
	if q.RequestID != "" {
		query = query.Where("request_id = ? OR client_request_id = ?", q.RequestID, q.RequestID)
	}
	if q.Status == "success" {
		query = query.Where("error = '' OR error IS NULL")
	} else if q.Status == "error" {
		query = query.Where("error != '' AND error IS NOT NULL")
	}

	// 列表不需要加载大文本字段
	listColumns := "id, time, request_id, client_request_id, request_model_name, request_api_key_name, channel_id, channel_name, actual_model_name, input_tokens, output_tokens, ftut, use_time, cost, error, attempts, total_attempts"

	// 获取缓存中符合条件的日志
	relayLogCacheLock.Lock()
	var cachedLogs []model.RelayLog
	for _, entry := range relayLogCache {
		if hasTimeFilter && (entry.Time < int64(*q.StartTime) || entry.Time > int64(*q.EndTime)) {
			continue
		}
		if q.Model != "" && entry.RequestModelName != q.Model {
			continue
		}
		if q.ChannelID != nil && entry.ChannelId != *q.ChannelID {
			continue
		}
		if q.RequestID != "" && entry.RequestID != q.RequestID && entry.ClientRequestID != q.RequestID {
			continue
		}
		if q.Status == "success" && entry.Error != "" {
			continue
		}
		if q.Status == "error" && entry.Error == "" {
			continue
		}
		// 清除大字段，减少内存占用
		entry.RequestContent = ""
		entry.ResponseContent = ""
		cachedLogs = append(cachedLogs, entry)
	}
	relayLogCacheLock.Unlock()

	// 反转缓存日志顺序（原本新的在末尾，反转后新的在前面，方便分页）
	for i, j := 0, len(cachedLogs)-1; i < j; i, j = i+1, j-1 {
		cachedLogs[i], cachedLogs[j] = cachedLogs[j], cachedLogs[i]
	}

	cacheCount := len(cachedLogs)
	offset := (page - 1) * pageSize

	var result []model.RelayLog

	// 先从缓存中取（缓存是最新的日志）
	if offset < cacheCount {
		cacheEnd := offset + pageSize
		if cacheEnd > cacheCount {
			cacheEnd = cacheCount
		}
		result = append(result, cachedLogs[offset:cacheEnd]...)
	}

	// 如果启用了日志保存，缓存不够时从数据库补充
	if enabled {
		remaining := pageSize - len(result)
		if remaining > 0 {
			dbOffset := 0
			if offset > cacheCount {
				dbOffset = offset - cacheCount
			}

			var dbLogs []model.RelayLog
			if err := query.Select(listColumns).Order("id DESC").Offset(dbOffset).Limit(remaining).Find(&dbLogs).Error; err != nil {
				return nil, err
			}
			result = append(result, dbLogs...)
		}
	}

	return result, nil
}

// RelayLogDetail 根据 ID 获取单条日志详情（包含请求和响应内容）
func RelayLogDetail(ctx context.Context, id int64) (*model.RelayLog, error) {
	// 先从缓存查找
	relayLogCacheLock.Lock()
	for _, entry := range relayLogCache {
		if entry.ID == id {
			result := entry
			relayLogCacheLock.Unlock()
			return &result, nil
		}
	}
	relayLogCacheLock.Unlock()

	// 缓存未命中，从数据库查询
	var log model.RelayLog
	if err := db.GetDB().WithContext(ctx).Where("id = ?", id).First(&log).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

func RelayLogClear(ctx context.Context) error {
	relayLogCacheLock.Lock()
	relayLogCache = make([]model.RelayLog, 0, relayLogMaxSize)
	relayLogCacheLock.Unlock()
	return db.GetDB().WithContext(ctx).Where("1 = 1").Delete(&model.RelayLog{}).Error
}

// asyncLogSink 异步批量日志写入器
type asyncLogSink struct {
	queue     chan model.RelayLog
	batchSize int
	dropped   atomic.Int64 // 丢弃计数（backpressure）
}

var logSink *asyncLogSink

// InitLogSink 初始化异步日志 Sink，在应用启动时调用
func InitLogSink() {
	logSink = &asyncLogSink{
		queue:     make(chan model.RelayLog, 5000),
		batchSize: 200,
	}
	go logSink.run()
}

// GetLogSinkDropped 返回丢弃计数
func GetLogSinkDropped() int64 {
	if logSink == nil {
		return 0
	}
	return logSink.dropped.Load()
}

func (s *asyncLogSink) run() {
	batch := make([]model.RelayLog, 0, s.batchSize)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case entry := <-s.queue:
			batch = append(batch, entry)
			if len(batch) >= s.batchSize {
				s.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				s.flush(batch)
				batch = batch[:0]
			}
		}
	}
}

func (s *asyncLogSink) flush(batch []model.RelayLog) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.GetDB().WithContext(ctx).CreateInBatches(batch, 50).Error; err != nil {
		log.Errorf("async log sink flush error: %v", err)
	}
}

func (s *asyncLogSink) Send(entry model.RelayLog) {
	select {
	case s.queue <- entry:
	default:
		s.dropped.Add(1)
	}
}
