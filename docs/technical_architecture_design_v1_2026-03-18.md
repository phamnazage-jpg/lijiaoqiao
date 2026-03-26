# 详细技术架构设计

> 版本：v1.0
> 日期：2026-03-18
> 依据：backend skill 最佳实践
> 状态：历史草稿（已被 `technical_architecture_optimized_v2_2026-03-18.md` 替代，不作为实施基线）

---

## 1. 系统架构概览

### 1.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                                    客户端层                                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐           │
│  │ Web App    │  │ Mobile App  │  │ SDK (Python)│  │ SDK (Node)  │           │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘           │
└─────────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                                 API Gateway (入口层)                               │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │ • 限流 • 鉴权 • 路由 • 日志 • 监控                                         │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                                 业务服务层                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │  Router      │  │  Auth       │  │  Billing     │  │  Provider    │       │
│  │  Service     │  │  Service    │  │  Service    │  │  Service    │       │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │  Tenant     │  │  Risk       │  │  Settlement │  │  Webhook    │       │
│  │  Service    │  │  Service    │  │  Service    │  │  Service    │       │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘       │
└─────────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                                 基础设施层                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │  PostgreSQL  │  │    Redis     │  │    Kafka     │  │    S3/MinIO │       │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘       │
└─────────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                                 外部集成层                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │   subapi     │  │  OpenAI API │  │ Anthropic   │  │  国内供应商   │       │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘       │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. 技术选型

### 2.1 技术栈

| 层级 | 技术 | 版本 | 说明 |
|------|------|------|------|
| API Gateway | Kong / Traefik | 3.x | 高性能网关 |
| 后端服务 | Go | 1.21 | 高并发 |
| Web框架 | Gin | 1.9 | 高性能 |
| 数据库 | PostgreSQL | 15 | 主数据库 |
| 缓存 | Redis | 7.x | 缓存+限流 |
| 消息队列 | Kafka | 3.x | 异步处理 |
| 服务网格 | Istio | 1.18 | 微服务治理 |
| 容器编排 | Kubernetes | 1.28 | 容器编排 |
| CI/CD | GitHub Actions | - | 持续集成 |
| 监控 | Prometheus + Grafana | - | 可观测性 |
| 日志 | ELK Stack | 8.x | 日志收集 |

### 2.2 项目结构

```
llm-gateway/
├── cmd/                      # 入口程序
│   ├── gateway/              # 网关服务
│   ├── router/               # 路由服务
│   ├── billing/              # 计费服务
│   └── admin/                # 管理后台
├── internal/                 # 内部包
│   ├── config/               # 配置管理
│   ├── middleware/           # 中间件
│   ├── handler/              # HTTP处理器
│   ├── service/              # 业务逻辑
│   ├── repository/           # 数据访问
│   └── model/                # 数据模型
├── pkg/                      # 公共包
│   ├── utils/                # 工具函数
│   ├── errors/               # 错误定义
│   └── constants/            # 常量定义
├── api/                      # API定义
│   ├── openapi/              # OpenAPI规范
│   └── proto/                # Protobuf定义
├── configs/                  # 配置文件
├── scripts/                  # 脚本
├── test/                     # 测试
└── docs/                     # 文档
```

---

## 3. 模块详细设计

### 3.1 API Gateway 模块

