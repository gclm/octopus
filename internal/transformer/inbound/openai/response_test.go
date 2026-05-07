package openai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	tmodel "github.com/gclm/octopus/internal/transformer/model"
	"github.com/samber/lo"
)

// ==================== TransformRequest ====================

func TestResponseInbound_TransformRequest_文本输入(t *testing.T) {
	body := `{
		"model": "gpt-4o",
		"input": "Hello, how are you?"
	}`
	in := &ResponseInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if req.Model != "gpt-4o" {
		t.Errorf("Model = %q", req.Model)
	}
	if len(req.Messages) != 1 {
		t.Fatalf("Messages 长度 = %d", len(req.Messages))
	}
	if req.Messages[0].Role != "user" {
		t.Errorf("Role = %q", req.Messages[0].Role)
	}
	if req.Messages[0].Content.Content == nil || *req.Messages[0].Content.Content != "Hello, how are you?" {
		t.Error("Content 不匹配")
	}
	if req.RawAPIFormat != tmodel.APIFormatOpenAIResponse {
		t.Errorf("RawAPIFormat = %q", req.RawAPIFormat)
	}
}

func TestResponseInbound_TransformRequest_Instructions映射为System(t *testing.T) {
	body := `{
		"model": "gpt-4o",
		"instructions": "You are a helpful assistant",
		"input": "hi"
	}`
	in := &ResponseInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("Messages 长度 = %d, want 2", len(req.Messages))
	}
	if req.Messages[0].Role != "system" {
		t.Errorf("第一条消息 Role = %q, want system", req.Messages[0].Role)
	}
	if req.Messages[1].Role != "user" {
		t.Errorf("第二条消息 Role = %q, want user", req.Messages[1].Role)
	}
}

func TestResponseInbound_TransformRequest_数组输入(t *testing.T) {
	body := `{
		"model": "gpt-4o",
		"input": [
			{"type": "message", "role": "user", "content": "Hello"},
			{"type": "message", "role": "assistant", "content": "Hi there"}
		]
	}`
	in := &ResponseInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("Messages 长度 = %d", len(req.Messages))
	}
	if req.TransformOptions.ArrayInputs == nil || !*req.TransformOptions.ArrayInputs {
		t.Error("ArrayInputs 应为 true")
	}
}

func TestResponseInbound_TransformRequest_Reasoning配置(t *testing.T) {
	body := `{
		"model": "o3",
		"input": "solve this",
		"reasoning": {"effort": "high", "max_tokens": 10000}
	}`
	in := &ResponseInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if req.ReasoningEffort != "high" {
		t.Errorf("ReasoningEffort = %q", req.ReasoningEffort)
	}
	if req.ReasoningBudget == nil || *req.ReasoningBudget != 10000 {
		t.Errorf("ReasoningBudget = %v", req.ReasoningBudget)
	}
}

func TestResponseInbound_TransformRequest_Tools(t *testing.T) {
	body := `{
		"model": "gpt-4o",
		"input": "weather?",
		"tools": [{"type": "function", "name": "get_weather", "description": "Get weather", "parameters": {"type": "object"}}]
	}`
	in := &ResponseInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("Tools 长度 = %d", len(req.Tools))
	}
	if req.Tools[0].Function.Name != "get_weather" {
		t.Errorf("Tool name = %q", req.Tools[0].Function.Name)
	}
}

func TestResponseInbound_TransformRequest_ImageGenerationTool(t *testing.T) {
	body := `{
		"model": "gpt-4o",
		"input": "generate image",
		"tools": [{"type": "image_generation", "output_format": "png", "size": "1024x1024"}]
	}`
	in := &ResponseInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("Tools 长度 = %d", len(req.Tools))
	}
	if req.Tools[0].Type != "image_generation" {
		t.Errorf("Tool type = %q", req.Tools[0].Type)
	}
}

func TestResponseInbound_TransformRequest_ToolChoice字符串(t *testing.T) {
	body := `{"model":"gpt-4o","input":"hi","tool_choice":"auto"}`
	in := &ResponseInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if req.ToolChoice == nil || req.ToolChoice.ToolChoice == nil || *req.ToolChoice.ToolChoice != "auto" {
		t.Errorf("ToolChoice = %v", req.ToolChoice)
	}
}

