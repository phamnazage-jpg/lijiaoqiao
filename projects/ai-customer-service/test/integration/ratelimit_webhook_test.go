package integration

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/platform/httpx"
)

// TestWebhookRateLimit_WithinLimit verifies that 5 requests within 1 second
// all pass when the rate limit is 10 req/s.
func TestWebhookRateLimit_WithinLimit(t *testing.T) {
	rl := httpx.NewRateLimiter(time.Second, 10)

	var passed int
	handler := rl.WithRateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		passed++
		w.WriteHeader(http.StatusOK)
	}))

	// Fresh request each time
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook", bytes.NewBufferString(`{}`))
		req.RemoteAddr = "192.168.1.50:12345"
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i+1, resp.Code)
		}
	}

	if passed != 5 {
		t.Fatalf("passed count = %d, want 5", passed)
	}
}

// TestWebhookRateLimit_ExceedLimit verifies that the 11th request within
// 1 second returns HTTP 429 when the rate limit is 10 req/s.
func TestWebhookRateLimit_ExceedLimit(t *testing.T) {
	rl := httpx.NewRateLimiter(time.Second, 10)

	var passed int
	handler := rl.WithRateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		passed++
		w.WriteHeader(http.StatusOK)
	}))

	// Send 10 requests — all should pass
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook", bytes.NewBufferString(`{}`))
		req.RemoteAddr = "10.0.0.99:54321"
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i+1, resp.Code)
		}
	}

	// 11th request — should be rate-limited
	req11 := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook", bytes.NewBufferString(`{}`))
	req11.RemoteAddr = "10.0.0.99:54321"
	resp11 := httptest.NewRecorder()
	handler.ServeHTTP(resp11, req11)
	if resp11.Code != http.StatusTooManyRequests {
		t.Fatalf("11th request: status = %d, want 429 (rate limited)", resp11.Code)
	}
	if passed != 10 {
		t.Fatalf("passed count = %d, want 10", passed)
	}
}

// TestWebhookRateLimit_DifferentIPs verifies that different IP addresses do
// not share rate limit quota.
func TestWebhookRateLimit_DifferentIPs(t *testing.T) {
	rl := httpx.NewRateLimiter(time.Second, 10)

	var countIP1, countIP2 int
	handler := rl.WithRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Forwarded-For") == "203.0.113.1" {
			countIP1++
		} else {
			countIP2++
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust IP1's quota: 10 requests with X-Forwarded-For: 203.0.113.1
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
		req.Header.Set("X-Forwarded-For", "203.0.113.1")
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
	}

	// Send 5 requests from IP2 — all should pass (independent quota)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
		req.Header.Set("X-Forwarded-For", "203.0.113.2")
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
	}

	if countIP1 != 10 {
		t.Fatalf("IP1 passed count = %d, want 10", countIP1)
	}
	if countIP2 != 5 {
		t.Fatalf("IP2 passed count = %d, want 5", countIP2)
	}

	// Exhaust IP2: send until first 429
	exceeded := false
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
		req.Header.Set("X-Forwarded-For", "203.0.113.2")
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code == http.StatusTooManyRequests {
			exceeded = true
			break
		}
	}
	if !exceeded {
		t.Fatalf("IP2: did not observe 429 after 11 requests within 1 second")
	}
}
