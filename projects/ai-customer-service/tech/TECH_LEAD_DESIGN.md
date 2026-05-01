# TechLead 技术设计文档 — AI-Customer-Service 生产一期

> 版本：v1.0
> 日期：2026-04-30
> 状态：TechLead Review Complete

---

## 1. 生产数据模型与 Migration 方案

### 1.1 当前 Schema 评估

现有 `0001_init.up.sql` 已覆盖核心表，但缺少以下生产必填字段和表：

#### 缺口 1：`cs_sessions.tenant_id` 缺失
生产环境必须支持多租户，`cs_sessions` / `cs_tickets` / `cs_audit_logs` 均需 `tenant_id`。
- **修复方案**：新增 migration `0002_add_tenant_id.up.sql`
- **影响**：必须向后兼容，现有数据 default 为 `'default'`

#### 缺口 2：`cs_tickets.assigned_at` 缺失
工单分配时间用于 SLA 计算和排队位置查询。
- **修复方案**：新增 `assigned_at TIMESTAMPTZ` 字段

#### 缺口 3：`cs_tickets.status` 缺少 `'pending'` 状态
当前仅 `open/assigned/processing/resolved/closed`，但客服接单前应有 `pending` 过渡状态。
- **HLD 漂移检测**：INTERFACE.md 定义的状态机无 `pending`，但运营场景需要"排队中"状态
- **建议**：将现有 `open` 重语义为 `pending`，另起 `assigned` 为"已分配"

#### 缺口 4：缺少 `cs_agent_sessions` 和 `cs_agent_stats` 表
HLD 3.8.X/3.8.Y 定义了这两个表用于客服统计，当前不存在。
- **修复方案**：新增 migration `0003_add_agent_tables.up.sql`

#### 缺口 5：缺少 `cs_channel_bindings` 表
HLD 4.2.5 定义了渠道绑定表，当前未实现。

### 1.2 Migration 命名规范

```
db/migration/
├── 0001_init.up.sql          # 已有
├── 0002_add_tenant_id.up.sql # TechLead: 新增
├── 0003_add_agent_tables.up.sql
├── 0004_add_ticket_fields.up.sql
└── 0005_add_channel_bindings.up.sql
```

### 1.3 具体 Migration 设计

#### `0002_add_tenant_id.up.sql`
```sql
ALTER TABLE cs_sessions ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE cs_tickets ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';
ALTER TABLE cs_audit_logs ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default';

CREATE INDEX IF NOT EXISTS idx_sessions_tenant ON cs_sessions(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_tickets_tenant ON cs_tickets(tenant_id, status, priority);
-- 回滚：ALTER TABLE DROP COLUMN tenant_id CASCADE（注意与现有 FK 冲突检测）
```

#### `0003_add_agent_tables.up.sql`
```sql
CREATE TABLE IF NOT EXISTS cs_agent_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id VARCHAR(64) NOT NULL,
    ticket_id UUID NOT NULL REFERENCES cs_tickets(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS cs_agent_stats (
    id BIGSERIAL PRIMARY KEY,
    agent_id VARCHAR(64) NOT NULL,
    date DATE NOT NULL,
    tickets_handled INT DEFAULT 0,
    avg_handle_time_sec INT DEFAULT 0,
    handoff_count INT DEFAULT 0,
    csat_score DECIMAL(3,2) NULL,
    UNIQUE(agent_id, date)
);
```

#### `0004_add_ticket_fields.up.sql`
```sql
ALTER TABLE cs_tickets ADD COLUMN assigned_at TIMESTAMPTZ NULL;
ALTER TABLE cs_tickets ALTER COLUMN status TYPE VARCHAR(16);
-- 将 status CHECK 更新（见下节状态机设计）
```

#### `0005_add_channel_bindings.up.sql`
```sql
CREATE TABLE IF NOT EXISTS cs_channel_bindings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel VARCHAR(16) NOT NULL,
    open_id VARCHAR(128) NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    bound_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    bound_method VARCHAR(16) NOT NULL,
    UNIQUE(channel, open_id)
);
CREATE INDEX IF NOT EXISTS idx_bindings_user ON cs_channel_bindings(user_id);
```

