# AI-Customer-Service 智能客服系统 — 高层设计文档 (HLD)

> 版本：v1.0
> 负责人：TechLead
> 目标读者：后端开发、QA、SRE
> 状态：初稿

---

## 1. 设计目标与约束

### 1.1 核心目标

| 指标 | 基准值 | 目标值 | 验证方式 |
|------|--------|--------|---------|
| 人工客服介入率 | 100% | ≤ 40% | 转人工工单数 / 总会话数 |
| 首次响应时间 | 人工排班时段 | ≤ 10 秒 | 用户消息到达至首次回复的 P99 |
| 常见问题一次解决率 | 0 | ≥ 75% | 用户标记已解决 / (总会话 - 明确转人工) |
| 用户满意度 CSAT | 无 | ≥ 4.0 / 5.0 | 每周抽样调查 |
| 系统可用性 | 无 | ≥ 99.5% | 健康检查通过率 7 天滑动窗口 |

### 1.2 技术约束（强制性）

- **语言**: Go 1.22+
- **HTTP 框架**: 标准库 `net/http` + 自定义中间件（禁止引入 Gin/Echo）
- **数据库**: PostgreSQL 15+ ，驱动 `jackc/pgx/v5`
- **缓存**: Redis，客户端 `redis/go-redis/v9`
- **配置**: YAML + Viper，环境变量覆盖敏感字段
- **日志/审计**: 结构化日志，审计事件模型与 supply-api/ 一致
- **错误码**: `{SOURCE}_{CATEGORY}_{CODE}` 格式，例如 `CS_SES_4001`
- **健康检查**: `/actuator/health` 、 `/actuator/health/live` 、 `/actuator/health/ready`
- **测试**: Go testing + testify，覆盖率门槛 domain ≥ 70%、service/handler ≥ 80%

### 1.3 运行模式

本系统必须同时支持两种运行模式：

| 模式 | 特征 | 部署方式 | 适用场景 |
|------|------|---------|---------|
| **独立运行** | 自有 `cmd/ai-customer-service/main.go`，独立数据库 schema，独立 docker-compose | `docker-compose up` 或单独容器 | 外部用户只需要客服能力 |
| **集成运行** | 作为 Go module 被 `gateway/` 引入，共享数据库连接池和配置 | 编译时作为子模块编译，运行时挂载到 gateway 主进程 | 立交桥用户希望获得一体化客服能力 |

**集成约束**:
- 独立运行时，系统必须提供完整的 HTTP API 、Webhook 接入和运营后台。
- 集成运行时，系统必须提供 `IntegrationPlugin` 接口，允许主程序通过配置开关启用/禁用各模块。
- 数据库 schema 必须使用独立的 `cs_` 前缀，避免与主项目表名冲突。
- 配置文件必须支持分离加载：独立运行时读取自己的 `config.yaml`，集成运行时合并到主项目配置。

---

## 2. 系统架构总览

### 2.1 逻辑架构图

```
+---------------------+     +---------------------+     +---------------------+
|   渠道层 (Gateway)   |     |   运营后台 (Web)    |     |   外部系统         |
|  - Telegram Bot     |     |  - 工单看板          |     |  - LLM 供应商 A    |
|  - Discord Bot      |     |  - 会话历史         |     |  - LLM 供应商 B    |
|  - 微信公众号       |     |  - 知识库管理       |     |  - 向量数据库        |
|  - 网页 Widget      |     |  - 转人工统计      |     |  - 新闻云/火山引擎  |
+----------+----------+     +----------+----------+     +----------+----------+
           |                           |                           |
           v                           v                           v
+-----------------------------------------------------------------------------+
|                         AI-Customer-Service Core Layer                      |
|  +----------------+  +----------------+  +----------------+  +-----------+  |
|  | Channel Adapter|  | Intent Engine  |  | RAG Engine     |  | Dialog    |  |
|  | (渠道适配器)    |  | (意图识别)    |  | (知识库检索)   |  | Manager   |  |
|  +----------------+  +----------------+  +----------------+  +-----------+  |
|  +----------------+  +----------------+  +----------------+  +-----------+  |
|  | Diagnosis Svc  |  | Handoff Svc    |  | Ticket Svc     |  | Knowledge |  |
|  | (诊断查询)     |  | (转人工)      |  | (工单管理)     |  | Base Svc  |  |
|  +----------------+  +----------------+  +----------------+  +-----------+  |
|  +----------------+  +----------------+  +----------------+  +-----------+  |
|  | LLM Client     |  | Auth/Identity  |  | Audit Svc      |  | Monitor   |  |
|  | (模型调用)     |  | (身份校验)    |  | (审计日志)     |  | Svc       |  |
|  +----------------+  +----------------+  +----------------+  +-----------+  |
+-----------------------------------------------------------------------------+
           |                           |                           |
           v                           v                           v
+---------------------+     +---------------------+     +---------------------+
|   PostgreSQL (cs_*) |     |   Redis             |     |   外部只读 API      |
|   - cs_sessions     |     |   - 会话上下文     |     |   - supply-api/     |
|   - cs_tickets      |     |   - 知识库缓存     |     |   - token-runtime/  |
|   - cs_kb_entries   |     |   - 频率限制       |     |   - NewAPI/Sub2API  |
|   - cs_audit_logs   |     |   - 工单锁       |     |                     |
+---------------------+     +---------------------+     +---------------------+
```

