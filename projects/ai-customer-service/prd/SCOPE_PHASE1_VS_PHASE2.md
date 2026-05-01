# 生产一期范围定义 vs Phase 2（接口级决策）

> 版本：v1.0 | 日期：2026-04-30
> 决策人：PM（小龙团队）
> 关联：QA_CHECKLIST.md、PRODUCTION_EXECUTION_PLAN.md、PRODUCTION_PHASE1_SCOPE.md

---

## 1. 背景

QA CHECKLIST.md 发现 16+ 接口与文档存在严重漂移，且错误码定义不一致。PM 需要决策每个漂移接口属于：
- **Phase 1**：生产一期必须实现，否则阻断上线
- **Phase 2**：可推迟到 Phase 2，不阻断当前上线
- **废弃**：从 INTERFACE.md 中移除，不实现

---

## 2. 决策原则

### Phase 1 原则（按 PRIORITY 排列）
真实持久化 > 安全审计 > 工单闭环 > 可观测 > 灰度可回滚

### Phase 2 原则
- RAG/知识库运营（KB 端点）
- 运营后台（dashboard/统计/质检）
- 身份核验
- 大模型 failover
- 商业化

---

## 3. 接口级决策

### 3.1 会话管理接口

| # | 接口 | 当前状态 | 决策 | 理由 |
|---|------|----------|------|------|
| 1 | `GET /api/v1/customer-service/tickets/{id}` — 工单详情 | ❌ 未实现 | **Phase 1** | 工单闭环必需：客服需要查询单个工单详情，assign/resolve/close 前必须能查询。运营人员需要查看工单处理历史。 |
| 2 | `GET /api/v1/customer-service/sessions/{id}` — 会话信息 | ❌ 未实现 | **Phase 2** | 生产一期会话仅通过 webhook 消息触发转人工，会话查询不是工单闭环必需路径。Phase 2 再实现。 |
| 3 | `GET /api/v1/customer-service/sessions/{id}/messages` — 会话消息历史 | ❌ 未实现 | **Phase 2** | 同上，会话消息历史对工单闭环非必需。Phase 2 实现，支持客服查看用户说了什么。 |
| 4 | `POST /api/v1/customer-service/sessions/{id}/feedback` — 反馈提交 | ❌ 未实现 | **Phase 1** | 工单闭环必需：客服解决工单后需要收集用户满意度反馈，记录在审计日志中。真实持久化要求。 |
| 5 | `POST /api/v1/customer-service/sessions/{id}/handoff` — 手动转人工 | ❌ 未实现（仅 webhook 触发） | **Phase 1** | 工单闭环必需：当前只有 webhook 意图触发自动转人工，但没有显式的手动转人工 API。客服无法主动为用户创建工单。**P0 阻断项**。 |

**决策说明 1-5：**
- 已有 `GET /tickets`（列表），但缺少 `GET /tickets/{id}`（详情），客服无法查看工单详情就无法处理工单。
- 会话查询与会话消息历史是运营视角功能，不是工单闭环核心链路，Phase 2 再做。
- 手动转人工 handoff 是紧急需求（用户说"转人工"但系统无法识别），Phase 1 必须实现。
- 反馈提交是工单解决的闭环动作，Phase 1 必须实现。

### 3.2 知识库接口（全系 7 个）

| # | 接口 | 当前状态 | 决策 | 理由 |
|---|------|----------|------|------|
| 6 | `GET /api/v1/customer-service/kb` — 列表知识库条目 | ❌ 未实现 | **Phase 2** | 知识库运营/RAG 相关，属于 Phase 2 范围。生产一期的 RAG 检索依赖预置知识库，不需要管理接口。 |
| 7 | `POST /api/v1/customer-service/kb` — 创建条目 | ❌ 未实现 | **Phase 2** | 同上 |
| 8 | `GET /api/v1/customer-service/kb/{id}` — 获取条目 | ❌ 未实现 | **Phase 2** | 同上 |
| 9 | `PUT /api/v1/customer-service/kb/{id}` — 更新条目 | ❌ 未实现 | **Phase 2** | 同上 |
| 10 | `DELETE /api/v1/customer-service/kb/{id}` — 删除条目 | ❌ 未实现 | **Phase 2** | 同上 |
| 11 | `POST /api/v1/customer-service/kb/{id}/publish` — 发布条目 | ❌ 未实现 | **Phase 2** | 同上 |
| 12 | `POST /api/v1/customer-service/kb/search` — 检索知识库 | ❌ 未实现 | **Phase 2** | 同上 |

