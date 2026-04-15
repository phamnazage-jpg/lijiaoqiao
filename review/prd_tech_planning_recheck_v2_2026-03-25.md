> 归档状态：已归档
> 归档批次：ARCHIVE-HISTORY-2026-04-14-A01
> 归档标识：AHP-20260414-029
> 当前用途：历史快照，仅供追溯；不得作为现行门禁、发布决议或实现状态事实源。
> 当前事实源：`docs/plans/2026-04-14-repo-integrity-refactor-plan.md`、`reports/archive/gate_verification/` 与仍在维护的签署/门禁文档。

# PRD 与技术规划二次复检报告（2026-03-25）

- 复检版本：v2.0
- 对应报告：`review/prd_tech_planning_expert_review_v1_2026-03-24.md`
- 复检目标：验证“不符合最佳实践”和“未闭环功能”缺口是否补齐

---

## 1. 复检结论

结论：**主要缺口已补齐，进入 CONDITIONAL GO（文档与流程层）**

说明：
1. 已从“流程级”下钻到“按钮级 + 接口级 + 用例级 + 任务级”。
2. 供应侧链路已并入门禁任务体系（`SUP-*`）。
3. 仍需在执行期提交真实运行证据，才能转为 GO。

---

## 2. 缺口关闭状态

| 缺口ID | 问题 | 当前状态 | 证据 |
|---|---|---|---|
| GAP-01 | 供应侧参数多口径（60% vs 85%） | 已关闭 | `docs/supply_side_product_design_v1_2026-03-18.md`（4.2 与 SD1/SD2 已对齐 60%、15-50%）；`docs/llm_gateway_subapi_evolution_plan_v4_2_2026-03-24.md` 参数冻结补充 |
| GAP-02 | 供应侧 SQL 错误与方言混用 | 已关闭（执行口径） | 新增 `sql/postgresql/supply_schema_v1.sql`；`docs/supply_detailed_design_v1_2026-03-18.md` 已改为“执行脚本唯一来源 + 修正统计 SQL 示例” |
| GAP-03 | PRD 仍为评审稿未冻结 | 已关闭 | 新增 `docs/llm_gateway_prd_v1_2026-03-25.md`（冻结稿）；`docs/llm_gateway_prd_v0_2026-03-16.md` 已标注历史 |
| GAP-04 | 架构双基线冲突 | 已关闭 | `docs/technical_architecture_design_v1_2026-03-18.md` 已标注历史；`docs/technical_architecture_optimized_v2_2026-03-18.md` 已指向 v4.2 |
| GAP-05 | 功能未细化到按钮级 | 已关闭 | `docs/supply_button_level_prd_v1_2026-03-25.md` |
| GAP-06 | 接口字段未锁定 | 已关闭 | `docs/supply_api_contract_openapi_draft_v1_2026-03-25.yaml` |
| GAP-07 | UI-SUP 用例不可执行 | 已关闭 | `docs/supply_ui_test_cases_executable_v1_2026-03-25.md` |
| GAP-08 | SUP 流程未并入发布门禁 | 已关闭 | `docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md` v1.3（新增 4.7 Workstream G） |
| GAP-09 | SUP 证据路径未落盘 | 已关闭（模板级） | `tests/supply/*.md`、`reports/supply_gate_review_2026-03-31.md` 已创建 |
| GAP-10 | 缺少命令级执行手册 | 已关闭 | `docs/supply_gate_command_playbook_v1_2026-03-25.md` + `scripts/supply-gate/*.sh` |

---

## 3. 再次检查结果（自动扫描）

1. `supply_side_product_design` 中已不再出现旧口径 `0.85` 与 `15-30%`（定价参数与待决策项已修正）。
2. `supply_detailed_design` 中已消除 `su.*` 别名错误和错误关联条件。
3. 供应侧执行 DDL 已落到 PostgreSQL 脚本：`sql/postgresql/supply_schema_v1.sql`。
4. PRD v1 冻结稿已生成，v0 已标注历史。
5. 架构旧稿已标注“历史草稿”，优化稿已切换到 v4.2 基线引用。

---

## 4. 残余风险（执行期）

1. 本次补齐主要是文档与模板层，尚未包含真实联调结果。
2. `SUP-004~SUP-007` 的执行报告当前是模板，需填充真实日志与指标。
3. 历史文档中仍保留旧方言示例，但均已标记为非实施基线；执行必须以 SSOT 和 PostgreSQL 脚本为准。

---

## 5. 进入 GO 前的最小动作

1. 跑完 `UI-SUP-ACC/PKG/SET` 全量用例并回填报告。
2. 跑完 `SEC-SUP-001/002`，确认 M-013~M-016 实测达标。
3. 在 `reports/supply_gate_review_2026-03-31.md` 完成签署并回填结论。
