# 路由策略模板设计文档 (v1)

- 版本：v1.0
- 日期：2026-04-02
- 目标阶段：P1（Router Core 策略层扩展）
- 关联文档：
  - `router_core_takeover_execution_plan_v3_2026-03-17.md`
  - `router_core_takeover_metrics_sql_dashboard_v1_2026-03-17.md`
  - `acceptance_gate_single_source_v1_2026-03-18.md`

---

## 1. 背景与目标

### 1.1 业务背景

立交桥项目（LLM Gateway）在 S2 阶段需要实现 Router Core 主路径接管率指标：

| 指标ID | 指标名称 | 目标值 | 验收条件 |
|--------|----------|--------|----------|
| M-006 | overall_takeover_pct | >= 60% | 全供应商主路径接管率 |
| M-007 | cn_takeover_pct | = 100% | 国内供应商主路径接管率 |
| M-008 | route_mark_coverage_pct | >= 99.9% | 路由标记覆盖率 |

当前 Router Core 仅支持简单的负载均衡策略（latency/round_robin/weighted/availability），无法满足基于模型、成本、质量、成本权衡的复杂路由需求。

### 1.2 设计目标

1. **策略配置化**：通过模板+参数实现路由策略定义，支持动态调整
2. **多维度决策**：支持基于模型、成本、质量、成本的路由决策
3. **Fallback 完善**：建立多级 Fallback 机制保障可用性
4. **可观测性**：与现有 ratelimit、alert 机制无缝集成
5. **可测试性**：策略可量化、可回放、可测试

---

## 2. 现有架构分析

### 2.1 现有组件

| 组件 | 路径 | 功能 |
|------|------|------|
| Router | `gateway/internal/router/router.go` | 负载均衡策略选择 |
| Adapter | `gateway/internal/adapter/adapter.go` | Provider 抽象接口 |
| OpenAIAdapter | `gateway/internal/adapter/openai_adapter.go` | OpenAI 协议实现 |
| RateLimiter | `gateway/internal/ratelimit/ratelimit.go` | TokenBucket/SlidingWindow 限流 |
| Alert | `gateway/internal/alert/alert.go` | 多渠道告警发送 |

### 2.2 现有 Router 核心接口

```go
// Router 接口 (adapter.go)
type Router interface {
    SelectProvider(ctx context.Context, model string) (ProviderAdapter, error)
    GetFallbackProviders(ctx context.Context, model string) ([]ProviderAdapter, error)
    RecordResult(ctx context.Context, provider string, success bool, latencyMs int64)
}
```

### 2.3 现有策略类型

```go
type LoadBalancerStrategy string
const (
    StrategyLatency     LoadBalancerStrategy = "latency"      // 最低延迟
    StrategyRoundRobin  LoadBalancerStrategy = "round_robin"  // 轮询
    StrategyWeighted    LoadBalancerStrategy = "weighted"     // 权重
    StrategyAvailability LoadBalancerStrategy = "availability" // 最低失败率
)
```

---

## 3. 路由策略模板设计

### 3.1 策略模板类型

#### 3.1.1 策略类型枚举

```go
// RoutingStrategyType 路由策略类型
type RoutingStrategyType string

const (
    // 基于成本
    StrategyCostBased        RoutingStrategyType = "cost_based"        // 最小成本
    StrategyCostAwareBalanced RoutingStrategyType = "cost_aware_balanced" // 成本权衡均衡

    // 基于质量
    StrategyQualityFirst     RoutingStrategyType = "quality_first"     // 最高质量
    StrategyQualityAware      RoutingStrategyType = "quality_aware"      // 质量感知

    // 基于延迟
    StrategyLatencyFirst      RoutingStrategyType = "latency_first"     // 最低延迟
    StrategyLatencyAware      RoutingStrategyType = "latency_aware"      // 延迟感知

    // 基于模型
    StrategyModelSpecific    RoutingStrategyType = "model_specific"    // 模型特定
    StrategyModelBalanced     RoutingStrategyType = "model_balanced"    // 模型均衡

    // 复合策略
    StrategyComposite         RoutingStrategyType = "composite"        // 复合策略
)
```

#### 3.1.2 策略模板结构

```go
// RoutingStrategyTemplate 路由策略模板
type RoutingStrategyTemplate struct {
    // 模板唯一标识
    ID string `json:"id"`

    // 模板名称
    Name string `json:"name"`

    // 策略类型
    Type RoutingStrategyType `json:"type"`

    // 策略参数
    Params StrategyParams `json:"params"`

    // 适用模型列表 (空表示全部)
    ApplicableModels []string `json:"applicable_models"`

    // 适用供应商列表 (空表示全部)
    ApplicableProviders []string `json:"applicable_providers"`

    // 优先级 (数字越小优先级越高)
    Priority int `json:"priority"`

    // 是否启用
    Enabled bool `json:"enabled"`

    // 描述
    Description string `json:"description"`

    // 灰度发布配置 (可选)
    RolloutConfig *RolloutConfig `json:"rollout_config,omitempty"`

    // A/B测试配置 (可选)
    ABConfig *ABTestConfig `json:"ab_config,omitempty"`
}

// RolloutConfig 灰度发布配置
type RolloutConfig struct {
    // 是否启用灰度
    Enabled bool `json:"enabled"`

    // 当前灰度百分比 (0-100)
    Percentage int `json:"percentage"`

    // 最大灰度百分比
    MaxPercentage int `json:"max_percentage"`

    // 每次增加百分比
    Increment int `json:"increment"`

    // 增加间隔
    IncrementInterval time.Duration `json:"increment_interval"`

    // 灰度规则 (用于特定用户/场景)
    Rules []RolloutRule `json:"rules,omitempty"`

    // 灰度开始时间
    StartTime *time.Time `json:"start_time,omitempty"`
}

// RolloutRule 灰度规则
type RolloutRule struct {
    // 规则类型: user_id, tenant_id, region, model
    Type string `json:"type"`

    // 规则值
    Values []string `json:"values"`

    // 是否强制启用
    Force bool `json:"force"`
}

// ABTestConfig A/B测试配置
type ABTestConfig struct {
    // 实验ID
    ExperimentID string `json:"experiment_id"`

    // 实验组ID
    ExperimentGroupID string `json:"experiment_group_id"`

    // 对照组ID
    ControlGroupID string `json:"control_group_id"`

    // 流量分配比例 (实验组百分比)
    TrafficSplit int `json:"traffic_split"` // 0-100

    // 分桶Key (用于一致性哈希)
    BucketKey string `json:"bucket_key"`

    // 实验开始时间
    StartTime *time.Time `json:"start_time,omitempty"`

    // 实验结束时间
    EndTime *time.Time `json:"end_time,omitempty"`

    // 实验假设
    Hypothesis string `json:"hypothesis,omitempty"`

    // 成功指标
    SuccessMetrics []string `json:"success_metrics,omitempty"`
}

// ABStrategyTemplate A/B测试策略模板
type ABStrategyTemplate struct {
    RoutingStrategyTemplate

    // 控制组策略 (原有策略)
    ControlStrategy *RoutingStrategyTemplate `json:"control_strategy"`

    // 实验组策略 (新策略)
    ExperimentStrategy *RoutingStrategyTemplate `json:"experiment_strategy"`

    // A/B配置
    Config ABTestConfig `json:"config"`
}

// ShouldApplyToRequest 判断请求是否应该使用实验组策略
func (t *ABStrategyTemplate) ShouldApplyToRequest(req *RoutingRequest) bool {
    if !t.Enabled || t.Config.ExperimentID == "" {
        return false
    }

    // 检查时间范围
    now := time.Now()
    if t.Config.StartTime != nil && now.Before(*t.Config.StartTime) {
        return false
    }
    if t.Config.EndTime != nil && now.After(*t.Config.EndTime) {
        return false
    }

    // 一致性哈希分桶
    bucket := hashString(fmt.Sprintf("%s:%s", t.Config.BucketKey, req.UserID)) % 100
    return bucket < t.Config.TrafficSplit
}

// hashString 计算字符串哈希值 (用于一致性分桶)
func hashString(s string) int {
    h := fnv.New32a()
    h.Write([]byte(s))
    return int(h.Sum32())
}

// StrategyParams 策略参数
type StrategyParams struct {
    // 成本参数
    CostParams *CostParams `json:"cost_params,omitempty"`

    // 质量参数
    QualityParams *QualityParams `json:"quality_params,omitempty"`

    // 延迟参数
    LatencyParams *LatencyParams `json:"latency_params,omitempty"`

    // 模型参数
    ModelParams *ModelParams `json:"model_params,omitempty"`

    // Fallback 配置
    FallbackConfig *FallbackConfig `json:"fallback_config,omitempty"`

    // 复合策略子策略
    SubStrategies []StrategyParams `json:"sub_strategies,omitempty"`
}
```

