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
	"lijiaoqiao/gateway/internal/metrics"
	gwerror "lijiaoqiao/gateway/pkg/error"
)

// 全局随机数生成器（线程安全）
var globalRand = rand.New(rand.NewSource(time.Now().UnixNano()))

// LoadBalancerStrategy 负载均衡策略
type LoadBalancerStrategy string

const (
	StrategyLatency      LoadBalancerStrategy = "latency"
	StrategyRoundRobin   LoadBalancerStrategy = "round_robin"
	StrategyWeighted     LoadBalancerStrategy = "weighted"
	StrategyAvailability LoadBalancerStrategy = "availability"
)

// CircuitState 熔断器状态
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // 熔断器关闭，正常流量
	CircuitOpen                         // 熔断器打开，流量被拒绝
	CircuitHalfOpen                     // 熔断器半开，允许试探流量
)

// String 实现 fmt.Stringer
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "CircuitClosed"
	case CircuitOpen:
		return "CircuitOpen"
	case CircuitHalfOpen:
		return "CircuitHalfOpen"
	default:
		return "Unknown"
	}
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	FailureRateThreshold     float64       // 失败率阈值（默认0.5）
	ConsecutiveFailureLimit  int64         // 连续失败次数阈值（默认5）
	HalfOpenSuccessThreshold int64         // 半开状态需要连续成功的次数（默认3）
	OpenTimeout              time.Duration // 熔断打开后的等待时间（默认30s）
}

// DefaultCircuitBreakerConfig 返回默认配置
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureRateThreshold:     0.5,
		ConsecutiveFailureLimit:  5,
		HalfOpenSuccessThreshold: 3,
		OpenTimeout:              30 * time.Second,
	}
}

// ProviderHealth Provider健康状态
type ProviderHealth struct {
	Name          string
	Available     bool
	LatencyMs     int64
	FailureRate   float64
	Weight        float64
	LastCheckTime time.Time

	// P3-B: 熔断器字段
	CircuitState         CircuitState // 当前熔断器状态
	ConsecutiveFailures  int64        // 连续失败次数（成功时重置）
	ConsecutiveSuccesses int64        // 连续成功次数（失败时重置）
	LastStateChange      time.Time    // 上次状态变更时间
	OpenReason           string       // 熔断打开的原因（用于调试/告警）
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

	// P3-B: 熔断器
	circuitConfig CircuitBreakerConfig
	healthChecker *HealthChecker // 后台健康检查器
}

// NewRouter 创建路由器
func NewRouter(strategy LoadBalancerStrategy) *Router {
	return &Router{
		providers:     make(map[string]adapter.ProviderAdapter),
		health:        make(map[string]*ProviderHealth),
		strategy:      strategy,
		circuitConfig: DefaultCircuitBreakerConfig(),
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

	// P3-B: 熔断器打开时不允许流量
	if health.CircuitState == CircuitOpen {
		return false
	}

	// 检查模型是否支持
	provider := r.providers[name]
	if provider == nil {
		return false
	}

	supportsModel := false
	for _, m := range provider.SupportedModels() {
		if m == model || m == "*" {
			supportsModel = true
			break
		}
	}
	if !supportsModel {
		return false
	}

	// P3-B: 半开状态允许试探请求通过（不管 Available 是否为 false）
	if health.CircuitState == CircuitHalfOpen {
		return true
	}

	// Closed 状态：走原有的 Available 检查
	if !health.Available {
		return false
	}

	return true
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
	// P3-C: 同步记录到 gateway metrics（无锁，非阻塞）
	metrics.RecordProviderResult(providerName, success, latencyMs)

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

	health.LastCheckTime = time.Now()

	// P3-B: 熔断器状态机接管可用性判断，不再直接基于 FailureRate 设置 Available
	// 状态转换在 transitionCircuit 中处理
	_ = r.transitionCircuitLocked(health, success)
}

// transitionCircuitLocked 在已持有锁的情况下执行熔断器状态转换
func (r *Router) transitionCircuitLocked(health *ProviderHealth, success bool) bool {
	cfg := r.circuitConfig
	now := time.Now()
	prevState := health.CircuitState

	switch health.CircuitState {
	case CircuitClosed:
		if !success {
			health.ConsecutiveFailures++
			health.ConsecutiveSuccesses = 0

			if health.FailureRate > cfg.FailureRateThreshold ||
				health.ConsecutiveFailures >= cfg.ConsecutiveFailureLimit {
				health.CircuitState = CircuitOpen
				health.OpenReason = "failure_rate_or_consecutive_failures"
				health.LastStateChange = now
				health.Available = false
				metrics.RecordCircuitStateChange(health.Name, "closed", "open")
			}
		} else {
			health.ConsecutiveSuccesses++
			health.ConsecutiveFailures = 0
		}

	case CircuitOpen:
		// Open 状态：等待超时后由健康检查循环处理，不在这里转换

	case CircuitHalfOpen:
		if success {
			health.ConsecutiveSuccesses++
			if health.ConsecutiveSuccesses >= cfg.HalfOpenSuccessThreshold {
				health.CircuitState = CircuitClosed
				health.LastStateChange = now
				health.Available = true
				health.ConsecutiveFailures = 0
				health.FailureRate = 0
				metrics.RecordCircuitStateChange(health.Name, "half_open", "closed")
			}
		} else {
			health.ConsecutiveFailures++
			health.ConsecutiveSuccesses = 0
			health.CircuitState = CircuitOpen
			health.LastStateChange = now
			health.OpenReason = "half_open_probe_failed"
			metrics.RecordCircuitStateChange(health.Name, "half_open", "open")
		}
	}

	return prevState != health.CircuitState
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
			CircuitState:  health.CircuitState,
		}
	}
	return result
}

// StartHealthChecker 启动后台健康检查（由bootstrap调用）
func (r *Router) StartHealthChecker(interval time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.healthChecker != nil {
		return // 已经启动
	}
	r.healthChecker = NewHealthChecker(r, interval, r.circuitConfig)
	r.healthChecker.Start()
}

// StopHealthChecker 停止后台健康检查（由shutdown调用）
func (r *Router) StopHealthChecker() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.healthChecker != nil {
		r.healthChecker.Stop()
		r.healthChecker = nil
	}
}