func TestResponseInbound_TransformRequest_ToolChoice结构体(t *testing.T) {
	body := `{"model":"gpt-4o","input":"hi","tool_choice":{"type":"function","name":"get_weather"}}`
	in := &ResponseInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if req.ToolChoice == nil || req.ToolChoice.NamedToolChoice == nil {
		t.Fatal("NamedToolChoice 不应为 nil")
	}
	if req.ToolChoice.NamedToolChoice.Function.Name != "get_weather" {
		t.Errorf("Function name = %q", req.ToolChoice.NamedToolChoice.Function.Name)
	}
}

func TestResponseInbound_TransformRequest_TextFormat(t *testing.T) {
	body := `{"model":"gpt-4o","input":"hi","text":{"format":{"type":"json_object"}}}`
	in := &ResponseInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if req.ResponseFormat == nil || req.ResponseFormat.Type != "json_object" {
		t.Errorf("ResponseFormat = %v", req.ResponseFormat)
	}
}

func TestResponseInbound_TransformRequest_额外字段映射(t *testing.T) {
	seed := int64(42)
	fp := 0.5
	pp := 0.3
	store := true
	st := "auto"
	user := "user_1"
	body, _ := json.Marshal(ResponsesRequest{
		Model:              "gpt-4o",
		Input:              ResponsesInput{Text: lo.ToPtr("hi")},
		PreviousResponseID: lo.ToPtr("resp_prev"),
		Seed:               &seed,
		FrequencyPenalty:   &fp,
		PresencePenalty:    &pp,
		Store:              &store,
		ServiceTier:        &st,
		User:               &user,
		MaxOutputTokens:    lo.ToPtr(int64(1000)),
	})
	in := &ResponseInbound{}
	req, err := in.TransformRequest(context.Background(), body)
	if err != nil {
		t.Fatal(err)
	}
	if req.PreviousResponseID == nil || *req.PreviousResponseID != "resp_prev" {
		t.Error("PreviousResponseID 不匹配")
	}
	if req.Seed == nil || *req.Seed != 42 {
		t.Error("Seed 不匹配")
	}
	if req.FrequencyPenalty == nil || *req.FrequencyPenalty != 0.5 {
		t.Error("FrequencyPenalty 不匹配")
	}
	if req.PresencePenalty == nil || *req.PresencePenalty != 0.3 {
		t.Error("PresencePenalty 不匹配")
	}
	if req.MaxCompletionTokens == nil || *req.MaxCompletionTokens != 1000 {
		t.Error("MaxCompletionTokens 不匹配")
	}
}

func TestResponseInbound_TransformRequest_模型必填(t *testing.T) {
	body := `{"input":"hi"}`
	in := &ResponseInbound{}
	_, err := in.TransformRequest(context.Background(), []byte(body))
	if err == nil || !strings.Contains(err.Error(), "model is required") {
		t.Errorf("应返回 model required 错误，实际: %v", err)
	}
}

func TestResponseInbound_TransformRequest_无效JSON(t *testing.T) {
	in := &ResponseInbound{}
	_, err := in.TransformRequest(context.Background(), []byte(`{bad`))
	if err == nil {
		t.Error("无效 JSON 应报错")
	}
}

func TestResponseInbound_TransformRequest_FunctionCallInput(t *testing.T) {
	body := `{
		"model": "gpt-4o",
		"input": [
			{"type": "message", "role": "user", "content": "weather?"},
			{"type": "function_call", "call_id": "fc_1", "name": "get_weather", "arguments": "{\"city\":\"Tokyo\"}"},
			{"type": "function_call_output", "call_id": "fc_1", "output": "Sunny, 25°C"}
		]
	}`
	in := &ResponseInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	// user + assistant(tool_call) + tool(result)
	roles := make([]string, 0, len(req.Messages))
	for _, m := range req.Messages {
		roles = append(roles, m.Role)
	}
	hasAssistant := false
	hasTool := false
	for _, m := range req.Messages {
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			hasAssistant = true
			if m.ToolCalls[0].Function.Name != "get_weather" {
				t.Errorf("function name = %q", m.ToolCalls[0].Function.Name)
			}
		}
		if m.Role == "tool" {
			hasTool = true
			if m.ToolCallID == nil || *m.ToolCallID != "fc_1" {
				t.Errorf("tool ToolCallID = %v", m.ToolCallID)
			}
		}
	}
	if !hasAssistant {
		t.Errorf("缺少 assistant 消息，实际 roles: %v", roles)
	}
	if !hasTool {
		t.Errorf("缺少 tool 消息，实际 roles: %v", roles)
	}
}

