> 归档状态：已归档
> 归档批次：ARCHIVE-HISTORY-2026-04-14-A01
> 归档标识：AHP-20260414-030
> 当前用途：历史快照，仅供追溯；不得作为现行门禁、发布决议或实现状态事实源。
> 当前事实源：`docs/plans/2026-04-14-repo-integrity-refactor-plan.md`、`reports/archive/gate_verification/` 与仍在维护的签署/门禁文档。

# PRD 与技术规划三次复检报告（2026-03-27）

- 复检版本：v3.0
- 对应报告：
  - `review/prd_tech_planning_expert_review_v1_2026-03-24.md`
  - `review/prd_tech_planning_recheck_v2_2026-03-25.md`
- 复检目标：验证“数据库域完整性 + 依赖兼容审计 + 分阶段质量门禁”是否达到行业最佳实践基线

---

## 1. 复检结论

结论：**CONDITIONAL GO（仅设计治理层） / NO-GO（实现发布层）**

说明：
1. 本轮新增缺口已从“发现问题”转为“有 SSOT + 有任务 + 有门禁 + 有 DDL”。
2. 已补齐 SQL 实库执行、依赖审计、阶段门禁回退演练证据；但仍缺连续 7 天指标观测与 SUP 实测闭环。
3. 新增事实审计确认 token 相关功能在运行态未完成（仅文档+mock），发布层必须维持 NO-GO。

---

## 2. 新增缺口与关闭状态

| 缺口ID | 问题 | 严重级别 | 当前状态 | 修订证据 |
|---|---|---|---|---|
| GAP-11 | 数据库仅 supply 域，核心域主表缺失 | P0 | 已关闭（设计层） | `sql/postgresql/platform_core_schema_v1.sql` + `docs/database_domain_model_and_governance_v1_2026-03-27.md` |
| GAP-12 | 缺少加密算法、单位、审计字段 | P0 | 已关闭（设计层） | `sql/postgresql/supply_schema_v1_patch_2026-03-27.sql` |
| GAP-13 | 供应域索引不足，无法覆盖高频组合查询 | P1 | 已关闭（设计层） | 同上 patch 索引补齐 |
| GAP-14 | 依赖版本兼容性审计缺失 | P0 | 已关闭（机制层） | `docs/dependency_compatibility_audit_baseline_v1_2026-03-27.md` + 门禁指标 M-017 |
| GAP-15 | 缺少分阶段全面测试质量检查，易偏离主线 | P0 | 已关闭（机制层） | `docs/acceptance_gate_single_source_v1_2026-03-18.md`（M-018/M-019）+ `docs/supply_test_plan_enhanced_v1_2026-03-25.md` |
| GAP-16 | 执行任务单未纳入 DB/依赖/阶段质量链路 | P1 | 已关闭（计划层） | `docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md`（新增 Workstream I） |
| GAP-17 | token 相关能力缺真实运行态实现（仅文档+mock） | P0 | 未关闭（实现层） | `reports/token_runtime_implementation_gap_review_2026-03-27.md` |

---

## 3. 行业最佳实践对齐检查

1. **数据库域建模**：已从单域扩展为 Core/IAM/Auth/Billing/Audit/Supply 六域协同，符合“业务域可独立演进”的原则。
2. **安全可审计性**：凭证相关字段采用“算法 + key alias + 版本 + 指纹”四要素，可支持密钥轮换与审计追踪。
3. **计量一致性**：新增单位字段（quota/price/amount/currency），降低跨域统计口径漂移风险。
4. **发布可控性**：新增依赖兼容审计四件套，避免“锁文件漂移”与“运行版本漂移”。
5. **质量防偏航**：引入 G0-G5 分阶段门禁与 M-018/M-019 指标，防止跳阶段推进。

---

## 4. 执行验证状态（进入 GO 前必须完成）

| 编号 | 验证项 | 状态 | 证据 |
|---|---|---|---|
| V-01 | 执行 `platform_core_schema_v1.sql` 与 `supply_schema_v1_patch_2026-03-27.sql` | 已完成 | `reports/db/sql_apply_2026-03-27.log`、`reports/db_schema_validation_report_2026-03-27.md` |
| V-02 | 回填 `M-017/M-018/M-019` 的首周实测值并持续观察 7 天 | 进行中 | 当前仅有首日证据，趋势数据未满 7 天 |
| V-03 | 完成一次“依赖变更 -> 兼容审计 -> 阻断/放行”演练并归档证据 | 已完成（基线演练） | `reports/dependency/dependency_audit_result_2026-03-27.md` |
| V-04 | 完成一次“阶段门禁失败 -> 回退前一阶段整改”的实战演练 | 已完成 | `reports/gates/stage_gate_drift_drill_report_2026-03-27.md` |
| V-05 | SUP-004~SUP-007 在可达环境跑通并回填 M-013~M-016 实测 | 已完成（local-mock） | `reports/supply_gate_review_2026-03-31.md`（有条件通过） |
| V-06 | token 运行态实现审计（后端实现/部署工件/鉴权链路） | 未完成 | `reports/token_runtime_implementation_gap_review_2026-03-27.md` |

---

## 5. 建议决策

1. 当前状态仅可进入“设计冻结 + 治理机制落地”的 `CONDITIONAL GO`，不得解释为实现可发布。
2. `V-02` 完成、`V-05` 在真实 staging 复核通过、且 `V-06` 关闭前，不得签发生产 `GO`。

---

## 6. P1/P2 收敛状态补充（2026-03-27）

| 编号 | 事项 | 级别 | 状态 | 证据 |
|---|---|---|---|---|
| P1-01 | 测试追踪路径与 OpenAPI 一致化 | P1 | 已关闭 | `docs/supply_test_plan_enhanced_v1_2026-03-25.md`、`reports/supply_traceability_matrix_2026-03-25.csv` |
| P1-02 | SUP 报告实名签署与签署编号 | P1 | 未关闭（受 WG-E 阻塞） | `reports/stage_e_blocker_report_2026-03-27.md` |
| P1-03 | 全局 P0 到供应侧/平台侧映射补齐 | P1 | 已关闭 | `docs/product/global_p0_to_supply_platform_mapping_v1_2026-03-27.md` |
| P2-01 | `/supply` vs `/supplier` 命名策略与兼容方案 | P2 | 已关闭（兼容保留） | `docs/api_naming_strategy_supply_vs_supplier_v1_2026-03-27.md`、`docs/supply_api_contract_openapi_draft_v1_2026-03-25.yaml` |