### 2.2 组件划分与职责

| 组件 | 职责 | 独立/集成兼容 |
|------|------|-------------|
| **Channel Adapter** | 封装各渠道的 Webhook 接口差异，将外部消息转换为内部统一消息格式 | 两种模式均支持，集成时通过 gateway/ 路由接入 |
| **Intent Engine** | 基于 LLM 的意图识别，输出意图类别、置信度、实体提取 | 两种模式均支持 |
| **RAG Engine** | 知识库向量检索 + 重排序，输出相关文档片段 | 两种模式均支持 |
| **Dialog Manager** | 会话状态管理、上下文维护（最近 5 轮）、转人工判断 | 两种模式均支持 |
| **Diagnosis Service** | 调用 supply-api / token-runtime 只读接口，查询用户配额、Token 消耗、错误日志 | 两种模式均支持，集成时通过内部接口调用 |
| **Handoff Service** | 转人工判断逻辑：置信度低、用户要求、敏感意图、身份失败 | 两种模式均支持 |
| **Ticket Service** | 工单创建、分配、状态迁移、关闭、会话上下文附加 | 两种模式均支持 |
| **Knowledge Base Service** | 知识库条目增删改查、索引管理、引用统计 | 两种模式均支持 |
| **LLM Client** | 多供应商 LLM 调用、failover、超时处理、流量控制 | 两种模式均支持 |
| **Auth/Identity Service** | 渠道用户身份校验、立交桥账户关联、API Key 前缀匹配 | 两种模式均支持 |
| **Audit Service** | 审计事件捕获、存储、查询 | 两种模式均支持 |
| **Monitor Service** | 埋点事件收集、指标汇总、暴露 Prometheus /metrics | 两种模式均支持 |

---

## 3. 核心模块设计

### 3.1 渠道适配器 (Channel Adapter)

#### 3.1.1 设计目标
封装 Telegram、Discord、微信、网页 Widget 的消息格式差异，对内部提供统一的 `UnifiedMessage` 结构。

#### 3.1.2 核心结构

```go
type UnifiedMessage struct {
    MessageID   string    // 渠道原生消息 ID
    Channel     string    // telegram | discord | wechat | widget
    OpenID      string    // 渠道用户唯一标识
    UserID      string    // 立交桥账户 ID（已绑定时）
    Content     string    // 消息内容（已过滤）
    ContentType string    // text | image | file | voice
    Timestamp   time.Time
    ReplyTo     string    // 回复的消息 ID
}

type ChannelAdapter interface {
    ParseWebhook(r *http.Request) (*UnifiedMessage, error)
    SendReply(ctx context.Context, msg *UnifiedMessage, reply string) error
    ValidateWebhook(r *http.Request) error  // 验证 Webhook 签名
    ChannelType() string
}
```

#### 3.1.3 渠道特定处理

| 渠道 | 接入方式 | 特殊处理 |
|------|---------|---------|
| Telegram | Webhook / 长连接 | 支持 Markdown 格式，消息长度限制 4096 字符 |
| Discord | Webhook / Bot API | 支持 Embed 格式，速率限制 5 次/秒 |
| 微信 | 客服消息 Webhook | 需要签名验证，回复时间窗口 48 小时 |
| Widget | WebSocket / SSE | 支持实时打字效果，跨域配置 CORS |

#### 3.1.4 消息过滤与安全
- 图片、文件、语音类消息直接返回 "暂不支持该类型消息"，不解析、不存储。
- 内容长度 > 2000 字符时，截断至 2000 字符并提示。

### 3.2 对话引擎 (Dialog Engine)

#### 3.2.1 会话状态机

```
├── idle (空闲)─────────────────────────┐
│         │                                          │
│    新消息   │                                    超时30分钟
│         ↓                                          ↓
├── processing (处理中)──────────────────┘
│         │
│    处理完成  │
│         ↓
├── waiting_feedback (等待用户反馈)───────────┐
│         │                                          │
│    解决/未解决   │                                    超时30分钟
│         │                                          ↓
│         ↓                                    closed (关闭)
├── handoff (已转人工)────────────────────────┘
│         │
│    工单关闭   → closed
┘
```

#### 3.2.2 上下文管理
- 每个会话保留最近 5 轮对话（用户 5 条 + 机器人 5 条 = 10 条）。
- 超出部分从 Redis List 中自动清理，不再参与 LLM 上下文。
- 会话超时 30 分钟无消息则自动关闭。

#### 3.2.3 处理流程