```go
// cmd/gateway/main.go
package main

import (
    "github.com/gin-gonic/gin"
    "llm-gateway/internal/middleware"
    "llm-gateway/internal/handler"
)

func main() {
    r := gin.Default()

    // 全局中间件
    r.Use(middleware.Logger())
    r.Use(middleware.Recovery())
    r.Use(middleware.CORS())

    // 限流
    r.Use(middleware.RateLimiter())

    // API路由
    v1 := r.Group("/v1")
    {
        // 认证
        v1.POST("/auth/token", handler.AuthToken)
        v1.POST("/auth/refresh", handler.RefreshToken)

        // 对话
        v1.POST("/chat/completions", middleware.AuthRequired(), handler.ChatCompletions)
        v1.POST("/completions", middleware.AuthRequired(), handler.Completions)

        // Embeddings
        v1.POST("/embeddings", middleware.AuthRequired(), handler.Embeddings)

        // 模型
        v1.GET("/models", handler.ListModels)

        // 用户
        users := v1.Group("/users")
        users.Use(middleware.AuthRequired())
        {
            users.GET("", handler.ListUsers)
            users.GET("/:id", handler.GetUser)
            users.PUT("/:id", handler.UpdateUser)
        }

        // API Key
        keys := v1.Group("/keys")
        keys.Use(middleware.AuthRequired())
        {
            keys.GET("", handler.ListKeys)
            keys.POST("", handler.CreateKey)
            keys.DELETE("/:id", handler.DeleteKey)
            keys.POST("/:id/rotate", handler.RotateKey)
        }

        // 计费
        billing := v1.Group("/billing")
        billing.Use(middleware.AuthRequired())
        {
            billing.GET("/balance", handler.GetBalance)
            billing.GET("/usage", handler.GetUsage)
            billing.GET("/invoices", handler.ListInvoices)
        }

        // 供应方
        supply := v1.Group("/supply")
        supply.Use(middleware.AuthRequired())
        {
            supply.GET("/accounts", handler.ListAccounts)
            supply.POST("/accounts", handler.CreateAccount)
            supply.GET("/packages", handler.ListPackages)
            supply.POST("/packages", handler.CreatePackage)
            supply.GET("/earnings", handler.GetEarnings)
        }
    }

    // 管理后台
    admin := r.Group("/admin")
    admin.Use(middleware.AdminRequired())
    {
        admin.GET("/stats", handler.AdminStats)
        admin.GET("/users", handler.AdminListUsers)
        admin.POST("/users/:id/disable", handler.DisableUser)
    }

    r.Run(":8080")
}
```

### 3.2 路由服务模块

```go
// internal/service/router.go
package service

import (
    "context"
    "time"
    "llm-gateway/internal/model"
    "llm-gateway/internal/adapter"
)

type RouterService struct {
    adapterRegistry *adapter.Registry
    metricsCollector *MetricsCollector
}

type RouteRequest struct {
    Model      string                 `json:"model"`
    Messages   []model.Message      `json:"messages"`
    Options    model.CompletionOptions `json:"options"`
    UserID     int64                `json:"user_id"`
    TenantID   int64                `json:"tenant_id"`
}

func (s *RouterService) Route(ctx context.Context, req RouteRequest) (*model.CompletionResponse, error) {
    // 1. 获取可用供应商
    providers := s.adapterRegistry.GetAvailableProviders(req.Model)
    if len(providers) == 0 {
        return nil, ErrNoProviderAvailable
    }

    // 2. 选择最优供应商
    selected := s.selectProvider(providers, req)

    // 3. 记录路由决策
    s.metricsCollector.RecordRoute(ctx, &RouteMetrics{
        Model: req.Model,
        Provider: selected.Name(),
        TenantID: req.TenantID,
    })

    // 4. 调用供应商
    resp, err := selected.Call(ctx, req)
    if err != nil {
        // 5. 失败时尝试fallback
        return s.tryFallback(ctx, req, err)
    }

    return resp, nil
}

func (s *RouterService) selectProvider(providers []*adapter.Provider, req RouteRequest) *adapter.Provider {
    // 多维度选择策略
    var best *adapter.Provider
    bestScore := -1.0

    for _, p := range providers {
        score := s.calculateScore(p, req)
        if score > bestScore {
            bestScore = score
            best = p
        }
    }

    return best
}

func (s *RouterService) calculateScore(p *adapter.Provider, req RouteRequest) float64 {
    // 延迟评分 (40%)
    latencyScore := 1.0 / (p.LatencyP99 + 1)

    // 可用性评分 (30%)
    availabilityScore := p.Availability

    // 成本评分 (20%)
    costScore := 1.0 / (p.CostPer1K + 1)

    // 质量评分 (10%)
    qualityScore := p.QualityScore

    return latencyScore*0.4 + availabilityScore*0.3 + costScore*0.2 + qualityScore*0.1
}
```

### 3.3 Provider Adapter 模块

