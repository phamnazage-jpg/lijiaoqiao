# 客服运营后台需求说明

> 版本：v1.0 | 状态：已生效
> 关联：tech/INTERFACE.md、PRODUCTION_PHASE1_STATUS.md

---

## 1. 概述

客服运营后台是客服团队日常操作的核心工具，提供工单管理、会话查询、运营统计等能力。本文档定义生产一期的后台需求范围与接口规范。

---

## 2. 当前已落地的后台能力

### 2.1 工单管理（API 层）

| 功能 | 接口 | 状态 | 代码位置 |
|------|------|------|----------|
| 工单列表 | `GET /api/v1/customer-service/tickets` | ✅ 已落地 | `internal/http/router.go` |
| 工单详情 | `GET /api/v1/customer-service/tickets/{id}` | ✅ 已落地 | `internal/http/router.go` |
| 工单分配 | `POST /api/v1/customer-service/tickets/{id}/assign` | ✅ 已落地 | `internal/http/router.go` |
| 工单解决 | `POST /api/v1/customer-service/tickets/{id}/resolve` | ✅ 已落地 | `internal/http/router.go` |
| 工单关闭 | `POST /api/v1/customer-service/tickets/{id}/close` | ✅ 已落地 | `internal/store/postgres/ticket_workflow.go` |
| 工单统计 | `GET /api/v1/customer-service/tickets/stats` | ❌ 未落地（无独立 stats endpoint） | — |

### 2.2 健康检查

| 功能 | 接口 | 状态 |
|------|------|------|
| 存活检查 | `GET /actuator/live` | ✅ 已落地 |
| 就绪检查 | `GET /actuator/ready` | ✅ 已落地（含 PostgreSQL 依赖检查） |
| 健康检查 | `GET /actuator/health` | ✅ 已落地 |

---

## 3. 运营后台需求清单（生产一期范围）

### 3.1 核心需求（生产一期必须落地）

#### P0：工单运营视图

**需求描述**：客服人员可通过后台看到所有工单，并执行分配/解决操作。

**已落地**：
- 工单列表（按 status / assigned_to / priority 过滤）
- 工单分配（assign）
- 工单解决（resolve）
- 工单统计（总计、各状态数量）

**已收口 P0 缺口**：
- ✅ 工单状态流转审计（assign/resolve/close 均通过 `TicketWorkflowStore.writeAudit` 写入审计日志）
- ✅ 工单关闭语义（resolve=已解决关闭；另有独立 close 接口支持显式关闭）

#### P1：转人工原因分析

**需求描述**：运营团队需要看到转人工的原因分布，用于优化机器人回答质量。

**当前状态**：代码中 `handoff_service.CreateTicket` 记录了 `handoff_reason`，但**无专门的后台聚合接口**。

**待实现**：
- `GET /api/v1/customer-service/admin/handoff-reasons` — 按原因聚合统计
- 关联 `tech/INTERFACE.md` 中已定义的 `/admin/handoff-reasons` 接口

#### P1：会话历史查看

**需求描述**：客服处理工单时需要查看用户完整的对话历史。

**当前状态**：`GET /api/v1/customer-service/sessions/{id}/messages` 接口**已定义但未完全落地**。

---

### 3.2 延伸需求（生产一期明确排除）

以下功能不在生产一期范围内：

| 功能 | 排除原因 |
|------|----------|
| 知识库 CRUD / 发布 / 审核 | Phase 4 才落地 |
| WebSocket 实时会话 | Phase 4 才落地 |
| 客服排班 / 考勤 | 独立系统 |
| 用户满意度评价 | P1 待落地 |
| 质检 / 录音存档 | 独立系统 |
| 多租户隔离 | 后续版本 |

---

## 4. 接口详细说明

### 4.1 工单列表 `GET /api/v1/customer-service/tickets`

**查询参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `status` | string | 过滤状态：`open`、`assigned`、`resolved`、`closed` |
| `assigned_to` | string | 过滤客服 |
| `priority` | string | 过滤优先级：`P1`、`P2`、`P3` |
| `page` | int | 页码（默认 1） |
| `page_size` | int | 每页条数（默认 20，最大 100） |

**响应**：

```json
{
  "tickets": [
    {
      "id": "uuid",
      "session_id": "string",
      "user_id": "string",
      "priority": "P1",
      "status": "open",
      "handoff_reason": "refund_request",
      "assigned_to": null,
      "resolution": null,
      "created_at": "2026-04-30T10:00:00Z",
      "updated_at": "2026-04-30T10:00:00Z"
    }
  ],
  "total": 50,
  "page": 1,
  "page_size": 20
}
```

### 4.2 工单分配 `POST /api/v1/customer-service/tickets/{id}/assign`

**请求**：
- Query 参数：`agent_id`（必填）

**错误码**：
- `CS_TKT_4001`：工单不存在（404）
- `CS_TKT_4002`：工单已被分配（409）
- `CS_AUTH_4001`：越权访问（403）

### 4.3 工单解决 `POST /api/v1/customer-service/tickets/{id}/resolve`

**请求**：
- Query 参数：`resolution`（必填，说明解决方式）

### 4.4 工单统计 `GET /api/v1/customer-service/tickets/stats`

**响应**：

```json
{
  "total": 100,
  "open": 15,
  "assigned": 30,
  "resolved": 55,
  "by_priority": {
    "P1": 20,
    "P2": 50,
    "P3": 30
  },
  "avg_resolution_time_minutes": 45
}
```

### 4.5 转人工原因统计 `GET /api/v1/customer-service/admin/handoff-reasons`

**响应**：

```json
{
  "reasons": [
    { "reason": "refund_request", "count": 45, "percentage": 35 },
    { "reason": "sensitive_content", "count": 30, "percentage": 23 },
    { "reason": "manual_request", "count": 25, "percentage": 19 },
    { "reason": "unknown", "count": 29, "percentage": 23 }
  ],
  "total": 129
}
```

---

## 5. 后台权限模型

### 5.1 角色定义

| 角色 | 权限 |
|------|------|
| `agent` | 查看自己被分配的工单、执行 assign/resolve |
| `supervisor` | 查看所有工单、查看统计数据、转人工原因分析 |
| `admin` | 所有权限 |

### 5.2 当前状态

生产一期**权限模型未落地**，所有接口无鉴权。Phase 4 运营后台才需要完整的 RBAC。

---

## 6. 当前版本状态

- **本文档版本**：v1.0
- **生效日期**：2026-04-30
- **下次审查**：Phase 4 开始前