```
1. 接收 UnifiedMessage
2. 身份校验：已绑定→提取 UserID；未绑定→请求邮箱/前缀校验
3. 意图识别：LLM 输出 [意图, 置信度, 实体]
4. 判断：
   a. 敏感意图（退款/封禁/安全）→ 直接转人工（P1 工单）
   b. 用户明确要求人工 → 转人工
   c. 置信度 < 0.60 → 转人工
   d. 其他 → 知识库检索 + LLM 生成回复
5. 回复用户，等待反馈
6. 用户反馈 "已解决" → 会话关闭
7. 用户反馈 "未解决" → 计算轮次，超过 3 轮 → 转人工
```

### 3.3 意图识别 (Intent Engine)

#### 3.3.1 意图分类

| 意图类别 | 示例 | 置信度阈值 | 处理方式 |
|---------|------|-----------|---------|
| api_key_management | "怎么生成 API Key" | ≥ 0.85 | 知识库 + 操作指引 |
| quota_query | "我的配额还剩多少" | ≥ 0.85 | 知识库 + 诊断查询 |
| model_routing | "怎么配置模型路由" | ≥ 0.85 | 知识库 + 代码示例 |
| error_debug | "返回 429 是什么意思" | ≥ 0.85 | 知识库 + 错误码释义 |
| billing | "怎么开发票" | ≥ 0.85 | 知识库 + 流程链接 |
| sensitive_refund | "我要申请退款" | ≥ 0.70 | **强制转人工** |
| sensitive_ban | "我的账户被封了" | ≥ 0.70 | **强制转人工** |
| sensitive_security | "我的数据泄露了" | ≥ 0.70 | **强制转人工** |
| handoff_request | "找人工、投诉" | ≥ 0.90 | **强制转人工** |
| unknown | 无法分类 | < 0.60 | 转人工 |

#### 3.3.2 LLM 调用提示词策略

```
系统 Prompt 结构：
1. 角色："你是立交桥平台的智能客服助手，仅回答与立交桥相关的问题。"
2. 范围限制："不要回答与立交桥无关的问题。不要提供内部系统架构、密钥、服务器地址等敏感信息。"
3. 数据隔离："仅使用当前用户的数据进行查询。如果用户未提供身份信息，不能查询任何个人数据。"
4. 输出格式：JSON，含 intent、confidence、entities、needs_human、sensitive 字段
```

#### 3.3.3 Failover 策略
- 主模型超时 5 秒 → 切换备用模型供应商。
- 备用模型也超时 5 秒 → 返回兑底回复 + 自动生成工单。
- 兑底回复不依赖大模型，为静态模板："当前咨询量较大，请稍后或提交工单由人工处理。"

### 3.4 RAG 知识库引擎

#### 3.4.1 索引管理
- 知识库条目使用 Markdown 格式，分块后通过嵌入模型生成向量。
- 向量存储于向量数据库（Milvus / Qdrant / PGVector），检索延迟 P99 < 200ms。
- 新条目发布后 30 秒内生效（异步重新索引）。

#### 3.4.2 检索流程
```
1. 用户问题 → 嵌入模型生成查询向量
2. 向量数据库 Top-K 检索（K=5）
3. 重排序：基于相关性 + 条目引用次数 + 最近更新时间
4. 取 Top-3 作为上下文片段
5. 拼接到 LLM Prompt 中生成回复
```

#### 3.4.3 知识库缺失处理
- 检索无结果且意图置信度 < 0.60 → 直接转人工。
- 记录 "知识库未命中" 事件，每日汇总给运营团队。

### 3.5 诊断服务 (Diagnosis Service)

#### 3.5.1 只读查询范围

| 查询类型 | 调用方 | 超时 | 失败处理 |
|---------|--------|------|---------|
| 用户身份校验 | supply-api/ 内部接口 | 2s | 请求邮箱二次校验 |
| 配额查询 | token-runtime/ 内部接口 | 2s | 回复通用说明，提示稍后重试 |
| Token 消耗 | token-runtime/ 内部接口 | 2s | 同上 |
| 最近错误日志 | supply-api/ 内部接口 | 3s | 回复通用排查步骤 |

#### 3.5.2 安全限制
- 所有查询必须携带当前会话的 user_id，系统不允许跨用户查询。
- API Key 前缀匹配时，若匹配到多个账户，请求邮箱二次校验；仍无法确定则转人工。
- 错误的 API Key 或密码不记录，仅记录失败次数与事件类型。

### 3.6 转人工机制 (Handoff Service)

#### 3.6.1 转人工触发条件（任意满足即触发）

| 条件 | 工单优先级 | 备注 |
|------|-----------|------|
| 意图置信度 < 0.60 | P2 | 标记原因：意图不明 |
| 用户发送“人工客服”等关键词 | P2 | 标记原因：用户要求 |
| 敏感意图（退款/封禁/安全） | P1 | 标记原因：敏感问题 |
| 身份校验失败累计 3 次 | P2 | 标记原因：身份失败 |
| 多轮对话未解决（> 3 轮） | P2 | 标记原因：未解决 |
| 主备模型均故障 | P1 | 标记原因：模型故障 |

#### 3.6.2 工单分配逻辑
- 未处理工单按优先级（P1 > P2 > P3）与时间升序排列。
- 客服点击“接收”后，工单状态在 1 秒内变更为 “处理中”并锁定为该客服。
- 排队超过 15 分钟向用户发送排队进度通知。