### 3.2 成本策略模板 (Cost-Based)

#### 3.2.1 最小成本策略

```go
// CostParams 成本参数
type CostParams struct {
    // 成本上限 (单位: 分/1K tokens)
    MaxCostPer1KTokens float64 `json:"max_cost_per_1k_tokens"`

    // 优先使用低成本供应商
    PreferLowCost bool `json:"prefer_low_cost"`

    // 成本权重 (0.0-1.0)
    CostWeight float64 `json:"cost_weight"`
}

// CostBasedTemplate 成本策略模板
type CostBasedTemplate struct {
    RoutingStrategyTemplate
    Params CostParams
}

// SelectProvider 实现
func (t *CostBasedTemplate) SelectProvider(ctx context.Context, req *RoutingRequest) (*RoutingDecision, error) {
    candidates := t.filterCandidates(req)

    if len(candidates) == 0 {
        return nil, ErrNoProviderAvailable
    }

    // 按成本排序
    sort.Slice(candidates, func(i, j int) bool {
        return candidates[i].CostPer1KTokens < candidates[j].CostPer1KTokens
    })

    // 选择成本最低且可用的
    for _, c := range candidates {
        if c.IsAvailable && c.CostPer1KTokens <= t.Params.MaxCostPer1KTokens {
            return &RoutingDecision{
                Provider:       c.Name,
                Strategy:       t.Type,
                CostPer1KTokens: c.CostPer1KTokens,
                EstimatedLatency: c.LatencyMs,
            }, nil
        }
    }

    return nil, ErrNoAffordableProvider
}
```

#### 3.2.2 成本权衡均衡策略

```go
// CostAwareBalancedParams 成本权衡参数
type CostAwareBalancedParams struct {
    // 成本权重
    CostWeight float64 `json:"cost_weight"` // 0.0-1.0

    // 质量权重
    QualityWeight float64 `json:"quality_weight"` // 0.0-1.0

    // 延迟权重
    LatencyWeight float64 `json:"latency_weight"` // 0.0-1.0

    // 成本上限
    MaxCostPer1KTokens float64 `json:"max_cost_per_1k_tokens"`

    // 延迟上限 (ms)
    MaxLatencyMs int64 `json:"max_latency_ms"`

    // 最低质量分数
    MinQualityScore float64 `json:"min_quality_score"`
}
```

### 3.3 质量策略模板 (Quality-Based)

```go
// QualityParams 质量参数
type QualityParams struct {
    // 质量评分 (0.0-1.0)
    QualityScore float64 `json:"quality_score"`

    // 最低质量门槛
    MinQualityThreshold float64 `json:"min_quality_threshold"`

    // 质量权重
    QualityWeight float64 `json:"quality_weight"`

    // 质量评估指标
    QualityMetrics []QualityMetric `json:"quality_metrics"`
}

// QualityMetric 质量评估指标
type QualityMetric struct {
    Name string  `json:"name"`
    Weight float64 `json:"weight"` // 权重
    Score float64 `json:"score"`   // 评分
}

// QualityFirstTemplate 质量优先策略模板
type QualityFirstTemplate struct {
    RoutingStrategyTemplate
    Params QualityParams
}
```

### 3.4 模型特定策略模板

```go
// ModelParams 模型参数
type ModelParams struct {
    // 模型到供应商的映射
    ModelProviderMapping map[string][]ModelProviderConfig `json:"model_provider_mapping"`

    // 默认供应商
    DefaultProvider string `json:"default_provider"`

    // 模型组
    ModelGroups map[string][]string `json:"model_groups"`
}

// ModelProviderConfig 模型供应商配置
type ModelProviderConfig struct {
    ProviderName string  `json:"provider_name"`
    Priority      int    `json:"priority"`      // 优先级
    Weight        float64 `json:"weight"`        // 权重
    FallbackOnly  bool   `json:"fallback_only"` // 仅作 Fallback
}

// ModelSpecificTemplate 模型特定策略模板
type ModelSpecificTemplate struct {
    RoutingStrategyTemplate
    Params ModelParams
}
```

### 3.5 复合策略模板