func TestResponseInbound_TransformRequest_ImageInput(t *testing.T) {
	body := `{
		"model": "gpt-4o",
		"input": [
			{"type": "input_image", "image_url": "https://example.com/img.png", "detail": "high"}
		]
	}`
	in := &ResponseInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	mc := req.Messages[0].Content.MultipleContent
	if len(mc) != 1 || mc[0].Type != "image_url" {
		t.Errorf("Content = %v", mc)
	}
	if mc[0].ImageURL == nil || mc[0].ImageURL.URL != "https://example.com/img.png" {
		t.Errorf("ImageURL = %v", mc[0].ImageURL)
	}
}

func TestResponseInbound_TransformRequest_ReasoningInput(t *testing.T) {
	body := `{
		"model": "gpt-4o",
		"input": [
			{"type": "reasoning", "summary": [{"type": "summary_text", "text": "I thought about it"}], "encrypted_content": "enc_123"}
		]
	}`
	in := &ResponseInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Messages) != 1 {
		t.Fatal("应有 1 条 assistant 消息")
	}
	if req.Messages[0].ReasoningContent == nil || *req.Messages[0].ReasoningContent != "I thought about it" {
		t.Errorf("ReasoningContent = %v", req.Messages[0].ReasoningContent)
	}
	if req.Messages[0].ReasoningSignature == nil || *req.Messages[0].ReasoningSignature != "enc_123" {
		t.Errorf("ReasoningSignature = %v", req.Messages[0].ReasoningSignature)
	}
}

func TestResponseInbound_TransformRequest_Include字段(t *testing.T) {
	body := `{"model":"gpt-4o","input":"hi","include":["reasoning.encrypted_content","message.input_image.image_url"]}`
	in := &ResponseInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Include) != 2 {
		t.Errorf("Include = %v", req.Include)
	}
}

// ==================== TransformResponse ====================

func makeRespResponse(finishReason, text string, reasoning *string, toolCalls []tmodel.ToolCall) *tmodel.InternalLLMResponse {
	resp := &tmodel.InternalLLMResponse{
		ID:     "resp_1",
		Object: "chat.completion",
		Model:  "gpt-4o",
		Choices: []tmodel.Choice{{
			Index:        0,
			FinishReason: lo.ToPtr(finishReason),
			Message: &tmodel.Message{
				Role:              "assistant",
				ReasoningContent:  reasoning,
				ToolCalls:         toolCalls,
				Content:           tmodel.MessageContent{Content: lo.ToPtr(text)},
			},
		}},
	}
	return resp
}

