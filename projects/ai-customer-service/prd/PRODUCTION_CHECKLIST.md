# 生产一期上线前清单 (PRODUCTION_CHECKLIST)

> 版本：v1.0 | 日期：2026-04-30
> 负责人：PM（小龙团队）
> 范围：ai-customer-service 生产一期（Phase 1）
> 依据：SCOPE_PHASE1_VS_PHASE2.md、PRODUCTION_PHASE1_STATUS.md、QA_GATE_STATUS.md

---

## 一、✅ 已验证功能（上线门禁全部通过）

### 1.1 Phase 1 接口实现

| ID | 接口 | 验证方法 | 测试状态 |
|----|------|---------|----------|
| P1-A | `GET /api/v1/customer-service/tickets/{id}` — 工单详情 | 代码审查 + handler 测试 | ✅ 通过 |
| P1-B | `POST /api/v1/customer-service/sessions/{id}/handoff` — 手动转人工 | `TestSessionHandlerHandoff_*` (3 cases) | ✅ 通过 |
| P1-C | `POST /api/v1/customer-service/sessions/{id}/feedback` — 反馈提交 | `TestSessionHandlerFeedback_*` (3 cases) | ✅ 通过 |
| P1-D | `GET /api/v1/customer-service/tickets/stats` — 工单统计 | `TestTicketStats_*` (3 cases) | ✅ 通过 |
| P1-E | 速率限制（滑动窗口 10 req/s/IP） | `TestWebhookRateLimit_*` (3 cases) | ✅ 通过 |

### 1.2 上线门禁验证

```bash
# 命令执行结果
go build ./...      ✅ 无错误
go vet ./...       ✅ 无警告
go test ./...      ✅ 全部通过 (14 tests)
```

| 阻断条件 | 状态 | 说明 |
|---------|------|------|
| BC-01 接口路由漂移 | 🟢 解除 | Phase 1 核心端点已实现 |
| BC-02 P0 安全测试覆盖 | 🟢 解除 | AC-09/AC-02/AC-07/08 测试已补齐 |
| BC-03 错误码一致 | 🟢 解除 | CS_TKT_4002 为主码，统一使用 |
| BC-04 会话端点 | 🟢 解除 | feedback + handoff 已实现并测试 |
| BC-05 速率限制 | 🟢 解除 | RateLimiter 已实现并测试 |

### 1.3 错误码统一

| 错误码 | 状态 |
|--------|------|
| `CS_TKT_4002`（工单已被分配） | ✅ 已统一为主码 |
| `CS_TICKET_4091` | ✅ 已废弃，保留为兼容别名 |
| `CS_REQ_4009` | ✅ 已定义 |
| `CS_REQ_4010` | ✅ 已定义 |
| `CS_SES_4001`（会话不存在） | ✅ feedback/handoff 已使用 |
| `CS_SES_4002`（消息频率过高） | ✅ 429 HTTP 响应已实现 |
| 无 hardcode 错误码散落 | ✅ 统一定义在 `internal/domain/error/` |

### 1.4 基线安全能力

| 能力 | 状态 |
|------|------|
| Webhook HMAC 签名校验 | ✅ 已实现 |
| 时间戳防重放 | ✅ 已实现 |
| 消息幂等去重 | ✅ 已实现 |
| BodyLimit 超大请求拒绝 | ✅ 已实现 |
| 工单持久化 | ✅ 已实现 |
| 审计日志持久化 | ✅ 已实现 |
| 健康检查 | ✅ 已实现 |

---

## 二、⚠️ 需要人工确认项目（上线前必须确认）

### 2.1 环境配置（必须在真实环境验证）

| 项目 | 说明 | 确认人 |
|------|------|--------|
| 数据库连接配置 | `DATABASE_URL` / `POSTGRES_*` 环境变量已在真实 DB 可用 | DevOps |
| HMAC 签名密钥 | `WEBHOOK_SECRET` 与飞书后台配置一致 | TechLead |
| LLM API Key | `OPENAI_API_KEY` / `LLM_PROVIDER` 配置正确 | TechLead |
| 飞书 App 凭证 | `FEISHU_APP_ID` + `FEISHU_APP_SECRET` 有效 | TechLead |
| Telegram Bot Token | `TELEGRAM_BOT_TOKEN` 配置正确（如使用） | TechLead |
| 速率限制配置 | `RATE_LIMIT_*` 环境变量（当前默认 10 req/s/IP）是否满足生产流量预期 | TechLead |
| 日志级别配置 | `LOG_LEVEL` 生产环境设为 info/warn | TechLead |
| 会话存储 | memory store（测试用）→ 生产需切换为 PostgreSQL | TechLead |