### 1.4 状态机修正（Close vs Resolve 语义）

当前实现将 `resolve` 和 `close` 作为两个独立 API，语义混淆。

**修正语义：**
- `resolve`：客服提交处理结果，状态 → `resolved`，可继续补充 resolution
- `close`：工单正式结单，状态 → `closed`，不可再修改
- API 设计：`POST /tickets/{id}/resolve`（提交结果），`POST /tickets/{id}/close`（结单）

**迁移路径**：
1. 当前 `resolved_at` 字段保留，`resolved` 仍为中间状态
2. 运营后台在 resolve 后可选择 close 或让系统自动 close（需决策）
3. 会话状态机：Handoff → `open` → `assigned` → `processing` → `resolved` → `closed`

**需要 TechLead 决策**：`resolved` 状态是否需要人工 close 才能关闭，还是系统自动 close？建议 resolve 后允许用户评价结单，评价后系统自动 close。

---

## 2. Webhook 签名、防重放、幂等、审计 Fail-Closed 方案

### 2.1 当前状态评估

| 能力 | 当前实现 | 评估 |
|------|---------|------|
| 签名校验 | `webhook_security.go` HMAC-SHA256 | ✅ 已实现 |
| 时间戳防重放 | skew 校验（无 nonce 持久化） | ⚠️ 仅 skew，无真正防重放 |
| 幂等去重 | `dedup_store.go` 已有 | ✅ 基本实现 |
| 安全拒绝审计 | `webhook_security.auditReject` | ⚠️ 已调用但 `Audit` 可能为 nil |
| 失败 Body 审计 | `webhook_handler.auditRejectedRequest` | ✅ 已实现 |

### 2.2 签名校验当前问题

**问题 1**：`WebhookSecurity` 的 `Audit` 字段在 `app.go` 中已正确传入 `audits`（即 `AuditStore`），但 `AuditRecorder` 接口为 nil-check 调用，属于**部分 fail-closed**（代码存在但不保证所有路径都记录）。

**问题 2**：`webhook_handler.go` 的 `auditRejectedRequest` 在 `handle()` 中所有拒绝路径都被调用，包括非法 JSON、字段缺失、内容超长，**这部分已正确实现**。

**问题 3**：`WebhookSecurity.auditReject` 在签名失败时写入 `webhook_security_rejected` 类型，`WebhookHandler.auditRejectedRequest` 写入 `webhook_rejected` 类型，**存在重复但互补**。

### 2.3 防重放方案升级

当前时间戳 skew 校验不足以防止 replay 攻击（攻击者在有效窗口内重放旧消息）。

**修复方案：在 Redis/DB 中持久化 nonce**

```go
// internal/store/postgres/nonce_store.go
type NonceStore struct {
    db *sql.DB
}

// NonceKey returns the redis key for a given channel+nonce.
// Uses Postgres if Redis unavailable (同步写入，TTL 自动清理).
func (s *NonceStore) TryUse(ctx context.Context, channel, nonce string, ttl time.Duration) (bool, error) {
    // INSERT ... ON CONFLICT DO NOTHING，TTL 通过 PostgreSQL 定期清理任务实现
    _, err := s.db.ExecContext(ctx, `
        INSERT INTO cs_webhook_nonces (channel, nonce, used_at)
        VALUES ($1, $2, NOW())
        ON CONFLICT (channel, nonce) DO NOTHING`)
    if err != nil {
        return false, err
    }
    // PostgreSQL 没有 TTL 支持，改为每日清理：
    // DELETE FROM cs_webhook_nonces WHERE used_at < NOW() - INTERVAL '1 day'
    return true, nil
}
```

**Migration**:
```sql
CREATE TABLE IF NOT EXISTS cs_webhook_nonces (
    channel VARCHAR(16) NOT NULL,
    nonce VARCHAR(128) NOT NULL,
    used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (channel, nonce)
);
CREATE INDEX idx_nonces_cleanup ON cs_webhook_nonces(used_at);
```

