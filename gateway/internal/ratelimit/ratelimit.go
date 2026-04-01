package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"lijiaoqiao/gateway/pkg/error"
)

// Algorithm 限流算法
type Algorithm string

const (
	TokenBucket  Algorithm = "token_bucket"
	SlidingWindow Algorithm = "sliding_window"
	FixedWindow  Algorithm = "fixed_window"
)

// Limiter 限流器接口
type Limiter interface {
	// Allow 检查是否允许请求
	Allow(ctx context.Context, key string) (bool, error)
	// AllowToken 检查是否允许消耗token
	AllowToken(ctx context.Context, key string, tokens int) (bool, error)
	// GetLimit 获取当前限制
	GetLimit(key string) *Limit
}

// Limit 限制配置
type Limit struct {
	RPM       int     // 请求数/分钟
	TPM       int     // Token数/分钟
	Burst     int     // 突发容量
	Remaining int     // 剩余请求数
	ResetAt   time.Time // 重置时间
}

// TokenBucketLimiter Token桶限流器
type TokenBucketLimiter struct {
	mu        sync.RWMutex
	buckets   map[string]*tokenBucket
	defaultRPM int
	defaultTPM int
	burstMultiplier float64
	cleanInterval time.Duration
}

type tokenBucket struct {
	tokens        float64
	maxTokens     float64
	tokensPerSec  float64
	lastRefill    time.Time
	mu            sync.Mutex
}

// NewTokenBucketLimiter 创建Token桶限流器
func NewTokenBucketLimiter(defaultRPM, defaultTPM int, burstMultiplier float64) *TokenBucketLimiter {
	limiter := &TokenBucketLimiter{
		buckets:        make(map[string]*tokenBucket),
		defaultRPM:     defaultRPM,
		defaultTPM:     defaultTPM,
		burstMultiplier: burstMultiplier,
		cleanInterval:  5 * time.Minute,
	}

	// 启动清理goroutine
	go limiter.cleanup()

	return limiter
}

// Allow 检查是否允许请求
func (l *TokenBucketLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return l.AllowToken(ctx, key, 1)
}

// AllowToken 检查是否允许消耗token
func (l *TokenBucketLimiter) AllowToken(ctx context.Context, key string, tokens int) (bool, error) {
	l.mu.Lock()
	bucket, exists := l.buckets[key]
	if !exists {
		bucket = l.newBucket(l.defaultRPM, l.defaultTPM)
		l.buckets[key] = bucket
	}
	l.mu.Unlock()

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// 补充token
	l.refill(bucket)

	// 检查是否有足够的token
	if bucket.tokens >= float64(tokens) {
		bucket.tokens -= float64(tokens)
		return true, nil
	}

	return false, nil
}

// GetLimit 获取当前限制
func (l *TokenBucketLimiter) GetLimit(key string) *Limit {
	l.mu.RLock()
	bucket, exists := l.buckets[key]
	l.mu.RUnlock()

	if !exists {
		return &Limit{
			RPM:   l.defaultRPM,
			TPM:   l.defaultTPM,
			Burst: int(float64(l.defaultRPM) * l.burstMultiplier),
		}
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	return &Limit{
		RPM:       l.defaultRPM,
		TPM:       l.defaultTPM,
		Burst:     int(bucket.maxTokens),
		Remaining: int(bucket.tokens),
		ResetAt:   bucket.lastRefill.Add(time.Minute),
	}
}

func (l *TokenBucketLimiter) newBucket(rpm, tpm int) *tokenBucket {
	burst := int(float64(rpm) * l.burstMultiplier)
	return &tokenBucket{
		tokens:        float64(burst),
		maxTokens:     float64(burst),
		tokensPerSec:  float64(rpm) / 60.0,
		lastRefill:    time.Now(),
	}
}

func (l *TokenBucketLimiter) refill(bucket *tokenBucket) {
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()

	// 添加新token
	bucket.tokens += elapsed * bucket.tokensPerSec
	if bucket.tokens > bucket.maxTokens {
		bucket.tokens = bucket.maxTokens
	}

	bucket.lastRefill = now
}

func (l *TokenBucketLimiter) cleanup() {
	ticker := time.NewTicker(l.cleanInterval)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for key, bucket := range l.buckets {
			bucket.mu.Lock()
			// 如果bucket完全空了且超过10分钟没使用，删除它
			if bucket.tokens >= bucket.maxTokens && now.Sub(bucket.lastRefill) > 10*time.Minute {
				delete(l.buckets, key)
			}
			bucket.mu.Unlock()
		}
		l.mu.Unlock()
	}
}

