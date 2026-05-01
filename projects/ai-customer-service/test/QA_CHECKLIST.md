# AI-Customer-Service 生产一期 QA 检查清单

> 生成时间：2026-04-30
> 项目路径：/home/long/project/立交桥/projects/ai-customer-service
> 覆盖范围：文档-实现一致性 · 威胁建模 · AC/失败路径/安全/性能矩阵 · 灰度回滚 · 漂移检测 · 阻断条件

---

## 一、文档-实现一致性检查清单

### 1.1 接口路由一致性

| # | 文档接口（INTERFACE.md） | 代码实现 | 路由文件 | 状态 |
|---|--------------------------|----------|----------|------|
| 1 | `POST /api/v1/customer-service/webhook/{channel}` | ✅ 已实现 | `router.go` → `HandleChannel` | **一致** |
| 2 | `POST /api/v1/customer-service/webhook`（统一入口） | ✅ 已实现 | `router.go` → `Handle` | **一致** |
| 3 | `GET /api/v1/customer-service/tickets` | ✅ 已实现（List 方法） | `router.go` → `/tickets` | **一致** |
| 4 | `GET /api/v1/customer-service/tickets/{id}` | ❌ **未实现** | 无 | **漂移** |
| 5 | `POST /api/v1/customer-service/tickets/{id}/assign` | ✅ 已实现 | `router.go` → `/tickets/*/assign` | **一致** |
| 6 | `POST /api/v1/customer-service/tickets/{id}/resolve` | ✅ 已实现 | `router.go` → `/tickets/*/resolve` | **一致** |
| 7 | `POST /api/v1/customer-service/tickets/{id}/close` | ✅ 已实现 | `router.go` → `/tickets/*/close` | **一致** |
| 8 | `GET /api/v1/customer-service/sessions/{id}` | ❌ **未实现** | 无 | **严重漂移** |
| 9 | `GET /api/v1/customer-service/sessions/{id}/messages` | ❌ **未实现** | 无 | **严重漂移** |
| 10 | `POST /api/v1/customer-service/sessions/{id}/feedback` | ❌ **未实现** | 无 | **严重漂移** |
| 11 | `POST /api/v1/customer-service/sessions/{id}/handoff` | ❌ **未实现**（仅通过 webhook 触发） | 无 | **严重漂移** |
| 12 | `GET /api/v1/customer-service/kb` | ❌ **未实现** | 无 | **漂移** |
| 13 | `POST /api/v1/customer-service/kb` | ❌ **未实现** | 无 | **漂移** |
| 14 | `GET /api/v1/customer-service/kb/{id}` | ❌ **未实现** | 无 | **漂移** |
| 15 | `PUT /api/v1/customer-service/kb/{id}` | ❌ **未实现** | 无 | **漂移** |
| 16 | `DELETE /api/v1/customer-service/kb/{id}` | ❌ **未实现** | 无 | **漂移** |
| 17 | `POST /api/v1/customer-service/kb/{id}/publish` | ❌ **未实现** | 无 | **漂移** |
| 18 | `POST /api/v1/customer-service/kb/search` | ❌ **未实现** | 无 | **漂移** |
| 19 | `GET /api/v1/customer-service/admin/dashboard` | ❌ **未实现** | 无 | **漂移** |
| 20 | `GET /api/v1/customer-service/admin/handoff-reasons` | ❌ **未实现** | 无 | **漂移** |
| 21 | `POST /api/v1/customer-service/admin/feedback-review` | ❌ **未实现** | 无 | **漂移** |
| 22 | `GET /api/v1/customer-service/tickets/stats` | ❌ **未实现** | 无 | **漂移** |

### 1.2 错误码一致性

