# NewAPI / Sub2API Adapter Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 `ai-customer-service` 增加面向 `Sub2API` 优先、`NewAPI` 同构兼容的最小平台适配层，支持入站原生消息适配、异步全事件流回写，以及准可靠投递。

**Architecture:** 在现有统一 webhook 主链之外新增平台入口 `/platforms/{platform}/webhook`，通过内置 adapter 将平台原生 payload 转换为 `UnifiedMessage`。主链处理后生成内部平台事件，先落库到 outbox，再由后台 worker 进行带重试的异步 callback 投递。

**Tech Stack:** Go 1.22, net/http, PostgreSQL, HMAC-SHA256, background worker, Go test, httptest

---

## 0. 实施原则

1. **先 Sub2API，后 NewAPI**  
   第一批只要求 Sub2API 真正可跑，NewAPI 只保留 profile 插槽和最小合同测试骨架。

2. **先入站，后出站，最后可靠性**  
   先打通平台入站 -> 主链，再接 outbox + callback，再补 dead letter / replay。

3. **适配逻辑边缘化**  
   不改 `dialog.Service` 的核心业务语义；平台差异收在 adapter / callback / outbox 层。

4. **TDD + 频繁提交**  
   每个 Task 都先写失败测试，再写最小实现，再跑验证，再提交。

---

### Task 1: 搭好平台适配骨架与路由入口

**Files:**
- Create: `internal/platformadapter/types.go`
- Create: `internal/platformadapter/registry.go`
- Create: `internal/platformadapter/sub2api_adapter.go`
- Create: `internal/platformadapter/newapi_adapter.go`
- Create: `internal/http/handlers/platform_webhook_handler.go`
- Modify: `internal/http/router.go`
- Test: `internal/platformadapter/registry_test.go`
- Test: `internal/http/handlers/platform_webhook_handler_test.go`

**Step 1: 写平台注册表失败测试**

写测试覆盖：

```go
func TestRegistry_ShouldResolveSub2APIAdapter(t *testing.T) {}
func TestRegistry_ShouldRejectUnknownPlatform(t *testing.T) {}
```

**Step 2: 运行测试确认失败**

Run:

```bash
go test ./internal/platformadapter ./internal/http/handlers -count=1
```

Expected:
- FAIL，提示 `platformadapter` 包或 handler 不存在

**Step 3: 写最小平台类型与注册表**

新增：

- `PlatformAdapter` 接口
- `IngressContext`
- `PlatformInboundMeta`
- `Registry`

最小接口：

```go
type PlatformAdapter interface {
    Platform() string
    ParseInbound(r *http.Request, body []byte, ctx IngressContext) (*message.UnifiedMessage, *PlatformInboundMeta, error)
    BuildIngressAck(result *dialog.Result, meta *PlatformInboundMeta) any
}
```

**Step 4: 写最小 handler 骨架**

`PlatformWebhookHandler` 先只做：

1. 路径读取 `{platform}` / `{channel}`
2. 从 registry 取 adapter
3. 读取 body
4. 调 adapter
5. 调现有 `dialog.Service`
6. 返回 adapter ack

**Step 5: 在 router 增加入口**

新增：

- `POST /api/v1/customer-service/platforms/{platform}/webhook`
- `POST /api/v1/customer-service/platforms/{platform}/webhook/{channel}`

**Step 6: 跑测试确认通过**

Run:

```bash
go test ./internal/platformadapter ./internal/http/handlers -count=1
```

Expected:
- PASS

**Step 7: Commit**

```bash
git add internal/platformadapter internal/http/handlers/platform_webhook_handler.go internal/http/handlers/platform_webhook_handler_test.go internal/http/router.go
git commit -m "feat(adapter): add platform webhook adapter skeleton"
```

---

### Task 2: 实现 Sub2API 入站最小适配

**Files:**
- Modify: `internal/platformadapter/sub2api_adapter.go`
- Create: `internal/platformadapter/sub2api_types.go`
- Test: `internal/platformadapter/sub2api_adapter_test.go`
- Modify: `internal/http/handlers/platform_webhook_handler_test.go`
- Reference: `docs/SUB2API_MINIMAL_WEBHOOK_MAPPING.md`