func TestResponseInbound_TransformResponse_基本响应(t *testing.T) {
	in := &ResponseInbound{}
	data, err := in.TransformResponse(context.Background(), makeRespResponse("stop", "Hello!", nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	var result ResponsesResponse
	json.Unmarshal(data, &result)
	if result.Status == nil || *result.Status != "completed" {
		t.Errorf("Status = %v", result.Status)
	}
	// 应有 message output item
	hasMessage := false
	for _, item := range result.Output {
		if item.Type == "message" {
			hasMessage = true
			if item.Role != "assistant" {
				t.Errorf("Role = %q", item.Role)
			}
		}
	}
	if !hasMessage {
		t.Error("缺少 message output item")
	}
}

func TestResponseInbound_TransformResponse_状态映射(t *testing.T) {
	tests := []struct {
		finish, want string
	}{
		{"stop", "completed"},
		{"length", "incomplete"},
		{"tool_calls", "completed"},
		{"error", "failed"},
	}
	for _, tt := range tests {
		t.Run(tt.finish, func(t *testing.T) {
			in := &ResponseInbound{}
			data, _ := in.TransformResponse(context.Background(), makeRespResponse(tt.finish, "x", nil, nil))
			var result ResponsesResponse
			json.Unmarshal(data, &result)
			if result.Status == nil || *result.Status != tt.want {
				t.Errorf("Status = %v, want %s", result.Status, tt.want)
			}
		})
	}
}

func TestResponseInbound_TransformResponse_Reasoning(t *testing.T) {
	in := &ResponseInbound{}
	reasoning := "I thought deeply"
	data, err := in.TransformResponse(context.Background(), makeRespResponse("stop", "Answer", &reasoning, nil))
	if err != nil {
		t.Fatal(err)
	}
	var result ResponsesResponse
	json.Unmarshal(data, &result)
	hasReasoning := false
	for _, item := range result.Output {
		if item.Type == "reasoning" {
			hasReasoning = true
			if len(item.Summary) == 0 || item.Summary[0].Text != "I thought deeply" {
				t.Errorf("Summary = %v", item.Summary)
			}
		}
	}
	if !hasReasoning {
		t.Error("缺少 reasoning output item")
	}
}

func TestResponseInbound_TransformResponse_ToolCalls(t *testing.T) {
	in := &ResponseInbound{}
	toolCalls := []tmodel.ToolCall{
		{ID: "fc_1", Type: "function", Function: tmodel.FunctionCall{Name: "get_weather", Arguments: `{"city":"Tokyo"}`}},
	}
	data, err := in.TransformResponse(context.Background(), makeRespResponse("tool_calls", "", nil, toolCalls))
	if err != nil {
		t.Fatal(err)
	}
	var result ResponsesResponse
	json.Unmarshal(data, &result)
	hasFnCall := false
	for _, item := range result.Output {
		if item.Type == "function_call" {
			hasFnCall = true
			if item.CallID != "fc_1" || item.Name != "get_weather" {
				t.Errorf("function_call = %+v", item)
			}
		}
	}
	if !hasFnCall {
		t.Error("缺少 function_call output item")
	}
}

func TestResponseInbound_TransformResponse_Usage(t *testing.T) {
	in := &ResponseInbound{}
	resp := makeRespResponse("stop", "hi", nil, nil)
	resp.Usage = &tmodel.Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		PromptTokensDetails: &tmodel.PromptTokensDetails{
			CachedTokens: 30,
		},
		CompletionTokensDetails: &tmodel.CompletionTokensDetails{
			ReasoningTokens: 20,
		},
	}
	data, _ := in.TransformResponse(context.Background(), resp)
	var result ResponsesResponse
	json.Unmarshal(data, &result)
	if result.Usage == nil {
		t.Fatal("Usage 不应为 nil")
	}
	if result.Usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d", result.Usage.InputTokens)
	}
	if result.Usage.InputTokenDetails.CachedTokens != 30 {
		t.Errorf("CachedTokens = %d", result.Usage.InputTokenDetails.CachedTokens)
	}
	if result.Usage.OutputTokenDetails.ReasoningTokens != 20 {
		t.Errorf("ReasoningTokens = %d", result.Usage.OutputTokenDetails.ReasoningTokens)
	}
}

func TestResponseInbound_TransformResponse_NilResponse(t *testing.T) {
	in := &ResponseInbound{}
	_, err := in.TransformResponse(context.Background(), nil)
	if err == nil {
		t.Error("nil response 应报错")
	}
}

func TestResponseInbound_TransformResponse_空输出应生成空message(t *testing.T) {
	in := &ResponseInbound{}
	resp := &tmodel.InternalLLMResponse{
		ID:     "resp_1",
		Object: "chat.completion",
		Model:  "gpt-4o",
		Choices: []tmodel.Choice{{
			Index:        0,
			FinishReason: lo.ToPtr("stop"),
			Message:      &tmodel.Message{Role: "assistant"},
		}},
	}
	data, _ := in.TransformResponse(context.Background(), resp)
	var result ResponsesResponse
	json.Unmarshal(data, &result)
	if len(result.Output) == 0 {
		t.Error("空输出应有默认 message")
	}
}

