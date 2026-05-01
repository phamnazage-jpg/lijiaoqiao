# Supply-Intelligence 高层技术设计文档（HLD）

> 文档版本：v1.0
> 撰写日期：2026-04-27
> 撰写人：TechLead
> 评审状态：待开发排期确认

---

## 1. 设计目标与范围

### 1.1 设计目标

为 Supply-Intelligence（供应链智能增强系统）建立可生产落地的技术方案，支撑以下业务目标的达成：

| 目标编号 | 目标描述 | 技术侧支撑 |
|---------|---------|-----------|
| BG-01 | 供应商账号异常状态标记平均时间 ≤ 15 分钟 | 探针调度周期 5 分钟 + 状态机自动迁移 |
| BG-02 | 新模型从发布到可售卖平均时间 ≤ 4 小时 | 每小时全网扫描 + 30 分钟内完成准入测试 |
| BG-03 | 供应商账号失效导致的用户可见错误率下降 80% | 探针实时标记 + Gateway 状态查询接口 P99 < 50ms |
| BG-04 | 人工维护供应商基础信息工作量减少 70% | 自动发现 + 自动测试 + 自动注册 |

### 1.2 设计范围

In Scope（按 Phase 交付）：

- **Phase 1**：供应商品质探针（模块 A）+ 运营工作台观测视图
- **Phase 2**：全网模型发现（模块 B）+ 模型准入测试（模块 C）
- **Phase 3**：账号自动注册（模块 D）+ 运营工作台完整干预能力

Out of Scope：

- 供应商侧计费系统对接与自动充值（OOS-01）
- 动态定价算法（OOS-02）
- TOS 法律合规性自动审查（OOS-03）
- 不支持公开注册接口的供应商自动注册（OOS-04）
- 模型版本语义级差异分析（OOS-05）
- 跨供应商模型能力等价性判定（OOS-06）

---

## 2. 系统架构总览

### 2.1 架构图（逻辑分层）

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              消费层                                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐ │
│  │   Gateway    │  │  运营工作台   │  │ NewAPI/Sub2 │  │   告警通知       │ │
│  │  (路由决策)   │  │  (Dashboard) │  │   API 适配层 │  │ (钉钉/企微/邮件) │ │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └─────────────────┘ │
└─────────┼─────────────────┼─────────────────┼──────────────────────────────┘
          │                 │                 │
          │ GET /health     │ REST/WebSocket  │ gRPC/REST
          │ (P99<50ms)      │                 │
