package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTokenBucketLimiter(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(60, 60000, 1.5) // 60 RPM
		ctx := context.Background()

		// Should allow multiple requests
		for i := 0; i < 5; i++ {
			allowed, err := limiter.Allow(ctx, "test-key")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !allowed {
				t.Errorf("request %d should be allowed", i+1)
			}
		}
	})

	t.Run("blocks requests over limit", func(t *testing.T) {
		// Use very low limits for testing
		limiter := &TokenBucketLimiter{
			buckets:        make(map[string]*tokenBucket),
			defaultRPM:     2,
			defaultTPM:     100,
			burstMultiplier: 1.0,
			cleanInterval:  10 * time.Minute,
		}
		// Pre-fill the bucket to capacity
		key := "test-key"
		bucket := limiter.newBucket(2, 100)
		limiter.buckets[key] = bucket

		ctx := context.Background()

		// First two should be allowed
		allowed, _ := limiter.Allow(ctx, key)
		if !allowed {
			t.Error("first request should be allowed")
		}

		allowed, _ = limiter.Allow(ctx, key)
		if !allowed {
			t.Error("second request should be allowed")
		}

		// Third should be blocked
		allowed, _ = limiter.Allow(ctx, key)
		if allowed {
			t.Error("third request should be blocked")
		}
	})

	t.Run("refills tokens over time", func(t *testing.T) {
		limiter := &TokenBucketLimiter{
			buckets:        make(map[string]*tokenBucket),
			defaultRPM:     60,
			defaultTPM:     60000,
			burstMultiplier: 1.0,
			cleanInterval:  10 * time.Minute,
		}
		key := "test-key"

		// Consume all tokens
		for i := 0; i < 60; i++ {
			limiter.Allow(context.Background(), key)
		}

		// Should be blocked now
		allowed, _ := limiter.Allow(context.Background(), key)
		if allowed {
			t.Error("should be blocked after consuming all tokens")
		}

		// Manually backdate the refill time to simulate time passing
		limiter.buckets[key].lastRefill = time.Now().Add(-2 * time.Minute)

		// Should allow again after time-based refill
		allowed, _ = limiter.Allow(context.Background(), key)
		if !allowed {
			t.Error("should allow after token refill")
		}
	})

	t.Run("separate buckets for different keys", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(2, 100, 1.0)
		ctx := context.Background()

		// Exhaust key1
		limiter.Allow(ctx, "key1")
		limiter.Allow(ctx, "key1")

		// key1 should be blocked
		allowed, _ := limiter.Allow(ctx, "key1")
		if allowed {
			t.Error("key1 should be rate limited")
		}

		// key2 should still work
		allowed, _ = limiter.Allow(ctx, "key2")
		if !allowed {
			t.Error("key2 should be allowed")
		}
	})

	t.Run("get limit returns correct values", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(60, 60000, 1.5)
		limiter.Allow(context.Background(), "test-key")

		limit := limiter.GetLimit("test-key")
		if limit.RPM != 60 {
			t.Errorf("expected RPM 60, got %d", limit.RPM)
		}
		if limit.TPM != 60000 {
			t.Errorf("expected TPM 60000, got %d", limit.TPM)
		}
		if limit.Burst != 90 { // 60 * 1.5
			t.Errorf("expected Burst 90, got %d", limit.Burst)
		}
	})
}