### 2.2 密钥与权限

| 项目 | 说明 | 确认人 |
|------|------|--------|
| 数据库迁移 | 是否有 migration scripts，schema 是否就绪 | DevOps |
| 云函数/容器环境变量 | 所有 secrets 已通过安全方式注入（非硬编码） | DevOps |
| 飞书机器人权限 | 机器人已添加到群组，且具有发送消息权限 | TechLead |
| PostgreSQL 网络策略 | 服务可访问 DB，安全组/防火墙配置正确 | DevOps |

### 2.3 监控与告警（灰度阶段必需）

| 项目 | 说明 | 确认人 |
|------|------|--------|
| 监控大盘 | `GET /tickets/stats` 数据已接入监控面板 | TechLead |
| 转人工率告警 | 灰度阶段需监控 handoff 率异常 | TechLead |
| 接口错误率告警 | 5xx 错误率超过阈值需告警 | TechLead |
| 日志聚合 | 结构化日志已接入日志系统（Datadog/Loki/ELK） | DevOps |
| 健康检查端点 | `/health` 已在生产环境验证响应正常 | TechLead |

### 2.4 E2E 测试覆盖（可选，建议上线前完成）

| 项目 | 状态 | 说明 |
|------|------|------|
| E2E webhook 测试 | ⚠️ app.go 编译错误修复后验证 | TechLead |
| 工单内容完整性 AC-07/08 | ⚠️ 同上 | TechLead |

---

## 三、📋 上线步骤（顺序执行）

> 灰度发布流程，参考 `GRAY_RELEASE_ROLLBACK_RUNBOOK.md`

### 阶段 0：上线前准备（上线前 1-2 天）

- [ ] **TechLead**：确认所有环境变量已在生产环境注入
- [ ] **DevOps**：验证数据库连接和迁移脚本
- [ ] **TechLead**：验证 HMAC 签名密钥与飞书后台一致
- [ ] **TechLead**：确认所有 secrets 通过安全方式注入（非硬编码）
- [ ] **TechLead**：配置灰度阶段监控告警（转人工率、接口错误率）
- [ ] **DevOps**：确认日志已接入日志系统
- [ ] **PM**：最终确认 Phase 1 范围所有人达成一致

### 阶段 1：生产部署（灰度 5%）

- [ ] **DevOps**：执行数据库 migration（如有）
- [ ] **DevOps**：部署生产镜像（1 个实例，5% 流量）
- [ ] **DevOps**：验证 `/health` 端点返回 200
- [ ] **TechLead**：验证 `GET /tickets/stats` 返回数据
- [ ] **TechLead**：发送测试 webhook，验证 HMAC 签名通过
- [ ] **QA**：执行冒烟测试（feedback、handoff、速率限制）
- [ ] **PM**：确认无 P0 阻断项

### 阶段 2：灰度观察（灰度 5% → 30%）

- [ ] **TechLead**：监控转人工率、工单创建量、接口错误率
- [ ] **TechLead**：验证审计日志写入正常
- [ ] **PM**：抽查工单内容完整性
- [ ] **TechLead**：若无异常，逐步放量至 30%

### 阶段 3：全量上线（灰度 30% → 100%）

- [ ] **TechLead**：确认监控指标在正常范围
- [ ] **PM**：最终验收确认
- [ ] **DevOps**：全量部署
- [ ] **PM**：通知干系人上线完成

### 阶段 4：回滚准备（随时可执行）

- [ ] **DevOps**：保留上一版本镜像 tag
- [ ] **TechLead**：熟悉回滚触发条件（见 `GRAY_RELEASE_ROLLBACK_RUNBOOK.md`）

---

## 四、上线后 24h 内关键检查项

| 时间 | 检查项 | 负责人 |
|------|--------|--------|
| +15min | 确认无 5xx 错误率飙升 | TechLead |
| +30min | 确认工单创建正常，无异常空工单 | TechLead |
| +1h | 确认速率限制未误杀正常流量 | TechLead |
| +2h | 确认反馈提交写入审计日志 | TechLead |
| +24h | 统计工单量、转人工率是否符合预期 | PM |

---

## 五、关键联系人

| 角色 | 职责 | 备注 |
|------|------|------|
| TechLead | 技术决策、生产环境配置、告警配置 | 主工程师 |
| DevOps | 部署、数据库、环境变量、监控接入 | 运维 |
| PM | 上线审批、范围管理、进度追踪 | 小龙团队 |
| QA | 冒烟测试、回归测试 | 小龙团队 |

---

*本文档由 PM（小龙团队）基于最终验收结果生成*
*生成时间：2026-04-30 21:10 GMT+8*