| # | 文档错误码 | 代码实际错误码 | 状态 |
|---|-----------|---------------|------|
| 1 | `CS_SES_4001`（会话不存在） | 代码中无对应错误码（会话端点未实现） | **未使用** |
| 2 | `CS_SES_4002`（消息频率过高） | 代码中无对应错误码（速率限制未实现） | **未使用** |
| 3 | `CS_SES_4003`（身份校验已锁定） | 代码中无对应错误码 | **未使用** |
| 4 | `CS_IDT_4001`（身份信息不匹配） | 代码中无对应错误码 | **未使用** |
| 5 | `CS_IDT_4002`（验证码错误） | 代码中无对应错误码 | **未使用** |
| 6 | `CS_TKT_4001`（工单不存在） | 代码无 GET ticket/{id}，无可触发路径 | **未使用** |
| 7 | `CS_TKT_4002`（工单已被分配） | `CS_TICKET_4091`（不等于文档） | **漂移** |
| 8 | `CS_KB_4001`（知识库条目不存在） | 知识库端点未实现 | **未使用** |
| 9 | `CS_KB_4002`（条目名称已存在） | 知识库端点未实现 | **未使用** |
| 10 | `CS_LLM_5001`（LLM 服务不可用） | 代码中无对应错误码 | **未使用** |
| 11 | `CS_LLM_5002`（LLM 超时） | 代码中无对应错误码 | **未使用** |
| 12 | `CS_AUTH_4001`（越权访问） | 代码中无对应错误码 | **未使用** |

### 1.3 业务逻辑一致性

| # | 文档要求 | 代码实现 | 一致性 |
|---|---------|---------|--------|
| 1 | 转人工后生成 P1 工单（敏感意图） | `handoff/service.go`：意图含 `NeedsHuman` 或 `Sensitive` → `ShouldHandoff=true`，`Priority=P1` | ✅ **一致** |
| 2 | 低置信度（<0.60）转人工 | `handoff/service.go`：`turnCount>=5 && confidence<0.7` → P2 工单（文档要求<0.60，代码使用<0.7） | ⚠️ **轻微漂移** |
| 3 | 对话上下文保留最近 6 轮 | `dialog/service.go`：超过 6 条时截断（`len(sess.Context)>6`） | ✅ **一致** |
| 4 | 消息幂等去重 | `DedupRepository.TryRecord` 实现 | ✅ **一致** |
| 5 | HMAC 签名校验 | `webhook_security.go` 实现 HMAC-SHA256 | ✅ **一致** |
| 6 | 时间戳防重放 | `webhook_security.go` 有 MaxSkew 检查，无持久化 nonce | ⚠️ **部分一致** |
| 7 | content > 2000 字截断 | `webhook_handler.go` 返回 400（不截断） | ⚠️ **漂移**（文档要求截断，代码拒绝） |

---

## 二、威胁建模到测试映射清单

### 2.1 威胁分类与测试覆盖