func TestSlidingWindowLimiter(t *testing.T) {
	t.Run("allows requests within window", func(t *testing.T) {
		limiter := NewSlidingWindowLimiter(time.Minute, 5)
		ctx := context.Background()

		for i := 0; i < 5; i++ {
			allowed, err := limiter.Allow(ctx, "test-key")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !allowed {
				t.Errorf("request %d should be allowed", i+1)
			}
		}
	})

	t.Run("blocks requests over window limit", func(t *testing.T) {
		limiter := NewSlidingWindowLimiter(time.Minute, 2)
		ctx := context.Background()

		limiter.Allow(ctx, "test-key")
		limiter.Allow(ctx, "test-key")

		allowed, _ := limiter.Allow(ctx, "test-key")
		if allowed {
			t.Error("third request should be blocked")
		}
	})

	t.Run("sliding window respects time", func(t *testing.T) {
		limiter := &SlidingWindowLimiter{
			windows:       make(map[string]*slidingWindow),
			windowSize:    time.Minute,
			maxRequests:   2,
			cleanInterval: 10 * time.Minute,
		}
		ctx := context.Background()
		key := "test-key"

		// Make requests
		limiter.Allow(ctx, key)
		limiter.Allow(ctx, key)

		// Should be blocked
		allowed, _ := limiter.Allow(ctx, key)
		if allowed {
			t.Error("should be blocked after reaching limit")
		}

		// Simulate time passing - move window forward
		limiter.windows[key].requests[0] = time.Now().Add(-2 * time.Minute)
		limiter.windows[key].requests[1] = time.Now().Add(-2 * time.Minute)

		// Should allow now
		allowed, _ = limiter.Allow(ctx, key)
		if !allowed {
			t.Error("should allow after old requests expire from window")
		}
	})

	t.Run("separate windows for different keys", func(t *testing.T) {
		limiter := NewSlidingWindowLimiter(time.Minute, 1)
		ctx := context.Background()

		limiter.Allow(ctx, "key1")

		allowed, _ := limiter.Allow(ctx, "key1")
		if allowed {
			t.Error("key1 should be rate limited")
		}

		allowed, _ = limiter.Allow(ctx, "key2")
		if !allowed {
			t.Error("key2 should be allowed")
		}
	})

	t.Run("get limit returns correct remaining", func(t *testing.T) {
		limiter := NewSlidingWindowLimiter(time.Minute, 10)
		ctx := context.Background()

		limiter.Allow(ctx, "test-key")
		limiter.Allow(ctx, "test-key")
		limiter.Allow(ctx, "test-key")

		limit := limiter.GetLimit("test-key")
		if limit.RPM != 10 {
			t.Errorf("expected RPM 10, got %d", limit.RPM)
		}
		if limit.Remaining != 7 {
			t.Errorf("expected Remaining 7, got %d", limit.Remaining)
		}
	})
}

func TestMiddleware(t *testing.T) {
	t.Run("allows request when under limit", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(60, 60000, 1.5)
		middleware := NewMiddleware(limiter)

		handler := middleware.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("sets rate limit headers when blocked", func(t *testing.T) {
		// Use very low limit so request is blocked
		limiter := &TokenBucketLimiter{
			buckets:        make(map[string]*tokenBucket),
			defaultRPM:     1,
			defaultTPM:     100,
			burstMultiplier: 1.0,
			cleanInterval:  10 * time.Minute,
		}
		// Exhaust the bucket - key is the extracted token, not the full Authorization header
		key := "test-token"
		bucket := limiter.newBucket(1, 100)
		bucket.tokens = 0
		limiter.buckets[key] = bucket

		middleware := NewMiddleware(limiter)
		handler := middleware.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called")
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+key)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Headers should be set when rate limited
		if rr.Header().Get("X-RateLimit-Limit") == "" {
			t.Error("expected X-RateLimit-Limit header to be set")
		}
		if rr.Header().Get("X-RateLimit-Remaining") == "" {
			t.Error("expected X-RateLimit-Remaining header to be set")
		}
		if rr.Header().Get("X-RateLimit-Reset") == "" {
			t.Error("expected X-RateLimit-Reset header to be set")
		}
	})

	t.Run("blocks request when over limit", func(t *testing.T) {
		// Use very low limit
		limiter := &TokenBucketLimiter{
			buckets:        make(map[string]*tokenBucket),
			defaultRPM:     1,
			defaultTPM:     100,
			burstMultiplier: 1.0,
			cleanInterval:  10 * time.Minute,
		}
		// Exhaust the bucket - key is the extracted token, not the full Authorization header
		key := "test-token"
		bucket := limiter.newBucket(1, 100)
		bucket.tokens = 0 // Exhaust
		limiter.buckets[key] = bucket

		middleware := NewMiddleware(limiter)
		handler := middleware.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called")
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+key)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusTooManyRequests {
			t.Errorf("expected status 429, got %d", rr.Code)
		}
	})

	t.Run("uses remote addr when no auth header", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(60, 60000, 1.5)
		middleware := NewMiddleware(limiter)

		handler := middleware.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		// No Authorization header
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})
}