**Step 1: 写 Sub2API payload 失败测试**

覆盖：

```go
func TestSub2APIAdapter_ShouldMapMinimalPayload(t *testing.T) {}
func TestSub2APIAdapter_ShouldRejectUnknownEnvelopeFields(t *testing.T) {}
func TestSub2APIAdapter_ShouldUseChannelOverrideWhenPresent(t *testing.T) {}
func TestSub2APIAdapter_ShouldRequireOpenIDAndContent(t *testing.T) {}
```

**Step 2: 运行测试确认失败**

Run:

```bash
go test ./internal/platformadapter -count=1
```

Expected:
- FAIL，字段映射或校验未实现

**Step 3: 定义 Sub2API 最小 payload 结构**

只实现第一版所需字段：

```go
type Sub2APIInboundPayload struct {
    MessageID string    `json:"message_id"`
    Channel   string    `json:"channel"`
    OpenID    string    `json:"open_id"`
    UserID    string    `json:"user_id,omitempty"`
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp,omitempty"`
    ReplyTo   string    `json:"reply_to,omitempty"`
}
```

不要一次性吞平台原生大包。

**Step 4: 实现最小 ParseInbound**

规则：

1. 只接受当前最小字段
2. 缺 `channel/open_id/content` 返回 `400`
3. `{channel}` path override 优先
4. 产出 `UnifiedMessage`
5. 记录 `PlatformInboundMeta`

**Step 5: 实现最小 ingress ack**

同步响应先返回：

```json
{
  "accepted": true,
  "platform": "sub2api",
  "session_id": "...",
  "ticket_id": "...",
  "event_id": "..."
}
```

**Step 6: 跑测试确认通过**

Run:

```bash
go test ./internal/platformadapter ./internal/http/handlers -count=1
```

Expected:
- PASS

**Step 7: Commit**

```bash
git add internal/platformadapter/sub2api_adapter.go internal/platformadapter/sub2api_types.go internal/platformadapter/sub2api_adapter_test.go internal/http/handlers/platform_webhook_handler_test.go
git commit -m "feat(adapter): add sub2api inbound adapter"
```

---

### Task 3: 增加平台级入站鉴权配置

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Create: `internal/http/handlers/platform_webhook_security.go`
- Test: `internal/http/handlers/platform_webhook_security_test.go`
- Modify: `internal/http/router.go`
- Modify: `docs/CONFIG_CONTRACT_BASELINE.md`

**Step 1: 先写配置失败测试**

覆盖：

```go
func TestPlatformAdapterConfig_ShouldFailInProdWhenSub2APIEnabledWithoutIngressSecret(t *testing.T) {}
func TestPlatformAdapterConfig_ShouldPassWhenAdaptersDisabled(t *testing.T) {}
```

**Step 2: 跑测试确认失败**

Run:

```bash
go test ./internal/config ./internal/http/handlers -count=1
```

Expected:
- FAIL

**Step 3: 增加最小平台适配配置**

新增配置项：

- `AI_CS_PLATFORM_ADAPTERS_ENABLED`
- `AI_CS_PLATFORM_SUB2API_ENABLED`
- `AI_CS_PLATFORM_SUB2API_INGRESS_SECRET`
- `AI_CS_PLATFORM_SUB2API_CALLBACK_BASE_URL`
- `AI_CS_PLATFORM_SUB2API_CALLBACK_SECRET`
- `AI_CS_PLATFORM_SUB2API_CALLBACK_TIMEOUT_MS`
- `AI_CS_PLATFORM_SUB2API_CALLBACK_MAX_RETRIES`
- `AI_CS_PLATFORM_NEWAPI_ENABLED`

**Step 4: 写平台入口安全包装器**

实现与现有 `WebhookSecurity` 同构的：

- `PlatformWebhookSecurity`

但按 platform profile 选择 secret，不要复用通用 webhook secret。

**Step 5: 在 router 给平台入口接安全包装**

平台入口独立挂安全中间件，不与现有 `/webhook` 混用 secret。

**Step 6: 跑测试确认通过**

Run:

```bash
go test ./internal/config ./internal/http/handlers -count=1
```

Expected:
- PASS

**Step 7: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go internal/http/handlers/platform_webhook_security.go internal/http/handlers/platform_webhook_security_test.go internal/http/router.go docs/CONFIG_CONTRACT_BASELINE.md
git commit -m "feat(adapter): add platform-specific ingress security config"
```

