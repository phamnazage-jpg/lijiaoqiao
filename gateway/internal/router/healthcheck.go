package router

import (
	"context"
	"sync"
	"time"
)

// HealthChecker 后台健康检查器
type HealthChecker struct {
	router   *Router
	interval time.Duration
	cfg      CircuitBreakerConfig
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(r *Router, interval time.Duration, cfg CircuitBreakerConfig) *HealthChecker {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &HealthChecker{
		router:   r,
		interval: interval,
		cfg:      cfg,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动健康检查循环
func (hc *HealthChecker) Start() {
	hc.wg.Add(1)
	go hc.runLoop()
}

// Stop 停止健康检查循环
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
	hc.wg.Wait()
}

func (hc *HealthChecker) runLoop() {
	defer hc.wg.Done()

	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.stopCh:
			return
		case <-ticker.C:
			hc.checkAllProviders()
		}
	}
}

func (hc *HealthChecker) checkAllProviders() {
	hc.router.mu.RLock()
	providers := make(map[string]interface {
		HealthCheck(ctx context.Context) bool
	})
	for name, prov := range hc.router.providers {
		providers[name] = prov
	}
	hc.router.mu.RUnlock()

	for name, prov := range providers {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		healthy := prov.HealthCheck(ctx)
		cancel()

		// 尝试将 Open 状态的 provider 转换到 HalfOpen
		changed := hc.router.CheckAndTransitionToHalfOpen(name, healthy)
		if changed {
			// P3-B-08: 记录状态变更（指标在 transition 函数中记录）
		}
	}
}
