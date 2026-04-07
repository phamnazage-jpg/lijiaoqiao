package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestP005_TokenBucketAlgorithm 验证令牌桶算法
func TestP005_TokenBucketAlgorithm(t *testing.T) {
	bucket := NewTokenBucket(10, 1) // 容量10，补充速率1/秒

	// 初始有10个令牌
	if !bucket.Allow() {
		t.Error("first 10 requests should be allowed")
	}

	// 消耗完令牌
	for i := 0; i < 9; i++ {
		bucket.Allow()
	}

	// 第11个请求应该被拒绝（没有令牌）
	if bucket.Allow() {
		t.Error("request beyond capacity should be denied")
	}

	t.Log("P0-05: 令牌桶容量验证通过")
}

// TestP005_TokenBucketRefill 验证令牌补充
func TestP005_TokenBucketRefill(t *testing.T) {
	bucket := NewTokenBucket(5, 100) // 容量5，补充速率100/秒

	// 消耗所有令牌
	for i := 0; i < 5; i++ {
		bucket.Allow()
	}

	// 应该没有令牌了
	if bucket.Allow() {
		t.Error("bucket should be empty")
	}

	// 等待20ms，应该补充2个令牌 (100/秒 = 1/10ms, 20ms = 2)
	time.Sleep(20 * time.Millisecond)

	if !bucket.Allow() {
		t.Error("after refill, request should be allowed")
	}

	t.Log("P0-05: 令牌补充验证通过")
}

// TestP005_RateLimitHeaders 验证限流响应头
func TestP005_RateLimitHeaders(t *testing.T) {
	config := RateLimitConfig{
		Enabled:    true,
		Requests:   100,
		Window:     time.Minute,
		HeaderType: "X-RateLimit",
	}

	handler := &RateLimitMiddleware{
		config:  &config,
		buckets: make(map[string]*TokenBucket),
		next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok"))
		}),
	}

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// 验证响应头存在
	if w.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("X-RateLimit-Limit header should be present")
	}
	if w.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("X-RateLimit-Remaining header should be present")
	}
	if w.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("X-RateLimit-Reset header should be present")
	}

	t.Log("P0-05: 限流响应头验证通过")
}

// TestP005_MultiDimensionRateLimit 验证多维度限流
func TestP005_MultiDimensionRateLimit(t *testing.T) {
	// 模拟多租户限流
	tenantLimits := map[string]*TokenBucket{
		"tenant_1": NewTokenBucket(100, 10),
		"tenant_2": NewTokenBucket(50, 5),
	}

	// tenant_1 100个请求应该通过
	for i := 0; i < 100; i++ {
		if !tenantLimits["tenant_1"].Allow() {
			t.Errorf("tenant_1 request %d should be allowed", i)
		}
	}

	// tenant_1 第101个应该拒绝
	if tenantLimits["tenant_1"].Allow() {
		t.Error("tenant_1 exceeded limit")
	}

	// tenant_2 50个请求应该通过
	for i := 0; i < 50; i++ {
		if !tenantLimits["tenant_2"].Allow() {
			t.Errorf("tenant_2 request %d should be allowed", i)
		}
	}

	// tenant_2 第51个应该拒绝
	if tenantLimits["tenant_2"].Allow() {
		t.Error("tenant_2 exceeded limit")
	}

	t.Log("P0-05: 多维度限流验证通过")
}

// TestP005_RateLimitConcurrency 验证并发安全性
func TestP005_RateLimitConcurrency(t *testing.T) {
	bucket := NewTokenBucket(100, 0) // 容量100，不补充

	var allowed int
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 200个并发请求
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if bucket.Allow() {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// 应该只有100个通过
	if allowed != 100 {
		t.Errorf("expected 100 allowed, got %d", allowed)
	}

	t.Log("P0-05: 并发安全性验证通过")
}

// TestP005_DegradationOnLimitExceeded 验证限流后降级
func TestP005_DegradationOnLimitExceeded(t *testing.T) {
	config := RateLimitConfig{
		Enabled:            true,
		Requests:           10,
		Window:             time.Second,
		DegradationEnabled: true,
		DegradationHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("rate limited"))
		}),
	}

	handler := &RateLimitMiddleware{
		config:  &config,
		buckets: make(map[string]*TokenBucket),
		next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok"))
		}),
	}

	// 消耗所有令牌
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	// 第11个请求应该返回限流响应
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}

	t.Log("P0-05: 限流降级验证通过")
}

