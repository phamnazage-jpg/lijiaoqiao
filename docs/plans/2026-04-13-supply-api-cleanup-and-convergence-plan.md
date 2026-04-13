# Supply API Cleanup And Convergence Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 建立一条真正可提交的清理车道，先修正文档与归档事实，再按模块收口 `Outbox` 和 `Compensation`，每个任务都带独立验证。

**Architecture:** 先把“事实源”收口，再做代码收口。根仓库 `reports/gates/` 在出现完整迁移清单前继续视为审计主目录；`supply-api/reports/archive/` 只保留项目内部的历史文档和已明确降级为“历史快照/非门禁证据”的报告。`Outbox` 和 `Compensation` 采用“领域契约不动、适配层逐步瘦身”的方式演进，避免直接删掉当前运行路径。

**Tech Stack:** Git, Go 1.x, PostgreSQL, Redis, Markdown, `rg`, `go test`

---

### Task 1: 冻结不安全清理边界

**Files:**
- Modify: `supply-api/reports/CLEANUP_REPORT_2026-04-13.md`
- Modify: `supply-api/reports/archive/2026-04-13/validation_report_2026-04-13.md`
- Modify: `supply-api/reports/archive/2026-04-13/playbook_validation_report_20260413_122847.md`
- Modify: `supply-api/reports/archive/test_playbook_report_20260413_114825.md`
- Test: `git diff --cached --name-status -- reports/gates supply-api/reports`

**Step 1: 运行事实校验，确认当前报告不可信**

Run:
```bash
git diff --cached --name-status -- reports/gates supply-api/reports
rg -n 'gate_verification|TIMESTAMP|删除 internal/outbox/|HTTP 服务器没有监听端口' supply-api/reports
```

Expected:
- `reports/gates/*` 只有删除，没有完整的新归档树。
- 报告中仍存在 `TIMESTAMP`、错误归档声明、错误架构判断。

**Step 2: 修正文档性质和边界**

Write:
```md
- 把清理报告改成“复核报告”，明确哪些内容可提交、哪些只能保留在工作区待处理。
- 把验证报告改成“历史快照 + 当前代码复核结论”，禁止继续作为当前门禁证据引用。
- 把剧本报告降级为“测试清单/人工核对摘要”，不再宣称数据库持久化和审计已被证明。
```

**Step 3: 运行校验，确认模板残留和错误结论已清除**

Run:
```bash
rg -n 'TIMESTAMP|gate_verification|删除 internal/outbox/|HTTP 服务器没有监听端口' supply-api/reports
git diff --check -- supply-api/reports/CLEANUP_REPORT_2026-04-13.md supply-api/reports/archive/2026-04-13/validation_report_2026-04-13.md supply-api/reports/archive/2026-04-13/playbook_validation_report_20260413_122847.md supply-api/reports/archive/test_playbook_report_20260413_114825.md
```

Expected:
- `TIMESTAMP` 不再出现。
- 清理报告不再声称根仓库门禁报告已经迁移完成。
- 差异格式检查通过。

**Step 4: 提交文档纠偏**

Run:
```bash
git add supply-api/reports/CLEANUP_REPORT_2026-04-13.md supply-api/reports/archive/2026-04-13/validation_report_2026-04-13.md supply-api/reports/archive/2026-04-13/playbook_validation_report_20260413_122847.md supply-api/reports/archive/test_playbook_report_20260413_114825.md
git commit -m "docs(cleanup): correct archive and validation reports"
```

Expected:
- 只提交文档纠偏，不携带根仓库 `reports/gates/*` 删除和本机配置改动。

### Task 2: 切出“可提交清理”最小集合

**Files:**
- Modify: `supply-api/reports/CLEANUP_REPORT_2026-04-13.md`
- Test: `git status --short`
- Test: `git diff --cached --name-status -- reports/gates supply-api/config supply-api/e2e supply-api/scripts`

**Step 1: 列出必须排除出首个清理提交的路径**

Run:
```bash
git diff --cached --name-status -- reports/gates supply-api/config supply-api/e2e supply-api/scripts
```

Expected:
- 能看到根仓库 `reports/gates/*` 批量删除。
- 能看到 `supply-api/e2e/README.md`、`supply-api/config/config.test.yaml`、`supply-api/scripts/production_test.sh` 等待进一步判断的内容。

**Step 2: 在复核报告里写清楚“首个可提交集合”**

Write:
```md
首个可提交集合仅包含：
1. `supply-api/docs/archive/` 下的真实文档归档；
2. `supply-api/reports/archive/` 下已明确标注性质的历史报告；
3. `docs/plans/2026-04-13-supply-api-cleanup-and-convergence-plan.md`。
```

**Step 3: 运行边界校验**

Run:
```bash
git status --short
git diff --cached --name-status -- reports/gates
```

Expected:
- 根仓库 `reports/gates/*` 仍留在工作区待处理，不进入首个清理提交。

**Step 4: 提交最小清理集合**