func TestResponseInbound_TransformResponse_图片内容(t *testing.T) {
	in := &ResponseInbound{}
	resp := &tmodel.InternalLLMResponse{
		ID:     "resp_1",
		Object: "chat.completion",
		Model:  "gpt-4o",
		Choices: []tmodel.Choice{{
			Index:        0,
			FinishReason: lo.ToPtr("stop"),
			Message: &tmodel.Message{
				Role: "assistant",
				Content: tmodel.MessageContent{
					MultipleContent: []tmodel.MessageContentPart{
						{Type: "image_url", ImageURL: &tmodel.ImageURL{URL: "data:image/png;base64,iVBOR"}},
					},
				},
			},
		}},
	}
	data, _ := in.TransformResponse(context.Background(), resp)
	var result ResponsesResponse
	json.Unmarshal(data, &result)
	hasImg := false
	for _, item := range result.Output {
		if item.Type == "image_generation_call" {
			hasImg = true
			if item.Result == nil || !strings.Contains(*item.Result, "iVBOR") {
				t.Errorf("Result = %v", item.Result)
			}
		}
	}
	if !hasImg {
		t.Error("缺少 image_generation_call")
	}
}

// ==================== TransformStream ====================

func makeRespStreamChunk(id, model, text string, reasoning *string, finishReason *string) *tmodel.InternalLLMResponse {
	return &tmodel.InternalLLMResponse{
		ID:     id,
		Object: "chat.completion.chunk",
		Model:  model,
		Choices: []tmodel.Choice{{
			Index:        0,
			FinishReason: finishReason,
			Delta: &tmodel.Message{
				Role:              "assistant",
				ReasoningContent:  reasoning,
				Content:           tmodel.MessageContent{Content: lo.ToPtr(text)},
			},
		}},
	}
}

func makeRespUsageChunk(id, model string, promptTokens, completionTokens int64) *tmodel.InternalLLMResponse {
	return &tmodel.InternalLLMResponse{
		ID:     id,
		Object: "chat.completion.chunk",
		Model:  model,
		Usage: &tmodel.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}
}

func TestResponseInbound_TransformStream_DONE(t *testing.T) {
	in := &ResponseInbound{}
	done := &tmodel.InternalLLMResponse{Object: "[DONE]"}
	data, err := in.TransformStream(context.Background(), done)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "[DONE]") {
		t.Errorf("应包含 [DONE]，实际: %s", data)
	}
}

func TestResponseInbound_TransformStream_文本流(t *testing.T) {
	in := &ResponseInbound{}
	ctx := context.Background()

	data, err := in.TransformStream(ctx, makeRespStreamChunk("resp_1", "gpt-4o", "Hello", nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "response.created") {
		t.Error("第一个 chunk 应包含 response.created")
	}
	if !strings.Contains(s, "response.in_progress") {
		t.Error("应包含 response.in_progress")
	}
	if !strings.Contains(s, "output_text.delta") {
		t.Error("应包含 output_text.delta")
	}
}

func TestResponseInbound_TransformStream_思考到文本(t *testing.T) {
	in := &ResponseInbound{}
	ctx := context.Background()

	reasoning := "Let me think..."
	data, _ := in.TransformStream(ctx, makeRespStreamChunk("resp_1", "gpt-4o", "", &reasoning, nil))
	if !strings.Contains(string(data), "reasoning_summary_text.delta") {
		t.Error("应包含 reasoning_summary_text.delta")
	}

	text := "Answer"
	data2, _ := in.TransformStream(ctx, makeRespStreamChunk("resp_1", "gpt-4o", text, nil, nil))
	s2 := string(data2)
	if !strings.Contains(s2, "reasoning_summary_text.done") {
		t.Error("切换到文本时应先关闭 reasoning")
	}
	if !strings.Contains(s2, "output_text.delta") {
		t.Error("应包含 output_text.delta")
	}
}

func TestResponseInbound_TransformStream_完整流程(t *testing.T) {
	in := &ResponseInbound{}
	ctx := context.Background()

	// 1. 文本内容
	in.TransformStream(ctx, makeRespStreamChunk("resp_1", "gpt-4o", "Hello", nil, nil))

	// 2. finish → 关闭 content part 和 output item
	finish := "stop"
	finishData, _ := in.TransformStream(ctx, makeRespStreamChunk("resp_1", "gpt-4o", "", nil, &finish))
	finishStr := string(finishData)
	if !strings.Contains(finishStr, "output_text.done") {
		t.Error("finish chunk 应包含 output_text.done")
	}
	if !strings.Contains(finishStr, "content_part.done") {
		t.Error("finish chunk 应包含 content_part.done")
	}
	if !strings.Contains(finishStr, "output_item.done") {
		t.Error("finish chunk 应包含 output_item.done")
	}

	// 3. usage → 触发 response.completed
	data, err := in.TransformStream(ctx, makeRespUsageChunk("resp_1", "gpt-4o", 100, 50))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "response.completed") {
		t.Error("应包含 response.completed")
	}
	if !strings.Contains(s, `"completed"`) {
		t.Error("状态应为 completed")
	}
}

