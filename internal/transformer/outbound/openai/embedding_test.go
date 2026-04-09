package openai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestEmbeddingOutbound_TransformResponse_EmptyData(t *testing.T) {
	emptyBody := `{
		"id": "emb-123",
		"object": "list",
		"created": 1772096386,
		"model": "text-embedding-ada-002",
		"data": [],
		"usage": {"prompt_tokens": 0, "total_tokens": 0}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(emptyBody)),
	}

	o := &EmbeddingOutbound{}
	result, err := o.TransformResponse(context.Background(), resp)

	if err == nil {
		t.Fatal("expected error for empty embedding data, got nil")
	}
	if !strings.Contains(err.Error(), "no embedding data") {
		t.Errorf("expected 'no embedding data' error, got: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got: %v", result)
	}
}

func TestEmbeddingOutbound_TransformResponse_ValidResponse(t *testing.T) {
	validBody := `{
		"id": "emb-123",
		"object": "list",
		"created": 1772096386,
		"model": "text-embedding-ada-002",
		"data": [{"object": "embedding", "index": 0, "embedding": [0.1, 0.2, 0.3]}],
		"usage": {"prompt_tokens": 5, "total_tokens": 5}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(validBody)),
	}

	o := &EmbeddingOutbound{}
	result, err := o.TransformResponse(context.Background(), resp)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.EmbeddingData) != 1 {
		t.Errorf("expected 1 embedding, got %d", len(result.EmbeddingData))
	}
}
