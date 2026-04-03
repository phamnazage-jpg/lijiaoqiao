package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lijiaoqiao/gateway/internal/router"
)

func TestMED05_RequestBodySizeLimit(t *testing.T) {
	// MED-05: Request body size should be limited to prevent DoS attacks
	// json.Decoder should use MaxBytes to limit request body size

	r := router.NewRouter(router.StrategyLatency)
	h := NewHandler(r)

	// Create a very large request body (exceeds 1MB limit)
	largeContent := strings.Repeat("a", 2*1024*1024) // 2MB
	largeBody := `{"model": "gpt-4", "messages": [{"role": "user", "content": "` + largeContent + `"}]}`

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(largeBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ChatCompletionsHandle(rr, req)

	// After fix: should return 413 Request Entity Too Large
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413 for large request body, got %d", rr.Code)
	}
}

func TestMED05_NormalRequestShouldPass(t *testing.T) {
	// Normal requests should still work
	r := router.NewRouter(router.StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("test", prov)

	h := NewHandler(r)

	body := `{"model": "gpt-4", "messages": [{"role": "user", "content": "hello"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ChatCompletionsHandle(rr, req)

	// Should succeed (status 200)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for normal request, got %d", rr.Code)
	}
}

func TestMED05_EmptyBodyShouldFail(t *testing.T) {
	// Empty request body should fail
	r := router.NewRouter(router.StrategyLatency)
	h := NewHandler(r)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ChatCompletionsHandle(rr, req)

	// Should fail with 400 Bad Request
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty body, got %d", rr.Code)
	}
}

func TestMED05_InvalidJSONShouldFail(t *testing.T) {
	// Invalid JSON should fail
	r := router.NewRouter(router.StrategyLatency)
	h := NewHandler(r)

	body := `{invalid json}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ChatCompletionsHandle(rr, req)

	// Should fail with 400 Bad Request
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d", rr.Code)
	}
}

// TestMaxBytesReaderWrapper tests the MaxBytes reader wrapper behavior
func TestMaxBytesReaderWrapper(t *testing.T) {
	// Test that limiting reader works correctly
	content := "hello world"
	limitedReader := io.LimitReader(bytes.NewReader([]byte(content)), 5)

	buf := make([]byte, 20)
	n, err := limitedReader.Read(buf)

	// Should only read 5 bytes
	if n != 5 {
		t.Errorf("expected to read 5 bytes, got %d", n)
	}
	if err != nil && err != io.EOF {
		t.Errorf("expected no error or EOF, got %v", err)
	}

	// Reading again should return 0 with EOF
	n2, err2 := limitedReader.Read(buf)
	if n2 != 0 {
		t.Errorf("expected 0 bytes on second read, got %d", n2)
	}
	if err2 != io.EOF {
		t.Errorf("expected EOF on second read, got %v", err2)
	}
}