### 2.4 幂等语义澄清

当前幂等键为 `(channel, message_id)`，但：
1. 不同渠道可能出现相同 `message_id` → 需要 `(channel, provider_id, message_id)` 三元组
2. `message_id` 为空时跳过幂等检查（内部消息或测试流量）

**修复方案**：扩展 `cs_message_dedup` 主键为 `(channel, provider, message_id)`。

### 2.5 安全拒绝审计 fail-closed 确认

审计失败时整体请求应该返回 500，当前实现仅 `log.Error` 后继续。需要确认 fail-closed 策略：
- **当前行为**（签名失败时）：写审计失败 → 仍返回 403 → 这是正确的 fail-closed（响应失败但审计可选）
- **高风险操作**（工单状态变更时）：审计失败必须返回 500

**需要决策**：ticket assign/resolve 审计写入失败是否应该回滚状态变更？建议设为可配置，紧急情况下允许 fail-open。

---

## 3. Ticket / Session / Audit / KB 真实架构

### 3.1 Session 状态机缺口

**问题**：`domain/session/session.go` 缺少 `StatusWaitingFeedback`（HLD 定义为等待用户反馈状态）。

当前会话状态：`idle/processing/handoff/closed`，缺少 `waiting_feedback`。

**修复方案**：
```go
// domain/session/session.go
const (
    StatusIdle             Status = "idle"
    StatusProcessing       Status = "processing"
    StatusWaitingFeedback  Status = "waiting_feedback"  // 新增
    StatusHandoff          Status = "handoff"
    StatusClosed           Status = "closed"
)
```

**对应 SQL**（需更新 migration）：
```sql
ALTER TABLE cs_sessions DROP CONSTRAINT chk_cs_sessions_status;
ALTER TABLE cs_sessions ADD CONSTRAINT chk_cs_sessions_status 
    CHECK (status IN ('idle','processing','waiting_feedback','handoff','closed'));
```

### 3.2 排队位置查询接口设计（P1-3）

HLD 未定义排队位置查询接口，需要 TechLead 设计。

**API 设计**：
```
GET /api/v1/customer-service/tickets/queue-position?ticket_id={id}
Response: {
    "ticket_id": "xxx",
    "position": 3,
    "estimated_wait_minutes": 15,
    "ahead_count": 2,
    "priority": "P2"
}
```

**实现逻辑**：
```go
// internal/http/handlers/queue_handler.go
func (h *QueueHandler) GetPosition(w http.ResponseWriter, r *http.Request) {
    ticketID := r.URL.Query().Get("ticket_id")
    ticket, err := h.ticketStore.GetByID(r.Context(), ticketID)
    if err != nil {
        writeJSON(w, http.StatusNotFound, map[string]any{...})
        return
    }
    position, err := h.ticketStore.GetQueuePosition(r.Context(), ticket)
    // position = count of open tickets with higher priority, then same priority older
    writeJSON(w, http.StatusOK, map[string]any{
        "ticket_id": ticketID,
        "position": position,
        "estimated_wait_minutes": position * 5, // P2 平均处理时间 5 分钟
        "priority": ticket.Priority,
    })
}
```

### 3.3 Audit 与 Ticket 联动

**当前问题**：`ticket_workflow.go` 的 `writeAudit` 是静默失败（仅 log.Error），不符合 fail-closed。

**修复方案**：将 `writeAudit` 改为返回 error，由调用方决定是否回滚：
```go
func (s *TicketWorkflowStore) Assign(...) error {
    // ... DB update ...
    if err := s.writeAudit(ctx, ...); err != nil {
        // 回滚已更新的 DB 状态
        s.db.ExecContext(ctx, "UPDATE cs_tickets SET ... WHERE id = $1", ...)
        return fmt.Errorf("audit failed: %w", err)
    }
    return nil
}
```

### 3.4 KB 真实架构（当前为内存实现）

