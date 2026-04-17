package router

import (
	"context"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lijiaoqiao/gateway/internal/adapter"
	gwerror "lijiaoqiao/gateway/pkg/error"
)

// 全局随机数生成器（线程安全）
var globalRand = rand.New(rand.NewSource(time.Now().UnixNano()))

// LoadBalancerStrategy 负载均衡策略
type LoadBalancerStrategy string

const (
	StrategyLatency     LoadBalancerStrategy = "latency"
	StrategyRoundRobin  LoadBalancerStrategy = "round_robin"
	StrategyWeighted    LoadBalancerStrategy = "weighted"
	StrategyAvailability LoadBalancerStrategy = "availability"
)

// ProviderHealth Provider健康状态
type ProviderHealth struct {
	Name          string
	Available     bool
	LatencyMs     int64
	FailureRate   float64
	Weight        float64
	LastCheckTime time.Time
}

// RegisteredModel 描述当前路由器已注册的可见模型。
type RegisteredModel struct {
	ID      string
	OwnedBy string
}

// Router 路由器
type Router struct {
	providers         map[string]adapter.ProviderAdapter
	health            map[string]*ProviderHealth
	strategy          LoadBalancerStrategy
	mu                sync.RWMutex
	roundRobinCounter uint64 // RoundRobin策略的原子计数器
}

// NewRouter 创建路由器
func NewRouter(strategy LoadBalancerStrategy) *Router {
	return &Router{
		providers: make(map[string]adapter.ProviderAdapter),
		health:    make(map[string]*ProviderHealth),
		strategy:  strategy,
	}
}

// RegisterProvider 注册Provider
func (r *Router) RegisterProvider(name string, provider adapter.ProviderAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers[name] = provider
	r.health[name] = &ProviderHealth{
		Name:          name,
		Available:     true,
		LatencyMs:     0,
		FailureRate:   0,
		Weight:        1.0,
		LastCheckTime: time.Now(),
	}
}

// SelectProvider 选择最佳Provider
func (r *Router) SelectProvider(ctx context.Context, model string) (adapter.ProviderAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var candidates []string
	for name := range r.providers {
		if r.isProviderAvailable(name, model) {
			candidates = append(candidates, name)
		}
	}

	if len(candidates) == 0 {
		return nil, gwerror.NewGatewayError(gwerror.ROUTER_NO_PROVIDER_AVAILABLE, "no provider available for model: "+model)
	}

	// 根据策略选择
	switch r.strategy {
	case StrategyLatency:
		return r.selectByLatency(candidates)
	case StrategyRoundRobin:
		return r.selectByRoundRobin(candidates)
	case StrategyWeighted:
		return r.selectByWeight(candidates)
	case StrategyAvailability:
		return r.selectByAvailability(candidates)
	default:
		return r.selectByLatency(candidates)
	}
}

func (r *Router) isProviderAvailable(name, model string) bool {
	health, ok := r.health[name]
	if !ok {
		return false
	}

	if !health.Available {
		return false
	}

	// 检查模型是否支持
	provider := r.providers[name]
	if provider == nil {
		return false
	}

	for _, m := range provider.SupportedModels() {
		if m == model || m == "*" {
			return true
		}
	}

	return false
}

func (r *Router) selectByRoundRobin(candidates []string) (adapter.ProviderAdapter, error) {
	if len(candidates) == 0 {
		return nil, gwerror.NewGatewayError(gwerror.ROUTER_NO_PROVIDER_AVAILABLE, "no available provider")
	}

	// 使用原子操作进行轮询选择
	index := atomic.AddUint64(&r.roundRobinCounter, 1) - 1
	return r.providers[candidates[index%uint64(len(candidates))]], nil
}

func (r *Router) selectByLatency(candidates []string) (adapter.ProviderAdapter, error) {
	var bestProvider adapter.ProviderAdapter
	var minLatency int64 = math.MaxInt64

	for _, name := range candidates {
		health := r.health[name]
		if health.LatencyMs < minLatency {
			minLatency = health.LatencyMs
			bestProvider = r.providers[name]
		}
	}

	if bestProvider == nil {
		return nil, gwerror.NewGatewayError(gwerror.ROUTER_NO_PROVIDER_AVAILABLE, "no available provider")
	}

	return bestProvider, nil
}

