# 项目整理复核报告

**日期**: 2026-04-13
**仓库**: /home/long/project/立交桥/supply-api

---

## 复核摘要

| 维度 | 结论 |
|------|------|
| `supply-api/docs/archive/` | 存在真实的文档归档动作，可作为首批可提交内容的一部分 |
| `supply-api/reports/archive/` | 可保留，但必须明确“历史快照”与“门禁证据”的区别 |
| 根仓库 `reports/gates/` | 当前是大规模删除，不是完成迁移，不能直接提交 |
| `config.dev.yaml` | 已被本机 Unix socket 配置污染，不应作为仓库样例直接提交 |
| `internal/outbox` / `internal/domain/outbox.go` | 不是可直接二选一的重复实现 |
| `internal/compensation` / `internal/domain/compensation.go` | 不是可直接二选一的重复实现 |

---

## 已确认可保留的归档

### 1. 废弃文档 (docs/archive/)

| 原路径 | 归档路径 |
|--------|----------|
| `docs/integration_test_strategy_v1.md` | `docs/archive/` |
| `docs/performance_test_baseline_v1.md` | `docs/archive/` |
| `docs/production_readiness_status_2026-04-07.md` | `docs/archive/` |
| `docs/testing_strategy_v1.md` | `docs/archive/` |

### 2. 废弃报告 (reports/archive/)

| 原路径 | 归档路径 |
|--------|----------|
| `reports/project_status_2026-04-08.md` | `reports/archive/` |
| `reports/test_coverage_report_2026-04-08.md` | `reports/archive/` |
| `reports/tdd_task_list_2026-04-09.md` | `reports/archive/tdd_task_lists/` |
| `reports/tdd_task_list_v2_2026-04-09.md` | `reports/archive/tdd_task_lists/` |

### 3. 当前不能直接提交的内容

#### 3.1 根仓库门禁证据仍处于“删除待处理”状态

复核后确认：根仓库本地已经存在 `reports/archive/gate_verification/` 归档树，但它此前既没有索引文件，也没有被仓库正式跟踪；同时 `reports/gates/` 的删除已进入暂存区，导致“本地归档存在”和“仓库可审计迁移”之间脱节。

截至 2026-04-13 的真实状态是：

- `reports/gates/` 工作区文件数为 0。
- `reports/gates/*` staged deletion 共 365 条。
- `reports/archive/gate_verification/` 本地文件数现已补齐到 1461。
- `reports/archive/` 当前整体仍是未跟踪目录。
- 此前缺失的 `metrics_daily_snapshots.csv`、`minimax_upstream_daily_snapshots.csv` 已按 `HEAD:reports/gates/*` 内容补档。

因此，以下内容仍不能直接宣称“已归档完成”：

- `reports/gates/backend_verify_*.md`
- `reports/gates/superpowers_stage_validation_*.md`
- `reports/gates/token_runtime_readiness_*.md`
- `reports/gates/token_runtime_smoke_*.log.*`
- `reports/gates/metrics_daily_snapshot_*.md`
- `reports/gates/metrics_trend_7d_*.md`

结论：索引和两个 CSV 缺口已补齐，下一步应核定哪些归档文件族正式纳入 Git，然后才能提交根仓库 `reports/gates/*` 的删除批次。

#### 3.2 本机配置和伪文档仍需隔离

- `config/config.dev.yaml` 当前包含本机 Unix socket 路径和本机用户，不应继续充当仓库样例。
- `config/config.test.yaml` 没有被仓库代码引用，且内容绑定本机 PostgreSQL socket 和本机用户，不应作为仓库测试样例保留。
- `e2e/README.md` 已修正为真实说明文档；E2E 源码应只保留在 `*_test.go`。
- `scripts/production_test.sh` 只有在输出目录迁移到 `reports/archive/production_runs/` 并允许环境变量覆盖后才适合作为仓库脚本保留。

### 4. 保留的有效文档

| 文件 | 说明 |
|------|------|
| `docs/alert_escalation_v1.md` | 告警升级策略 |
| `docs/integration_tests.md` | 集成测试指南 |
| `docs/logging_standardization_plan_v1.md` | 日志标准化计划 |
| `docs/project_experience_summary.md` | 项目经验总结 |
| `reports/tdd_task_list_v3_2026-04-09.md` | 最新 TDD 任务列表 |

---

## 已确认的构建产物清理方向

以下文件属于构建产物或临时输出，删除方向是正确的，但仍应和“文档归档提交”拆开处理：
- `supply-api` - 编译后的二进制文件
- `e2e.test` - E2E 测试二进制
- `coverage.out`, `cover.out`, `mid_coverage.out`, `audit_cover.out`, `iam_cover.out` - 覆盖率输出

---

## 架构复核：当前不是“重复实现二选一”

### 1. Outbox：领域模型 + 运行器适配层

| 路径 | 包类型 |
|------|--------|
| `internal/outbox/outbox.go` | `outbox` 包 |
| `internal/domain/outbox.go` | `domain` 包 |

当前运行路径由 `cmd/supply-api/main.go` 装配，实际启动的是 `internal/outbox.OutboxProcessorRunner`。`internal/domain/outbox.go` 提供的是领域模型、重试语义和处理器契约；`internal/outbox/outbox.go` 负责把 `repository.OutboxEvent`、`messaging.MessageBroker` 和后台轮询运行器拼接起来。

结论：这里的问题是“契约和 DTO 映射重复”，不是“保留 domain 或保留 outbox 二选一”。在完成适配层瘦身前，不能直接删除 `internal/outbox/`。

### 2. Compensation：处理器 + 默认执行器

| 路径 | 包类型 |
|------|--------|
| `internal/compensation/compensation.go` | `compensation` 包 |
| `internal/domain/compensation.go` | `domain` 包 |

当前运行路径同样由 `cmd/supply-api/main.go` 装配：`domain.NewCompensationProcessor(...)` 负责批量补偿流程控制，`compensation.NewDefaultCompensationExecutor()` 提供具体操作执行。

结论：这里的问题是“默认执行器过大、派发方式过硬编码”，不是“两份补偿实现重复”。在拆出 handler registry 之前，不能删除任一侧。

---

## 首个可提交清理集合

首个可提交集合应严格限制为以下内容：

1. `supply-api/docs/archive/` 下的真实文档迁移。
2. `supply-api/reports/archive/` 下已明确标注为“历史快照/非门禁证据”的报告。
3. `docs/plans/2026-04-13-supply-api-cleanup-and-convergence-plan.md`。

不应纳入首个提交的内容：

- 根仓库 `reports/gates/*` 的批量删除。
- `config/config.dev.yaml` 的本机化修改。
- `e2e/README.md` 这类伪文档。
- 任何没有原始证据链支撑的“PASS 结论”。

## 建议的后续步骤

1. **先提交文档纠偏**: 只提交归档说明、计划文件和已降级为历史快照的报告。
2. **恢复样例配置边界**: 让 `config.dev.yaml` 回到仓库样例状态，本机参数转移到本地覆盖文件。
3. **按模块收口 Outbox**: 先统一契约和退避策略，再移除运行器里的手工 DTO 复制。
4. **按模块收口 Compensation**: 保留 `processor + executor` 结构，拆分大 switch 为 handler registry。
5. **最后迁移根仓库门禁证据**: 先建索引，再移动，再删除。

---

**报告生成时间**: 2026-04-13T20:10:00+08:00
