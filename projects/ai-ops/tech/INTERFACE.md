# AI-Ops 核心接口设计

> 版本：v1.0 | 状态：初稿

---

## 1. 内部模块间接口

### 1.1 MetricService

```go
type MetricService interface {
    // 采集指标
    Collect(ctx context.Context, source string, metrics []MetricPoint) error
    // 查询时序数据
    Query(ctx context.Context, req MetricQueryRequest) (*MetricQueryResult, error)
    // 获取最新值
    GetLatest(ctx context.Context, source, metricName string) (*MetricPoint, error)
    // 存储保留期检查
    PurgeExpired(ctx context.Context, before time.Time) (int64, error)
}

type MetricPoint struct {
    Source    string
    Name      string
    Value     float64
    Tags      map[string]string
    Timestamp time.Time
}

type MetricQueryRequest struct {
    Source    string
    Name      string
    StartTime time.Time
    EndTime   time.Time
    Interval  time.Duration // 聚合间隔
    Tags      map[string]string
}

type MetricQueryResult struct {
    Points []MetricPoint
}
```

### 1.2 AlertService

```go
type AlertService interface {
    // 规则 CRUD
    CreateRule(ctx context.Context, rule AlertRule) (*AlertRule, error)
    UpdateRule(ctx context.Context, rule AlertRule) (*AlertRule, error)
    DeleteRule(ctx context.Context, ruleID string) error
    GetRule(ctx context.Context, ruleID string) (*AlertRule, error)
    ListRules(ctx context.Context, filter RuleFilter) ([]AlertRule, error)

    // 告警事件管理
    ListAlerts(ctx context.Context, filter AlertFilter) ([]AlertEvent, error)
    Acknowledge(ctx context.Context, alertID, actorID string) error
    Ignore(ctx context.Context, alertID, actorID string) error
    Escalate(ctx context.Context, alertID, reason string) error

    // 实时评估
    Evaluate(ctx context.Context, ruleID string) (*AlertEvent, error)
}

type AlertRule struct {
    ID             string
    Name           string
    MetricSource   string
    MetricName     string
    ThresholdType  string // > < = regex
    ThresholdValue string
    DurationMin    int
    Level          string // P0 P1 P2 P3
    ChannelIDs     []string
    HealingAction  *string
    HealingConfig  map[string]any
    IsSandboxed    bool
    Enabled        bool
    Version        int
}

type AlertEvent struct {
    ID              string
    RuleID          string
    Level           string
    ResourceType    string
    ResourceID      string
    CurrentValue    string
    ThresholdValue  string
    Status          string // triggered notified healing resolved escalated acknowledged
    IsAggregated    bool
    AggregatedCount int
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

### 1.3 HealingService

```go
type HealingService interface {
    // 执行自愈动作
    Execute(ctx context.Context, action HealingAction, target ResourceTarget) (*HealingResult, error)
    // 获取可用动作列表
    ListActions(ctx context.Context) []HealingActionMeta
    // 回滚自愈动作
    Rollback(ctx context.Context, executionID string) error
    // 查询执行历史
    ListExecutions(ctx context.Context, filter ExecutionFilter) ([]HealingExecution, error)
}

type HealingAction struct {
    Type   string // restart_service switch_provider throttle isolate_node
    Config map[string]any
}

type ResourceTarget struct {
    Type string // service provider model
    ID   string
}

type HealingResult struct {
    ExecutionID string
    Success     bool
    BeforeState map[string]any
    AfterState  map[string]any
    Error       *string
    ExecutedAt  time.Time
}
```

### 1.4 AuditService

```go
type AuditService interface {
    // 记录审计事件
    Record(ctx context.Context, event AuditEvent) error
    // 查询审计日志
    Query(ctx context.Context, filter AuditFilter) ([]AuditEvent, error)
    // 回滚操作
    Rollback(ctx context.Context, eventID string, actorID string) (*AuditEvent, error)
    // 影响面计算
    CalculateImpact(ctx context.Context, objectType, objectID string, proposedState map[string]any) (*ImpactReport, error)
}

type AuditEvent struct {
    EventID     string
    TenantID    string
    ObjectType  string
    ObjectID    string
    Action      string // create update delete rollback
    BeforeState map[string]any
    AfterState  map[string]any
    RequestID   string
    ResultCode  string
    SourceIP    string
    ActorID     string
    CreatedAt   time.Time
}

type ImpactReport struct {
    RiskLevel       string  // low medium high
    EstimatedRejectRate float64 // 预估拒绝率
    AffectedResources []string
    RequiresConfirm   bool
}
```

### 1.5 CapacityService

```go
type CapacityService interface {
    // 获取容量视图
    GetDashboard(ctx context.Context, scope CapacityScope) (*CapacityDashboard, error)
    // 增长率预测
    PredictGrowth(ctx context.Context, metric string, horizon time.Duration) (*GrowthPrediction, error)
    // 设置容量阈值
    SetThreshold(ctx context.Context, metric string, threshold float64) error
}

type CapacityDashboard struct {
    Metrics      []CapacityMetric
    Predictions  []GrowthPrediction
    LastUpdated  time.Time
}