```go
// internal/adapter/registry.go
package adapter

import (
    "context"
    "sync"
)

type Registry struct {
    mu        sync.RWMutex
    providers map[string]Provider
    fallback  map[string]string
    health    map[string]*HealthStatus
}

type Provider interface {
    Name() string
    Call(ctx context.Context, req interface{}) (interface{}, error)
    HealthCheck(ctx context.Context) error
    GetCapabilities() Capabilities
}

type Capabilities struct {
    SupportsStreaming bool
    SupportsFunctionCall bool
    SupportsVision bool
    MaxTokens int
    Models []string
}

type HealthStatus struct {
    IsHealthy  bool
    Latency   time.Duration
    LastCheck time.Time
}

func NewRegistry() *Registry {
    return &Registry{
        providers: make(map[string]Provider),
        fallback:  make(map[string]string),
        health:   make(map[string]*HealthStatus),
    }
}

func (r *Registry) Register(name string, p Provider, fallback string) {
    r.mu.Lock()
    defer r.mu.Unlock()

    r.providers[name] = p
    if fallback != "" {
        r.fallback[name] = fallback
    }

    // 启动健康检查
    go r.healthCheckLoop(name, p)
}

func (r *Registry) Get(name string) (Provider, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    p, ok := r.providers[name]
    if !ok {
        return nil, ErrProviderNotFound
    }

    // 检查健康状态
    if health, ok := r.health[name]; ok && !health.IsHealthy {
        // 尝试fallback
        if fallback, ok := r.fallback[name]; ok {
            return r.providers[fallback], nil
        }
    }

    return p, nil
}

func (r *Registry) healthCheckLoop(name string, p Provider) {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        err := p.HealthCheck(ctx)
        cancel()

        r.mu.Lock()
        r.health[name] = &HealthStatus{
            IsHealthy: err == nil,
            LastCheck: time.Now(),
        }
        r.mu.Unlock()
    }
}
```

### 3.4 计费服务模块

```go
// internal/service/billing.go
package service

import (
    "context"
    "decimal"
    "llm-gateway/internal/model"
)

type BillingService struct {
    repo        *repository.BillingRepository
    balanceMgr  *BalanceManager
    notifier    *WebhookNotifier
}

type Money struct {
    Amount   decimal.Decimal
    Currency string
}

func (s *BillingService) ProcessRequest(ctx context.Context, req *model.LLMRequest) (*model.BillingRecord, error) {
    // 1. 预扣额度
    estimatedCost := s.EstimateCost(req)
    reserved, err := s.balanceMgr.Reserve(ctx, req.UserID, estimatedCost)
    if err != nil {
        return nil, ErrInsufficientBalance
    }

    // 2. 处理请求（实际扣费）
    actualCost := s.CalculateActualCost(req.Response)

    // 3. 补偿差额
    diff := actualCost.Sub(reserved.Amount)
    if diff.IsPositive() {
        err = s.balanceMgr.Charge(ctx, req.UserID, diff)
    } else if diff.IsNegative() {
        err = s.balanceMgr.Refund(ctx, req.UserID, diff.Abs())
    }

    // 4. 记录账单
    record := &model.BillingRecord{
        UserID:         req.UserID,
        RequestID:       req.ID,
        Model:          req.Model,
        PromptTokens:   req.Response.Usage.PromptTokens,
        CompletionTokens: req.Response.Usage.CompletionTokens,
        Amount:         actualCost,
        Status:         model.BillingStatusSettled,
    }

    err = s.repo.Create(ctx, record)
    if err != nil {
        // 记录失败，触发补偿
        s.notifier.NotifyBillingAnomaly(ctx, record, err)
    }

    return record, nil
}

func (s *BillingService) EstimateCost(req *model.LLMRequest) Money {
    // 使用模型定价估算
    price := s.repo.GetModelPrice(req.Model)

    promptCost := decimal.NewFromInt(int64(req.Messages.Tokens()))
        .Mul(price.InputPer1K)
        .Div(decimal.NewFromInt(1000))

    // 估算输出
    estimatedOutput := decimal.NewFromInt(int64(req.Options.MaxTokens))
    outputCost := estimatedOutput
        .Mul(price.OutputPer1K)
        .Div(decimal.NewFromInt(1000))

    total := promptCost.Add(outputCost)

    return Money{Amount: total.Round(2), Currency: "USD"}
}
```

### 3.5 风控服务模块