**决策说明 6-12：**
知识库 CRUD/发布/审核属于 Phase 2 的「RAG/知识库运营」范围。生产一期仅需要预置知识库内容能正常检索，不需要管理接口。

### 3.3 运营后台接口

| # | 接口 | 当前状态 | 决策 | 理由 |
|---|------|----------|------|------|
| 13 | `GET /api/v1/customer-service/admin/dashboard` — 运营大盘 | ❌ 未实现 | **Phase 2** | 属于 Phase 2「运营后台」范围。生产一期可先通过 `GET /tickets` 和数据库查询实现最小监控。 |
| 14 | `GET /api/v1/customer-service/admin/handoff-reasons` — 转人工统计 | ❌ 未实现 | **Phase 2** | 同上，运营后台统计功能，Phase 2 再做。 |
| 15 | `POST /api/v1/customer-service/admin/feedback-review` — 质检提交 | ❌ 未实现 | **Phase 2** | 同上，运营后台质检功能，Phase 2 再做。 |

**决策说明 13-15：**
运营后台属于 Phase 2 范围。生产一期不实现，不阻断上线。

### 3.4 工单统计接口

| # | 接口 | 当前状态 | 决策 | 理由 |
|---|------|----------|------|------|
| 16 | `GET /api/v1/customer-service/tickets/stats` — 工单统计 | 🔄 实现中 | **Phase 1** | 可观测/灰度可回滚必需：灰度阶段需要监控转人工率、工单创建量等指标。运营人员需要实时统计数据。 |
| 17 | 速率限制（请求频率控制） | 🔄 实现中 | **Phase 1** | 防止接口滥用，保护服务稳定性；`CS_SES_4002` 错误码对应实现。 |

**决策说明 16：**
工单统计是生产一期可观测能力的最小子集，必须实现以便在灰度阶段监控核心 SLA 指标。

---

## 4. 错误码漂移决策

### 4.1 CS_TICKET_4091 vs CS_TKT_4002 不一致

| 文档定义 | 代码实际 | 决策 |
|----------|----------|------|
| `CS_TKT_4002`（工单已被分配） | `CS_TICKET_4091` | **统一为文档值 `CS_TKT_4002`** |

**理由**：`CS_TKT_4002` 更符合错误码命名规范（业务前缀_资源_序号）。代码中散落的 `CS_TICKET_4091` 需要统一改为 `CS_TKT_4002`。

**修复方案**：
- 在 `internal/domain/error/` 包中统一定义错误码常量
- 所有 handler 引用统一常量，不在业务代码中 hardcode 错误码
- 废弃 `CS_TICKET_4091`，统一使用 `CS_TKT_4002`

### 4.2 未使用错误码归档

以下错误码在 INTERFACE.md 中定义，但代码中无触发路径，决策如下：

| 错误码 | 状态 | 决策 |
|--------|------|------|
| `CS_SES_4001`（会话不存在） | 未使用 | **归档 Phase 2**：Phase 1 没有 GET session/{id} 接口，无法触发此错误 |
| `CS_SES_4002`（消息频率过高） | 未实现 | **归档 Phase 2**：速率限制未实现 |
| `CS_SES_4003`（身份校验已锁定） | 未实现 | **归档 Phase 2**：身份核验未实现 |
| `CS_IDT_4001`（身份信息不匹配） | 未实现 | **归档 Phase 2**：身份核验未实现 |
| `CS_IDT_4002`（验证码错误） | 未实现 | **归档 Phase 2**：身份核验未实现 |
| `CS_KB_4001`（知识库条目不存在） | 未实现 | **归档 Phase 2**：KB 接口 Phase 2 实现 |
| `CS_KB_4002`（条目名称已存在） | 未实现 | **归档 Phase 2**：KB 接口 Phase 2 实现 |
| `CS_LLM_5001`（LLM 服务不可用） | 未实现 | **归档 Phase 2**：大模型 failover 未实现 |
| `CS_LLM_5002`（LLM 超时） | 未实现 | **归档 Phase 2**：大模型 failover 未实现 |
| `CS_AUTH_4001`（越权访问） | 未实现 | **归档 Phase 2**：RBAC 未实现 |

