package openai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	tmodel "github.com/gclm/octopus/internal/transformer/model"
)

func TestEmbeddingInbound_TransformRequest(t *testing.T) {
	body := `{"model":"text-embedding-3-small","input":"Hello world","dimensions":256}`
	in := &EmbeddingInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if req.Model != "text-embedding-3-small" {
		t.Errorf("Model = %q", req.Model)
	}
	if req.EmbeddingInput == nil || req.EmbeddingInput.Single == nil || *req.EmbeddingInput.Single != "Hello world" {
		t.Errorf("Input = %v", req.EmbeddingInput)
	}
	if req.EmbeddingDimensions == nil || *req.EmbeddingDimensions != 256 {
		t.Errorf("Dimensions = %v", req.EmbeddingDimensions)
	}
	if req.RawAPIFormat != tmodel.APIFormatOpenAIEmbedding {
		t.Errorf("RawAPIFormat = %q", req.RawAPIFormat)
	}
}

func TestEmbeddingInbound_TransformRequest_数组输入(t *testing.T) {
	body := `{"model":"e","input":["a","b"]}`
	in := &EmbeddingInbound{}
	req, err := in.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if req.EmbeddingInput == nil || len(req.EmbeddingInput.Multiple) != 2 {
		t.Errorf("Input = %v", req.EmbeddingInput)
	}
}

func TestEmbeddingInbound_TransformResponse(t *testing.T) {
	in := &EmbeddingInbound{}
	resp := &tmodel.InternalLLMResponse{
		ID:     "emb-1",
		Object: "list",
		Model:  "text-embedding-3-small",
		EmbeddingData: []tmodel.EmbeddingObject{
			{Object: "embedding", Index: 0, Embedding: tmodel.Embedding{FloatArray: []float64{0.1, 0.2}}},
		},
	}
	data, err := in.TransformResponse(context.Background(), resp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "embedding") {
		t.Errorf("响应应包含 embedding，实际: %s", data)
	}
	// JSON 应使用 data 字段（不是 embedding_data）
	var result map[string]json.RawMessage
	json.Unmarshal(data, &result)
	if _, ok := result["data"]; !ok {
		t.Error("缺少 data 字段")
	}
	if _, ok := result["embedding_data"]; ok {
		t.Error("不应包含 embedding_data 字段")
	}
}

func TestEmbeddingInbound_TransformStream_不支持(t *testing.T) {
	in := &EmbeddingInbound{}
	_, err := in.TransformStream(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("应返回不支持的错误，实际: %v", err)
	}
}

func TestEmbeddingInbound_GetInternalResponse(t *testing.T) {
	in := &EmbeddingInbound{}
	resp := &tmodel.InternalLLMResponse{ID: "emb-1"}
	in.TransformResponse(context.Background(), resp)

	got, err := in.GetInternalResponse(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != "emb-1" {
		t.Error("应返回存储的响应")
	}
}

func TestEmbeddingInbound_TransformRequest_无效JSON(t *testing.T) {
	in := &EmbeddingInbound{}
	_, err := in.TransformRequest(context.Background(), []byte(`{bad`))
	if err == nil {
		t.Error("无效 JSON 应报错")
	}
}
