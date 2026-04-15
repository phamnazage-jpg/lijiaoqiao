# 规划设计闭环执行任务清单（Superpowers v2）

- 日期：2026-03-30
- 基线来源：`docs/plans/2026-03-25-superpowers-execution-tasklist-v1.md`
- 目标：基于最新真实证据重排执行优先级，持续推进到 staging 可复核、可签署。

> 机审稿引用约束：活文档只允许引用 `review/outputs/current_machine_review_sources.md` 中声明的现行机审稿。本页保留的旧 `tok007` 路径仅作历史续跑证据，不得继续当作当前事实源。

---

## 1. 实际状态复盘（以证据为准）

### 1.1 已闭环工作流（开发阶段）

1. `WG-A/WG-B/WG-C` 已完成：需求冻结、OpenAPI 契约对齐、追踪矩阵一致化。
2. `WG-F/WG-G` 已完成：全局 P0 映射、命名策略、跨文档一致性与最终决议草稿链路。
3. TOK 链路已完成开发闭环（`TOK-002 ~ TOK-007`）：包含 runtime、门禁汇总、复审与候选稿生成。

关键证据：
1. `reports/superpowers_execution_progress_2026-03-27.md`
2. `reports/alignment_validation_checkpoint_12_2026-03-30.md` ~ `reports/alignment_validation_checkpoint_27_2026-03-30.md`
3. `reports/gates/superpowers_stage_validation_2026-03-30_212426.md`

### 1.2 未闭环工作流（真实环境）

1. `WG-D/WG-E` 仍未完成真实 staging 证据闭环，当前仅有 local/mock 与 dry-run 证据。
2. 最终签署决议当前为 `NO-GO`，核心阻塞集中在 `F-01/F-02/F-04`（P0）与 `F-03`（P1）。

关键证据：
1. `review/final_decision_2026-03-31.md`
2. `reports/supply_gate_review_2026-03-31.md`
3. `reports/token_runtime_implementation_gap_review_2026-03-30.md`

---

## 2. 状态矩阵（v2）

| 工作流 | 状态 | 说明 | 下一动作 |
|---|---|---|---|
| WG-A 需求冻结 | DONE | 已完成冻结与决议映射 | 仅维护 |
| WG-B 契约对齐 | DONE | OpenAPI 与幂等头已落地 | 仅维护 |
| WG-C 测试矩阵 | DONE | 路径一致化与规则文档已落地 | 仅维护 |
| WG-D 真实联调 | BLOCKED（外部依赖） | 缺真实 staging 地址与有效短期 token | 优先解锁 F-01/F-02/F-04 |
| WG-E 报告签署 | BLOCKED（依赖 WG-D） | 缺真实证据，无法转 GO | 与 WG-D 同步推进 |
| WG-F 一致性收尾 | DONE | 命名策略与映射补齐完成 | 仅维护 |
| WG-G 全局校验 | DONE（开发口径） | 校验链路可执行，决议一致性脚本已在跑 | 补真实口径复核 |
| TOK 运行态链路 | DONE（开发口径） | M-021 开发阶段 100% | 需 staging 实证回填 |

---

## 3. P0/P1 阻塞项（从最终决议回填）

| 编号 | 等级 | 阻塞描述 | Owner | 截止日期 | 退出条件 |
|---|---|---|---|---|---|
| F-01 | P0 | staging DNS 与 `API_BASE_URL` 可达性修复，重跑 SUP-004~007 | PLAT + QA | 2026-04-01 | `staging_precheck_and_run.sh` 在真实环境 PASS |
| F-02 | P0 | 补齐 M-013~M-016 staging 实测值 | SEC + QA | 2026-04-01 | `sec_sup_boundary_report` 回填真实 PASS |
| F-04 | P0 | token runtime staging 联调取证 | ARCH + PLAT + SEC | 2026-04-03 | `M-021` 与边界指标 staging 证据齐全 |
| F-03 | P1 | M-017/M-018/M-019 连续 7 天趋势证据 | PLAT + PMO | 2026-04-05 | 趋势报告满足 7 天口径 |

---

## 4. 批次执行计划（从 2026-03-30 起）

### Batch-MON-01（当前批次，先做“可持续执行”能力）

1. `MON-001`：新增 Minimax 7 日趋势脚本（监控链路补齐）。
2. `MON-002`：将 Minimax 日快照接入 `superpowers_release_pipeline.sh`（可选、默认关闭、非阻断）。
3. `MON-003`：更新命令手册，补齐执行与断言说明。
4. `MON-004`：产出对齐验证报告（Checkpoint-28）。

执行结果（2026-03-30）：

