package openai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gclm/octopus/internal/transformer/model"
)

func TestCompactResponseOutbound_TransformRequest_UsesCompactEndpoint(t *testing.T) {
	outbound := NewCompactResponseOutbound()
	request := &model.InternalLLMRequest{Model: "gpt-5"}

	httpRequest, err := outbound.TransformRequest(context.Background(), request, "https://example.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}
	if got, want := httpRequest.URL.Path, "/v1/responses/compact"; got != want {
		t.Fatalf("path = %s, want %s", got, want)
	}
}

func TestCompactResponseOutbound_TransformResponse_Passthrough(t *testing.T) {
	outbound := NewCompactResponseOutbound()
	raw := `{"object":"response.compaction","id":"rc_123"}`

	resp, err := outbound.TransformResponse(context.Background(), &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(raw)),
	})
	if err != nil {
		t.Fatalf("TransformResponse() error = %v", err)
	}
	if got, want := resp.Object, "response.compaction"; got != want {
		t.Fatalf("object = %s, want %s", got, want)
	}
	if got := string(resp.RawResponse); got != raw {
		t.Fatalf("RawResponse = %s, want %s", got, raw)
	}
}
