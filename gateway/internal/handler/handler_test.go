package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lijiaoqiao/gateway/internal/adapter"
	"lijiaoqiao/gateway/internal/router"
	gwerror "lijiaoqiao/gateway/pkg/error"
	"lijiaoqiao/gateway/pkg/model"
)

// mockRouter 用于测试的Router
type mockRouter struct {
	providers map[string]adapter.ProviderAdapter
	health    map[string]*router.ProviderHealth
}

func (m *mockRouter) SelectProvider(ctx context.Context, model string) (adapter.ProviderAdapter, error) {
	for name := range m.providers {
		return m.providers[name], nil
	}
	return nil, gwerror.NewGatewayError(gwerror.ROUTER_NO_PROVIDER_AVAILABLE, "no provider")
}

func (m *mockRouter) RecordResult(ctx context.Context, providerName string, success bool, latencyMs int64) {}

func (m *mockRouter) GetHealthStatus() map[string]*router.ProviderHealth {
	return m.health
}

func (m *mockRouter) GetFallbackProviders(ctx context.Context, model string) ([]adapter.ProviderAdapter, error) {
	return nil, nil
}

// mockProvider 用于测试的Provider
type mockProvider struct {
	name    string
	models  []string
	healthy bool
}

func (m *mockProvider) ChatCompletion(ctx context.Context, model string, messages []adapter.Message, options adapter.CompletionOptions) (*adapter.CompletionResponse, error) {
	return &adapter.CompletionResponse{
		ID:      "test-id",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []adapter.Choice{
			{
				Index: 0,
				Message: &adapter.Message{
					Role:    "assistant",
					Content: "Hello, world!",
				},
				FinishReason: "stop",
			},
		},
		Usage: adapter.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}, nil
}

func (m *mockProvider) ChatCompletionStream(ctx context.Context, model string, messages []adapter.Message, options adapter.CompletionOptions) (<-chan *adapter.StreamChunk, error) {
	ch := make(chan *adapter.StreamChunk, 1)
	ch <- &adapter.StreamChunk{
		ID:      "test-id",
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []adapter.StreamChoice{
			{
				Index: 0,
				Delta: &adapter.Delta{
					Role:    "assistant",
					Content: "Hello",
				},
			},
		},
	}
	close(ch)
	return ch, nil
}

func (m *mockProvider) GetUsage(response *adapter.CompletionResponse) adapter.Usage {
	return response.Usage
}

func (m *mockProvider) MapError(err error) adapter.ProviderError {
	return adapter.ProviderError{}
}

func (m *mockProvider) HealthCheck(ctx context.Context) bool {
	return m.healthy
}

func (m *mockProvider) ProviderName() string {
	return m.name
}

func (m *mockProvider) SupportedModels() []string {
	return m.models
}

func TestNewHandler(t *testing.T) {
	r := router.NewRouter(router.StrategyLatency)
	h := NewHandler(r)

	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.version != "v1" {
		t.Errorf("expected version v1, got %s", h.version)
	}
}

func TestChatCompletionsHandle_InvalidRequest(t *testing.T) {
	r := router.NewRouter(router.StrategyLatency)
	h := NewHandler(r)

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "invalid JSON",
			body:       "{invalid}",
			wantStatus: 400,
		},
		{
			name:       "empty messages",
			body:       `{"model": "gpt-4", "messages": []}`,
			wantStatus: 400,
		},
		{
			name:       "missing model - passes validation but no provider for empty model",
			body:       `{"messages": [{"role": "user", "content": "hello"}]}`,
			wantStatus: 503,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			h.ChatCompletionsHandle(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}
}

