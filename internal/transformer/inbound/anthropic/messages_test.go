package anthropic

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	tmodel "github.com/gclm/octopus/internal/transformer/model"
	"github.com/samber/lo"
)

// --- TransformRequest ---

func TestTransformRequest_基本消息(t *testing.T) {
	body := `{
		"model": "claude-sonnet-4-20250514",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": "Hello"}]
	}`
	in := &MessagesInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatalf("TransformRequest 失败: %v", err)
	}
	if req.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q", req.Model)
	}
	if req.MaxTokens == nil || *req.MaxTokens != 1024 {
		t.Errorf("MaxTokens = %v", req.MaxTokens)
	}
	if len(req.Messages) != 1 {
		t.Fatalf("Messages 长度 = %d", len(req.Messages))
	}
	if req.Messages[0].Content.Content == nil || *req.Messages[0].Content.Content != "Hello" {
		t.Errorf("Content = %v", req.Messages[0].Content.Content)
	}
	if req.RawAPIFormat != tmodel.APIFormatAnthropicMessage {
		t.Errorf("RawAPIFormat = %q", req.RawAPIFormat)
	}
}

func TestTransformRequest_MaxTokens默认值(t *testing.T) {
	body := `{"model":"m","max_tokens":0,"messages":[{"role":"user","content":"hi"}]}`
	in := &MessagesInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if req.MaxTokens == nil || *req.MaxTokens != 1 {
		t.Errorf("max_tokens < 1 时应默认为 1，实际 %v", req.MaxTokens)
	}
}

func TestTransformRequest_SystemPrompt字符串(t *testing.T) {
	body := `{"model":"m","max_tokens":100,"system":"You are helpful","messages":[{"role":"user","content":"hi"}]}`
	in := &MessagesInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatalf("TransformRequest: %v", err)
	}
	if len(req.Messages) != 2 || req.Messages[0].Role != "system" {
		t.Fatalf("应包含 system 消息，实际 messages 数量=%d", len(req.Messages))
	}
	if req.Messages[0].Content.Content == nil || *req.Messages[0].Content.Content != "You are helpful" {
		t.Errorf("system content 错误")
	}
}

func TestTransformRequest_SystemPrompt数组(t *testing.T) {
	body := `{"model":"m","max_tokens":100,"system":[{"type":"text","text":"Rule 1"},{"type":"text","text":"Rule 2","cache_control":{"type":"ephemeral"}}],"messages":[{"role":"user","content":"hi"}]}`
	in := &MessagesInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	systemCount := 0
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemCount++
		}
	}
	if systemCount != 2 {
		t.Errorf("system 消息数量 = %d, want 2", systemCount)
	}
	if req.TransformerMetadata["anthropic_system_array_format"] != "true" {
		t.Error("应标记 anthropic_system_array_format")
	}
}