func (r *Router) selectByWeight(candidates []string) (adapter.ProviderAdapter, error) {
	var totalWeight float64
	for _, name := range candidates {
		totalWeight += r.health[name].Weight
	}

	randVal := globalRand.Float64() * totalWeight
	var cumulative float64

	for _, name := range candidates {
		cumulative += r.health[name].Weight
		if randVal <= cumulative {
			return r.providers[name], nil
		}
	}

	return r.providers[candidates[0]], nil
}

func (r *Router) selectByAvailability(candidates []string) (adapter.ProviderAdapter, error) {
	var bestProvider adapter.ProviderAdapter
	var minFailureRate float64 = math.MaxFloat64

	for _, name := range candidates {
		health := r.health[name]
		if health.FailureRate < minFailureRate {
			minFailureRate = health.FailureRate
			bestProvider = r.providers[name]
		}
	}

	if bestProvider == nil {
		return nil, gwerror.NewGatewayError(gwerror.ROUTER_NO_PROVIDER_AVAILABLE, "no available provider")
	}

	return bestProvider, nil
}

// GetFallbackProviders 获取Fallback Providers
func (r *Router) GetFallbackProviders(ctx context.Context, model string) ([]adapter.ProviderAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var fallbacks []adapter.ProviderAdapter

	for name, provider := range r.providers {
		if name == "primary" {
			continue // 跳过主Provider
		}
		if r.isProviderAvailable(name, model) {
			fallbacks = append(fallbacks, provider)
		}
	}

	return fallbacks, nil
}

// RegisteredModels 返回当前已注册 provider 聚合后的模型列表。
func (r *Router) RegisteredModels() []RegisteredModel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providerNames := make([]string, 0, len(r.providers))
	for name := range r.providers {
		providerNames = append(providerNames, name)
	}
	sort.Strings(providerNames)

	ownerByModel := make(map[string]string)
	for _, providerName := range providerNames {
		provider := r.providers[providerName]
		if provider == nil {
			continue
		}
		for _, model := range provider.SupportedModels() {
			model = strings.TrimSpace(model)
			if model == "" || model == "*" {
				continue
			}
			if _, exists := ownerByModel[model]; !exists {
				ownerByModel[model] = providerName
			}
		}
	}

	modelIDs := make([]string, 0, len(ownerByModel))
	for modelID := range ownerByModel {
		modelIDs = append(modelIDs, modelID)
	}
	sort.Strings(modelIDs)

	models := make([]RegisteredModel, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		models = append(models, RegisteredModel{
			ID:      modelID,
			OwnedBy: ownerByModel[modelID],
		})
	}

	return models
}

// RecordResult 记录调用结果
func (r *Router) RecordResult(ctx context.Context, providerName string, success bool, latencyMs int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	health, ok := r.health[providerName]
	if !ok {
		return
	}

	// 更新延迟
	if latencyMs > 0 {
		// 指数移动平均
		if health.LatencyMs == 0 {
			health.LatencyMs = latencyMs
		} else {
			health.LatencyMs = (health.LatencyMs*7 + latencyMs) / 8
		}
	}

	// 更新失败率
	if success {
		// 成功时快速恢复：使用0.5的下降因子加速恢复
		health.FailureRate = health.FailureRate * 0.5
		if health.FailureRate < 0.01 {
			health.FailureRate = 0
		}
	} else {
		// 失败时逐步上升
		health.FailureRate = health.FailureRate*0.9 + 0.1
		if health.FailureRate > 1 {
			health.FailureRate = 1
		}
	}

	// 检查是否应该标记为不可用
	if health.FailureRate > 0.5 {
		health.Available = false
	}

	health.LastCheckTime = time.Now()
}

// UpdateHealth 更新健康状态
func (r *Router) UpdateHealth(providerName string, available bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if health, ok := r.health[providerName]; ok {
		health.Available = available
		health.LastCheckTime = time.Now()
	}
}

// GetHealthStatus 获取健康状态
func (r *Router) GetHealthStatus() map[string]*ProviderHealth {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*ProviderHealth)
	for name, health := range r.health {
		result[name] = &ProviderHealth{
			Name:          health.Name,
			Available:     health.Available,
			LatencyMs:     health.LatencyMs,
			FailureRate:   health.FailureRate,
			Weight:        health.Weight,
			LastCheckTime: health.LastCheckTime,
		}
	}
	return result
}
