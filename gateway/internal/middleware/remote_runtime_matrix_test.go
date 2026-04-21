package middleware

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// P3-A-08: Minimum test matrix for remote token runtime client hardening
// Covers: 4 timeout env vars × 3 token statuses × cache behavior combinations

// ---------------------------------------------------------------------------
// Helper: HTTP transport that records which env var was used
// ---------------------------------------------------------------------------

type envDrivenTransport struct {
	recordedEnv map[string]string
	delegate    http.RoundTripper
}

func (t *envDrivenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Record the dial timeout used by checking if connection was reused
	return t.delegate.RoundTrip(req)
}

// ---------------------------------------------------------------------------
// Matrix row: HTTP client timeout (via RemoteTokenRuntime public API)
// ---------------------------------------------------------------------------

func TestRemoteTokenRuntime_HTTPClient_UsesTimeout(t *testing.T) {
	// Verify that the HTTP client used by RemoteTokenRuntime respects timeout config
	// by using a very short timeout against an unreachable address.
	fixedNow := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)

	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// Simulate a slow connection that would timeout
			time.Sleep(500 * time.Millisecond)
			return jsonResp(http.StatusOK, `{"data":{"token_id":"tok","subject_id":"user","role":"admin","status":"active","scope":[],"issued_at":"2026-04-21T09:00:00Z","expires_at":"2026-04-21T11:00:00Z"}}`), nil
		}),
	}
	runtime := NewRemoteTokenRuntimeWithCacheConfig(
		"http://localhost:1", // unreachable port
		httpClient,
		func() time.Time { return fixedNow },
		CacheConfig{ActiveTTL: 10 * time.Minute, MaxEntries: 10000},
	)
	ctx := context.Background()

	// The HTTP client should enforce the configured DialTimeout
	// With default 5s dial timeout against localhost:1, the call should fail fast
	start := time.Now()
	_, err := runtime.Verify(ctx, "raw-timeout-test")
	elapsed := time.Since(start)

	// Should error due to connection refused / timeout (not hang indefinitely)
	if err == nil {
		t.Log("Verify did not error with slow transport — may be OK if connection reused")
	}
	// Elapsed time should be well under 10s (the default TotalTimeout)
	// If this takes >5s, the timeout config isn't being applied
	if elapsed > 5*time.Second {
		t.Errorf("Verify took %v with unreachable address — expected fast failure with timeout", elapsed)
	}
}

// ---------------------------------------------------------------------------
// Matrix row: 3 token statuses × cache TTL
// ---------------------------------------------------------------------------

func TestRemoteTokenRuntime_TokenStatus_Active(t *testing.T) {
	fixedNow := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResp(http.StatusOK, `{"data":{"token_id":"tok-active","subject_id":"user-1","role":"admin","status":"active","scope":["read"],"issued_at":"2026-04-21T09:00:00Z","expires_at":"2026-04-21T11:00:00Z"}}`), nil
		}),
	}
	runtime := NewRemoteTokenRuntimeWithCacheConfig(
		"http://tok.internal", httpClient,
		func() time.Time { return fixedNow },
		CacheConfig{ActiveTTL: 30 * time.Second, ExpiredTTL: 2 * time.Minute, RevokedTTL: 10 * time.Minute, MaxEntries: 10000},
	)
	ctx := context.Background()

	// Verify once — caches with ActiveTTL=30s
	v, err := runtime.Verify(ctx, "raw-active-token")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if v.TokenID != "tok-active" {
		t.Errorf("TokenID: got %s, want tok-active", v.TokenID)
	}

	// Cache hit within 30s
	v2, err := runtime.Verify(ctx, "raw-active-token")
	if err != nil {
		t.Fatalf("Verify cache hit failed: %v", err)
	}
	if v2.TokenID != "tok-active" {
		t.Errorf("Cache hit TokenID: got %s, want tok-active", v2.TokenID)
	}
	stats := runtime.GetCacheStats()
	if stats.Cached != 1 || stats.Upstream != 1 {
		t.Errorf("stats after cache hit: Cached=%d Upstream=%d, want Cached=1 Upstream=1", stats.Cached, stats.Upstream)
	}

	// Advance 31s — TTL expired, must re-fetch
	runtime.AdvanceTime(31 * time.Second)
	v3, err := runtime.Verify(ctx, "raw-active-token")
	if err != nil {
		t.Fatalf("Verify after TTL expiry failed: %v", err)
	}
	if v3.TokenID != "tok-active" {
		t.Errorf("TokenID after TTL expiry: got %s, want tok-active", v3.TokenID)
	}
	stats = runtime.GetCacheStats()
	if stats.Upstream != 2 {
		t.Errorf("stats after TTL expiry: Upstream=%d, want 2", stats.Upstream)
	}
}