type CapacityMetric struct {
    Name      string
    Current   float64
    Limit     float64
    Unit      string
    Utilization float64
}

type GrowthPrediction struct {
    Metric        string
    DailyGrowth   float64
    DaysToLimit   *int // nil 表示不会达到上限
}
```

---

## 2. 外部系统集成接口

### 2.1 与 Bridge Gateway 集成

| 方法 | 路径 | 请求 | 响应 | 说明 |
|------|------|------|------|------|
| 查询服务状态 | `GET /internal/gateway/health` | - | `{"status":"up","services":{}}` | 诊断时查询各服务健康状态 |
| 获取路由策略 | `GET /internal/gateway/routes` | - | `{"routes":[]}` | 读取当前路由配置，用于影响面分析 |
| 修改路由策略 | `POST /internal/gateway/routes` | `{"action":"switch_provider","target":"","config":{}}` | `{"success":true}` | 自愈动作调用，需审计 |
| 获取请求量统计 | `GET /internal/gateway/metrics` | `?metric=qps&duration=5m` | `{"value":1234.5}` | 采集指标数据 |

### 2.2 与 supply-api 集成

| 方法 | 路径 | 请求 | 响应 | 说明 |
|------|------|------|------|------|
| 查询供应商状态 | `GET /internal/supply/accounts/health` | - | `{"accounts":[]}` | 诊断供应商健康状态 |
| 获取审计日志格式 | `GET /internal/supply/audit/schema` | - | `{"schema":{}}` | 确保审计事件格式一致 |

### 2.3 与 platform-token-runtime 集成

| 方法 | 路径 | 请求 | 响应 | 说明 |
|------|------|------|------|------|
| 获取 Token 消耗 | `GET /internal/runtime/token-usage` | `?window=1h` | `{"total":12345,"by_model":{}}` | 采集 Token 消耗指标 |
| 获取容量使用率 | `GET /internal/runtime/capacity` | - | `{"utilization":0.75}` | 采集容量指标 |

---

## 3. API 接口规范

### 3.1 REST API 基础

- **基础路径**: `/api/v1/ai-ops/`
- **内部路径** (集成模式): `/internal/ai-ops/`
- **内容类型**: `application/json`
- **错误响应格式**:

```json
{
  "error": {
    "code": "OPS_ALT_4001",
    "message": "告警规则不存在",
    "details": {}
  }
}
```

### 3.2 核心端点

#### 告警规则管理

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/ai-ops/rules` | 列表告警规则 |
| POST | `/api/v1/ai-ops/rules` | 创建规则 |
| GET | `/api/v1/ai-ops/rules/{id}` | 获取规则 |
| PUT | `/api/v1/ai-ops/rules/{id}` | 更新规则（乐观锁 version） |
| DELETE | `/api/v1/ai-ops/rules/{id}` | 删除规则 |
| POST | `/api/v1/ai-ops/rules/{id}/evaluate` | 手动触发规则评估 |

#### 告警事件

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/ai-ops/alerts` | 列表告警事件 |
| POST | `/api/v1/ai-ops/alerts/{id}/ack` | 确认告警 |
| POST | `/api/v1/ai-ops/alerts/{id}/ignore` | 忽略告警 |
| POST | `/api/v1/ai-ops/alerts/{id}/escalate` | 升级告警 |

#### 自愈动作

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/ai-ops/healing/actions` | 列表可用自愈动作 |
| POST | `/api/v1/ai-ops/healing/execute` | 执行自愈动作（人工触发） |
| POST | `/api/v1/ai-ops/healing/{execution_id}/rollback` | 回滚自愈动作 |
| GET | `/api/v1/ai-ops/healing/executions` | 查询执行历史 |

#### 审计与配置

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/ai-ops/audit` | 查询审计日志 |
| POST | `/api/v1/ai-ops/audit/{id}/rollback` | 回滚配置变更 |
| GET | `/api/v1/ai-ops/capacity` | 获取容量大盘 |

### 3.3 错误码定义

| 错误码 | HTTP 状态 | 说明 |
|---------|-----------|------|
| `OPS_ALT_4001` | 404 | 告警规则不存在 |
| `OPS_ALT_4002` | 409 | 规则名称已存在 |
| `OPS_ALT_4003` | 400 | 规则参数无效 |
| `OPS_ALT_4101` | 400 | 回滚目标不存在 |
| `OPS_ALT_4102` | 409 | 回滚目标已被后续修改覆盖 |
| `OPS_HEAL_4001` | 400 | 自愈动作类型不支持 |
| `OPS_HEAL_4002` | 409 | 自愈动作正在执行中 |
| `OPS_HEAL_4003` | 400 | 回滚目标执行不存在 |
| `OPS_AUD_4001` | 403 | 无权进行审计操作 |
| `OPS_AUD_4101` | 400 | 回滚目标资源不存在 |
| `OPS_CAP_4001` | 400 | 容量指标不存在 |

### 3.4 WebSocket 接口

**路径**: `/ws/v1/ai-ops/alerts`

- 客户端订阅后，实时推送新告警事件。
- 支持按级别过滤：`?levels=P0,P1`。
- 心跳间隔 30 秒。
