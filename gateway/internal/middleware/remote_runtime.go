package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"lijiaoqiao/gateway/internal/metrics"
)

// P3-A: Cache TTL and eviction implementation
// - active=30s, expired=2m, revoked=10m
// - max_entries=10000
// - evict expired first, then oldest by insert order

// CacheConfig controls cache TTL and eviction behavior
type CacheConfig struct {
	ActiveTTL  time.Duration // default 30s
	ExpiredTTL time.Duration // default 2m
	RevokedTTL time.Duration // default 10m
	MaxEntries int           // default 10000
}

// DefaultCacheConfig returns safe defaults per P3-A design
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		ActiveTTL:  30 * time.Second,
		ExpiredTTL: 2 * time.Minute,
		RevokedTTL: 10 * time.Minute,
		MaxEntries: 10000,
	}
}

// cachedTokenEntry holds a cached token with TTL metadata
type cachedTokenEntry struct {
	token    remoteResolvedToken
	cachedAt time.Time // when this entry was cached
}

// RemoteTokenRuntime introspects tokens via HTTP and caches results
type RemoteTokenRuntime struct {
	baseURL    string
	httpClient *http.Client
	now        func() time.Time
	cacheCfg   CacheConfig

	mu      sync.RWMutex
	records map[string]cachedTokenEntry
	// evictionOrder tracks insertion order for LRU-like eviction
	evictionOrder []string

	// Stats for testing
	cacheHits     int64
	upstreamCalls int64
}

// AdvanceTime advances the internal clock for deterministic TTL testing
func (r *RemoteTokenRuntime) AdvanceTime(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Advance the now function but NOT cachedAt - this makes now()-cachedAt grow
	current := r.now()
	r.now = func() time.Time {
		return current.Add(d)
	}
	// Note: do NOT update cachedAt - TTL check uses now()-cachedAt
}

// NewRemoteTokenRuntime creates a runtime with default cache config
func NewRemoteTokenRuntime(baseURL string, httpClient *http.Client, now func() time.Time) *RemoteTokenRuntime {
	return NewRemoteTokenRuntimeWithCacheConfig(baseURL, httpClient, now, DefaultCacheConfig())
}

// NewRemoteTokenRuntimeWithCacheConfig creates a runtime with custom cache config
func NewRemoteTokenRuntimeWithCacheConfig(baseURL string, httpClient *http.Client, now func() time.Time, cacheCfg CacheConfig) *RemoteTokenRuntime {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if now == nil {
		now = time.Now
	}
	if cacheCfg.ActiveTTL == 0 {
		cacheCfg.ActiveTTL = 30 * time.Second
	}
	if cacheCfg.ExpiredTTL == 0 {
		cacheCfg.ExpiredTTL = 2 * time.Minute
	}
	if cacheCfg.RevokedTTL == 0 {
		cacheCfg.RevokedTTL = 10 * time.Minute
	}
	if cacheCfg.MaxEntries == 0 {
		cacheCfg.MaxEntries = 10000
	}
	return &RemoteTokenRuntime{
		baseURL:       strings.TrimRight(baseURL, "/"),
		httpClient:    httpClient,
		now:           now,
		cacheCfg:      cacheCfg,
		records:       make(map[string]cachedTokenEntry),
		evictionOrder: make([]string, 0, cacheCfg.MaxEntries),
	}
}

// CacheStats holds cache hit/miss/eviction counters
type CacheStats struct {
	Cached   int64 // total cache hits
	Upstream int64 // total upstream calls
	Evicted  int64 // total evicted entries
}

// GetCacheStats returns cache statistics for testing
func (r *RemoteTokenRuntime) GetCacheStats() CacheStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return CacheStats{
		Cached:   r.cacheHits,
		Upstream: r.upstreamCalls,
		Evicted:  0, // Could track if needed
	}
}

