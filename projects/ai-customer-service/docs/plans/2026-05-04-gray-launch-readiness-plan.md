# Gray Launch Readiness Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将 `ai-customer-service` 从“代码级可运行的一期后端骨架”推进到“具备小流量灰度上线条件的生产一期服务”。

**Architecture:** 先收口“单一事实源”和部署契约，避免继续用错误文档驱动上线；再补齐后台鉴权、真实联调、可观测、灰度/回滚闭环四个生产阻断面。坚持最小范围推进：不在本轮补完整 LLM/RAG/运营后台，而是把 Phase 1 的真实范围做成可灰度交付物。

**Tech Stack:** Go 1.22, net/http, PostgreSQL, HMAC webhook security, Go testing, system/deployment docs

---

## 0. 目标范围定义

本计划的“可灰度上线”仅指：

1. `POST /api/v1/customer-service/webhook` 及 `POST /api/v1/customer-service/webhook/{channel}` 可在真实预生产环境接入。
2. 工单最小闭环可用：创建、查询、分配、解决、关闭、反馈。
3. 关键后台接口有基本鉴权和角色校验。
4. 真实 PostgreSQL、migration、审计、dedup、health、监控、回滚有证据化验证。
5. 文档、配置契约、代码实现一致。

本计划**不包含**：

1. 真实 LLM / 多供应商 failover。
2. 真实 RAG 检索和知识库运营后台。
3. Telegram / Discord / 微信专有适配器的完整产品化实现。
4. 完整客服运营后台 UI。

---

### Task 1: 收口上线口径与单一事实源

**Files:**
- Modify: `docs/PRODUCTION_LAUNCH.md`
- Modify: `docs/REVIEW_REPORT_2026-05-04.md`
- Modify: `PRODUCTION_PHASE1_STATUS.md`
- Modify: `prd/PRODUCTION_CHECKLIST.md`
- Modify: `docs/P0_P1_P2_RECTIFICATION_EXECUTION_BOARD.md`

**Step 1: 写文档一致性检查清单**

在本任务开始前，先列出 5 个必须统一的事实：

```text
1. 当前范围是 Phase 1 后端最小闭环，不是 PRD 全量范围
2. 当前未实现真实 LLM/RAG
3. 当前未实现完整运营后台
4. 当前是否允许灰度，必须以真实环境验证为准
5. 部署变量必须与 internal/config/config.go 一致
```

**Step 2: 修正过宽表述**

修改 `docs/PRODUCTION_LAUNCH.md`：
- 删除或降级“已通过全部上线门禁，可灰度发布”
- 将“LLM + RAG + 多渠道能力”改为“目标能力/非当前已交付”
- 保留当前真实已交付：webhook、ticket、audit、health、postgres

**Step 3: 回写阶段状态文档**

在 `PRODUCTION_PHASE1_STATUS.md` 和 `prd/PRODUCTION_CHECKLIST.md` 中统一三层结论：
- 代码级门禁
- 预生产门禁
- 灰度放量门禁

**Step 4: 复核并更新执行板**

将 `docs/P0_P1_P2_RECTIFICATION_EXECUTION_BOARD.md` 中与“可直接上线”相关的状态更新为基于真实环境证据的状态。

**Step 5: 验证文档中不再出现错误口径**

Run:

```bash
rg -n "可灰度发布|允许上线|LLM 的意图识别 \\+ 知识库 RAG|多渠道 Webhook 接收" .
```

Expected:
- 不再在 `docs/PRODUCTION_LAUNCH.md` 中看到把当前代码误表述为已具备完整能力的语句

**Step 6: Commit**

```bash
git add docs/PRODUCTION_LAUNCH.md docs/REVIEW_REPORT_2026-05-04.md PRODUCTION_PHASE1_STATUS.md prd/PRODUCTION_CHECKLIST.md docs/P0_P1_P2_RECTIFICATION_EXECUTION_BOARD.md
git commit -m "docs(ai-customer-service): align launch status with verified phase-1 scope"
```

---

### Task 2: 收口部署配置契约

**Files:**
- Modify: `docs/PRODUCTION_LAUNCH.md`
- Modify: `docs/RUNBOOK.md`
- Modify: `docs/CONFIG_CONTRACT_BASELINE.md`
- Test: `internal/config/config_test.go`