func TestResponseInbound_TransformStream_ToolCall流(t *testing.T) {
	in := &ResponseInbound{}
	ctx := context.Background()

	// 文本
	in.TransformStream(ctx, makeRespStreamChunk("resp_1", "gpt-4o", "Let me", nil, nil))

	// tool call
	toolChunk := &tmodel.InternalLLMResponse{
		ID:     "resp_1",
		Object: "chat.completion.chunk",
		Model:  "gpt-4o",
		Choices: []tmodel.Choice{{
			Index: 0,
			Delta: &tmodel.Message{
				ToolCalls: []tmodel.ToolCall{
					{ID: "fc_1", Type: "function", Index: 0, Function: tmodel.FunctionCall{Name: "get_weather", Arguments: `{"city":`}},
				},
			},
		}},
	}
	data, _ := in.TransformStream(ctx, toolChunk)
	s := string(data)
	if !strings.Contains(s, "function_call_arguments.delta") {
		t.Error("应包含 function_call_arguments.delta")
	}

	// 续传
	toolChunk2 := &tmodel.InternalLLMResponse{
		ID:     "resp_1",
		Object: "chat.completion.chunk",
		Model:  "gpt-4o",
		Choices: []tmodel.Choice{{
			Index: 0,
			Delta: &tmodel.Message{
				ToolCalls: []tmodel.ToolCall{
					{Index: 0, Function: tmodel.FunctionCall{Arguments: `"Tokyo"}`}},
				},
			},
		}},
	}
	data2, _ := in.TransformStream(ctx, toolChunk2)
	if !strings.Contains(string(data2), "function_call_arguments.delta") {
		t.Error("续传应包含 function_call_arguments.delta")
	}
}