```go
// internal/service/risk.go
package service

import (
    "llm-gateway/internal/model"
)

type RiskService struct {
    rules       []RiskRule
    rateLimiter *RateLimiter
}

type RiskRule struct {
    Name      string
    Condition func(*model.LLMRequest, *model.User) bool
    Score     int
    Action    RiskAction
}

type RiskAction string

const (
    RiskActionAllow  RiskAction = "allow"
    RiskActionBlock  RiskAction = "block"
    RiskActionReview RiskAction = "review"
)

func (s *RiskService) Evaluate(ctx context.Context, req *model.LLMRequest) *RiskResult {
    var totalScore int
    var triggers []string

    user := s.getUser(ctx, req.UserID)

    for _, rule := range s.rules {
        if rule.Condition(req, user) {
            totalScore += rule.Score
            triggers = append(triggers, rule.Name)
        }
    }

    // 决策
    if totalScore >= 70 {
        return &RiskResult{
            Action:  RiskActionBlock,
            Score:   totalScore,
            Triggers: triggers,
        }
    } else if totalScore >= 40 {
        return &RiskResult{
            Action:  RiskActionReview,
            Score:   totalScore,
            Triggers: triggers,
        }
    }

    return &RiskResult{
        Action:  RiskActionAllow,
        Score:   totalScore,
    }
}

// 预定义风控规则
func DefaultRiskRules() []RiskRule {
    return []RiskRule{
        {
            Name: "high_velocity",
            Condition: func(req *model.LLMRequest, user *model.User) bool {
                return req.TokensPerMinute > 1000
            },
            Score: 30,
            Action: RiskActionBlock,
        },
        {
            Name: "new_account_high_value",
            Condition: func(req *model.LLMRequest, user *model.User) bool {
                return user.AccountAgeDays < 7 && req.EstimatedCost > 100
            },
            Score: 35,
            Action: RiskActionReview,
        },
        {
            Name: "unusual_model",
            Condition: func(req *model.LLMRequest, user *model.User) bool {
                return !user.PreferredModels.Contains(req.Model)
            },
            Score: 15,
            Action: RiskActionReview,
        },
    }
}
```

---

## 4. 数据流设计

### 4.1 请求处理流程

```
用户请求
    │
    ▼
┌─────────────────┐
│  API Gateway    │
│  • 限流         │
│  • 鉴权         │
│  • 日志         │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  路由决策      │
│  • 模型映射    │
│  • 供应商选择 │
└────────┬────────┘
         │
    ┌────┴────┐
    │         │
    ▼         ▼
┌────────┐ ┌────────┐
│ Provider│ │ Fallback│
│  A     │ │  B     │
└────┬───┘ └────┬───┘
     │         │
     └────┬────┘
          │
          ▼
┌─────────────────┐
│  计费处理       │
│  • 预扣         │
│  • 实际扣费    │
│  • 记录        │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  响应返回      │
└─────────────────┘
```

### 4.2 异步处理流程

```
请求处理
    │
    ▼
┌─────────────────┐
│  同步：预扣+执行 │
│                 │
│  • 预扣额度     │
│  • 调用供应商   │
│  • 实际扣费    │
└────────┬────────┘
         │
    ┌────┴────┐
    │         │
    ▼         ▼
┌────────┐ ┌────────┐
│ 同步响应 │ │ 异步队列│
│         │ │        │
│ • 返回   │ │ • 记录使用量│
│ • 更新   │ │ • 统计    │
│   余额   │ │ • 对账    │
└────────┘ └────┬────┘
                 │
                 ▼
          ┌─────────────┐
          │ Kafka Topic │
          │ • usage     │
          │ • billing   │
          │ • audit     │
          └──────┬──────┘
                 │
          ┌─────┴─────┐
          │           │
          ▼           ▼
    ┌─────────┐ ┌─────────┐
    │ 消费者   │ │ 消费者   │
    │ • 写入DB │ │ • 对账   │
    │ • 监控   │ │ • 告警   │
    └─────────┘ └─────────┘
```

---

## 5. API 设计规范

### 5.1 RESTful API 设计

