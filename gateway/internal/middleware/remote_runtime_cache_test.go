package middleware

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// P3-A-03/04: Test cache TTL eviction and max_entries eviction

func TestRemoteTokenRuntime_CacheTTL_Eviction(t *testing.T) {
	// Fixed time for deterministic testing
	fixedNow := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)

	calls := 0
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"data":{
						"token_id":"tok-active",
						"subject_id":"user-1",
						"role":"admin",
						"status":"active",
						"scope":["read","write"],
						"issued_at":"2026-04-21T09:00:00Z",
						"expires_at":"2026-04-21T11:00:00Z"
					}
				}`)),
				Header: make(http.Header),
			}, nil
		}),
	}

	// Create runtime with small cache TTL for testing: active=1s, expired=2s, revoked=3s
	runtime := NewRemoteTokenRuntimeWithCacheConfig(
		"http://tok.internal",
		httpClient,
		func() time.Time { return fixedNow },
		CacheConfig{
			ActiveTTL:  1 * time.Second,
			ExpiredTTL: 2 * time.Second,
			RevokedTTL: 3 * time.Second,
			MaxEntries: 10000, // Large enough for this test
		},
	)

	ctx := context.Background()

	// First call - should hit upstream and cache
	_, err := runtime.Verify(ctx, "raw-token-active")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 upstream call, got %d", calls)
	}

	// Second call within TTL - should hit cache
	_, err = runtime.Verify(ctx, "raw-token-active")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 upstream call (cache hit), got %d", calls)
	}

	// Advance time beyond active TTL (1s)
	runtime.AdvanceTime(2 * time.Second)

	// Third call after TTL - should miss cache and call upstream
	_, err = runtime.Verify(ctx, "raw-token-active")
	if err != nil {
		t.Fatalf("Verify failed after TTL expiry: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 upstream calls (TTL expired), got %d", calls)
	}
}

func TestRemoteTokenRuntime_CacheMaxEntries_Eviction(t *testing.T) {
	fixedNow := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)

	callCount := 0
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			callCount++
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"data":{
						"token_id":"tok-evict",
						"subject_id":"user-evict",
						"role":"viewer",
						"status":"active",
						"scope":["read"],
						"issued_at":"2026-04-21T09:00:00Z",
						"expires_at":"2026-04-21T11:00:00Z"
					}
				}`)),
				Header: make(http.Header),
			}, nil
		}),
	}

	// Create runtime with small MaxEntries to trigger eviction
	runtime := NewRemoteTokenRuntimeWithCacheConfig(
		"http://tok.internal",
		httpClient,
		func() time.Time { return fixedNow },
		CacheConfig{
			ActiveTTL:  10 * time.Second,
			ExpiredTTL: 10 * time.Second,
			RevokedTTL: 10 * time.Second,
			MaxEntries: 3, // Small size to trigger eviction
		},
	)

	ctx := context.Background()

	// Fill cache beyond max entries
	for i := 0; i < 5; i++ {
		token := "raw-token-" + strings.TrimSpace(strings.Repeat("x", i))
		_, err := runtime.Verify(ctx, token)
		if err != nil {
			t.Fatalf("Verify %d failed: %v", i, err)
		}
	}

	// At this point, oldest entries should have been evicted
	// A new call with a new token should trigger upstream call
	_, err := runtime.Verify(ctx, "raw-token-new")
	if err != nil {
		t.Fatalf("Verify new token failed: %v", err)
	}

	// Should have called upstream because cache was full and oldest entry was evicted
	if callCount != 6 {
		t.Fatalf("expected 6 upstream calls (oldest entry evicted), got %d", callCount)
	}
}

func TestRemoteTokenRuntime_CacheMetrics(t *testing.T) {
	fixedNow := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)

	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"data":{
						"token_id":"tok-metrics",
						"subject_id":"user-metrics",
						"role":"admin",
						"status":"active",
						"scope":["read","write"],
						"issued_at":"2026-04-21T09:00:00Z",
						"expires_at":"2026-04-21T11:00:00Z"
					}
				}`)),
				Header: make(http.Header),
			}, nil
		}),
	}

	runtime := NewRemoteTokenRuntimeWithCacheConfig(
		"http://tok.internal",
		httpClient,
		func() time.Time { return fixedNow },
		DefaultCacheConfig(),
	)

	ctx := context.Background()

	// First call - should miss cache and call upstream
	runtime.Verify(ctx, "raw-token")
	if h := runtime.GetCacheStats(); h.Cached != 0 || h.Upstream != 1 {
		t.Fatalf("first call: expected cache miss (Cached=0, Upstream=1), got Cached=%d Upstream=%d", h.Cached, h.Upstream)
	}

	// Second call - should hit cache (same token, within TTL)
	runtime.Verify(ctx, "raw-token")
	if h := runtime.GetCacheStats(); h.Cached != 1 || h.Upstream != 1 {
		t.Fatalf("second call: expected cache hit (Cached=1, Upstream=1), got Cached=%d Upstream=%d", h.Cached, h.Upstream)
	}
}