func TestTransformRequest_Thinking配置(t *testing.T) {
	tests := []struct {
		name       string
		json       string
		wantEffort string
		wantBudget bool
	}{
		{"enabled low", `{"type":"enabled","budget_tokens":3000}`, "low", true},
		{"enabled medium", `{"type":"enabled","budget_tokens":10000}`, "medium", true},
		{"enabled high", `{"type":"enabled","budget_tokens":50000}`, "high", true},
		{"disabled", `{"type":"disabled"}`, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"model":"m","max_tokens":100,"thinking":` + tt.json + `,"messages":[{"role":"user","content":"hi"}]}`
			in := &MessagesInbound{}
			req, err := in.TransformRequest(context.Background(), []byte(body))
			if err != nil {
				t.Fatal(err)
			}
			if req.ReasoningEffort != tt.wantEffort {
				t.Errorf("ReasoningEffort = %q, want %q", req.ReasoningEffort, tt.wantEffort)
			}
			if tt.wantBudget != (req.ReasoningBudget != nil) {
				t.Errorf("ReasoningBudget 存在 = %v, want %v", req.ReasoningBudget != nil, tt.wantBudget)
			}
		})
	}
}

func TestTransformRequest_AdaptiveThinking(t *testing.T) {
	body := `{"model":"m","max_tokens":100,"thinking":{"type":"adaptive"},"output_config":{"effort":"medium"},"messages":[{"role":"user","content":"hi"}]}`
	in := &MessagesInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if req.ReasoningEffort != "medium" {
		t.Errorf("ReasoningEffort = %q, want medium", req.ReasoningEffort)
	}
	if !req.AdaptiveThinking {
		t.Error("AdaptiveThinking 应为 true")
	}
}

func TestTransformRequest_工具调用(t *testing.T) {
	body := `{"model":"m","max_tokens":100,"tools":[{"name":"get_weather","description":"Get weather","input_schema":{"type":"object"}}],"messages":[{"role":"user","content":"weather?"}]}`
	in := &MessagesInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Tools) != 1 || req.Tools[0].Function.Name != "get_weather" {
		t.Errorf("Tools = %v", req.Tools)
	}
}

func TestTransformRequest_ToolResult消息(t *testing.T) {
	body := `{"model":"m","max_tokens":100,"messages":[
		{"role":"user","content":"weather?"},
		{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"get_weather","input":{}}]},
		{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_1","content":"Sunny"}]}
	]}`
	in := &MessagesInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	hasTool := false
	for _, m := range req.Messages {
		if m.Role == "tool" {
			hasTool = true
			if m.ToolCallID == nil || *m.ToolCallID != "tu_1" {
				t.Errorf("tool ToolCallID = %v", m.ToolCallID)
			}
		}
	}
	if !hasTool {
		t.Error("缺少 tool 角色")
	}
}

func TestTransformRequest_StopSequences(t *testing.T) {
	t.Run("单个", func(t *testing.T) {
		body := `{"model":"m","max_tokens":100,"stop_sequences":["END"],"messages":[{"role":"user","content":"hi"}]}`
		in := &MessagesInbound{}
		req, err := in.TransformRequest(context.Background(), []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		if req.Stop == nil || req.Stop.Stop == nil || *req.Stop.Stop != "END" {
			t.Errorf("Stop = %v", req.Stop)
		}
	})
	t.Run("多个", func(t *testing.T) {
		body := `{"model":"m","max_tokens":100,"stop_sequences":["A","B"],"messages":[{"role":"user","content":"hi"}]}`
		in := &MessagesInbound{}
		req, err := in.TransformRequest(context.Background(), []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		if req.Stop == nil || len(req.Stop.MultipleStop) != 2 {
			t.Errorf("Stop = %v", req.Stop)
		}
	})
}

func TestTransformRequest_图片内容(t *testing.T) {
	body := `{"model":"m","max_tokens":100,"messages":[{"role":"user","content":[
		{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBOR"}},
		{"type":"text","text":"describe"}
	]}]}`
	in := &MessagesInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	mc := req.Messages[0].Content.MultipleContent
	if len(mc) != 2 {
		t.Fatalf("MultipleContent 长度 = %d", len(mc))
	}
	if mc[0].Type != "image_url" {
		t.Errorf("第一个 part 类型 = %q", mc[0].Type)
	}
	if !strings.HasPrefix(mc[0].ImageURL.URL, "data:image/png;base64,") {
		t.Errorf("图片 URL 格式错误: %q", mc[0].ImageURL.URL)
	}
}

func TestTransformRequest_ThinkingBlock保留(t *testing.T) {
	body := `{"model":"m","max_tokens":100,"messages":[{"role":"assistant","content":[
		{"type":"thinking","thinking":"Let me think","signature":"sig123"},
		{"type":"text","text":"Answer is 42"}
	]}]}`
	in := &MessagesInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	msg := req.Messages[0]
	if msg.ReasoningContent == nil || *msg.ReasoningContent != "Let me think" {
		t.Errorf("ReasoningContent = %v", msg.ReasoningContent)
	}
	if msg.ReasoningSignature == nil || *msg.ReasoningSignature != "sig123" {
		t.Errorf("ReasoningSignature = %v", msg.ReasoningSignature)
	}
}

func TestTransformRequest_Metadata(t *testing.T) {
	body := `{"model":"m","max_tokens":100,"metadata":{"user_id":"u_123"},"messages":[{"role":"user","content":"hi"}]}`
	in := &MessagesInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if req.Metadata["user_id"] != "u_123" {
		t.Errorf("user_id metadata = %q", req.Metadata["user_id"])
	}
}

func TestTransformRequest_无效JSON(t *testing.T) {
	in := &MessagesInbound{}
	_, err := in.TransformRequest(context.Background(), []byte(`{invalid`))
	if err == nil {
		t.Error("无效 JSON 应返回错误")
	}
}

// --- TransformResponse ---

func makeResponse(finishReason, text string, reasoning, reasoningSig *string, toolCalls []tmodel.ToolCall) *tmodel.InternalLLMResponse {
	return &tmodel.InternalLLMResponse{
		ID:     "msg_1",
		Object: "chat.completion",
		Model:  "claude-3",
		Choices: []tmodel.Choice{{
			Index:        0,
			FinishReason: lo.ToPtr(finishReason),
			Message: &tmodel.Message{
				Role: "assistant",
				Content: tmodel.MessageContent{
					Content: lo.ToPtr(text),
				},
				ReasoningContent:  reasoning,
				ReasoningSignature: reasoningSig,
				ToolCalls:         toolCalls,
			},
		}},
	}
}

func TestTransformResponse_基本响应(t *testing.T) {
	in := &MessagesInbound{}
	data, err := in.TransformResponse(context.Background(), makeResponse("stop", "Hello!", nil, nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	var result Message
	json.Unmarshal(data, &result)
	if result.StopReason == nil || *result.StopReason != "end_turn" {
		t.Errorf("StopReason = %v", result.StopReason)
	}
	if len(result.Content) != 1 || result.Content[0].Type != "text" {
		t.Errorf("Content = %v", result.Content)
	}
}

func TestTransformResponse_FinishReason映射(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"stop", "end_turn"},
		{"length", "max_tokens"},
		{"tool_calls", "tool_use"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			in := &MessagesInbound{}
			data, err := in.TransformResponse(context.Background(), makeResponse(tt.input, "x", nil, nil, nil))
			if err != nil {
				t.Fatal(err)
			}
			var result Message
			json.Unmarshal(data, &result)
			if *result.StopReason != tt.want {
				t.Errorf("StopReason = %q, want %q", *result.StopReason, tt.want)
			}
		})
	}
}

func TestTransformResponse_Thinking内容(t *testing.T) {
	in := &MessagesInbound{}
	thinking := "I need to reason..."
	sig := "sig_abc"
	data, err := in.TransformResponse(context.Background(), makeResponse("stop", "Answer", &thinking, &sig, nil))
	if err != nil {
		t.Fatal(err)
	}
	var result Message
	json.Unmarshal(data, &result)
	if len(result.Content) < 2 {
		t.Fatalf("应包含 thinking + text，实际 %d 个 block", len(result.Content))
	}
	if result.Content[0].Type != "thinking" {
		t.Errorf("第一个 block 类型 = %q", result.Content[0].Type)
	}
	if result.Content[0].Thinking == nil || *result.Content[0].Thinking != "I need to reason..." {
		t.Errorf("Thinking = %v", result.Content[0].Thinking)
	}
	if result.Content[0].Signature == nil || *result.Content[0].Signature != "sig_abc" {
		t.Errorf("Signature = %v", result.Content[0].Signature)
	}
}

func TestTransformResponse_默认Signature(t *testing.T) {
	in := &MessagesInbound{}
	thinking := "hmm"
	data, err := in.TransformResponse(context.Background(), makeResponse("stop", "Answer", &thinking, nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	var result Message
	json.Unmarshal(data, &result)
	if result.Content[0].Type != "thinking" {
		t.Fatal("第一个 block 应为 thinking")
	}
	if result.Content[0].Signature == nil || *result.Content[0].Signature == "" {
		t.Error("无 signature 时应填充默认值")
	}
}

func TestTransformResponse_ToolCalls(t *testing.T) {
	in := &MessagesInbound{}
	toolCalls := []tmodel.ToolCall{
		{ID: "tu_1", Type: "function", Index: 0, Function: tmodel.FunctionCall{Name: "get_weather", Arguments: `{"city":"Tokyo"}`}},
	}
	data, err := in.TransformResponse(context.Background(), makeResponse("tool_calls", "", nil, nil, toolCalls))
	if err != nil {
		t.Fatal(err)
	}
	var result Message
	json.Unmarshal(data, &result)
	hasToolUse := false
	for _, block := range result.Content {
		if block.Type == "tool_use" {
			hasToolUse = true
			if block.ID != "tu_1" || block.Name == nil || *block.Name != "get_weather" {
				t.Errorf("tool_use block = %+v", block)
			}
		}
	}
	if !hasToolUse {
		t.Error("缺少 tool_use block")
	}
}

func TestTransformResponse_ToolCalls无效Arguments(t *testing.T) {
	in := &MessagesInbound{}
	toolCalls := []tmodel.ToolCall{
		{ID: "tu_1", Type: "function", Index: 0, Function: tmodel.FunctionCall{Name: "fn", Arguments: "invalid{json"}},
	}
	data, err := in.TransformResponse(context.Background(), makeResponse("tool_calls", "", nil, nil, toolCalls))
	if err != nil {
		t.Fatal(err)
	}
	var result Message
	json.Unmarshal(data, &result)
	for _, block := range result.Content {
		if block.Type == "tool_use" {
			if string(block.Input) != "{}" {
				t.Errorf("无效 arguments 应回退为 {}，实际 %s", block.Input)
			}
		}
	}
}

// --- TransformStream ---

func makeStreamChunk(id, model, text string, reasoning *string, finishReason *string) *tmodel.InternalLLMResponse {
	return &tmodel.InternalLLMResponse{
		ID:     id,
		Object: "chat.completion.chunk",
		Model:  model,
		Choices: []tmodel.Choice{{
			Index:        0,
			FinishReason: finishReason,
			Delta: &tmodel.Message{
				Role: "assistant",
				Content: tmodel.MessageContent{
					Content: lo.ToPtr(text),
				},
				ReasoningContent: reasoning,
			},
		}},
	}
}

func makeUsageChunk(id, model string, promptTokens, completionTokens, cachedTokens int64) *tmodel.InternalLLMResponse {
	return &tmodel.InternalLLMResponse{
		ID:     id,
		Object: "chat.completion.chunk",
		Model:  model,
		Choices: []tmodel.Choice{},
		Usage: &tmodel.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
			PromptTokensDetails: &tmodel.PromptTokensDetails{
				CachedTokens: cachedTokens,
			},
		},
	}
}

func TestTransformStream_DONE(t *testing.T) {
	in := &MessagesInbound{}
	done := &tmodel.InternalLLMResponse{Object: "[DONE]"}
	result, err := in.TransformStream(context.Background(), done)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("[DONE] 应返回 nil")
	}
}

func TestTransformStream_基本文本流(t *testing.T) {
	in := &MessagesInbound{}
	ctx := context.Background()

	data, err := in.TransformStream(ctx, makeStreamChunk("id1", "claude-3", "Hello", nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "message_start") {
		t.Error("第一个 chunk 应包含 message_start")
	}
	if !strings.Contains(s, "content_block_start") {
		t.Error("第一个 chunk 应包含 content_block_start")
	}
	if !strings.Contains(s, "text_delta") {
		t.Error("应包含 text_delta")
	}

	data2, err := in.TransformStream(ctx, makeStreamChunk("id1", "claude-3", " world", nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	s2 := string(data2)
	if strings.Contains(s2, "message_start") {
		t.Error("后续 chunk 不应包含 message_start")
	}
	if !strings.Contains(s2, "text_delta") {
		t.Error("应包含 text_delta")
	}
}

func TestTransformStream_思考到文本(t *testing.T) {
	in := &MessagesInbound{}
	ctx := context.Background()

	thinking := "Let me think..."
	data, err := in.TransformStream(ctx, makeStreamChunk("id1", "claude-3", "", &thinking, nil))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "thinking_delta") {
		t.Error("应包含 thinking_delta")
	}

	text := "Answer"
	data2, err := in.TransformStream(ctx, makeStreamChunk("id1", "claude-3", text, nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	s2 := string(data2)
	if !strings.Contains(s2, "content_block_stop") {
		t.Error("切换到文本时应先停止 thinking block")
	}
	if !strings.Contains(s2, "text_delta") {
		t.Error("应包含 text_delta")
	}
}

func TestTransformStream_完整流程(t *testing.T) {
	in := &MessagesInbound{}
	ctx := context.Background()

	in.TransformStream(ctx, makeStreamChunk("id1", "claude-3", "Hello", nil, nil))

	finish := "stop"
	in.TransformStream(ctx, makeStreamChunk("id1", "claude-3", "", nil, &finish))

	data, err := in.TransformStream(ctx, makeUsageChunk("id1", "claude-3", 100, 50, 20))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "message_delta") {
		t.Error("应包含 message_delta")
	}
	if !strings.Contains(s, "message_stop") {
		t.Error("应包含 message_stop")
	}
	if !strings.Contains(s, "end_turn") {
		t.Error("stop 应映射为 end_turn")
	}
}

func TestTransformStream_Finish映射(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"stop", "end_turn"},
		{"length", "max_tokens"},
		{"tool_calls", "tool_use"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			in := &MessagesInbound{}
			ctx := context.Background()
			in.TransformStream(ctx, makeStreamChunk("id1", "claude-3", "hi", nil, nil))
			finish := tt.input
			in.TransformStream(ctx, makeStreamChunk("id1", "claude-3", "", nil, &finish))
			data, err := in.TransformStream(ctx, makeUsageChunk("id1", "claude-3", 10, 5, 0))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), tt.want) {
				t.Errorf("finish %q 应映射为 %q，实际: %s", tt.input, tt.want, string(data))
			}
		})
	}
}

func TestTransformStream_ToolCall流(t *testing.T) {
	in := &MessagesInbound{}
	ctx := context.Background()

	in.TransformStream(ctx, makeStreamChunk("id1", "claude-3", "Let me", nil, nil))

	toolChunk := &tmodel.InternalLLMResponse{
		ID:     "id1",
		Object: "chat.completion.chunk",
		Model:  "claude-3",
		Choices: []tmodel.Choice{{
			Index: 0,
			Delta: &tmodel.Message{
				Role: "assistant",
				ToolCalls: []tmodel.ToolCall{
					{ID: "tu_1", Type: "function", Index: 0, Function: tmodel.FunctionCall{Name: "get_weather", Arguments: `{"city":`}},
				},
			},
		}},
	}
	data, err := in.TransformStream(ctx, toolChunk)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "content_block_stop") {
		t.Error("切换到 tool 时应先停止 text block")
	}
	if !strings.Contains(s, "tool_use") {
		t.Error("应包含 tool_use content_block_start")
	}
	if !strings.Contains(s, "input_json_delta") {
		t.Error("应包含 input_json_delta")
	}

	toolChunk2 := &tmodel.InternalLLMResponse{
		ID:     "id1",
		Object: "chat.completion.chunk",
		Model:  "claude-3",
		Choices: []tmodel.Choice{{
			Index: 0,
			Delta: &tmodel.Message{
				Role: "assistant",
				ToolCalls: []tmodel.ToolCall{
					{Index: 0, Function: tmodel.FunctionCall{Arguments: `"Tokyo"}`}},
				},
			},
		}},
	}
	data2, err := in.TransformStream(ctx, toolChunk2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data2), "input_json_delta") {
		t.Error("续传 tool call 应包含 input_json_delta")
	}
}

// --- GetInternalResponse ---

func TestGetInternalResponse_非流式(t *testing.T) {
	in := &MessagesInbound{}
	in.TransformResponse(context.Background(), makeResponse("stop", "Hello", nil, nil, nil))

	got, err := in.GetInternalResponse(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Choices[0].Message.Content.Content == nil || *got.Choices[0].Message.Content.Content != "Hello" {
		t.Error("返回的响应内容不匹配")
	}
}

func TestGetInternalResponse_流式聚合(t *testing.T) {
	in := &MessagesInbound{}
	ctx := context.Background()

	in.TransformStream(ctx, makeStreamChunk("id1", "claude-3", "Hello", nil, nil))
	in.TransformStream(ctx, makeStreamChunk("id1", "claude-3", " world", nil, nil))

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

func TestGetInternalResponse_空(t *testing.T) {
	in := &MessagesInbound{}
	got, _ := in.GetInternalResponse(context.Background())
	if got != nil {
		t.Error("无数据时应返回 nil")
	}
}

// --- thinkingBudgetToReasoningEffort ---

func TestThinkingBudgetToReasoningEffort(t *testing.T) {
	tests := []struct {
		budget int64
		want   string
	}{
		{1000, EffortLow},
		{5000, EffortLow},
		{5001, EffortMedium},
		{15000, EffortMedium},
		{15001, EffortHigh},
		{100000, EffortHigh},
	}
	for _, tt := range tests {
		got := thinkingBudgetToReasoningEffort(tt.budget)
		if got != tt.want {
			t.Errorf("thinkingBudgetToReasoningEffort(%d) = %q, want %q", tt.budget, got, tt.want)
		}
	}
}

// --- SystemPrompt JSON ---

func TestSystemPrompt_UnmarshalJSON(t *testing.T) {
	t.Run("字符串", func(t *testing.T) {
		var sp SystemPrompt
		if err := json.Unmarshal([]byte(`"You are helpful"`), &sp); err != nil {
			t.Fatal(err)
		}
		if sp.Prompt == nil || *sp.Prompt != "You are helpful" {
			t.Errorf("Prompt = %v", sp.Prompt)
		}
	})
	t.Run("数组", func(t *testing.T) {
		var sp SystemPrompt
		if err := json.Unmarshal([]byte(`[{"type":"text","text":"Rule 1"}]`), &sp); err != nil {
			t.Fatal(err)
		}
		if len(sp.MultiplePrompts) != 1 {
			t.Errorf("MultiplePrompts 长度 = %d", len(sp.MultiplePrompts))
		}
	})
}

func TestSystemPrompt_MarshalJSON(t *testing.T) {
	t.Run("字符串", func(t *testing.T) {
		sp := &SystemPrompt{Prompt: lo.ToPtr("hi")}
		data, err := json.Marshal(sp)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != `"hi"` {
			t.Errorf("Marshal = %s", data)
		}
	})
	t.Run("数组", func(t *testing.T) {
		sp := SystemPrompt{MultiplePrompts: []SystemPromptPart{{Type: "text", Text: "r1"}}}
		data, err := json.Marshal(sp)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "r1") {
			t.Errorf("Marshal = %s", data)
		}
	})
}

// --- MessageContent JSON ---

func TestMessageContent_UnmarshalJSON(t *testing.T) {
	t.Run("字符串", func(t *testing.T) {
		var mc MessageContent
		if err := json.Unmarshal([]byte(`"hello"`), &mc); err != nil {
			t.Fatal(err)
		}
		if mc.Content == nil || *mc.Content != "hello" {
			t.Error("解析字符串失败")
		}
	})
	t.Run("数组", func(t *testing.T) {
		var mc MessageContent
		if err := json.Unmarshal([]byte(`[{"type":"text","text":"hi"}]`), &mc); err != nil {
			t.Fatal(err)
		}
		if len(mc.MultipleContent) != 1 {
			t.Error("解析数组失败")
		}
	})
	t.Run("null", func(t *testing.T) {
		var mc MessageContent
		err := json.Unmarshal([]byte(`null`), &mc)
		if err == nil {
			t.Error("null content 应报错")
		}
	})
}

// --- ExtractTrivalBlocks ---

func TestExtractTrivalBlocks(t *testing.T) {
	t.Run("字符串内容", func(t *testing.T) {
		mc := MessageContent{Content: lo.ToPtr("hello")}
		blocks := mc.ExtractTrivalBlocks(nil)
		if len(blocks) != 1 || blocks[0].Type != "text" {
			t.Errorf("blocks = %v", blocks)
		}
	})
	t.Run("多内容块", func(t *testing.T) {
		mc := MessageContent{
			MultipleContent: []MessageContentBlock{
				{Type: "text", Text: lo.ToPtr("hi")},
				{Type: "image_url"},
			},
		}
		blocks := mc.ExtractTrivalBlocks(nil)
		if len(blocks) != 2 {
			t.Errorf("blocks 长度 = %d", len(blocks))
		}
	})
	t.Run("空内容", func(t *testing.T) {
		mc := MessageContent{Content: lo.ToPtr("")}
		blocks := mc.ExtractTrivalBlocks(nil)
		if len(blocks) != 0 {
			t.Errorf("空字符串应返回空 blocks，实际 %d", len(blocks))
		}
	})
}