```yaml
# openapi.yaml
openapi: 3.0.3
info:
  title: LLM Gateway API
  version: 1.0.0
  description: Enterprise LLM Gateway API

servers:
  - url: https://api.lgateway.com/v1
    description: Production server
  - url: https://staging-api.lgateway.com/v1
    description: Staging server

paths:
  /chat/completions:
    post:
      summary: Create a chat completion
      operationId: createChatCompletion
      tags:
        - Chat
      security:
        - BearerAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ChatCompletionRequest'
      responses:
        '200':
          description: Successful response
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ChatCompletionResponse'
        '400':
          $ref: '#/components/responses/BadRequest'
        '401':
          $ref: '#/components/responses/Unauthorized'
        '429':
          $ref: '#/components/responses/RateLimited'
        '500':
          $ref: '#/components/responses/InternalServerError'

components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT

  schemas:
    ChatCompletionRequest:
      type: object
      required:
        - model
        - messages
      properties:
        model:
          type: string
          description: Model identifier
        messages:
          type: array
          items:
            $ref: '#/components/schemas/Message'
        temperature:
          type: number
          minimum: 0
          maximum: 2
          default: 1.0
        max_tokens:
          type: integer
          minimum: 1
          maximum: 32000
        stream:
          type: boolean
          default: false

    Message:
      type: object
      required:
        - role
        - content
      properties:
        role:
          type: string
          enum: [system, user, assistant]
        content:
          type: string

  responses:
    BadRequest:
      description: Bad request
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Error'
    Unauthorized:
      description: Unauthorized
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Error'
    RateLimited:
      description: Rate limited
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Error'
    InternalServerError:
      description: Internal server error
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Error'
```

### 5.2 错误响应格式

```json
{
  "error": {
    "code": "BILLING_001",
    "message": "Insufficient balance",
    "message_i18n": {
      "zh_CN": "余额不足",
      "en_US": "Insufficient balance"
    },
    "details": {
      "required": 100.00,
      "available": 50.00,
      "top_up_url": "/v1/billing/top-up"
    },
    "trace_id": "req_abc123",
    "retryable": false,
    "doc_url": "https://docs.lgateway.com/errors/billing-001"
  }
}
```

---

## 6. 数据库设计

### 6.1 核心表结构

```sql
-- 用户表
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(100),
    tenant_id BIGINT REFERENCES tenants(id),
    role VARCHAR(20) DEFAULT 'user',
    status VARCHAR(20) DEFAULT 'active',
    mfa_enabled BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- API Keys表
CREATE TABLE api_keys (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    key_prefix VARCHAR(20) NOT NULL,
    description VARCHAR(200),
    permissions JSONB DEFAULT '{}',
    rate_limit_rpm INT DEFAULT 60,
    rate_limit_tpm INT DEFAULT 100000,
    status VARCHAR(20) DEFAULT 'active',
    expires_at TIMESTAMP,
    last_used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 租户表
CREATE TABLE tenants (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    plan VARCHAR(20) DEFAULT 'free',
    status VARCHAR(20) DEFAULT 'active',
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 账单记录表
CREATE TABLE billing_records (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    tenant_id BIGINT REFERENCES tenants(id),
    request_id VARCHAR(64) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    model VARCHAR(50) NOT NULL,
    prompt_tokens INT NOT NULL,
    completion_tokens INT NOT NULL,
    amount DECIMAL(10, 4) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    status VARCHAR(20) DEFAULT 'settled',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 使用量记录表
CREATE TABLE usage_records (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    tenant_id BIGINT REFERENCES tenants(id),
    api_key_id BIGINT REFERENCES api_keys(id),
    request_id VARCHAR(64) NOT NULL,
    model VARCHAR(50) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    prompt_tokens INT DEFAULT 0,
    completion_tokens INT DEFAULT 0,
    latency_ms INT,
    status_code INT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 索引
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_tenant ON users(tenant_id);
CREATE INDEX idx_api_keys_user ON api_keys(user_id);
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX idx_billing_user ON billing_records(user_id, created_at);
CREATE INDEX idx_billing_tenant ON billing_records(tenant_id, created_at);
CREATE INDEX idx_usage_user ON usage_records(user_id, created_at);
CREATE INDEX idx_usage_request ON usage_records(request_id);
```

---

## 7. 消息队列运维简化

### 7.1 Kafka运维挑战分析