| 威胁类别 | 威胁项 | 测试函数 | 覆盖状态 | 说明 |
|---------|--------|---------|---------|------|
| **T1: Webhook 签名绕过** | T1.1: 无签名请求 | `webhook_handler_test.go:TestWebhookSecurityRejectsMissingSignature` | ✅ **已覆盖** | |
| | T1.2: 伪造签名 | 无测试 | ❌ **未覆盖** | |
| | T1.3: 时间戳重放（旧时间戳 within skew） | 无测试 | ❌ **未覆盖** | |
| | T1.4: 篡改 body 后签名不匹配 | 无测试 | ❌ **未覆盖** | |
| **T2: 消息注入/重放** | T2.1: 重复 message_id 去重 | `dialog_service_test.go` 部分验证 | ⚠️ **部分覆盖** | dialog service 有去重，但无专门 E2E 测试 |
| | T2.2: 1 秒 10 消息频率攻击 | 无速率限制实现 | ❌ **未覆盖**（且功能不存在） |
| | T2.3: 超长消息 DoS（>2000字） | `webhook_handler_test.go:TestWebhookRejectsLongContent` | ✅ **已覆盖** | |
| **T3: 意图注入/Prompt Injection** | T3.1: 恶意指令注入 | 无测试 | ❌ **未覆盖** | |
| | T3.2: 绕过关键词检测 | 无测试 | ❌ **未覆盖** | |
| **T4: 越权访问** | T4.1: 未授权用户访问他人工单 | 无 RBAC 测试 | ❌ **未覆盖** | |
| | T4.2: 跨用户会话隔离 | 无测试 | ❌ **未覆盖** | |
| | T4.3: 攻击者写操作返回 403 | 无测试 | ❌ **未覆盖** | |
| **T5: 审计绕过** | T5.1: 签名失败不记审计 | `webhook_handler_test.go:TestWebhookSecurityRejectsMissingSignature` 有审计检查 | ✅ **已覆盖** | |
| | T5.2: 非法 body 不记审计 | `webhook_handler_test.go:TestWebhookRejectsAndAuditsMissingFields` | ✅ **已覆盖** | |
| | T5.3: 工单状态变更审计 | `ticket_handler_test.go:TestTicketHandlerAssignAuditsStateChange` | ✅ **已覆盖** | |
| **T6: 错误信息泄露** | T6.1: 内部错误堆栈泄露 | 无测试 | ❌ **未覆盖** | |
| | T6.2: LLM 内部错误信息泄露 | 无测试 | ❌ **未覆盖** | |
| **T7: 适配层失控** | T7.1: NewAPI/Sub2API 消息格式异常 | 无测试 | ❌ **未覆盖** | |
| | T7.2: 渠道消息格式不匹配 | 无测试 | ❌ **未覆盖** | |

---

## 三、AC / 失败路径 / 安全 / 性能 / 灾备测试矩阵

### 3.1 AC 测试覆盖矩阵

| AC | 描述 | 测试函数 | 文件 | 覆盖状态 | 缺口说明 |
|----|------|---------|------|---------|---------|
| AC-01 | 多渠道消息接入 | `TestWebhook_MainPath`, `TestWebhook_InvalidPayload`, `TestWebhook_SignedRequestPath` | `webhook_e2e_test.go` | ⚠️ **部分覆盖** | 仅 widget 渠道测试；Telegram/Discord/微信无测试 |
| AC-02 | 意图识别与知识库回复 | `TestDialogService_Process` | `dialog_service_test.go` | ⚠️ **部分覆盖** | 仅测试"查询额度"一条；无置信度边界、无 RAG 质量验证 |
| AC-03 | 用户数据只读查询 | 无测试 | - | ❌ **未覆盖** | supply-api 集成未实现 |
| AC-04 | 多轮对话与上下文保持 | 无专门测试 | - | ❌ **未覆盖** | 仅 dialog service 内隐验证，无独立测试 |
| AC-05 | 身份核验 | 无测试 | - | ❌ **未覆盖** | 身份核验功能未实现 |
| AC-06 | 大模型故障 Failover | 无测试 | - | ❌ **未覆盖** | 故障注入测试不存在 |
| AC-07 | 兜底回复与工单生成 | `TestWebhook_HandoffPath` | `webhook_e2e_test.go` | ⚠️ **部分覆盖** | 仅验证返回 200，未验证工单内容 |
| AC-08 | 明确转人工 | `TestWebhook_HandoffPath` | `webhook_e2e_test.go` | ⚠️ **部分覆盖** | 仅触发意图，未验证工单生成内容 |
| AC-09 | 敏感意图自动转人工 | 无专门测试 | - | ❌ **未覆盖** | 无测试"退款"/"数据泄露"→P1 工单 |
| AC-10 | 工单后台分配与处理 | `TestTicketHandlerAssignAuditsStateChange`, `TestTicketHandlerResolveAuditsStateChange`, `TestTicketHandlerCloseRequiresResolution`, `TestTicketHandlerAssignPassesActorAndSourceIP`, `TestTicketHandlerClosePassesActorAndSourceIP` | `ticket_handler_test.go` | ✅ **已覆盖** | 测试较为完整 |
| AC-11 | 知识库条目管理 | 无测试 | - | ❌ **未覆盖** | 知识库端点未实现 |
| AC-12 | 对话埋点与监控 | 无测试 | - | ❌ **未覆盖** | metrics/tracing 未实现 |
| AC-13 | 权限边界 | 无测试 | - | ❌ **未覆盖** | RBAC 未实现 |