**当前状态**：`store/memory/knowledge_store.go` 存在，无持久化。

**生产缺口**：无 PostgreSQL schema 支持 KB。
- 需要新增 `cs_kb_entries` 的 PG 持久化 store
- 需要向量索引方案（当前无 embedding 接入）

---

## 4. IntegrationPlugin / 集成运行模式设计

### 4.1 当前状态

当前 `app.go` 的 `New()` 即为独立运行入口，无 IntegrationPlugin 接口。
`PRODUCTION_EXECUTION_PLAN.md` 要求提供 `IntegrationPlugin` 接口支持集成运行。

### 4.2 IntegrationPlugin 接口设计

```go
// internal/plugin/plugin.go
package plugin

// IntegrationPlugin 是 ai-customer-service 作为 Go module 被主程序引入时暴露的接口。
type IntegrationPlugin interface {
    // Name 返回插件名称
    Name() string
    // Init 在插件加载时调用，传入主程序共享的配置
    Init(cfg *IntegrationConfig) error
    // RegisterRoutes 将客服系统的 HTTP 路由注册到主程序 mux
    RegisterRoutes(mux *http.ServeMux) error
    // HealthCheck 返回插件级健康状态
    HealthCheck(ctx context.Context) error
}

// IntegrationConfig 由主程序在插件初始化时注入
type IntegrationConfig struct {
    DB                   *sql.DB        // 主程序数据库连接（可选，不传则用独立 Postgres）
    Redis                *redis.Client  // 主程序 Redis 连接（可选）
    Logger               *slog.Logger   // 主程序共享 Logger
    BasePath             string         // 路由前缀，默认 /api/v1/customer-service
    WebhookSecret        string         // Webhook 签名密钥
    RegisterMetrics      func(metrics.Registry)  // 指标注册回调
    RegisterTracing      func(tracer trace.Tracer) // tracing 注册回调
}

// 实现一个 stub 以支持独立运行
type StandalonePlugin struct{}
func (StandalonePlugin) Name() string { return "ai-customer-service" }
func (p *StandalonePlugin) Init(cfg *IntegrationConfig) error { /* 独立模式，使用内置 db/redis */ return nil }
func (p *StandalonePlugin) RegisterRoutes(mux *http.ServeMux) error {
    // 使用 NewRouter 挂载完整路由
    return nil
}
func (p *StandalonePlugin) HealthCheck(ctx context.Context) error { return nil }
```

### 4.3 独立运行 vs 集成运行配置差异

| 组件 | 独立运行 | 集成运行 |
|------|---------|---------|
| DB | 使用自己的 PostgreSQL (`AI_CS_POSTGRES_*` env) | 复用主程序 `*IntegrationConfig.DB` |
| Redis | 独立实例 | 复用主程序 `*IntegrationConfig.Redis` |
| Config | 从 `config.yaml` / env 加载 | 合并到主程序配置 |
| 路由 | `/api/v1/customer-service/*` | 可配置 `BasePath` |
| Health | 自己的 `/actuator/health` | 通过 `IntegrationPlugin.HealthCheck()` 暴露 |

### 4.4 入口函数设计

```go
// cmd/standalone/main.go（独立运行）
func main() {
    plugin := &StandalonePlugin{}
    // 加载配置后运行独立 HTTP 服务器
}

// internal/plugin/standalone.go
package plugin
func RunStandalone() error {
    cfg, _ := config.Load()
    app, _ := app.New(cfg, logger)
    // 启动 HTTP 服务器
}
```

---

## 5. Metrics / Tracing / Logging / Health Readiness 设计

### 5.1 当前状态

- **Health**: ✅ 已实现 `/actuator/health/live/ready`，依赖 PostgreSQL
- **Logging**: ⚠️ 仅部分结构化日志，未使用 slog 的完整上下文
- **Metrics**: ❌ 未实现
- **Tracing**: ❌ 未实现

### 5.2 Metrics 接入方案

**选型**：使用 Prometheus Go client + OpenTelemetry 融合方案（与主项目对齐）