// TestP005_RateLimitConfig 验证限流配置
func TestP005_RateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()

	if !config.Enabled {
		t.Error("rate limit should be enabled by default")
	}

	if config.Requests != 1000 {
		t.Errorf("expected default requests 1000, got %d", config.Requests)
	}

	if config.Window != time.Minute {
		t.Errorf("expected default window 1 minute, got %v", config.Window)
	}

	t.Log("P0-05: 限流配置验证通过")
}

// TestP005_Summary 测试总结
func TestP005_Summary(t *testing.T) {
	t.Log("=== P0-05 限流策略测试总结 ===")
	t.Log("问题: PRD P0要求基础限流策略，但所有技术文档均未定义限流算法")
	t.Log("")
	t.Log("修复方案:")
	t.Log("  - 令牌桶算法 (Token Bucket)")
	t.Log("  - 多维度限流 (tenant/user/IP/endpoint)")
	t.Log("  - 滑动窗口 (Sliding Window)")
	t.Log("  - 降级策略 (返回429或队列)")
}

// ==================== TokenBucket Reset Tests ====================

func TestTokenBucket_Reset(t *testing.T) {
	bucket := NewTokenBucket(5, 1) // capacity 5, refill 1/second

	// Consume some tokens
	for i := 0; i < 3; i++ {
		bucket.Allow()
	}

	// Verify tokens consumed
	if bucket.Remaining() != 2 {
		t.Errorf("expected 2 remaining after 3 allows, got %d", bucket.Remaining())
	}

	// Reset
	bucket.Reset()

	// Verify full capacity restored
	if bucket.Remaining() != 5 {
		t.Errorf("expected 5 remaining after reset, got %d", bucket.Remaining())
	}
}

// TestTokenBucket_RefillOverflow tests the refill logic when tokens exceed capacity
func TestTokenBucket_RefillOverflow(t *testing.T) {
	bucket := NewTokenBucket(5, 100) // capacity 5, refill 100/second

	// Consume all tokens
	for i := 0; i < 5; i++ {
		bucket.Allow()
	}

	// Remaining should be 0
	if bucket.Remaining() != 0 {
		t.Errorf("expected 0 remaining, got %d", bucket.Remaining())
	}

	// Wait for refill - should get more than capacity
	time.Sleep(50 * time.Millisecond) // 100/second = 5 tokens in 50ms

	// Remaining should be capped at capacity (5)
	remaining := bucket.Remaining()
	if remaining != 5 {
		t.Errorf("expected 5 (capped at capacity), got %d", remaining)
	}
}

// TestGetBucket_NewBucketCreation tests bucket creation on first access
func TestGetBucket_NewBucketCreation(t *testing.T) {
	config := &RateLimitConfig{
		Enabled:  true,
		Requests: 50,
		Window:   time.Minute,
	}

	rl := &RateLimitMiddleware{
		config:  config,
		buckets: make(map[string]*TokenBucket),
		next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	}

	// First access - should create new bucket
	bucket1 := rl.getBucket("new-key")
	if bucket1 == nil {
		t.Fatal("expected non-nil bucket")
	}

	// Second access - should return same bucket
	bucket2 := rl.getBucket("new-key")
	if bucket1 != bucket2 {
		t.Error("expected same bucket for same key")
	}

	// Different key - should create new bucket
	bucket3 := rl.getBucket("different-key")
	if bucket3 == bucket1 {
		t.Error("expected different bucket for different key")
	}
}

// ==================== getRateLimitKey Tests ====================

func TestGetRateLimitKey_Default(t *testing.T) {
	config := &RateLimitConfig{
		Enabled: true,
		Requests: 100,
		Window: time.Minute,
		// No LimitBy... set
	}

	rl := &RateLimitMiddleware{
		config: config,
		buckets: make(map[string]*TokenBucket),
	}

	req := httptest.NewRequest("GET", "/test", nil)
	key := rl.getRateLimitKey(req)

	if key != "default" {
		t.Errorf("expected 'default', got '%s'", key)
	}
}

