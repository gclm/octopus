package authropic

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestMessageOutbound_TransformResponse_EmptyContent(t *testing.T) {
	emptyBody := `{
		"id": "msg_123",
		"type": "message",
		"role": "assistant",
		"content": [],
		"model": "claude-3-opus-20240229",
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 10, "output_tokens": 0}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(emptyBody)),
	}

	o := &MessageOutbound{}
	result, err := o.TransformResponse(context.Background(), resp)

	if err == nil {
		t.Fatal("expected error for empty content, got nil")
	}
	if !strings.Contains(err.Error(), "no content blocks") {
		t.Errorf("expected 'no content blocks' error, got: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got: %v", result)
	}
}

func TestMessageOutbound_TransformResponse_ValidResponse(t *testing.T) {
	validBody := `{
		"id": "msg_123",
		"type": "message",
		"role": "assistant",
		"content": [{"type": "text", "text": "hello"}],
		"model": "claude-3-opus-20240229",
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 10, "output_tokens": 5}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(validBody)),
	}

	o := &MessageOutbound{}
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
