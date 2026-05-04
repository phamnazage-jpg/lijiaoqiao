# AI-Customer-Service 生产一期执行状态

> 更新时间：基于当前代码现状人工核对。
> 目的：把生产一期要求映射到当前实现边界，避免继续把原型能力误报为“已完成”。

## 1. 当前结论

当前项目仍处于**生产一期未完成**状态，但已具备以下已落地能力：

- 基础配置加载与 HTTP 超时/Body Limit 配置
- webhook body schema 校验
- webhook HMAC 签名与时间戳防重放校验
- 消息幂等去重
- 基于依赖检查的 `/actuator/health`、`/actuator/health/live`、`/actuator/health/ready`
- 转人工工单创建
- 工单列表 / 分配 / 解决 / 关闭最小闭环 API
- 审计日志持久化写入
- PostgreSQL migration 基础表结构
- 后台接口最小 header 鉴权与角色校验

但距离“生产一期完成”仍有明显缺口，不能作为可灰度上线结论。

---

## 2. 生产一期需求到当前代码映射

### 2.1 入口安全

| 要求 | 当前状态 | 代码位置 | 备注 |
|---|---|---|---|
| 请求体大小限制 | 已完成 | `internal/platform/httpx/limits.go`, `internal/http/router.go` | 已挂到 webhook 路由 |
| JSON schema/字段约束 | 部分完成 | `internal/http/handlers/webhook_handler.go` | 仅完成最小字段必填与 unknown field 拒绝 |
| webhook 签名校验 | 已完成 | `internal/http/handlers/webhook_security.go` | HMAC-SHA256 |
| 时间戳防重放 | 已完成 | `internal/http/handlers/webhook_security.go` | 仅做 skew 校验，未持久化 nonce |
| 幂等去重 | 已完成 | `internal/store/postgres/dedup_store.go`, `internal/store/memory/dedup_store.go` | 基于 `(channel,message_id)` |
| 速率限制 | 已完成 | `internal/platform/httpx/limits.go`, `internal/http/router.go` | 当前已挂到 webhook 路由 |
| 渠道级独立 webhook 适配器 | 未完成 | 当前仅具备统一 webhook 与路径覆写 channel | 与最终多渠道适配目标仍有距离 |

### 2.2 工单闭环

| 要求 | 当前状态 | 代码位置 | 备注 |
|---|---|---|---|
| 转人工自动创建工单 | 已完成 | `internal/service/dialog/service.go` | 退款/敏感意图触发 |
| 工单持久化 | 已完成 | `internal/store/postgres/ticket_store.go` | PostgreSQL / memory 均可 |
| 工单列表 | 已完成 | `internal/http/handlers/ticket_handler.go` | `GET /tickets` |
| 工单分配 | 已完成 | `internal/http/handlers/ticket_handler.go`, `internal/store/postgres/ticket_workflow.go` | 当前由 header 鉴权 + query 业务参数驱动 |
| 工单解决 | 已完成 | 同上 | 当前由 header 鉴权 + query 业务参数驱动 |
| 工单关闭 | 已完成 | `internal/http/handlers/ticket_handler.go`, `internal/store/postgres/ticket_workflow.go` | 当前由 header 鉴权 + query 业务参数驱动 |
| 工单回复用户 | 未完成 | 无 | 尚无人工回消息链路 |
| 排队位置查询 | 未完成 | 无 | 文档要求未落地 |

### 2.3 审计与可追溯

| 要求 | 当前状态 | 代码位置 | 备注 |
|---|---|---|---|
| message processed 审计 | 已完成 | `internal/service/dialog/service.go` | 成功路径会写审计 |
| 审计持久化 | 已完成 | `internal/store/postgres/audit_store.go` | 写 `cs_audit_logs` |
| fail-closed 审计 | 已完成 | `dialog.Process()` | 审计失败时整体返回错误 |
| 安全拒绝事件审计 | 已完成 | `internal/http/handlers/webhook_security.go` | 签名缺失/时间戳异常/签名不匹配会写审计 |
| 工单状态流转审计 | 已完成 | `internal/http/handlers/ticket_handler.go`, `internal/store/postgres/ticket_workflow.go` | assign/resolve/close 已写状态流转审计 |
| source_ip / actor / action 分类完备 | 部分完成 | `internal/http/handlers/ticket_handler.go`, `internal/http/handlers/session_handler.go`, `internal/store/postgres/audit_store.go` | 当前已记录 source_ip/actor，但完整分类体系仍可继续收紧 |

### 2.4 运维与健康检查

| 要求 | 当前状态 | 代码位置 | 备注 |
|---|---|---|---|
| liveness / readiness 区分 | 已完成 | `internal/http/handlers/health_handler.go` | |
| readiness 检查依赖 | 已完成 | `internal/platform/health/dependency.go`, `internal/store/postgres/healthcheck.go` | 当前仅 postgres |
| graceful shutdown | 已完成 | `internal/app/app.go` | |
| 结构化日志 | 部分完成 | `internal/platform/logging/logger.go`, `webhook_handler.go` | 仅少量入口日志 |
| metrics/tracing | 未完成 | 无 | P1 缺口 |
| 灰度/回滚 runbook | 部分完成 | `docs/RUNBOOK.md`, `prd/GRAY_RELEASE_ROLLBACK_RUNBOOK.md` | 文档已交付，演练与证据化验证待补 |

---

## 3. 当前与文档的主要漂移

1. 文档中的最终形态仍包含真实多渠道适配器、LLM、RAG 与运营后台，当前代码尚未覆盖这些范围。
2. 当前后台接口已加最小 header 鉴权，但完整 RBAC、用户级数据隔离仍未落地。
3. 当前仍缺人工回复用户链路与排队位置查询。
4. 文档要求更完整的运维可观测（metrics/tracing/SLO），当前尚未实现。

---

## 4. 剩余 P0 / P1 缺口排序

### P0（继续执行必须优先收口）

1. 完整 RBAC 与用户级数据隔离补齐
2. 工单 API 与接口文档继续对齐（尤其是后台鉴权契约）
3. 人工回复用户链路补齐
4. 灰度与回滚演练证据化

### P1（生产一期仍必须完成）

1. 排队位置查询
2. metrics / tracing / SLO 基础设施
3. 灰度/回滚演练
4. 真实多渠道适配器产品化
5. 真实 LLM / RAG 能力

---

## 5. 本轮执行边界

本轮后续代码推进应聚焦：

1. 补齐安全拒绝审计
2. 补齐工单状态流转审计
3. 补齐工单关闭/文档对齐的最小闭环
4. 扩展自动化测试覆盖主路径/失败路径/安全路径

在这些项完成前，不应把项目汇报为“生产一期已完成”。