---

### Task 4: 定义平台事件模型与 outbox 表结构

**Files:**
- Create: `db/migration/0002_platform_event_outbox.up.sql`
- Create: `internal/domain/platformevent/event.go`
- Create: `internal/domain/platformevent/event_test.go`
- Create: `internal/store/postgres/platform_event_store.go`
- Create: `internal/store/postgres/platform_event_store_test.go`
- Reference: `docs/plans/2026-05-06-newapi-sub2api-adapter-design.md`

**Step 1: 写 store 失败测试**

覆盖：

```go
func TestPlatformEventStore_ShouldInsertPendingEvent(t *testing.T) {}
func TestPlatformEventStore_ShouldListPendingEventsInOrder(t *testing.T) {}
```

**Step 2: 跑测试确认失败**

Run:

```bash
go test ./internal/store/postgres -count=1
```

Expected:
- FAIL

**Step 3: 定义事件模型**

新增 `platformevent.Event`：

- `ID`
- `Platform`
- `EventType`
- `SessionID`
- `TicketID`
- `SourceMessageID`
- `CallbackTarget`
- `Payload`
- `Status`
- `AttemptCount`
- `NextAttemptAt`
- `CreatedAt`

**Step 4: 补 migration**

建表至少包括：

1. `cs_platform_callbacks`
2. `cs_platform_event_outbox`
3. `cs_platform_event_delivery_attempts`
4. `cs_platform_event_dead_letters`

第一版不做过度 schema 拆分，优先让 outbox 可用。

**Step 5: 实现最小 Postgres store**

支持：

1. 插入 pending event
2. 拉取 due events
3. 标记 delivered
4. 标记 retry
5. 标记 dead letter

**Step 6: 跑测试确认通过**

Run:

```bash
go test ./internal/domain/platformevent ./internal/store/postgres -count=1
```

Expected:
- PASS

**Step 7: Commit**

```bash
git add db/migration/0002_platform_event_outbox.up.sql internal/domain/platformevent internal/store/postgres/platform_event_store.go internal/store/postgres/platform_event_store_test.go
git commit -m "feat(adapter): add platform event outbox schema and store"
```

---

### Task 5: 在主链接入平台事件生成

**Files:**
- Modify: `internal/service/dialog/service.go`
- Create: `internal/service/platformevents/builder.go`
- Create: `internal/service/platformevents/builder_test.go`
- Modify: `internal/http/handlers/platform_webhook_handler.go`
- Modify: `internal/http/handlers/platform_webhook_handler_test.go`

**Step 1: 写失败测试**

覆盖：

```go
func TestPlatformWebhookHandler_ShouldEnqueueMessageReceivedAndReplyGenerated(t *testing.T) {}
func TestPlatformWebhookHandler_ShouldEnqueueHandoffAndTicketCreatedWhenNeeded(t *testing.T) {}
```

**Step 2: 跑测试确认失败**

Run:

```bash
go test ./internal/service/... ./internal/http/handlers -count=1
```

Expected:
- FAIL

**Step 3: 新增事件构建器**

从 `dialog.Result + PlatformInboundMeta` 构建：

1. `message.received`
2. `message.processing`
3. `intent.resolved`
4. `handoff.triggered`
5. `ticket.created`
6. `reply.generated`

**Step 4: 在平台 handler 中落 outbox**

当前平台入口成功后：

1. 先调主链
2. 再构建事件
3. 批量写入 outbox
4. 返回 ingress ack

第一版不要把 outbox 失败静默吞掉；应返回 `500` 并记录日志/审计。

**Step 5: 跑测试确认通过**

