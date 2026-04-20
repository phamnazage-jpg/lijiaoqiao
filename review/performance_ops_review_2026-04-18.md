# 性能和运维可观测性评审报告

**项目**: 立交桥 (Gateway + Supply-API)  
**评审日期**: 2026-04-18  
**评审范围**: 并发模型、连接复用、限流过载保护、日志指标、容器化、故障恢复

---

## 1. 并发模型

### 1.1 HTTP Server 并发架构

**Go HTTP Server 模型**  
- Gateway 使用标准库 `net/http.Server`，goroutine-per-request 模式
- Server 配置合理的超时: `ReadTimeout=30s`, `WriteTimeout=30s`, `IdleTimeout=120s`
- 优雅关闭: 接收 SIGINT/SIGTERM 信号，等待 30 秒超时后强制关闭

```go
// main.go:29-34
go func() {
    if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("Server failed: %v", err)
    }
}()
```

**评估**: ✅ 符合 Go 生产级服务标准模式

### 1.2 路由层并发控制

**Router 使用 `sync.RWMutex`**
```go
// router.go:51
type Router struct {
    providers  map[string]adapter.ProviderAdapter
    health     map[string]*ProviderHealth
    strategy   LoadBalancerStrategy
    mu         sync.RWMutex
    roundRobinCounter uint64 // 原子计数器
}
```

- `SelectProvider()` 使用 `RLock()` 允许并发读
- `RecordResult()` 使用 `Lock()` 独占写
- RoundRobin 使用 `atomic.AddUint64` 无锁原子操作

**评估**: ✅ 读写锁分离设计合理，RoundRobin 无锁实现正确

### 1.3 Token Bucket 限流器并发

```go
// ratelimit.go:82-92
func (l *TokenBucketLimiter) AllowToken(ctx context.Context, key string, tokens int) (bool, error) {
    l.mu.Lock()
    bucket, exists := l.buckets[key]
    if !exists { bucket = l.newBucket(...) }
    l.mu.Unlock()

    bucket.mu.Lock()  // 每个 bucket 有独立锁
    defer bucket.mu.Unlock()
    l.refill(bucket)
    // ...
}
```

**评估**: ✅ 两层锁设计精细化，减少锁竞争

### 1.4 潜在问题

| 问题 | 位置 | 严重度 |
|------|------|--------|
| RemoteTokenRuntime 并发安全 | `middleware/remote_runtime.go:89-94` | 中 |
| 内存 Token Runtime 清理无锁隔离 | `middleware/runtime.go:155-171` | 低 |

---

## 2. 连接复用

### 2.1 HTTP Client 配置分析

**OpenAI Adapter** (Critical Issue)
```go
// openai_adapter.go:27-30
httpClient: &http.Client{
    Timeout: 60 * time.Second,
},
```

**问题**:
- ❌ **未配置 `Transport`**，使用 `http.DefaultTransport`
- ❌ 无 `MaxIdleConns` 设置，默认仅 100 个 idle 连接
- ❌ 无 `IdleConnTimeout`，连接可能长期占用
- ❌ 无 `MaxConnsPerHost`，同一上游连接数无限制
- ❌ 每个 `OpenAIAdapter` 实例创建独立的 `http.Client`

### 2.2 连接池配置缺失

| 配置项 | 当前状态 | 影响 |
|--------|----------|------|
| `MaxIdleConns` | 默认 100 | 上游连接数受限 |
| `MaxIdleConnsPerHost` | 默认 2 | 同 host 复用受限 |
| `IdleConnTimeout` | 默认永久 | 空闲连接不释放 |
| `MaxConnsPerHost` | 无限制 | 可能创建过多连接 |
| `ConnKeepAlive` | 默认 | 长连接时间不确定 |

### 2.3 其他 HTTP Client 使用

```go
// bootstrap.go:165
return middleware.NewRemoteTokenRuntime(cfg.TokenRuntimeURL, http.DefaultClient, time.Now)
```

- Remote Token Runtime 使用 `http.DefaultClient`
- 默认 client 无连接池优化，高并发下性能差

### 2.4 数据库连接池

```go
// bootstrap.go:140-154 - 数据库审计发射器
dsn := fmt.Sprintf("postgres://%s:***@%s:%d/%s?sslmode=disable", ...)
auditor, err := middleware.NewDatabaseAuditEmitter(dsn, time.Now)
```

- 使用 `jackc/pgx/v5` 连接池库
- 配置中有 `MaxConns` 参数但未在 bootstrap 中使用
- **问题**: 数据库连接池大小未显式配置

### 2.5 Redis 连接池

```go
// config.go:80
PoolSize int  // 配置存在但未在 bootstrap 中使用
```

- Redis 配置有 `PoolSize`，但未看到实际应用
- **问题**: Redis 连接池未初始化

---

## 3. 限流过载保护

### 3.1 限流算法

**TokenBucketLimiter**
```go
// ratelimit.go:61-73
func NewTokenBucketLimiter(defaultRPM, defaultTPM int, burstMultiplier float64) *TokenBucketLimiter {
    limiter := &TokenBucketLimiter{
        buckets:        make(map[string]*tokenBucket),
        defaultRPM:     defaultRPM,
        defaultTPM:     defaultTPM,
        burstMultiplier: burstMultiplier,
        cleanInterval:  5 * time.Minute,
    }
    go limiter.cleanup()  // 后台清理
    return limiter
}
```

