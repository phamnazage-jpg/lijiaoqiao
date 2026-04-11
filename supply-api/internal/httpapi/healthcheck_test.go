package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockHealthChecker Mock健康检查器
type mockHealthChecker struct {
	name    string
	healthy bool
	err     error
}

func (m *mockHealthChecker) Check(ctx context.Context) error {
	return m.err
}

func (m *mockHealthChecker) Name() string {
	return m.name
}

// TestP107_HealthEndpoint 健康端点
func TestP107_HealthEndpoint(t *testing.T) {
	handler := NewHealthHandler()

	// 添加模拟检查
	handler.AddHealthCheck(&mockHealthChecker{name: "db", healthy: true, err: nil})
	handler.AddHealthCheck(&mockHealthChecker{name: "cache", healthy: true, err: nil})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Status != "healthy" {
		t.Errorf("expected status healthy, got %s", response.Status)
	}

	if len(response.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(response.Checks))
	}

	t.Log("P1-07: 健康端点验证通过")
}

// TestP107_ReadinessEndpoint 就绪端点
func TestP107_ReadinessEndpoint(t *testing.T) {
	handler := NewHealthHandler()

	// 添加模拟检查
	handler.AddReadinessCheck(&mockHealthChecker{name: "db", healthy: true, err: nil})

	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	handler.ServeReadiness(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ReadinessResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Status != "ready" {
		t.Errorf("expected status ready, got %s", response.Status)
	}

	t.Log("P1-07: 就绪端点验证通过")
}

// TestP107_ReadinessNotReady 就绪检查失败
func TestP107_ReadinessNotReady(t *testing.T) {
	handler := NewHealthHandler()

	// 添加失败的检查
	handler.AddReadinessCheck(&mockHealthChecker{name: "db", healthy: false, err: context.DeadlineExceeded})

	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	handler.ServeReadiness(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}

	var response ReadinessResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Status != "not_ready" {
		t.Errorf("expected status not_ready, got %s", response.Status)
	}

	t.Log("P1-07: 就绪检查失败验证通过")
}

// TestP107_LivenessEndpoint 存活端点
func TestP107_LivenessEndpoint(t *testing.T) {
	handler := NewHealthHandler()

	req := httptest.NewRequest("GET", "/live", nil)
	w := httptest.NewRecorder()

	handler.ServeLiveness(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response LivenessResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Status != "alive" {
		t.Errorf("expected status alive, got %s", response.Status)
	}

	t.Log("P1-07: 存活端点验证通过")
}

// TestP107_HealthCheckerInterface 健康检查器接口
func TestP107_HealthCheckerInterface(t *testing.T) {
	// 验证实现了HealthChecker接口
	var _ HealthChecker = &DBHealthChecker{}
	var _ HealthChecker = &CacheHealthChecker{}

	t.Log("P1-07: 健康检查器接口验证通过")
}

// TestP107_ResponseTimestamps 响应时间戳
func TestP107_ResponseTimestamps(t *testing.T) {
	handler := NewHealthHandler()
	handler.AddHealthCheck(&mockHealthChecker{name: "db", healthy: true, err: nil})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHealth(w, req)

	var response HealthResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	if response.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}

	if response.DurationMs < 0 {
		t.Error("duration should be non-negative")
	}

	t.Logf("P1-07: 响应时间戳验证通过 (duration=%dms)", response.DurationMs)
}

// TestP107_Summary 测试总结
func TestP107_Summary(t *testing.T) {
	t.Log("=== P1-007 健康检查和就绪探针测试总结 ===")
	t.Log("问题: 中间件文档提到排除健康检查，但未定义具体端点和逻辑")
	t.Log("")
	t.Log("修复方案:")
	t.Log("  - /health: 综合健康检查 (db, cache, external services)")
	t.Log("  - /ready: 就绪探针 (依赖项是否就绪)")
	t.Log("  - /live: 存活探针 (服务是否存活)")
	t.Log("")
	t.Log("响应示例:")
	t.Log("  /health: {status: healthy, checks: [...]}")
	t.Log("  /ready: {status: ready, checks: [...]}")
	t.Log("  /live: {status: alive}")
}