### 3.2 边缘/失败路径（EC）覆盖矩阵

| EC | 场景 | 测试函数 | 覆盖状态 | 缺口说明 |
|----|------|---------|---------|---------|
| EC-01 | 超长消息（>2000字） | `TestWebhookRejectsLongContent` | ✅ **已覆盖** | |
| EC-02 | 1秒10消息频率限制 | 无测试 | ❌ **未覆盖**（且功能不存在） | |
| EC-03 | 知识库无结果+低置信度 | 无测试 | ❌ **未覆盖** | |
| EC-04 | API Key 前缀匹配多账户 | 无测试 | ❌ **未覆盖** | |
| EC-05 | supply-api 超时 >3s | 无测试 | ❌ **未覆盖** | |
| EC-06 | 多渠道同时会话隔离 | 无测试 | ❌ **未覆盖** | |
| EC-07 | 用户发送图片/语音 | 无测试 | ❌ **未覆盖** | |
| EC-08 | 系统维护窗口期 | 无测试 | ❌ **未覆盖** | |
| EC-09 | 客服队列满员 | 无测试 | ❌ **未覆盖** | |
| EC-10 | 数据库连接池耗尽 | 无测试 | ❌ **未覆盖** | |

### 3.3 安全测试矩阵

| 安全测试项 | 测试函数 | 覆盖状态 | 说明 |
|-----------|---------|---------|------|
| Webhook HMAC 签名验证 | `TestWebhookSecurityRejectsMissingSignature`, `TestWebhookSecurityAcceptsSignedRequest` | ✅ **已覆盖** | |
| JSON schema/字段校验 | `TestWebhookRejectsUnknownFields`, `TestWebhookRejectsAndAuditsMissingFields` | ✅ **已覆盖** | |
| 请求体大小限制 | `TestWebhookRejectsLongContent` | ✅ **已覆盖** | |
| 幂等去重 | `dialog_service_test.go` 内隐验证 | ⚠️ **部分覆盖** | 无专门去重测试 |
| 速率限制 | 无测试 | ❌ **未覆盖** | 功能未实现 |
| RBAC 权限边界 | 无测试 | ❌ **未覆盖** | 功能未实现 |
| 审计日志完整性 | `TestWebhookRejectsAndAuditsMissingFields`, `ticket_handler_test.go` assign/resolve/close | ✅ **已覆盖** | 成功路径和 webhook 拒绝路径有覆盖 |
| 错误信息脱敏 | 无测试 | ❌ **未覆盖** | |
| Prompt Injection | 无测试 | ❌ **未覆盖** | |
| 跨用户会话隔离 | 无测试 | ❌ **未覆盖** | |

### 3.4 性能测试矩阵

| 性能指标 | 文档目标 | 测试函数 | 覆盖状态 |
|---------|---------|---------|---------|
| 对话首次响应 P99 < 5s | <5s | 无测试 | ❌ **未覆盖** |
| 意图识别 P99 < 5s | <5s | 无测试 | ❌ **未覆盖** |
| Token 查询 P99 < 3s | <3s | 无测试 | ❌ **未覆盖** |
| 工单看板加载 < 2s | <2s | 无测试 | ❌ **未覆盖** |
| 向量检索 P99 < 200ms | <200ms | 无测试 | ❌ **未覆盖** |
| 模型 Failover 切换 < 5s | <5s | 无测试 | ❌ **未覆盖** |
| 会话历史加载 < 1s | <1s | 无测试 | ❌ **未覆盖** |