// ttlForStatus returns the TTL for a given token status
func (r *RemoteTokenRuntime) ttlForStatus(status TokenStatus) time.Duration {
	switch status {
	case TokenStatusActive:
		return r.cacheCfg.ActiveTTL
	case TokenStatusExpired:
		return r.cacheCfg.ExpiredTTL
	case TokenStatusRevoked:
		return r.cacheCfg.RevokedTTL
	default:
		return r.cacheCfg.ActiveTTL
	}
}

// isEntryExpired checks if a cached entry has exceeded its TTL
func (r *RemoteTokenRuntime) isEntryExpired(entry cachedTokenEntry) bool {
	ttl := r.ttlForStatus(entry.token.status)
	return r.now().After(entry.cachedAt.Add(ttl))
}

// evictExpired removes all expired entries from the cache
func (r *RemoteTokenRuntime) evictExpired() int {
	evicted := 0
	for key, entry := range r.records {
		if r.isEntryExpired(entry) {
			delete(r.records, key)
			evicted++
			metrics.RecordCacheEviction()
		}
	}
	// Compact evictionOrder
	if evicted > 0 {
		newOrder := make([]string, 0, len(r.records))
		for _, key := range r.evictionOrder {
			if _, ok := r.records[key]; ok {
				newOrder = append(newOrder, key)
			}
		}
		r.evictionOrder = newOrder
	}
	return evicted
}

// evictToMakeRoom ensures at least one slot is available, preferring expired then oldest
func (r *RemoteTokenRuntime) evictToMakeRoom() {
	if len(r.records) < r.cacheCfg.MaxEntries {
		return
	}

	// First, try to evict expired entries
	if r.evictExpired() > 0 {
		if len(r.records) < r.cacheCfg.MaxEntries {
			return
		}
	}

	// If still full, evict oldest entries (FIFO)
	toEvict := len(r.records) - r.cacheCfg.MaxEntries + 1
	if toEvict > 0 && len(r.evictionOrder) > 0 {
		for i := 0; i < toEvict && len(r.evictionOrder) > 0; i++ {
			key := r.evictionOrder[0]
			r.evictionOrder = r.evictionOrder[1:]
			delete(r.records, key)
			metrics.RecordCacheEviction()
		}
	}
}

type remoteResolvedToken struct {
	verified  VerifiedToken
	status    TokenStatus
	expiresAt time.Time
}

type remoteIntrospectResponse struct {
	Data struct {
		TokenID   string    `json:"token_id"`
		SubjectID string    `json:"subject_id"`
		Role      string    `json:"role"`
		Status    string    `json:"status"`
		Scope     []string  `json:"scope"`
		IssuedAt  time.Time `json:"issued_at"`
		ExpiresAt time.Time `json:"expires_at"`
	} `json:"data"`
}

