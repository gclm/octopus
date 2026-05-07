package helper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gclm/octopus/internal/model"
	tmodel "github.com/gclm/octopus/internal/transformer/model"
	"github.com/gclm/octopus/internal/transformer/outbound"
	"github.com/gclm/octopus/internal/utils/xstrings"
	"github.com/samber/lo"
	"github.com/tmaxmax/go-sse"
)

const channelTestConcurrency = 5

type ChannelTestConfig struct {
	Endpoints     []model.Endpoint     `json:"endpoints" binding:"required"`
	Keys          []model.ChannelKey   `json:"keys" binding:"required"`
	Model         string               `json:"model"`
	CustomModel   string               `json:"custom_model"`
	Proxy         bool                 `json:"proxy"`
	CustomHeader  []model.CustomHeader `json:"custom_header,omitempty"`
	ParamOverride *string              `json:"param_override,omitempty"`
	ChannelProxy  *string              `json:"channel_proxy,omitempty"`
}

type ModelTestResult struct {
	Model            string                `json:"model"`
	BaseUrl          string                `json:"base_url"`
	EndpointType     outbound.OutboundType `json:"endpoint_type"`
	Success          bool                  `json:"success"`
	ResponseTimeMs   int64                 `json:"response_time_ms"`
	FirstTokenTimeMs int64                 `json:"first_token_time_ms"`
	Error            string                `json:"error,omitempty"`
}

type ChannelTestTaskInfo struct {
	Model        string                `json:"model"`
	BaseUrl      string                `json:"base_url"`
	EndpointType outbound.OutboundType `json:"endpoint_type"`
}

type StreamEventType string

const (
	StreamEventStart  StreamEventType = "start"
	StreamEventResult StreamEventType = "result"
	StreamEventDone   StreamEventType = "done"
	StreamEventError  StreamEventType = "error"
)

type StreamEvent struct {
	Type       StreamEventType       `json:"type"`
	Tasks      []ChannelTestTaskInfo `json:"tasks,omitempty"`
	Skipped    []string              `json:"skipped,omitempty"`
	SkipReason string                `json:"skip_reason,omitempty"`
	Result     *ModelTestResult      `json:"result,omitempty"`
	Error      string                `json:"error,omitempty"`
	Total      int                   `json:"total,omitempty"`
	Success    int                   `json:"success,omitempty"`
	Failed     int                   `json:"failed,omitempty"`
}

type channelTestTask struct {
	modelName string
	endpoint  model.Endpoint
}

func ChannelToTestConfig(channel *model.Channel) ChannelTestConfig {
	if channel == nil {
		return ChannelTestConfig{}
	}
	return ChannelTestConfig{
		Endpoints:     channel.Endpoints,
		Keys:          channel.Keys,
		Model:         channel.Model,
		CustomModel:   channel.CustomModel,
		Proxy:         channel.Proxy,
		CustomHeader:  channel.CustomHeader,
		ParamOverride: channel.ParamOverride,
		ChannelProxy:  channel.ChannelProxy,
	}
}

func TestChannelStream(ctx context.Context, channel *model.Channel, sendEvent func(StreamEvent)) error {
	return TestChannelConfigStream(ctx, ChannelToTestConfig(channel), sendEvent)
}

