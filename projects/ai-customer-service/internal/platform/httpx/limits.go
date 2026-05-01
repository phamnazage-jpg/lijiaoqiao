package httpx

import (
	"net/http"
	"sync"
	"time"
)

// WithBodyLimit wraps the next handler, enforcing a maximum request body size.
func WithBodyLimit(next http.Handler, limit int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}

// RateLimiter implements a per-key (IP or channel) sliding-window rate limiter.
// It does NOT block the main flow — on exceed it writes 429 and returns,
// but does not propagate an error.
type RateLimiter struct {
	mu       sync.RWMutex
	counters map[string]*slidingWindow
	window   time.Duration
	limit    int
}

type slidingWindow struct {
	mu     sync.Mutex
	tokens []time.Time
}

// NewRateLimiter creates a rate limiter that allows max `limit` requests
// per `window` duration per key.
func NewRateLimiter(window time.Duration, limit int) *RateLimiter {
	if limit <= 0 {
		limit = 10
	}
	if window <= 0 {
		window = time.Second
	}
	return &RateLimiter{
		counters: make(map[string]*slidingWindow),
		window:   window,
		limit:    limit,
	}
}

// Allow returns true if the request for the given key is within the rate limit,
// false if it should be rejected with 429.
func (rl *RateLimiter) Allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-rl.window)

	// P0-1 fix: use write lock for GetOrCreate to avoid data race on map write
	rl.mu.Lock()
	sw, exists := rl.counters[key]
	if !exists {
		rl.counters[key] = &slidingWindow{tokens: make([]time.Time, 0, rl.limit)}
		sw = rl.counters[key]
	}
	rl.mu.Unlock()

	sw.mu.Lock()
	defer sw.mu.Unlock()

	// Remove expired tokens
	var valid []time.Time
	for _, t := range sw.tokens {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	sw.tokens = valid

	if len(sw.tokens) >= rl.limit {
		return false
	}
	sw.tokens = append(sw.tokens, now)
	return true
}

// WithRateLimit wraps the next handler with per-key rate limiting.
// The key is extracted from X-Forwarded-For or r.RemoteAddr.
// Exceeding the limit returns HTTP 429 without propagating an error.
func (rl *RateLimiter) WithRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := rateLimitKey(r)
		if !rl.Allow(key) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":"CS_SES_4002","message":"message rate limit exceeded"}}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimitKey extracts a stable key for rate limiting.
// It prefers X-Forwarded-For (first IP) over RemoteAddr.
func rateLimitKey(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		for i := 0; i < len(fwd); i++ {
			if fwd[i] == ',' {
				return fwd[:i]
			}
		}
		return fwd
	}
	// Strip port from RemoteAddr
	addr := r.RemoteAddr
	if idx := lastIndexByte(addr, ':'); idx > 0 {
		return addr[:idx]
	}
	return addr
}

func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}
