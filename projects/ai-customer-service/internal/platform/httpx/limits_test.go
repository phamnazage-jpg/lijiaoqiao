package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_WithinLimit(t *testing.T) {
	rl := NewRateLimiter(time.Second, 10)
	key := "test-key"

	for i := 0; i < 10; i++ {
		if !rl.Allow(key) {
			t.Errorf("request %d should be allowed (within limit)", i+1)
		}
	}
}

func TestRateLimiter_ExceedLimit(t *testing.T) {
	rl := NewRateLimiter(time.Second, 10)
	key := "test-key"

	// First 10 requests allowed
	for i := 0; i < 10; i++ {
		rl.Allow(key)
	}

	// 11th request should be rejected
	if rl.Allow(key) {
		t.Error("11th request should be rejected (exceed limit)")
	}
}

func TestRateLimiter_DifferentKeys(t *testing.T) {
	rl := NewRateLimiter(time.Second, 10)

	// Use up all quota for key1
	for i := 0; i < 10; i++ {
		rl.Allow("key1")
	}

	// key1 should be rejected now
	if rl.Allow("key1") {
		t.Error("key1 should be rejected after exhausting quota")
	}

	// key2 should still be allowed (different key, independent quota)
	if !rl.Allow("key2") {
		t.Error("key2 should be allowed (different key does not share quota)")
	}
}

func TestRateLimiter_CleanupOldEntries(t *testing.T) {
	rl := NewRateLimiter(50*time.Millisecond, 5)
	key := "cleanup-key"

	// Use up all quota
	for i := 0; i < 5; i++ {
		rl.Allow(key)
	}

	// Verify limit is reached
	if rl.Allow(key) {
		t.Error("should be at limit before cleanup")
	}

	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)

	// After window expires, should be allowed again
	if !rl.Allow(key) {
		t.Error("request should be allowed after old entries are cleaned up")
	}
}

func TestRateLimiter_WithRateLimit(t *testing.T) {
	rl := NewRateLimiter(time.Second, 2)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := rl.WithRateLimit(handler)

	// First two requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}

	// Third request should be rate limited (429)
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
}

func TestRateLimiter_WithRateLimit_XForwardedFor(t *testing.T) {
	rl := NewRateLimiter(time.Second, 1)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := rl.WithRateLimit(handler)

	// First request with X-Forwarded-For should succeed
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("first request: expected 200, got %d", rec.Code)
	}

	// Second request with same IP in X-Forwarded-For should be rejected
	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	rec = httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("second request: expected 429, got %d", rec.Code)
	}

	// Different X-Forwarded-For IP should succeed
	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	req.Header.Set("X-Forwarded-For", "10.0.0.2")
	rec = httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("different IP: expected 200, got %d", rec.Code)
	}
}