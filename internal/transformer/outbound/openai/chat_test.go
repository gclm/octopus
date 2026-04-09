package openai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestChatOutbound_TransformResponse_EmptyChoices(t *testing.T) {
	// 模拟上游中转返回的空壳响应（HTTP 200，有 JSON 包体，但无 choices）
	emptyBody := `{
		"id": "chatcmpl-20260226085904630433901GfXOX42G",
		"object": "chat.completion",
		"created": 1772096386,
		"model": "gemini-3-flash-preview",
		"usage": {
			"prompt_tokens": 0,
			"completion_tokens": 0,
			"total_tokens": 0
		}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(emptyBody)),
	}

	o := &ChatOutbound{}
	result, err := o.TransformResponse(context.Background(), resp)

	if err == nil {
		t.Fatal("expected error for empty choices, got nil")
	}
	if !strings.Contains(err.Error(), "no choices") {
		t.Errorf("expected 'no choices' error, got: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got: %v", result)
	}
}

func TestChatOutbound_TransformResponse_ValidResponse(t *testing.T) {
	validBody := `{
		"id": "chatcmpl-123",
		"object": "chat.completion",
		"created": 1772096386,
		"model": "gpt-4",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": "hello"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(validBody)),
	}

	o := &ChatOutbound{}
	result, err := o.TransformResponse(context.Background(), resp)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Choices) != 1 {
		t.Errorf("expected 1 choice, got %d", len(result.Choices))
	}
}

func TestChatOutbound_TransformResponse_EmptyBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader("")),
	}

	o := &ChatOutbound{}
	_, err := o.TransformResponse(context.Background(), resp)

	if err == nil {
		t.Fatal("expected error for empty body, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected empty body error, got: %v", err)
	}
}