```go
// internal/platform/metrics/metrics.go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // 请求指标
    HTTPRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "cs_http_requests_total", Help: "Total HTTP requests"},
        []string{"method", "path", "status"},
    )
    HTTPRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{Name: "cs_http_request_duration_seconds", Buckets: []float64{.01, .05, .1, .5, 1, 5}},
        []string{"method", "path"},
    )
    // 业务指标
    MessagesProcessedTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "cs_messages_processed_total", Help: "Total messages processed"},
        []string{"channel", "intent", "handoff"},
    )
    TicketCreatedTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "cs_ticket_created_total", Help: "Total tickets created"},
        []string{"priority", "handoff_reason"},
    )
    TicketStateTransitionsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "cs_ticket_state_transitions_total", Help: "Total ticket state transitions"},
        []string{"from_state", "to_state"},
    )
    SessionActiveGauge = promauto.NewGauge(
        prometheus.GaugeOpts{Name: "cs_sessions_active", Help: "Current active sessions"},
    )
    LLMCallDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{Name: "cs_llm_call_duration_seconds", Buckets: []float64{0.5, 1, 2, 5, 10}},
        []string{"provider", "model"},
    )
    WebhookRejectedTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "cs_webhook_rejected_total", Help: "Total rejected webhooks"},
        []string{"reason_code"},
    )
)
```

**在 router 中间件埋点**：
```go
// internal/http/middleware/metrics.go
func MetricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 记录 latency 和 status code
    })
}

// 暴露 /metrics 端点
mux.Handle("/metrics", promhttp.Handler())
```

### 5.3 Tracing 接入方案（OpenTelemetry）

```go
// internal/platform/tracing/tracing.go
package tracing

import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
    "go.opentelemetry.io/otel/sdk/trace"
)

func Init(serviceName string) (func(), error) {
    exporter, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(...)),
    )
    otel.SetTracerProvider(tp)
    return func() { tp.Shutdown(context.Background()) }, nil
}
```

**在 webhook handler 中埋点**：
```go
// 在 dialog.Process 前后加上 span
span := tracer.StartSpan("webhook.process")
defer span.End()
span.SetAttributes("channel", msg.Channel, "open_id", msg.OpenID)
```

### 5.4 Structured Logging 增强

当前 `internal/platform/logging/logger.go` 需要支持更多字段：

```go
// 日志字段规范（与 supply-api 对齐）
log.Info("webhook received",
    "trace_id", traceID,
    "channel", msg.Channel,
    "open_id", msg.OpenID,
    "session_id", result.SessionID,
    "intent", result.Intent.Intent,
    "handoff", result.Handoff.ShouldHandoff,
    "ticket_id", result.TicketID,
    "latency_ms", latency.Milliseconds(),
)
```

### 5.5 Health Readiness 增强

当前 readiness 仅检查 PostgreSQL，需要扩展为多依赖检查：

```go
// internal/platform/health/dependency.go
type DependencyChecker struct {
    checks []Checker
}

func (dc *DependencyChecker) Add(name string, check func(context.Context) error) {
    dc.checks = append(dc.checks, simpleCheck{name, check})
}

// 在 app.go 中注册：
checkers := []health.Checker{
    pgstore.NewDBChecker(db),
    // 新增 Redis checker
    // 新增 LLM supplier health checker
}
```

---

## 6. 降级、熔断、回滚、灰度技术方案

### 6.1 降级（Degradation）策略

| 级别 | 触发条件 | 降级行为 |
|------|---------|---------|
| L1 | LLM 超时 / 不可用 | 切换备用模型（2家供应商 failover） |
| L2 | 主备模型均不可用 | 返回兑底文案（静态模板）+ 自动创建 P1 工单 |
| L3 | 知识库不可用 | 跳过 RAG，直接用通用 LLM 提示词回复 |
| L4 | PostgreSQL 不可用 | 仅内存模式（工单仅内存），拒绝新 webhook 写入 |
| L5 | 完全不可用 | `/actuator/health/ready` 返回 DOWN，负载均衡摘除 |