┌─────────┼─────────────────┼─────────────────┼──────────────────────────────┐
│         │                 │                 │                              │
│  ┌──────▼─────────────────▼─────────────────▼──────────────────────────┐  │
│  │                     API Gateway Layer                               │  │
│  │  /api/v1/supply-intelligence/*  (独立运行)                          │  │
│  │  /internal/supply-intelligence/* (集成运行)                         │  │
│  └──────┬──────────────────────────────────────────────────────────────┘  │
│         │                                                                  │
│  ┌──────▼──────────────────────────────────────────────────────────────┐  │
│  │                     Application Service Layer                        │  │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌──────────────┐  │  │
│  │  │ProbeService │ │DiscoverySvc │ │AdmissionSvc │ │AutoRegSvc    │  │  │
│  │  │ (品质探针)   │ │(模型发现)   │ │(准入测试)   │ │(自动注册)    │  │  │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └──────────────┘  │  │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌──────────────┐  │  │
│  │  │StateMachine │ │PricingDB    │ │HealthBoard │ │OpsWorkBench │  │  │
│  │  │ (状态机)    │ │(定价数据库)  │ │(健康大盘)   │ │(运营工作台)   │  │  │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └──────────────┘  │  │
│  └──────┬──────────────────────────────────────────────────────────────┘  │
│         │                                                                  │
│  ┌──────▼──────────────────────────────────────────────────────────────┐  │
│  │                     Domain & Infrastructure Layer                    │  │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌──────────────┐  │  │
│  │  │ProbeExecutor│ │Scanner      │ │TestRunner   │ │BrowserEngine │  │  │
│  │  │ (探针执行器) │ │(扫描器)     │ │(测试执行器)  │ │(浏览器自动化) │  │  │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └──────────────┘  │  │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌──────────────┐  │  │
│  │  │Scheduler    │ │AuditEmitter │ │KMSClient    │ │RateLimiter   │  │  │
│  │  │ (任务调度)   │ │(审计发射器)  │ │(KMS 客户端) │ │(限流器)      │  │  │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └──────────────┘  │  │
│  └──────┬──────────────────────────────────────────────────────────────┘  │
│         │                                                                  │
├─────────┼──────────────────────────────────────────────────────────────────┤
│         │                    外部依赖层                                   │
│  ┌──────▼──────┐ ┌─────────────┐ ┌─────────────┐ ┌────────────────────┐ │
│  │ PostgreSQL  │ │   Redis     │ │ Job Scheduler│ │ 供应商 API / Web    │ │
│  │ (主存储)    │ │  (缓存/队列) │ │ (Temporal/   │ │ (OpenAI/Anthropic/ │ │
│  │             │ │             │ │ 内部 Cron)   │ │  阿里云/百度等)     │ │
│  └─────────────┘ └─────────────┘ └─────────────┘ └────────────────────┘ │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐                        │
│  │  KMS 服务   │ │SMS/邮件网关  │ │  supply-api │                        │
│  │ (凭证加密)   │ │(验证码)     │ │ (现有服务)   │                        │
│  └─────────────┘ └─────────────┘ └─────────────┘                        │
└──────────────────────────────────────────────────────────────────────────┘
```

### 2.2 部署形态

本系统支持两种运行模式，对应两套构建产物：

| 模式 | 构建产物 | 数据库 Schema | HTTP 前缀 | 适用场景 |
|------|---------|--------------|-----------|---------|
| **独立运行** | `cmd/supply-intelligence/main.go` → 独立二进制/容器 | `supply_intelligence_*` + 独立连接池 | `/api/v1/supply-intelligence/` | 外部用户仅需供应链管理能力 |
| **集成运行** | `pkg/supplyintelligence/plugin.go` → Go module | `supply_intelligence_*` + 共享连接池 | `/internal/supply-intelligence/` | 立交桥用户一体化供应链能力 |

**集成运行时**：主进程通过 `IntegrationPlugin` 接口注册各模块 Handler 与 Background Worker，通过配置开关 `supply_intelligence.enabled_modules` 控制子模块挂载。

### 2.3 核心组件职责

| 组件 | 职责 | 对应 PRD 模块 |
|------|------|-------------|
| `ProbeService` | 调度探针任务、解析结果、驱动状态机 | 模块 A |
| `DiscoveryService` | 扫描供应商模型列表、比对差异、生成候选 | 模块 B |
| `AdmissionService` | 调度准入测试、评估结果、生成 package 草稿 | 模块 C |
| `AutoRegistrationService` | 触发注册流程、编排验证步骤、凭证加密存储 | 模块 D |
| `HealthBoardService` | 聚合探针/测试/注册数据，生成健康大盘指标 | 模块 E（数据） |
| `OpsWorkbenchService` | 处理人工干预请求、权限校验、审计记录 | 模块 E（操作） |
| `PricingDBService` | 维护模型定价数据库、支持远程更新与本地 fallback | 竞品对标 |
| `StateMachine` | 统一状态迁移规则、校验、乐观锁冲突处理 | 通用 |
| `AuditEmitter` | 异步发射审计事件、脱敏、批量写入 | 通用 |

---

## 3. 核心模块设计

### 3.1 供应商品质探针（Supply Health Probe）

#### 3.1.1 探针类型与判定规则

| 探针类型 | 请求方式 | 成功判定 | 失败判定 | inconclusive |
|---------|---------|---------|---------|-------------|
| `connectivity` | HEAD /models 或等效端点 | HTTP 2xx，latency ≤ 10s | HTTP 401/403，TCP/DNS 失败，latency > 10s | HTTP 429，HTTP 5xx，响应体解析失败 |
| `quota` | 调用额度查询接口（若供应商支持） | 返回可用额度 > 0 | 返回额度 = 0 或接口报错 | 接口不存在或返回非预期格式 |
| `key_validity` | 发送一条低成本 completion 请求 | HTTP 2xx，响应体合规 | HTTP 401/403，响应格式不合法 | HTTP 429，超时 |

#### 3.1.2 状态机规则

```
                    ┌──────────────┐
                    │   active     │
                    └──────┬───────┘
                           │ 1次明确失败
                           ▼
                    ┌──────────────┐
         ┌─────────│  suspended   │◄────────┐
         │ 恢复成功 └──────┬───────┘         │
         │               │ 连续3次失败      │ 429 inconclusive
         ▼               ▼                  │ (不计入失败)
┌──────────────┐  ┌──────────────┐         │
│   active     │  │  disabled    │─────────┘
│ (人工恢复)   │  │              │
└──────────────┘  └──────────────┘
```

**规则约束**：
- `active` → `suspended`：需 1 次明确失败（HTTP 401/403/超时 > 10s / TCP 不可达）。
- `suspended` → `disabled`：需连续 3 次探针失败，每次间隔 ≥ 5 分钟。
- `suspended` → `active`：1 次探针成功即可恢复。
- `disabled` → `active`：仅允许人工操作触发，系统不自动恢复。
- `active` → `disabled` 的直接迁移被禁止，必须经过 `suspended`。

#### 3.1.3 探针调度策略

- **周期**：默认 5 分钟/账号，可通过配置 `probe.interval_seconds` 热更新（60 秒内生效）。
- **并发**：使用 Worker Pool 模型，默认池大小 = 50，单账号探针超时 = 15 秒。
- **退避**：遇到 429 时，指数退避 1min → 2min → 4min，最多重试 3 次，仍 429 则本次跳过。
- **分批**：按 `platform` 分组错峰，避免同时冲击同一供应商。

#### 3.1.4 风险评分模型

```go
type RiskAssessment struct {
    Score       int    // 0-100
    Reason      string // 机器可读原因码
    Severity    string // info / warning / critical
    SuggestedAction string // none / suspend / disable / investigate
}
```

评分规则（示例）：
- 连通性失败 + 额度正常 = 60 分，`warning`
- 连通性失败 + 密钥无效 = 80 分，`critical`，建议 `suspend`
- 连续 2 次 `warning` = 提升至 `critical`

### 3.2 全网模型发现（Model Discovery）

#### 3.2.1 扫描源与适配器

每个供应商实现 `ModelListScanner` 接口：

```go
type ModelListScanner interface {
    // 返回当前供应商所有 model_id 列表
    Scan(ctx context.Context) ([]ModelInfo, error)
    // 供应商唯一标识
    Platform() string
    // 扫描器健康检查
    HealthCheck(ctx context.Context) error
}
```

**扫描源类型**：

| 类型 | 示例供应商 | 实现方式 | 优先级 |
|------|-----------|---------|--------|
| REST API | OpenAI, Anthropic | HTTP GET /models，解析 JSON | 高 |
| 文档页面 | 部分国内供应商 | Playwright / colly 抓取 HTML | 中 |
| RSS/变更日志 | HuggingFace | RSS 订阅 + 解析 | 中 |
| 社区监控 | HN, Twitter | 外部数据源接入（Phase 2 后） | 低 |

#### 3.2.2 发现比对算法

1. 获取供应商侧当前 `model_id` 集合 S_current。
2. 查询本平台 `supply_packages` 中 `platform = X` 且 `status ∈ {active, paused, draft}` 的 `model` 集合 S_platform。
3. 差集计算：
   - `S_current - S_platform` → 新增模型，插入 `model_candidates`（`discovered`）。
   - `S_platform - S_current` → 疑似下架模型，生成告警待办，但 **不自动变更** `supply_packages.status`。
4. 重命名检测（边缘场景 B1）：旧 ID 消失 + 新 ID 出现 + 能力描述相似度 > 0.85 → 生成运营待办，不做自动关联。

#### 3.2.3 扫描周期与容错

- **周期**：默认 1 小时，配置项 `discovery.interval_seconds`。
- **分页容错**：若某页返回 500，已获取页正常处理，失败页在下一周期重试（FP-07）。
- **缓存 TTL**：扫描结果在 Redis 缓存 30 分钟，避免重复请求供应商接口。

### 3.3 模型准入测试（Model Admission Test）

#### 3.3.1 测试流水线

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│  discovered │───►│   queued    │───►│   testing   │───►│ test_passed │
│  (发现)     │    │  (入队列)   │    │  (执行中)   │    │ (测试通过)  │
└─────────────┘    └─────────────┘    └──────┬──────┘    └─────────────┘
                                             │
                                    ┌────────▼────────┐
                                    │   test_failed   │
                                    │   (测试失败)     │
                                    └─────────────────┘
```

#### 3.3.2 测试维度与通过标准

| 维度 | 检查项 | 通过标准 | 权重 |
|------|--------|---------|------|
| 接口可用性 | HTTP 状态码 | 200 | 必须 |
| 响应格式合规 | JSON Schema 校验（OpenAI-compatible） | 100% 通过 | 必须 |
| 延迟 | P50 / P99 | P50 < 5s, P99 < 30s | 必须 |
| Token 计数一致性 | 请求 token 数 vs 响应 usage 字段 | 误差 ≤ 5% | 必须 |
| 错误码映射 | 发送无效参数，验证错误码 | 返回 4xx 且 body 含 `error` 字段 | 必须 |
| 功能覆盖 | chat / completion / embedding | 按模型类型选择对应 endpoint | 必须 |

**通过定义**：所有“必须”维度通过，且无任何测试用例超时（超时阈值 60 秒/用例）。

#### 3.3.3 测试隔离

- 准入测试必须使用 **独立测试账号**（`supply_accounts` 中 `usage_type = test`），禁止触碰生产账号。
- 测试账号被探针标记为 `suspended` 时，准入测试流水线立即失败，原因写入 `test_account_unavailable`（FP-04）。
- 测试请求添加 `X-Supply-Intelligence-Test: true` 头部，便于供应商侧识别（如支持）。

#### 3.3.4 测试用例集管理

- 测试用例由 QA 团队维护，存储于 `configs/admission_tests/` 目录下，按模型类型分组。
- 用例格式：YAML 定义请求模板 + 预期响应断言（JSONPath）。
- 每类模型最少 5 个用例，覆盖正常请求、超长输入、特殊字符、空输入、错误参数。
- 用例变更后，系统 60 秒内热加载，不重启进程（AC-12）。

### 3.4 账号自动注册（Account Auto-Registration）

#### 3.4.1 注册流程状态机

```
┌─────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐
│ pending │──►│registering│──►│verifying │──►│ applying │──►│completed │
│ (触发)  │   │ (注册中)  │   │ (验证中)  │   │ (申请Key) │   │ (完成)   │
└─────────┘   └────┬─────┘   └────┬─────┘   └────┬─────┘   └────┬─────┘
                   │              │              │              │
                   ▼              ▼              ▼              ▼
            ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐
            │  failed  │   │  failed  │   │  failed  │   │ dead_letter│
            │ (注册失败)│   │ (验证失败)│   │ (申请失败)│   │ (死信)    │
            └──────────┘   └──────────┘   └──────────┘   └──────────┘
```

#### 3.4.2 供应商注册适配器

每个供应商实现 `RegistrationAdapter` 接口：

```go
type RegistrationAdapter interface {
    // 是否支持自动注册
    IsSupported() bool
    // 执行注册，返回临时凭证
    Register(ctx context.Context, req RegistrationRequest) (*RegistrationResult, error)
    // 验证账号（邮件/SMS）
    Verify(ctx context.Context, accountID string, code string) error
    // 申请 API Key
    ApplyAPIKey(ctx context.Context, accountID string) (string, error)
    // 平台标识
    Platform() string
}
```

**实现策略**：
- 优先使用官方注册 API（REST）。
- 无官方 API 时，使用 Playwright 浏览器自动化作为 fallback。
- 浏览器自动化流程需记录 DOM 选择器版本，供应商前端改版时触发告警。

### Playwright Fallback 的运维复杂度说明
- **额外依赖**：需要 Playwright 浏览器二进制（Chromium/ Firefox/WebKit），Docker 镜像体积增加约 200MB
- **DOM版本管理**：每个供应商的注册表单需维护独立的选择器配置文件，供应商前端改版后需手动更新
- **CI/CD要求**：浏览器测试需要在有头模式下运行，CI 需配置`--headed`模式或使用 playwright-chromium
- **备选方案**：若 Playwright 维护成本过高，可考虑将自动注册范围缩小至仅有官方 API 的供应商，自动注册模块降级为"手动注册辅助"（仅生成注册任务工单，不自动执行）

#### 3.4.3 Fail-Closed 设计

- SMS/邮件网关返回 503 或超时 → 注册任务整体标记 `failed`，审计日志记录 `auto_register_failed`，**不向任何上游返回成功状态**（AC-09）。
- KMS 服务不可用时，明文凭证不得落盘；注册流程在加密步骤阻塞 60 秒，超时后任务标记 `failed`（FP-10）。
- 死信队列：失败任务 24 小时后自动重试，最多重试 3 次，最终进入 `dead_letter` 状态，触发人工告警。

### 3.5 供应商健康大盘（Health Board）

#### 3.5.1 指标聚合

| 指标 | 计算方式 | 刷新周期 |
|------|---------|---------|
| 账号健康度 | active 账号数 / (active + suspended) 账号数 | 实时（基于探针结果） |
| 模型覆盖率 | 平台 active 模型数 / 全网 discovered 模型数 | 每小时 |
| 探针成功率 | 最近 1 小时 success / total | 5 分钟 |
| 平均延迟 | 最近 1 小时探针 latency P50/P99 | 5 分钟 |
| 风险账号数 | risk_score ≥ 60 的账号数 | 实时 |
| 待处理候选数 | status = discovered 的 candidate 数 | 实时 |

#### 3.5.2 北极星指标（SFI）

```
SFI = (过去1小时成功探针账号数 / 应探针账号总数) ×
      (过去24小时进入active的新模型数 / 过去24小时发现的新模型总数)
```

- 目标值：SFI ≥ 0.95
- 采集周期：每小时计算一次，写入时序数据库（Prometheus 或独立 TSDB）。
- 连续 7 天 SFI < 0.70 触发项目失败判定线（止损条件 3）。

### 3.6 模型比价（Pricing Comparison）

#### 3.6.1 定价数据库设计

参考 LiteLLM `model_prices_and_context_window_backup.json`，维护以下字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `model_id` | VARCHAR(100) | 全局模型标识 |
| `platform` | VARCHAR(50) | 供应商 |
| `input_cost_per_token` | DECIMAL(18,12) | 输入 token 单价（美元） |
| `output_cost_per_token` | DECIMAL(18,12) | 输出 token 单价（美元） |
| `context_window` | INT | 上下文长度 |
| `max_tokens` | INT | 最大输出 token 数 |
| `supports_vision` | BOOLEAN | 是否支持视觉 |
| `supports_function_calling` | BOOLEAN | 是否支持函数调用 |
| `supports_batch` | BOOLEAN | 是否支持批量定价 |
| `tiered_pricing` | JSONB | 分层定价规则 |
| `updated_at` | TIMESTAMPTZ | 更新时间 |
| `source_hash` | VARCHAR(64) | 数据源 SHA256 |

**更新机制**：
- 主数据源：远程拉取 LiteLLM 镜像（可配置镜像源），SHA256 校验完整性（参考 Sub2API）。
- Fallback：本地缓存文件 `data/model_prices_fallback.json`，启动时若远程失败则加载本地。
- 自定义覆盖：平台可通过 `pricing_overrides` 表对特定供应商-模型组合设置覆盖价格。

#### 3.6.2 模型家族回退

参考 Sub2API 设计，对未知模型按命名规则回退到已知模型定价：

```go
// 回退规则（按优先级）
1. 精确匹配 model_id
2. 前缀匹配：gpt-4-turbo-2024-04-09 → gpt-4-turbo
3. 家族匹配：gpt-5.3-unknown → gpt-5.1
4. 能力匹配：claude-unknown-vision → claude-sonnet (若 supports_vision=true)
5. 默认回退：unknown → 平台默认定价（需人工审核）
```

回退决策记录到 `pricing_fallback_log`，供运营人员定期审查。

### 3.7 预测分析（Predictive Analytics）

#### 3.7.1 模型下线预测

基于以下信号生成预测：
- 供应商模型列表中该模型连续 3 个扫描周期未出现。
- 该模型近期（7 天）用量趋势下降 > 50%。
- 供应商官方发布 deprecation 公告（如有 RSS/公告源）。

预测结果写入 `predictions` 表，置信度 ≥ 0.7 时触发运营告警。

#### 3.7.2 供应商变动预测

- 监控供应商 API 文档变更频率、Rate Limit 调整、定价变更。
- 高频变动标记为 `unstable`，健康大盘中展示风险标签。

### 3.8 运营工作台（Operations Dashboard）

#### 3.8.1 核心视图

| 视图 | 内容 | 数据刷新 |
|------|------|---------|
| 待处理候选模型 | `discovered` / `test_failed` candidate 列表 | 实时（WebSocket 推送） |
| 账号健康列表 | 全部账号状态、最近探针时间、risk_score | 5 分钟轮询 |
| 状态变更待确认 | 系统建议的 `suspend` / `disable` 操作 + 人工确认按钮 | 实时 |
| 自动注册队列 | `pending` / `running` / `failed` 任务列表 | 实时 |
| 供应链覆盖率 | 覆盖率百分比、趋势图、竞品对比（如数据可用） | 每小时 |

#### 3.8.2 人工干预操作

| 操作 | 权限要求 | 效果 | 审计记录 |
|------|---------|------|---------|
| 一键确认上架 | `supply:ops:publish` | `draft` → `active` | `action=manual_publish` |
| 忽略此模型 | `supply:ops:ignore` | `discovered` → `ignored`，`ignored_until = NOW() + 7d` | `action=manual_ignore` |
| 手动触发探针 | `supply:ops:probe` | 立即执行单次探针 | `action=manual_probe` |
| 强制上架（测试失败） | `supply:ops:force_publish` | `draft` + `manually_forced=true`，需填写理由 | `action=manual_force_publish` |
| 暂停自动探针 | `supply:ops:pause_probe` | `auto_probe_enabled = false` | `action=pause_auto_probe` |

**并发控制**：所有干预操作使用乐观锁或幂等键（`IdempotencyKey`），重复操作返回 409 Conflict（FP-09）。

---

## 4. 数据模型设计

### 4.1 ER 关系图

```
┌────────────────────┐       ┌────────────────────┐       ┌────────────────────┐
│  supply_accounts   │       │ supply_intelligence│       │  supply_packages   │
│   (已有表，只读)    │◄──────│  _model_candidates │──────►│   (已有表，读写)   │
└────────────────────┘       └────────────────────┘       └────────────────────┘
         │                            │                            │
         │                            │                            │
         ▼                            ▼                            ▼
┌────────────────────┐       ┌────────────────────┐       ┌────────────────────┐
│si_probe_execution  │       │ si_pricing_db      │       │ si_predictions     │
│_logs               │       │                    │       │                    │
└────────────────────┘       └────────────────────┘       └────────────────────┘
         │
         │
         ▼
┌────────────────────┐       ┌────────────────────┐       ┌────────────────────┐
│si_auto_registration│       │ si_audit_events    │       │ si_health_metrics  │
│_tasks              │       │ (扩展字段)          │       │                    │
└────────────────────┘       └────────────────────┘       └────────────────────┘
```

### 4.2 核心表结构

#### 4.2.1 `supply_intelligence_model_candidates`

```sql
CREATE TABLE supply_intelligence_model_candidates (
    id              BIGSERIAL PRIMARY KEY,
    platform        VARCHAR(50) NOT NULL,
    model_id        VARCHAR(100) NOT NULL,
    model_name      VARCHAR(200),
    status          VARCHAR(20) NOT NULL DEFAULT 'discovered'
                    CHECK (status IN ('discovered','testing','test_passed','test_failed','ignored','expired')),
    discovered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    tested_at       TIMESTAMPTZ,
    failure_reason  TEXT,
    ignored_until   TIMESTAMPTZ,
    test_log_url    TEXT,               -- 测试日志对象存储路径
    package_draft_id BIGINT,            -- 关联 supply_packages.id (draft)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version         INT NOT NULL DEFAULT 1,  -- 乐观锁

    UNIQUE(platform, model_id)
);

CREATE INDEX idx_candidates_status ON supply_intelligence_model_candidates(status);
CREATE INDEX idx_candidates_discovered_at ON supply_intelligence_model_candidates(discovered_at);
CREATE INDEX idx_candidates_platform ON supply_intelligence_model_candidates(platform);
```

#### 4.2.2 `supply_intelligence_auto_registration_tasks`

```sql
CREATE TABLE supply_intelligence_auto_registration_tasks (
    id                  BIGSERIAL PRIMARY KEY,
    platform            VARCHAR(50) NOT NULL,
    task_type           VARCHAR(20) NOT NULL
                        CHECK (task_type IN ('register','verify','rotate_key')),
    status              VARCHAR(20) NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','running','completed','failed','dead_letter')),
    context             JSONB NOT NULL DEFAULT '{}',
    result_account_id   BIGINT,         -- 关联 supply_accounts.id
    failure_reason      TEXT,
    retry_count         INT NOT NULL DEFAULT 0,
    next_retry_at       TIMESTAMPTZ,
    credential_fingerprint VARCHAR(64),  -- API Key 哈希指纹，非明文
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version             INT NOT NULL DEFAULT 1
);

CREATE INDEX idx_reg_tasks_status ON supply_intelligence_auto_registration_tasks(status, next_retry_at);
CREATE INDEX idx_reg_tasks_platform ON supply_intelligence_auto_registration_tasks(platform);
```

#### 4.2.3 `supply_intelligence_probe_execution_logs`

```sql
CREATE TABLE supply_intelligence_probe_execution_logs (
    id              BIGSERIAL PRIMARY KEY,
    account_id      BIGINT NOT NULL,    -- supply_accounts.id
    probe_type      VARCHAR(20) NOT NULL
                    CHECK (probe_type IN ('connectivity','quota','key_validity')),
    result          VARCHAR(20) NOT NULL
                    CHECK (result IN ('success','failure','inconclusive')),
    http_status     INT,
    latency_ms      INT,
    error_code      VARCHAR(50),
    error_message   TEXT,
    risk_score      INT,
    risk_reason     VARCHAR(100),
    executed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    request_id      VARCHAR(64) NOT NULL
);

CREATE INDEX idx_probe_logs_account_executed
    ON supply_intelligence_probe_execution_logs(account_id, executed_at DESC);
CREATE INDEX idx_probe_logs_executed_at
    ON supply_intelligence_probe_execution_logs(executed_at)
    WHERE executed_at < NOW() - INTERVAL '30 days';  -- 用于清理
```

**保留策略**：30 天自动清理，使用 PostgreSQL 分区表按 `executed_at` 月分区。

#### 4.2.4 `supply_intelligence_pricing_db`

```sql
CREATE TABLE supply_intelligence_pricing_db (
    id                      BIGSERIAL PRIMARY KEY,
    model_id                VARCHAR(100) NOT NULL,
    platform                VARCHAR(50) NOT NULL,
    input_cost_per_token    DECIMAL(18,12) NOT NULL,
    output_cost_per_token   DECIMAL(18,12) NOT NULL,
    context_window          INT,
    max_tokens              INT,
    supports_vision         BOOLEAN DEFAULT FALSE,
    supports_function_calling BOOLEAN DEFAULT FALSE,
    supports_batch          BOOLEAN DEFAULT FALSE,
    tiered_pricing          JSONB,
    source_hash             VARCHAR(64),
    is_fallback             BOOLEAN DEFAULT FALSE,  -- 是否为回退定价
    fallback_reason         TEXT,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(model_id, platform)
);

CREATE INDEX idx_pricing_model ON supply_intelligence_pricing_db(model_id);
CREATE INDEX idx_pricing_platform ON supply_intelligence_pricing_db(platform);
```

#### 4.2.5 `supply_intelligence_health_metrics`

```sql
CREATE TABLE supply_intelligence_health_metrics (
    id              BIGSERIAL PRIMARY KEY,
    metric_name     VARCHAR(50) NOT NULL,
    platform        VARCHAR(50),        -- NULL 表示全局
    account_id      BIGINT,
    value           DECIMAL(18,6) NOT NULL,
    labels          JSONB,
    recorded_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_health_metrics_name_time
    ON supply_intelligence_health_metrics(metric_name, recorded_at DESC);
CREATE INDEX idx_health_metrics_platform
    ON supply_intelligence_health_metrics(platform, recorded_at DESC);
```

#### 4.2.6 `supply_intelligence_predictions`

```sql
CREATE TABLE supply_intelligence_predictions (
    id              BIGSERIAL PRIMARY KEY,
    object_type     VARCHAR(20) NOT NULL  -- model / account / platform
                    CHECK (object_type IN ('model','account','platform')),
    object_id       VARCHAR(100) NOT NULL,
    prediction_type VARCHAR(20) NOT NULL  -- deprecation / failure / price_change
                    CHECK (prediction_type IN ('deprecation','failure','price_change')),
    confidence      DECIMAL(3,2) NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    predicted_at    TIMESTAMPTZ NOT NULL,
    reason          TEXT NOT NULL,
    status          VARCHAR(20) DEFAULT 'open'
                    CHECK (status IN ('open','confirmed','dismissed','expired')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_predictions_object ON supply_intelligence_predictions(object_type, object_id);
CREATE INDEX idx_predictions_confidence ON supply_intelligence_predictions(confidence) WHERE status = 'open';
```

### 4.3 实体关系说明

- `supply_accounts`（已有表）：本系统只读（探针、注册写入状态），不修改已有 schema。
- `supply_packages`（已有表）：准入测试通过时生成 `draft` 记录，运营确认后更新为 `active`。
- `model_candidates` → `supply_packages`：通过 `package_draft_id` 外键关联（可空）。
- `probe_execution_logs` → `supply_accounts`：逻辑外键，不建立物理 FK（避免已有表变更耦合）。

### 候选模型数据量估算（供 TechLead 参考）

假设：
- 目标供应商：10 个
- 全网模型扫描周期：每小时 1 次
- 新模型发现率：每个供应商每周平均新增 2 个 model_id
- 测试失败重试：平均每个候选模型测试 2 次才确定最终状态

则：
- 每日新增候选：10 供应商 × 2 模型 × 7 天 = 140 条
- 每月候选记录增量：约 4200 条（其中约 60% 最终变为 test_passed/test_failed，约 40% 处于 discovered/testing 状态）
- 每条记录大小：约 2KB（含 metadata 和状态）
- 30 天保留数据量：约 126KB × 30 天 ≈ 12MB（不含探针日志）

结论：30 天清理策略是合理的，但探针执行日志（每账号每天约 288 条）需单独控制。

建议：`probe_execution_logs` 表独立设置 30 天清理策略；`model_candidates` 表对 test_failed 和 ignored 状态单独设置 90 天保留。

---

## 5. 关键流程设计

### 5.1 发现 → 测试 → 准入 → 上线 → 监控 → 下线 全自动化闭环

```
┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐
│  发现    │──►│  测试    │──►│  准入    │──►│  上线    │──►│  监控    │──►│  下线    │
│ Discovery│   │ Admission│   │ Approve  │   │ Publish  │   │ Monitor  │   │ Deprecate│
└────┬─────┘   └────┬─────┘   └────┬─────┘   └────┬─────┘   └────┬─────┘   └────┬─────┘
     │              │              │              │              │              │
  每小时扫描    标准化测试用例   运营一键确认    Gateway路由    探针周期检测   下架模型告警
  生成candidate  生成draft包    或自动上架(配置)  表更新        状态自动迁移   运营人工确认
```

**全自动模式（配置开启时）**：
- `test_passed` → 无需人工确认，直接 `draft` → `active`（仅限白名单供应商）。
- `deprecated` 告警后 24 小时无运营响应 → 自动 `active` → `paused`（可配置）。

### 5.2 探针执行与状态变更流程

```sequence
Scheduler -> ProbeService: TriggerProbe(accountID)
ProbeService -> ProbeExecutor: Execute(ctx, account)
ProbeExecutor -> SupplierAPI: Health Check Request (timeout=15s)
SupplierAPI --> ProbeExecutor: Response
ProbeExecutor -> ProbeService: ProbeResult
ProbeService -> StateMachine: EvaluateTransition(account, result)
StateMachine -> StateMachine: Check rules & version
StateMachine -> PostgreSQL: UPDATE supply_accounts SET status=?, version=version+1 WHERE version=?
PostgreSQL --> StateMachine: RowsAffected
StateMachine -> AuditEmitter: EmitStateTransitionEvent
StateMachine -> AlertNotifier: SendAlert if status degraded
```

**乐观锁冲突处理（FP-03）**：
- 若 `version` 不匹配，`RowsAffected = 0`。
- 探针记录冲突日志，放弃本次状态变更，由下次探针或运营人员覆盖。
- 不无限重试，避免活锁。

### 5.3 自动注册端到端流程

```sequence
OpsConfig -> AutoRegService: EnableAutoReg(platform=Y, threshold=2)
AutoRegService -> AccountStore: CountActive(platform=Y)
AccountStore --> AutoRegService: count=1 (< threshold)
AutoRegService -> RegistrationTaskStore: CreateTask(register)
Scheduler -> AutoRegService: PollPendingTasks()
AutoRegService -> RegistrationAdapter: Register(ctx, req)
RegistrationAdapter -> SupplierAPI: POST /register
SupplierAPI --> RegistrationAdapter: {user_id: "..."}
RegistrationAdapter -> SMSGateway: RequestVerificationCode(phone)
SMSGateway --> RegistrationAdapter: 503 Service Unavailable
RegistrationAdapter --> AutoRegService: ErrSMSUnavailable
AutoRegService -> RegistrationTaskStore: UpdateStatus(failed, retry_count+1, next_retry_at=NOW()+24h)
AutoRegService -> AuditEmitter: Emit(auto_register_failed)
AutoRegService -> AlertNotifier: NotifyOps(注册失败)
```

---

## 6. 技术选型理由及备选方案

### 6.1 技术栈选型

| 层级 | 选型 | 理由 | 备选方案 | 不选原因 |
|------|------|------|---------|---------|
| 语言 | Go 1.22+ | 与立交桥主项目一致；高并发性能好；静态编译易部署 | Python | 与 gateway/ supply-api 技术栈不一致，增加运维复杂度 |
| HTTP 框架 | 标准库 `net/http` + 自定义中间件 | PRD 明确约束；与 gateway/ supply-api 保持一致 | Gin, Echo | PRD 禁止引入第三方框架 |
| 数据库 | PostgreSQL 15+ | 已有基础设施；JSONB 支持灵活 schema；分区表支持日志清理 | MySQL 8 | 已有 schema 和团队经验在 PG |
| 驱动 | `jackc/pgx/v5` | 性能优；支持批量 Copy；与 supply-api 一致 | `lib/pq` | 维护状态差，功能不足 |
| 缓存/队列 | Redis (`go-redis/v9`) | 已有基础设施；支持 List/Stream 做轻量队列 | Kafka | 引入过重，当前数据量 Redis 足够 |
| 配置 | YAML + Viper | 支持热更新、环境变量覆盖 | etcd/consul | 当前规模无需外部配置中心 |
| 调度器 | 平台统一 Job Scheduler (Temporal/内部 Cron) | PRD 依赖假设 ASP-05；分布式定时任务可靠性高 | 自研 Cron | 不重新造调度器 |
| 浏览器自动化 | Playwright (Go 社区版) | 现代浏览器支持好；供应商前端改版易适配 | Selenium | API 较旧，社区活跃度下降 |
| 时序数据 | Prometheus + Grafana | 已有监控基础设施；与 SFI 指标天然契合 | InfluxDB | 增加额外存储成本 |

### 6.2 设计模式选型

| 模式 | 来源 | 应用场景 | 选型理由 |
|------|------|---------|---------|
| **Strategy（策略）** | 通用 | 探针类型、扫描器、注册适配器、路由策略 | 每供应商行为差异大，策略模式隔离变化 |
| **State（状态）** | 通用 | 账号状态机、candidate 状态机、注册任务状态机 | 状态迁移规则集中管理，避免散落的 if/else |
| **Pipeline（管道）** | 通用 | 准入测试流水线 | 测试阶段可插拔，支持并行与串行组合 |
| **Circuit Breaker（熔断）** | LiteLLM | 供应商 API 调用 | 连续失败时快速失败，保护供应商侧和本系统资源 |
| **Cooldown（冷却）** | LiteLLM | 探针失败后的临时跳过 | 避免对已故障账号的无效重试 |
| **Proxy + Account 关联** | Sub2API | 供应商代理与账号管理 | 网络代理与账号解耦，支持多代理池 |
| **UsageLog + CleanupTask** | Sub2API | 探针日志、审计日志 | 定时清理过期数据，控制存储成本 |

---

## 7. 与立交桥主系统的集成点

### 7.1 与 Bridge Token Gateway 的集成

| 集成方向 | 接口 | 契约 | SLA |
|---------|------|------|-----|
| SI → Gateway | `GET /internal/supply-intelligence/accounts/health` | 返回账号实时状态（active/suspended/disabled），JSON 数组 | P99 < 50ms，可用性 ≥ 99.9% |
| SI → Gateway | `GET /internal/supply-intelligence/packages/active` | 返回平台当前 active 的 supply_packages 列表（含模型元数据） | P99 < 100ms |
| Gateway → SI | WebHook `POST /internal/supply-intelligence/events/routing-failure` | Gateway 路由失败时上报，SI 用于辅助风险评分 | 异步，容忍延迟 < 5s |

**Gateway 路由决策流程（边缘场景 B3）**：
1. Gateway 收到用户请求，需要选择供应商账号。
2. Gateway 查询本地缓存（Redis）中的账号状态，缓存 TTL = 30 秒。
3. 缓存 miss 或过期时，调用 SI 健康查询接口。
4. 若账号为 `suspended` 或 `disabled`，从候选池移除。

### 7.2 与 Channel Manager（supply-api）的集成

| 集成方向 | 接口 | 契约 | 说明 |
|---------|------|------|-----|
| SI → supply-api | `AccountStore.GetByID` / `AccountStore.List` | 读取 `supply_accounts` 记录 | 只读，不修改已有表 |
| SI → supply-api | `PackageStore.CreateDraft` / `PackageStore.UpdateStatus` | 创建/更新 supply_packages | 通过 supply-api 内部接口，不直接写表 |
| SI → supply-api | `AuditStore.Emit` | 审计事件写入 | 复用 supply-api 审计基础设施 |
| SI → supply-api | `VerifyService.Verify` | 新注册账号验证 | 自动注册成功后调用已有验证流程 |
| supply-api → SI | `IntegrationPlugin.Register(mux)` | 集成运行时挂载 Handler | 编译时依赖，运行时开关控制 |

### 7.3 与 NewAPI / Sub2API 的集成

| 集成方向 | 接口 | 契约 |
|---------|------|------|
| SI → NewAPI/Sub2API | `POST /v1/suppliers/status` | 推送供应商健康状态 |
| SI → NewAPI/Sub2API | `POST /v1/models/sync` | 推送新发现的模型列表 |
| NewAPI/Sub2API → SI | `GET /api/v1/supply-intelligence/models` | 查询平台模型库 |
| NewAPI/Sub2API → SI | `GET /api/v1/supply-intelligence/pricing` | 查询定价数据库 |

**适配层设计**：
- `NewAPIAdapter` 和 `Sub2APIAdapter` 实现统一 `ExternalPlatformAdapter` 接口。
- 鉴权：API Key + HMAC-SHA256 签名，密钥通过 KMS 管理。
- 独立部署时，适配器通过配置文件中的 `external_platforms` 数组启用。

---

## 8. 安全设计

### 8.1 账号安全

| 措施 | 实现 |
|------|------|
| 凭证加密 | 所有 API Key 经 KMS 加密后存储，`supply_accounts.credential` 字段为密文。KMS 不可用时，明文不落盘（FP-10）。 |
| 凭证指纹 | `credential_fingerprint` 存储 SHA256 哈希，用于快速比对和审计追踪，不存明文。 |
| 最小权限 | 探针账号、测试账号、注册账号分离，测试账号仅有最低 API 调用权限。 |
| 轮换提醒 | 密钥有效期 < 30 天时， health board 显示黄色警告；< 7 天时红色警告。 |

### 8.2 测试隔离

- 准入测试网络隔离：测试流量走独立出口 IP 池（如有），或至少使用独立账号。
- 测试数据隔离：测试请求添加标识头部，供应商侧可识别。
- 资源限制：单测试任务 CPU 限制 1 core，内存 512MB，超时 60 秒/用例。

### 8.3 数据同步一致性

- **最终一致性**：探针状态变更到 Gateway 感知，最大延迟 = 探针周期(5min) + 缓存 TTL(30s) + 网络延迟 < 6 分钟。
- **审计一致性**：所有状态变更先写审计日志，再写业务表，同一事务内完成。
- **跨服务一致性**：SI 与 supply-api 之间的操作通过内部接口 + 乐观锁保证，无分布式事务（2PC），失败时人工介入。

### 8.4 访问控制

| 资源 | 操作 | 所需权限 |
|------|------|---------|
| 探针配置 | 查看/修改 | `supply:intelligence:config:read/write` |
| 账号状态 | 手动变更 | `supply:ops:account:manage` |
| 模型上架 | 确认/强制上架 | `supply:ops:publish` / `supply:ops:force_publish` |
| 自动注册 | 启用/禁用 | `supply:intelligence:autoreg:admin` |
| 审计日志 | 查询 | `supply:audit:read` |

---

## 9. 性能考量

### 9.1 数据量估算

| 指标 | Phase 1 | Phase 2 | Phase 3 |
|------|---------|---------|---------|
| 供应商数量 | 10 | 20 | 30 |
| 账号总数 | 100 | 300 | 500 |
| 模型候选数/月 | 0 | 50 | 100 |
| 探针日志/天 | 28,800 (100×288) | 86,400 | 144,000 |
| 探针日志/月 | ~86万 | ~260万 | ~430万 |

### 9.2 扫描并发

- **探针并发**：Worker Pool = 50，每账号 15 秒超时，理论最大吞吐量 = 50 × (60/15) × 60 = 12,000 次/小时。
- **实际负载**：500 账号 × 每 5 分钟 = 6,000 次/小时，池大小充足。
- **扫描并发**：每供应商串行扫描，供应商间并行，最大并发 = 供应商数（≤ 30），对平台出口带宽要求低。

### 9.3 测试队列

- **队列实现**：Redis List + 消费者 Worker。
- **最大并行测试数**：10（可配置），避免对供应商测试账号的过度并发。
- **队列深度告警**：`discovered` 状态堆积 > 20 个且持续 24 小时触发 P2 告警（AC-05 关联）。

### 9.4 数据库性能

- **探针日志表**：按 `executed_at` 月分区，查询最近 30 天数据走分区裁剪。
- **索引策略**：所有查询字段均有索引，无全表扫描查询。
- **连接池**：独立运行时池大小 = 20（max_open），集成运行时共享 supply-api 连接池。

### 9.5 缓存策略

| 数据 | 缓存位置 | TTL | 更新触发 |
|------|---------|-----|---------|
| 账号状态 | Redis | 30 秒 | 探针状态变更时失效 |
| 供应商模型列表 | Redis | 30 分钟 | 扫描任务完成时写入 |
| 定价数据库 | 本地内存 + Redis | 1 小时 | 远程拉取成功时更新 |
| 健康大盘 | Redis + 前端缓存 | 5 分钟 | 定时聚合任务写入 |

---

## 10. 风险评估与缓解策略

### 10.1 技术风险

| 风险编号 | 风险描述 | 概率 | 影响 | 缓解措施 |
|---------|---------|------|------|---------|
| R-01 | 探针频率过高导致供应商封禁平台 IP | 中 | 高 | 1. 频率可配置（默认 5 分钟）；2. 使用平台统一出口 IP 池；3. 遵守供应商 Rate Limit；4. 每家供应商独立限流器（令牌桶，rate=10/min） |
| R-02 | 供应商模型列表返回缓存旧数据，导致下架误判 | 中 | 中 | 1. 列表响应加 TTL 校验；2. 结合官方文档 RSS 交叉验证；3. 不自动下架，只生成告警 |
| R-03 | 浏览器自动化因供应商前端改版失效 | 高 | 中 | 1. 优先官方 API；2. Playwright 流程版本化；3. 前端改版监控（DOM 签名校验）；4. 失效时自动降级为人工注册 |
| R-04 | 准入测试用例不足，test_passed 但上线后用户报错 | 中 | 高 | 1. QA 维护并定期评审用例；2. 上线后 24h 内对新模型增加采样监控（Gateway 侧）；3. 运营可一键回退 |
| R-05 | model_candidates 表数据膨胀 | 低 | 中 | 1. `test_failed` 超过 30 天自动清理；2. `ignored` 超过 7 天自动恢复或清理；3. 按 `discovered_at` 分区 |
| R-06 | 本系统故障导致状态误标记 | 低 | 极高 | 1. 灰度三阶段上线；2. 回滚条件：1h 内误报率 > 5% 立即关闭自动变更；3. 生产环境首次只告警不改状态（Phase 2） |
| R-07 | 调度器（Temporal/内部 Cron）不可用 | 低 | 中 | 1. 调度失败时探针/扫描延迟，不引入错误状态；2. 独立运行时内置 fallback 本地 cron（最小功能） |

### 10.2 合规风险

| 风险 | 缓解 |
|------|------|
| 自动注册收集个人信息（邮箱/手机） | 符合平台隐私政策；数据最小化原则；注册完成后脱敏存储；90 天审计日志保留 |
| 审计日志泄露凭证 | 审计日志中的请求/响应摘要经 Sanitizer 脱敏；API Key 只存指纹；完整请求体不写入日志 |
| 跨供应商数据聚合的法律风险 | 定价数据为公开信息；模型列表为公开信息；不涉及用户隐私数据跨境 |

### 10.3 威胁建模

| 威胁场景 | 攻击/故障路径 | 影响 | 控制措施 | 验证要求 |
|---------|---------------|------|---------|---------|
| 凭证明文泄露 | 注册/探针流程在日志、DB、内存 dump 中输出明文凭证 | 供应商账号被接管 | KMS 加密、日志脱敏、指纹比对替代明文、KMS 不可用 fail-closed | 安全测试必须覆盖日志/DB/异常路径无明文 |
| 自动注册滥用 | 注册模块被批量滥用触发垃圾注册或封号 | 供应商封禁、资产损失 | 频控、验证码、审批开关、人工兜底、账号生命周期审计 | 并发重复注册与风控场景必须稳定阻断 |
| 错误状态传播 | Probe/Admission 误判后将错误状态同步给 gateway 或外部系统 | 错误下架/错误上架，影响真实流量 | 三阶段灰度、人工确认、状态机乐观锁、告警不直接改状态 | 首次生产阶段只告警不自动变更状态 |
| 外部适配接口越权 | NewAPI/Sub2API 拉取超出授权的数据或触发敏感操作 | 数据泄露、越权控制 | 最小字段暴露、鉴权、幂等、只读/读写接口分离、审计 | 合同测试覆盖字段边界、鉴权失败、重放请求 |
| 调度器或浏览器自动化失效 | Scheduler/Playwright 失效导致发现/注册链路静默坏掉 | 模型发现停滞、注册失败积压 | 健康告警、fallback 本地 cron、人工接管、失败队列可见 | 必须验证故障时不会静默标记成功 |

### 10.4 设计阶段门控结论

**结论：REQUEST_CHANGES（补齐威胁与阻断门禁后，方可进入开发）**

**放行前必须满足：**
- 探针、发现、准入、注册、运营干预五条主链路都要提供真实实现落点和后续测试阻断项。
- 凭证保护、状态同步、自动注册、外部适配四类高风险点必须在测试设计中有独立安全/异常回归用例。
- 独立运行 / 集成运行 / IntegrationPlugin / OpenAPI / 适配层要求必须进入统一验收矩阵。
- 对首次生产放量场景必须明确“只告警不自动变更”的保护边界和撤销条件。

**阻断条件：**
- 凭证保护不能证明 fail-closed。
- 状态机迁移与审计写入无法形成同事务或等价可追踪闭环。
- 无法证明集成模式中的路由、worker、内部接口全部真实挂载。

---

## 11. 可重用的设计模式

### 11.1 模块内复用

| 模式 | 应用位置 | 说明 |
|------|---------|------|
| **Adapter 模式** | 供应商扫描器、注册适配器、外部平台适配器 | 统一接口隔离供应商差异 |
| **Pipeline 模式** | 准入测试、注册流程 | 阶段可配置、可观测、可回滚 |
| **Worker Pool 模式** | 探针执行、测试执行 | 控制并发、支持背压 |
| **Outbox 模式** | 审计事件发射 | 本地事务写 outbox 表，异步消费保证最终一致性 |
| **Circuit Breaker + Cooldown** | 供应商 API 调用 | 连续失败时进入冷却期，保护双方 |

### 11.2 跨项目复用

| 模式 | 来源 | 本系统应用 | 可被复用到 |
|------|------|-----------|-----------|
| **IntegrationPlugin** | 本系统设计 | 集成运行时挂载到 supply-api | gateway/ 等其他需要插件化集成的模块 |
| **PricingDB + Fallback** | LiteLLM/Sub2API | 模型定价数据库与回退算法 | 任何需要模型成本计算的模块（如 billing-engine） |
| **Risk Score Model** | 本系统设计 | 账号风险评分 | 用户侧风控、支付风控 |
| **State Machine with Optimistic Lock** | supply-api | 账号状态迁移 | 任何需要状态机的业务（结算、订单） |

---

## 12. 技术栈与集成约束

### 12.1 统一技术栈
本项目必须与立交桥主项目保持一致：
- **语言**: Go 1.22+
- **HTTP框架**: 标准库 `net/http` + 自定义中间件（禁止引入 Gin/Echo 等第三方框架，保持与 gateway/ 和 supply-api/ 的一致性）
- **数据库**: PostgreSQL 15+ ，驱动 `jackc/pgx/v5`
- **缓存**: Redis，客户端 `redis/go-redis/v9`
- **配置**: YAML + Viper，环境变量覆盖敏感字段
- **日志/审计**: 结构化日志，审计事件模型与 supply-api/ 一致
- **错误码**: `{SOURCE}_{CATEGORY}_{CODE}` 格式，例如 `SUP_INT_4001`
- **健康检查**: `/actuator/health` 、 `/actuator/health/live` 、 `/actuator/health/ready`
- **测试**: Go testing + testify，覆盖率门槛 domain ≥ 70%、service/handler ≥ 80%

### 12.2 独立运行与集成运行
本系统必须同时支持两种运行模式：

| 模式 | 特征 | 部署方式 | 适用场景 |
|------|------|---------|---------|
| **独立运行** | 自有 `cmd/supply-intelligence/main.go`，独立数据库 schema，独立 docker-compose | `docker-compose up` 或单独容器 | 外部用户只需要供应链管理能力，不想接入立交桥全套 |
| **集成运行** | 作为 Go module 被 `supply-api/` 引入，共享数据库连接池和配置，通过内部接口注册 | 编译时作为子模块编译，运行时挂载到 supply-api 主进程 | 立交桥用户希望获得一体化供应链能力 |

**集成约束**:
- 独立运行时，系统必须提供完整的 HTTP API 和运营工作台。
- 集成运行时，系统必须提供 `IntegrationPlugin` 接口，允许主程序通过配置开关启用/禁用各模块。
- 数据库 schema 必须使用独立的 `supply_intelligence_` 前缀，避免与主项目表名冲突。
- 配置文件必须支持分离加载：独立运行时读取自己的 `config.yaml`，集成运行时合并到主项目配置。

### 12.3 NewAPI / Sub2API 适配支持
本系统的核心能力必须能够对接 NewAPI 和 Sub2API 系统：
- **供应商状态同步**: 提供标准化的供应商健康状态接口，NewAPI/Sub2API 可定期获取供应商可用性状态。
- **模型列表推送**: 提供 `/models` 接口返回平台已发现、已测试通过的模型列表，NewAPI/Sub2API 可消费此数据自动补充自己的模型库。
- **账号注册适配**: 自动注册模块通过适配层支持 NewAPI/Sub2API 的账号管理 API，实现跨平台账号生命周期管理。
- **独立部署时**: 通过配置文件指定 NewAPI/Sub2API 的管理端点地址和鉴权信息，本系统通过适配层（Adapter）与之交互。
- **集成部署时**: 若立交桥 gateway/ 已接入 NewAPI/Sub2API，本系统通过 supply-api/ 的内部接口操作上游状态。

### 12.4 对外接口契约
- 必须提供 OpenAPI 3.0 接口文档，确保 NewAPI/Sub2API 开发者可以独立接入。
- 接口路径前缀默认为 `/api/v1/supply-intelligence/`，集成运行时可通过配置改为 `/internal/supply-intelligence/`。

---

## 13. 变更日志

| 版本 | 日期 | 变更内容 | 作者 |
|------|------|---------|------|
| v1.0 | 2026-04-27 | 初始版本：系统架构、模块设计、数据模型、流程设计、技术选型、集成点、安全、性能、风险 | TechLead |

---

## 附录 A：术语表

| 术语 | 说明 |
|------|------|
| SI | Supply-Intelligence，本系统 |
| SFI | Supply Freshness Index，供应链接新鲜度指数 |
| Candidate | 候选模型（`model_candidates` 记录） |
| Probe | 品质探针，检测供应商账号健康状态 |
| Admission Test | 准入测试，验证新模型是否符合平台标准 |
| Fail-Closed | 依赖条件不满足时显式关闭功能，不静默降级 |
| KMS | Key Management Service，密钥管理服务 |

## 附录 B：参考文档

1. [PRD.md](../prd/PRD.md) — 产品需求文档
2. [competitor-analysis.md](../prd/competitor-analysis.md) — 竞品分析报告
3. [INTERFACE.md](./INTERFACE.md) — 核心接口设计
4. [DEPLOYMENT.md](./DEPLOYMENT.md) — 部署设计
5. [supply-api/CLAUDE.md](../../supply-api/CLAUDE.md) — supply-api 项目规范

---

## 附录 Y：参考文档与外部依赖

| 参考项目 | 版本/日期 | URL | 用途 |
|---------|---------|-----|------|
| LiteLLM | v1.40.0 (2026-03) | https://docs.litellm.ai/ | 模型接口标准化、健康检查设计 |
| Sub2API | main分支 (2026-04) | https://github.com/WeI-Shaw/sub2api | 公告系统、用户体系参考 |
| Intercom | - | https://www.intercom.com/ | 客服体验对标 |
| Prometheus | 3.x (2026-Q1) | https://prometheus.io/ | 时序数据存储 |
| VictoriaMetrics | 1.100.x (2026-Q1) | https://victoriametrics.com/ | 时序数据备选存储 |
| Playwright | 1.50.x (2026-Q1) | https://playwright.dev/ | 浏览器自动化 |
| Qdrant | 1.12.x (2026-Q1) | https://qdrant.tech/ | 向量数据库备选 |
| PGVector | 0.8.x (2026-Q1) | https://github.com/pgvector/pgvector | PostgreSQL向量扩展 |

注：以上版本号为评审时（2026-04-28）的最新稳定版，随着项目开发应定期更新。