// Verify implements TokenVerifier by calling upstream and caching the result
func (r *RemoteTokenRuntime) Verify(ctx context.Context, rawToken string) (VerifiedToken, error) {
	// Check cache first using rawToken as key
	cached, ok := r.checkCache(rawToken)
	if ok {
		metrics.RecordCacheHit()
		r.mu.Lock()
		r.cacheHits++
		r.mu.Unlock()
		return cached, nil
	}
	metrics.RecordCacheMiss()
	r.mu.Lock()
	r.upstreamCalls++
	r.mu.Unlock()

	// Call upstream
	payload := map[string]string{"token": rawToken}
	body, err := json.Marshal(payload)
	if err != nil {
		return VerifiedToken{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/api/v1/platform/tokens/introspect", bytes.NewReader(body))
	if err != nil {
		return VerifiedToken{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	// P3-C-08: 从请求上下文透传 trace ID，避免生成新的 ID 截断链路
	if reqID, ok := RequestIDFromContext(ctx); ok && reqID != "" {
		req.Header.Set("X-Request-Id", reqID)
	} else {
		req.Header.Set("X-Request-Id", fmt.Sprintf("gateway-introspect-%d", r.now().UnixNano()))
	}

	start := time.Now()
	resp, err := r.httpClient.Do(req)
	latencyNs := time.Since(start).Nanoseconds()
	metrics.RecordTokenRuntime(latencyNs, err != nil)

	if err != nil {
		return VerifiedToken{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return VerifiedToken{}, fmt.Errorf("token introspection failed with status %d", resp.StatusCode)
	}

	var result remoteIntrospectResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return VerifiedToken{}, err
	}
	if strings.TrimSpace(result.Data.TokenID) == "" {
		return VerifiedToken{}, errors.New("token introspection response missing token_id")
	}

	status := TokenStatus(strings.ToLower(strings.TrimSpace(result.Data.Status)))
	verified := VerifiedToken{
		TokenID:   result.Data.TokenID,
		SubjectID: result.Data.SubjectID,
		Role:      result.Data.Role,
		Scope:     append([]string(nil), result.Data.Scope...),
		IssuedAt:  result.Data.IssuedAt,
		ExpiresAt: result.Data.ExpiresAt,
	}

	// Cache the result
	r.putCache(rawToken, verified, status, result.Data.ExpiresAt)

	return verified, nil
}

// Resolve implements TokenStatusResolver using cached state
func (r *RemoteTokenRuntime) Resolve(ctx context.Context, tokenID string) (TokenStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, entry := range r.records {
		if entry.token.verified.TokenID == tokenID {
			if r.isEntryExpired(entry) {
				return "", errors.New("token status cache expired; verify must run before resolve")
			}
			return entry.token.status, nil
		}
	}
	return "", errors.New("token status not cached; verify must run before resolve")
}

// checkCache looks up a token in the cache by rawToken
func (r *RemoteTokenRuntime) checkCache(rawToken string) (VerifiedToken, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.records[rawToken]
	if !ok {
		return VerifiedToken{}, false
	}
	if r.isEntryExpired(entry) {
		return VerifiedToken{}, false
	}
	return entry.token.verified, true
}

// putCache stores a token in the cache by rawToken
func (r *RemoteTokenRuntime) putCache(rawToken string, verified VerifiedToken, status TokenStatus, expiresAt time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Evict expired entries first to make room
	r.evictExpired_Locked()

	// Evict oldest if at capacity
	r.evictToMakeRoom_Locked()

	entry := cachedTokenEntry{
		token: remoteResolvedToken{
			verified:  verified,
			status:    status,
			expiresAt: expiresAt,
		},
		cachedAt: r.now(),
	}
	r.records[rawToken] = entry
	r.evictionOrder = append(r.evictionOrder, rawToken)
}

// evictExpired_Locked must be called with mu held
func (r *RemoteTokenRuntime) evictExpired_Locked() int {
	evicted := 0
	for key, entry := range r.records {
		if r.isEntryExpired(entry) {
			delete(r.records, key)
			evicted++
			metrics.RecordCacheEviction()
		}
	}
	if evicted > 0 {
		newOrder := make([]string, 0, len(r.records))
		for _, key := range r.evictionOrder {
			if _, ok := r.records[key]; ok {
				newOrder = append(newOrder, key)
			}
		}
		r.evictionOrder = newOrder
	}
	return evicted
}

// evictToMakeRoom_Locked must be called with mu held
func (r *RemoteTokenRuntime) evictToMakeRoom_Locked() {
	if len(r.records) < r.cacheCfg.MaxEntries {
		return
	}

	// Try expired eviction first
	if r.evictExpired_Locked() > 0 && len(r.records) < r.cacheCfg.MaxEntries {
		return
	}

	// FIFO eviction
	toEvict := len(r.records) - r.cacheCfg.MaxEntries + 1
	for i := 0; i < toEvict && len(r.evictionOrder) > 0; i++ {
		key := r.evictionOrder[0]
		r.evictionOrder = r.evictionOrder[1:]
		delete(r.records, key)
		metrics.RecordCacheEviction()
	}
}
