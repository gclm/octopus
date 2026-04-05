package model

import "testing"

func TestInternalLLMResponseIsEmpty_TreatsRefusalAsNonEmpty(t *testing.T) {
	finishReason := "content_filter"
	resp := &InternalLLMResponse{
		Choices: []Choice{{
			Message:      &Message{Refusal: "blocked by safety policy"},
			FinishReason: &finishReason,
		}},
	}

	if resp.IsEmpty() {
		t.Fatal("expected refusal response to be treated as non-empty")
	}
}

func TestInternalLLMResponseIsEmpty_TreatsContentFilterAsNonEmpty(t *testing.T) {
	finishReason := "content_filter"
	resp := &InternalLLMResponse{
		Choices: []Choice{{
			Message:      &Message{},
			FinishReason: &finishReason,
		}},
	}

	if resp.IsEmpty() {
		t.Fatal("expected content_filter finish reason to be treated as non-empty")
	}
}

func TestInternalLLMResponseIsEmpty_DetectsTrulyEmptyChoice(t *testing.T) {
	resp := &InternalLLMResponse{
		Choices: []Choice{{
			Message: &Message{},
		}},
	}

	if !resp.IsEmpty() {
		t.Fatal("expected empty response to remain empty")
	}
}
