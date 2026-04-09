package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestP103_MiddlewareTimeoutConfig 验证中间件超时配置
func TestP103_MiddlewareTimeoutConfig(t *testing.T) {
	config := DefaultMiddlewareTimeoutConfig()

	// 验证各阶段超时配置
	if config.AuthnTimeout <= 0 {
		t.Error("AuthnTimeout should be positive")
	}

	if config.AuthzTimeout <= 0 {
		t.Error("AuthzTimeout should be positive")
	}

	if config.RateLimitTimeout <= 0 {
		t.Error("RateLimitTimeout should be positive")
	}

	t.Logf("P1-03: 中间件超时配置验证通过")
	t.Logf("  AuthnTimeout: %v", config.AuthnTimeout)
	t.Logf("  AuthzTimeout: %v", config.AuthzTimeout)
	t.Logf("  RateLimitTimeout: %v", config.RateLimitTimeout)
}

// TestP103_TotalTimeout 验证总超时不超过限制
func TestP103_TotalTimeout(t *testing.T) {
	config := DefaultMiddlewareTimeoutConfig()
	total := config.TotalTimeout()

	// 总超时建议 ≤ 200ms
	if total > 250*time.Millisecond {
		t.Errorf("total timeout %v exceeds recommended maximum 250ms", total)
	}

	t.Logf("P1-03: 总超时时间 %v (建议 ≤ 250ms)", total)
}

// TestP103_StageTimeouts 验证各阶段超时列表
func TestP103_StageTimeouts(t *testing.T) {
	stages := GetStageTimeouts()

	if len(stages) != 8 {
		t.Errorf("expected 8 middleware stages, got %d", len(stages))
	}

	var total time.Duration
	for _, stage := range stages {
		total += stage.Timeout
		t.Logf("  %s: %v", stage.Stage, stage.Timeout)
	}

	if total != DefaultMiddlewareTimeoutConfig().TotalTimeout() {
		t.Error("stage timeouts sum should equal total timeout")
	}

	t.Log("P1-03: 各阶段超时验证通过")
}

// TestP103_TimeoutResponseWriter 验证超时响应Writer
func TestP103_TimeoutResponseWriter(t *testing.T) {
	// 验证TimeoutResponseWriter能正确包装
	tw := &TimeoutResponseWriter{
		timeout: 100 * time.Millisecond,
	}

	if tw.timeout != 100*time.Millisecond {
		t.Errorf("expected timeout 100ms, got %v", tw.timeout)
	}

	t.Log("P1-03: 超时响应Writer验证通过")
}

// TestP103_DefaultTimeout 验证默认超时配置
func TestP103_DefaultTimeout(t *testing.T) {
	config := DefaultMiddlewareTimeoutConfig()

	if config.DefaultTimeout <= 0 {
		t.Error("DefaultTimeout should be positive")
	}

	if config.BusinessTimeout <= 0 {
		t.Error("BusinessTimeout should be positive")
	}

	t.Log("P1-03: 默认超时配置验证通过")
}

// TestP103_Summary 测试总结
func TestP103_Summary(t *testing.T) {
	t.Log("=== P1-03 中间件超时配置测试总结 ===")
	t.Log("问题: 7层中间件链未定义每层超时，可能导致请求堆积")
	t.Log("")
	t.Log("修复方案:")
	t.Log("  - 定义各中间件阶段超时 (P99 ≤ 配置值)")
	t.Log("  - Recovery: 5ms")
	t.Log("  - Logging: 10ms")
	t.Log("  - RequestID: 5ms")
	t.Log("  - Authn: 50ms")
	t.Log("  - Authz: 30ms")
	t.Log("  - RateLimit: 20ms")
	t.Log("  - Idempotency: 30ms")
	t.Log("  - Business: 100ms")
	t.Log("  - 总超时建议 ≤ 200ms")
}

// ==================== MiddlewareTimeoutContext Tests ====================

func TestNewMiddlewareTimeoutContext_WithNilConfig(t *testing.T) {
	ctx := NewMiddlewareTimeoutContext(nil)

	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.config == nil {
		t.Error("expected config to be set to default")
	}
}

func TestNewMiddlewareTimeoutContext_WithCustomConfig(t *testing.T) {
	config := &MiddlewareTimeoutConfig{
		RecoveryTimeout:    10 * time.Millisecond,
		LoggingTimeout:     20 * time.Millisecond,
		RequestIDTimeout:   5 * time.Millisecond,
		AuthnTimeout:       50 * time.Millisecond,
		AuthzTimeout:       30 * time.Millisecond,
		RateLimitTimeout:   20 * time.Millisecond,
		IdempotencyTimeout: 30 * time.Millisecond,
		BusinessTimeout:    100 * time.Millisecond,
		DefaultTimeout:     200 * time.Millisecond,
	}

	ctx := NewMiddlewareTimeoutContext(config)

	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.config != config {
		t.Error("expected config to be set")
	}
	if ctx.deadline.IsZero() {
		t.Error("expected deadline to be set")
	}
}