### 3.7 知识库管理 (Knowledge Base Service)

#### 3.7.1 条目结构

```go
type KBEntry struct {
    ID            string    // UUID
    Title         string    // 标题
    Content       string    // Markdown 内容
    Category      string    // api_key | quota | billing | routing | error_code | onboarding | other
    Tags          []string  // 标签
    ReferenceCount int      // 被引用次数
    LastQueriedAt time.Time // 最近被查询时间
    Status        string    // draft | published | deprecated
    CreatedBy     string
    CreatedAt     time.Time
    UpdatedAt     time.Time
    Version       int       // 乐观锁
}
```

#### 3.7.2 更新机制
- 运营后台增删改查条目，点击“发布”后 30 秒内生效。
- 产品文档变更时，知识库更新为发布 checklist 项。
- 每周生成知识库未命中报告，驱动文档补充。

### 3.8 运营后台

#### 3.8.1 核心视图

| 视图 | 内容 | 权限 |
|------|------|------|
| 工单看板 | 未处理工单按优先级与时间排列，支持分配、关闭、标记 | cs:agent |
| 会话历史 | 用户与机器人的完整对话，支持搜索与筛选 | cs:agent, cs:admin |
| 知识库管理 | 条目增删改查、发布、引用统计 | cs:admin |
| 转人工统计 | 每日 Top 10 转人工原因饼图 | cs:admin |
| 模型回复质检 | 每日抽样 5% 对话，运营人员可标记错误答案 | cs:admin |

### 3.8.X 运营后台数据模型扩展

#### cs_agent_sessions — 客服人员会话绑定
| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | UUID | PK | |
| `agent_id` | VARCHAR(64) | NOT NULL | 客服人员ID |
| `ticket_id` | UUID | NOT NULL, FK | 关联工单 |
| `joined_at` | TIMESTAMPTZ | NOT NULL | 加入时间 |
| `left_at` | TIMESTAMPTZ | NULL | 离开时间 |

#### cs_agent_stats — 客服统计（每日聚合）
| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | BIGSERIAL | PK | |
| `agent_id` | VARCHAR(64) | NOT NULL | |
| `date` | DATE | NOT NULL | |
| `tickets_handled` | INT | DEFAULT 0 | 处理工单数 |
| `avg_handle_time_sec` | INT | DEFAULT 0 | 平均处理时长 |
| `handoff_count` | INT | DEFAULT 0 | 被转接次数 |
| `csat_score` | DECIMAL(3,2) | NULL | 用户满意度 |

### 3.8.Y 运营后台核心API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/ai-customer-service/dashboard/stats` | 获取今日统计（会话量/转人工率/解决率/CSAT） |
| GET | `/api/v1/ai-customer-service/dashboard/handoff-reasons` | 获取转人工原因分布 Top10 |
| GET | `/api/v1/ai-customer-service/dashboard/kb-miss-rate` | 获取知识库未命中率趋势 |

---

## 4. 数据模型设计

### 4.1 核心实体关系图 (ER)

```
+----------------+       +----------------+       +----------------+
| cs_sessions    |<----->| cs_messages    |<----->| cs_tickets     |
+----------------+       +----------------+       +----------------+
        |                                               |
        |                                               |
        v                                               v
+----------------+       +----------------+       +----------------+
| cs_kb_entries  |       | cs_audit_logs  |       | cs_channel_bindings |
+----------------+       +----------------+       +----------------+
```

### 4.2 数据表结构

#### 4.2.1 `cs_sessions` — 会话

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | UUID | PK, 默认 gen_random_uuid() | 会话唯一标识 |
| `channel` | VARCHAR(16) | NOT NULL, CHECK IN ('telegram','discord','wechat','widget') | 渠道 |
| `open_id` | VARCHAR(128) | NOT NULL | 渠道用户标识 |
| `user_id` | VARCHAR(64) | NULL | 立交桥账户 ID（已绑定时） |
| `status` | VARCHAR(16) | NOT NULL, DEFAULT 'idle', CHECK IN ('idle','processing','waiting_feedback','handoff','closed') | 会话状态 |
| `turn_count` | INT | NOT NULL, DEFAULT 0 | 已进行轮次 |
| `last_message_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | 最后消息时间 |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | 创建时间 |
| `updated_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | 更新时间 |

**索引**: `CREATE INDEX idx_sessions_channel_openid ON cs_sessions(channel, open_id) WHERE status != 'closed';`

#### 4.2.2 `cs_messages` — 消息

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | UUID | PK | 消息 ID |
| `session_id` | UUID | NOT NULL, FK -> cs_sessions | 所属会话 |
| `direction` | VARCHAR(8) | NOT NULL, CHECK IN ('in','out') | in=用户发送, out=机器人回复 |
| `content` | TEXT | NOT NULL | 消息内容 |
| `content_type` | VARCHAR(16) | NOT NULL, DEFAULT 'text' | text | image | file | voice |
| `intent` | VARCHAR(32) | NULL | 意图类别（仅 in 方向） |
| `confidence` | DECIMAL(3,2) | NULL | 置信度（0.00-1.00） |
| `model_provider` | VARCHAR(32) | NULL | 使用的 LLM 供应商 |
| `latency_ms` | INT | NULL | 生成回复耗时（仅 out 方向） |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | 创建时间 |