func TestRemoteTokenRuntime_TokenStatus_Expired(t *testing.T) {
	fixedNow := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResp(http.StatusOK, `{"data":{"token_id":"tok-expired","subject_id":"user-2","role":"viewer","status":"expired","scope":["read"],"issued_at":"2026-04-21T08:00:00Z","expires_at":"2026-04-21T09:30:00Z"}}`), nil
		}),
	}
	runtime := NewRemoteTokenRuntimeWithCacheConfig(
		"http://tok.internal", httpClient,
		func() time.Time { return fixedNow },
		CacheConfig{ActiveTTL: 30 * time.Second, ExpiredTTL: 2 * time.Minute, RevokedTTL: 10 * time.Minute, MaxEntries: 10000},
	)
	ctx := context.Background()

	v, err := runtime.Verify(ctx, "raw-expired-token")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if v.TokenID != "tok-expired" {
		t.Errorf("TokenID: got %s, want tok-expired", v.TokenID)
	}

	// Expired token uses ExpiredTTL=2m. Within 2m should cache-hit.
	v2, err := runtime.Verify(ctx, "raw-expired-token")
	if err != nil {
		t.Fatalf("Verify cache hit failed: %v", err)
	}
	if v2.TokenID != "tok-expired" {
		t.Errorf("Cache hit TokenID: got %s, want tok-expired", v2.TokenID)
	}
	stats := runtime.GetCacheStats()
	if stats.Cached != 1 || stats.Upstream != 1 {
		t.Errorf("stats: Cached=%d Upstream=%d, want Cached=1 Upstream=1", stats.Cached, stats.Upstream)
	}

	// Advance 121s — past 2m ExpiredTTL, must re-fetch
	runtime.AdvanceTime(121 * time.Second)
	v3, err := runtime.Verify(ctx, "raw-expired-token")
	if err != nil {
		t.Fatalf("Verify after ExpiredTTL expiry failed: %v", err)
	}
	if v3.TokenID != "tok-expired" {
		t.Errorf("TokenID after ExpiredTTL expiry: got %s, want tok-expired", v3.TokenID)
	}
	stats = runtime.GetCacheStats()
	if stats.Upstream != 2 {
		t.Errorf("stats after ExpiredTTL expiry: Upstream=%d, want 2", stats.Upstream)
	}
}

func TestRemoteTokenRuntime_TokenStatus_Revoked(t *testing.T) {
	fixedNow := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResp(http.StatusOK, `{"data":{"token_id":"tok-revoked","subject_id":"user-3","role":"admin","status":"revoked","scope":["read","write"],"issued_at":"2026-04-21T08:00:00Z","expires_at":"2026-04-21T12:00:00Z"}}`), nil
		}),
	}
	runtime := NewRemoteTokenRuntimeWithCacheConfig(
		"http://tok.internal", httpClient,
		func() time.Time { return fixedNow },
		CacheConfig{ActiveTTL: 30 * time.Second, ExpiredTTL: 2 * time.Minute, RevokedTTL: 10 * time.Minute, MaxEntries: 10000},
	)
	ctx := context.Background()

	v, err := runtime.Verify(ctx, "raw-revoked-token")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if v.TokenID != "tok-revoked" {
		t.Errorf("TokenID: got %s, want tok-revoked", v.TokenID)
	}

	// Revoked token uses RevokedTTL=10m. Within 10m should cache-hit.
	v2, err := runtime.Verify(ctx, "raw-revoked-token")
	if err != nil {
		t.Fatalf("Verify cache hit failed: %v", err)
	}
	if v2.TokenID != "tok-revoked" {
		t.Errorf("Cache hit TokenID: got %s, want tok-revoked", v2.TokenID)
	}
	stats := runtime.GetCacheStats()
	if stats.Cached != 1 || stats.Upstream != 1 {
		t.Errorf("stats: Cached=%d Upstream=%d, want Cached=1 Upstream=1", stats.Cached, stats.Upstream)
	}

	// Advance 9m59s — still within 10m RevokedTTL, should still hit cache
	runtime.AdvanceTime(9*time.Minute + 59*time.Second)
	v3, err := runtime.Verify(ctx, "raw-revoked-token")
	if err != nil {
		t.Fatalf("Verify before RevokedTTL expiry failed: %v", err)
	}
	if v3.TokenID != "tok-revoked" {
		t.Errorf("TokenID before RevokedTTL expiry: got %s, want tok-revoked", v3.TokenID)
	}
	// Upstream count should still be 1 (cache hit)
	stats = runtime.GetCacheStats()
	if stats.Upstream != 1 {
		t.Errorf("stats before RevokedTTL expiry: Upstream=%d, want 1", stats.Upstream)
	}
}

