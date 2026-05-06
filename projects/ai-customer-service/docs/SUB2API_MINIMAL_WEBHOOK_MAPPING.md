# Sub2API 最小接入映射清单

> 状态：可用于最小 webhook 接入验证  
> 最近更新：2026-05-06  
> 适用范围：`ai-customer-service` 当前 Phase 1 实现  
> 目标：验证“能否挂到 tksea 服务器上的 Sub2API 后面跑最小 webhook 场景”

---

## 1. 结论先行

当前版本的 `ai-customer-service`：

- **足以支持 Sub2API 的最小 webhook 转发接入**
- **不足以支持完整的 Sub2API 适配层**

这里的“最小 webhook 转发接入”指的是：

> Sub2API 把用户消息按当前统一格式转成一个标准 JSON，请求到  
> `POST /api/v1/customer-service/webhook`
> 或  
> `POST /api/v1/customer-service/webhook/{channel}`

然后消息进入当前客服主链：

`webhook -> dialog -> intent -> handoff -> ticket/audit/dedup`

这里的“不足以支持完整适配层”指的是：

1. 当前没有真正的 Sub2API 原生适配器实现
2. 当前没有落地 `GET /api/v1/customer-service/kb`
3. 当前没有 Sub2API 原生消息结构到 `UnifiedMessage` 的自动转换层
4. 当前没有 Sub2API 联调合同测试闭环

所以本清单的定位非常明确：

> **先验证最小消息转发能不能跑通，不等同于“已经完成 Sub2API 深度集成”。**

---

## 2. 当前 webhook 的真实契约

当前服务真实接收的消息结构在：

- [message.go](/home/long/project/立交桥/projects/ai-customer-service/internal/domain/message/message.go)

真实字段如下：

```json
{
  "message_id": "string",
  "channel": "string",
  "open_id": "string",
  "user_id": "string, optional",
  "content": "string",
  "content_type": "string, optional",
  "timestamp": "RFC3339 timestamp, optional",
  "reply_to": "string, optional"
}
```

但**最小可用集合**只有 3 个必填字段：

```json
{
  "channel": "string",
  "open_id": "string",
  "content": "string"
}
```

如果要启用去重，建议再补：

```json
{
  "message_id": "stable unique id"
}
```

真实入口在：

- [router.go](/home/long/project/立交桥/projects/ai-customer-service/internal/http/router.go)
- [webhook_handler.go](/home/long/project/立交桥/projects/ai-customer-service/internal/http/handlers/webhook_handler.go)

可用路径：

1. `POST /api/v1/customer-service/webhook`
2. `POST /api/v1/customer-service/webhook/{channel}`

第二种路径下，URL 中的 `{channel}` 会覆盖 body 里的 `channel`。

---

## 3. Sub2API -> 当前 webhook 的最小字段映射

### 3.1 推荐映射

| 当前 webhook 字段 | Sub2API 侧来源 | 必填 | 说明 |
|---|---|---:|---|
| `message_id` | 上游消息唯一 ID / request ID / event ID | 建议 | 用于 dedup；为空则不去重 |
| `channel` | 固定值或来源渠道标识 | 是 | 例如 `sub2api` / `web` / `widget` |
| `open_id` | 用户唯一标识 | 是 | 必须稳定；可用 user id / external user id |
| `user_id` | 平台内部用户 ID | 否 | 当前主链不强依赖 |
| `content` | 用户原始文本 | 是 | 当前仅文本主链最稳 |
| `content_type` | 固定 `text/plain` 或 `text` | 否 | 当前可省略 |
| `timestamp` | 事件时间 | 否 | 不传则服务端自动补当前时间 |
| `reply_to` | 上游会话/消息关联 ID | 否 | 当前主链不强依赖 |

### 3.2 最小推荐 body

最稳的最小 body：

```json
{
  "message_id": "sub2api-msg-001",
  "channel": "sub2api",
  "open_id": "user-123",
  "content": "我要退款"
}
```

