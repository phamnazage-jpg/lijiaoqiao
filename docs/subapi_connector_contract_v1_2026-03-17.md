# Subapi Connector 契约清单 v1

- 版本：v1.0
- 日期：2026-03-17
- 适用阶段：S1-S2（`subapi` 外部服务模块化接入 + 客户迁移）
- 契约目标：为我方 Router Core 与 `subapi` 之间的调用定义“稳定接口 + 稳定语义 + 可回归验证”。

## 1. 设计目标与边界

## 1.1 目标

1. 将 `subapi` 视为“可替换外部能力模块”，而不是核心业务真相源。
2. 固定北向协议与字段语义，避免上游快速迭代导致我方主路径抖动。
3. 对请求、响应、错误、流式事件建立统一归一模型，支撑审计、计费、告警。

## 1.2 非目标

1. 不在本契约覆盖 `subapi` 后台管理 API（`/admin/*`）。
2. 不在本契约定义我方控制面业务模型（租户、账务、RBAC）的内部存储结构。
3. 不将 `subapi` 私有实现细节（调度算法内部参数）暴露到我方公共 API。

## 2. 接入范围（协议与端点）

## 2.1 Canonical 端点（Connector 只使用这一组）

| 协议域 | Method | Path | 说明 |
|---|---|---|---|
| Anthropic 兼容 | `POST` | `/v1/messages` | 统一消息入口 |
| Anthropic 兼容 | `POST` | `/v1/messages/count_tokens` | 仅计数，不计费记录 |
| OpenAI 兼容 | `POST` | `/v1/chat/completions` | Chat Completions |
| OpenAI 兼容 | `POST` | `/v1/responses` | Responses |
| OpenAI 兼容 | `POST` | `/v1/responses/*subpath` | Responses 子资源 |
| OpenAI 兼容（WS） | `GET` | `/v1/responses` | WebSocket 升级入口 |
| 通用 | `GET` | `/v1/models` | 模型目录 |
| 通用 | `GET` | `/v1/usage` | 用量/额度信息 |
| Gemini 原生 | `GET` | `/v1beta/models` | 模型列表 |
| Gemini 原生 | `GET` | `/v1beta/models/:model` | 模型详情 |
| Gemini 原生 | `POST` | `/v1beta/models/*modelAction` | `generateContent`/`streamGenerateContent` |

## 2.2 Alias 端点处理

1. `subapi` 提供不带 `/v1` 的别名（如 `/responses`、`/chat/completions`）。
2. Connector 禁止调用 alias，统一走 canonical 端点，避免路由歧义。

## 3. 认证与请求头契约

## 3.1 认证头优先级

1. `/v1/*`：
   - `Authorization: Bearer <key>` > `x-api-key` > `x-goog-api-key`
2. `/v1beta/*`：
   - `x-goog-api-key` > `Authorization: Bearer <key>` > `x-api-key` > `key(query，仅兼容)`

## 3.2 禁止项

1. 禁止通过 query 传 `key`/`api_key`（仅保留兼容读取，不作为标准路径）。
2. Connector 默认不发送 query key。

## 3.3 会话亲和相关头

1. OpenAI 兼容：支持 `session_id`、`conversation_id`，请求体支持 `prompt_cache_key`。
2. Gemini CLI：支持 `x-gemini-api-privileged-user-id` + 请求体 tmp 路径哈希会话识别。
3. Anthropic 兼容：可使用 `metadata.user_id` 补充分组内会话亲和。

## 3.4 北向/南向边界（安全强约束，新增）

1. 北向（客户 -> 我方网关）：
   - 禁止接收任何 query key（`key`/`api_key`），统一要求 header 鉴权。
   - 外部携带 query key 的请求必须被拒绝并记录审计事件。
2. 南向（我方网关 -> subapi connector）：
   - 仅允许 header 方式传递凭证（`Authorization`/`x-api-key`/`x-goog-api-key`）。
   - Connector 默认不透传 query key。
3. 历史兼容策略：
   - 若为极少数遗留客户端保留“内部改写”能力，必须在北向入口完成 query->header 改写，且该兼容规则需要白名单与审计日志。
   - 该兼容规则默认关闭，并纳入版本化淘汰计划。

## 4. 请求体契约（最小强约束）

## 4.1 通用约束

1. Body 必须是非空 JSON。
2. `model` 必须为非空字符串。
3. `stream` 若出现，必须是布尔值。

## 4.2 OpenAI Responses 额外约束

1. `previous_response_id` 若存在，必须是 `resp_*` 风格，不可传 message id。
2. `input.type=function_call_output` 场景必须满足 call 上下文约束（否则应返回 `400 invalid_request_error`）。

## 4.3 Gemini Action 路径约束

1. `*modelAction` 支持两种格式：`{model}:{action}` 或 `{model}/{action}`。
2. `streamGenerateContent` 视为流式请求。

## 5. 响应归一契约（Connector 内部模型）

Connector 必须将不同协议响应归一为以下内部结构：

```json
{
  "protocol": "openai|anthropic|gemini",
  "request_id": "string",
  "upstream_request_id": "string",
  "model_requested": "string",
  "model_actual": "string",
  "stream": true,
  "usage": {
    "input_tokens": 0,
    "output_tokens": 0,
    "cache_creation_input_tokens": 0,
    "cache_read_input_tokens": 0,
    "total_tokens": 0
  },
  "first_token_ms": 0,
  "duration_ms": 0,
  "client_disconnect": false
}
```