### 3.5 灾备/恢复测试矩阵

| 灾备场景 | 测试函数 | 覆盖状态 |
|---------|---------|---------|
| 主模型 500 切换备用 | 无测试 | ❌ **未覆盖** |
| 主模型超时切换备用 | 无测试 | ❌ **未覆盖** |
| 双模型均故障 → 兜底回复 | 无测试 | ❌ **未覆盖** |
| PostgreSQL 故障 → 降级 | 无测试 | ❌ **未覆盖** |
| Redis 故障 → 降级 | 无测试 | ❌ **未覆盖** |
| 备份恢复演练 | 无测试 | ❌ **未覆盖** |

---

## 四、灰度与回滚演练检查表

### 4.1 灰度发布门禁

| # | 检查项 | 当前状态 | 是否可执行 | 备注 |
|---|--------|---------|-----------|------|
| 1 | 所有 AC 测试用例 100% 通过 | ❌ | 不可执行 | AC-03/04/05/06/09/12/13 完全无测试 |
| 2 | 单元测试覆盖率达标（domain ≥70%, service/handler ≥80%） | ❌ | 不可执行 | 无覆盖率报告 |
| 3 | 意图识别准确率测试（20 个常见问题，正确率 ≥85%） | ❌ | 不可执行 | 无准确率测试 |
| 4 | RAG 检索质量测试（20 个查询，Recall@3 ≥80%） | ❌ | 不可执行 | 无 RAG 质量测试 |
| 5 | 模型 Failover 演练（主/备故障场景全部通过） | ❌ | 不可执行 | 无故障注入测试 |
| 6 | 安全渗透测试（权限越界、Prompt Injection） | ❌ | 不可执行 | 无渗透测试 |
| 7 | 性能基准测试通过 | ❌ | 不可执行 | 无性能测试 |
| 8 | OpenAPI 文档与实现一致 | ❌ | 不可执行 | 接口漂移 16+ 项 |

### 4.2 回滚演练检查

| # | 回滚场景 | 检查步骤 | 当前状态 |
|---|---------|---------|---------|
| 1 | 回滚 webhook 路由变更 | 1. 重启服务 2. POST /webhook → 200 3. 检查审计日志 | ⚠️ 部分可执行 |
| 2 | 回滚工单 API 变更 | 1. 分配工单 2. 检查 audit_store 写入 3. GET /tickets → 列表正常 | ⚠️ 部分可执行（无 GET ticket/{id}） |
| 3 | 数据库 migration 回滚 | 1. 检查 migration 脚本 2. 验证 cs_* 表结构 | ⚠️ 有 migration 脚本但无回滚测试 |
| 4 | 配置变更回滚 | 1. 修改 AI_CS_WEBHOOK_SECRET 2. 验证签名校验 3. 回滚环境变量 4. 验证 | ⚠️ 配置可改但无自动化回滚测试 |
| 5 | 独立运行 → 集成运行切换 | 1. 独立模式启动 2. 检查 /actuator/health/live, /ready 3. 切换集成模式 4. 路由正常 | ❌ 集成模式未实现 |

---

## 五、实施漂移检测点

### 5.1 自动化漂移检测（建议 CI/CD 集成）