```go
// CompositeParams 复合策略参数
type CompositeParams struct {
    // 子策略列表
    Strategies []StrategyConfig `json:"strategies"`

    // 组合方式
    CombineMode CombineMode `json:"combine_mode"`
}

// StrategyConfig 策略配置
type StrategyConfig struct {
    StrategyID   string  `json:"strategy_id"`
    Weight        float64 `json:"weight"`         // 权重 (用于加权评分)
    FallbackTier  int     `json:"fallback_tier"`   // Fallback 层级
}

// CombineMode 组合模式
type CombineMode string

const (
    // 加权评分
    CombineWeightedScore CombineMode = "weighted_score"
    // 优先级链
    CombinePriorityChain CombineMode = "priority_chain"
    // 条件分支
    CombineConditional   CombineMode = "conditional"
)

// CompositeTemplate 复合策略模板
type CompositeTemplate struct {
    RoutingStrategyTemplate
    Params CompositeParams
}
```

---

## 4. Fallback 策略设计

### 4.1 多级 Fallback 架构

```go
// FallbackConfig Fallback 配置
type FallbackConfig struct {
    // Fallback 层级
    Tiers []FallbackTier `json:"tiers"`

    // 最大重试次数
    MaxRetries int `json:"max_retries"`

    // 重试间隔
    RetryIntervalMs int64 `json:"retry_interval_ms"`

    // 是否启用快速失败
    FailFast bool `json:"fail_fast"`

    // Fallback 条件
    Conditions *FallbackConditions `json:"conditions,omitempty"`
}

// FallbackTier Fallback 层级
type FallbackTier struct {
    // 层级编号 (1, 2, 3, ...)
    Tier int `json:"tier"`

    // 触发条件
    Trigger *FallbackTrigger `json:"trigger,omitempty"`

    // 该层级的 Provider 列表
    Providers []string `json:"providers"`

    // 超时时间 (ms)
    TimeoutMs int64 `json:"timeout_ms"`
}

// FallbackTrigger Fallback 触发条件
type FallbackTrigger struct {
    // 错误类型
    ErrorTypes []string `json:"error_types,omitempty"`

    // 延迟阈值 (ms)
    LatencyThresholdMs int64 `json:"latency_threshold_ms,omitempty"`

    // 失败率阈值
    FailureRateThreshold float64 `json:"failure_rate_threshold,omitempty"`

    // 状态码
    StatusCodes []int `json:"status_codes,omitempty"`
}

// FallbackConditions Fallback 条件
type FallbackConditions struct {
    // 需要 Fallback 的错误类型
    RetryableErrors []string `json:"retryable_errors"`

    // 不可重试的错误类型 (直接失败)
    NonRetryableErrors []string `json:"non_retryable_errors"`

    // 需要手动确认的错误
    ManualInterventionErrors []string `json:"manual_intervention_errors"`
}
```

### 4.2 Fallback 执行流程

```
请求进入
    │
    ▼
┌─────────────────┐
│ 选择主策略 Provider │
└────────┬────────┘
         │
    ┌────▼────┐
    │ 调用成功？ │
    └────┬────┘
    是   │   否
    │   ├──────────────────────┐
    ▼   ▼                      ▼
┌─────────┐  ┌───────────────│───────────────┐
│ 返回响应 │  │ 检查 Fallback 条件              │
└─────────┘  └────┬───────────────────────────┘
                  │
             ┌────▼────┐
             │ 触发条件？ │
             └────┬────┘
             是   │   否
             │    │
    ┌────────▼──┐ │
    │ 执行 Tier1 │─┼──► 返回错误
    │ Fallback  │ │
    └────┬──────┘ │
         │        │
    ┌────▼────┐   │
    │ 调用成功？│   │
    └────┬────┘   │
    是   │   否   │
    │    ├───────┼───────┐
    ▼    │       │       │
┌─────────┐ │   │       │
│ 返回响应 │ │   │       │
└─────────┘ │   │       │
           ▼   ▼       ▼
      ┌──────────────│──────────┐
      │ 执行后续 Tier Fallback      │
      └──────────────────────────┘
```

### 4.3 Fallback 与 Ratelimit 集成

#### 4.3.1 集成设计

Fallback与Ratelimit的集成需要考虑以下场景：

| 场景 | 限流策略 | 说明 |
|------|----------|------|
| 主请求限流 | 使用主限流器 | 正常请求使用主限流器配额 |
| Fallback请求限流(ReuseMainQuota=true) | 复用主限流器 | Fallback请求复用主请求未消耗的配额 |
| Fallback请求限流(ReuseMainQuota=false) | 使用独立限流器 | Fallback使用独立的fallback_rpm/fallback_tpm配额 |
| Tier降级限流 | 逐级递减 | 每层Tier使用更低的限流阈值 |

#### 4.3.2 Fallback限流执行流程

```
主请求限流检查
    │
    ├─ 通过 → 执行主Provider
    │           │
    │           ├─ 成功 → 返回响应
    │           │
    │           └─ 失败 → 检查Fallback条件
    │                       │
    │                       ├─ ReuseMainQuota=true → 继续使用主配额检查
    │                       │                       │
    │                       │                       ├─ 通过 → 执行Fallback
    │                       │                       │
    │                       │                       └─ 不通过 → 返回限流错误
    │                       │
    │                       └─ ReuseMainQuota=false → 使用Fallback独立配额
    │                                                   │
    │                                                   ├─ 通过 → 执行Fallback
    │                                                   │
    │                                                   └─ 不通过 → 返回限流错误
    │
    └─ 不通过 → 直接返回限流错误
```

#### 4.3.3 代码实现

```go
// FallbackRateLimitConfig Fallback 限流配置
type FallbackRateLimitConfig struct {
    // 独立的 Fallback 限流 Key 前缀
    KeyPrefix string `json:"key_prefix"`

    // Fallback 请求的独立 RPM 限制
    FallbackRPM int `json:"fallback_rpm"`

    // Fallback 请求的独立 TPM 限制
    FallbackTPM int `json:"fallback_tpm"`

    // 是否复用主请求的限流配额
    ReuseMainQuota bool `json:"reuse_main_quota"`
}

// FallbackRateLimiter Fallback 限流器
type FallbackRateLimiter struct {
    mainLimiter     *ratelimit.TokenBucketLimiter
    fallbackLimiter *ratelimit.TokenBucketLimiter
    config          FallbackRateLimitConfig
}

// Allow 检查Fallback请求是否允许
func (l *FallbackRateLimiter) Allow(ctx context.Context, key string, tier int) (bool, error) {
    if l.config.ReuseMainQuota {
        // 复用主配额：Fallback请求与主请求共享配额
        return l.mainLimiter.Allow(ctx, key)
    }

    // 使用独立Fallback配额
    fallbackKey := fmt.Sprintf("%s:tier%d", l.config.KeyPrefix, tier)
    return l.fallbackLimiter.Allow(ctx, fallbackKey)
}

// GetFallbackRPM 获取指定Tier的Fallback RPM限制
func (l *FallbackRateLimiter) GetFallbackRPM(tier int) int {
    // Tier越高，限流越宽松
    baseRPM := l.config.FallbackRPM
    return baseRPM * (tier + 1) // Tier1=1x, Tier2=2x, Tier3=3x
}

// IsQuotaExhausted 检查配额是否耗尽
func (l *FallbackRateLimiter) IsQuotaExhausted(ctx context.Context, key string) bool {
    mainTokens, mainAvailable := l.mainLimiter.GetTokenCount(ctx, key)
    if l.config.ReuseMainQuota {
        return !mainAvailable || mainTokens <= 0
    }

    fbTokens, fbAvailable := l.fallbackLimiter.GetTokenCount(ctx, key)
    return !fbAvailable || fbTokens <= 0
}
```

