package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// ==================== P0-05 限流策略实现 ====================

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled    bool          // 是否启用
	Requests   int           // 窗口内最大请求数
	Window     time.Duration // 时间窗口
	HeaderType string        // 响应头类型: "X-RateLimit" | "RateLimit-Policy"

	// 多维度限流
	LimitByTenant   bool // 按租户限流
	LimitByUser     bool // 按用户限流
	LimitByIP       bool // 按IP限流
	LimitByEndpoint bool // 按端点限流

	// 降级策略
	DegradationEnabled bool        // 是否启用降级
	DegradationHandler http.Handler // 降级处理器
	FallbackCode       int         // 降级时返回的HTTP状态码
}

// DefaultRateLimitConfig 默认限流配置
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		Enabled:    true,
		Requests:   1000,
		Window:     time.Minute,
		HeaderType: "X-RateLimit",

		LimitByTenant:   true,
		LimitByUser:     true,
		LimitByIP:       true,
		LimitByEndpoint: true,

		DegradationEnabled: true,
		FallbackCode:       http.StatusTooManyRequests,
	}
}

// TokenBucket 令牌桶
type TokenBucket struct {
	mu         sync.Mutex
	capacity   int        // 桶容量
	rate       int        // 每秒补充的令牌数
	tokens     int        // 当前令牌数
	lastRefill time.Time  // 上次补充时间
}

// NewTokenBucket 创建令牌桶
func NewTokenBucket(capacity int, rate int) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		rate:       rate,
		tokens:     capacity,
		lastRefill: time.Now(),
	}
}

// Allow 检查是否允许请求
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// 补充令牌
	tb.refill()

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	return false
}

// refill 补充令牌
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	// 计算应该补充的令牌数（使用float64精确计算）
	tokensToAdd := int(elapsed.Seconds()*float64(tb.rate)) + tb.tokens

	if tokensToAdd > tb.capacity {
		tb.tokens = tb.capacity
	} else {
		tb.tokens = tokensToAdd
	}
	tb.lastRefill = now
}

// Remaining 返回剩余令牌数
func (tb *TokenBucket) Remaining() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	return tb.tokens
}

// Reset 重置令牌桶
func (tb *TokenBucket) Reset() {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.tokens = tb.capacity
	tb.lastRefill = time.Now()
}

// RateLimitMiddleware 限流中间件
type RateLimitMiddleware struct {
	config  *RateLimitConfig
	buckets map[string]*TokenBucket // 限流桶
	mu      sync.RWMutex
	next    http.Handler
}

// ServeHTTP 实现http.Handler
func (rl *RateLimitMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !rl.config.Enabled {
		rl.next.ServeHTTP(w, r)
		return
	}

	// 生成限流key
	key := rl.getRateLimitKey(r)

	// 获取或创建限流桶
	bucket := rl.getBucket(key)

	// 检查是否允许
	if !bucket.Allow() {
		rl.handleRateLimitExceeded(w, r, bucket)
		return
	}

	// 设置响应头
	rl.setRateLimitHeaders(w, bucket)

	rl.next.ServeHTTP(w, r)
}

// getRateLimitKey 获取限流key
func (rl *RateLimitMiddleware) getRateLimitKey(r *http.Request) string {
	var keyParts []string

	if rl.config.LimitByTenant {
		keyParts = append(keyParts, fmt.Sprintf("tenant:%d", getTenantIDFromRequest(r)))
	}

	if rl.config.LimitByUser {
		keyParts = append(keyParts, fmt.Sprintf("user:%s", getUserIDFromRequest(r)))
	}

	if rl.config.LimitByIP {
		keyParts = append(keyParts, fmt.Sprintf("ip:%s", getClientIP(r)))
	}

	if rl.config.LimitByEndpoint {
		keyParts = append(keyParts, r.URL.Path)
	}

	if len(keyParts) == 0 {
		return "default"
	}

	result := keyParts[0]
	for i := 1; i < len(keyParts); i++ {
		result = fmt.Sprintf("%s:%s", result, keyParts[i])
	}
	return result
}

// getBucket 获取限流桶
func (rl *RateLimitMiddleware) getBucket(key string) *TokenBucket {
	rl.mu.RLock()
	bucket, exists := rl.buckets[key]
	rl.mu.RUnlock()

	if exists {
		return bucket
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// 双重检查
	if bucket, exists = rl.buckets[key]; exists {
		return bucket
	}

	bucket = NewTokenBucket(rl.config.Requests, 0) // 固定容量，不自动补充
	rl.buckets[key] = bucket

	return bucket
}

// handleRateLimitExceeded 处理限流超出
func (rl *RateLimitMiddleware) handleRateLimitExceeded(w http.ResponseWriter, r *http.Request, bucket *TokenBucket) {
	// 设置重试响应头
	resetTime := time.Now().Add(rl.config.Window)
	w.Header().Set("Retry-After", strconv.Itoa(int(rl.config.Window.Seconds())))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))

	if rl.config.DegradationEnabled && rl.config.DegradationHandler != nil {
		rl.config.DegradationHandler.ServeHTTP(w, r)
		return
	}

	http.Error(w, "rate limit exceeded", rl.config.FallbackCode)
}

// setRateLimitHeaders 设置限流响应头
func (rl *RateLimitMiddleware) setRateLimitHeaders(w http.ResponseWriter, bucket *TokenBucket) {
	prefix := rl.config.HeaderType

	w.Header().Set(prefix+"-Limit", strconv.Itoa(rl.config.Requests))
	w.Header().Set(prefix+"-Remaining", strconv.Itoa(bucket.Remaining()))

	resetTime := time.Now().Add(rl.config.Window).Unix()
	w.Header().Set(prefix+"-Reset", strconv.FormatInt(resetTime, 10))
}

// Helper functions

// getTenantIDFromRequest 从请求获取租户ID
func getTenantIDFromRequest(r *http.Request) int64 {
	// 从JWT claims获取租户ID
	if claims := GetTokenClaims(r.Context()); claims != nil {
		return claims.TenantID
	}
	return 0
}

// getUserIDFromRequest 从请求获取用户ID
func getUserIDFromRequest(r *http.Request) string {
	// 从JWT claims获取用户ID
	if claims := GetTokenClaims(r.Context()); claims != nil {
		return claims.SubjectID
	}
	return "unknown"
}

// NewRateLimitHandler 创建限流中间件包装器
// 用于简化在中间件链路中的使用
func NewRateLimitHandler(config *RateLimitConfig, next http.Handler) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		config:  config,
		buckets: make(map[string]*TokenBucket),
		next:    next,
	}
}