**决策说明**：
这些错误码是 Phase 2 功能的占位符。Phase 1 不实现这些功能，也就不需要这些错误码。Phase 2 实现时直接从 `internal/domain/error/` 包中启用。

---

## 5. Phase 1 真实范围总结

### 5.1 需实现的接口（共 6 个）

| # | 接口 | 优先级 | 阻断原因 |
|---|------|--------|----------|
| P1-A | `GET /api/v1/customer-service/tickets/{id}` | **P0** | 工单闭环必需，客服需要查看详情才能处理 |
| P1-B | `POST /api/v1/customer-service/sessions/{id}/handoff` | **P0** | 手动转人工必需，当前只能 webhook 触发 |
| P1-C | `POST /api/v1/customer-service/sessions/{id}/feedback` | **P0** | 工单解决后反馈收集，工单闭环必需 |
| P1-D | `GET /api/v1/customer-service/tickets/stats` | **P1** | 可观测必需，灰度阶段监控 SLA |
| P1-E | 错误码统一（`CS_TKT_4002`） | **P0** | 文档与代码一致性要求 |

### 5.2 Phase 2 归档（16 个接口 + 10 个错误码）

| 类别 | 接口/错误码数 | 说明 |
|------|--------------|------|
| 知识库 KB 全系 | 7 接口 | Phase 2 RAG/知识库运营 |
| 运营后台 admin | 3 接口 | Phase 2 运营后台 |
| 会话管理（查询类） | 2 接口 | Phase 2 再实现 |
| 未使用错误码 | 10 个 | Phase 2 功能占位符 |

### 5.3 废弃（0 个）

无接口从 INTERFACE.md 中永久删除，均为 Phase 2 推迟。

---

## 6. Phase 1 完成标准

以下测试必须 100% 通过才能上线：

### P0 必须通过（阻断上线）

| 测试项 | 说明 |
|--------|------|
| 工单详情查询 | `GET /tickets/{id}` 返回正确工单，404 时返回 `CS_TKT_4001` |
| 手动转人工 | `POST /sessions/{id}/handoff` 创建工单，状态=open |
| 反馈提交 | `POST /sessions/{id}/feedback` 写入反馈记录 |
| 错误码一致性 | 所有错误码使用统一常量，无 hardcode |
| 文档更新 | INTERFACE.md 中标注 Phase 1/Phase 2 接口 |

### P1 必须通过（强烈建议）

| 测试项 | 说明 |
|--------|------|
| 工单统计 | `GET /tickets/stats` 返回今日/本周工单数据 |
| AC-07/08 E2E | 转人工后工单内容完整性（session_id/user_id/channel/priority） |
| 审计完整性 | feedback 提交写入审计日志 |

---

## 7. 门禁更新

### PRODUCTION_EXECUTION_PLAN.md 补充

在 Gate B（允许联调前）中增加：

```
- [x] Phase 1 真实范围已定义（6 个接口 + 错误码统一）
- [x] 16+ 漂移接口已明确 Phase 1/Phase 2/废弃分类
- [ ] GET /tickets/{id} 已实现并测试通过
- [ ] POST /sessions/{id}/handoff 已实现并测试通过
- [ ] POST /sessions/{id}/feedback 已实现并测试通过
- [ ] GET /tickets/stats 已实现并测试通过
- [ ] 错误码全局统一（无 hardcode 散落）
```

---

## 8. INTERFACE.md 更新标注

所有 Phase 1 接口在 INTERFACE.md 中标注 ✅；Phase 2 接口标注 🔲 Phase 2。

---

## 9. 版本信息

- **本文档版本**：v1.0
- **生效日期**：2026-04-30
- **下次审查**：Phase 1 接口实现完成后