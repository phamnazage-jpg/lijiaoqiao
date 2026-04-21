package router

import (
	"time"
)

// transitionCircuit 尝试进行熔断器状态转换
// 返回是否发生了状态转换
func (r *Router) transitionCircuit(providerName string, success bool, cfg CircuitBreakerConfig) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	health, ok := r.health[providerName]
	if !ok {
		return false
	}

	now := time.Now()
	prevState := health.CircuitState

	switch health.CircuitState {
	case CircuitClosed:
		if !success {
			health.ConsecutiveFailures++
			health.ConsecutiveSuccesses = 0

			// 检查是否应该打开熔断器
			if health.FailureRate > cfg.FailureRateThreshold ||
				health.ConsecutiveFailures >= cfg.ConsecutiveFailureLimit {
				health.CircuitState = CircuitOpen
				health.OpenReason = "failure_rate_or_consecutive_failures"
				health.LastStateChange = now
				health.Available = false
			}
		} else {
			health.ConsecutiveSuccesses++
			health.ConsecutiveFailures = 0
			// 成功后重置连续失败计数，让 FailureRate 慢慢下降
		}

	case CircuitOpen:
		// 处于打开状态，检查是否超时可以进入半开
		if now.Sub(health.LastStateChange) >= cfg.OpenTimeout {
			health.CircuitState = CircuitHalfOpen
			health.LastStateChange = now
			health.ConsecutiveFailures = 0
			health.ConsecutiveSuccesses = 0
			// 半开状态下 Available 仍为 false，但允许少量试探请求
		}

	case CircuitHalfOpen:
		if success {
			health.ConsecutiveSuccesses++
			if health.ConsecutiveSuccesses >= cfg.HalfOpenSuccessThreshold {
				// 成功达到阈值，关闭熔断器
				health.CircuitState = CircuitClosed
				health.LastStateChange = now
				health.Available = true
				health.ConsecutiveFailures = 0
				health.FailureRate = 0 // 重置失败率
			}
		} else {
			health.ConsecutiveFailures++
			health.ConsecutiveSuccesses = 0
			// 失败，回到打开状态
			health.CircuitState = CircuitOpen
			health.LastStateChange = now
			health.OpenReason = "half_open_probe_failed"
		}
	}

	return prevState != health.CircuitState
}

// CheckAndTransitionToHalfOpen 检查 Open 状态的 provider 是否可以进入半开
// 由后台健康检查循环调用
func (r *Router) CheckAndTransitionToHalfOpen(providerName string, providerHealthy bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	health, ok := r.health[providerName]
	if !ok {
		return false
	}

	if health.CircuitState != CircuitOpen {
		return false
	}

	// Provider 自身健康，转换到 HalfOpen 允许试探流量
	if providerHealthy {
		health.CircuitState = CircuitHalfOpen
		health.LastStateChange = time.Now()
		health.ConsecutiveFailures = 0
		health.ConsecutiveSuccesses = 0
		return true
	}

	return false
}

// GetCircuitState 返回 provider 的熔断器状态
func (r *Router) GetCircuitState(providerName string) CircuitState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	health, ok := r.health[providerName]
	if !ok {
		return CircuitClosed
	}
	return health.CircuitState
}

// SetCircuitConfig 设置熔断器配置
func (r *Router) SetCircuitConfig(cfg CircuitBreakerConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.circuitConfig = cfg
}

// GetCircuitConfig 返回熔断器配置
func (r *Router) GetCircuitConfig() CircuitBreakerConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.circuitConfig
}