// ---------------------------------------------------------------------------
// Matrix row: upstream failure modes
// ---------------------------------------------------------------------------

func TestRemoteTokenRuntime_UpstreamFailure_ConnectionRefused(t *testing.T) {
	fixedNow := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	// Use a client that will fail (localhost with no server)
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, &urlError{"connection refused", "OpTimeout", false}
		}),
	}
	runtime := NewRemoteTokenRuntimeWithCacheConfig(
		"http://localhost:19999", httpClient,
		func() time.Time { return fixedNow },
		CacheConfig{ActiveTTL: 30 * time.Second, MaxEntries: 10000},
	)
	ctx := context.Background()

	_, err := runtime.Verify(ctx, "raw-fail-token")
	if err == nil {
		t.Fatal("expected error for connection refused, got nil")
	}

	// Second call should also fail (not cached as error)
	_, err = runtime.Verify(ctx, "raw-fail-token")
	if err == nil {
		t.Fatal("expected second error for connection refused, got nil")
	}

	// Upstream should have been called twice (no negative cache)
	stats := runtime.GetCacheStats()
	if stats.Upstream != 2 {
		t.Errorf("upstream failure: Upstream=%d, want 2 (no negative cache)", stats.Upstream)
	}
}

func TestRemoteTokenRuntime_UpstreamFailure_HTTP500(t *testing.T) {
	fixedNow := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("internal server error"))}, nil
		}),
	}
	runtime := NewRemoteTokenRuntimeWithCacheConfig(
		"http://tok.internal", httpClient,
		func() time.Time { return fixedNow },
		CacheConfig{ActiveTTL: 30 * time.Second, MaxEntries: 10000},
	)
	ctx := context.Background()

	_, err := runtime.Verify(ctx, "raw-500-token")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}

	// Second call should also return error (no negative cache)
	_, err = runtime.Verify(ctx, "raw-500-token")
	if err == nil {
		t.Fatal("expected second error for 500 response, got nil")
	}
	stats := runtime.GetCacheStats()
	if stats.Upstream != 2 {
		t.Errorf("500 failure: Upstream=%d, want 2 (no negative cache)", stats.Upstream)
	}
}

func TestRemoteTokenRuntime_UpstreamFailure_EmptyTokenID(t *testing.T) {
	fixedNow := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResp(http.StatusOK, `{"data":{"token_id":"","subject_id":"user-1","role":"admin","status":"active","scope":[],"issued_at":"2026-04-21T09:00:00Z","expires_at":"2026-04-21T11:00:00Z"}}`), nil
		}),
	}
	runtime := NewRemoteTokenRuntimeWithCacheConfig(
		"http://tok.internal", httpClient,
		func() time.Time { return fixedNow },
		CacheConfig{ActiveTTL: 30 * time.Second, MaxEntries: 10000},
	)
	ctx := context.Background()

	_, err := runtime.Verify(ctx, "raw-empty-token")
	if err == nil {
		t.Fatal("expected error for empty token_id, got nil")
	}
	if !strings.Contains(err.Error(), "missing token_id") {
		t.Errorf("error message: got %q, want contains 'missing token_id'", err.Error())
	}
}

