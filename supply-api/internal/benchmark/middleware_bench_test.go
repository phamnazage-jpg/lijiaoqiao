//go:build slow
// +build slow

package benchmark

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/pkg/logging"
	"lijiaoqiao/supply-api/internal/middleware"
)

// mockLogger 用于基准测试的 mock Logger
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...map[string]interface{})  {}
func (m *mockLogger) Info(msg string, fields ...map[string]interface{})   {}
func (m *mockLogger) Warn(msg string, fields ...map[string]interface{})  {}
func (m *mockLogger) Error(msg string, fields ...map[string]interface{}) {}
func (m *mockLogger) Fatal(msg string, fields ...map[string]interface{})  {}
func (m *mockLogger) WithFields(fields map[string]interface{}) logging.Logger { return m }

// BenchmarkLoggingMiddleware 基准测试：日志中间件性能
func BenchmarkLoggingMiddleware(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	logger := &mockLogger{}
	loggingHandler := middleware.Logging(handler, logger)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		loggingHandler.ServeHTTP(rec, req)
	}
}

// BenchmarkTracingMiddleware 基准测试：追踪中间件性能
func BenchmarkTracingMiddleware(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tracingHandler := middleware.TracingMiddleware(handler)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
		rec := httptest.NewRecorder()
		tracingHandler.ServeHTTP(rec, req)
	}
}

// BenchmarkTimeoutMiddleware 基准测试：超时中间件性能
func BenchmarkTimeoutMiddleware(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 使用足够长的超时时间（100ms），确保 handler 几乎总是正常完成
	// 避免因超时设置过短导致的 race 条件
	timeoutHandler := middleware.WithTimeoutMiddleware(handler, 100*time.Millisecond)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		timeoutHandler.ServeHTTP(rec, req)
	}
}

// BenchmarkHTTPHandler_Empty 基准测试：空 Handler 性能（基线）
func BenchmarkHTTPHandler_Empty(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "OK")
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}