Run:
```bash
git add docs/plans/2026-04-13-supply-api-cleanup-and-convergence-plan.md supply-api/reports/CLEANUP_REPORT_2026-04-13.md supply-api/reports/archive/2026-04-13/validation_report_2026-04-13.md supply-api/reports/archive/2026-04-13/playbook_validation_report_20260413_122847.md supply-api/reports/archive/test_playbook_report_20260413_114825.md
git commit -m "docs(plan): add committable cleanup and convergence plan"
```

Expected:
- 提交只包含计划和已纠偏报告。

### Task 3: 恢复配置样例与本机配置的边界

**Files:**
- Modify: `supply-api/config/config.dev.yaml`
- Create: `supply-api/config/config.local.example.yaml`
- Test: `supply-api/internal/config/config_samples_test.go`

**Step 1: 写失败校验，证明样例已被本机配置污染**

Run:
```bash
cd supply-api && go test ./internal/config
```

Expected:
- `TestDevSampleConfigLoads` 失败，报 `expected database host localhost, got /var/run/postgresql`。

**Step 2: 写最小修复**

Write:
```yaml
# config.dev.yaml 保持仓库样例值
database:
  host: "localhost"
  port: 5432

# config.local.example.yaml 提供 Unix socket / 本机数据库示例
database:
  host: "/var/run/postgresql"
  port: 5432
```

**Step 3: 运行样例测试**

Run:
```bash
cd supply-api && go test ./internal/config
```

Expected:
- `internal/config` 全部通过。

**Step 4: 提交配置边界修复**

Run:
```bash
git add supply-api/config/config.dev.yaml supply-api/config/config.local.example.yaml supply-api/internal/config/config_samples_test.go
git commit -m "chore(config): separate sample config from local overrides"
```

Expected:
- 仓库样例恢复可复现，本机配置有独立承载位置。

### Task 4: Outbox 收口第一阶段，先统一契约，再削掉重复映射

**Files:**
- Modify: `supply-api/internal/domain/outbox.go`
- Modify: `supply-api/internal/outbox/outbox.go`
- Modify: `supply-api/internal/repository/outbox.go`
- Test: `supply-api/internal/domain/outbox_test.go`
- Test: `supply-api/internal/outbox/outbox_test.go`
- Test: `supply-api/internal/repository/outbox_test.go`

**Step 1: 写失败测试，锁定当前重复点**

Write:
```go
func TestOutboxRunner_UsesSharedBackoffAndFailureContract(t *testing.T) {
    // 断言 runner 与 domain 使用同一份退避策略和失败语义
}
```

**Step 2: 运行失败测试**

Run:
```bash
cd supply-api && go test ./internal/domain ./internal/outbox ./internal/repository -run "Outbox"
```

Expected:
- 新测试失败，或现有测试不能证明共享契约已成立。

**Step 3: 写最小实现**

Write:
```go
// domain/outbox.go
type RetryBackoff interface {
    Next(retryCount int) time.Time
}

// internal/outbox/outbox.go
// 复用 domain 的 backoff 计算，避免 runner 自带一份重复逻辑。
```

**Step 4: 运行包级测试**

Run:
```bash
cd supply-api && go test ./internal/domain ./internal/outbox ./internal/repository
```

Expected:
- 三个包全部通过。

**Step 5: 提交 Outbox 第一阶段**

Run:
```bash
git add supply-api/internal/domain/outbox.go supply-api/internal/outbox/outbox.go supply-api/internal/repository/outbox.go supply-api/internal/domain/outbox_test.go supply-api/internal/outbox/outbox_test.go supply-api/internal/repository/outbox_test.go
git commit -m "refactor(outbox): share domain contract across runner and repository"
```

Expected:
- 仍保留 `internal/outbox` 运行器，但不再维护两套退避/失败语义。

### Task 5: Outbox 收口第二阶段，消除 `domain` 和 `repository` 之间的事件复制

**Files:**
- Modify: `supply-api/internal/domain/outbox.go`
- Modify: `supply-api/internal/outbox/outbox.go`
- Modify: `supply-api/internal/messaging/outbox_broker.go`
- Test: `supply-api/internal/outbox/outbox_test.go`

**Step 1: 写失败测试，锁定 DTO 转换次数**

Write:
```go
func TestOutboxRunner_PublishesWithoutAdHocDomainEventCopy(t *testing.T) {
    // 断言 runner 不再手工复制 repository.OutboxEvent -> domain.OutboxEvent
}
```

**Step 2: 运行失败测试**

Run:
```bash
cd supply-api && go test ./internal/outbox -run "PublishesWithoutAdHocDomainEventCopy"
```

Expected:
- 失败，证明当前还有运行时手工拷贝。

**Step 3: 写最小实现**

Write:
```go
type DispatchableOutboxEvent interface {
    GetEventID() string
    GetEventType() string
    GetPayload() json.RawMessage
}
```

**Step 4: 运行测试**

Run:
```bash
cd supply-api && go test ./internal/domain ./internal/outbox ./internal/messaging ./internal/repository
```

Expected:
- 事件发布路径通过，转换层减少到单点。

**Step 5: 提交 Outbox 第二阶段**

