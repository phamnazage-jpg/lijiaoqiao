# 工单运营闭环 SOP

> 版本：v1.0 | 状态：已生效
> 关联：tech/INTERFACE.md、PRODUCTION_PHASE1_STATUS.md

---

## 1. 工单生命周期

```
用户触发转人工
    → [待落地] 工单创建（含排队位置）
    → 客服接单（assign）
    → 客服处理
    → 客服解决（resolve）
    → [待明确] 工单关闭（close？）
    → 用户满意度反馈（可选）
```

---

## 2. 各状态定义

| 状态 | 含义 | 触发条件 | 当前是否落地 |
|------|------|----------|--------------|
| `open` | 待接单 | 转人工触发工单创建 | ✅ 已落地 |
| `assigned` | 已分配 | 客服主动接单或系统分配 | ✅ 已落地 |
| `resolved` | 已解决 | 客服处理完毕 | ✅ 已落地 |
| `closed` | 已关闭 | 显式调用 close 接口 | ✅ 已落地（`TicketWorkflowStore.Close`） |

---

## 3. 触发转人工的条件

### 3.1 自动转人工（系统触发）

以下意图识别结果会**自动创建工单**（代码：`internal/service/dialog/service.go`）：

- 退款请求（intent = refund / 退款）
- 敏感内容（intent.sensitive = true）

### 3.2 手动转人工

- 用户发送"人工客服"、"转人工"等关键词（需 RAG 识别后触发）
- 会话 turnCount 超过阈值（待实现）

---

## 4. 工单创建流程

### 4.1 当前已落地（最小闭环）

**接口**：`POST /api/v1/customer-service/sessions/{session_id}/handoff`

**代码**：`internal/service/dialog/service.go` → `handoff_service.CreateTicket`

**流程**：
1. 对话服务检测到需要转人工
2. 创建 ticket 记录（session_id, user_id, priority, handoff_reason）
3. ticket 状态 = `open`
4. 触发审计日志写入

**缺失项**：
- 工单创建时**未记录上下文快照**（`context_snapshot` 字段为空）
- 排队位置**未实现**（用户无法查询前面还有多少人）
- 工单创建**未主动通知**客服（无消息推送链路）

### 4.2 待落地项

| 缺失项 | 优先级 | 说明 |
|--------|--------|------|
| 工单创建时上下文快照 | P0 | 用于客服接手时了解会话历史 |
| 排队位置查询 API | P1 | `GET /tickets/queue-position` |
| 客服新工单通知 | P1 | 飞书/邮件/站内信通知 |
| 客服回复用户链路 | P1 | 人工消息推送回用户 |

---

## 5. 工单分配流程

### 5.1 已落地

**接口**：`POST /api/v1/customer-service/tickets/{id}/assign?agent_id={agent_id}`

**代码**：`internal/http/handlers/ticket_handler.go` → `POST /tickets/{id}/assign`

**流程**：
1. 客服调用 assign 接口
2. 更新 ticket.status = `assigned`，ticket.assigned_to = agent_id
3. 写入审计日志（✅ 已落地：调用 `TicketWorkflowStore.writeAudit`）

**缺失项**：
- 工单状态流转审计 ✅ 已落地（`TicketWorkflowStore.writeAudit` 在 assign 时调用）

---

## 6. 工单解决流程

### 6.1 已落地

**接口**：`POST /api/v1/customer-service/tickets/{id}/resolve?resolution={resolution}`

**流程**：
1. 客服处理完毕后调用 resolve
2. 更新 ticket.status = `resolved`，ticket.resolution = resolution
3. 写入审计日志（✅ 已落地：调用 `TicketWorkflowStore.writeAudit`）

**缺失项**：
- 工单状态流转审计 ✅ 已落地（`TicketWorkflowStore.writeAudit` 在 resolve 时调用）

---

## 7. 工单关闭流程

### 7.1 当前状态

**已落地**：`TicketWorkflowStore.Close` 接口已实现，支持显式关闭工单。

**语义定义**：
- `resolve` = 客服确认问题已解决，工单进入 `resolved` 状态
- `close` = 工单正式关闭，进入 `closed` 状态（resolved 后可选调用）
- 已解决工单（resolved）可直接 close；未解决工单也可强制 close

---

## 8. 客服工作台操作规范（API 层）

### 8.1 班次开始

1. 调用 `GET /api/v1/customer-service/tickets?status=open` 查看当前待接单工单
2. 按 priority（ P1 > P2 > P3）和创建时间排序

### 8.2 接单

```bash
curl -X POST "https://{host}/api/v1/customer-service/tickets/{ticket_id}/assign?agent_id={agent_id}"
```

成功后工单状态变为 `assigned`

### 8.3 处理与解决

```bash
curl -X POST "https://{host}/api/v1/customer-service/tickets/{ticket_id}/resolve?resolution={解决说明}"
```

### 8.4 工单列表查询

```bash
# 查看所有 open 工单
curl "https://{host}/api/v1/customer-service/tickets?status=open"

# 查看指定客服的工单
curl "https://{host}/api/v1/customer-service/tickets?assigned_to={agent_id}"

# 查看统计
curl "https://{host}/api/v1/customer-service/tickets/stats"
```

---

## 9. 用户侧体验

### 9.1 转人工后用户感知

**当前已落地**：用户发送敏感/退款意图 → 收到机器人回复"已为您转接人工客服，请稍候"

**待落地**：
- 排队位置（如"前面还有 3 位在等待"）
- 人工客服接单通知
- 人工处理进度更新
- 解决后的满意度评价

---

## 10. SOP 执行检查单

### 客服班次检查

- [ ] 登录运营后台，查看当前 open 工单数量
- [ ] 按 P1优先原则接单
- [ ] 处理完毕后调用 resolve 接口
- [ ] 如遇无法解决的工单，升级 Team Lead

### 异常处理

- [ ] 工单 assign 后长时间（> 2h）未 resolve → 系统告警（待实现）/ 人工巡检
- [ ] 同一用户连续创建 > 3 个 open 工单 → 异常标记，人工复核
- [ ] 工单创建失败（服务异常） → 降级：保留内存记录 → 恢复后补录

---

## 11. 当前版本状态

- **本文档版本**：v1.0
- **生效日期**：2026-04-30
- **下次审查**：灰度阶段复盘
