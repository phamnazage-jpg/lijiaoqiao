# 生产一期范围与门禁定义

> 版本：v1.0 | 状态：已生效
> 关联：PRODUCTION_EXECUTION_PLAN.md、PRODUCTION_PHASE1_STATUS.md、tech/INTERFACE.md

---

## 1. 生产一期目标定位

生产一期是 ai-customer-service 从原型验证到生产可用的第一步。目标不是功能完备，而是**入口安全、闭环真实、运维可控**，在有限范围内做到生产级别质量。

---

## 2. 已落地能力（生产一期基线）

以下能力已在代码中实现并通过验证：

| 能力 | 代码位置 | 说明 |
|------|----------|------|
| webhook HMAC 签名校验 | `internal/http/handlers/webhook_security.go` | HMAC-SHA256，skew 校验 |
| 时间戳防重放 | `internal/http/handlers/webhook_security.go` | skew window 内有效 |
| 消息幂等去重 | `internal/store/postgres/dedup_store.go` | `(channel, message_id)` 去重 |
| 工单创建 | `internal/service/dialog/service.go` | 退款/敏感意图触发转人工 |
| 工单持久化 | `internal/store/postgres/ticket_store.go` | PostgreSQL |
| 工单列表/分配/解决 | `internal/http/handlers/ticket_handler.go` | `GET /tickets`、`POST /assign`、`POST /resolve` |
| 审计日志持久化 | `internal/store/postgres/audit_store.go` | 写入 `cs_audit_logs`，fail-closed |
| 健康检查 | `internal/http/handlers/health_handler.go` | `/live`、`/ready`（含 PostgreSQL 依赖检查） |
| 请求体大小限制 | `internal/platform/httpx/limits.go` | 全局 BodyLimit 配置 |
| JSON Schema 校验 | `internal/http/handlers/webhook_handler.go` | 最小字段必填与 unknown field 拒绝 |
| graceful shutdown | `internal/app/app.go` | 优雅停机 |

---

## 3. 生产一期明确排除范围

以下能力**不在生产一期范围内**，不作为阶段完成的阻塞项：

- 人工回复用户链路（人工客服 → 用户消息推送）
- 排队位置查询
- webhook 速率限制
- metrics / tracing / SLO 监控面板
- 知识库 CRUD / 发布 / 审核
- WebSocket 实时会话
- 多租户隔离
- 外部系统（NewAPI/Sub2API）深度集成

---

## 4. 剩余 P0 缺口（门禁必须项）

在以下 P0 缺口**全部收口**前，不得将项目状态汇报为"生产一期完成"：

### P0-1：工单状态流转审计
- **当前状态**：✅ 已落地，`TicketWorkflowStore` 在 Assign/Resolve/Close 时均调用 `writeAudit`
- **代码位置**：`internal/store/postgres/ticket_workflow.go`
- **记录内容**：before_state（隐式）/ after_state（显式）、actor_id、source_ip、action（assign/resolve/close）

### P0-2：安全拒绝事件审计
- **当前状态**：✅ 已落地，`WebhookSecurity.auditReject` 在签名缺失/无效/过期/body 读取失败时均写入审计
- **代码位置**：`internal/http/handlers/webhook_security.go`
- **记录内容**：Type=`webhook_security_rejected`，Action=`security_reject`，error_code、path、timestamp 等信息

### P0-3：工单关闭语义明确
- **当前状态**：只有 resolve，没有 close 语义
- **要求**：工单关闭语义明确为 resolve=已解决关闭，或补充 close 接口
- **代码位置**：`internal/http/handlers/ticket_handler.go`

### P0-4：Webhook 路由对齐
- **当前状态**：已落地统一入口 `/api/v1/customer-service/webhook`
- **INTERFACE.md 定义**：`/api/v1/customer-service/webhook/{channel}`（按渠道独立入口）
- **当前方案**：统一入口通过 Query/Body 中的 `channel` 字段识别渠道，与 INTERFACE 定义兼容，无需路由拆分
- **说明**：生产一期采用统一入口简化运维；如后续渠道量增加，可扩展为 `/webhook/{channel}` 路径

---

## 5. 门禁检查表

### Gate A：允许进入生产底座实现
- [x] 生产一期范围文档已建立（本文档）
- [x] PM / TechLead / QA 对范围达成一致
- [ ] TechLead 生产架构方案已冻结

### Gate B：允许联调前
- [x] webhook 签名、防重放、幂等、鉴权、审计 fail-closed 已具备
- [x] P0-1（工单状态流转审计）已落地
- [x] P0-2（安全拒绝事件审计）已落地
- [x] P0-3（工单关闭语义）已明确：resolve=已解决关闭，另有独立 close 接口支持
- [x] P0-4（Webhook 路由）已对齐：统一入口兼容 INTERFACE 定义
- [ ] OpenAPI 与实现一致（无漂移）
- [x] readiness 健康检查可真实阻断坏实例
- [ ] 关键失败路径自动化测试存在

### Gate C：允许灰度前
- [ ] P1 缺口（速率限制、人工回复链路、排队位置查询、metrics/tracing）明确完成或推迟计划
- [ ] 灰度/回滚 Runbook 已完成并演练
- [ ] 工单闭环真实可用
- [ ] 监控告警上线

---

## 6. 范围变更策略

任何范围变更（如新增功能、调低优先级）必须：
1. PM 提出书面变更申请
2. TechLead 评估技术影响
3. 三方（PM/TechLead/QA）签字确认
4. 更新本文档版本号

---

## 7. 当前版本状态

- **本文档版本**：v1.1
- **生效日期**：2026-04-30
- **更新内容**：P0-1（工单状态流转审计）、P0-2（安全拒绝事件审计）、P0-4（Webhook 路由对齐）已确认落地，更新门禁检查表状态
- **下次审查**：灰度前最终检查
