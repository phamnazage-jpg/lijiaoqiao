# NewAPI / Sub2API 适配增强设计

> 日期：2026-05-06  
> 状态：设计稿  
> 适用项目：`projects/ai-customer-service`  
> 设计边界：**最小接入层、内置适配器、入站 + 异步全事件流回写、Sub2API 优先、准可靠投递**

---

## 1. 目标与边界

本设计解决的问题不是“把 `ai-customer-service` 做成另一个 NewAPI/Sub2API”，而是让它能够**稳定挂接在 NewAPI/Sub2API 后面，作为客服能力子系统运行**。当前代码已经具备 webhook、会话、意图、转人工、工单、审计、去重、PostgreSQL 落库、Gate B/Gate C 脚本化验证等底座，缺的是把外部平台原生消息接进来、再把内部处理结果以平台可消费的事件流回推出去的适配层。

第一版范围严格限制为：

1. **Sub2API 优先**，NewAPI 保持同构兼容位，不追求双平台一次做满。
2. **内置适配器**，不新增外部 shim 作为主路径。
3. **入站适配**：把平台原生消息转换为 `UnifiedMessage` 并进入现有主链。
4. **出站回写**：把内部处理结果、工单、错误、回调状态转成异步事件回推给上游平台。
5. **准可靠投递**：事件持久化、重试、死信/补偿到位，但不追求复杂的跨系统 exactly-once。

明确不做的内容：

1. 完整平台级管理后台
2. 知识库共享 API 的全量产品化
3. NewAPI/Sub2API 全量管理协议一比一兼容
4. 任意平台原生结构透传

结论是：**第一版目标是“可稳定接入和可观测回推”，不是“完整兼容替代”。**

---

## 2. 总体架构

推荐架构是在现有 HTTP 入口和对话主链之间插入一个**平台适配层（Platform Adapter Layer）**，并在主链处理完成后插入一个**事件出站层（Event Outbox + Delivery Layer）**。这样可以保持当前客服核心逻辑不被平台协议污染，同时把平台差异收口在边缘。

逻辑结构如下：

```text
Sub2API / NewAPI
   -> Platform Ingress Handler
   -> Adapter Registry
   -> Platform Adapter (normalize)
   -> UnifiedMessage
   -> dialog / intent / handoff / ticket / audit / dedup
   -> Internal Domain Events
   -> Event Outbox
   -> Delivery Worker
   -> Platform Callback Endpoint
```

核心原则：

1. **核心主链不感知平台细节**  
   `dialog.Service` 继续只消费 `UnifiedMessage`，不直接理解 Sub2API/NewAPI 原生字段。

2. **适配逻辑边缘化**  
   平台差异集中在 adapter 目录中，用接口抽象隔离。

3. **事件先落库再投递**  
   所有异步回调事件进入 outbox 后再由 worker 重试发送，避免平台短时不可用导致结果丢失。

4. **同步 HTTP 只做最小确认**  
   入站请求同步返回“收到并入链”的最小响应，不在主请求路径里等待整条回调链路完成。

这样做的收益是：现有 webhook 主链、Gate B/Gate C 验证、鉴权、工单状态机都可以复用，不需要重写核心业务。

---

## 3. 入站适配设计

第一版入站适配增加一个新的入口族，而不是强行把平台原生大包塞进现有 `UnifiedMessage` handler。建议新增：

```text
POST /api/v1/customer-service/platforms/{platform}/webhook
POST /api/v1/customer-service/platforms/{platform}/webhook/{channel}
```

其中 `{platform}` 第一版支持：

1. `sub2api`
2. `newapi`（保留同构位，可先实现最小 profile）

当前状态补充：
- `sub2api` 已完成第一版最小接入、outbox、callback worker、dead letter 和 E2E 验证
- `newapi` 当前仅保留同构 adapter profile，占位返回 `501 profile not implemented`

新增接口：

