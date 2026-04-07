package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ==================== Idempotency Response Writer Tests ====================

func TestWriteIdempotencyError(t *testing.T) {
	w := httptest.NewRecorder()
	writeIdempotencyError(w, http.StatusConflict, "IDEM_001", "duplicate request")

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", w.Header().Get("Content-Type"))
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["error"].(map[string]interface{})["code"] != "IDEM_001" {
		t.Errorf("expected code 'IDEM_001', got '%v'", resp["error"].(map[string]interface{})["code"])
	}
}

func TestWriteIdempotencyProcessing(t *testing.T) {
	w := httptest.NewRecorder()
	writeIdempotencyProcessing(w, 500, "req-123")

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", w.Code)
	}
	if w.Header().Get("Retry-After-Ms") != "500" {
		t.Errorf("expected Retry-After-Ms '500', got '%s'", w.Header().Get("Retry-After-Ms"))
	}
	if w.Header().Get("X-Request-Id") != "req-123" {
		t.Errorf("expected X-Request-Id 'req-123', got '%s'", w.Header().Get("X-Request-Id"))
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["error"].(map[string]interface{})["code"] != "IDEMPOTENCY_IN_PROGRESS" {
		t.Errorf("expected code 'IDEMPOTENCY_IN_PROGRESS', got '%v'", resp["error"].(map[string]interface{})["code"])
	}
}

func TestWriteIdempotentReplay(t *testing.T) {
	w := httptest.NewRecorder()
	body := json.RawMessage(`{"status":"ok"}`)
	writeIdempotentReplay(w, http.StatusOK, body)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Header().Get("X-Idempotent-Replay") != "true" {
		t.Errorf("expected X-Idempotent-Replay 'true', got '%s'", w.Header().Get("X-Idempotent-Replay"))
	}
}

func TestWriteIdempotentReplay_NilBody(t *testing.T) {
	w := httptest.NewRecorder()
	writeIdempotentReplay(w, http.StatusOK, nil)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// ==================== Context ID Functions Tests ====================

func TestWithTenantID(t *testing.T) {
	ctx := context.Background()
	ctx = WithTenantID(ctx, 123)

	if tenantID := getTenantID(ctx); tenantID != 123 {
		t.Errorf("expected tenantID 123, got %d", tenantID)
	}
}

func TestWithOperatorID(t *testing.T) {
	ctx := context.Background()
	ctx = WithOperatorID(ctx, 456)

	if operatorID := getOperatorID(ctx); operatorID != 456 {
		t.Errorf("expected operatorID 456, got %d", operatorID)
	}
}

func TestGetOperatorID(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, operatorIDKey, int64(789))

	if operatorID := GetOperatorID(ctx); operatorID != 789 {
		t.Errorf("expected operatorID 789, got %d", operatorID)
	}
}

func TestGetOperatorID_NotSet(t *testing.T) {
	ctx := context.Background()

	if operatorID := GetOperatorID(ctx); operatorID != 0 {
		t.Errorf("expected operatorID 0, got %d", operatorID)
	}
}

func TestGetOperatorID_WrongType(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, operatorIDKey, "not an int64")

	if operatorID := GetOperatorID(ctx); operatorID != 0 {
		t.Errorf("expected operatorID 0, got %d", operatorID)
	}
}

// ==================== Status Capturing Response Writer Tests ====================

func TestStatusCapturingResponseWriter_WriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	scrw := &statusCapturingResponseWriter{
		ResponseWriter: w,
		statusCode:    0,
	}

	scrw.WriteHeader(http.StatusCreated)

	if scrw.statusCode != http.StatusCreated {
		t.Errorf("expected statusCode 201, got %d", scrw.statusCode)
	}
	if w.Code != http.StatusCreated {
		t.Errorf("expected w.Code 201, got %d", w.Code)
	}
}

func TestStatusCapturingResponseWriter_Write(t *testing.T) {
	w := httptest.NewRecorder()
	scrw := &statusCapturingResponseWriter{
		ResponseWriter: w,
		statusCode:    http.StatusOK,
		body:          []byte{},
	}

	n, _ := scrw.Write([]byte("hello"))

	if n != 5 {
		t.Errorf("expected 5 bytes written, got %d", n)
	}
	if string(scrw.body) != "hello" {
		t.Errorf("expected body 'hello', got '%s'", string(scrw.body))
	}
}