#### 4.3.4 与现有ratelimit.TokenBucketLimiter的兼容性

| 接口 | 兼容性 | 说明 |
|------|--------|------|
| Allow(ctx, key) | 兼容 | FallbackRateLimiter.Allow()签名与TokenBucketLimiter.Allow()一致 |
| GetTokenCount() | 扩展 | FallbackRateLimiter扩展此接口用于查询配额 |
| 配额计算 | 兼容 | Fallback配额计算逻辑与主限流器一致 |
| 监控指标 | 兼容 | 复用的mainLimiter指标体系，不需要额外埋点 |

**兼容性结论**：FallbackRateLimiter设计为对现有TokenBucketLimiter的包装器，不破坏现有限流逻辑，可渐进式集成。

---

## 5. 路由决策引擎

### 5.1 路由请求结构

```go
// RoutingRequest 路由请求
type RoutingRequest struct {
    // 请求 ID
    RequestID string `json:"request_id"`

    // 模型名称
    Model string `json:"model"`

    // 供应商列表
    Providers []ProviderInfo `json:"providers"`

    // 用户信息
    UserID string `json:"user_id"`
    GroupID string `json:"group_id"`

    // 请求上下文
    Context *RequestContext `json:"context,omitempty"`

    // 策略约束
    Constraints *RoutingConstraints `json:"constraints,omitempty"`
}

// ProviderInfo Provider 信息
type ProviderInfo struct {
    Name             string  `json:"name"`
    Model            string  `json:"model"`
    Available        bool    `json:"available"`
    LatencyMs        int64   `json:"latency_ms"`
    CostPer1KTokens  float64 `json:"cost_per_1k_tokens"`
    QualityScore     float64 `json:"quality_score"`
    FailureRate      float64 `json:"failure_rate"`
    RPM              int     `json:"rpm"`
    TPM              int     `json:"tpm"`
    Region           string  `json:"region"`
    IsCN             bool    `json:"is_cn"`
}

// RequestContext 请求上下文
type RequestContext struct {
    // 优先级
    Priority Priority `json:"priority"`

    // 是否关键请求
    IsCritical bool `json:"is_critical"`

    // 预算限制
    BudgetLimit float64 `json:"budget_limit,omitempty"`

    // 延迟预算
    LatencyBudgetMs int64 `json:"latency_budget_ms,omitempty"`
}

// Priority 优先级
type Priority int

const (
    PriorityLow    Priority = 0
    PriorityNormal Priority = 1
    PriorityHigh   Priority = 2
    Priorityurgent Priority = 3 // 关键请求
)

// RoutingConstraints 路由约束
type RoutingConstraints struct {
    // 允许的供应商
    AllowedProviders []string `json:"allowed_providers,omitempty"`

    // 禁止的供应商
    BlockedProviders []string `json:"blocked_providers,omitempty"`

    // 允许的区域
    AllowedRegions []string `json:"allowed_regions,omitempty"`

    // 最大成本
    MaxCost float64 `json:"max_cost,omitempty"`

    // 最大延迟
    MaxLatencyMs int64 `json:"max_latency_ms,omitempty"`
}
```

### 5.2 路由决策结果

```go
// RoutingDecision 路由决策
type RoutingDecision struct {
    // 选择的 Provider
    Provider string `json:"provider"`

    // 使用的策略
    Strategy RoutingStrategyType `json:"strategy"`

    // 决策分数 (用于审计)
    Score float64 `json:"score"`

    // 预估成本
    EstimatedCost float64 `json:"estimated_cost"`

    // 预估延迟
    EstimatedLatency int64 `json:"estimated_latency"`

    // 预估质量
    EstimatedQuality float64 `json:"estimated_quality"`

    // 决策原因
    Reason string `json:"reason"`

    // Fallback 列表
    FallbackProviders []string `json:"fallback_providers"`

    // 决策时间
    DecisionTime time.Time `json:"decision_time"`

    // 路由标记 (用于 M-008)
    RouterEngine string `json:"router_engine"` // "router_core" or "subapi_path"
}
```

### 5.3 路由引擎核心

```go
// RoutingEngine 路由引擎
type RoutingEngine struct {
    // 策略注册表
    strategies map[string]RoutingStrategy

    // Provider 管理器
    providerManager *ProviderManager

    // Fallback 管理器
    fallbackManager *FallbackManager

    // 指标收集器
    metricsCollector *MetricsCollector

    // 告警管理器
    alertManager *alert.Manager

    // 配置
    config *RoutingEngineConfig
}

// RoutingEngineConfig 路由引擎配置
type RoutingEngineConfig struct {
    // 默认策略
    DefaultStrategy string `json:"default_strategy"`

    // 策略匹配顺序
    StrategyMatchOrder []string `json:"strategy_match_order"`

    // 启用策略缓存
    EnableStrategyCache bool `json:"enable_strategy_cache"`

    // 策略缓存 TTL
    StrategyCacheTTL time.Duration `json:"strategy_cache_ttl"`

    // 启用降级
    EnableDegradation bool `json:"enable_degradation"`

    // 降级阈值
    DegradationThreshold float64 `json:"degradation_threshold"`
}

// SelectProvider 选择 Provider
func (e *RoutingEngine) SelectProvider(ctx context.Context, req *RoutingRequest) (*RoutingDecision, error) {
    // 1. 匹配策略
    strategy := e.matchStrategy(req)
    if strategy == nil {
        strategy = e.getDefaultStrategy()
    }

    // 2. 执行策略
    decision, err := strategy.Select(ctx, req)
    if err != nil {
        // 3. 执行 Fallback
        fbDecision, fbErr := e.handleFallback(ctx, req, err)
        if fbErr != nil {
            return nil, fbErr
        }
        // M-008: Fallback路径也需要记录接管标记
        e.metricsCollector.RecordTakeoverMark(req.RequestID, fbDecision.RouterEngine)
        return fbDecision, nil
    }

    // 4. 记录指标
    decision.RouterEngine = "router_core" // M-008: 标记为router_core主路径
    e.recordDecision(decision, req)

    // M-008: 记录接管标记 (确保100%覆盖)
    e.metricsCollector.RecordTakeoverMark(req.RequestID, decision.RouterEngine)

    // 5. 检查是否需要告警
    e.checkAlerts(decision, req)

    return decision, nil
}

// matchStrategy 匹配策略
func (e *RoutingEngine) matchStrategy(req *RoutingRequest) RoutingStrategy {
    for _, strategyID := range e.config.StrategyMatchOrder {
        strategy, ok := e.strategies[strategyID]
        if !ok {
            continue
        }

        template := strategy.GetTemplate()
        if !template.Enabled {
            continue
        }

        if e.isApplicable(req, template) {
            return strategy
        }
    }
    return nil
}
```

