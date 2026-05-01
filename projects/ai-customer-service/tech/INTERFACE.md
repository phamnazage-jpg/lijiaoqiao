# AI-Customer-Service 核心接口设计

> 版本：v1.0 | 状态：初稿

---

## 1. 内部模块间接口

### 1.1 ChannelAdapter

```go
type ChannelAdapter interface {
    ParseWebhook(r *http.Request) (*UnifiedMessage, error)
    SendReply(ctx context.Context, msg *UnifiedMessage, reply string) error
    ValidateWebhook(r *http.Request) error
    ChannelType() string
}

type UnifiedMessage struct {
    MessageID   string
    Channel     string // telegram | discord | wechat | widget
    OpenID      string
    UserID      string
    Content     string
    ContentType string // text | image | file | voice
    Timestamp   time.Time
    ReplyTo     string
}
```

### 1.2 IntentEngine

```go
type IntentEngine interface {
    Recognize(ctx context.Context, sessionID string, message string, context []MessageContext) (*IntentResult, error)
}

type IntentResult struct {
    Intent      string             // 意图类别
    Confidence  float64            // 0.00 - 1.00
    Entities    map[string]string  // 提取的实体
    NeedsHuman  bool               // 是否需要转人工
    Sensitive   bool               // 是否敏感意图
}

type MessageContext struct {
    Direction string
    Content   string
    Timestamp time.Time
}
```

### 1.3 RAGEngine

```go
type RAGEngine interface {
    Retrieve(ctx context.Context, query string, topK int) ([]RetrievalResult, error)
    IndexEntry(ctx context.Context, entry KBEntry) error
    DeleteIndex(ctx context.Context, entryID string) error
}

type RetrievalResult struct {
    EntryID     string
    Title       string
    Content     string
    Score       float64
    Category    string
}
```

### 1.4 DialogManager

```go
type DialogManager interface {
    GetOrCreateSession(ctx context.Context, channel, openID string) (*Session, error)
    UpdateSession(ctx context.Context, sessionID string, updates SessionUpdates) error
    CloseSession(ctx context.Context, sessionID string, reason string) error
    GetContext(ctx context.Context, sessionID string, maxTurns int) ([]MessageContext, error)
    AddMessage(ctx context.Context, sessionID string, msg Message) error
}

type Session struct {
    ID           string
    Channel      string
    OpenID       string
    UserID       string
    Status       string // idle processing waiting_feedback handoff closed
    TurnCount    int
    LastMessageAt time.Time
}

type SessionUpdates struct {
    Status        *string
    UserID        *string
    TurnCount     *int
    LastMessageAt *time.Time
}
```

### 1.5 DiagnosisService

```go
type DiagnosisService interface {
    VerifyIdentity(ctx context.Context, email string, code string) (*IdentityResult, error)
    QueryQuota(ctx context.Context, userID string) (*QuotaInfo, error)
    QueryTokenUsage(ctx context.Context, userID string, window time.Duration) (*TokenUsage, error)
    QueryErrorLogs(ctx context.Context, userID string, limit int) ([]ErrorLog, error)
}

type IdentityResult struct {
    Matched   bool
    UserID    string
    Attempts  int
    Locked    bool
}

type QuotaInfo struct {
    TotalQuota     int64
    UsedQuota      int64
    RemainingQuota int64
    ResetAt        time.Time
}
```

### 1.6 HandoffService

```go
type HandoffService interface {
    ShouldHandoff(ctx context.Context, intent *IntentResult, turnCount int, identityFailures int) (*HandoffDecision, error)
    CreateTicket(ctx context.Context, sessionID string, reason string, priority string) (*Ticket, error)
    AssignTicket(ctx context.Context, ticketID string, agentID string) error
    CloseTicket(ctx context.Context, ticketID string, resolution string) error
}

type HandoffDecision struct {
    ShouldHandoff bool
    Reason        string
    Priority      string // P1 P2 P3
}

type Ticket struct {
    ID              string
    SessionID       string
    UserID          string
    Priority        string
    Status          string
    HandoffReason   string
    AssignedTo      string
    ContextSnapshot string
    CreatedAt       time.Time
}
```

### 1.7 KnowledgeBaseService

```go
type KnowledgeBaseService interface {
    CreateEntry(ctx context.Context, entry KBEntry) (*KBEntry, error)
    UpdateEntry(ctx context.Context, entry KBEntry) (*KBEntry, error)
    DeleteEntry(ctx context.Context, entryID string) error
    GetEntry(ctx context.Context, entryID string) (*KBEntry, error)
    ListEntries(ctx context.Context, filter KBFilter) ([]KBEntry, error)
    PublishEntry(ctx context.Context, entryID string) error
}

type KBEntry struct {
    ID             string
    Title          string
    Content        string
    Category       string
    Tags           []string
    ReferenceCount int
    Status         string // draft published deprecated
    Version        int
}
```

### 1.8 LLMClient

```go
type LLMClient interface {
    Generate(ctx context.Context, prompt string, options LLMOptions) (*LLMResponse, error)
    GenerateWithRAG(ctx context.Context, prompt string, context []RetrievalResult, options LLMOptions) (*LLMResponse, error)
    GetEmbedding(ctx context.Context, text string) ([]float32, error)
}

type LLMResponse struct {
    Content      string
    Provider     string
    Model        string
    LatencyMs    int
    TokenUsage   TokenUsageInfo
}

type LLMOptions struct {
    MaxTokens   int
    Temperature float64
    Timeout     time.Duration
}
```

---

## 2. 外部系统集成接口

### 2.1 与 Bridge Gateway 集成