**Step 1: 写出真实变量清单**

以 `internal/config/config.go` 为唯一基线，整理以下变量：

```text
AI_CS_ADDR
AI_CS_POSTGRES_ENABLED
AI_CS_POSTGRES_DSN
AI_CS_POSTGRES_MIGRATION_DIR
AI_CS_POSTGRES_MAX_OPEN_CONNS
AI_CS_POSTGRES_MAX_IDLE_CONNS
AI_CS_POSTGRES_CONN_MAX_LIFETIME_SEC
AI_CS_WEBHOOK_SECRET
AI_CS_WEBHOOK_TIMESTAMP_HEADER
AI_CS_WEBHOOK_SIGNATURE_HEADER
AI_CS_WEBHOOK_MAX_SKEW_SECONDS
AI_CS_RUNTIME_ENV
```

**Step 2: 修正文档中的伪变量**

将 `POSTGRES_HOST`、`SERVER_PORT`、`WEBHOOK_HMAC_KEY` 等非真实变量全部替换或注明为废弃口径。

**Step 3: 为缺省/非法值补测试**

在 `internal/config/config_test.go` 增加针对以下场景的测试：
- `AI_CS_RUNTIME_ENV=production` 且 `AI_CS_POSTGRES_ENABLED=false` -> fail
- `AI_CS_RUNTIME_ENV=production` 且 `AI_CS_WEBHOOK_SECRET=""` -> fail
- 非 prod 下 memory 模式 -> pass

**Step 4: 运行测试**

Run:

```bash
go test ./internal/config -count=1
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add docs/PRODUCTION_LAUNCH.md docs/RUNBOOK.md docs/CONFIG_CONTRACT_BASELINE.md internal/config/config_test.go
git commit -m "docs(config): align deployment contract with runtime config loader"
```

---

### Task 3: 为后台接口补最小鉴权和角色边界

**Files:**
- Modify: `internal/http/router.go`
- Modify: `internal/http/handlers/ticket_handler.go`
- Modify: `internal/http/handlers/session_handler.go`
- Create: `internal/http/middleware/authz.go`
- Create: `internal/http/middleware/authz_test.go`
- Modify: `internal/http/router_test.go`
- Modify: `prd/IDENTITY_AND_PERMISSION_STRATEGY.md`

**Step 1: 先写失败测试**

至少覆盖：

```go
func TestTicketAssign_shouldReject_whenMissingAuthHeader(t *testing.T) {}
func TestTicketResolve_shouldReject_whenRoleNotAllowed(t *testing.T) {}
func TestSessionHandoff_shouldReject_whenActorSpoofedByQueryOnly(t *testing.T) {}
```

**Step 2: 运行测试确认失败**

Run:

```bash
go test ./internal/http/... -count=1
```

Expected:
- FAIL，提示缺少鉴权中间件或权限校验

**Step 3: 写最小实现**

实现原则：
- 不上完整 OAuth/JWT 平台
- 先引入最小 header-based 鉴权，供预生产和灰度环境使用
- 建议从请求头读取：
  - `X-CS-Actor-ID`
  - `X-CS-Actor-Role`
- 允许角色：
  - `agent`
  - `supervisor`
  - `admin`
- 将 `actor_id` 从 query 参数降为只读兼容，不作为可信来源

**Step 4: 权限规则落地**

最小规则：
- `GET /tickets/{id}`: `agent/supervisor/admin`
- `POST /tickets/{id}/assign`: `supervisor/admin`
- `POST /tickets/{id}/resolve`: `agent/supervisor/admin`
- `POST /tickets/{id}/close`: `supervisor/admin`
- `POST /sessions/{id}/handoff`: `agent/supervisor/admin`
- `POST /sessions/{id}/feedback`: 可匿名或系统，但要记录来源

**Step 5: 跑测试**

Run:

```bash
go test ./internal/http/... -count=1
```

Expected:
- PASS

**Step 6: 更新策略文档**

把 `prd/IDENTITY_AND_PERMISSION_STRATEGY.md` 中“当前未落地”的状态更新为“Phase 1 最小鉴权已落地，完整 RBAC 仍未完成”。

