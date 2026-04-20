package adapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIAdapter OpenAI适配器
type OpenAIAdapter struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	models     []string
}

// NewOpenAIAdapter 创建OpenAI适配器
func NewOpenAIAdapter(baseURL, apiKey string, models []string) *OpenAIAdapter {
	// P0-01: 配置带连接池的 HTTP Transport，避免生产环境连接耗尽
	transport := &http.Transport{
		MaxIdleConns:        100,              // 最大空闲连接数
		MaxIdleConnsPerHost: 10,               // 每主机最大空闲连接数
		IdleConnTimeout:     90 * time.Second, // 空闲连接超时
		DisableKeepAlives:   false,            // 启用 keep-alive
	}
	return &OpenAIAdapter{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second,
		},
		models: models,
	}
}

// ChatCompletion 实现ChatCompletion接口
func (a *OpenAIAdapter) ChatCompletion(ctx context.Context, model string, messages []Message, options CompletionOptions) (*CompletionResponse, error) {
	// 构建请求
	reqBody := map[string]interface{}{
		"model": model,
		"messages": messages,
	}

	if options.Temperature > 0 {
		reqBody["temperature"] = options.Temperature
	}
	if options.MaxTokens > 0 {
		reqBody["max_tokens"] = options.MaxTokens
	}
	if options.TopP > 0 {
		reqBody["top_p"] = options.TopP
	}
	if len(options.Stop) > 0 {
		reqBody["stop"] = options.Stop
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 发送请求
	url := fmt.Sprintf("%s/v1/chat/completions", a.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiKey))

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		if json.Unmarshal(body, &errResp) == nil {
			if errDetail, ok := errResp["error"].(map[string]interface{}); ok {
				return nil, a.MapError(fmt.Errorf("%v", errDetail))
			}
		}
		return nil, a.MapError(fmt.Errorf("unexpected status: %d", resp.StatusCode))
	}

	// 解析响应
	var result struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// 转换响应
	response := &CompletionResponse{
		ID:      result.ID,
		Object:  result.Object,
		Created: result.Created,
		Model:   result.Model,
		Choices: make([]Choice, len(result.Choices)),
	}

	for i, c := range result.Choices {
		response.Choices[i] = Choice{
			Message: &Message{
				Role:    c.Message.Role,
				Content: c.Message.Content,
			},
			FinishReason: c.FinishReason,
		}
	}

	response.Usage = Usage{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
		TotalTokens:      result.Usage.TotalTokens,
	}

	return response, nil
}

// ChatCompletionStream 实现流式ChatCompletion
func (a *OpenAIAdapter) ChatCompletionStream(ctx context.Context, model string, messages []Message, options CompletionOptions) (<-chan *StreamChunk, error) {
	// 构建请求
	reqBody := map[string]interface{}{
		"model":    model,
		"messages": messages,
		"stream":   true,
	}

	if options.Temperature > 0 {
		reqBody["temperature"] = options.Temperature
	}
	if options.MaxTokens > 0 {
		reqBody["max_tokens"] = options.MaxTokens
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/chat/completions", a.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiKey))

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, a.MapError(fmt.Errorf("unexpected status: %d, body: %s", resp.StatusCode, string(body)))
	}

	ch := make(chan *StreamChunk, 100)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) < 6 {
				continue
			}

			// SSE格式: data: {...}
			if string(line[:5]) != "data:" {
				continue
			}

			data := line[6:]
			if string(data) == "[DONE]" {
				return
			}

			var chunk struct {
				ID      string `json:"id"`
				Object  string `json:"object"`
				Created int64  `json:"created"`
				Model   string `json:"model"`
				Choices []struct {
					Delta        struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
			}

			if json.Unmarshal(data, &chunk) != nil {
				continue
			}

			streamChunk := &StreamChunk{
				ID:      chunk.ID,
				Object:  chunk.Object,
				Created: chunk.Created,
				Model:   chunk.Model,
				Choices: make([]StreamChoice, len(chunk.Choices)),
			}

			for i, c := range chunk.Choices {
				streamChunk.Choices[i] = StreamChoice{
					Delta: &Delta{
						Role:    c.Delta.Role,
						Content: c.Delta.Content,
					},
					FinishReason: c.FinishReason,
				}
			}

			select {
			case ch <- streamChunk:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// GetUsage 获取使用量
func (a *OpenAIAdapter) GetUsage(response *CompletionResponse) Usage {
	return response.Usage
}

// MapError 错误码映射
func (a *OpenAIAdapter) MapError(err error) ProviderError {
	// 简化实现，实际应根据OpenAI错误响应映射
	errStr := err.Error()

	if contains(errStr, "invalid_api_key") {
		return ProviderError{Code: "PROVIDER_001", Message: "Invalid API key", HTTPStatus: 401, Retryable: false}
	}
	if contains(errStr, "rate_limit") {
		return ProviderError{Code: "PROVIDER_002", Message: "Rate limit exceeded", HTTPStatus: 429, Retryable: true}
	}
	if contains(errStr, "quota") {
		return ProviderError{Code: "PROVIDER_003", Message: "Quota exceeded", HTTPStatus: 402, Retryable: false}
	}
	if contains(errStr, "model_not_found") {
		return ProviderError{Code: "PROVIDER_004", Message: "Model not found", HTTPStatus: 404, Retryable: false}
	}

	return ProviderError{Code: "PROVIDER_005", Message: "Provider error", HTTPStatus: 502, Retryable: true}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// HealthCheck 健康检查
func (a *OpenAIAdapter) HealthCheck(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/v1/models", a.baseURL), nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiKey))

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// ProviderName 供应商名称
func (a *OpenAIAdapter) ProviderName() string {
	return "openai"
}

// SupportedModels 支持的模型列表
func (a *OpenAIAdapter) SupportedModels() []string {
	return a.models
}