| 方法 | 路径 | 请求 | 响应 | 说明 |
|------|------|------|------|------|
| Webhook 接收 | `POST /api/v1/customer-service/webhook/{channel}` | `UnifiedMessage` | `{"received":true}` | 接收渠道消息 |
| 消息回复 | `POST {gateway_callback_url}` | `{"session_id":"","content":""}` | `{"sent":true}` | 调用 Gateway 发送接口 |
| 状态查询 | `GET /actuator/health` | - | `{"status":"up"}` | Gateway 健康检查 |

### 2.2 与 platform-token-runtime 集成

| 方法 | 路径 | 请求 | 响应 | 说明 |
|------|------|------|------|------|
| 配额查询 | `GET /internal/runtime/quota` | `?user_id={uid}` | `QuotaInfo` | 延迟 < 500ms |
| Token 消耗 | `GET /internal/runtime/token-usage` | `?user_id={uid}&window=1d` | `TokenUsage` | 延迟 < 500ms |
| 错误日志 | `GET /internal/runtime/error-logs` | `?user_id={uid}&limit=5` | `[]ErrorLog` | 延迟 < 3s |

### 2.3 与 supply-api 集成

| 方法 | 路径 | 请求 | 响应 | 说明 |
|------|------|------|------|------|
| 用户身份校验 | `GET /internal/supply/users/verify` | `?email={email}` 或 `?api_key_prefix={prefix}` | `{"matched":true,"user_id":""}` | 延迟 < 2s |
| 审计日志格式 | `GET /internal/supply/audit/schema` | - | `{"schema":{}}` | 格式一致 |

### 2.4 与 NewAPI / Sub2API 集成

| 方法 | 路径 | 请求 | 响应 | 说明 |
|------|------|------|------|------|
| Webhook 接入 | `POST /api/v1/customer-service/webhook/{channel}` | `渠道原生消息格式` | `{"received":true}` | 适配层转换为 UnifiedMessage |
| 工单查询 | `GET /api/v1/customer-service/tickets` | `?status=open&external_system=newapi` | `[]Ticket` | 外部系统获取工单 |
| 知识库查询 | `GET /api/v1/customer-service/kb` | `?query={q}&limit=5` | `[]KBEntry` | 知识库共享 |

---

## 3. API 接口规范

### 3.1 REST API 基础

- **基础路径** (独立运行): `/api/v1/customer-service/`
- **基础路径** (集成运行): `/internal/customer-service/`
- **内容类型**: `application/json`
- **错误响应格式**:

```json
{
  "error": {
    "code": "CS_SES_4001",
    "message": "会话不存在",
    "details": {}
  }
}
```

### 3.2 核心端点

#### 会话管理

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/v1/customer-service/webhook/{channel}` | 接收渠道 Webhook |
| GET | `/api/v1/customer-service/sessions/{id}` | 获取会话信息 |
| GET | `/api/v1/customer-service/sessions/{id}/messages` | 获取会话消息 |
| POST | `/api/v1/customer-service/sessions/{id}/feedback` | 提交解决/未解决反馈 |
| POST | `/api/v1/customer-service/sessions/{id}/handoff` | 人工触发转人工 |

#### 工单管理

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/customer-service/tickets` | 列表工单 |
| GET | `/api/v1/customer-service/tickets/{id}` | 获取工单 |
| POST | `/api/v1/customer-service/tickets/{id}/assign` | 分配工单 |
| POST | `/api/v1/customer-service/tickets/{id}/resolve` | 解决工单 |
| POST | `/api/v1/customer-service/tickets/{id}/close` | 关闭工单 |
| GET | `/api/v1/customer-service/tickets/stats` | 工单统计 |

#### 知识库

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/customer-service/kb` | 列表知识库条目 |
| POST | `/api/v1/customer-service/kb` | 创建条目 |
| GET | `/api/v1/customer-service/kb/{id}` | 获取条目 |
| PUT | `/api/v1/customer-service/kb/{id}` | 更新条目 |
| DELETE | `/api/v1/customer-service/kb/{id}` | 删除条目 |
| POST | `/api/v1/customer-service/kb/{id}/publish` | 发布条目 |
| POST | `/api/v1/customer-service/kb/search` | 检索知识库 |

#### 运营后台

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/customer-service/admin/dashboard` | 运营大盘 |
| GET | `/api/v1/customer-service/admin/handoff-reasons` | 转人工原因统计 |
| POST | `/api/v1/customer-service/admin/feedback-review` | 提交对话质检结果 |

### 3.3 错误码定义

| 错误码 | HTTP 状态 | 说明 |
|---------|-----------|------|
| `CS_SES_4001` | 404 | 会话不存在 |
| `CS_SES_4002` | 429 | 消息频率过高 |
| `CS_SES_4003` | 403 | 身份校验已锁定 |
| `CS_IDT_4001` | 400 | 身份信息不匹配 |
| `CS_IDT_4002` | 400 | 验证码错误 |
| `CS_TKT_4001` | 404 | 工单不存在 |
| `CS_TKT_4002` | 409 | 工单已被分配 |
| `CS_KB_4001` | 404 | 知识库条目不存在 |
| `CS_KB_4002` | 409 | 条目名称已存在 |
| `CS_LLM_5001` | 503 | LLM 服务不可用 |
| `CS_LLM_5002` | 504 | LLM 超时 |
| `CS_AUTH_4001` | 403 | 越权访问 |

### 3.4 WebSocket 接口

**路径**: `/ws/v1/customer-service/sessions/{session_id}`

- 网页 Widget 客户端订阅，实时推送机器人回复。
- 心跳间隔 30 秒。