| # | 检测点 | 检测方法 | 当前状态 | 优先级 |
|---|--------|---------|---------|--------|
| D-01 | 接口路由漂移 | 启动服务 + OpenAPI 扫描 + 与 INTERFACE.md 对比 | ⚠️ 16+ 项漂移 | **P0** |
| D-02 | 错误码一致性 | 扫描所有 error code 与文档定义对比 | ⚠️ 多处漂移 | **P0** |
| D-03 | 测试覆盖率 | `go test -cover` 验证 domain/service/handler 覆盖率 | ❌ 未集成 | **P1** |
| D-04 | 审计事件完整性 | 扫描代码中 `audit.Add` 调用点与 TEST_DESIGN.md 审计要求对比 | ⚠️ 安全拒绝审计已有，但工单状态变更审计在 mock 中，真实实现待验证 | **P1** |
| D-05 | 意图识别关键词覆盖 | 扫描 intent/service.go 的关键词与 TEST_DESIGN.md AC-02 场景对比 | ⚠️ 意图识别硬编码关键词，无外部配置 | **P1** |
| D-06 | 超时配置一致性 | 扫描代码中 hardcoded timeout 与 TEST_DESIGN.md 性能基准对比 | ⚠️ 无统一超时配置 | **P1** |
| D-07 | 健康检查依赖完整性 | 检查 `/actuator/health/ready` 的依赖检查项（当前仅 postgres） | ⚠️ 缺少 Redis/外部 API 依赖检查 | **P2** |
| D-08 | 速率限制配置 | 扫描代码确认是否有速率限制中间件 | ❌ 完全未实现 | **P2** |

### 5.2 手动漂移审计（上线前必须执行）

- [ ] 对比 `tech/INTERFACE.md` 全部 22 个端点与代码实现
- [ ] 对比 `tech/TEST_DESIGN.md` 全部 58 条测试用例与实际测试覆盖
- [ ] 审查 `internal/service/intent/service.go` 的硬编码关键词是否覆盖 AC-02 场景
- [ ] 审查错误码是否全局统一定义（非散落在 handler 中）
- [ ] 审查 webhook 幂等去重是否持久化（非仅内存）

---

## 六、上线阻断条件清单

> 以下任一条件未满足，**必须阻断上线**。

### 🔴 P0 阻断条件（必须全部解决）

| # | 阻断条件 | 当前状态 | 说明 |
|---|---------|---------|------|
| P0-01 | **工单状态流转审计完整性** | ⚠️ 部分通过 | `ticket_handler_test.go` 有测试，但真实 store 实现（`ticket_workflow.go`）的审计写入依赖待验证 |
| P0-02 | **安全拒绝事件审计完整性** | ✅ 已实现 | `webhook_handler.go` 已对所有拒绝场景写审计 |
| P0-03 | **接口路由与文档一致** | ❌ 未通过 | 16+ 接口未实现，上线后面向用户/API 的契约严重不完整 |
| P0-04 | **AC-07/AC-08 转人工工单生成完整性** | ⚠️ 部分通过 | E2E 测试仅验证返回 200，未验证工单实际内容（session_id/user_id/channel/priority） |
| P0-05 | **错误码全局统一定义** | ❌ 未通过 | 错误码散落在 handler 中，无统一错误定义；`CS_TICKET_4091` 与文档 `CS_TKT_4002` 不一致 |

### 🟡 P1 阻断条件（上线前必须解决或明确延期范围）

| # | 阻断条件 | 当前状态 | 说明 |
|---|---------|---------|------|
| P1-01 | **意图识别准确率验证** | ❌ 未通过 | 无 AC-02 准确率测试，无法证明意图识别质量 |
| P1-02 | **RAG 检索质量验证** | ❌ 未通过 | 无 RAG 质量测试，无法证明知识库检索效果 |
| P1-03 | **Failover 故障切换验证** | ❌ 未通过 | 无 AC-06 故障注入测试，无法证明灾备能力 |
| P1-04 | **RBAC 权限边界验证** | ❌ 未通过 | 无 AC-13 权限测试，无法证明跨用户隔离 |
| P1-05 | **性能基准验证** | ❌ 未通过 | 无性能测试，无法证明 P99 延迟达标 |
| P1-06 | **EC-02 速率限制** | ❌ 未实现 | 生产环境无速率限制，面临 DoS 风险 |

---

## 七、现有测试覆盖度评估

### 7.1 测试文件清单