**索引**: `CREATE INDEX idx_messages_session_id ON cs_messages(session_id, created_at DESC);`

#### 4.2.3 `cs_tickets` — 工单

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | UUID | PK | 工单 ID |
| `session_id` | UUID | NOT NULL, FK -> cs_sessions | 来源会话 |
| `user_id` | VARCHAR(64) | NULL | 用户 ID |
| `priority` | VARCHAR(4) | NOT NULL, CHECK IN ('P0','P1','P2','P3') | 优先级 |
| `status` | VARCHAR(16) | NOT NULL, DEFAULT 'open', CHECK IN ('open','assigned','processing','resolved','closed') | 状态 |
| `handoff_reason` | VARCHAR(32) | NOT NULL | 转人工原因 |
| `assigned_to` | VARCHAR(64) | NULL | 分配给的客服人员 ID |
| `context_snapshot` | JSONB | NOT NULL | 会话上下文快照 |
| `resolution` | TEXT | NULL | 处理结果 |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | 创建时间 |
| `resolved_at` | TIMESTAMPTZ | NULL | 解决时间 |
| `updated_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | 更新时间 |

**索引**: `CREATE INDEX idx_tickets_status_priority ON cs_tickets(status, priority, created_at);`

#### 4.2.4 `cs_kb_entries` — 知识库条目

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | UUID | PK | 条目 ID |
| `title` | VARCHAR(256) | NOT NULL | 标题 |
| `content` | TEXT | NOT NULL | Markdown 内容 |
| `category` | VARCHAR(32) | NOT NULL | 分类 |
| `tags` | VARCHAR(32)[] | DEFAULT '{}' | 标签数组 |
| `reference_count` | INT | NOT NULL, DEFAULT 0 | 被引用次数 |
| `last_queried_at` | TIMESTAMPTZ | NULL | 最近被查询时间 |
| `status` | VARCHAR(16) | NOT NULL, DEFAULT 'draft', CHECK IN ('draft','published','deprecated') | 状态 |
| `created_by` | VARCHAR(64) | NOT NULL | 创建人 |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | 创建时间 |
| `updated_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | 更新时间 |
| `version` | INT | NOT NULL, DEFAULT 1 | 乐观锁 |

**索引**: `CREATE INDEX idx_kb_status ON cs_kb_entries(status);`

#### 4.2.5 `cs_channel_bindings` — 渠道绑定

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | UUID | PK | 绑定 ID |
| `channel` | VARCHAR(16) | NOT NULL | 渠道 |
| `open_id` | VARCHAR(128) | NOT NULL | 渠道用户标识 |
| `user_id` | VARCHAR(64) | NOT NULL | 立交桥账户 ID |
| `bound_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | 绑定时间 |
| `bound_method` | VARCHAR(16) | NOT NULL | oauth | api_key_prefix | email_verify |

**约束**: `UNIQUE(channel, open_id)`

#### 4.2.6 `cs_audit_logs` — 审计日志

与 supply-api/ 审计规范一致，对象类型包括 `cs_session`、`cs_ticket`、`cs_kb_entry`。

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | UUID | PK | 事件 ID |
| `tenant_id` | VARCHAR(64) | NOT NULL | 工作区 ID |
| `object_type` | VARCHAR(32) | NOT NULL | 对象类型 |
| `object_id` | VARCHAR(64) | NOT NULL | 对象 ID |
| `action` | VARCHAR(16) | NOT NULL | create | update | delete | handoff | resolve |
| `before_state` | JSONB | NULL | 变更前 |
| `after_state` | JSONB | NULL | 变更后 |
| `actor_id` | VARCHAR(64) | NOT NULL | 操作人 ID |
| `source_ip` | VARCHAR(45) | NULL | 来源 IP |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | 创建时间 |

### 4.3 Redis 缓存设计

| Key 模式 | 用途 | TTL |
|----------|------|-----|
| `cs:session:{session_id}` | 会话状态与上下文 | 30 min |
| `cs:rate_limit:{channel}:{open_id}` | 消息频率限制计数 | 1 min |
| `cs:identity_fail:{session_id}` | 身份校验失败次数 | 10 min |
| `cs:kb:vector:{entry_id}` | 知识库条目向量（若使用 Redis 作为向量存储） | 无 |
| `cs:ticket_lock:{ticket_id}` | 工单分配锁 | 5 min |

---

## 5. 关键流程设计

### 5.1 用户问题自助解决流程

```
用户发送消息
    ↓
Channel Adapter 解析为 UnifiedMessage
    ↓
Auth/Identity Service 身份校验
    ↓
Dialog Manager 检查会话状态，更新上下文
    ↓
Intent Engine 识别意图 + 置信度
    ↓
是否敏感/人工/低置信度？
    │──是 → Handoff Service 生成工单 → 通知用户排队/等待
    ↓否
RAG Engine 检索知识库
    ↓
