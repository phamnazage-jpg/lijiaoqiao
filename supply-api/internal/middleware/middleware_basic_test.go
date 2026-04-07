package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockLogger mock logging.Logger
type mockLogger struct {
	infos []map[string]interface{}
}

func (m *mockLogger) Info(msg string, fields ...map[string]interface{}) {
	if len(fields) > 0 {
		m.infos = append(m.infos, fields[0])
	}
}

func (m *mockLogger) Debug(msg string, fields ...map[string]interface{}) {}
func (m *mockLogger) Warn(msg string, fields ...map[string]interface{}) {}
func (m *mockLogger) Error(msg string, fields ...map[string]interface{}) {}
func (m *mockLogger) Fatal(msg string, fields ...map[string]interface{}) {}

// ==================== Recovery Tests ====================

func TestRecovery_Basic(t *testing.T) {
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	handler := Recovery(nextHandler)

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

func TestRecovery_PanicRecovered(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	handler := Recovery(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestRecovery_NilPanic(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(nil)
	})

	handler := Recovery(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(w, req)
}

// ==================== RequestID Tests ====================

func TestRequestID_WithExistingHeader(t *testing.T) {
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	handler := RequestID(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-Id", "test-request-id")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}
	if w.Header().Get("X-Request-Id") != "test-request-id" {
		t.Errorf("expected X-Request-Id 'test-request-id', got '%s'", w.Header().Get("X-Request-Id"))
	}
}

func TestRequestID_WithUppercaseHeader(t *testing.T) {
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	handler := RequestID(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "test-request-id-uppercase")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}
	if w.Header().Get("X-Request-Id") != "test-request-id-uppercase" {
		t.Errorf("expected X-Request-Id 'test-request-id-uppercase', got '%s'", w.Header().Get("X-Request-Id"))
	}
}

func TestRequestID_NoHeader(t *testing.T) {
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	handler := RequestID(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}
	// Should not set header if not provided
	if w.Header().Get("X-Request-Id") != "" {
		t.Errorf("expected no X-Request-Id, got '%s'", w.Header().Get("X-Request-Id"))
	}
}

// ==================== Logging Tests ====================

func TestLogging_Basic(t *testing.T) {
	logger := &mockLogger{}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := Logging(nextHandler, logger)

	req := httptest.NewRequest("GET", "/api/v1/test?query=123", nil)
	req.Header.Set("X-Request-Id", "req-123")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}
	if len(logger.infos) != 1 {
		t.Errorf("expected 1 log entry, got %d", len(logger.infos))
	}
	if logger.infos[0]["method"] != "GET" {
		t.Errorf("expected method 'GET', got '%v'", logger.infos[0]["method"])
	}
	if logger.infos[0]["path"] != "/api/v1/test" {
		t.Errorf("expected path '/api/v1/test', got '%v'", logger.infos[0]["path"])
	}
	if logger.infos[0]["request_id"] != "req-123" {
		t.Errorf("expected request_id 'req-123', got '%v'", logger.infos[0]["request_id"])
	}
}

func TestLogging_WithTraceContext(t *testing.T) {
	logger := &mockLogger{}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	handler := Logging(nextHandler, logger)

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("X-Request-Id", "req-456")

	// Add trace context to request using exported function
	tc := &TraceContext{
		TraceID: "test-trace-id",
		SpanID:  "test-span-id",
	}
	ctx := WithTraceContext(req.Context(), tc)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called")
	}
	if logger.infos[0]["trace_id"] != "test-trace-id" {
		t.Errorf("expected trace_id 'test-trace-id', got '%v'", logger.infos[0]["trace_id"])
	}
	if logger.infos[0]["span_id"] != "test-span-id" {
		t.Errorf("expected span_id 'test-span-id', got '%v'", logger.infos[0]["span_id"])
	}
}

func TestLogging_NoRequestID(t *testing.T) {
	logger := &mockLogger{}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	})

	handler := Logging(nextHandler, logger)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if _, ok := logger.infos[0]["request_id"]; ok {
		t.Error("should not have request_id in log")
	}
}