func TestResponseInbound_TransformStream_空chunk(t *testing.T) {
	in := &ResponseInbound{}
	ctx := context.Background()

	// 无 choices、无 usage 的 chunk（只有 response.created）
	data, err := in.TransformStream(ctx, &tmodel.InternalLLMResponse{
		ID:     "resp_1",
		Object: "chat.completion.chunk",
		Model:  "gpt-4o",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("至少应有 response.created 事件")
	}
}

// ==================== GetInternalResponse ====================

func TestResponseInbound_GetInternalResponse_非流式(t *testing.T) {
	in := &ResponseInbound{}
	in.TransformResponse(context.Background(), makeRespResponse("stop", "Hello", nil, nil))
	got, _ := in.GetInternalResponse(context.Background())
	if got == nil || got.ID != "resp_1" {
		t.Error("应返回存储的响应")
	}
}

func TestResponseInbound_GetInternalResponse_流式聚合(t *testing.T) {
	in := &ResponseInbound{}
	ctx := context.Background()
	in.TransformStream(ctx, makeRespStreamChunk("resp_1", "gpt-4o", "Hello", nil, nil))
	in.TransformStream(ctx, makeRespStreamChunk("resp_1", "gpt-4o", " world", nil, nil))

	got, _ := in.GetInternalResponse(ctx)
	if got == nil {
		t.Fatal("应返回聚合响应")
	}
	content := *got.Choices[0].Message.Content.Content
	if content != "Hello world" {
		t.Errorf("聚合内容 = %q", content)
	}
}

func TestResponseInbound_GetInternalResponse_空(t *testing.T) {
	in := &ResponseInbound{}
	got, _ := in.GetInternalResponse(context.Background())
	if got != nil {
		t.Error("无数据时应返回 nil")
	}
}

// ==================== 辅助转换函数 ====================

func TestConvertInputToMessages_文本(t *testing.T) {
	input := &ResponsesInput{Text: lo.ToPtr("hello")}
	msgs, err := convertInputToMessages(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].Role != "user" {
		t.Errorf("Messages = %v", msgs)
	}
}

func TestConvertInputToMessages_数组(t *testing.T) {
	input := &ResponsesInput{
		Items: []ResponsesItem{
			{Type: "message", Role: "user", Content: &ResponsesInput{Text: lo.ToPtr("hi")}},
			{Type: "message", Role: "assistant", Content: &ResponsesInput{Text: lo.ToPtr("hello")}},
		},
	}
	msgs, err := convertInputToMessages(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Errorf("Messages 长度 = %d", len(msgs))
	}
}

func TestConvertInputToMessages_Nil(t *testing.T) {
	msgs, err := convertInputToMessages(nil)
	if err != nil || msgs != nil {
		t.Errorf("nil 应返回 nil, nil")
	}
}

func TestConvertItemToMessage_InputText(t *testing.T) {
	item := &ResponsesItem{Type: "input_text", Text: lo.ToPtr("hello")}
	msg, err := convertItemToMessage(item)
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || msg.Content.Content == nil || *msg.Content.Content != "hello" {
		t.Errorf("Message = %v", msg)
	}
}

func TestConvertItemToMessage_Nil(t *testing.T) {
	msg, err := convertItemToMessage(nil)
	if err != nil || msg != nil {
		t.Error("nil 应返回 nil, nil")
	}
}

func TestConvertItemToMessage_未知类型(t *testing.T) {
	msg, err := convertItemToMessage(&ResponsesItem{Type: "unknown_type"})
	if err != nil || msg != nil {
		t.Error("未知类型应返回 nil, nil")
	}
}

func TestConvertInputToMessageContent_单项文本(t *testing.T) {
	input := ResponsesInput{Text: lo.ToPtr("hi")}
	mc := convertInputToMessageContent(input)
	if mc.Content == nil || *mc.Content != "hi" {
		t.Errorf("应为简单文本，实际 %v", mc)
	}
}

func TestConvertInputToMessageContent_多项(t *testing.T) {
	input := ResponsesInput{
		Items: []ResponsesItem{
			{Type: "input_text", Text: lo.ToPtr("a")},
			{Type: "input_image", ImageURL: lo.ToPtr("https://example.com/img.png")},
		},
	}
	mc := convertInputToMessageContent(input)
	if len(mc.MultipleContent) != 2 {
		t.Errorf("MultipleContent 长度 = %d", len(mc.MultipleContent))
	}
}

func TestConvertContentItemsToMessageContent_单项(t *testing.T) {
	items := []ResponsesContentItem{{Type: "output_text", Text: "hello"}}
	mc := convertContentItemsToMessageContent(items)
	if mc.Content == nil || *mc.Content != "hello" {
		t.Errorf("应为简单文本，实际 %v", mc)
	}
}

func TestConvertContentItemsToMessageContent_多项(t *testing.T) {
	items := []ResponsesContentItem{
		{Type: "output_text", Text: "a"},
		{Type: "output_text", Text: "b"},
	}
	mc := convertContentItemsToMessageContent(items)
	if len(mc.MultipleContent) != 2 {
		t.Errorf("MultipleContent 长度 = %d", len(mc.MultipleContent))
	}
}

func TestConvertToolChoiceToInternal_Nil(t *testing.T) {
	if result := convertToolChoiceToInternal(nil); result != nil {
		t.Error("nil 应返回 nil")
	}
}

func TestConvertUsageToResponses_Nil(t *testing.T) {
	if result := convertUsageToResponses(nil); result != nil {
		t.Error("nil 应返回 nil")
	}
}

func TestConvertUsageToResponses_完整(t *testing.T) {
	usage := &tmodel.Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		PromptTokensDetails: &tmodel.PromptTokensDetails{CachedTokens: 30},
		CompletionTokensDetails: &tmodel.CompletionTokensDetails{ReasoningTokens: 10},
	}
	result := convertUsageToResponses(usage)
	if result.InputTokens != 100 || result.OutputTokens != 50 {
		t.Errorf("Usage = %+v", result)
	}
	if result.InputTokenDetails.CachedTokens != 30 {
		t.Errorf("CachedTokens = %d", result.InputTokenDetails.CachedTokens)
	}
	if result.OutputTokenDetails.ReasoningTokens != 10 {
		t.Errorf("ReasoningTokens = %d", result.OutputTokenDetails.ReasoningTokens)
	}
}