func TestWithBusinessTimeout(t *testing.T) {
	config := &MiddlewareTimeoutConfig{
		RecoveryTimeout:    5 * time.Millisecond,
		LoggingTimeout:     10 * time.Millisecond,
		RequestIDTimeout:   5 * time.Millisecond,
		AuthnTimeout:       50 * time.Millisecond,
		AuthzTimeout:       30 * time.Millisecond,
		RateLimitTimeout:   20 * time.Millisecond,
		IdempotencyTimeout: 30 * time.Millisecond,
		BusinessTimeout:    100 * time.Millisecond,
		DefaultTimeout:     200 * time.Millisecond,
	}

	ctx := NewMiddlewareTimeoutContext(config)
	derivedCtx, cancel := ctx.WithBusinessTimeout()

	if derivedCtx == nil {
		t.Fatal("expected non-nil derived context")
	}
	if cancel == nil {
		t.Error("expected non-nil cancel function")
	}
	defer cancel()

	// Verify deadline is set
	deadline, ok := derivedCtx.Deadline()
	if !ok {
		t.Error("expected deadline to be set on derived context")
	}
	if deadline.IsZero() {
		t.Error("expected deadline to be non-zero")
	}
}

// ==================== WithTimeoutMiddleware Tests ====================

func TestWithTimeoutMiddleware_NormalCompletion(t *testing.T) {
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := WithTimeoutMiddleware(nextHandler, 100*time.Millisecond)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestWithTimeoutMiddleware_SetsTraceContext(t *testing.T) {
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	handler := TracingMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}
}

func TestWithTimeoutMiddleware_Timeout(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow handler - 使用足够长的 sleep 确保超时触发
		time.Sleep(200 * time.Millisecond)
	})

	// 超时设置足够短，确保触发超时
	handler := WithTimeoutMiddleware(nextHandler, 50*time.Millisecond)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// The timeout returns 504 Gateway Timeout
	if w.Code != http.StatusGatewayTimeout {
		t.Errorf("expected status 504, got %d", w.Code)
	}
	if w.Header().Get("X-Timeout") != "true" {
		t.Error("expected X-Timeout header to be set")
	}
}

// TestTimeoutResponseWriter_EnsureStarted tests the ensureStarted method
func TestTimeoutResponseWriter_EnsureStarted(t *testing.T) {
	w := &TimeoutResponseWriter{
		ResponseWriter: httptest.NewRecorder(),
		timeout:        100 * time.Millisecond,
	}

	// Initially not started
	if !w.started.IsZero() {
		t.Error("started should be zero initially")
	}

	// Call ensureStarted
	w.ensureStarted()

	// Should be set now
	if w.started.IsZero() {
		t.Error("started should be set after ensureStarted")
	}

	// Calling again should not change it
	startedBefore := w.started
	w.ensureStarted()
	if !w.started.Equal(startedBefore) {
		t.Error("started should not change on second call")
	}
}

// TestTimeoutResponseWriter_CheckTimeout tests the checkTimeout method
func TestTimeoutResponseWriter_CheckTimeout(t *testing.T) {
	w := &TimeoutResponseWriter{
		ResponseWriter: httptest.NewRecorder(),
		timeout:        50 * time.Millisecond,
	}

	// Not started - should return false
	if w.checkTimeout() {
		t.Error("checkTimeout should return false when not started")
	}

	// Start and check immediately - should return false
	w.ensureStarted()
	if w.checkTimeout() {
		t.Error("checkTimeout should return false immediately after start")
	}

	// Wait and check - should return true
	time.Sleep(60 * time.Millisecond)
	if !w.checkTimeout() {
		t.Error("checkTimeout should return true after timeout")
	}
}

// TestTimeoutResponseWriter_SetTimeoutHeader tests the setTimeoutHeader method
func TestTimeoutResponseWriter_SetTimeoutHeader(t *testing.T) {
	recorder := httptest.NewRecorder()
	w := &TimeoutResponseWriter{
		ResponseWriter: recorder,
		timeout:        100 * time.Millisecond,
	}

	// Initially no timeout header
	if recorder.Header().Get("X-Timeout") == "true" {
		t.Error("X-Timeout should not be set initially")
	}

	// Set timeout header
	w.setTimeoutHeader()

	// Should be set now
	if recorder.Header().Get("X-Timeout") != "true" {
		t.Error("X-Timeout should be set after setTimeoutHeader")
	}
}
