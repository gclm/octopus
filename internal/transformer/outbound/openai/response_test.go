package openai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestResponseOutbound_TransformResponse_EmptyOutput(t *testing.T) {
	emptyBody := `{
		"id": "resp-123",
		"object": "response",
		"model": "gpt-4",
		"created_at": 1772096386,
		"output": [],
		"status": "completed"
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(emptyBody)),
	}

	o := &ResponseOutbound{}
	result, err := o.TransformResponse(context.Background(), resp)

	if err == nil {
		t.Fatal("expected error for empty output, got nil")
	}
	if !strings.Contains(err.Error(), "no output") {
		t.Errorf("expected 'no output' error, got: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got: %v", result)
	}
}

func TestResponseOutbound_TransformResponse_ValidResponse(t *testing.T) {
	validBody := `{
		"id": "resp-123",
		"object": "response",
		"model": "gpt-4",
		"created_at": 1772096386,
		"output": [{"type": "message", "role": "assistant", "content": [{"type": "output_text", "text": "hello"}]}],
		"status": "completed",
		"usage": {"input_tokens": 10, "output_tokens": 5, "total_tokens": 15}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(validBody)),
	}

	o := &ResponseOutbound{}
	result, err := o.TransformResponse(context.Background(), resp)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}
