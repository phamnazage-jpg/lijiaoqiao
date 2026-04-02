package fallback

import (
	"context"
	"errors"

	"lijiaoqiao/gateway/internal/adapter"
	"lijiaoqiao/gateway/internal/router/strategy"
)

// ErrAllTiersFailed 所有Fallback层级都失败
var ErrAllTiersFailed = errors.New("all fallback tiers failed")

// ErrRateLimitExceeded 限流错误
var ErrRateLimitExceeded = errors.New("rate limit exceeded")

// FallbackHandler Fallback处理器
type FallbackHandler struct {
	tiers    []TierConfig
	router   FallbackRouter
	metrics  FallbackMetrics
	providerGetter ProviderGetter
}

// TierConfig Fallback层级配置
type TierConfig struct {
	Tier      int
	Providers []string
	TimeoutMs int64
}

// FallbackMetrics Fallback指标接口
type FallbackMetrics interface {
	RecordTakeoverMark(provider string, tier int)
}

// ProviderGetter Provider获取器接口
type ProviderGetter interface {
	GetProvider(name string) adapter.ProviderAdapter
}

// FallbackRouter Fallback路由器接口
type FallbackRouter interface {
	SelectProvider(ctx context.Context, req *strategy.RoutingRequest, providerName string) (*strategy.RoutingDecision, error)
}

// NewFallbackHandler 创建Fallback处理器
func NewFallbackHandler() *FallbackHandler {
	return &FallbackHandler{
		tiers: make([]TierConfig, 0),
	}
}

// SetTiers 设置Fallback层级
func (h *FallbackHandler) SetTiers(tiers []TierConfig) {
	h.tiers = tiers
}

// SetRouter 设置路由器
func (h *FallbackHandler) SetRouter(router FallbackRouter) {
	h.router = router
}

// SetMetrics 设置指标收集器
func (h *FallbackHandler) SetMetrics(metrics FallbackMetrics) {
	h.metrics = metrics
}

// SetProviderGetter 设置Provider获取器
func (h *FallbackHandler) SetProviderGetter(getter ProviderGetter) {
	h.providerGetter = getter
}

// Handle 处理Fallback
func (h *FallbackHandler) Handle(ctx context.Context, req *strategy.RoutingRequest) (*strategy.RoutingDecision, error) {
	if len(h.tiers) == 0 {
		return nil, ErrAllTiersFailed
	}

	// 按层级顺序尝试
	for _, tier := range h.tiers {
		decision, err := h.tryTier(ctx, req, tier)
		if err == nil {
			// 成功，记录指标
			if h.metrics != nil {
				h.metrics.RecordTakeoverMark(decision.Provider, tier.Tier)
			}
			return decision, nil
		}

		// 检查是否是限流错误
		if errors.Is(err, ErrRateLimitExceeded) {
			// 限流错误立即返回，不继续降级
			return nil, err
		}

		// 其他错误，尝试下一层级
		continue
	}

	return nil, ErrAllTiersFailed
}

// tryTier 尝试单个层级
func (h *FallbackHandler) tryTier(ctx context.Context, req *strategy.RoutingRequest, tier TierConfig) (*strategy.RoutingDecision, error) {
	for _, providerName := range tier.Providers {
		decision, err := h.router.SelectProvider(ctx, req, providerName)
		if err == nil {
			decision.TakeoverMark = true
			return decision, nil
		}

		// 检查是否是限流错误
		if isRateLimitError(err) {
			return nil, ErrRateLimitExceeded
		}

		// 其他错误，继续尝试下一个provider
		continue
	}

	return nil, ErrAllTiersFailed
}

// isRateLimitError 判断是否是限流错误
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	// 检查错误消息中是否包含rate limit
	return containsRateLimit(err.Error())
}

func containsRateLimit(s string) bool {
	return len(s) > 0 && (contains(s, "rate limit") || contains(s, "ratelimit") || contains(s, "too many requests"))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