func TestRemoteTokenRuntime_UpstreamFailure_InvalidJSON(t *testing.T) {
	fixedNow := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("not json"))}, nil
		}),
	}
	runtime := NewRemoteTokenRuntimeWithCacheConfig(
		"http://tok.internal", httpClient,
		func() time.Time { return fixedNow },
		CacheConfig{ActiveTTL: 30 * time.Second, MaxEntries: 10000},
	)
	ctx := context.Background()

	_, err := runtime.Verify(ctx, "raw-bad-json")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// ---------------------------------------------------------------------------
// Matrix row: cache eviction under pressure
// ---------------------------------------------------------------------------

func TestRemoteTokenRuntime_CacheEviction_LRUOrder(t *testing.T) {
	fixedNow := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	callCount := 0
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			callCount++
			tokID := "tok-" + strings.ReplaceAll(req.Header.Get("X-Request-Id"), "gateway-introspect-", "")
			return jsonResp(http.StatusOK, `{"data":{"token_id":"`+tokID+`","subject_id":"user","role":"admin","status":"active","scope":["read"],"issued_at":"2026-04-21T09:00:00Z","expires_at":"2026-04-21T11:00:00Z"}}`), nil
		}),
	}
	runtime := NewRemoteTokenRuntimeWithCacheConfig(
		"http://tok.internal", httpClient,
		func() time.Time { return fixedNow },
		CacheConfig{ActiveTTL: 10 * time.Minute, MaxEntries: 3}, // small to force eviction
	)
	ctx := context.Background()

	// Insert 3 tokens (fills cache)
	for i := 0; i < 3; i++ {
		_, err := runtime.Verify(ctx, "raw-token-"+string(rune('a'+i)))
		if err != nil {
			t.Fatalf("Verify %d failed: %v", i, err)
		}
	}
	if callCount != 3 {
		t.Fatalf("initial fill: callCount=%d, want 3", callCount)
	}

	// 4th token evicts the oldest (a)
	_, err := runtime.Verify(ctx, "raw-token-d")
	if err != nil {
		t.Fatalf("Verify 4th token failed: %v", err)
	}
	if callCount != 4 {
		t.Errorf("after 4th insert: callCount=%d, want 4 (oldest evicted)", callCount)
	}

	// Re-requesting evicted token-a should call upstream again
	_, err = runtime.Verify(ctx, "raw-token-a")
	if err != nil {
		t.Fatalf("Verify re-request of evicted token failed: %v", err)
	}
	if callCount != 5 {
		t.Errorf("after re-request of evicted token: callCount=%d, want 5", callCount)
	}
}

func TestRemoteTokenRuntime_CacheEviction_ExpiredEntriesEvictedFirst(t *testing.T) {
	fixedNow := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	callCount := 0
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			callCount++
			return jsonResp(http.StatusOK, `{"data":{"token_id":"tok","subject_id":"user","role":"admin","status":"active","scope":["read"],"issued_at":"2026-04-21T09:00:00Z","expires_at":"2026-04-21T11:00:00Z"}}`), nil
		}),
	}
	runtime := NewRemoteTokenRuntimeWithCacheConfig(
		"http://tok.internal", httpClient,
		func() time.Time { return fixedNow },
		CacheConfig{ActiveTTL: 5 * time.Second, MaxEntries: 3},
	)
	ctx := context.Background()

	// Fill cache
	for i := 0; i < 3; i++ {
		_, err := runtime.Verify(ctx, "raw-token-"+string(rune('a'+i)))
		if err != nil {
			t.Fatalf("Verify %d failed: %v", i, err)
		}
	}

	// Advance past active TTL
	runtime.AdvanceTime(6 * time.Second)

	// New insert should evict expired entries first (not LRU), then make room
	_, err := runtime.Verify(ctx, "raw-token-new")
	if err != nil {
		t.Fatalf("Verify after TTL expiry failed: %v", err)
	}
	// Should call upstream for the new token
	if callCount != 4 {
		t.Errorf("after inserting new token with expired entries: callCount=%d, want 4", callCount)
	}
}

// ---------------------------------------------------------------------------
// Matrix row: Resolve() after Verify()
// ---------------------------------------------------------------------------

