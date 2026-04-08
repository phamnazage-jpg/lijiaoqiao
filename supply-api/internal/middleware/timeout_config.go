package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ==================== P1-03 中间件超时配置 ====================

// MiddlewareTimeoutConfig 中间件超时配置
type MiddlewareTimeoutConfig struct {
	// 各中间件超时配置
	RecoveryTimeout    time.Duration // Recovery中间件
	LoggingTimeout     time.Duration // Logging中间件
	RequestIDTimeout   time.Duration // RequestID中间件
	AuthnTimeout       time.Duration // 认证中间件
	AuthzTimeout       time.Duration // 授权中间件
	RateLimitTimeout   time.Duration // 限流中间件
	IdempotencyTimeout time.Duration // 幂等中间件
	BusinessTimeout    time.Duration // 业务处理

	// 默认超时
	DefaultTimeout time.Duration
}

// DefaultMiddlewareTimeoutConfig 返回默认中间件超时配置
// 根据PRD和行业最佳实践：建议总超时 ≤ 200ms
func DefaultMiddlewareTimeoutConfig() *MiddlewareTimeoutConfig {
	return &MiddlewareTimeoutConfig{
		// 快速中间件（内存操作）
		RecoveryTimeout:    5 * time.Millisecond,
		LoggingTimeout:     10 * time.Millisecond,
		RequestIDTimeout:    5 * time.Millisecond,

		// 网络操作相关
		AuthnTimeout:        50 * time.Millisecond, // JWT验证+缓存查询
		AuthzTimeout:        30 * time.Millisecond, // 权限检查
		RateLimitTimeout:    20 * time.Millisecond, // 限流检查
		IdempotencyTimeout: 30 * time.Millisecond, // 幂等检查

		// 业务处理（最灵活）
		BusinessTimeout:     100 * time.Millisecond,

		// 默认兜底超时
		DefaultTimeout:      200 * time.Millisecond,
	}
}

// TotalTimeout 计算总超时时间
func (c *MiddlewareTimeoutConfig) TotalTimeout() time.Duration {
	return c.RecoveryTimeout +
		c.LoggingTimeout +
		c.RequestIDTimeout +
		c.AuthnTimeout +
		c.AuthzTimeout +
		c.RateLimitTimeout +
		c.IdempotencyTimeout +
		c.BusinessTimeout
}

// MiddlewareTimeoutContext 带超时的中间件上下文
type MiddlewareTimeoutContext struct {
	config   *MiddlewareTimeoutConfig
	deadline time.Time
}

// NewMiddlewareTimeoutContext 创建带超时的中间件上下文
func NewMiddlewareTimeoutContext(config *MiddlewareTimeoutConfig) *MiddlewareTimeoutContext {
	if config == nil {
		config = DefaultMiddlewareTimeoutConfig()
	}

	return &MiddlewareTimeoutContext{
		config:   config,
		deadline: time.Now().Add(config.TotalTimeout()),
	}
}

// WithBusinessTimeout 创建带业务超时的上下文
func (c *MiddlewareTimeoutContext) WithBusinessTimeout() (context.Context, context.CancelFunc) {
	return context.WithDeadline(context.Background(), c.deadline)
}

// TimeoutResponseWriter 超时响应writer
type TimeoutResponseWriter struct {
	http.ResponseWriter
	mu      sync.Mutex
	timeout time.Duration
	started time.Time
}

func (w *TimeoutResponseWriter) ensureStarted() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started.IsZero() {
		w.started = time.Now()
	}
}

func (w *TimeoutResponseWriter) checkTimeout() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started.IsZero() {
		return false
	}
	return time.Since(w.started) > w.timeout
}

func (w *TimeoutResponseWriter) setTimeoutHeader() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Header().Set("X-Timeout", "true")
}

// WithTimeoutMiddleware 返回带超时检测的中间件
//
// 设计说明：
// - handler 在 goroutine 中执行
// - 超时时不等待 handler 完成，直接发送超时响应
// - 使用互斥锁确保响应只发送一次
// - 实际生产中应设置合理的超时时间使 handler 有机会在超时前完成
func WithTimeoutMiddleware(next http.Handler, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var mu sync.Mutex
		responseSent := false

		handlerDone := make(chan struct{})

		go func() {
			next.ServeHTTP(w, r)
			close(handlerDone)
		}()

		select {
		case <-handlerDone:
			return
		case <-time.After(timeout):
			mu.Lock()
			if !responseSent {
				responseSent = true
				mu.Unlock()
				w.Header().Set("X-Timeout", "true")
				http.Error(w, fmt.Sprintf("middleware timeout after %v", timeout), http.StatusGatewayTimeout)
				return
			}
			mu.Unlock()
			return
		}
	})
}

// MiddlewareStageTimeout 中间件阶段超时配置
type MiddlewareStageTimeout struct {
	Stage string
	Timeout time.Duration
}

// GetStageTimeouts 获取各阶段超时配置
func GetStageTimeouts() []MiddlewareStageTimeout {
	config := DefaultMiddlewareTimeoutConfig()
	return []MiddlewareStageTimeout{
		{"recovery", config.RecoveryTimeout},
		{"logging", config.LoggingTimeout},
		{"request_id", config.RequestIDTimeout},
		{"authn", config.AuthnTimeout},
		{"authz", config.AuthzTimeout},
		{"ratelimit", config.RateLimitTimeout},
		{"idempotency", config.IdempotencyTimeout},
		{"business", config.BusinessTimeout},
	}
}