| 文件 | 测试函数数 | 覆盖的 AC | 覆盖的威胁 |
|------|----------|---------|-----------|
| `test/e2e/webhook_e2e_test.go` | 4 | AC-01（部分）, AC-07（部分）, AC-08（部分） | T2.3 |
| `test/integration/dialog_service_test.go` | 1 | AC-02（部分） | T2.1（隐含） |
| `internal/http/handlers/webhook_handler_test.go` | 6 | AC-01（部分）, AC-12（部分） | T1.1, T2.3, T5.1, T5.2 |
| `internal/http/handlers/ticket_handler_test.go` | 5 | AC-10 | T5.3 |
| `internal/config/config_test.go` | 2 | - | - |

**总计：18 个测试函数**

### 7.2 P0 缺口专项评估

| P0 缺口 | 是否有测试捕捉 | 测试函数 | 评估结论 |
|---------|--------------|---------|---------|
| 工单状态流转审计 | ✅ 有测试 | `TestTicketHandlerAssignAuditsStateChange`, `TestTicketHandlerResolveAuditsStateChange` | **已覆盖**（但仅在 mock 层面，真实 workflow store 集成测试缺失） |
| 安全拒绝审计 | ✅ 有测试 | `TestWebhookRejectsAndAuditsMissingFields`, `TestWebhookSecurityRejectsMissingSignature` | **已覆盖** |
| AC-07/08 工单内容完整性 | ⚠️ 部分 | `TestWebhook_HandoffPath` 仅验证 HTTP 200 | **未充分覆盖** |

### 7.3 核心链路测试覆盖度

```
Webhook 接收 → 签名校验 → JSON 解析 → 去重检查 → 意图识别 → 转人工判断 → 工单生成 → 审计写入
     ✅            ✅          ✅          ✅          ⚠️          ⚠️          ⚠️          ✅
```

```
Ticket Assign → 工单状态变更 → 审计写入
     ✅              ✅           ✅
```

```
Ticket Resolve → 工单状态变更 → 审计写入
     ✅              ✅           ✅
```

---

## 八、缺口优先级排序与修复建议

### 立即修复（P0，上线前必须）

1. **补充 AC-07/08 E2E 测试**：验证转人工后工单的 `session_id`、`user_id`、`channel`、`priority` 字段完整性
2. **统一错误码**：将散落的错误码归一化为 `internal/domain/error/` 包，与文档一致
3. **补充接口路由**：至少提供 `GET tickets/{id}` 和 `POST sessions/{id}/handoff` 的最小实现，或在文档中明确说明为 Phase 2

### 尽快补齐（P1，本周内）

4. **补充 AC-02 意图识别测试**：至少测试"退款"、"数据泄露"、"人工"、"额度" 4 条核心路径
5. **补充速率限制**：实现并测试 EC-02 频率限制
6. **补充配置覆盖度测试**：验证 `AI_CS_MAX_BODY_BYTES` 等关键环境变量
7. **补充性能基准测试**：至少验证 `/actuator/health/ready` 响应时间 < 100ms

### 中期完善（P2，上线后迭代）

8. RAG 检索质量测试（AC-11）
9. Failover 故障注入测试（AC-06）
10. RBAC 权限边界测试（AC-13）
11. 监控/metrics 基础设施

---

## 九、测试执行命令

```bash
# 快速回归（当前可执行）
cd /home/long/project/立交桥/projects/ai-customer-service
go test ./test/e2e/... ./test/integration/... ./internal/http/handlers/... ./internal/config/... -v

# 覆盖率报告（需补齐）
go test ./... -coverprofile=coverage.out -covermode=atomic
go tool cover -html=coverage.out -o coverage.html

# 门禁检查（当前漂移 16+ 项，需修复后执行）
# ./scripts/qa-gate.sh  # 待实现
```

---

*本文档为机器生成，每完成一个检查项请在 PR 中标注。*
*QA 负责人签名：___________ 日期：2026-04-30*
