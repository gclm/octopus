package openai

import (
	"context"
	"strings"
	"testing"

	tmodel "github.com/gclm/octopus/internal/transformer/model"
	"github.com/samber/lo"
)

func TestChatInbound_TransformRequest(t *testing.T) {
	body := `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"stream":true}`
	in := &ChatInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if req.Model != "gpt-4" {
		t.Errorf("Model = %q", req.Model)
	}
	if req.Stream == nil || !*req.Stream {
		t.Error("Stream 应为 true")
	}
}

func TestChatInbound_TransformRequest_无效JSON(t *testing.T) {
	in := &ChatInbound{}
	_, err := in.TransformRequest(context.Background(), []byte(`{bad`))
	if err == nil {
		t.Error("无效 JSON 应报错")
	}
}

func TestChatInbound_TransformResponse(t *testing.T) {
	in := &ChatInbound{}
	resp := &tmodel.InternalLLMResponse{
		ID:     "chatcmpl-1",
		Object: "chat.completion",
		Model:  "gpt-4",
		Choices: []tmodel.Choice{{
			Index:        0,
			FinishReason: lo.ToPtr("stop"),
			Message: &tmodel.Message{
				Role:    "assistant",
				Content: tmodel.MessageContent{Content: lo.ToPtr("Hello")},
			},
		}},
	}
	data, err := in.TransformResponse(context.Background(), resp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Hello") {
		t.Errorf("响应应包含 Hello，实际: %s", data)
	}
	// 应存储
	got, _ := in.GetInternalResponse(context.Background())
	if got == nil || got.ID != "chatcmpl-1" {
		t.Error("GetInternalResponse 应返回存储的响应")
	}
}

func TestChatInbound_TransformStream_DONE(t *testing.T) {
	in := &ChatInbound{}
	done := &tmodel.InternalLLMResponse{Object: "[DONE]"}
	data, err := in.TransformStream(context.Background(), done)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "[DONE]") {
		t.Errorf("应包含 [DONE]，实际: %s", data)
	}
}

func TestChatInbound_TransformStream_普通chunk(t *testing.T) {
	in := &ChatInbound{}
	chunk := &tmodel.InternalLLMResponse{
		ID:     "id1",
		Object: "chat.completion.chunk",
		Model:  "gpt-4",
		Choices: []tmodel.Choice{{
			Index: 0,
			Delta: &tmodel.Message{
				Role:    "assistant",
				Content: tmodel.MessageContent{Content: lo.ToPtr("Hello")},
			},
		}},
	}
	data, err := in.TransformStream(context.Background(), chunk)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "data: ") {
		t.Errorf("应以 data: 开头，实际: %s", data)
	}
}

func TestChatInbound_TransformStream_空Choices(t *testing.T) {
	in := &ChatInbound{}
	chunk := &tmodel.InternalLLMResponse{
		ID:      "id1",
		Object:  "chat.completion.chunk",
		Model:   "gpt-4",
		Choices: []tmodel.Choice{},
	}
	data, err := in.TransformStream(context.Background(), chunk)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"choices":[]`) {
		t.Errorf("空 choices 应序列化为空数组，实际: %s", data)
	}
}

func TestChatInbound_GetInternalResponse_流式聚合(t *testing.T) {
	in := &ChatInbound{}
	ctx := context.Background()

	chunk1 := &tmodel.InternalLLMResponse{
		ID: "id1", Object: "chat.completion.chunk", Model: "gpt-4",
		Choices: []tmodel.Choice{{Index: 0, Delta: &tmodel.Message{
			Role: "assistant", Content: tmodel.MessageContent{Content: lo.ToPtr("Hello")},
		}}},
	}
	chunk2 := &tmodel.InternalLLMResponse{
		ID: "id1", Object: "chat.completion.chunk", Model: "gpt-4",
		Choices: []tmodel.Choice{{Index: 0, Delta: &tmodel.Message{
			Content: tmodel.MessageContent{Content: lo.ToPtr(" world")},
		}}},
	}
	in.TransformStream(ctx, chunk1)
	in.TransformStream(ctx, chunk2)

	got, err := in.GetInternalResponse(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("应返回聚合响应")
	}
	content := *got.Choices[0].Message.Content.Content
	if content != "Hello world" {
		t.Errorf("聚合内容 = %q, want Hello world", content)
	}
}

func TestChatInbound_GetInternalResponse_空(t *testing.T) {
	in := &ChatInbound{}
	got, _ := in.GetInternalResponse(context.Background())
	if got != nil {
		t.Error("无数据时应返回 nil")
	}
}

// --- mergeToolCall ---

func TestMergeToolCall(t *testing.T) {
	t.Run("新增", func(t *testing.T) {
		result := mergeToolCall(nil, tmodel.ToolCall{
			Index: 0, ID: "tc_1", Type: "function",
			Function: tmodel.FunctionCall{Name: "fn", Arguments: `{"a":`},
		})
		if len(result) != 1 {
			t.Fatalf("长度 = %d", len(result))
		}
		if result[0].ID != "tc_1" {
			t.Errorf("ID = %q", result[0].ID)
		}
	})
	t.Run("合并", func(t *testing.T) {
		existing := []tmodel.ToolCall{
			{Index: 0, ID: "tc_1", Function: tmodel.FunctionCall{Name: "fn", Arguments: `{"a":`}},
		}
		result := mergeToolCall(existing, tmodel.ToolCall{
			Index: 0, Function: tmodel.FunctionCall{Arguments: `1}`},
		})
		if len(result) != 1 {
			t.Fatalf("长度 = %d", len(result))
		}
		if result[0].Function.Arguments != `{"a":1}` {
			t.Errorf("Arguments = %q", result[0].Function.Arguments)
		}
	})
	t.Run("多个不同index", func(t *testing.T) {
		existing := []tmodel.ToolCall{
			{Index: 0, ID: "tc_1", Function: tmodel.FunctionCall{Name: "fn1"}},
		}
		result := mergeToolCall(existing, tmodel.ToolCall{
			Index: 1, ID: "tc_2", Function: tmodel.FunctionCall{Name: "fn2"},
		})
		if len(result) != 2 {
			t.Fatalf("长度 = %d", len(result))
		}
	})
}
