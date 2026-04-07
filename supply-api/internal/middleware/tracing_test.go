package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestP106_TraceContextCreation 创建追踪上下文
func TestP106_TraceContextCreation(t *testing.T) {
	tc := NewTraceContext()

	if tc.TraceID == "" {
		t.Error("TraceID should not be empty")
	}

	if tc.SpanID == "" {
		t.Error("SpanID should not be empty")
	}

	if len(tc.TraceID) != 32 {
		t.Errorf("TraceID should be 32 characters, got %d", len(tc.TraceID))
	}

	if len(tc.SpanID) != 16 {
		t.Errorf("SpanID should be 16 characters, got %d", len(tc.SpanID))
	}

	t.Logf("P1-06: TraceID=%s, SpanID=%s", tc.TraceID, tc.SpanID)
}

// TestP106_ParseTraceParent 解析traceparent header
func TestP106_ParseTraceParent(t *testing.T) {
	testCases := []struct {
		name        string
		traceParent string
		wantErr     bool
	}{
		{
			name:        "valid traceparent",
			traceParent: "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
			wantErr:     false,
		},
		{
			name:        "empty traceparent",
			traceParent: "",
			wantErr:     true,
		},
		{
			name:        "invalid version",
			traceParent: "01-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
			wantErr:     true,
		},
		{
			name:        "too short",
			traceParent: "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331",
			wantErr:     true,
		},
		{
			name:        "invalid trace-flags",
			traceParent: "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-02",
			wantErr:     true,
		},
		{
			name:        "not-sampled flag valid",
			traceParent: "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-00",
			wantErr:     false,
		},
		{
			name:        "trace-id too short",
			traceParent: "00-0af7651916cd43dd8448eb211c80319c-b7ad6b716920333-01",
			wantErr:     true,
		},
		{
			name:        "span-id too short",
			traceParent: "00-0af7651916cd43dd8448eb211c80319c-b7ad6b71692033-01",
			wantErr:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := ParseTraceParent(tc.traceParent)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if parsed == nil {
					t.Error("parsed should not be nil")
				}
			}
		})
	}

	t.Log("P1-06: traceparent解析验证通过")
}

// TestP106_FormatTraceParent 格式化traceparent header
func TestP106_FormatTraceParent(t *testing.T) {
	tc := &TraceContext{
		TraceID:   "0af7651916cd43dd8448eb211c80319c",
		SpanID:    "b7ad6b7169203331",
		TraceFlags: TraceFlagSampled,
	}

	formatted := tc.FormatTraceParent()
	expected := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

	if formatted != expected {
		t.Errorf("expected %s, got %s", expected, formatted)
	}

	t.Log("P1-06: traceparent格式化验证通过")
}

// TestP106_ChildSpan 创建子Span
func TestP106_ChildSpan(t *testing.T) {
	parent := &TraceContext{
		TraceID:   "0af7651916cd43dd8448eb211c80319c",
		SpanID:    "b7ad6b7169203331",
		TraceFlags: TraceFlagSampled,
	}

	child := parent.NewChildSpanContext()

	// TraceID应该相同
	if child.TraceID != parent.TraceID {
		t.Error("child TraceID should inherit from parent")
	}

	// SpanID应该不同
	if child.SpanID == parent.SpanID {
		t.Error("child SpanID should be different from parent")
	}

	// TraceFlags应该相同
	if child.TraceFlags != parent.TraceFlags {
		t.Error("child TraceFlags should inherit from parent")
	}

	t.Log("P1-06: 子Span创建验证通过")
}

// TestP106_ContextPropagation Context传播
func TestP106_ContextPropagation(t *testing.T) {
	tc := NewTraceContext()
	ctx := context.Background()

	// 设置到context
	ctx = WithTraceContext(ctx, tc)

	// 从context获取
	retrieved, ok := GetTraceContext(ctx)

	if !ok {
		t.Error("should be able to retrieve TraceContext from context")
	}

	if retrieved.TraceID != tc.TraceID {
		t.Error("retrieved TraceID should match")
	}

	t.Log("P1-06: Context传播验证通过")
}