func TestChannelConfigStream(ctx context.Context, cfg ChannelTestConfig, sendEvent func(StreamEvent)) error {
	if sendEvent == nil {
		sendEvent = func(StreamEvent) {}
	}

	client, err := ChannelHttpClient(&model.Channel{Proxy: cfg.Proxy, ChannelProxy: cfg.ChannelProxy})
	if err != nil {
		return fmt.Errorf("创建 HTTP 客户端失败: %w", err)
	}

	key := selectChannelTestKey(cfg.Keys)
	if key == "" {
		return fmt.Errorf("没有可用的 API Key")
	}

	tasks, skipped, err := buildChannelTestTasks(ctx, cfg, key)
	if err != nil {
		return err
	}

	taskInfos := make([]ChannelTestTaskInfo, 0, len(tasks))
	for _, task := range tasks {
		taskInfos = append(taskInfos, ChannelTestTaskInfo{
			Model:        task.modelName,
			BaseUrl:      task.endpoint.BaseUrl,
			EndpointType: task.endpoint.Type,
		})
	}
	sendEvent(StreamEvent{
		Type:       StreamEventStart,
		Tasks:      taskInfos,
		Skipped:    skipped,
		SkipReason: "TTS 模型不支持聊天测试",
		Total:      len(tasks),
	})

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, channelTestConcurrency)
	successCount := 0
	failedCount := 0

	for _, task := range tasks {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(task channelTestTask) {
			defer wg.Done()
			defer func() { <-sem }()

			result := testSingleModel(ctx, client, task.endpoint, key, task.modelName, cfg.CustomHeader, cfg.ParamOverride)
			mu.Lock()
			if result.Success {
				successCount++
			} else {
				failedCount++
			}
			sendEvent(StreamEvent{Type: StreamEventResult, Result: &result})
			mu.Unlock()
		}(task)
	}

	wg.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	sendEvent(StreamEvent{Type: StreamEventDone, Total: len(tasks), Success: successCount, Failed: failedCount})
	return nil
}

func selectChannelTestKey(keys []model.ChannelKey) string {
	for _, key := range keys {
		if key.Enabled && strings.TrimSpace(key.ChannelKey) != "" {
			return strings.TrimSpace(key.ChannelKey)
		}
	}
	return ""
}

func buildChannelTestTasks(ctx context.Context, cfg ChannelTestConfig, key string) ([]channelTestTask, []string, error) {
	endpoints := enabledChannelTestEndpoints(cfg.Endpoints)
	if len(endpoints) == 0 {
		return nil, nil, fmt.Errorf("没有可用的 Endpoint")
	}

	models := uniqueStrings(xstrings.SplitTrimCompact(",", cfg.Model, cfg.CustomModel))
	if len(models) == 0 {
		var err error
		models, err = FetchModelsWithChannelProxy(ctx, endpoints, key, cfg.Proxy, cfg.ChannelProxy, cfg.CustomHeader)
		if err != nil {
			return nil, nil, fmt.Errorf("获取模型列表失败: %w", err)
		}
		models = uniqueStrings(models)
	}
	if len(models) == 0 {
		return nil, nil, fmt.Errorf("没有可测试模型")
	}

	// 分离 TTS 模型和聊天模型
	var chatModels, ttsModels []string
	for _, m := range models {
		if isTTSModel(m) {
			ttsModels = append(ttsModels, m)
		} else {
			chatModels = append(chatModels, m)
		}
	}

	if len(chatModels) == 0 {
		return nil, ttsModels, fmt.Errorf("没有可测试的聊天模型（TTS 模型已跳过）")
	}

	tasks := make([]channelTestTask, 0, len(endpoints)*len(chatModels))
	for _, ep := range endpoints {
		for _, modelName := range chatModels {
			tasks = append(tasks, channelTestTask{modelName: modelName, endpoint: ep})
		}
	}
	return tasks, ttsModels, nil
}

func enabledChannelTestEndpoints(endpoints []model.Endpoint) []model.Endpoint {
	result := make([]model.Endpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		if ep.Enabled && strings.TrimSpace(ep.BaseUrl) != "" {
			ep.BaseUrl = strings.TrimRight(strings.TrimSpace(ep.BaseUrl), "/")
			result = append(result, ep)
		}
	}
	return result
}

