package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewOpenAIAdapter(t *testing.T) {
	adapter := NewOpenAIAdapter("https://api.openai.com", "test-key", []string{"gpt-4", "gpt-3.5"})

	if adapter.baseURL != "https://api.openai.com" {
		t.Errorf("unexpected baseURL: %s", adapter.baseURL)
	}
	if adapter.apiKey != "test-key" {
		t.Errorf("unexpected apiKey: %s", adapter.apiKey)
	}
	if len(adapter.models) != 2 {
		t.Errorf("expected 2 models, got %d", len(adapter.models))
	}
	if adapter.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestOpenAIAdapter_ProviderName(t *testing.T) {
	adapter := NewOpenAIAdapter("https://api.openai.com", "test-key", []string{"gpt-4"})
	if adapter.ProviderName() != "openai" {
		t.Errorf("expected openai, got %s", adapter.ProviderName())
	}
}

func TestOpenAIAdapter_SupportedModels(t *testing.T) {
	models := []string{"gpt-4", "gpt-3.5-turbo"}
	adapter := NewOpenAIAdapter("https://api.openai.com", "test-key", models)

	result := adapter.SupportedModels()
	if len(result) != 2 {
		t.Errorf("expected 2 models, got %d", len(result))
	}
	if result[0] != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", result[0])
	}
}

func TestOpenAIAdapter_GetUsage(t *testing.T) {
	adapter := NewOpenAIAdapter("https://api.openai.com", "test-key", []string{"gpt-4"})

	resp := &CompletionResponse{
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	usage := adapter.GetUsage(resp)
	if usage.PromptTokens != 10 {
		t.Errorf("expected 10, got %d", usage.PromptTokens)
	}
	if usage.TotalTokens != 15 {
		t.Errorf("expected 15, got %d", usage.TotalTokens)
	}
}

func TestOpenAIAdapter_MapError(t *testing.T) {
	adapter := NewOpenAIAdapter("https://api.openai.com", "test-key", []string{"gpt-4"})

	tests := []struct {
		name         string
		errMsg       string
		wantCode     string
		wantHTTP     int
		wantRetryable bool
	}{
		{
			name:         "invalid_api_key",
			errMsg:       "invalid_api_key",
			wantCode:     "PROVIDER_001",
			wantHTTP:     401,
			wantRetryable: false,
		},
		{
			name:         "rate_limit",
			errMsg:       "rate_limit exceeded",
			wantCode:     "PROVIDER_002",
			wantHTTP:     429,
			wantRetryable: true,
		},
		{
			name:         "quota",
			errMsg:       "quota exceeded",
			wantCode:     "PROVIDER_003",
			wantHTTP:     402,
			wantRetryable: false,
		},
		{
			name:         "model_not_found",
			errMsg:       "model_not_found error",
			wantCode:     "PROVIDER_004",
			wantHTTP:     404,
			wantRetryable: false,
		},
		{
			name:         "unknown_error",
			errMsg:       "some unknown error",
			wantCode:     "PROVIDER_005",
			wantHTTP:     502,
			wantRetryable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provErr := adapter.MapError(&testError{msg: tt.errMsg})
			if provErr.Code != tt.wantCode {
				t.Errorf("expected code %s, got %s", tt.wantCode, provErr.Code)
			}
			if provErr.HTTPStatus != tt.wantHTTP {
				t.Errorf("expected http status %d, got %d", tt.wantHTTP, provErr.HTTPStatus)
			}
			if provErr.Retryable != tt.wantRetryable {
				t.Errorf("expected retryable %v, got %v", tt.wantRetryable, provErr.Retryable)
			}
		})
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestOpenAIAdapter_ChatCompletion_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type application/json")
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("expected Authorization header")
		}

		// 返回模拟响应
		resp := map[string]interface{}{
			"id":      "chatcmpl-123",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": "Hello!",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	adapter := NewOpenAIAdapter(server.URL, "test-key", []string{"gpt-4"})

	messages := []Message{
		{Role: "user", Content: "Hi"},
	}

	resp, err := adapter.ChatCompletion(context.Background(), "gpt-4", messages, CompletionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ID != "chatcmpl-123" {
		t.Errorf("expected chatcmpl-123, got %s", resp.ID)
	}
	if resp.Choices[0].Message.Content != "Hello!" {
		t.Errorf("expected Hello!, got %s", resp.Choices[0].Message.Content)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("expected 15, got %d", resp.Usage.TotalTokens)
	}
}

func TestOpenAIAdapter_ChatCompletion_WithOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		// 验证选项被正确传递
		if reqBody["temperature"] != 0.7 {
			t.Errorf("expected temperature 0.7, got %v", reqBody["temperature"])
		}
		if reqBody["max_tokens"] != 100.0 {
			t.Errorf("expected max_tokens 100, got %v", reqBody["max_tokens"])
		}
		if reqBody["top_p"] != 0.9 {
			t.Errorf("expected top_p 0.9, got %v", reqBody["top_p"])
		}

		resp := map[string]interface{}{
			"id":      "chatcmpl-123",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": "Hi",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	adapter := NewOpenAIAdapter(server.URL, "test-key", []string{"gpt-4"})

	messages := []Message{{Role: "user", Content: "Hi"}}
	options := CompletionOptions{
		Temperature: 0.7,
		MaxTokens:   100,
		TopP:        0.9,
	}

	_, err := adapter.ChatCompletion(context.Background(), "gpt-4", messages, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenAIAdapter_ChatCompletion_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "invalid_api_key"}}`))
	}))
	defer server.Close()

	adapter := NewOpenAIAdapter(server.URL, "wrong-key", []string{"gpt-4"})

	_, err := adapter.ChatCompletion(context.Background(), "gpt-4", []Message{{Role: "user", Content: "Hi"}}, CompletionOptions{})
	if err == nil {
		t.Fatal("expected error")
	}

	provErr, ok := err.(ProviderError)
	if !ok {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Code != "PROVIDER_001" {
		t.Errorf("expected PROVIDER_001, got %s", provErr.Code)
	}
	if provErr.HTTPStatus != 401 {
		t.Errorf("expected 401, got %d", provErr.HTTPStatus)
	}
}

func TestOpenAIAdapter_ChatCompletion_RateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": {"message": "rate_limit_exceeded"}}`))
	}))
	defer server.Close()

	adapter := NewOpenAIAdapter(server.URL, "test-key", []string{"gpt-4"})

	_, err := adapter.ChatCompletion(context.Background(), "gpt-4", []Message{{Role: "user", Content: "Hi"}}, CompletionOptions{})
	if err == nil {
		t.Fatal("expected error")
	}

	provErr, ok := err.(ProviderError)
	if !ok {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Code != "PROVIDER_002" {
		t.Errorf("expected PROVIDER_002, got %s", provErr.Code)
	}
	if !provErr.Retryable {
		t.Error("expected Retryable to be true")
	}
}