// TestP106_IsSampled 采样标志检查
func TestP106_IsSampled(t *testing.T) {
	sampled := &TraceContext{
		TraceID:   "0af7651916cd43dd8448eb211c80319c",
		SpanID:    "b7ad6b7169203331",
		TraceFlags: TraceFlagSampled,
	}

	notSampled := &TraceContext{
		TraceID:   "0af7651916cd43dd8448eb211c80319c",
		SpanID:    "b7ad6b7169203331",
		TraceFlags: TraceFlagNotSampled,
	}

	if !sampled.IsSampled() {
		t.Error("sampled context should return true")
	}

	if notSampled.IsSampled() {
		t.Error("not sampled context should return false")
	}

	t.Log("P1-06: 采样标志检查验证通过")
}

// TestP106_Summary 测试总结
func TestP106_Summary(t *testing.T) {
	t.Log("=== P1-006 分布式追踪集成测试总结 ===")
	t.Log("问题: 文档提到request_id和trace_id，但未定义与OpenTelemetry/Jaeger集成")
	t.Log("")
	t.Log("修复方案:")
	t.Log("  - W3C Trace Context标准实现")
	t.Log("  - traceparent header解析和格式化")
	t.Log("  - 支持traceparent/tracestate header")
	t.Log("  - 与现有request_id映射")
}

// ==================== Additional TraceContext Tests ====================

func TestTraceContext_LogFields(t *testing.T) {
	tc := &TraceContext{
		TraceID:   "test-trace-id-12345678901234",
		SpanID:    "test-span-id-1",
		TraceFlags: TraceFlagSampled,
	}

	fields := tc.LogFields()

	if fields["trace_id"] != "test-trace-id-12345678901234" {
		t.Errorf("expected trace_id 'test-trace-id-12345678901234', got '%s'", fields["trace_id"])
	}
	if fields["span_id"] != "test-span-id-1" {
		t.Errorf("expected span_id 'test-span-id-1', got '%s'", fields["span_id"])
	}
}

// ==================== TracingMiddleware Tests ====================

func TestTracingMiddleware_WithValidTraceParent(t *testing.T) {
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		// Verify trace context was injected
		tc, ok := GetTraceContext(r.Context())
		if !ok {
			t.Error("expected trace context in request")
		}
		if tc == nil {
			t.Error("expected non-nil trace context")
		}
	})

	handler := TracingMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}
}

func TestTracingMiddleware_WithInvalidTraceParent(t *testing.T) {
	nextCalled := false
	var capturedCtx context.Context
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		capturedCtx = r.Context()
	})

	handler := TracingMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("traceparent", "invalid-traceparent")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called even with invalid traceparent")
	}

	// Should generate new trace context
	tc, ok := GetTraceContext(capturedCtx)
	if !ok {
		t.Error("expected trace context to be generated")
	}
	if tc.TraceID == "" {
		t.Error("expected non-empty TraceID")
	}
	if tc.SpanID == "" {
		t.Error("expected non-empty SpanID")
	}
}

func TestTracingMiddleware_NoTraceParent(t *testing.T) {
	nextCalled := false
	var capturedCtx context.Context
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		capturedCtx = r.Context()
	})

	handler := TracingMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}

	// Should generate new trace context
	tc, ok := GetTraceContext(capturedCtx)
	if !ok {
		t.Error("expected trace context to be generated")
	}
	if tc.TraceID == "" {
		t.Error("expected non-empty TraceID")
	}
}

func TestTracingMiddleware_PreservesExistingContext(t *testing.T) {
	nextCalled := false
	var capturedCtx context.Context
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		capturedCtx = r.Context()
	})

	handler := TracingMiddleware(nextHandler)

	// Create request with existing context
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(context.Background(), "existing-key", "existing-value"))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}

	// Verify existing context value is preserved
	if capturedCtx.Value("existing-key") != "existing-value" {
		t.Error("expected existing context value to be preserved")
	}
}