---

## 6. 配置化设计

### 6.1 策略配置示例 (YAML)

```yaml
# routing_strategies.yaml
strategies:
  # 成本优先策略
  - id: "cost_first"
    name: "成本优先策略"
    type: "cost_based"
    enabled: true
    priority: 10
    applicable_models: ["*"]
    applicable_providers: ["*"]
    description: "优先选择成本最低的可用 Provider"
    params:
      cost_params:
        max_cost_per_1k_tokens: 0.1
        prefer_low_cost: true
        cost_weight: 1.0
      fallback_config:
        max_retries: 2
        retry_interval_ms: 100
        fail_fast: true
        tiers:
          - tier: 1
            providers: ["openai", "anthropic"]
            timeout_ms: 5000
          - tier: 2
            providers: ["gemini", "azure"]
            timeout_ms: 8000

  # 质量优先策略
  - id: "quality_first"
    name: "质量优先策略"
    type: "quality_first"
    enabled: true
    priority: 20
    applicable_models: ["gpt-4", "claude-3-opus", "gemini-ultra"]
    applicable_providers: ["openai", "anthropic"]
    description: "针对高端模型的质量优先策略"
    params:
      quality_params:
        min_quality_threshold: 0.9
        quality_weight: 1.0
        quality_metrics:
          - name: "accuracy"
            weight: 0.4
            score: 0.95
          - name: "coherence"
            weight: 0.3
            score: 0.9
          - name: "safety"
            weight: 0.3
            score: 0.95
      fallback_config:
        max_retries: 1
        tiers:
          - tier: 1
            providers: ["anthropic", "openai"]
            timeout_ms: 10000

  # 国内供应商策略 (M-007 支持)
  - id: "cn_provider"
    name: "国内供应商优先策略"
    type: "model_specific"
    enabled: true
    priority: 5  # 高优先级
    applicable_models: ["*"]
    applicable_providers: ["*"]
    description: "国内供应商 100% 接管策略"
    params:
      model_params:
        default_provider: "cn_primary"
        model_groups:
          cn_preferred:
            - "deepseek"
            - "qwen"
            - "yi"
      fallback_config:
        max_retries: 3
        tiers:
          - tier: 1
            providers: ["deepseek", "qwen", "yi"]
            trigger:
              error_types: ["rate_limit", "server_error"]
            timeout_ms: 5000
          - tier: 2
            providers: ["openai", "anthropic"]  # 国际供应商兜底
            trigger:
              error_types: ["timeout", "unavailable"]
            timeout_ms: 8000

  # 复合策略示例
  - id: "balanced_composite"
    name: "均衡复合策略"
    type: "composite"
    enabled: true
    priority: 15
    applicable_models: ["*"]
    description: "综合考虑成本、质量、延迟的均衡策略"
    params:
      cost_params:
        max_cost_per_1k_tokens: 0.15
      quality_params:
        min_quality_threshold: 0.8
      latency_params:
        max_latency_ms: 3000
      composite_params:
        combine_mode: "weighted_score"
        strategies:
          - strategy_id: "cost_weighted"
            weight: 0.3
          - strategy_id: "quality_weighted"
            weight: 0.4
          - strategy_id: "latency_weighted"
            weight: 0.3

  # 灰度发布策略示例
  - id: "gray_rollout_quality_first"
    name: "质量优先策略-灰度发布"
    type: "quality_first"
    enabled: true
    priority: 25
    applicable_models: ["gpt-4o", "claude-3-5-sonnet"]
    description: "灰度发布中的质量优先策略"
    rollout:
      enabled: true
      percentage: 10      # 初始10%流量
      max_percentage: 100
      increment: 10       # 每次增加10%
      increment_interval: 24h
      rules:
        - type: "tenant_id"
          values: ["tenant_001", "tenant_002"]
          force: true     # 强制启用
        - type: "region"
          values: ["cn"]
          force: false
      start_time: "2026-04-01T00:00:00Z"

  # A/B测试策略示例
  - id: "ab_test_quality_vs_cost"
    name: "质量优先vs成本优先-A/B测试"
    type: "ab_test"
    enabled: true
    priority: 30
    applicable_models: ["*"]
    description: "A/B测试：质量优先策略 vs 成本优先策略"
    ab_config:
      experiment_id: "exp_quality_vs_cost_001"
      experiment_group_id: "quality_first"
      control_group_id: "cost_first"
      traffic_split: 50    # 50%流量到实验组(质量优先)
      bucket_key: "user_id"
      start_time: "2026-04-01T00:00:00Z"
      end_time: "2026-04-30T23:59:59Z"
      hypothesis: "质量优先策略可以提高用户满意度"
      success_metrics:
        - "user_satisfaction_score"
        - "task_completion_rate"
        - "average_latency"
    params:
      # 实验组配置 (质量优先)
      quality_params:
        min_quality_threshold: 0.85
        quality_weight: 0.7
      # 对照组配置 (成本优先)
      cost_params:
        max_cost_per_1k_tokens: 0.08
        cost_weight: 0.7
```

### 6.2 策略加载器