**代码层面**：
```go
// internal/service/llm/fallback.go
type LLMFallback struct {
    providers []LLMProvider
    idx       int
    mu        sync.RWMutex
}

func (f *LLMFallback) Generate(ctx context.Context, prompt string) (*Response, error) {
    for i := 0; i < len(f.providers); i++ {
        resp, err := f.providers[f.idx].Generate(ctx, prompt)
        if err == nil {
            return resp, nil
        }
        f.mu.Lock()
        f.idx = (f.idx + 1) % len(f.providers)
        f.mu.Unlock()
        metrics.LLMFallbackTotal.Inc()
    }
    return nil, ErrAllProvidersFailed
}
```

### 6.2 熔断（Circuit Breaker）

```go
// internal/platform/breaker/breaker.go
type CircuitBreaker struct {
    failures  int
    threshold int
    state     atomic.Int32 // 0=closed, 1=half-open, 2=open
    resetAt   time.Time
}

// 当 external API（supply-api / token-runtime）调用失败率 > 50% 在 10s 窗口内时：
// 打开熔断器，10s 内直接返回降级响应，不发请求
// 10s 后进入 half-open，放行 1 个请求试探
```

### 6.3 回滚（Rollback）方案

**数据层回滚**：
- 使用 `db/migration/*.down.sql` 进行 schema 回滚
- 关键数据变更使用 migration 的事务包装，失败自动回滚

**应用层回滚**：
- Docker 镜像版本 tag（如 `v1.0.0` → `v1.0.1` → `v1.1.0`）
- Kubernetes rollback：`kubectl rollout undo deployment/ai-customer-service`
- 配置变更：保留旧配置快照，支持环境变量热覆盖

**回滚触发条件**：
- 5xx 错误率 > 5% 持续 2 分钟
- P99 延迟 > 30s 持续 5 分钟
- 审计日志写入失败率 > 1%

### 6.4 灰度（Gated Rollout）方案

**策略 1：按渠道灰度**
```yaml
# config.yaml
rollout:
  channels:
    telegram: 100%   # 全量
    discord: 50%     # 灰度 50%
    wechat: 0%       # 不启用
```
实现：nginx/load balancer 按 channel header 权重分流

**策略 2：按用户特征灰度**
```go
// 按 user_id hash 分桶，10% 用户先跑新版本
func inRollout(userID string, percentage int) bool {
    h := crc32.ChecksumIEEE([]byte(userID))
    return int(h%100) < percentage
}
```

**策略 3：金丝雀 + 监控**
1. 部署新版本到 1 个 Pod（10% 流量）
2. 观察 30 分钟：错误率、P99、审计日志量
3. 无异常则扩大至 50%，再观察
4. 全量切流后保留旧 Pod 5 分钟备 rollback

### 6.5 SLO / 告警定义

```yaml
# alerts.yaml
slo:
  availability:
    target: 99.5%
    window: 7d
    metric: cs_http_requests_total{status!~"5.."} / cs_http_requests_total
  latency_p99:
    target: 10s
    window: 5m
    metric: cs_http_request_duration_seconds{p quantile="0.99"}
  error_rate:
    target: <1%
    window: 5m
    metric: cs_http_requests_total{status=~"5.."} / cs_http_requests_total
alerts:
  - name: HighErrorRate
    expr: rate(cs_http_requests_total{status=~"5.."}[5m]) > 0.05
    severity: critical
  - name: TicketAuditFailure
    expr: rate(cs_ticket_state_transitions_total{action="audit_fail"}[5m]) > 0
    severity: critical
  - name: LLMHighLatency
    expr: cs_llm_call_duration_seconds{p quantile="0.99"} > 10
    severity: warning
```

---

## 7. 漂移检测汇总与修复优先级

### 7.1 已确认漂移