**Step 7: Commit**

```bash
git add internal/http/router.go internal/http/handlers/ticket_handler.go internal/http/handlers/session_handler.go internal/http/middleware/authz.go internal/http/middleware/authz_test.go internal/http/router_test.go prd/IDENTITY_AND_PERMISSION_STRATEGY.md
git commit -m "feat(auth): add minimal auth and role checks for phase-1 admin APIs"
```

---

### Task 4: 收口工单闭环语义

**Files:**
- Modify: `internal/http/handlers/ticket_handler.go`
- Modify: `internal/store/postgres/ticket_workflow.go`
- Modify: `internal/store/memory/ticket_workflow.go`
- Modify: `internal/http/handlers/ticket_handler_test.go`
- Modify: `test/e2e/full_ticket_flow_test.go`
- Modify: `prd/TICKET_OPERATIONS_SOP.md`
- Modify: `tech/INTERFACE.md`

**Step 1: 补测试，明确 resolve 和 close 的语义**

覆盖：
- assign 后 resolve 成功
- resolve 后 close 成功
- 已 close 工单不可再次 resolve
- 不存在工单返回明确错误

**Step 2: 运行测试确认边界失败**

Run:

```bash
go test ./internal/http/handlers ./internal/store/... ./test/e2e -count=1
```

Expected:
- FAIL，暴露当前状态机或文档不一致问题

**Step 3: 实现最小一致语义**

建议：
- `resolve` 表示“给出处理结论，但工单仍可后续关闭”
- `close` 表示“最终关闭，不可再变更”

**Step 4: 对齐接口文档**

在 `tech/INTERFACE.md` 和 `prd/TICKET_OPERATIONS_SOP.md` 明确：
- 各状态定义
- 可执行动作
- 返回错误码

**Step 5: 跑测试**

Run:

```bash
go test ./internal/http/handlers ./internal/store/... ./test/e2e -count=1
```

Expected:
- PASS

**Step 6: Commit**

```bash
git add internal/http/handlers/ticket_handler.go internal/store/postgres/ticket_workflow.go internal/store/memory/ticket_workflow.go internal/http/handlers/ticket_handler_test.go test/e2e/full_ticket_flow_test.go prd/TICKET_OPERATIONS_SOP.md tech/INTERFACE.md
git commit -m "fix(ticket): align resolve and close semantics across stores and docs"
```

---

### Task 5: 建立真实预生产验证脚本与证据

**Files:**
- Create: `scripts/verify_preprod_gate_b.sh`
- Create: `docs/PREPROD_VERIFICATION_RECORD.md`
- Modify: `docs/RUNBOOK.md`
- Modify: `test/QA_GATE_STATUS.md`

**Step 1: 写预生产 Gate B 检查脚本**

脚本至少覆盖：
- 环境变量完整性校验
- 服务启动
- migration 执行
- `/actuator/health/live`
- `/actuator/health/ready`
- webhook 有签名请求
- ticket/audit 入库验证

**Step 2: 先用本地/容器化环境跑一遍**

Run:

```bash
bash scripts/verify_preprod_gate_b.sh
```

Expected:
- 输出每项 PASS/FAIL

**Step 3: 把验证结果沉淀为记录**

在 `docs/PREPROD_VERIFICATION_RECORD.md` 中记录：
- 时间
- 环境
- commit
- 执行命令
- 结果截图或关键输出摘要

**Step 4: QA 门禁回写**

更新 `test/QA_GATE_STATUS.md`，将“真实环境门禁未闭环”替换为当前实际结果。

**Step 5: Commit**

```bash
git add scripts/verify_preprod_gate_b.sh docs/PREPROD_VERIFICATION_RECORD.md docs/RUNBOOK.md test/QA_GATE_STATUS.md
git commit -m "test(preprod): add gate-b verification script and evidence record"
```

---

### Task 6: 建立最小监控与灰度观察面

**Files:**
- Modify: `docs/MONITORING_ALERTING.md`
- Modify: `prd/SERVICE_SLA.md`
- Modify: `prd/GRAY_RELEASE_ROLLBACK_RUNBOOK.md`
- Create: `docs/GRAY_DASHBOARD_MINIMUM.md`

