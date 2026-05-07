package helper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gclm/octopus/internal/model"
	"github.com/gclm/octopus/internal/transformer/outbound"
)

func TestChannelTestStreamUsesConfiguredModelsBeforeFetching(t *testing.T) {
	var modelsFetchCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			atomic.AddInt32(&modelsFetchCount, 1)
			http.Error(w, "不应该请求模型列表", http.StatusInternalServerError)
		case "/chat/completions":
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"id\":\"chatcmpl-test\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"}}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		default:
			t.Fatalf("收到未预期请求路径: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	channel := &model.Channel{
		Endpoints:   []model.Endpoint{{Type: outbound.OutboundTypeOpenAIChat, BaseUrl: server.URL, Enabled: true}},
		Keys:        []model.ChannelKey{{Enabled: true, ChannelKey: "sk-test"}},
		Model:       "gpt-a,gpt-b",
		CustomModel: "gpt-c",
	}

	events := collectChannelTestEvents(t, channel)

	if got := atomic.LoadInt32(&modelsFetchCount); got != 0 {
		t.Fatalf("配置了渠道模型时不应回退请求 /models，实际请求次数: %d", got)
	}

	start := firstEventOfType(t, events, StreamEventStart)
	if start.Total != 3 {
		t.Fatalf("start 事件总数 = %d，期望 3", start.Total)
	}

	results := resultEvents(events)
	if len(results) != 3 {
		t.Fatalf("结果数量 = %d，期望 3", len(results))
	}
	seen := map[string]bool{}
	for _, result := range results {
		if !result.Success {
			t.Fatalf("模型 %s 测试失败: %s", result.Model, result.Error)
		}
		if result.BaseUrl != server.URL {
			t.Fatalf("结果 base_url = %q，期望 %q", result.BaseUrl, server.URL)
		}
		if result.EndpointType != outbound.OutboundTypeOpenAIChat {
			t.Fatalf("结果 endpoint_type = %d，期望 %d", result.EndpointType, outbound.OutboundTypeOpenAIChat)
		}
		seen[result.Model] = true
	}
	for _, name := range []string{"gpt-a", "gpt-b", "gpt-c"} {
		if !seen[name] {
			t.Fatalf("缺少模型测试结果: %s", name)
		}
	}
}

func TestChannelTestStreamReturnsErrorWhenNoModelCanBeResolved(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(model.OpenAIModelList{})
	}))
	defer server.Close()

	channel := &model.Channel{
		Endpoints: []model.Endpoint{{Type: outbound.OutboundTypeOpenAIChat, BaseUrl: server.URL, Enabled: true}},
		Keys:      []model.ChannelKey{{Enabled: true, ChannelKey: "sk-test"}},
	}

	err := TestChannelStream(context.Background(), channel, func(StreamEvent) {})
	if err == nil || !strings.Contains(err.Error(), "没有可测试模型") {
		t.Fatalf("期望返回没有可测试模型错误，实际: %v", err)
	}
}

func TestChannelTestStreamReportsUnsupportedEndpointType(t *testing.T) {
	channel := &model.Channel{
		Endpoints: []model.Endpoint{{Type: outbound.OutboundTypeOpenAIEmbedding, BaseUrl: "http://example.test", Enabled: true}},
		Keys:      []model.ChannelKey{{Enabled: true, ChannelKey: "sk-test"}},
		Model:     "text-embedding-3-small",
	}

	events := collectChannelTestEvents(t, channel)
	results := resultEvents(events)
	if len(results) != 1 {
		t.Fatalf("结果数量 = %d，期望 1", len(results))
	}
	if results[0].Success {
		t.Fatal("embedding 端点不应被当作聊天测试成功")
	}
	if !strings.Contains(results[0].Error, "不支持聊天测试") {
		t.Fatalf("错误信息 = %q，期望包含不支持聊天测试", results[0].Error)
	}

	done := firstEventOfType(t, events, StreamEventDone)
	if done.Success != 0 || done.Failed != 1 {
		t.Fatalf("done 汇总 success=%d failed=%d，期望 success=0 failed=1", done.Success, done.Failed)
	}
}

func collectChannelTestEvents(t *testing.T, channel *model.Channel) []StreamEvent {
	t.Helper()
	var events []StreamEvent
	err := TestChannelStream(context.Background(), channel, func(event StreamEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("TestChannelStream 返回错误: %v", err)
	}
	return events
}

func firstEventOfType(t *testing.T, events []StreamEvent, eventType StreamEventType) StreamEvent {
	t.Helper()
	for _, event := range events {
		if event.Type == eventType {
			return event
		}
	}
	t.Fatalf("缺少事件类型: %s", eventType)
	return StreamEvent{}
}

func resultEvents(events []StreamEvent) []ModelTestResult {
	var results []ModelTestResult
	for _, event := range events {
		if event.Type == StreamEventResult && event.Result != nil {
			results = append(results, *event.Result)
		}
	}
	return results
}