```go
// StrategyLoader 策略加载器
type StrategyLoader struct {
    configPath string
}

// LoadStrategies 加载策略
func (l *StrategyLoader) LoadStrategies(path string) ([]*RoutingStrategyTemplate, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read strategy config: %w", err)
    }

    var config struct {
        Strategies []*RoutingStrategyTemplate `json:"strategies"`
    }

    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("failed to parse strategy config: %w", err)
    }

    return config.Strategies, nil
}

// WatchChanges 监听配置变化
func (l *StrategyLoader) WatchChanges(ctx context.Context, callback func([]*RoutingStrategyTemplate)) error {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return err
    }
    defer watcher.Close()

    err = watcher.Watch(l.configPath)
    if err != nil {
        return err
    }

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case event := <-watcher.Events:
            if event.Op&fsnotify.Write == fsnotify.Write {
                strategies, err := l.LoadStrategies(l.configPath)
                if err != nil {
                    log.Printf("failed to reload strategies: %v", err)
                    continue
                }
                callback(strategies)
            }
        }
    }
}
```

---

## 7. 与现有组件集成

### 7.1 与 RateLimit 集成

```go
// RoutingRateLimitMiddleware 路由限流中间件
type RoutingRateLimitMiddleware struct {
    limiter        ratelimit.Limiter
    strategyLimiter *ratelimit.TokenBucketLimiter
}

// Allow 检查请求是否允许
func (m *RoutingRateLimitMiddleware) Allow(ctx context.Context, key string, strategyID string) (bool, error) {
    // 1. 检查主限流
    allowed, err := m.limiter.Allow(ctx, key)
    if err != nil {
        return false, err
    }
    if !allowed {
        return false, nil
    }

    // 2. 检查策略级限流 (可选)
    if m.strategyLimiter != nil {
        strategyKey := fmt.Sprintf("%s:%s", key, strategyID)
        allowed, err = m.strategyLimiter.Allow(ctx, strategyKey)
        if err != nil {
            return false, err
        }
        if !allowed {
            return false, nil
        }
    }

    return true, nil
}
```

### 7.2 与 Alert 集成

```go
// RoutingAlertConfig 路由告警配置
type RoutingAlertConfig struct {
    // 接管率告警阈值
    TakeoverRateThreshold float64 `json:"takeover_rate_threshold"`

    // 失败率告警阈值
    FailureRateThreshold float64 `json:"failure_rate_threshold"`

    // 延迟告警阈值 (ms)
    LatencyThresholdMs int64 `json:"latency_threshold_ms"`

    // 连续告警次数阈值
    AlertConsecutiveCount int `json:"alert_consecutive_count"`
}

// RoutingAlerter 路由告警器
type RoutingAlerter struct {
    alertManager *alert.Manager
    config       *RoutingAlertConfig

    // 告警计数
    alertCounts map[string]int
    mu          sync.Mutex
}

// OnTakeoverRateAlert 接管率告警
func (a *RoutingAlerter) OnTakeoverRateAlert(ctx context.Context, decision *RoutingDecision, req *RoutingRequest) {
    a.mu.Lock()
    defer a.mu.Unlock()

    key := fmt.Sprintf("takeover:%s", req.Model)
    a.alertCounts[key]++

    if a.alertCounts[key] >= a.config.AlertConsecutiveCount {
        a.alertManager.Send(ctx, &alert.Alert{
            Type:     alert.AlertHighErrorRate,
            Title:    "Takeover Rate Alert",
            Message:  fmt.Sprintf("Takeover rate below threshold for model %s: %.2f%%", req.Model, decision.Score*100),
            Severity: "warning",
            Metadata: map[string]interface{}{
                "model":          req.Model,
                "takeover_rate":  decision.Score,
                "threshold":      a.config.TakeoverRateThreshold,
                "request_id":     req.RequestID,
            },
        })
        a.alertCounts[key] = 0
    }
}

// OnProviderFailureAlert Provider 故障告警
func (a *RoutingAlerter) OnProviderFailureAlert(ctx context.Context, provider, model string, err error) {
    a.alertManager.SendProviderFailureAlert(ctx, provider, err)
}
```

### 7.3 与 Metrics 集成 (M-006/M-007/M-008 支持)

```go
// RoutingMetrics 路由指标
type RoutingMetrics struct {
    // 路由决策计数器
    decisionsTotal *prometheus.CounterVec

    // 路由决策延迟
    decisionLatency *prometheus.HistogramVec

    // Provider 状态
    providerStatus *prometheus.GaugeVec

    // 接管率 (用于 M-006, M-007)
    takeoverRate *prometheus.GaugeVec
}

// RecordDecision 记录路由决策
func (m *RoutingMetrics) RecordDecision(decision *RoutingDecision, req *RoutingRequest) {
    m.decisionsTotal.WithLabelValues(
        decision.Provider,
        string(decision.Strategy),
        req.Model,
        decision.RouterEngine,
    ).Inc()

    m.decisionLatency.WithLabelValues(
        decision.Provider,
        string(decision.Strategy),
    ).Observe(float64(decision.EstimatedLatency))
}

// RecordTakeoverMark 记录接管标记 (用于 M-008)
func (m *RoutingMetrics) RecordTakeoverMark(requestID, routerEngine string) {
    m.takeoverRate.WithLabelValues(routerEngine).Inc()
}

// UpdateTakeoverRate 更新接管率
func (m *RoutingMetrics) UpdateTakeoverRate(overallRate, cnRate float64) {
    m.providerStatus.WithLabelValues("overall_takeover").Set(overallRate)
    m.providerStatus.WithLabelValues("cn_takeover").Set(cnRate)
}
```

---

## 8. 可量化与可测试设计

### 8.1 策略评分模型