func TestOpenAIAdapter_ChatCompletion_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 长时间等待，确保context会取消
		select {}
	}))
	defer server.Close()

	adapter := NewOpenAIAdapter(server.URL, "test-key", []string{"gpt-4"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, err := adapter.ChatCompletion(ctx, "gpt-4", []Message{{Role: "user", Content: "Hi"}}, CompletionOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "xyz", false},
		{"", "", true},
		{"a", "abc", false},
		{"abc", "abc", true},
	}

	for _, tt := range tests {
		got := contains(tt.s, tt.substr)
		if got != tt.want {
			t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}

func TestContainsHelper(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "lo wo", true},
		{"hello world", "xyz", false},
		{"abc", "abc", true},
		{"abc", "abcd", false},
		{"ab", "abc", false},
	}

	for _, tt := range tests {
		got := containsHelper(tt.s, tt.substr)
		if got != tt.want {
			t.Errorf("containsHelper(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}

func TestOpenAIAdapter_HealthCheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("expected /v1/models, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter := NewOpenAIAdapter(server.URL, "test-key", []string{"gpt-4"})

	healthy := adapter.HealthCheck(context.Background())
	if !healthy {
		t.Error("expected health check to pass")
	}
}

func TestOpenAIAdapter_HealthCheck_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	adapter := NewOpenAIAdapter(server.URL, "wrong-key", []string{"gpt-4"})

	healthy := adapter.HealthCheck(context.Background())
	if healthy {
		t.Error("expected health check to fail")
	}
}

func TestOpenAIAdapter_HealthCheck_ContextTimeout(t *testing.T) {
	// 使用一个会延迟响应的服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 延迟关闭连接
		time.Sleep(10 * time.Second)
	}))
	defer server.Close()

	adapter := NewOpenAIAdapter(server.URL, "test-key", []string{"gpt-4"})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	healthy := adapter.HealthCheck(ctx)
	if healthy {
		t.Error("expected health check to fail due to timeout")
	}
}

func TestOpenAIAdapter_ChatCompletionStream_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求头
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type application/json")
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("expected Authorization header")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		// 发送SSE格式的流式响应
		fmt.Fprintf(w, "data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"created\":1234567890,\"model\":\"gpt-4\",\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"Hello\"},\"finish_reason\":\"stop\"}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	adapter := NewOpenAIAdapter(server.URL, "test-key", []string{"gpt-4"})

	messages := []Message{{Role: "user", Content: "Hi"}}
	ch, err := adapter.ChatCompletionStream(context.Background(), "gpt-4", messages, CompletionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	chunks := 0
	for chunk := range ch {
		chunks++
		if chunk.ID != "chatcmpl-1" {
			t.Errorf("expected chatcmpl-1, got %s", chunk.ID)
		}
	}

	if chunks != 1 {
		t.Errorf("expected 1 chunk, got %d", chunks)
	}
}

func TestOpenAIAdapter_ChatCompletionStream_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "server error"}}`))
	}))
	defer server.Close()

	adapter := NewOpenAIAdapter(server.URL, "test-key", []string{"gpt-4"})

	_, err := adapter.ChatCompletionStream(context.Background(), "gpt-4", []Message{{Role: "user", Content: "Hi"}}, CompletionOptions{})
	if err == nil {
		t.Fatal("expected error")
	}

	provErr, ok := err.(ProviderError)
	if !ok {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	// MapError returns 502 for unknown errors
	if provErr.HTTPStatus != 502 {
		t.Errorf("expected 502, got %d", provErr.HTTPStatus)
	}
}

func TestOpenAIAdapter_ChatCompletionStream_ContextCanceled(t *testing.T) {
	// 这个测试验证当context在请求发送前就被取消时会发生错误
	// 由于context已被取消，http.NewRequestWithContext会立即返回错误
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called when context is already canceled")
	}))
	defer server.Close()

	adapter := NewOpenAIAdapter(server.URL, "test-key", []string{"gpt-4"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	ch, err := adapter.ChatCompletionStream(ctx, "gpt-4", []Message{{Role: "user", Content: "Hi"}}, CompletionOptions{})
	// 当context已取消时，http.NewRequestWithContext会返回错误
	if err == nil {
		t.Fatal("expected error for canceled context")
	}

	// ch可能是nil也可能有值，取决于错误发生的时机
	if ch != nil {
		for range ch {
			// 不应该收到任何数据
		}
	}
}