需要用户数据？
    │──是 → Diagnosis Service 查询只读 API
    ↓否
LLM Client 生成回复
    ↓
Channel Adapter 发送回复
    ↓
等待用户反馈（30 min 超时关闭）
```

### 5.2 转人工流程

```
触发条件满足
    ↓
Dialog Manager 更新会话状态 → handoff
    ↓
Ticket Service 创建工单（含会话上下文快照）
    ↓
Audit Service 记录 handoff 事件
    ↓
通知渠道：用户收到排队/等待提示
    ↓
客服后台：工单入队列
    ↓
客服接收 → 状态变更为 processing
    ↓
客服解决 → 状态变更为 resolved → 关闭会话
```

### 5.3 大模型故障 Failover 流程

```
LLM Client 调用主模型
    ↓
超时 5 秒
    ↓
切换至备用模型
    ↓
超时 5 秒
    ↓
返回兑底回复 + 自动生成工单
    ↓
Monitor Service 记录 failover 事件并触发告警
```

---

## 6. 技术选型理由及备选方案

| 技术点 | 选型 | 理由 | 备选方案 |
|--------|------|------|---------|
| HTTP 框架 | 标准库 net/http | 与 gateway/ 、supply-api/ 一致，避免框架依赖 | 无 |
| 数据库 | PostgreSQL 15+ | 与主项目一致，支持 JSONB 和向量扩展 | 无 |
| 向量数据库 | PGVector | 无需额外部署，与 PostgreSQL 共存，支持中文语义检索 | Milvus (高性能、分布式) / Qdrant (轻量、Cloud-native) |
| LLM 供应商 | 主：OpenAI GPT-4o；备：阿里云通义千问 | 中英文理解能力强，API 稳定，备用保障国内访问 | Claude / 火山引擎 |
| 嵌入模型 | OpenAI text-embedding-3-small | 成本低、效果好，与 LLM 供应商一致 | 中文嵌入模型（如 BGE） |
| 缓存 | Redis | 与主项目一致，支持会话、频率限制 | 无 |
| 消息队列 | 内部 Go channel + worker pool | 足够支撑当前并发，避免额外依赖 | Kafka (未来高并发) |
| 向量索引更新 | 异步 worker | 知识库变更不频繁，异步更新足够 | 无 |

---

## 7. 与立交桥主系统的集成点

### 7.1 Gateway 集成

| 集成点 | 接口形式 | 说明 |
|--------|---------|------|
| 消息接入 | Webhook POST /api/v1/customer-service/webhook/{channel} | Gateway 将渠道消息转发至客服系统 |
| 消息回复 | HTTP POST 回调 | 客服系统调用 Gateway 消息发送接口 |
| 状态查询 | GET /actuator/health | Gateway 健康检查，不健康时跳过客服路由 |

### 7.2 platform-token-runtime 集成

| 集成点 | 接口形式 | 说明 |
|--------|---------|------|
| 配额查询 | 内部 gRPC / HTTP 只读接口 | 延迟 < 500ms，带 user_id 校验 |
| Token 消耗查询 | 内部 gRPC / HTTP 只读接口 | 延迟 < 500ms |
| 错误日志查询 | 内部 gRPC / HTTP 只读接口 | 返回最近 5 条 |

### 7.3 supply-api 集成

| 集成点 | 接口形式 | 说明 |
|--------|---------|------|
| 用户身份校验 | 内部 gRPC / HTTP 只读接口 | API Key 前缀匹配、邮箱验证 |
| 审计日志格式 | 约定 | 与 supply-api/ 审计规范一致 |

### 7.4 NewAPI / Sub2API 集成

| 集成点 | 接口形式 | 说明 |
|--------|---------|------|
| Webhook 接入 | 标准化 POST 接口 | NewAPI/Sub2API 可配置将用户消息转发至本系统 |
| 工单推送 | REST API 或 Webhook 回调 | NewAPI/Sub2API 可定期获取待处理工单状态 |
| 知识库共享 | REST API 查询 | NewAPI/Sub2API 可消费知识库数据 |
| 适配层 | Adapter 接口 | 独立部署时通过配置指定对方 Webhook 地址和鉴权信息 |

---

## 8. 安全设计

### 8.1 数据保护
- 客服系统 **仅拥有只读查询权限**。任何写操作（修改配额、重置密码、删除用户）必须通过工单由人工授权后执行。
- 用户数据查询必须携带当前会话的 user_id，系统不允许跨用户查询。
- API Key 前缀匹配时不存储完整 API Key。
- 错误的身份信息不记录，仅记录失败次数。

### 8.2 审计日志
- 所有会话创建、转人工、工单状态变更、知识库变更均需记录审计事件。
- 审计事件与 supply-api/ 保持一致的结构和存储方式。
- 保留期 ≥ 90 天。

### 8.3 越权防护
- 运营后台基于 RBAC，角色：`cs:agent`（客服）、`cs:admin`（运营管理）。
- 客服系统接口调用 supply-api / token-runtime 时使用内部服务账户，不使用用户凭证。
- 内部服务账户仅拥有只读权限。

### 8.4 Prompt Injection 防护
- 系统 Prompt 中明确禁止回复非当前用户数据、禁止提供内部系统架构或密钥。
- 定期红队测试（每月一次），检验 Prompt Injection 防护效果。
- 敏感操作意图（退款/封禁/安全）强制转人工，不走 LLM 生成回复流程。

---

## 9. 性能考量

### 9.1 并发估算

| 场景 | 峰值 QPS | 平均 QPS | 说明 |
|------|-----------|-----------|------|
| 消息接入 | 100 | 20 | 各渠道汇总，含小流量高峰 |
| 知识库检索 | 100 | 20 | 每次用户消息触发 1 次 |
| LLM 调用 | 100 | 20 | 主模型 + 备用模型合并 |
| 只读 API 查询 | 100 | 20 | 并行于 LLM 调用 |
| 运营后台 | 10 | 2 | 内部使用，低并发 |

### 9.2 延迟目标

| 链路 | 目标延迟 |
|------|---------|
| 消息接收到首次回复 | P99 ≤ 10 秒 |
| 意图识别 | P99 ≤ 2 秒 |
| 知识库检索 | P99 ≤ 200 ms |
| 只读 API 查询 | P99 ≤ 3 秒 |
| 工单创建 | P99 ≤ 1 秒 |
| 运营后台页面加载 | P99 ≤ 2 秒 |

### 9.3 存储估算

| 数据 | 每日增量 | 90 天总量 | 说明 |
|------|---------|------------|------|
| 消息 | 50 万条 | 4500 万条 | 平均每条 200 字符 |
| 会话 | 5 万个 | 450 万个 | 含已关闭会话 |
| 工单 | 5000 个 | 45 万个 | 转人工率 10% |
| 审计日志 | 10 万条 | 900 万条 | 含所有事件 |
| 知识库条目 | 稳定 500 条 | 500 条 | 增长缓慢 |
| 向量数据 | ~200 MB | 200 MB | 500 条 × 1536 维 × 4 字节 |

---

## 10. 风险评估与缓解策略

| 风险编号 | 风险描述 | 概率 | 影响 | 缓解策略 |
|---------|---------|------|------|---------|
| R-1 | LLM 幻觉导致错误指导用户配置 | 中 | 高 | 1. 回答范围限制在知识库内容；2. 涉及操作必须附带官方文档链接；3. 每日抽样 5% 对话质检；4. 高风险意图强制转人工 |
| R-2 | 用户通过 Prompt Injection 泄露敏感数据 | 中 | 高 | 1. 系统 Prompt 明确禁止；2. user_id 强制校验；3. 全量安全审计日志；4. 定期红队测试 |
| R-3 | 模型供应商涨价或停服 | 低 | 中 | 1. 至少 2 家供应商；2. 30 秒内切换能力；3. 兑底回复不依赖大模型 |
| R-4 | 知识库维护跟不上产品迭代 | 高 | 中 | 1. 发布 checklist 强制同步；2. 每周未命中报告；3. 预留半日/周运营人力 |
| R-5 | Gateway Webhook 接入改造超出预期 | 中 | 中 | 1. Phase 1 先验证网页 Widget 独立接入；2. 明确不改造 Gateway 核心路由 |
| R-6 | 数据库连接池耗尽 | 低 | 高 | 1. 连接池监控与预警；2. 降级模式：仅返回静态 FAQ 链接；3. 容器自动重启 |

### 10.1 威胁建模

| 威胁场景 | 攻击路径 | 影响 | 控制措施 | 验证要求 |
|---------|---------|------|---------|---------|
| Prompt Injection 绕过安全边界 | 用户输入恶意提示词诱导模型泄露内部信息或跨会话数据 | 敏感信息泄露、错误操作建议 | System Prompt 禁止输出内部信息；敏感意图强制转人工；会话级 user_id 强绑定；响应输出增加敏感词审计 | 红队注入样例每月回归；高风险样例必须稳定拒绝 |
| 渠道伪造 Webhook | 外部伪造渠道回调向系统注入假消息/假工单 | 工单污染、审计失真 | 渠道签名校验、时间戳窗口校验、幂等键、防重放 nonce | 每个渠道提供签名失败/重放攻击测试用例 |
| 运营后台越权查询 | 客服/运营绕过 RBAC 查看非授权会话和工单 | 用户隐私泄露 | RBAC + 资源级过滤；后端强制按 user_id / workspace 过滤；审计查询行为 | QA 必测跨用户/跨角色访问 403 |
| Adapter 调用外部只读 API 失控 | 诊断查询未限流导致压垮 supply-api / token-runtime | 上游链路抖动、级联故障 | 限流、超时、熔断、降级静态 FAQ/排障链接 | 压测和故障注入时验证 fail-open/fail-closed 策略 |
| 审计日志篡改或缺失 | 工单/转人工/知识库变更未留痕或被覆盖 | 无法追责、无法回放 | 审计事件单独写入；不可变追加；失败重试队列；90 天保留 | 审计写入失败必须告警且阻断高风险操作 |

### 10.2 设计阶段门控结论

**结论：REQUEST_CHANGES（补齐实现与验证门禁后，方可进入开发）**

**放行前必须满足：**
- HLD 中所有关键能力都能映射到真实实现落点：渠道接入、意图识别、RAG、转人工、工单、审计、监控。
- TechLead 任务拆解必须继续细化到文件/函数级，确保 Engineer 不会在实现阶段自行改架构。
- QA 必须基于本 HLD 补充调用链检查点：定义 → 装配 → 调用 → 入口。
- 运行模式、OpenAPI、IntegrationPlugin、NewAPI/Sub2API 适配要求均需在后续实现验证中列为阻断项。

**阻断条件：**
- 任一高风险链路（Webhook 鉴权、越权访问、审计留痕、降级策略）未提供可执行验证方案。
- 任一关键能力只有接口声明没有真实挂载入口。
- 无法证明独立运行与集成运行两种模式都可交付。

---

## 11. 技术栈与集成约束

### 11.1 统一技术栈
本项目必须与立交桥主项目保持一致：
- **语言**: Go 1.22+
- **HTTP框架**: 标准库 `net/http` + 自定义中间件（禁止引入 Gin/Echo 等第三方框架，保持与 gateway/ 和 supply-api/ 的一致性）
- **数据库**: PostgreSQL 15+ ，驱动 `jackc/pgx/v5`
- **缓存**: Redis，客户端 `redis/go-redis/v9`
- **配置**: YAML + Viper，环境变量覆盖敏感字段
- **日志/审计**: 结构化日志，审计事件模型与 supply-api/ 一致
- **错误码**: `{SOURCE}_{CATEGORY}_{CODE}` 格式，例如 `CS_SES_4001`
- **健康检查**: `/actuator/health` 、 `/actuator/health/live` 、 `/actuator/health/ready`
- **测试**: Go testing + testify，覆盖率门槛 domain ≥ 70%、service/handler ≥ 80%

### 11.2 独立运行与集成运行
本系统必须同时支持两种运行模式：

| 模式 | 特征 | 部署方式 | 适用场景 |
|------|------|---------|---------|
| **独立运行** | 自有 `cmd/ai-customer-service/main.go`，独立数据库 schema，独立 docker-compose | `docker-compose up` 或单独容器 | 外部用户只需要客服能力，不想接入立交桥全套 |
| **集成运行** | 作为 Go module 被 `gateway/` 引入，共享数据库连接池和配置，通过内部接口注册 | 编译时作为子模块编译，运行时挂载到 gateway 主进程 | 立交桥用户希望获得一体化客服能力 |

**集成约束**:
- 独立运行时，系统必须提供完整的 HTTP API、Webhook 接入和运营后台。
- 集成运行时，系统必须提供 `IntegrationPlugin` 接口，允许主程序通过配置开关启用/禁用各模块。
- 数据库 schema 必须使用独立的 `cs_` 前缀，避免与主项目表名冲突。
- 配置文件必须支持分离加载：独立运行时读取自己的 `config.yaml`，集成运行时合并到主项目配置。

### 11.3 NewAPI / Sub2API 适配支持
本系统的核心能力必须能够对接 NewAPI 和 Sub2API 系统：
- **Webhook 接入**: 提供标准化的 Webhook 接口，NewAPI/Sub2API 可配置将用户消息转发至本系统。
- **工单推送**: 提供标准化工单接口，NewAPI/Sub2API 可定期获取待处理工单状态。
- **知识库共享**: 提供知识库查询接口，NewAPI/Sub2API 可消费此数据补充自己的帮助文档。
- **独立部署时**: 通过配置文件指定 NewAPI/Sub2API 的 Webhook 地址和鉴权信息，本系统通过适配层（Adapter）与之交互。
- **集成部署时**: 若立交桥 gateway/ 已接入 NewAPI/Sub2API，本系统通过 gateway/ 的内部路由接口接入客服能力。

### 11.4 对外接口契约
- 必须提供 OpenAPI 3.0 接口文档，确保 NewAPI/Sub2API 开发者可以独立接入。
- 接口路径前缀默认为 `/api/v1/customer-service/`，集成运行时可通过配置改为 `/internal/customer-service/`。

---

## 12. 可重用的设计模式

| 设计模式 | 来源 | 应用场景 |
|---------|------|---------|
| **Channel Adapter** | 竞品（Intercom） | 封装渠道差异，支持新渠道插件化扩展 |
| **RAG Pipeline** | 行业实践 | 知识库检索增强生成，与具体业务解耦 |
| **Failover Chain** | LiteLLM | 多 LLM 供应商自动切换 |
| **Dialog State Machine** | 行业实践 | 会话状态管理，支持异步事件驱动 |
| **Integration Plugin** | 本项目设计 | 独立/集成双模式支持，通过接口隔离主项目 |

---

## 13. 变更日志

| 版本 | 日期 | 修改人 | 内容 |
|------|------|--------|------|
| v1.0 | 2026-04-27 | TechLead | 初稿：系统架构、核心模块、数据模型、流程设计、技术选型、集成点、安全、性能、风险 |

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