## 5.1 字段规则

1. `upstream_request_id`：优先从响应头 `x-request-id` 提取。
2. `request_id`：若协议返回 body request id，则保留；否则由 Connector 生成稳定 ID。
3. `usage.total_tokens`：若上游未提供，按 `input + output` 计算；均缺失时置 0。
4. `usage` 缺失不视为失败，但必须打 `usage_extracted=false` 观测标签（用于后续补偿分析）。

## 6. 错误归一契约

## 6.1 上游原生错误格式（三类）

1. OpenAI 风格：`{"error":{"type":"...","message":"..."}}`
2. Anthropic 风格：`{"type":"error","error":{"type":"...","message":"..."}}`
3. Google 风格：`{"error":{"code":403,"message":"...","status":"PERMISSION_DENIED"}}`

## 6.2 Connector 统一错误结构

```json
{
  "http_status": 429,
  "category": "auth|billing|rate_limit|upstream|validation|internal",
  "code": "RATE_LIMIT_EXCEEDED",
  "message": "human readable",
  "retryable": true
}
```

## 6.3 默认映射规则

1. `401/403` 上游鉴权类错误 -> `category=auth`，`retryable=false`。
2. `429` -> `category=rate_limit`，`retryable=true`。
3. `500/502/503/504/529` -> `category=upstream`，`retryable=true`。
4. 计费检查失败（余额/订阅/额度） -> `category=billing`，`retryable` 取决于具体码。
5. JSON/字段校验失败 -> `category=validation`，`retryable=false`。

## 7. 流式契约（SSE/WS）

## 7.1 SSE

1. 流开始后如发生错误，按事件帧返回，不再回写普通 JSON 错误。
2. 一旦已向客户端写出任何流内容，Connector 禁止触发“同请求 failover 重放”。
3. 必须记录首字时延 `first_token_ms` 与终止类型（正常结束/上游错误/客户端断开）。

## 7.2 WebSocket（OpenAI Responses）

1. 首帧必须包含合法 JSON 且含 `model`。
2. 若首帧不合法，立即关闭连接并返回协议错误。
3. 多轮 turn 必须重新获取并发槽位（避免长连接长期占槽）。

## 8. 重试与回退契约（Connector 侧）

1. 仅在“未输出任何字节”时允许请求级重试。
2. 流式一旦开始输出，禁止自动重试。
3. 推荐重试上限：
   - 非流式：最多 2 次（指数退避）
   - 流式：0 次（依赖上游内部 failover）
4. HTTP 429/503 可进入短退避重试；4xx 校验/鉴权错误直接失败。

## 9. 版本与兼容治理

## 9.1 版本锁定

1. 生产固定 `subapi` 精确版本（`vX.Y.Z`），不允许漂移到未验证版本。
2. 升级必须进入周级升级窗口，禁止临时直升生产。

## 9.2 契约测试门槛（每次升级必须通过）

1. OpenAI `/v1/chat/completions` 非流/流式基础场景。
2. OpenAI `/v1/responses` + `previous_response_id` 校验场景。
3. Anthropic `/v1/messages` + `count_tokens` 场景。
4. Gemini `/v1beta/models/*` 普通与流式场景。
5. 三类错误格式的归一验证（OpenAI/Anthropic/Google）。
6. 流式中断与客户端断开场景（保证不重放、不漏记）。

## 10. 与 S2 目标的对齐（执行约束）

1. `S2` 结束时：
   - 全供应商主路径由自研 Router Core 接管率 `>= 60%`
   - 国内 LLM 供应商主路径接管率 `= 100%`
2. `subapi` 在 S2 后只承担：
   - 长尾协议兼容
   - 备用回退通道

## 11. 证据来源（本地代码）

1. 路由范围与协议入口：  
`/home/long/project/立交桥/llm-gateway-competitors/sub2api-tar/backend/internal/server/routes/gateway.go`
2. OpenAI/Anthropic 错误与流式处理：  
`/home/long/project/立交桥/llm-gateway-competitors/sub2api-tar/backend/internal/handler/openai_gateway_handler.go`
3. Claude 兼容入口、`models`/`usage`/`count_tokens`：  
`/home/long/project/立交桥/llm-gateway-competitors/sub2api-tar/backend/internal/handler/gateway_handler.go`
4. Gemini 原生入口与错误格式：  
`/home/long/project/立交桥/llm-gateway-competitors/sub2api-tar/backend/internal/handler/gemini_v1beta_handler.go`
5. 认证优先级与禁用 query key：  
`/home/long/project/立交桥/llm-gateway-competitors/sub2api-tar/backend/internal/server/middleware/api_key_auth.go`  
`/home/long/project/立交桥/llm-gateway-competitors/sub2api-tar/backend/internal/server/middleware/api_key_auth_google.go`
6. 会话哈希与 header 语义：  
`/home/long/project/立交桥/llm-gateway-competitors/sub2api-tar/backend/internal/service/openai_gateway_service.go`  
`/home/long/project/立交桥/llm-gateway-competitors/sub2api-tar/backend/internal/service/gateway_service.go`

## 12. 下一步（v1 -> v1.1）

1. 把本契约转成机器可执行测试清单（YAML + golden cases）。
2. 为每个端点补充“最小请求样例 + 最小响应样例”文件。
3. 将错误映射表下沉为配置化规则，减少硬编码发布频率。
