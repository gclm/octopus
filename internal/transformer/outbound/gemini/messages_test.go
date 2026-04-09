package gemini

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestMessagesOutbound_TransformResponse_EmptyCandidates(t *testing.T) {
	emptyBody := `{
		"candidates": [],
		"usageMetadata": {
			"promptTokenCount": 0,
			"candidatesTokenCount": 0,
			"totalTokenCount": 0
		}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(emptyBody)),
	}

	o := &MessagesOutbound{}
	result, err := o.TransformResponse(context.Background(), resp)

	if err == nil {
		t.Fatal("expected error for empty candidates, got nil")
	}
	if !strings.Contains(err.Error(), "no candidates") {
		t.Errorf("expected 'no candidates' error, got: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got: %v", result)
	}
}

func TestMessagesOutbound_TransformResponse_ValidResponse(t *testing.T) {
	validBody := `{
		"candidates": [{
			"content": {
				"parts": [{"text": "hello"}],
				"role": "model"
			},
			"finishReason": "STOP",
			"index": 0
		}],
		"usageMetadata": {
			"promptTokenCount": 10,
			"candidatesTokenCount": 5,
			"totalTokenCount": 15
		}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(validBody)),
	}

	o := &MessagesOutbound{}
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
