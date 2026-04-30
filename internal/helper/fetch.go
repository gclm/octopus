package helper

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gclm/octopus/internal/model"
	"github.com/gclm/octopus/internal/transformer/outbound"
)

func FetchModels(ctx context.Context, endpoints []model.Endpoint, key string, proxy bool, customHeader []model.CustomHeader) ([]string, error) {
	channel := &model.Channel{Proxy: proxy, CustomHeader: customHeader}
	client, err := ChannelHttpClient(channel)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var allModels []string

	for _, ep := range endpoints {
		if !ep.Enabled || ep.BaseUrl == "" {
			continue
		}
		var models []string
		switch ep.Type {
		case outbound.OutboundTypeAnthropic:
			models, err = fetchAnthropicModels(client, ctx, ep.BaseUrl, key, customHeader)
		case outbound.OutboundTypeGemini:
			models, err = fetchGeminiModels(client, ctx, ep.BaseUrl, key, customHeader)
		default:
			models, err = fetchOpenAIModels(client, ctx, ep.BaseUrl, key, customHeader)
		}
		if err != nil {
			continue
		}
		for _, m := range models {
			if _, ok := seen[m]; !ok {
				seen[m] = struct{}{}
				allModels = append(allModels, m)
			}
		}
	}
	return allModels, nil
}

// refer: https://platform.openai.com/docs/api-reference/models/list
func fetchOpenAIModels(client *http.Client, ctx context.Context, baseURL string, key string, customHeader []model.CustomHeader) ([]string, error) {
	req, _ := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		baseURL+"/models",
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+key)
	for _, header := range customHeader {
		if header.HeaderKey != "" {
			req.Header.Set(header.HeaderKey, header.HeaderValue)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result model.OpenAIModelList

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, m.ID)
	}
	return models, nil
}

// refer: https://ai.google.dev/api/models
func fetchGeminiModels(client *http.Client, ctx context.Context, baseURL string, key string, customHeader []model.CustomHeader) ([]string, error) {
	var allModels []string
	pageToken := ""

	for {
		req, _ := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			baseURL+"/models",
			nil,
		)
		req.Header.Set("X-Goog-Api-Key", key)
		for _, header := range customHeader {
			if header.HeaderKey != "" {
				req.Header.Set(header.HeaderKey, header.HeaderValue)
			}
		}
		if pageToken != "" {
			q := req.URL.Query()
			q.Add("pageToken", pageToken)
			req.URL.RawQuery = q.Encode()
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var result model.GeminiModelList

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		for _, m := range result.Models {
			name := strings.TrimPrefix(m.Name, "models/")
			allModels = append(allModels, name)
		}

		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}
	if len(allModels) == 0 {
		return fetchOpenAIModels(client, ctx, baseURL, key, customHeader)
	}
	return allModels, nil
}

// refer: https://platform.claude.com/docs
func fetchAnthropicModels(client *http.Client, ctx context.Context, baseURL string, key string, customHeader []model.CustomHeader) ([]string, error) {

	var allModels []string
	var afterID string
	for {

		req, _ := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			baseURL+"/models",
			nil,
		)
		req.Header.Set("X-Api-Key", key)
		req.Header.Set("Anthropic-Version", "2023-06-01")
		for _, header := range customHeader {
			if header.HeaderKey != "" {
				req.Header.Set(header.HeaderKey, header.HeaderValue)
			}
		}
		q := req.URL.Query()

		if afterID != "" {
			q.Set("after_id", afterID)
		}
		req.URL.RawQuery = q.Encode()

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var result model.AnthropicModelList

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		for _, m := range result.Data {
			allModels = append(allModels, m.ID)
		}

		if !result.HasMore {
			break
		}

		afterID = result.LastID
	}
	if len(allModels) == 0 {
		return fetchOpenAIModels(client, ctx, baseURL, key, customHeader)
	}
	return allModels, nil
}