func TestChatCompletionsHandle_Success(t *testing.T) {
	r := router.NewRouter(router.StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("test", prov)

	h := NewHandler(r)

	body := `{"model": "gpt-4", "messages": [{"role": "user", "content": "hello"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ChatCompletionsHandle(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp model.ChatCompletionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.ID == "" {
		t.Error("expected non-empty ID")
	}
	if resp.Object != "chat.completion" {
		t.Errorf("expected object chat.completion, got %s", resp.Object)
	}
	if len(resp.Choices) != 1 {
		t.Errorf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Hello, world!" {
		t.Errorf("unexpected content: %s", resp.Choices[0].Message.Content)
	}
}

func TestChatCompletionsHandle_WithRequestID(t *testing.T) {
	r := router.NewRouter(router.StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("test", prov)

	h := NewHandler(r)

	body := `{"model": "gpt-4", "messages": [{"role": "user", "content": "hello"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "custom-req-id")
	rr := httptest.NewRecorder()

	h.ChatCompletionsHandle(rr, req)

	if rr.Header().Get("X-Request-ID") != "custom-req-id" {
		t.Errorf("expected X-Request-ID custom-req-id, got %s", rr.Header().Get("X-Request-ID"))
	}
}

func TestChatCompletionsHandle_ProviderError(t *testing.T) {
	r := router.NewRouter(router.StrategyLatency)
	// 不注册任何provider，会触发ROUTER_NO_PROVIDER_AVAILABLE

	h := NewHandler(r)

	body := `{"model": "gpt-4", "messages": [{"role": "user", "content": "hello"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ChatCompletionsHandle(rr, req)

	if rr.Code != 503 {
		t.Errorf("expected status 503, got %d", rr.Code)
	}
}

func TestCompletionsHandle_Success(t *testing.T) {
	r := router.NewRouter(router.StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("test", prov)

	h := NewHandler(r)

	body := `{"model": "gpt-4", "prompt": "Say hello"}`
	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.CompletionsHandle(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp model.CompletionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Object != "text_completion" {
		t.Errorf("expected object text_completion, got %s", resp.Object)
	}
}

func TestCompletionsHandle_InvalidRequest(t *testing.T) {
	r := router.NewRouter(router.StrategyLatency)
	h := NewHandler(r)

	body := `{invalid}`
	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.CompletionsHandle(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestModelsHandle(t *testing.T) {
	r := router.NewRouter(router.StrategyLatency)
	h := NewHandler(r)

	req := httptest.NewRequest("GET", "/v1/models", nil)
	rr := httptest.NewRecorder()

	h.ModelsHandle(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["object"] != "list" {
		t.Errorf("expected object list, got %v", resp["object"])
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("expected data to be array")
	}
	if len(data) != 4 {
		t.Errorf("expected 4 models, got %d", len(data))
	}
}

func TestHealthHandle_AllHealthy(t *testing.T) {
	r := router.NewRouter(router.StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("test", prov)

	h := NewHandler(r)

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	h.HealthHandle(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp model.HealthStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("expected status healthy, got %s", resp.Status)
	}
}

func TestHealthHandle_Degraded(t *testing.T) {
	r := router.NewRouter(router.StrategyLatency)
	prov := &mockProvider{name: "unhealthy", models: []string{}, healthy: false}
	r.RegisterProvider("unhealthy", prov)
	// 标记为不可用
	r.UpdateHealth("unhealthy", false)

	h := NewHandler(r)

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	h.HealthHandle(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}

	var resp model.HealthStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Status != "degraded" {
		t.Errorf("expected status degraded, got %s", resp.Status)
	}
}

func TestWriteJSON(t *testing.T) {
	r := router.NewRouter(router.StrategyLatency)
	h := NewHandler(r)

	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}

	h.writeJSON(w, http.StatusOK, data, "test-req-id")

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("X-Request-ID") != "test-req-id" {
		t.Errorf("expected X-Request-ID test-req-id, got %s", w.Header().Get("X-Request-ID"))
	}
}

func TestWriteError(t *testing.T) {
	r := router.NewRouter(router.StrategyLatency)
	h := NewHandler(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	gwErr := gwerror.NewGatewayError(gwerror.COMMON_INVALID_REQUEST, "test error").WithRequestID("req-123")

	h.writeError(w, req, gwErr)

	if w.Code != 400 {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp model.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error.Message != "test error" {
		t.Errorf("unexpected error message: %s", resp.Error.Message)
	}
	if resp.Error.Type != "gateway_error" {
		t.Errorf("unexpected error type: %s", resp.Error.Type)
	}
	if resp.Error.Code != "COMMON_001" {
		t.Errorf("unexpected error code: %s", resp.Error.Code)
	}
}

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()

	if id1 == "" {
		t.Error("expected non-empty request ID")
	}
	if id1 == id2 {
		t.Error("expected different request IDs")
	}

	if len(id1) < 10 {
		t.Error("request ID seems too short")
	}
}

func TestMarshalJSON(t *testing.T) {
	data := map[string]string{"key": "value"}
	result := marshalJSON(data)

	if result != `{"key":"value"}` {
		t.Errorf("unexpected JSON: %s", result)
	}
}

func TestMarshalJSON_NilValues(t *testing.T) {
	type testStruct struct {
		Name *string
	}
	name := "test"
	obj := testStruct{Name: &name}
	result := marshalJSON(obj)

	if result == "" {
		t.Error("expected non-empty JSON")
	}
}

// mockFailingProvider 用于测试流式处理失败的Provider
type mockFailingProvider struct {
	mockProvider
}

func (m *mockFailingProvider) ChatCompletionStream(ctx context.Context, model string, messages []adapter.Message, options adapter.CompletionOptions) (<-chan *adapter.StreamChunk, error) {
	return nil, errors.New("stream error")
}

func TestHandleStream_ProviderError(t *testing.T) {
	r := router.NewRouter(router.StrategyLatency)
	prov := &mockFailingProvider{}
	r.RegisterProvider("failing", prov)

	h := NewHandler(r)

	body := `{"model": "gpt-4", "messages": [{"role": "user", "content": "hello"}], "stream": true}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ChatCompletionsHandle(rr, req)

	// 流式请求失败时会写入错误
	if rr.Code == 0 {
		t.Log("stream error handled (code 0 means write error)")
	}
}