func TestGetRateLimitKey_ByTenant(t *testing.T) {
	config := &RateLimitConfig{
		Enabled:      true,
		Requests:     100,
		Window:       time.Minute,
		LimitByTenant: true,
	}

	rl := &RateLimitMiddleware{
		config: config,
		buckets: make(map[string]*TokenBucket),
	}

	req := httptest.NewRequest("GET", "/test", nil)
	key := rl.getRateLimitKey(req)

	if key != "tenant:0" {
		t.Errorf("expected 'tenant:0', got '%s'", key)
	}
}

func TestGetRateLimitKey_ByUser(t *testing.T) {
	config := &RateLimitConfig{
		Enabled:    true,
		Requests:   100,
		Window:     time.Minute,
		LimitByUser: true,
	}

	rl := &RateLimitMiddleware{
		config: config,
		buckets: make(map[string]*TokenBucket),
	}

	req := httptest.NewRequest("GET", "/test", nil)
	key := rl.getRateLimitKey(req)

	if key != "user:unknown" {
		t.Errorf("expected 'user:unknown', got '%s'", key)
	}
}

func TestGetRateLimitKey_ByIP(t *testing.T) {
	config := &RateLimitConfig{
		Enabled:  true,
		Requests: 100,
		Window:   time.Minute,
		LimitByIP: true,
	}

	rl := &RateLimitMiddleware{
		config: config,
		buckets: make(map[string]*TokenBucket),
	}

	req := httptest.NewRequest("GET", "/test", nil)
	key := rl.getRateLimitKey(req)

	if key == "" {
		t.Error("expected non-empty key")
	}
}

func TestGetRateLimitKey_ByEndpoint(t *testing.T) {
	config := &RateLimitConfig{
		Enabled:       true,
		Requests:      100,
		Window:        time.Minute,
		LimitByEndpoint: true,
	}

	rl := &RateLimitMiddleware{
		config: config,
		buckets: make(map[string]*TokenBucket),
	}

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	key := rl.getRateLimitKey(req)

	if key != "/api/v1/test" {
		t.Errorf("expected '/api/v1/test', got '%s'", key)
	}
}

func TestGetRateLimitKey_MultipleDimensions(t *testing.T) {
	config := &RateLimitConfig{
		Enabled:      true,
		Requests:     100,
		Window:       time.Minute,
		LimitByTenant: true,
		LimitByUser:   true,
		LimitByIP:     true,
	}

	rl := &RateLimitMiddleware{
		config: config,
		buckets: make(map[string]*TokenBucket),
	}

	req := httptest.NewRequest("GET", "/test", nil)
	key := rl.getRateLimitKey(req)

	// Should contain multiple parts
	if key == "default" {
		t.Error("expected multi-part key")
	}
	if key == "" {
		t.Error("expected non-empty key")
	}
}

// ==================== handleRateLimitExceeded Tests ====================

func TestHandleRateLimitExceeded_FallbackCode(t *testing.T) {
	config := &RateLimitConfig{
		Enabled:      true,
		Requests:     10,
		Window:       time.Second,
		FallbackCode: http.StatusServiceUnavailable,
		// DegradationEnabled = false
	}

	rl := &RateLimitMiddleware{
		config:  config,
		buckets: make(map[string]*TokenBucket),
		next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("next handler should not be called")
		}),
	}

	bucket := NewTokenBucket(10, 0)
	// Exhaust all tokens
	for i := 0; i < 10; i++ {
		bucket.Allow()
	}

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Directly call handleRateLimitExceeded
	rl.handleRateLimitExceeded(w, req, bucket)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

func TestHandleRateLimitExceeded_WithoutDegradation(t *testing.T) {
	config := &RateLimitConfig{
		Enabled:      true,
		Requests:     1,
		Window:       time.Second,
		FallbackCode: http.StatusTooManyRequests,
		// DegradationEnabled = false by default
	}

	handler := &RateLimitMiddleware{
		config:  config,
		buckets: make(map[string]*TokenBucket),
		next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}

	// First request should succeed
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("first request: expected 200, got %d", w.Code)
	}

	// Second request should be rate limited
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("second request: expected 429, got %d", w.Code)
	}
}

func TestServeHTTP_Disabled(t *testing.T) {
	config := &RateLimitConfig{
		Enabled: false, // Disabled
	}

	nextCalled := false
	handler := &RateLimitMiddleware{
		config: config,
		buckets: make(map[string]*TokenBucket),
		next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		}),
	}

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("next handler should be called when rate limit is disabled")
	}
}