| 任务 | 状态 | 证据 |
|---|---|---|
| MON-001 | DONE | `scripts/ci/minimax_upstream_trend_report.sh` + `reports/gates/minimax_upstream_trend_7d_2026-03-30.md` |
| MON-002 | DONE | `scripts/ci/superpowers_release_pipeline.sh` + `reports/gates/superpowers_release_pipeline_2026-03-30_235224.md` |
| MON-003 | DONE | `docs/supply_gate_command_playbook_v1_2026-03-25.md` |
| MON-004 | DONE | `reports/alignment_validation_checkpoint_28_2026-03-30.md` |

### Batch-STG-01（下一批次，真实环境解锁）

1. `STG-001`：确认真实 staging 网关地址并更新 `scripts/supply-gate/.env`。
2. `STG-002`：注入有效 owner/viewer/admin 短期 token（禁用占位值）。
3. `STG-003`：执行 `scripts/ci/staging_release_pipeline.sh`（真实环境，不允许 local/mock）。
4. `STG-004`：回填 `F-01/F-02/F-04` 证据到最终决议与评审报告。

当前门禁检查（2026-03-30）：
1. `scripts/supply-gate/.env` 中 `API_BASE_URL` 仍处于阻塞态（非真实 staging 可发布地址）。
2. 因 `STG-001` 未完成，`STG-003` 当前不得触发真实放行判定。

本机开发测试续跑结果（2026-03-31）：
1. `local/mock` 口径 `staging_release_pipeline` 已复跑通过：`reports/gates/staging_release_pipeline_2026-03-31_100116.md`。
2. STG 本地续跑中识别并修复 `PHASE-10` 阻塞（M-021 smoke 端口冲突与控制流提前退出）。
3. 修复后 `superpowers_release_pipeline` 与 `tok007` 复审链路恢复，结论维持 `CONDITIONAL_GO`。
4. `STG-001/STG-002`（真实 staging 地址与真 token）仍未完成，真实放行证据仍阻塞。

本机端口基线固化结果（2026-03-31）：
1. 已清理蚊子残留进程与冲突端口占用，详见 `reports/gates/local_dev_port_baseline_2026-03-31.md`。
2. 清理后再次复测 `staging_release_pipeline`：`reports/gates/staging_release_pipeline_2026-03-31_100942.md`（PASS）。
3. 对齐验证补充：`reports/alignment_validation_checkpoint_30_2026-03-31.md`。

真实 STG 前置自动化补齐（2026-03-31）：
1. 已新增本地 `.env.staging-real` 一键生成脚本：`scripts/ci/generate_local_staging_env.sh`。
2. 已新增真实 STG 就绪度检查脚本：`scripts/ci/staging_real_readiness_check.sh`。
3. 当前 `.env.staging-real` 就绪检查结论为 `BLOCKED`：`reports/gates/staging_real_readiness_2026-03-31_110213.md`。
4. 阻塞原因聚焦在 `STG-RDY-004/008`（API_BASE_URL 仍为本地地址且无真实外网可达性）。

完整开发测试续跑结果（2026-03-31 12:31）：
1. 已重新生成 `.env.staging-real` 且三类 token 均为非占位值：`reports/gates/local_staging_env_generation_2026-03-31_123102.md`。
2. `local/mock` 口径 `staging_release_pipeline` 再次通过：`reports/gates/staging_release_pipeline_2026-03-31_123148.md`。
3. `superpowers_release_pipeline` 与 `tok007` 复审链路再次通过，机判维持 `CONDITIONAL_GO`：`reports/gates/superpowers_release_pipeline_2026-03-31_123150.md`、`review/outputs/tok007_release_recheck_2026-03-31_123153.md`（历史续跑稿）。
4. 真实 STG 就绪度检查仍为 `BLOCKED`：`reports/gates/staging_real_readiness_2026-03-31_123159.md`（`STG-RDY-004/008` 未关闭）。
5. Minimax 上游 smoke 继续保持 `PASS`：`reports/gates/minimax_upstream_smoke_2026-03-31_123210.md`。

---

## 5. 执行约束

1. `local/mock` 结果仅可作为开发演练证据，不可替代 staging 放行证据。
2. 任何 `P0` 项未关闭，最终结论不得上调为 `GO`。
3. 所有阶段结论以脚本返回码 + 报告产物双重校验为准。
4. 活文档若需引用 `review/outputs/` 下机审稿，只允许引用页首声明的现行机审稿；历史续跑稿只能用于追溯。

---

## 6. 与 v1 的关系

1. `v1` 保留原子任务定义（A~G）。
2. `v2` 作为执行态总控视图，负责状态、批次与阻塞跟踪。