| 挑战 | 影响 | 简化方案 |
|------|------|----------|
| 集群管理复杂 | 运维成本高 | 使用托管服务 |
| 分区副本同步 | 数据延迟 | 优化配置 |
| 消费者组管理 | 消费积压 | 简化架构 |
| 监控告警 | 噪声过多 | 精简指标 |
| 容量规划 | 扩展困难 | 自动化伸缩 |

### 7.2 托管Kafka服务选型

```yaml
# 消息队列服务选型
recommended:
  # 阿里云Kafka（国内）
  aliyun:
    type: managed
    version: 2.2.0
    features:
      - 自动分区重平衡
      - 死信队列支持
      - 跨可用区容灾
    ops_benefits:
      - 免运维
      - SLA 99.9%
      - 按量计费

  # AWS MSK（海外）
  aws_msk:
    type: managed
    version: 2.8.0
    features:
      - MSK Serverless免容量规划
      - 精细访问控制
    ops_benefits:
      - 与AWS生态集成
      - 托管升级

alternatives:
  # 轻量级替代方案
  redis_streams:
    use_case: "低延迟小消息"
    limitations:
      - 无持久化保证
      - 单线程消费

  # 业务简单时的选择
  database_queues:
    use_case: "消息量<1000/s"
    limitations:
      - 性能有限
      - 需自行实现重试
```

### 7.3 Topic设计简化

```python
# 简化的Topic设计 - 从原来的10+个精简为4个
TOPIC_DESIGN = {
    # 核心业务Topic
    "llm.requests": {
        "partitions": 6,
        "retention": "7d",
        "description": "LLM请求流转"
    },

    # 异步计费Topic
    "llm.billing": {
        "partitions": 3,
        "retention": "30d",
        "description": "计费流水"
    },

    # 通知事件Topic
    "llm.events": {
        "partitions": 3,
        "retention": "3d",
        "description": "各类事件通知"
    },

    # 监控数据Topic
    "llm.metrics": {
        "partitions": 1,
        "retention": "1d",
        "description": "原始监控数据"
    }
}
```

### 7.4 消费者组简化

```python
# 简化的消费者组设计
class SimplifiedConsumerGroup:
    """简化消费者组管理"""

    # 原来：每个服务多个消费者组
    # 优化后：一个服务一个消费者组

    def __init__(self):
        self.groups = {
            "router-service": {
                "topics": ["llm.requests"],
                "consumers": 3,  # 与分区数匹配
                "strategy": "round_robin"
            },
            "billing-service": {
                "topics": ["llm.billing"],
                "consumers": 2,
                "strategy": "failover"
            },
            "notification-service": {
                "topics": ["llm.events"],
                "consumers": 1,
                "strategy": "broadcast"
            }
        }

    def get_consumer_count(self, group: str) -> int:
        """自动计算消费者数量"""
        return self.groups[group]["consumers"]
```

### 7.5 自动化运维脚本

```bash
#!/bin/bash
# scripts/kafka-ops.sh - Kafka运维自动化

set -e

# 1. 主题健康检查
check_topics() {
    echo "=== 检查Topic状态 ==="
    kafka-topics.sh --bootstrap-server $KAFKA_BROKER --list | while read topic; do
        partitions=$(kafka-topics.sh --bootstrap-server $KAFKA_BROKER \
            --topic $topic --describe | grep -c "Leader:")
        lag=$(kafka-consumer-groups.sh --bootstrap-server $KAFKA_BROKER \
            --group $(get_group_for_topic $topic) \
            --describe | awk '{sum+=$6} END {print sum}')

        echo "Topic: $topic, Partitions: $partitions, Lag: $lag"

        if [ $lag -gt 1000 ]; then
            alert "消费积压告警: $topic 积压 $lag 条"
        fi
    done
}

# 2. 自动创建Topic（幂等）
ensure_topics() {
    for topic in "${!TOPIC_DESIGN[@]}"; do
        config="${TOPIC_DESIGN[$topic]}"
        kafka-topics.sh --bootstrap-server $KAFKA_BROKER \
            --topic $topic --create \
            --partitions ${config[partitions]} \
            --replication-factor 3 \
            --config retention.ms=${config[retention]} \
            2>/dev/null || echo "Topic $topic already exists"
    done
}

# 3. 消费延迟监控
monitor_lag() {
    for group in $(kafka-consumer-groups.sh --bootstrap-server $KAFKA_BROKER \
        --list 2>/dev/null); do
        lag=$(kafka-consumer-groups.sh --bootstrap-server $KAFKA_BROKER \
            --group $group --describe | awk '{sum+=$6} END {print sum}')

        prometheus_pushgateway "kafka_consumer_lag" $lag "group=$group"
    done
}
```