```go
type PlatformAdapter interface {
    Platform() string
    ParseInbound(*http.Request, []byte, IngressContext) (*message.UnifiedMessage, *PlatformInboundMeta, error)
    BuildIngressAck(*dialog.Result, *PlatformInboundMeta) any
}
```

设计要点：

1. **平台原生请求体不再直接喂给现有 webhook handler**  
   先在 adapter 里裁剪、校验、映射，再构造 `UnifiedMessage`。

2. **保留平台元数据**
   `PlatformInboundMeta` 记录：
   - platform
   - tenant / app / upstream endpoint
   - raw event id
   - callback target
   - callback auth profile
   - source user/session ids

3. **统一进入现有主链**
   Adapter 输出只允许是干净的 `UnifiedMessage`，这样 `dialog.Service`、dedup、ticket、audit 无需大改。

4. **同步确认最小化**
   入站 HTTP 响应只表达：
   - `accepted`
   - `event_id`
   - `session_id`（如果已生成）
   不承担完整业务结果回写职责。

Sub2API 优先意味着第一版先针对 tksea 场景定义一个明确的 inbound profile，而不是试图抽象所有平台差异。

---

## 4. 出站全事件流设计

你明确要求第一版不是只回最终结果，而是做**全事件流异步回调**。这意味着需要在内部定义一个稳定的事件模型，而不是拿日志拼 webhook。

建议的事件类型：

1. `message.received`
2. `message.rejected`
3. `message.deduplicated`
4. `message.processing`
5. `intent.resolved`
6. `handoff.triggered`
7. `ticket.created`
8. `ticket.assigned`
9. `ticket.resolved`
10. `ticket.closed`
11. `reply.generated`
12. `callback.delivered`
13. `callback.failed`

事件统一结构建议：

```json
{
  "event_id": "uuid",
  "event_type": "reply.generated",
  "platform": "sub2api",
  "occurred_at": "2026-05-06T12:00:00Z",
  "session_id": "uuid",
  "ticket_id": "uuid",
  "source_message_id": "platform-msg-id",
  "attempt": 1,
  "payload": {}
}
```

关键设计点：

1. **事件类型稳定、字段尽量固定**
2. **事件 payload 面向平台消费，而不是内部 debug**
3. **每条事件必须有 `event_id` 供下游幂等**
4. **reply / handoff / ticket 是关键事件，必须可补偿重放**

这样第一版虽然不是完整平台集成，但已经具备后续扩展到状态同步、工单联动和运营侧诊断的事件基础。

---

## 5. 准可靠投递设计

你选择的是“准可靠投递”，这决定了我们不能把异步回调只做成 best-effort。推荐实现是**Outbox + Delivery Worker + Retry Policy + Dead Letter**。

新增持久化表建议：

1. `cs_platform_callbacks`
   - 配置每个 platform target 的回调地址、签名方式、启停状态
2. `cs_platform_event_outbox`
   - 存放待投递事件
3. `cs_platform_event_delivery_attempts`
   - 存放每次尝试结果
4. `cs_platform_event_dead_letters`
   - 存放超出重试上限的事件

投递策略：

1. 业务主链中先生成事件并落 `outbox`
2. 后台 worker 轮询领取事件
3. 成功后标记 delivered
4. 失败后指数退避重试
5. 达到上限后进入 dead letter
6. 提供人工或脚本重放入口

推荐默认策略：

1. 首次立即投递
2. 之后 `10s / 30s / 60s / 5m / 15m`
3. 最多 5 次
4. 超过进入 dead letter

这不是严格 exactly-once，但对第一版已经足够现实：

- 上游通过 `event_id` 幂等
- 我们保证“不轻易丢”
- 重试/死信让失败可追踪可恢复

---

## 6. 配置与安全设计

适配层要想落地，配置必须从“单 webhook secret”提升为“平台适配配置”。建议新增：