Run:

```bash
go test ./internal/service/... ./internal/http/handlers -count=1
```

Expected:
- PASS

**Step 6: Commit**

```bash
git add internal/service/platformevents internal/service/dialog/service.go internal/http/handlers/platform_webhook_handler.go internal/http/handlers/platform_webhook_handler_test.go
git commit -m "feat(adapter): enqueue platform outbox events from inbound flow"
```

---

### Task 6: 实现 callback 投递 worker

**Files:**
- Create: `internal/service/platformdelivery/worker.go`
- Create: `internal/service/platformdelivery/signer.go`
- Create: `internal/service/platformdelivery/worker_test.go`
- Create: `internal/service/platformdelivery/signer_test.go`
- Modify: `internal/app/app.go`
- Modify: `internal/config/config.go`

**Step 1: 写失败测试**

覆盖：

```go
func TestWorker_ShouldDeliverPendingEventToCallbackServer(t *testing.T) {}
func TestWorker_ShouldRetryWhenCallbackReturns5xx(t *testing.T) {}
func TestSigner_ShouldProduceStableTimestampAndSignatureHeaders(t *testing.T) {}
```

**Step 2: 跑测试确认失败**

Run:

```bash
go test ./internal/service/platformdelivery -count=1
```

Expected:
- FAIL

**Step 3: 实现 callback signer**

为出站事件添加：

- `X-CS-Timestamp`
- `X-CS-Signature`

算法与平台 callback secret 对齐。

**Step 4: 实现最小 worker**

职责：

1. 拉取 due events
2. 发送 callback
3. 成功标记 delivered
4. 失败按退避设置 `next_attempt_at`

**Step 5: 在 app 启动 worker**

只在：

- `AI_CS_PLATFORM_ADAPTERS_ENABLED=true`

时启动。

**Step 6: 跑测试确认通过**

Run:

```bash
go test ./internal/service/platformdelivery ./internal/app -count=1
```

Expected:
- PASS

**Step 7: Commit**

```bash
git add internal/service/platformdelivery internal/app/app.go internal/config/config.go
git commit -m "feat(adapter): add platform callback delivery worker"
```

---

### Task 7: 增加重试、死信和投递尝试审计

**Files:**
- Modify: `internal/store/postgres/platform_event_store.go`
- Modify: `internal/store/postgres/platform_event_store_test.go`
- Modify: `internal/service/platformdelivery/worker.go`
- Modify: `internal/service/platformdelivery/worker_test.go`
- Create: `docs/RUNBOOK_PLATFORM_CALLBACKS.md`

**Step 1: 写失败测试**

覆盖：

```go
func TestWorker_ShouldMoveEventToDeadLetterAfterMaxRetries(t *testing.T) {}
func TestWorker_ShouldPersistDeliveryAttemptAudit(t *testing.T) {}
```

**Step 2: 跑测试确认失败**

Run:

```bash
go test ./internal/store/postgres ./internal/service/platformdelivery -count=1
```

Expected:
- FAIL

**Step 3: 实现尝试记录与死信**

要求：

1. 每次 callback 尝试都写 `delivery_attempts`
2. 达到最大次数写 `dead_letters`
3. outbox 主记录进入 terminal status

**Step 4: 补运行手册**

新增 runbook 说明：

1. 如何查看 pending / failed / dead letter
2. 如何手动重放
3. 如何区分平台回调失败与主链失败

**Step 5: 跑测试确认通过**

Run:

```bash
go test ./internal/store/postgres ./internal/service/platformdelivery -count=1
```

Expected:
- PASS

**Step 6: Commit**

```bash
git add internal/store/postgres/platform_event_store.go internal/store/postgres/platform_event_store_test.go internal/service/platformdelivery/worker.go internal/service/platformdelivery/worker_test.go docs/RUNBOOK_PLATFORM_CALLBACKS.md
git commit -m "feat(adapter): add callback retry audit and dead letter handling"
```

---

### Task 8: 新增端到端 Sub2API 接入测试