func isTTSModel(modelName string) bool {
	name := strings.ToLower(modelName)
	return strings.Contains(name, "-tts") || strings.Contains(name, "tts-")
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func testSingleModel(ctx context.Context, client *http.Client, ep model.Endpoint, key string, modelName string, customHeader []model.CustomHeader, paramOverride *string) ModelTestResult {
	result := ModelTestResult{Model: modelName, BaseUrl: ep.BaseUrl, EndpointType: ep.Type}

	if !outbound.IsChatChannelType(ep.Type) {
		result.Error = fmt.Sprintf("端点类型 %d 不支持聊天测试", ep.Type)
		return result
	}

	outAdapter := outbound.Get(ep.Type)
	if outAdapter == nil {
		result.Error = fmt.Sprintf("不支持的端点类型: %d", ep.Type)
		return result
	}

	stream := true
	maxTokens := int64(1000)
	internalReq := &tmodel.InternalLLMRequest{
		Model: modelName,
		Messages: []tmodel.Message{
			{Role: "user", Content: tmodel.MessageContent{Content: lo.ToPtr("Hi")}},
		},
		Stream:    &stream,
		MaxTokens: &maxTokens,
	}

	httpReq, err := outAdapter.TransformRequest(ctx, internalReq, ep.BaseUrl, key)
	if err != nil {
		result.Error = fmt.Sprintf("构造请求失败: %v", err)
		return result
	}

	for _, h := range customHeader {
		if h.HeaderKey != "" {
			httpReq.Header.Set(h.HeaderKey, h.HeaderValue)
		}
	}
	if err := applyParamOverride(httpReq, paramOverride); err != nil {
		result.Error = fmt.Sprintf("参数覆盖无效: %v", err)
		return result
	}

	start := time.Now()
	resp, err := client.Do(httpReq)
	if err != nil {
		result.Error = fmt.Sprintf("请求失败: %v", err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		result.ResponseTimeMs = time.Since(start).Milliseconds()
		return result
	}

	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.Contains(strings.ToLower(ct), "text/event-stream") {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		result.Error = fmt.Sprintf("上游返回非 SSE 响应 (%s): %s", ct, string(body))
		result.ResponseTimeMs = time.Since(start).Milliseconds()
		return result
	}

	hasContent := false
	firstTokenRecorded := false
	var contentParts []string

	readCfg := &sse.ReadConfig{MaxEventSize: 1024 * 1024}
	for ev, err := range sse.Read(resp.Body, readCfg) {
		if err != nil {
			if !hasContent {
				result.Error = fmt.Sprintf("流式读取失败: %v", err)
			}
			break
		}

		internalResp, err := outAdapter.TransformStream(ctx, []byte(ev.Data))
		if err != nil || internalResp == nil {
			continue
		}

		if internalResp.Object == "[DONE]" {
			break
		}

		for _, choice := range internalResp.Choices {
			if choice.Delta == nil {
				continue
			}
			if choice.Delta.Content.Content != nil && *choice.Delta.Content.Content != "" {
				if !firstTokenRecorded {
					result.FirstTokenTimeMs = time.Since(start).Milliseconds()
					firstTokenRecorded = true
				}
				hasContent = true
				contentParts = append(contentParts, *choice.Delta.Content.Content)
			}
			for _, part := range choice.Delta.Content.MultipleContent {
				if part.Text != nil && *part.Text != "" {
					if !firstTokenRecorded {
						result.FirstTokenTimeMs = time.Since(start).Milliseconds()
						firstTokenRecorded = true
					}
					hasContent = true
					contentParts = append(contentParts, *part.Text)
				}
			}
		}
	}

	result.ResponseTimeMs = time.Since(start).Milliseconds()
	result.Success = hasContent

	if !hasContent && result.Error == "" {
		if len(contentParts) == 0 {
			result.Error = "响应无内容"
		} else {
			result.Error = fmt.Sprintf("流式读取异常，已收集内容长度: %d", len(strings.Join(contentParts, "")))
		}
	}

	return result
}

func applyParamOverride(req *http.Request, paramOverride *string) error {
	if req == nil || paramOverride == nil || strings.TrimSpace(*paramOverride) == "" {
		return nil
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	merged, err := MergeJSONBody(body, *paramOverride)
	if err != nil {
		return err
	}
	req.Body = io.NopCloser(strings.NewReader(string(merged)))
	req.ContentLength = int64(len(merged))
	return nil
}

func MergeJSONBody(base []byte, override string) ([]byte, error) {
	if strings.TrimSpace(override) == "" {
		return base, nil
	}
	var baseMap map[string]any
	if err := json.Unmarshal(base, &baseMap); err != nil {
		return nil, err
	}
	var overrideMap map[string]any
	if err := json.Unmarshal([]byte(override), &overrideMap); err != nil {
		return nil, err
	}
	for key, value := range overrideMap {
		baseMap[key] = value
	}
	return json.Marshal(baseMap)
}