// SlidingWindowLimiter 滑动窗口限流器
type SlidingWindowLimiter struct {
	mu        sync.RWMutex
	windows   map[string]*slidingWindow
	windowSize time.Duration
	maxRequests int
	cleanInterval time.Duration
}

type slidingWindow struct {
	requests []time.Time
	mu       sync.Mutex
}

func NewSlidingWindowLimiter(windowSize time.Duration, maxRequests int) *SlidingWindowLimiter {
	limiter := &SlidingWindowLimiter{
		windows:      make(map[string]*slidingWindow),
		windowSize:   windowSize,
		maxRequests:  maxRequests,
		cleanInterval: 1 * time.Minute,
	}

	go limiter.cleanup()
	return limiter
}

func (l *SlidingWindowLimiter) Allow(ctx context.Context, key string) (bool, error) {
	l.mu.Lock()
	window, exists := l.windows[key]
	if !exists {
		window = &slidingWindow{requests: make([]time.Time, 0)}
		l.windows[key] = window
	}
	l.mu.Unlock()

	window.mu.Lock()
	defer window.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.windowSize)

	// 清理过期的请求
	validRequests := make([]time.Time, 0)
	for _, t := range window.requests {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}
	window.requests = validRequests

	// 检查是否超过限制
	if len(window.requests) >= l.maxRequests {
		return false, nil
	}

	window.requests = append(window.requests, now)
	return true, nil
}

func (l *SlidingWindowLimiter) AllowToken(ctx context.Context, key string, tokens int) (bool, error) {
	// 对于滑动窗口，tokens只是计数，这里简化为1个请求
	return l.Allow(ctx, key)
}

func (l *SlidingWindowLimiter) GetLimit(key string) *Limit {
	l.mu.RLock()
	window, exists := l.windows[key]
	l.mu.RUnlock()

	remaining := l.maxRequests
	if exists {
		window.mu.Lock()
		cutoff := time.Now().Add(-l.windowSize)
		count := 0
		for _, t := range window.requests {
			if t.After(cutoff) {
				count++
			}
		}
		remaining = l.maxRequests - count
		if remaining < 0 {
			remaining = 0
		}
		window.mu.Unlock()
	}

	return &Limit{
		RPM:   l.maxRequests,
		ResetAt: time.Now().Add(l.windowSize),
		Remaining: remaining,
	}
}

func (l *SlidingWindowLimiter) cleanup() {
	ticker := time.NewTicker(l.cleanInterval)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for key, window := range l.windows {
			window.mu.Lock()
			cutoff := now.Add(-l.windowSize * 2)
			validRequests := make([]time.Time, 0)
			for _, t := range window.requests {
				if t.After(cutoff) {
					validRequests = append(validRequests, t)
				}
			}
			if len(validRequests) == 0 && now.Sub(window.requests[len(window.requests)-1]) > l.windowSize*2 {
				delete(l.windows, key)
			} else {
				window.requests = validRequests
			}
			window.mu.Unlock()
		}
		l.mu.Unlock()
	}
}

// Middleware 限流中间件
type Middleware struct {
	limiter Limiter
}

func NewMiddleware(limiter Limiter) *Middleware {
	return &Middleware{limiter: limiter}
}

func (m *Middleware) Limit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 使用API Key作为限流key
		key := r.Header.Get("Authorization")
		if key == "" {
			key = r.RemoteAddr
		}

		allowed, err := m.limiter.Allow(r.Context(), key)
		if err != nil {
			writeError(w, error.NewGatewayError(error.COMMON_INTERNAL_ERROR, "rate limiter error"))
			return
		}

		if !allowed {
			limit := m.limiter.GetLimit(key)
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit.RPM))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", limit.Remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", limit.ResetAt.Unix()))

			writeError(w, error.NewGatewayError(error.RATE_LIMIT_EXCEEDED, "rate limit exceeded"))
			return
		}

		next.ServeHTTP(w, r)
	}
}

import "net/http"

func writeError(w http.ResponseWriter, err *error.GatewayError) {
	info := err.GetErrorInfo()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(info.HTTPStatus)
	w.Write([]byte(fmt.Sprintf(`{"error":{"message":"%s","code":"%s"}}`, err.Message, err.Code)))
}