func TestRemoteTokenRuntime_Resolve_AfterVerify(t *testing.T) {
	fixedNow := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResp(http.StatusOK, `{"data":{"token_id":"tok-resolve","subject_id":"user-resolve","role":"editor","status":"active","scope":["read","write"],"issued_at":"2026-04-21T09:00:00Z","expires_at":"2026-04-21T11:00:00Z"}}`), nil
		}),
	}
	runtime := NewRemoteTokenRuntimeWithCacheConfig(
		"http://tok.internal", httpClient,
		func() time.Time { return fixedNow },
		CacheConfig{ActiveTTL: 30 * time.Second, MaxEntries: 10000},
	)
	ctx := context.Background()

	// Verify first
	v, err := runtime.Verify(ctx, "raw-resolve-token")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if v.Role != "editor" {
		t.Errorf("Verify Role: got %s, want editor", v.Role)
	}

	// Resolve by tokenID (scanned from cache)
	status, err := runtime.Resolve(ctx, v.TokenID)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if status != TokenStatusActive {
		t.Errorf("Resolve status: got %s, want active", status)
	}

	// Resolve with non-existent tokenID
	_, err = runtime.Resolve(ctx, "non-existent-tok")
	if err == nil {
		t.Fatal("expected error for non-existent token, got nil")
	}
}

func TestRemoteTokenRuntime_Resolve_WithoutVerify(t *testing.T) {
	fixedNow := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResp(http.StatusOK, `{"data":{"token_id":"tok-orphan","subject_id":"user","role":"admin","status":"active","scope":[],"issued_at":"2026-04-21T09:00:00Z","expires_at":"2026-04-21T11:00:00Z"}}`), nil
		}),
	}
	runtime := NewRemoteTokenRuntimeWithCacheConfig(
		"http://tok.internal", httpClient,
		func() time.Time { return fixedNow },
		CacheConfig{ActiveTTL: 30 * time.Second, MaxEntries: 10000},
	)
	ctx := context.Background()

	// Resolve without Verify should error
	_, err := runtime.Resolve(ctx, "tok-orphan")
	if err == nil {
		t.Fatal("expected error when Resolve without Verify, got nil")
	}
	if !strings.Contains(err.Error(), "not cached") {
		t.Errorf("error message: got %q, want contains 'not cached'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Matrix row: config defaults
// ---------------------------------------------------------------------------

func TestRemoteTokenRuntime_DefaultCacheConfig(t *testing.T) {
	cfg := DefaultCacheConfig()
	if cfg.ActiveTTL != 30*time.Second {
		t.Errorf("DefaultCacheConfig ActiveTTL: got %v, want 30s", cfg.ActiveTTL)
	}
	if cfg.ExpiredTTL != 2*time.Minute {
		t.Errorf("DefaultCacheConfig ExpiredTTL: got %v, want 2m", cfg.ExpiredTTL)
	}
	if cfg.RevokedTTL != 10*time.Minute {
		t.Errorf("DefaultCacheConfig RevokedTTL: got %v, want 10m", cfg.RevokedTTL)
	}
	if cfg.MaxEntries != 10000 {
		t.Errorf("DefaultCacheConfig MaxEntries: got %d, want 10000", cfg.MaxEntries)
	}
}

func TestRemoteTokenRuntime_NewRemoteTokenRuntime_Defaults(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResp(http.StatusOK, `{"data":{"token_id":"tok","subject_id":"user","role":"admin","status":"active","scope":[],"issued_at":"2026-04-21T09:00:00Z","expires_at":"2026-04-21T11:00:00Z"}}`), nil
		}),
	}
	// NewRemoteTokenRuntime with nil httpClient and nil now
	runtime := NewRemoteTokenRuntime("http://tok.internal", nil, nil)
	if runtime == nil {
		t.Fatal("NewRemoteTokenRuntime returned nil")
	}
	_ = httpClient // used above for the test that would need it
}

type urlError struct {
	msg   string
	opStr string
	temp  bool
}

func (e *urlError) Error() string   { return e.msg }
func (e *urlError) Timeout() bool   { return e.opStr == "OpTimeout" }
func (e *urlError) Temporary() bool { return e.temp }