### 7.6 监控指标精简

```yaml
# 精简的Kafka监控指标 - 避免噪声
kafka_metrics:
  essential:
    - name: kafka_consumer_group_lag_max
      description: 最大消费延迟
      alert_threshold: 1000

    - name: kafka_topic_partition_under_replicated
      description: 副本不同步数
      alert_threshold: 0

    - name: kafka_server_broker_topic_messages_in_total
      description: 消息入站速率
      alert_threshold: rate_change > 50%

  optional:
    # 以下指标仅在排查问题时启用
    - kafka_network_request_metrics
    - kafka_consumer_fetch_manager_metrics
    - kafka_producer_metrics
```

### 7.7 容量规划自动化

```python
# 自动容量规划
class KafkaCapacityPlanner:
    """Kafka容量自动规划"""

    def calculate_requirements(self, metrics: dict) -> dict:
        """基于实际流量计算容量"""
        # 峰值QPS
        peak_qps = metrics["peak_qps"]

        # 平均消息大小
        avg_msg_size = metrics["avg_msg_size_kb"] * 1024

        # 保留期
        retention_days = 7

        # 计算所需磁盘
        disk_per_day = peak_qps * avg_msg_size * 86400
        total_disk = disk_per_day * retention_days

        # 推荐配置
        return {
            "partitions": min(peak_qps // 100, 12),  # 最大12分区
            "replication_factor": 3,
            "disk_gb": total_disk / (1024**3),
            "broker_count": 3,
            "scaling_trigger": "disk_usage > 70%"
        }
```

### 7.8 故障自愈机制

```python
# Kafka故障自愈
class KafkaSelfHealing:
    """Kafka自愈机制"""

    def __init__(self):
        self.healing_rules = {
            "under_replicated": {
                "detect": "partition.replicas - in.sync.replicas > 0",
                "action": "trigger_preferred_reelection",
                "cooldown": 300  # 5分钟
            },
            "controller_failover": {
                "detect": "controller_epoch跳跃",
                "action": "等待自动选举",
                "cooldown": 60
            },
            "partition_offline": {
                "detect": "leader == -1",
                "action": "assign_new_leader",
                "cooldown": 60
            }
        }

    async def check_and_heal(self):
        """定期检查并自愈"""
        for rule_name, rule in self.healing_rules.items():
            if self.should_heal(rule_name):
                await self.execute_healing(rule_name, rule)
```

---

## 7. 一致性验证

### 7.1 与现有文档一致性

| 设计项 | 对应文档 | 一致性 |
|--------|----------|--------|
| Provider Adapter | `architecture_solution_v1.md` | ✅ |
| 路由策略 | `architecture_solution_v1.md` | ✅ |
| 计费精度 | `business_solution_v1.md` | ✅ |
| 安全机制 | `security_solution_v1.md` | ✅ |
| API版本管理 | `api_solution_v1.md` | ✅ |
| 错误码体系 | `api_solution_v1.md` | ✅ |
| 限流机制 | `p1_optimization_solution_v1.md` | ✅ |
| Webhook | `p1_optimization_solution_v1.md` | ✅ |

---

## 8. 实施计划

### 8.1 开发阶段

| 阶段 | 内容 | 周数 |
|------|------|------|
| Phase 1 | 基础设施 + API Gateway | 3周 |
| Phase 2 | 核心服务开发 | 4周 |
| Phase 3 | 集成测试 | 2周 |
| Phase 4 | 性能优化 | 2周 |

---

**文档状态**：详细技术架构设计
**关联文档**：
- `architecture_solution_v1_2026-03-18.md`
- `api_solution_v1_2026-03-18.md`
- `security_solution_v1_2026-03-18.md`