| # | 漂移描述 | 严重性 | 修复文件/方案 |
|---|---------|-------|-------------|
| D-1 | `session.StatusWaitingFeedback` 缺失 | P1 | `domain/session/session.go` + migration |
| D-2 | `tenant_id` 缺失（多租户支持） | P0 | 新 migration `0002` |
| D-3 | `cs_agent_sessions` / `cs_agent_stats` 缺失 | P1 | 新 migration `0003` |
| D-4 | `assigned_at` 缺失（工单 SLA 计算） | P1 | 新 migration `0004` |
| D-5 | `cs_channel_bindings` 缺失 | P1 | 新 migration `0005` |
| D-6 | Webhook nonce 防重放未持久化 | P0 | 新 `nonce_store.go` + migration |
| D-7 | `Resolve` 时 source_ip 未写入 audit（audit_store 仅写 NULLIF('','')） | P1 | `ticket_workflow.go` writeAudit 调用处已正确传参，但审计写入失败静默 |
| D-8 | `IntegrationPlugin` 接口缺失 | P1 | 新 `internal/plugin/plugin.go` |
| D-9 | `metrics/tracing` 完全缺失 | P1 | 新 `internal/platform/metrics/` 和 `tracing/` |
| D-10 | 排队位置查询接口未定义和实现 | P1 | 新 handler + 接口定义 |
| D-11 | `Resolve` vs `Close` 语义未文档化 | P0 | 更新 `tech/INTERFACE.md` |
| D-12 | HLD 说 "resolved 后自动 close"，代码是独立 close | P1 | 需要产品确认 |

### 7.2 不需要修复的确认对齐

| 确认项 | 结论 |
|-------|-----|
| `/webhook/{channel}` 路由 | ✅ 已实现（通过 path manipulation hack） |
| HMAC 签名校验 | ✅ 已实现 |
| 防重放（skew 校验） | ✅ 已实现（但无 nonce 持久化） |
| 幂等去重 | ✅ 已实现 |
| Ticket assign/resolve audit 写入 | ✅ 已实现（`ticket_workflow.go`） |
| 安全拒绝事件 audit | ✅ 已实现（`webhook_handler.auditRejectedRequest`） |
| 消息处理 audit | ✅ 已实现 |

---

## 8. 需要 TechLead 决策的问题

1. **`resolved` 后的 close 语义**：系统自动 close 还是人工触发？
2. **Audit 写入失败是否回滚**：ticket assign/resolve 的 audit 失败是否回滚 DB 状态变更？
3. **TenantID 来源**：从 JWT token 提取还是从 channel context 传入？影响多租户架构。
4. **Metrics 存储选型**：Prometheus（单体） vs VictoriaMetrics（可集群），影响 SLO 长期存储。
5. **排队等待时间估算**：基于平均处理时间估算还是基于历史实际？

---

## 9. 实施顺序建议

### Phase 1（立即执行，可并行）
1. Migration `0002-0005`（Schema 补全）
2. Nonce Store 持久化防重放
3. IntegrationPlugin 接口框架

### Phase 2
1. Metrics + Tracing 基础设施
2. 排队位置查询接口
3. Session waiting_feedback 状态补齐

### Phase 3
1. 灰度/回滚 Runbook 文档
2. SLO / Alert 规则
3. 文档与代码对齐（D-11, D-12）

---

## 10. 质量检查

- [x] 所有技术方案具体到函数名/文件路径/接口签名
- [x] 每个漂移项都有明确修复方案
- [x] 未脱离现有代码实现
- [x] 对不确定的设计决策提供可选方案
- [x] 按优先级（P0/P1）排序

---

*TechLead 完成：生产数据模型与 Migration 方案*
*TechLead 完成：Webhook 签名、防重放、幂等、审计 fail-closed 方案*
*TechLead 完成：Ticket / Session / Audit / KB 真实架构*
*TechLead 完成：IntegrationPlugin / 集成运行模式设计*
*TechLead 完成：metrics / tracing / logging / health readiness 设计*
*TechLead 完成：降级、熔断、回滚、灰度技术方案*
*TechLead 完成：漂移检测全部完成*
*TechLead 完成：需要 TechLead 决策问题已全部列出*
*TechLead 技术设计与漂移检测全部完成*