Run:
```bash
git add supply-api/internal/domain/outbox.go supply-api/internal/outbox/outbox.go supply-api/internal/messaging/outbox_broker.go supply-api/internal/outbox/outbox_test.go
git commit -m "refactor(outbox): remove runner event copy layer"
```

Expected:
- `main.go` 装配关系保持不变，运行器只负责调度，不再持有重复 DTO 逻辑。

### Task 6: Compensation 收口第一阶段，明确“处理器”和“执行器”的职责边界

**Files:**
- Modify: `supply-api/internal/domain/compensation.go`
- Modify: `supply-api/internal/compensation/compensation.go`
- Test: `supply-api/internal/domain/compensation_test.go`
- Test: `supply-api/internal/domain/compensation_context_test.go`

**Step 1: 写失败测试，锁定职责边界**

Write:
```go
func TestCompensationProcessor_DoesNotKnowExecutorImplementationDetails(t *testing.T) {
    // 断言 processor 只依赖 OperationExecutor，不依赖具体执行器细节
}
```

**Step 2: 运行失败测试**

Run:
```bash
cd supply-api && go test ./internal/domain -run "CompensationProcessor"
```

Expected:
- 失败，或现有测试没有覆盖职责边界。

**Step 3: 写最小实现**

Write:
```go
// domain/compensation.go
type OperationExecutor interface {
    Execute(ctx context.Context, operationType string, payload json.RawMessage) error
}

// compensation/compensation.go
// 默认执行器只保留 operationType -> handler 的派发逻辑。
```

**Step 4: 运行测试**

Run:
```bash
cd supply-api && go test ./internal/domain ./internal/compensation
```

Expected:
- `domain` 和 `compensation` 相关测试通过。

**Step 5: 提交 Compensation 第一阶段**

Run:
```bash
git add supply-api/internal/domain/compensation.go supply-api/internal/compensation/compensation.go supply-api/internal/domain/compensation_test.go supply-api/internal/domain/compensation_context_test.go
git commit -m "refactor(compensation): isolate processor from executor details"
```

Expected:
- 清理报告不再把两者标成重复实现。

### Task 7: Compensation 收口第二阶段，拆出可替换的操作处理器

**Files:**
- Modify: `supply-api/internal/compensation/compensation.go`
- Create: `supply-api/internal/compensation/handlers.go`
- Test: `supply-api/internal/compensation/compensation_test.go`

**Step 1: 写失败测试，锁定不同操作类型的派发**

Write:
```go
func TestDefaultCompensationExecutor_DispatchesByOperationType(t *testing.T) {
    // account.create / package.publish / settlement.withdraw / quota.deduct
}
```

**Step 2: 运行失败测试**

Run:
```bash
cd supply-api && go test ./internal/compensation -run "DispatchesByOperationType"
```

Expected:
- 测试失败，证明当前执行器仍是大 switch。

**Step 3: 写最小实现**

Write:
```go
type Handler func(ctx context.Context, payload json.RawMessage) error

type DefaultCompensationExecutor struct {
    handlers map[string]Handler
}
```

**Step 4: 运行测试**

Run:
```bash
cd supply-api && go test ./internal/compensation ./internal/domain
```

Expected:
- 默认执行器通过 map 派发，后续新增操作不再修改大 switch 主体。

**Step 5: 提交 Compensation 第二阶段**

Run:
```bash
git add supply-api/internal/compensation/compensation.go supply-api/internal/compensation/handlers.go supply-api/internal/compensation/compensation_test.go
git commit -m "refactor(compensation): replace switch executor with handler registry"
```

Expected:
- 执行器扩展点清晰，符合 OCP。

### Task 8: 根仓库门禁证据迁移前，先做归档索引而不是批量删除

**Files:**
- Create: `reports/archive/gate_verification/INDEX_2026-04-13.md`
- Test: `reports/gates/`
- Test: `reports/archive/gate_verification/`

**Step 1: 写失败校验，证明当前只有删除没有索引**

Run:
```bash
find reports/archive/gate_verification -maxdepth 2 -type f 2>/dev/null
git diff --cached --name-status -- reports/gates
```

Expected:
- 归档目录缺失或为空。
- 根仓库门禁证据仍处于“删了但没登记”的状态。

**Step 2: 写最小索引**

Write:
```md
# Gate Verification Archive Index

- 原始目录: reports/gates
- 迁移批次: 2026-04-13
- 迁移原则: 先登记，再移动，再删除
- 清单: <filename> -> <archive path>
```

**Step 3: 运行索引校验**

Run:
```bash
test -f reports/archive/gate_verification/INDEX_2026-04-13.md
rg -n "先登记，再移动，再删除" reports/archive/gate_verification/INDEX_2026-04-13.md
```

Expected:
- 索引存在，迁移原则可追溯。

**Step 4: 提交索引和首批真实迁移**

Run:
```bash
git add reports/archive/gate_verification/INDEX_2026-04-13.md
git commit -m "docs(gates): add archive index before moving gate evidence"
```

Expected:
- 审计链从“批量删文件”改成“可追溯迁移”。