- 支持 RPM (请求/分钟) 和 TPM (Token/分钟)
- 突发容量配置 (`BurstMultiplier=1.5`)
- 后台 goroutine 清理过期 bucket

**SlidingWindowLimiter**
```go
// ratelimit.go:200-230
func (l *SlidingWindowLimiter) Allow(ctx context.Context, key string) (bool, error) {
    // 滑动窗口算法，O(n) 时间复杂度
}
```

- **问题**: `AllowToken()` 忽略 tokens 参数，固定按 1 请求处理
- **问题**: 每次检查都遍历整个请求列表，O(n) 复杂度

### 3.2 限流中间件

```go
// ratelimit.go:303-328
func (m *Middleware) Limit(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        key := extractRateLimitKey(r)  // 使用 Authorization header
        if key == "" { key = r.RemoteAddr }

        allowed, err := m.limiter.Allow(r.Context(), key)
        if !allowed {
            w.Header().Set("X-RateLimit-Limit", ...)
            w.Header().Set("X-RateLimit-Remaining", ...)
            w.Header().Set("X-RateLimit-Reset", ...)
        }
    }
}
```

**优点**:
- ✅ 返回标准 RateLimit 头
- ✅ 基于 API Key 精细化限流
- ✅ fallback 到 IP 限流

**缺点**:
- ❌ 全局无基于租户/服务级别的限流
- ❌ 无针对上游 Provider 的限流保护

### 3.3 Provider 失败熔断

```go
// router.go:302-305
if health.FailureRate > 0.5 {
    health.Available = false
}
```

- 失败率 > 50% 自动摘除 Provider
- 恢复机制: 成功后指数下降 (`FailureRate * 0.5`)

**问题**:
- ❌ 无熔断恢复后的渐进式加入
- ❌ 无熔断期间的请求排队/快速失败
- ❌ 无针对特定错误的差异化处理

### 3.4 缺失的过载保护

| 功能 | 状态 | 说明 |
|------|------|------|
| 熔断器 (Circuit Breaker) | ❌ 不存在 | 无 |
| 重试策略 | ⚠️ 注释存在 | `MaxRetries=3` 配置但未使用 |
| 请求队列/背压 | ❌ 不存在 | 无 |
| 上游自适应限流 | ❌ 不存在 | 无 |

---

## 4. 日志指标

### 4.1 日志系统

**Critical Issue: 无结构化日志**

```go
// main.go - 仅使用标准 log 包
log.Printf("Starting gateway server on %s", server.Addr)
log.Fatalf("Failed to load config: %v", err)
```

**问题**:
- ❌ 使用标准库 `log`，无结构化字段
- ❌ 无日志级别控制
- ❌ 无日志采样
- ❌ 无请求 ID 传递到日志
- ❌ 无性能日志（无请求耗时记录）

### 4.2 指标系统

**RoutingMetrics 存在但未接入**
```go
// router/metrics/routing_metrics.go
type RoutingMetrics struct {
    totalRequests     int64
    totalTakeovers    int64
    primaryTakeovers  int64
    fallbackTakeovers int64
    providerStats     map[string]*ProviderStat
    strategyStats     map[string]*StrategyStat
}
```

**问题**:
- ❌ `RoutingMetrics` 未在 `BuildServer` 中创建和注入
- ❌ 无 Prometheus metrics endpoint
- ❌ 无 OpenTelemetry tracing
- ❌ 无 W3C Trace Context 支持

### 4.3 健康检查

```go
// handler.go:299-323
func (h *Handler) HealthHandle(w http.ResponseWriter, r *http.Request) {
    healthStatus := h.router.GetHealthStatus()
    // 返回 provider 级别健康状态
}
```

- ✅ `/health`, `/healthz`, `/readyz` 端点
- ✅ Provider 级别健康状态
- ✅ 返回 `degraded` 状态当有 Provider 不可用

### 4.4 审计日志

```go
// middleware/runtime.go:193-215
type MemoryAuditEmitter struct {
    events []AuditEvent  // 内存存储，无持久化
}
```

- ✅ 支持内存审计发射器
- ✅ 支持数据库审计发射器 (可选)
- ❌ 无审计日志导出到外部系统

### 4.5 告警系统

```go
// alert/alert.go
type Manager struct {
    senders []Sender  // Email, DingTalk, Feishu
}
```

- ✅ 支持多种告警渠道
- ⚠️ 告警发送器未连接到实际触发点
- ❌ 无预算告警触发逻辑
- ❌ 无 Provider 失败自动告警

---

## 5. 容器化

### 5.1 现状

| 组件 | Dockerfile | docker-compose | K8s |
|------|------------|---------------|-----|
| gateway | ❌ 不存在 | ❌ 不存在 | ❌ 不存在 |
| supply-api | ❌ 不存在 | ✅ 存在 | ❌ 不存在 |
| postgres | N/A | ✅ 测试用 | ❌ 不存在 |
| redis | N/A | ✅ 测试用 | ❌ 不存在 |