**Step 1: 确认灰度阶段只看最小指标**

必须包含：

```text
1. webhook 5xx
2. webhook reject 数
3. ticket 创建量
4. handoff 比率
5. audit 写入失败数
6. readiness down 次数
7. postgres 连接异常
8. 单实例重启次数
```

**Step 2: 为每个指标写告警阈值**

示例：
- webhook 5xx > 1% 持续 5 分钟 -> 触发回滚评估
- readiness 连续 3 次 DOWN -> 从灰度池摘流量

**Step 3: 写灰度放量节奏**

建议默认：
- 5% / 30min
- 20% / 2h
- 50% / 半天
- 100% / 次日

每一级都必须有进入和回退条件。

**Step 4: 文档回写**

把以上阈值和动作同步回：
- `docs/MONITORING_ALERTING.md`
- `prd/SERVICE_SLA.md`
- `prd/GRAY_RELEASE_ROLLBACK_RUNBOOK.md`

**Step 5: Commit**

```bash
git add docs/MONITORING_ALERTING.md prd/SERVICE_SLA.md prd/GRAY_RELEASE_ROLLBACK_RUNBOOK.md docs/GRAY_DASHBOARD_MINIMUM.md
git commit -m "docs(gray): define minimum metrics, thresholds, and rollout gates"
```

---

### Task 7: 建立灰度放行清单

**Files:**
- Create: `docs/GRAY_LAUNCH_CHECKLIST.md`
- Modify: `docs/P0_P1_P2_RECTIFICATION_EXECUTION_BOARD.md`
- Modify: `docs/REVIEW_REPORT_2026-05-04.md`

**Step 1: 设计一页式放行清单**

清单必须包含：
- 代码级门禁
- 预生产 Gate B
- 鉴权门禁
- 工单闭环门禁
- 观测门禁
- 回滚门禁

**Step 2: 用 checkbox 明确阻断条件**

示例：

```markdown
- [ ] go test ./... 通过
- [ ] go test -race ./... 通过
- [ ] 真实 PostgreSQL migration 成功
- [ ] 后台接口鉴权已启用
- [ ] webhook 签名联调通过
- [ ] ticket/audit 入库可验证
- [ ] 最小监控告警上线
- [ ] 回滚脚本/Runbook 演练通过
```

**Step 3: 将执行板状态改为面向灰度**

执行板中未闭环项按：
- 未开始
- 进行中
- 已完成
- 已阻塞

重新标注。

**Step 4: Commit**

```bash
git add docs/GRAY_LAUNCH_CHECKLIST.md docs/P0_P1_P2_RECTIFICATION_EXECUTION_BOARD.md docs/REVIEW_REPORT_2026-05-04.md
git commit -m "docs(release): add gray launch checklist and update execution board"
```

---

## 里程碑与退出条件

### Milestone A：文档和配置真实收口

退出条件：
- `docs/PRODUCTION_LAUNCH.md` 不再夸大现状
- 部署变量文档与 `internal/config/config.go` 一致

### Milestone B：后台最小可信

退出条件：
- `tickets` / `sessions` 关键接口具备最小鉴权
- `actor_id` 不再来自不可信 query 参数

### Milestone C：预生产可验证

退出条件：
- `scripts/verify_preprod_gate_b.sh` 可重复执行
- 有一份真实 `PREPROD_VERIFICATION_RECORD`

### Milestone D：可灰度

退出条件：
- 灰度指标、阈值、回滚条件清晰
- `GRAY_LAUNCH_CHECKLIST` 全部打勾

---

## 推荐执行顺序

1. Task 1
2. Task 2
3. Task 3
4. Task 4
5. Task 5
6. Task 6
7. Task 7

这个顺序的原因：
- 先收口口径，避免边做边漂
- 再补接口安全，避免把不可信后台继续往前推
- 再做联调和灰度准备，保证验证基于可信实现

---

Plan complete and saved to `docs/plans/2026-05-04-gray-launch-readiness-plan.md`. Two execution options:

**1. Subagent-Driven (this session)** - 我按任务逐项执行、每项做验证和回写，适合现在直接推进

**2. Parallel Session (separate)** - 在独立会话按计划批量执行，适合长周期整改