**Files:**
- Create: `test/integration/sub2api_webhook_flow_test.go`
- Create: `test/e2e/sub2api_callback_flow_test.go`
- Modify: `tech/TEST_DESIGN.md`
- Modify: `test/QA_GATE_STATUS.md`

**Step 1: 写端到端失败测试**

覆盖：

```go
func TestSub2APIWebhookFlow_ShouldCreateSessionTicketAndOutboxEvents(t *testing.T) {}
func TestSub2APICallbackFlow_ShouldDeliverOrderedEventsWithStableEventIDs(t *testing.T) {}
func TestSub2APICallbackFlow_ShouldDeadLetterAfterMaxRetries(t *testing.T) {}
```

**Step 2: 跑测试确认失败**

Run:

```bash
go test ./test/integration ./test/e2e -count=1
```

Expected:
- FAIL

**Step 3: 接通测试依赖**

1. 使用 mock callback server
2. 使用 Postgres 测试库
3. 走真实平台入口 `/platforms/sub2api/webhook`
4. 验证 outbox / delivery / dead letter

**Step 4: 更新测试设计与 QA 文档**

把原来“NewAPI/Sub2API 适配层验证待实现”改成：

1. 已有 Sub2API 最小接入联调测试
2. NewAPI 同构位待实现

**Step 5: 跑测试确认通过**

Run:

```bash
go test ./test/integration ./test/e2e -count=1
go test ./... -count=1
```

Expected:
- PASS

**Step 6: Commit**

```bash
git add test/integration/sub2api_webhook_flow_test.go test/e2e/sub2api_callback_flow_test.go tech/TEST_DESIGN.md test/QA_GATE_STATUS.md
git commit -m "test(adapter): add sub2api end-to-end adapter coverage"
```

---

### Task 9: 预留 NewAPI profile 与适配扩展点

**Files:**
- Modify: `internal/platformadapter/newapi_adapter.go`
- Create: `internal/platformadapter/newapi_adapter_test.go`
- Modify: `docs/plans/2026-05-06-newapi-sub2api-adapter-design.md`

**Step 1: 写最小失败测试**

覆盖：

```go
func TestNewAPIAdapter_ShouldBeRegisteredButDisabledByDefault(t *testing.T) {}
```

**Step 2: 跑测试确认失败**

Run:

```bash
go test ./internal/platformadapter -count=1
```

Expected:
- FAIL

**Step 3: 实现同构占位**

要求：

1. registry 中可注册 `newapi`
2. 默认不开启
3. 明确返回“profile not implemented”而不是 silent success

**Step 4: 跑测试确认通过**

Run:

```bash
go test ./internal/platformadapter -count=1
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add internal/platformadapter/newapi_adapter.go internal/platformadapter/newapi_adapter_test.go docs/plans/2026-05-06-newapi-sub2api-adapter-design.md
git commit -m "feat(adapter): reserve newapi adapter profile extension point"
```

---

## 最终整体验证

所有 Task 完成后必须执行：

```bash
go test ./... -count=1
go test -race ./...
go vet ./...
bash -n scripts/verify_preprod_gate_b.sh
bash -n scripts/verify_gate_c_rollback.sh
```

如果新增了平台脚本，再追加：

```bash
bash scripts/verify_platform_adapter_sub2api.sh
```

Expected:
- 全部 PASS

---

## 交付完成判定

满足以下条件才算第一版完成：

1. `sub2api` 平台入口可用
2. 原生 payload 可映射到 `UnifiedMessage`
3. 成功创建 session / ticket / audit / dedup
4. 全事件流可进入 outbox
5. callback worker 可投递、重试、死信
6. 端到端测试通过
7. QA 文档与 runbook 已更新

---

## 风险提醒

1. **不要一次性做完整平台协议**
   第一版只做 Sub2API 优先的最小 profile。

2. **不要把平台字段渗透进核心主链**
   平台差异只能留在 adapter/meta/event 边缘层。

3. **不要跳过 outbox 直接同步回调**
   你已经要求准可靠投递，不能退回 best-effort。

4. **不要省掉 dead letter**
   没有 dead letter，就没有真正的可恢复性闭环。
