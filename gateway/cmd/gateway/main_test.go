package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lijiaoqiao/gateway/internal/adapter"
	"lijiaoqiao/gateway/internal/app"
	"lijiaoqiao/gateway/internal/handler"
	"lijiaoqiao/gateway/internal/middleware"
	"lijiaoqiao/gateway/internal/ratelimit"
	"lijiaoqiao/gateway/internal/router"
)

type testProvider struct {
	name   string
	models []string
}

func (p *testProvider) ChatCompletion(_ context.Context, model string, messages []adapter.Message, _ adapter.CompletionOptions) (*adapter.CompletionResponse, error) {
	content := ""
	if len(messages) > 0 {
		content = messages[0].Content
	}
	return &adapter.CompletionResponse{
		ID:      "resp-1",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []adapter.Choice{{Index: 0, Message: &adapter.Message{Role: "assistant", Content: content}, FinishReason: "stop"}},
		Usage:   adapter.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
	}, nil
}

func (p *testProvider) ChatCompletionStream(_ context.Context, _ string, _ []adapter.Message, _ adapter.CompletionOptions) (<-chan *adapter.StreamChunk, error) {
	ch := make(chan *adapter.StreamChunk)
	close(ch)
	return ch, nil
}

func (p *testProvider) GetUsage(response *adapter.CompletionResponse) adapter.Usage {
	return response.Usage
}

func (p *testProvider) MapError(err error) adapter.ProviderError {
	return adapter.ProviderError{Code: "provider_error", Message: err.Error(), HTTPStatus: http.StatusBadGateway}
}

func (p *testProvider) HealthCheck(context.Context) bool {
	return true
}

func (p *testProvider) ProviderName() string {
	return p.name
}

func (p *testProvider) SupportedModels() []string {
	return p.models
}

func TestCreateMux_ProtectsCompletionRoutes(t *testing.T) {
	now := time.Now()
	tokenRuntime := middleware.NewInMemoryTokenRuntime(func() time.Time { return now })
	r := router.NewRouter(router.StrategyLatency)
	h := handler.NewHandler(r)
	limiter := ratelimit.NewMiddleware(ratelimit.NewTokenBucketLimiter(60, 60000, 1.5))

	authConfig := middleware.AuthMiddlewareConfig{
		Verifier:       tokenRuntime,
		StatusResolver: tokenRuntime,
		Authorizer:     middleware.NewScopeRoleAuthorizer(),
		Auditor:        middleware.NewMemoryAuditEmitter(),
		ProtectedPrefixes: []string{
			"/v1/chat/completions",
			"/v1/completions",
			"/api/v1/chat/completions",
			"/api/v1/completions",
		},
		ExcludedPrefixes: []string{"/health", "/healthz", "/readyz"},
		Now:              func() time.Time { return now },
	}

	mux := app.BuildMux(h, limiter, authConfig)

	for _, path := range []string{
		"/v1/chat/completions",
		"/v1/completions",
		"/api/v1/chat/completions",
		"/api/v1/completions",
	} {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{}`))
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected %s to return 401, got %d", path, rr.Code)
		}
	}
}

func TestCreateMux_HealthRoutesRemainOpen(t *testing.T) {
	now := time.Now()
	tokenRuntime := middleware.NewInMemoryTokenRuntime(func() time.Time { return now })
	r := router.NewRouter(router.StrategyLatency)
	h := handler.NewHandler(r)
	limiter := ratelimit.NewMiddleware(ratelimit.NewTokenBucketLimiter(60, 60000, 1.5))

	authConfig := middleware.AuthMiddlewareConfig{
		Verifier:          tokenRuntime,
		StatusResolver:    tokenRuntime,
		Authorizer:        middleware.NewScopeRoleAuthorizer(),
		ProtectedPrefixes: []string{"/v1/chat/completions"},
		ExcludedPrefixes:  []string{"/health", "/healthz", "/readyz"},
		Now:               func() time.Time { return now },
	}

	mux := app.BuildMux(h, limiter, authConfig)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected /health to return 200, got %d", rr.Code)
	}
}

func TestCreateMux_CompletionsRouteUsesCompletionsHandler(t *testing.T) {
	now := time.Now()
	tokenRuntime := middleware.NewInMemoryTokenRuntime(func() time.Time { return now })
	token, err := tokenRuntime.Issue(context.Background(), "user1", "user", []string{"gateway:invoke"}, time.Hour)
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	r := router.NewRouter(router.StrategyLatency)
	r.RegisterProvider("test", &testProvider{name: "test", models: []string{"gpt-4"}})
	h := handler.NewHandler(r)
	limiter := ratelimit.NewMiddleware(ratelimit.NewTokenBucketLimiter(60, 60000, 1.5))

	authConfig := middleware.AuthMiddlewareConfig{
		Verifier:       tokenRuntime,
		StatusResolver: tokenRuntime,
		Authorizer:     middleware.NewScopeRoleAuthorizer(),
		Auditor:        middleware.NewMemoryAuditEmitter(),
		ProtectedPrefixes: []string{
			"/v1/completions",
		},
		ExcludedPrefixes: []string{"/health", "/healthz", "/readyz"},
		Now:              func() time.Time { return now },
	}

	mux := app.BuildMux(h, limiter, authConfig)
	reqBody := `{"model":"gpt-4","prompt":"hello","max_tokens":16}`
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBufferString(reqBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		body, _ := io.ReadAll(rr.Result().Body)
		t.Fatalf("expected 200, got %d: %s", rr.Code, strings.TrimSpace(string(body)))
	}

	body, _ := io.ReadAll(rr.Result().Body)
	if !strings.Contains(string(body), `"object":"text_completion"`) {
		t.Fatalf("expected completions response, got %s", string(body))
	}
}
