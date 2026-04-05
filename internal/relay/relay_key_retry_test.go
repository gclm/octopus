package relay

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gclm/octopus/internal/db"
	dbmodel "github.com/gclm/octopus/internal/model"
	"github.com/gclm/octopus/internal/op"
	"github.com/gclm/octopus/internal/transformer/inbound"
	"github.com/gclm/octopus/internal/transformer/outbound"
	"github.com/gin-gonic/gin"
)

func TestHandler_RetriesNextKeyWithinSameChannelOn429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initRelayTestDB(t)

	keyHits := map[string]int{}
	originalHTTPClient := channelHTTPClient
	channelHTTPClient = func(channel *dbmodel.Channel) (*http.Client, error) {
		return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			keyHits[r.Header.Get("Authorization")]++
			switch r.Header.Get("Authorization") {
			case "Bearer key-1":
				return newJSONResponse(http.StatusTooManyRequests, `{"error":{"message":"rate limited"}}`), nil
			case "Bearer key-2":
				return newJSONResponse(http.StatusOK, `{"id":"chatcmpl-1","object":"chat.completion","created":123,"model":"gpt-5","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`), nil
			default:
				return newJSONResponse(http.StatusUnauthorized, `{"error":{"message":"unexpected key"}}`), nil
			}
		})}, nil
	}
	t.Cleanup(func() {
		channelHTTPClient = originalHTTPClient
	})

	ctx := context.Background()
	channel := &dbmodel.Channel{
		Name:    "retry-channel",
		Type:    outbound.OutboundTypeOpenAIChat,
		Enabled: true,
		BaseUrls: []dbmodel.BaseUrl{{
			URL: "https://example.com/v1",
		}},
		Keys: []dbmodel.ChannelKey{
			{Enabled: true, ChannelKey: "key-1", Remark: "first"},
			{Enabled: true, ChannelKey: "key-2", Remark: "second"},
		},
	}
	if err := op.ChannelCreate(channel, ctx); err != nil {
		t.Fatalf("ChannelCreate() error = %v", err)
	}
	group := &dbmodel.Group{
		Name: "gpt-5",
		Mode: dbmodel.GroupModeScored,
		Items: []dbmodel.GroupItem{{
			ChannelID: channel.ID,
			ModelName: "gpt-5",
			Priority:  1,
		}},
	}
	if err := op.GroupCreate(group, ctx); err != nil {
		t.Fatalf("GroupCreate() error = %v", err)
	}

	body := []byte(`{"model":"gpt-5","messages":[{"role":"user","content":"hello"}]}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	Handler(inbound.InboundTypeOpenAIChat, c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); !bytes.Contains([]byte(got), []byte(`"content":"ok"`)) {
		t.Fatalf("unexpected response body: %s", got)
	}
	if keyHits["Bearer key-1"] != 1 || keyHits["Bearer key-2"] != 1 {
		t.Fatalf("expected one request per key, got key1=%d key2=%d", keyHits["Bearer key-1"], keyHits["Bearer key-2"])
	}

	storedChannel, err := op.ChannelGet(channel.ID, ctx)
	if err != nil {
		t.Fatalf("ChannelGet() error = %v", err)
	}
	firstKey := findChannelKeyByRemark(t, storedChannel.Keys, "first")
	secondKey := findChannelKeyByRemark(t, storedChannel.Keys, "second")
	if firstKey.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("first key status = %d, want %d", firstKey.StatusCode, http.StatusTooManyRequests)
	}
	if secondKey.StatusCode != http.StatusOK {
		t.Fatalf("second key status = %d, want %d", secondKey.StatusCode, http.StatusOK)
	}
	if firstKey.LastUseTimeStamp == 0 || secondKey.LastUseTimeStamp == 0 {
		t.Fatalf("expected key timestamps to be updated, got first=%d second=%d", firstKey.LastUseTimeStamp, secondKey.LastUseTimeStamp)
	}
}

func TestHandler_PreservesOpenAIInboundMetadataAcrossFallbacks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initRelayTestDB(t)

	requestBodies := map[string]string{}
	originalHTTPClient := channelHTTPClient
	channelHTTPClient = func(channel *dbmodel.Channel) (*http.Client, error) {
		return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return nil, err
			}
			requestBodies[channel.Name] = string(body)

			switch channel.Type {
			case outbound.OutboundTypeOpenAIChat:
				return newJSONResponse(http.StatusServiceUnavailable, `{"error":{"message":"temporary upstream failure"}}`), nil
			case outbound.OutboundTypeAnthropic:
				return newJSONResponse(http.StatusOK, `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"model":"claude-3-5-sonnet","stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`), nil
			default:
				return newJSONResponse(http.StatusBadGateway, `{"error":{"message":"unexpected channel type"}}`), nil
			}
		})}, nil
	}
	t.Cleanup(func() {
		channelHTTPClient = originalHTTPClient
	})

	ctx := context.Background()
	openAIChannel := &dbmodel.Channel{
		Name:    "openai-first",
		Type:    outbound.OutboundTypeOpenAIChat,
		Enabled: true,
		BaseUrls: []dbmodel.BaseUrl{{
			URL: "https://example.com/v1",
		}},
		Keys: []dbmodel.ChannelKey{{Enabled: true, ChannelKey: "sk-openai"}},
	}
	if err := op.ChannelCreate(openAIChannel, ctx); err != nil {
		t.Fatalf("ChannelCreate(openai) error = %v", err)
	}

	anthropicChannel := &dbmodel.Channel{
		Name:    "anthropic-fallback",
		Type:    outbound.OutboundTypeAnthropic,
		Enabled: true,
		BaseUrls: []dbmodel.BaseUrl{{
			URL: "https://example.com/v1",
		}},
		Keys: []dbmodel.ChannelKey{{Enabled: true, ChannelKey: "sk-anthropic"}},
	}
	if err := op.ChannelCreate(anthropicChannel, ctx); err != nil {
		t.Fatalf("ChannelCreate(anthropic) error = %v", err)
	}

	group := &dbmodel.Group{
		Name: "gpt-5",
		Mode: dbmodel.GroupModeScored,
		Items: []dbmodel.GroupItem{
			{
				ChannelID: openAIChannel.ID,
				ModelName: "gpt-5",
				Priority:  1,
			},
			{
				ChannelID: anthropicChannel.ID,
				ModelName: "claude-3-5-sonnet",
				Priority:  2,
			},
		},
	}
	if err := op.GroupCreate(group, ctx); err != nil {
		t.Fatalf("GroupCreate() error = %v", err)
	}

	body := []byte(`{"model":"gpt-5","messages":[{"role":"user","content":"hello"}],"metadata":{"user_id":"user-123"}}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	Handler(inbound.InboundTypeOpenAIChat, c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); !bytes.Contains([]byte(got), []byte(`"content":"ok"`)) {
		t.Fatalf("unexpected response body: %s", got)
	}
	if !strings.Contains(requestBodies["openai-first"], `"metadata":{"user_id":"user-123"}`) {
		t.Fatalf("expected OpenAI-compatible request to retain metadata, got %s", requestBodies["openai-first"])
	}
	if !strings.Contains(requestBodies["anthropic-fallback"], `"metadata":{"user_id":"user-123"}`) {
		t.Fatalf("expected anthropic fallback request to retain metadata, got %s", requestBodies["anthropic-fallback"])
	}
}

func TestHandler_StripsAnthropicMetadataWhenForwardingToNonAnthropicChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initRelayTestDB(t)

	requestBodies := map[string]string{}
	originalHTTPClient := channelHTTPClient
	channelHTTPClient = func(channel *dbmodel.Channel) (*http.Client, error) {
		return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return nil, err
			}
			requestBodies[channel.Name] = string(body)
			return newJSONResponse(http.StatusOK, `{"id":"chatcmpl-1","object":"chat.completion","created":123,"model":"gpt-4o-mini","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`), nil
		})}, nil
	}
	t.Cleanup(func() {
		channelHTTPClient = originalHTTPClient
	})

	ctx := context.Background()
	channel := &dbmodel.Channel{
		Name:    "openai-target",
		Type:    outbound.OutboundTypeOpenAIChat,
		Enabled: true,
		BaseUrls: []dbmodel.BaseUrl{{
			URL: "https://example.com/v1",
		}},
		Keys: []dbmodel.ChannelKey{{Enabled: true, ChannelKey: "sk-openai"}},
	}
	if err := op.ChannelCreate(channel, ctx); err != nil {
		t.Fatalf("ChannelCreate() error = %v", err)
	}

	group := &dbmodel.Group{
		Name: "claude-3-5-sonnet",
		Mode: dbmodel.GroupModeScored,
		Items: []dbmodel.GroupItem{{
			ChannelID: channel.ID,
			ModelName: "gpt-4o-mini",
			Priority:  1,
		}},
	}
	if err := op.GroupCreate(group, ctx); err != nil {
		t.Fatalf("GroupCreate() error = %v", err)
	}

	body := []byte(`{"model":"claude-3-5-sonnet","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"metadata":{"user_id":"user-123"}}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	Handler(inbound.InboundTypeAnthropic, c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if strings.Contains(requestBodies["openai-target"], `"metadata"`) {
		t.Fatalf("expected Anthropic-only metadata to be stripped on non-Anthropic outbound, got %s", requestBodies["openai-target"])
	}
}

func initRelayTestDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "relay-test.db")
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := op.InitCache(); err != nil {
		t.Fatalf("InitCache() error = %v", err)
	}
}

func findChannelKeyByRemark(t *testing.T, keys []dbmodel.ChannelKey, remark string) dbmodel.ChannelKey {
	t.Helper()
	for _, key := range keys {
		if key.Remark == remark {
			return key
		}
	}
	t.Fatalf("channel key with remark %q not found", remark)
	return dbmodel.ChannelKey{}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func newJSONResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}
