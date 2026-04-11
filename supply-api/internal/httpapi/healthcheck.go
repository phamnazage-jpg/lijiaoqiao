package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// ==================== P1-007 健康检查和就绪探针 ====================

// HealthChecker 健康检查接口
type HealthChecker interface {
	// Check 执行健康检查
	Check(ctx context.Context) error
	// Name 返回检查器名称
	Name() string
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status     string            `json:"status"` // "healthy" | "unhealthy" | "degraded"
	Timestamp  time.Time         `json:"timestamp"`
	DurationMs int64             `json:"duration_ms"`
	Checks     []HealthCheckResult `json:"checks,omitempty"`
}

// HealthCheckResult 单个检查结果
type HealthCheckResult struct {
	Name      string `json:"name"`
	Status    string `json:"status"` // "ok" | "error" | "warn"
	DurationMs int64 `json:"duration_ms"`
	Error     string `json:"error,omitempty"`
	Message   string `json:"message,omitempty"`
}

// ReadinessResponse 就绪检查响应
type ReadinessResponse struct {
	Status     string            `json:"status"` // "ready" | "not_ready"
	Timestamp  time.Time         `json:"timestamp"`
	Checks     []ReadinessCheckResult `json:"checks,omitempty"`
}

// ReadinessCheckResult 就绪检查结果
type ReadinessCheckResult struct {
	Name      string `json:"name"`
	Status    string `json:"status"` // "ready" | "not_ready"
	Error     string `json:"error,omitempty"`
}

// LivenessResponse 存活检查响应
type LivenessResponse struct {
	Status    string    `json:"status"` // "alive"
	Timestamp time.Time `json:"timestamp"`
}

// DefaultHealthChecker 默认健康检查器
type DefaultHealthChecker struct {
	checks []HealthChecker
}

// NewDefaultHealthChecker 创建默认健康检查器
func NewDefaultHealthChecker() *DefaultHealthChecker {
	return &DefaultHealthChecker{
		checks: make([]HealthChecker, 0),
	}
}

// AddCheck 添加健康检查
func (h *DefaultHealthChecker) AddCheck(checker HealthChecker) {
	h.checks = append(h.checks, checker)
}

// Check 执行所有健康检查
func (h *DefaultHealthChecker) Check(ctx context.Context) error {
	for _, checker := range h.checks {
		if err := checker.Check(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Name 返回名称
func (h *DefaultHealthChecker) Name() string {
	return "default"
}

// DBHealthChecker 数据库健康检查
type DBHealthChecker struct {
	checkFn func(ctx context.Context) error
}

// NewDBHealthChecker 创建数据库健康检查
func NewDBHealthChecker(checkFn func(ctx context.Context) error) *DBHealthChecker {
	return &DBHealthChecker{checkFn: checkFn}
}

func (c *DBHealthChecker) Check(ctx context.Context) error {
	return c.checkFn(ctx)
}

func (c *DBHealthChecker) Name() string {
	return "database"
}

// CacheHealthChecker 缓存健康检查
type CacheHealthChecker struct {
	checkFn func(ctx context.Context) error
}

// NewCacheHealthChecker 创建缓存健康检查
func NewCacheHealthChecker(checkFn func(ctx context.Context) error) *CacheHealthChecker {
	return &CacheHealthChecker{checkFn: checkFn}
}

func (c *CacheHealthChecker) Check(ctx context.Context) error {
	return c.checkFn(ctx)
}

func (c *CacheHealthChecker) Name() string {
	return "cache"
}

// HealthHandler 健康检查处理器
type HealthHandler struct {
	healthChecker  *DefaultHealthChecker
	readinessChecks []HealthChecker
	livenessChecks []HealthChecker
}

// NewHealthHandler 创建健康检查处理器
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{
		healthChecker:  NewDefaultHealthChecker(),
		readinessChecks: make([]HealthChecker, 0),
		livenessChecks: make([]HealthChecker, 0),
	}
}

// NewHealthHandlerWithDefaults 创建带默认检查器的健康检查处理器
// P1-007修复: 统一健康检查实现，消除重复的inline handlers
func NewHealthHandlerWithDefaults(
	dbHealthCheck func(ctx context.Context) error,
	redisHealthCheck func(ctx context.Context) error,
) *HealthHandler {
	h := NewHealthHandler()

	if dbHealthCheck != nil {
		h.AddHealthCheck(NewDBHealthChecker(dbHealthCheck))
		h.AddReadinessCheck(NewDBHealthChecker(dbHealthCheck))
	}

	if redisHealthCheck != nil {
		h.AddHealthCheck(NewCacheHealthChecker(redisHealthCheck))
		h.AddReadinessCheck(NewCacheHealthChecker(redisHealthCheck))
	}

	// 存活检查总是返回OK（不需要外部依赖）
	h.AddLivenessCheck(&staticHealthChecker{name: "liveness", err: nil})

	return h
}

// staticHealthChecker 静态健康检查器
type staticHealthChecker struct {
	name string
	err  error
}

func (c *staticHealthChecker) Check(ctx context.Context) error {
	return c.err
}

func (c *staticHealthChecker) Name() string {
	return c.name
}

// AddHealthCheck 添加健康检查
func (h *HealthHandler) AddHealthCheck(checker HealthChecker) {
	h.healthChecker.AddCheck(checker)
}

// AddReadinessCheck 添加就绪检查
func (h *HealthHandler) AddReadinessCheck(checker HealthChecker) {
	h.readinessChecks = append(h.readinessChecks, checker)
}

// AddLivenessCheck 添加存活检查
func (h *HealthHandler) AddLivenessCheck(checker HealthChecker) {
	h.livenessChecks = append(h.livenessChecks, checker)
}

// ServeHealth 处理健康检查请求
func (h *HealthHandler) ServeHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	response := HealthResponse{
		Timestamp:  start,
		DurationMs: 0,
		Checks:     make([]HealthCheckResult, 0),
	}

	overallStatus := "healthy"

	for _, checker := range h.healthChecker.checks {
		checkStart := time.Now()
		err := checker.Check(ctx)

		result := HealthCheckResult{
			Name:       checker.Name(),
			DurationMs: time.Since(checkStart).Milliseconds(),
		}

		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
			overallStatus = "unhealthy"
		} else {
			result.Status = "ok"
		}

		response.Checks = append(response.Checks, result)
	}

	response.Status = overallStatus
	response.DurationMs = time.Since(start).Milliseconds()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ServeReadiness 处理就绪检查请求
func (h *HealthHandler) ServeReadiness(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	response := ReadinessResponse{
		Timestamp: time.Now(),
		Checks:    make([]ReadinessCheckResult, 0),
	}

	overallStatus := "ready"

	for _, checker := range h.readinessChecks {
		err := checker.Check(ctx)

		result := ReadinessCheckResult{
			Name: checker.Name(),
		}

		if err != nil {
			result.Status = "not_ready"
			result.Error = err.Error()
			overallStatus = "not_ready"
		} else {
			result.Status = "ready"
		}

		response.Checks = append(response.Checks, result)
	}

	response.Status = overallStatus

	statusCode := http.StatusOK
	if overallStatus == "not_ready" {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// ServeLiveness 处理存活检查请求
func (h *HealthHandler) ServeLiveness(w http.ResponseWriter, r *http.Request) {
	response := LivenessResponse{
		Status:    "alive",
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// RegisterHealthRoutes 注册健康检查路由
func (h *HealthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/actuator/health", h.ServeHealth)
	mux.HandleFunc("/actuator/health/ready", h.ServeReadiness)
	mux.HandleFunc("/actuator/health/live", h.ServeLiveness)
}