如果走带 channel 的路径：

`POST /api/v1/customer-service/webhook/sub2api`

那么 body 可以进一步简化为：

```json
{
  "message_id": "sub2api-msg-001",
  "open_id": "user-123",
  "content": "我要退款"
}
```

但从当前实现看，**没有 body 内 `channel` 会被判缺字段**，因为 handler 先校验 body，再由 path override 覆盖。  
所以现阶段最稳妥的做法仍然是：

> **即使用了 `/webhook/{channel}`，body 里也继续带上 `channel`。**

推荐保持：

```json
{
  "message_id": "sub2api-msg-001",
  "channel": "sub2api",
  "open_id": "user-123",
  "content": "我要退款"
}
```

---

## 4. 不能直接多传“原生大包”

这是当前接入里最容易踩坑的一点。

`webhook_handler.go` 对 JSON 使用了：

```go
decoder.DisallowUnknownFields()
```

这意味着：

> **body 里只要带当前结构外的字段，就会直接 `400`。**

所以 Sub2API 那边**不能**把自己原生完整事件包直接透传过来，例如这类做法会失败：

```json
{
  "message_id": "sub2api-msg-001",
  "channel": "sub2api",
  "open_id": "user-123",
  "content": "我要退款",
  "conversation": {},
  "metadata": {},
  "user": {},
  "model": "gpt-4o"
}
```

正确做法是：

> **先在 Sub2API 侧或中间 shim 中裁剪，只保留当前 webhook 认识的字段。**

---

## 5. 签名鉴权要求

当前 webhook 如果启用了 `AI_CS_WEBHOOK_SECRET`，就必须带签名。

真实逻辑在：

- [webhook_security.go](/home/long/project/立交桥/projects/ai-customer-service/internal/http/handlers/webhook_security.go)

默认请求头：

- `X-CS-Timestamp`
- `X-CS-Signature`

签名算法：

```text
hex(hmac_sha256(secret, timestamp + "." + raw_body))
```

注意：

1. `timestamp` 是 Unix 秒级时间戳
2. `raw_body` 是**最终发送出去的原始 JSON 字节串**
3. 不能对 body 做二次格式化后再复算
4. 默认允许时钟偏差是 `300s`

### 5.1 伪代码

```text
ts = current_unix_seconds()
body = exact_json_bytes
signature = HMAC_SHA256_HEX(secret, ts + "." + body)

POST /api/v1/customer-service/webhook
Headers:
  Content-Type: application/json
  X-CS-Timestamp: <ts>
  X-CS-Signature: <signature>
Body:
  <body>
```

### 5.2 curl 示例

```bash
TS=$(date +%s)
BODY='{"message_id":"sub2api-msg-001","channel":"sub2api","open_id":"user-123","content":"我要退款"}'
SIG=$(printf '%s.%s' "$TS" "$BODY" | openssl dgst -sha256 -hmac "replace-with-real-secret" | awk '{print $2}')

curl -X POST "http://<host>/api/v1/customer-service/webhook" \
  -H "Content-Type: application/json" \
  -H "X-CS-Timestamp: $TS" \
  -H "X-CS-Signature: $SIG" \
  -d "$BODY"
```

---

## 6. 当前最小成功判定

如果接入正确，成功响应会是 `HTTP 200`，body 类似：

```json
{
  "received": true,
  "session_id": "uuid",
  "reply": "已为您转人工客服，请稍候，我们会尽快处理。",
  "intent": "refund",
  "handoff": true,
  "ticket_id": "uuid"
}
```

其中最值得看的是：

1. `received=true`
2. `session_id` 非空
3. `handoff` 是否符合预期
4. `ticket_id` 在需要转人工时非空

---

## 7. 当前最容易失败的 7 个点

### 7.1 body 多传了未知字段

结果：

- `400 Bad Request`

原因：

- `DisallowUnknownFields()` 拒绝未知字段

处理：