```go
// ScoringModel 评分模型
type ScoringModel struct {
    // 成本分数 (越低越好)
    CostScore float64 `json:"cost_score"`

    // 质量分数 (越高越好)
    QualityScore float64 `json:"quality_score"`

    // 延迟分数 (越低越好)
    LatencyScore float64 `json:"latency_score"`

    // 可用性分数 (越高越好)
    AvailabilityScore float64 `json:"availability_score"`

    // 综合分数
    TotalScore float64 `json:"total_score"`

    // 权重配置 (如果不指定则使用DefaultScoreWeights)
    Weights ScoreWeights `json:"weights"`
}

// CalculateScore 计算 Provider 分数
func (m *ScoringModel) CalculateScore(provider *ProviderInfo, weights *ScoreWeights) float64 {
    // 如果没有传入权重，使用默认权重
    if weights == nil {
        weights = &DefaultScoreWeights
    }

    // 归一化分数
    costNorm := m.normalizeCost(provider.CostPer1KTokens)
    qualityNorm := m.normalizeQuality(provider.QualityScore)
    latencyNorm := m.normalizeLatency(provider.LatencyMs)
    availabilityNorm := m.normalizeAvailability(provider.FailureRate)

    // 加权求和
    total := costNorm*weights.CostWeight +
             qualityNorm*weights.QualityWeight +
             latencyNorm*weights.LatencyWeight +
             availabilityNorm*weights.AvailabilityWeight

    return total
}

// ScoreWeights 分数权重
type ScoreWeights struct {
    CostWeight        float64 `json:"cost_weight"`
    QualityWeight     float64 `json:"quality_weight"`
    LatencyWeight     float64 `json:"latency_weight"`
    AvailabilityWeight float64 `json:"availability_weight"`
}

// 默认评分权重 (与技术架构一致)
const DefaultScoreWeights = ScoreWeights{
    CostWeight:         0.2,  // 20%
    QualityWeight:      0.1,  // 10%
    LatencyWeight:      0.4,  // 40%
    AvailabilityWeight: 0.3,  // 30%
}

// DefaultScoringModel 默认评分模型 (使用固定权重)
type DefaultScoringModel struct {
    ScoringModel
}

func NewDefaultScoringModel() *DefaultScoringModel {
    return &DefaultScoringModel{
        ScoringModel: ScoringModel{
            Weights: DefaultScoreWeights,
        },
    }
}

// CalculateScore 使用默认权重计算分数
func (m *DefaultScoringModel) CalculateScore(provider *ProviderInfo) float64 {
    return m.ScoringModel.CalculateScore(provider, &DefaultScoreWeights)
}
```

### 8.2 单元测试示例

```go
// Strategy_test.go
func TestCostBasedStrategy_SelectProvider(t *testing.T) {
    template := &RoutingStrategyTemplate{
        ID:   "test_cost",
        Type:  StrategyCostBased,
        Enabled: true,
        Params: StrategyParams{
            CostParams: &CostParams{
                MaxCostPer1KTokens: 0.05,
                PreferLowCost: true,
                CostWeight: 1.0,
            },
        },
    }

    strategy := NewCostBasedStrategy(template)
    req := &RoutingRequest{
        RequestID: "test-001",
        Model: "gpt-3.5-turbo",
        Providers: []ProviderInfo{
            {Name: "openai", CostPer1KTokens: 0.002, Available: true},
            {Name: "anthropic", CostPer1KTokens: 0.015, Available: true},
            {Name: "expensive", CostPer1KTokens: 0.1, Available: true},
        },
    }

    decision, err := strategy.Select(context.Background(), req)
    assert.NoError(t, err)
    assert.Equal(t, "openai", decision.Provider)
    assert.LessOrEqual(t, decision.EstimatedCost, 0.05)
}

func TestFallbackStrategy_TierExecution(t *testing.T) {
    template := &RoutingStrategyTemplate{
        ID:   "test_fallback",
        Type:  StrategyCostBased,
        Enabled: true,
        Params: StrategyParams{
            FallbackConfig: &FallbackConfig{
                MaxRetries: 2,
                Tiers: []FallbackTier{
                    {Tier: 1, Providers: []string{"primary"}, TimeoutMs: 100},
                    {Tier: 2, Providers: []string{"secondary"}, TimeoutMs: 200},
                },
            },
        },
    }

    // 测试 Tier 降级
    // ...
}

func TestABStrategyTemplate_TrafficSplit(t *testing.T) {
    // 准备A/B测试策略
    template := &ABStrategyTemplate{
        RoutingStrategyTemplate: RoutingStrategyTemplate{
            ID:      "test_ab",
            Type:    StrategyComposite,
            Enabled: true,
        },
        ControlStrategy: &RoutingStrategyTemplate{
            ID:   "control",
            Type:  StrategyCostBased,
        },
        ExperimentStrategy: &RoutingStrategyTemplate{
            ID:   "experiment",
            Type:  StrategyQualityFirst,
        },
        Config: ABTestConfig{
            ExperimentID:  "exp_001",
            TrafficSplit:  20, // 20%流量到实验组
            BucketKey:    "user_id",
        },
    }

    // 模拟1000个用户请求
    experimentCount := 0
    controlCount := 0

    for i := 0; i < 1000; i++ {
        req := &RoutingRequest{
            UserID: fmt.Sprintf("user_%d", i),
        }

        if template.ShouldApplyToRequest(req) {
            experimentCount++
        } else {
            controlCount++
        }
    }

    // 验证流量分配比例 (允许5%误差)
    assert.InDelta(t, 200, experimentCount, 50, "实验组流量应在150-250之间")
    assert.InDelta(t, 800, controlCount, 50, "对照组流量应在750-850之间")
}

func TestRolloutConfig_Percentage(t *testing.T) {
    template := &RoutingStrategyTemplate{
        ID:   "test_rollout",
        Type:  StrategyCostBased,
        Enabled: true,
        RolloutConfig: &RolloutConfig{
            Enabled:          true,
            Percentage:       30,  // 30%流量
            MaxPercentage:    100,
            Increment:        10,
            IncrementInterval: 24 * time.Hour,
        },
    }

    // 验证初始灰度百分比
    assert.Equal(t, 30, template.RolloutConfig.Percentage)

    // 模拟灰度增长
    template.RolloutConfig.Percentage += template.RolloutConfig.Increment
    assert.Equal(t, 40, template.RolloutConfig.Percentage)

    // 验证不超过最大百分比
    template.RolloutConfig.Percentage = 95
    template.RolloutConfig.Percentage += template.RolloutConfig.Increment
    assert.Equal(t, 100, template.RolloutConfig.Percentage)
}

func TestFallbackRateLimiter_Integration(t *testing.T) {
    // 准备限流器
    mainLimiter := ratelimit.NewTokenBucketLimiter(100, 1000) // 100 RPM, 1000 TPM
    fallbackLimiter := ratelimit.NewTokenBucketLimiter(50, 500) // 50 RPM, 500 TPM

    rateLimiter := &FallbackRateLimiter{
        mainLimiter:     mainLimiter,
        fallbackLimiter: fallbackLimiter,
        config: FallbackRateLimitConfig{
            KeyPrefix:      "fallback",
            FallbackRPM:   50,
            FallbackTPM:   500,
            ReuseMainQuota: false,
        },
    }

    ctx := context.Background()
    key := "test_user"

    // 验证主限流器正常工作
    allowed, _ := rateLimiter.Allow(ctx, key, 1)
    assert.True(t, allowed)

    // 验证Fallback限流器正常工作
    allowed, _ = rateLimiter.Allow(ctx, key, 1)
    assert.True(t, allowed)

    // 验证配额耗尽后拒绝
    // (需要消耗完所有令牌...)
}

func TestM008_TakeoverMarkCoverage(t *testing.T) {
    // 验证M-008 route_mark_coverage指标采集
    engine := setupTestEngine()

    testCases := []struct {
        name           string
        providerResult error
        expectMark     bool
        expectEngine   string
    }{
        {
            name:           "主路径成功",
            providerResult: nil,
            expectMark:     true,
            expectEngine:   "router_core",
        },
        {
            name:           "主路径失败_Fallback成功",
            providerResult: ErrProviderUnavailable,
            expectMark:     true,
            expectEngine:   "router_core",
        },
        {
            name:           "主路径和Fallback都失败",
            providerResult: ErrAllProvidersUnavailable,
            expectMark:     false,
            expectEngine:   "",
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            req := &RoutingRequest{
                RequestID: fmt.Sprintf("test-%s", tc.name),
                Model:     "test-model",
            }

            decision, err := engine.SelectProvider(context.Background(), req)

            if tc.expectMark {
                assert.NoError(t, err)
                assert.Equal(t, tc.expectEngine, decision.RouterEngine)

                // 验证RecordTakeoverMark被调用
                mark := engine.metricsCollector.GetTakeoverMark(req.RequestID)
                assert.NotEmpty(t, mark)
            }
        })
    }
}
```