```text
AI_CS_PLATFORM_ADAPTERS_ENABLED=true
AI_CS_PLATFORM_SUB2API_ENABLED=true
AI_CS_PLATFORM_SUB2API_INGRESS_SECRET=...
AI_CS_PLATFORM_SUB2API_CALLBACK_BASE_URL=...
AI_CS_PLATFORM_SUB2API_CALLBACK_SECRET=...
AI_CS_PLATFORM_SUB2API_CALLBACK_TIMEOUT_MS=3000
AI_CS_PLATFORM_SUB2API_CALLBACK_MAX_RETRIES=5
AI_CS_PLATFORM_NEWAPI_ENABLED=false
```

安全要求：

1. **入站鉴权**  
   平台入口不能复用当前通用 webhook 约束的最小集合就草率上线，必须明确平台级 secret/profile。

2. **出站签名**
   回调给 Sub2API/NewAPI 的事件也要带时间戳与签名，避免被伪造。

3. **最小字段原则**
   只回推平台真正需要的字段，不把完整上下文、敏感用户数据默认外发。

4. **审计闭环**
   所有 callback 失败、重试、死信、重放都进入 `audit` 或独立 delivery attempts 表。

安全上最重要的一条是：

> **平台适配层必须是“显式启用、显式配置、显式审计”的能力，不允许默认裸开。**

---

## 7. 测试与门禁设计

第一版适配增强必须新增独立测试层，而不能只靠现有 webhook 测试顺带覆盖。

建议测试分层：

1. **Unit**
   - 平台原生 payload -> `UnifiedMessage` 映射
   - callback payload 组装
   - 签名算法
   - 重试策略

2. **Integration**
   - 平台入站请求 -> 主链处理 -> outbox 落库
   - outbox -> callback mock server
   - 失败重试 -> dead letter

3. **E2E**
   - Sub2API mock 发原生消息
   - `ai-customer-service` 创建 session / ticket / audit
   - callback mock 收到全事件流

第一版阻断门禁建议至少包含：

1. `sub2api` 最小接入 happy path
2. `message_id` 去重 path
3. 未知字段/非法签名 path
4. callback 5xx 重试 path
5. callback 最终 dead letter path
6. 回滚后 callback 恢复 path

这里要特别强调：

> 当前 `tech/TEST_DESIGN.md` 里 NewAPI/Sub2API 适配验证还是待实现项，第一版增强后必须把它提升为真正可执行的合同测试和联调测试，而不是继续停留在文档层。

---

## 8. 分阶段实施建议

为了不把当前 Phase 1 拖爆，建议按 3 个 implementation batch 执行：

### Batch 1：Sub2API 入站最小适配

1. 新增 `/platforms/sub2api/webhook`
2. 新增 adapter 接口和 `sub2api` profile
3. 原生 payload -> `UnifiedMessage`
4. 复用现有主链
5. 单测 + 集成测试

### Batch 2：事件 outbox 与异步回调

1. 设计事件模型
2. 新增 outbox 表
3. 新增 worker
4. 新增 callback 签名与投递
5. 失败重试 + dead letter

### Batch 3：NewAPI profile 与运维可观测

1. 新增 `newapi` adapter profile
2. 新增 delivery metrics / dashboard
3. 新增重放工具与 runbook
4. 补 Gate B / Gate C 适配层联调门禁

这个顺序的理由很简单：

1. 先把 Sub2API 场景跑通
2. 再把异步事件流做稳
3. 最后复用同一套抽象支持 NewAPI

---

## 9. 最终建议

我推荐按这份设计推进，因为它满足四个约束：

1. **符合项目规划**：确实开始支持 NewAPI/Sub2API
2. **不破坏当前主链**：平台差异不侵入核心客服逻辑
3. **可先解决 tksea / Sub2API 的真实问题**：不是空转设计
4. **可灰度实施**：Batch 1 完成就能先验证最小接入

最终建议一句话概括：

> **把 NewAPI/Sub2API 支持做成“内置适配器 + 事件 outbox”的最小集成层，而不是把 `ai-customer-service` 重做成另一个平台。**

下一步如果继续，最合理的是直接基于这份设计拆 implementation plan，而不是直接开写代码。