- 只保留映射表中的字段

### 7.2 缺 `channel` / `open_id` / `content`

结果：

- `400 Bad Request`

处理：

- 保证最小 3 字段始终存在

### 7.3 未带签名头

结果：

- `403`

处理：

- 带上 `X-CS-Timestamp` / `X-CS-Signature`

### 7.4 签名对的是“格式化后的 body”，不是实际发送 body

结果：

- `403 invalid webhook signature`

处理：

- 用最终发送的原始 JSON 字节串算签名

### 7.5 `timestamp` 漂移过大

结果：

- `403 stale webhook request`

处理：

- 确保 Sub2API 所在机时钟同步

### 7.6 `message_id` 不稳定或重复策略错误

结果：

- 重复消息可能被判为：
  - 正常新消息
  - 或 `duplicate message ignored`

处理：

- 让 `message_id` 对同一条上游消息稳定唯一

### 7.7 内容过长

结果：

- 当前不会拒绝
- 但会被截断到 `2000` 字符

处理：

- 如果 Sub2API 可能转发超长内容，最好先在上游截断或摘要化

---

## 8. 推荐的两种接法

### 8.1 方案 A：Sub2API 直接转发

前提：

1. Sub2API 支持自定义 webhook 目标地址
2. Sub2API 支持自定义请求头
3. Sub2API 支持自定义 body 模板，且能只输出当前需要字段

这种方案最简单，链路最短。

### 8.2 方案 B：Sub2API -> shim -> ai-customer-service

如果 Sub2API 不能：

1. 自定义 body 到足够细
2. 自定义 HMAC 头
3. 裁剪原始事件包

那就不要硬接。

应该改成：

`Sub2API -> 轻量 shim -> ai-customer-service webhook`

这个 shim 只做三件事：

1. 把 Sub2API 原始消息映射成 `UnifiedMessage`
2. 去掉未知字段
3. 按当前算法补 `X-CS-Timestamp` 和 `X-CS-Signature`

这是当前版本最稳的工程方案。

---

## 9. 当前版本对 Sub2API 的真实支持边界

### 已支持

1. 标准 webhook POST 接入
2. HMAC 鉴权
3. 基于 `message_id` 的 dedup
4. 文本消息进入主链
5. 自动产生 `session / ticket / audit`

### 未支持

1. Sub2API 原生消息结构直接接入
2. Sub2API 专用 adapter
3. Sub2API 工单拉取接口合同
4. 知识库共享接口落地
5. Sub2API 合同测试/联调测试

因此当前准确表述是：

> **当前版本可以先验证“挂到 tksea 服务器上的 Sub2API 后面跑最小 webhook 场景”，但不能宣称“已经完整支持 Sub2API 集成”。**

---

## 10. 建议的最小验证顺序

### 第一步：直接打通单条消息

目标：

- 一条最小 body 返回 `200`

### 第二步：验证 dedup

目标：

- 同一 `message_id` 重放，返回 `duplicate message ignored`

### 第三步：验证真实业务文本

目标：

- 例如“我要退款”能触发 `handoff=true`

### 第四步：再决定要不要补 shim / adapter

如果前三步都只能靠大量平台侧 hack 才能做到，就应立即转为方案 B：

> **加一个轻量 shim，不要继续硬耦合 Sub2API 原生结构。**

---

## 11. 当前建议

如果你的目标是：

> “先验证能不能挂到 tksea 服务器上的 Sub2API 后面跑最小 webhook 场景”

那我建议直接按下面顺序推进：

1. 让 Sub2API 输出最小 body
2. 按当前签名算法补头
3. 先连到 `POST /api/v1/customer-service/webhook`
4. 跑单条消息验证
5. 跑重复消息验证

如果 Sub2API 做不到：

1. 自定义最小 body
2. 自定义签名头

就立刻切到：

> **Sub2API -> shim -> ai-customer-service**

不要在 Sub2API 本体里过度折腾。