### 8.3 集成测试场景

```go
// Integration_test.go
func TestRoutingEngine_E2E_WithTakeoverMetrics(t *testing.T) {
    // 1. 准备测试环境
    engine := setupTestEngine()

    // 2. 注入测试 Provider
    engine.providerManager.RegisterProvider(&ProviderInfo{
        Name: "test_provider",
        Model: "test-model",
        Available: true,
        CostPer1KTokens: 0.01,
        QualityScore: 0.9,
        LatencyMs: 100,
    })

    // 3. 模拟请求
    req := &RoutingRequest{
        RequestID: "test-e2e-001",
        Model: "test-model",
        Providers: engine.providerManager.GetAllProviders(),
    }

    // 4. 执行路由
    decision, err := engine.SelectProvider(context.Background(), req)

    // 5. 验证决策
    assert.NotNil(t, decision)
    assert.NoError(t, err)
    assert.Equal(t, "test_provider", decision.Provider)
    assert.Equal(t, "router_core", decision.RouterEngine) // M-008

    // 6. 验证指标记录
    metrics := engine.metricsCollector.GetMetrics()
    assert.Equal(t, 1, metrics["decisions_total"])
    assert.Contains(t, metrics["router_engine_mark"], "router_core")
}
```

---

## 9. 文件结构

```
gateway/internal/
├── router/
│   ├── router.go                 # 基础 Router
│   ├── router_test.go           # 基础 Router 测试
│   ├── strategy/
│   │   ├── strategy.go          # 策略接口定义
│   │   ├── strategy_template.go # 策略模板
│   │   ├── cost_strategy.go     # 成本策略
│   │   ├── quality_strategy.go   # 质量策略
│   │   ├── latency_strategy.go  # 延迟策略
│   │   ├── model_strategy.go     # 模型策略
│   │   ├── composite_strategy.go # 复合策略
│   │   └── strategy_test.go     # 策略测试
│   ├── engine/
│   │   ├── engine.go            # 路由引擎
│   │   ├── engine_test.go       # 引擎测试
│   │   └── config.go            # 引擎配置
│   ├── fallback/
│   │   ├── fallback.go          # Fallback 逻辑
│   │   ├── fallback_test.go     # Fallback 测试
│   │   └── conditions.go       # 触发条件
│   ├── metrics/
│   │   └── metrics.go          # 路由指标 (M-006/M-007/M-008)
│   └── config/
│       ├── config.go           # 路由配置
│       └── strategies.yaml      # 策略配置文件
```

---

## 10. 实施计划

### 10.1 P1 阶段任务分解

| 任务 | 描述 | 依赖 | 优先级 |
|------|------|------|--------|
| T-001 | 定义策略模板结构体和接口 | 无 | P0 |
| T-002 | 实现成本策略 (CostBasedStrategy) | T-001 | P0 |
| T-003 | 实现质量策略 (QualityStrategy) | T-001 | P0 |
| T-004 | 实现模型策略 (ModelStrategy) | T-001 | P0 |
| T-005 | 设计 Fallback 机制 | T-002/T-003/T-004 | P0 |
| T-006 | 实现路由引擎 (RoutingEngine) | T-001~T-005 | P0 |
| T-007 | 集成 RateLimit | T-006 | P1 |
| T-008 | 集成 Alert | T-006 | P1 |
| T-009 | 实现 Metrics 收集 (M-006/M-007/M-008) | T-006 | P1 |
| T-010 | 配置化策略加载器 | T-006 | P1 |
| T-011 | 单元测试 | T-002~T-010 | P1 |
| T-012 | 集成测试 | T-011 | P2 |

### 10.2 验收标准

1. **策略可配置**：策略模板可通过 YAML 配置加载
2. **策略可切换**：运行时可动态切换策略
3. **Fallback 有效**：Provider 故障时可正确降级
4. **指标可观测**：M-006/M-007/M-008 指标可采集
5. **告警可触发**：异常情况可触发告警
6. **测试可覆盖**：核心逻辑单元测试覆盖率 >= 80%

---

## 11. 附录

### 11.1 术语表

| 术语 | 定义 |
|------|------|
| Takeover Rate | 自研 Router Core 接管请求的比例 |
| Router Engine | 路由引擎字段，标记请求是否由自研 Router Core 处理 |
| Fallback | 当主路径失败时的备选路径 |
| Strategy Template | 路由策略模板，定义路由决策的规则和参数 |

### 11.2 参考文档

1. `router_core_takeover_execution_plan_v3_2026-03-17.md`
2. `router_core_takeover_metrics_sql_dashboard_v1_2026-03-17.md`
3. `acceptance_gate_single_source_v1_2026-03-18.md`
4. `gateway/internal/router/router.go`
5. `gateway/internal/adapter/adapter.go`
6. `gateway/internal/ratelimit/ratelimit.go`
7. `gateway/internal/alert/alert.go`

---

## 12. 更新记录

| 版本 | 日期 | 作者 | 变更内容 |
|------|------|------|----------|
| v1.0 | 2026-04-02 | Claude | 初始版本 |
| v1.1 | 2026-04-02 | Claude | 修复评审问题：<br>- 明确评分模型默认权重(延迟40%/可用性30%/成本20%/质量10%)<br>- 完善M-008 route_mark_coverage全路径采集逻辑<br>- 增加A/B测试支持(ABStrategyTemplate)<br>- 增加灰度发布支持(RolloutConfig)<br>- 明确Fallback与Ratelimit集成点与兼容性 |