// ==================== ResponsesInput JSON ====================

func TestResponsesInput_UnmarshalJSON(t *testing.T) {
	t.Run("字符串", func(t *testing.T) {
		var input ResponsesInput
		if err := json.Unmarshal([]byte(`"hello"`), &input); err != nil {
			t.Fatal(err)
		}
		if input.Text == nil || *input.Text != "hello" {
			t.Error("解析字符串失败")
		}
	})
	t.Run("数组", func(t *testing.T) {
		var input ResponsesInput
		if err := json.Unmarshal([]byte(`[{"type":"message","role":"user","content":"hi"}]`), &input); err != nil {
			t.Fatal(err)
		}
		if len(input.Items) != 1 {
			t.Error("解析数组失败")
		}
	})
	t.Run("无效格式", func(t *testing.T) {
		var input ResponsesInput
		err := json.Unmarshal([]byte(`123`), &input)
		if err == nil {
			t.Error("无效格式应报错")
		}
	})
}

func TestResponsesToolChoice_UnmarshalJSON(t *testing.T) {
	t.Run("字符串", func(t *testing.T) {
		var tc ResponsesToolChoice
		if err := json.Unmarshal([]byte(`"auto"`), &tc); err != nil {
			t.Fatal(err)
		}
		if tc.Mode == nil || *tc.Mode != "auto" {
			t.Errorf("Mode = %v", tc.Mode)
		}
	})
	t.Run("结构体", func(t *testing.T) {
		var tc ResponsesToolChoice
		if err := json.Unmarshal([]byte(`{"type":"function","name":"fn"}`), &tc); err != nil {
			t.Fatal(err)
		}
		if tc.Type == nil || *tc.Type != "function" {
			t.Errorf("Type = %v", tc.Type)
		}
	})
	t.Run("无效", func(t *testing.T) {
		var tc ResponsesToolChoice
		err := json.Unmarshal([]byte(`123`), &tc)
		if err == nil {
			t.Error("应报错")
		}
	})
}

func TestResponsesItem_IsOutputMessageContent(t *testing.T) {
	t.Run("有 output_text", func(t *testing.T) {
		item := ResponsesItem{
			Content: &ResponsesInput{
				Items: []ResponsesItem{{Type: "output_text", Text: lo.ToPtr("hi")}},
			},
		}
		if !item.isOutputMessageContent() {
			t.Error("应为 output message content")
		}
	})
	t.Run("无 content", func(t *testing.T) {
		item := ResponsesItem{}
		if item.isOutputMessageContent() {
			t.Error("无 content 不应是")
		}
	})
	t.Run("无 output_text", func(t *testing.T) {
		item := ResponsesItem{
			Content: &ResponsesInput{
				Items: []ResponsesItem{{Type: "input_text", Text: lo.ToPtr("hi")}},
			},
		}
		if item.isOutputMessageContent() {
			t.Error("无 output_text 不应是")
		}
	})
}

func TestResponsesItem_GetContentItems(t *testing.T) {
	t.Run("有内容", func(t *testing.T) {
		item := ResponsesItem{
			Content: &ResponsesInput{
				Items: []ResponsesItem{
					{Type: "output_text", Text: lo.ToPtr("hello")},
					{Type: "output_text", Text: lo.ToPtr("world")},
				},
			},
		}
		items := item.GetContentItems()
		if len(items) != 2 {
			t.Errorf("items 长度 = %d", len(items))
		}
	})
	t.Run("nil content", func(t *testing.T) {
		item := ResponsesItem{}
		if item.GetContentItems() != nil {
			t.Error("nil content 应返回 nil")
		}
	})
}
