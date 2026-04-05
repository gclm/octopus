package openai

import (
	"testing"

	"github.com/gclm/octopus/internal/transformer/model"
)

func TestConvertToResponsesRequest_PreservesExtendedOptions(t *testing.T) {
	reasoningBudget := int64(2048)
	topLogprobs := int64(3)
	req := &model.InternalLLMRequest{
		Model:           "gpt-5",
		Include:         []string{"reasoning.encrypted_content"},
		TopLogprobs:     &topLogprobs,
		ReasoningEffort: "medium",
		ReasoningBudget: &reasoningBudget,
	}

	got := ConvertToResponsesRequest(req)
	if got == nil {
		t.Fatal("expected request to be converted")
	}
	if len(got.Include) != 1 || got.Include[0] != "reasoning.encrypted_content" {
		t.Fatalf("Include = %#v", got.Include)
	}
	if got.TopLogprobs == nil || *got.TopLogprobs != topLogprobs {
		t.Fatalf("TopLogprobs = %#v", got.TopLogprobs)
	}
	if got.Reasoning == nil {
		t.Fatal("expected reasoning to be present")
	}
	if got.Reasoning.Effort != "medium" {
		t.Fatalf("Reasoning.Effort = %q", got.Reasoning.Effort)
	}
	if got.Reasoning.MaxTokens == nil || *got.Reasoning.MaxTokens != reasoningBudget {
		t.Fatalf("Reasoning.MaxTokens = %#v", got.Reasoning.MaxTokens)
	}
}