### 5.2 Supply-API Docker Compose

```yaml
# supply-api/deploy/docker-compose.yml
services:
  postgres:
    image: postgres:15-alpine
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U supply_test"]
      interval: 5s
      timeout: 5s
      retries: 5
  redis:
    image: redis:7-alpine
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
```

- ✅ 有 healthcheck 配置
- ⚠️ 仅用于测试基础设施，非生产部署

### 5.3 容器化缺口

**Gateway 缺失**:
- ❌ 无多阶段构建 Dockerfile
- ❌ 无非 root 用户运行
- ❌ 无优雅停止信号处理（虽然代码支持）
- ❌ 无探针配置说明
- ❌ 无资源限制配置示例

---

## 6. 故障恢复

### 6.1 优雅关闭

```go
// main.go:36-51
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
server.Shutdown(ctx)
```

- ✅ 捕获 SIGINT/SIGTERM
- ✅ 30 秒优雅关闭超时
- ❌ 无处理中的请求完成跟踪
- ❌ 无关闭后钩子（如审计 flush）

### 6.2 Fallback 机制

```go
// router/fallback/fallback.go
func (h *FallbackHandler) Handle(ctx context.Context, req *strategy.RoutingRequest) (*strategy.RoutingDecision, error) {
    for _, tier := range h.tiers {
        decision, err := h.tryTier(ctx, req, tier)
        if err == nil { return decision, nil }
        if errors.Is(err, ErrRateLimitExceeded) { return nil, err }  // 限流不降级
    }
    return nil, ErrAllTiersFailed
}
```

- ✅ 多层级 Fallback
- ✅ 限流错误不降级
- ❌ FallbackHandler 未接入 `BuildServer`

### 6.3 实验性模块未接入

```go
// bootstrap.go:171-172 注释
// resolveStrategy 只暴露当前主启动链路已验证的策略。
// cost_based、cost_aware 与 fallback 仍停留在实验模块，未接入 BuildServer。
```

| 模块 | 状态 |
|------|------|
| FallbackHandler | ❌ 未接入 |
| CostBasedStrategy | ❌ 未接入 |
| CostAwareStrategy | ❌ 未接入 |
| RoutingMetrics | ❌ 未接入 |

### 6.4 恢复能力矩阵

| 场景 | 当前能力 |
|------|----------|
| 上游 Provider 瞬时故障 | ✅ 基于失败率的自动摘除 |
| 上游 Provider 持续故障 | ❌ 无渐进恢复，需手动 |
| 限流触发 | ✅ 返回 429 + 降级 |
| 本地 OOM | ❌ 无保护 |
| 数据库连接断开 | ⚠️ 仅记录日志 |
| Redis 连接断开 | ⚠️ 仅记录日志 |

---

## 7. 综合评估

### 7.1 评分矩阵

| 维度 | 评分 (1-5) | 说明 |
|------|------------|------|
| 并发模型 | 4 | RWMutex + 原子操作，设计合理 |
| 连接复用 | 2 | 无 Transport 配置，连接池缺失 |
| 限流过载 | 3 | 基础限流存在，熔断器缺失 |
| 日志指标 | 2 | 无结构化日志，无 metrics 导出 |
| 容器化 | 1 | gateway 完全无容器化配置 |
| 故障恢复 | 3 | 优雅关闭存在，fallback 未接入 |

### 7.2 关键风险

1. **连接池配置缺失** - 高并发下可能耗尽上游连接
2. **无结构化日志** - 生产环境无法有效追踪问题
3. **Gateway 无容器化** - 无法部署到 K8s 环境
4. **熔断器缺失** - 无法防止故障级联
5. **实验模块未接入** - Fallback 等高可用能力不可用

### 7.3 建议优先级

| 优先级 | 改进项 |
|--------|--------|
| P0 | 添加 HTTP Transport 连接池配置 |
| P0 | 实现结构化日志 (slog/zap) |
| P0 | Gateway Dockerfile |
| P1 | 接入 RoutingMetrics + Prometheus |
| P1 | 接入 FallbackHandler |
| P1 | 添加熔断器模式 |
| P2 | OpenTelemetry Tracing |
| P2 | 告警触发点接入 |

---

## 附录: 关键代码位置

| 文件 | 用途 |
|------|------|
| `gateway/cmd/gateway/main.go` | 服务入口，优雅关闭 |
| `gateway/internal/adapter/openai_adapter.go:27` | HTTP Client 配置 |
| `gateway/internal/ratelimit/ratelimit.go` | 限流实现 |
| `gateway/internal/router/router.go` | 路由 + 健康状态 |
| `gateway/internal/handler/handler.go` | HTTP Handler |
| `gateway/internal/app/bootstrap.go` | 服务构建 (未接入 metrics/fallback) |
| `gateway/internal/router/metrics/routing_metrics.go` | 指标收集 (未接入) |
| `gateway/internal/router/fallback/fallback.go` | Fallback (未接入) |

---

*评审人: Hermes Agent*  
*生成时间: 2026-04-18*
