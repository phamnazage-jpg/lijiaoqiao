# 专家最终决议（2026-03-31）

- 对应任务：`EXP-006`
- 关联材料：
  - `docs/llm_gateway_subapi_evolution_plan_v4_2_2026-03-24.md`
  - `docs/acceptance_gate_single_source_v1_2026-03-18.md`（v1.2）
  - `review/outputs/exp006_decision_meeting_packet_v1_2026-03-24.md`（会前包）
  - `review/prd_tech_planning_recheck_v3_2026-03-27.md`
  - `reports/supply_gate_review_2026-03-31.md`
  - `reports/token_runtime_implementation_gap_review_2026-03-30.md`

## 1. 会议信息

| 字段 | 内容 |
|---|---|
| 会议时间 | 2026-03-31（草案待排期） |
| 主持人 | 王磊（ARCH，待确认） |
| 记录人 | PMO（待确认） |
| 参会角色 | 架构、安全、合规、SRE、QA、产品、管理层 |
| 会议纪要路径 | `review/outputs/` |

## 2. 总体结论

- [ ] GO
- [x] CONDITIONAL GO
- [ ] NO-GO

决议依据摘要：
1. SUP-004~SUP-007 已在 local-mock 环境通过，但 staging 真实环境尚未复核通过。
2. M-013~M-016 已有 mock 实测值，但生产门槛要求的 staging/线上口径数据仍缺失。
3. M-006~M-008 与连续 7 天门禁趋势证据未齐，尚不满足生产发布条件。
4. token 运行态已在开发阶段收敛并通过 M-021 检查，但真实 staging 证据仍未闭环，不具备生产放行前提。

## 3. 评分结果汇总

| 维度 | 得分 | 备注 |
|---|---:|---|
| 兼容性 | 74 | 审计机制补齐，mock 链路通过，staging 待复核 |
| 安全性 | 68 | M-013~M-016 在 mock 通过，生产口径待补 |
| 可靠性 | 72 | 阶段回退演练通过，生产链路仍缺验证 |
| 运维简化 | 76 | 任务链路与门禁链路已打通 |
| 账务正确性 | 66 | mock 账务链路通过，生产口径待补 |
| 合规可审计 | 74 | 审计字段与数据库基线已补齐 |
| 总分 | 72 | 低于生产发布建议阈值（80） |

## 4. 硬门槛核对（含凭证边界）

| 指标ID | 指标名 | 目标值 | 开发阶段口径 | staging/生产口径 | 结论（通过/不通过） | 证据路径 | 核对人 |
|---|---|---|---|---|---|---|---|
| M-004 | billing_error_rate_pct | <=0.1% | 0（mock） | 待实测 | 有条件通过 | `tests/supply/ui_sup_set_report_2026-03-29.md` | 何静（FIN） |
| M-005 | billing_conflict_rate_pct | <=0.01% | 0（mock） | 待实测 | 有条件通过 | `tests/supply/ui_sup_set_report_2026-03-29.md` | 何静（FIN） |
| M-006 | overall_takeover_pct | >=60% | N/A | 待实测 | 不通过 | 待运行验收SQL | 王磊（ARCH） |
| M-007 | cn_takeover_pct | =100% | N/A | 待实测 | 不通过 | 待运行验收SQL | 王磊（ARCH） |
| M-008 | route_mark_coverage_pct | >=99.9% | N/A | 待实测 | 不通过 | 待运行验收SQL | 王磊（ARCH） |
| M-013 | supplier_credential_exposure_events | =0 | 0（mock） | 待实测 | 有条件通过 | `tests/supply/sec_sup_boundary_report_2026-03-30.md` | 周敏（SEC） |
| M-014 | platform_credential_ingress_coverage_pct | =100% | 100%（mock） | 待实测 | 有条件通过 | `tests/supply/sec_sup_boundary_report_2026-03-30.md` | 周敏（SEC） |
| M-015 | direct_supplier_call_by_consumer_events | =0 | 0（mock） | 待实测 | 有条件通过 | `tests/supply/sec_sup_boundary_report_2026-03-30.md` | 周敏（SEC） |
| M-016 | query_key_external_reject_rate_pct | =100% | 100%（mock） | 待实测 | 有条件通过 | `tests/supply/sec_sup_boundary_report_2026-03-30.md` | 周敏（SEC） |
| M-017 | dependency_compat_audit_pass_pct | =100% | 100% | 待连续7天观测 | 通过 | `reports/dependency/dependency_audit_result_2026-03-27.md` | 李娜（PLAT） |
| M-018 | stage_quality_gate_pass_pct | =100% | 演练通过 | 待连续7天观测 | 有条件通过 | `reports/gates/stage_gate_drift_drill_report_2026-03-27.md` | 孙悦（QA） |
| M-019 | requirement_traceability_coverage_pct | =100% | 进行中 | 待连续7天观测 | 不通过 | `reports/supply_traceability_matrix_2026-03-25.csv` | 孙悦（QA） |
| M-021 | token_runtime_readiness_pct | =100% | 100%（开发阶段） | 待实测 | 有条件通过 | `reports/gates/token_runtime_readiness_*.md` + `reports/token_runtime_implementation_gap_review_2026-03-30.md` | 王磊（ARCH）+周敏（SEC） |

判定规则：
1. 任一硬门槛不满足，默认 `NO-GO`。
2. 任一凭证边界指标（M-013~M-016）不满足，按 `P0` 处理并冻结升波。

## 5. Round 闭环核对

| Round | 必须关闭项 | 状态（已关闭/未关闭） | 证据路径 |
|---|---|---|---|
| Round-1 | R1-ISSUE-001~006 | 未关闭 | `review/rounds/round1_architecture_review.md` |
| Round-2 | R2-COMP-001~007, R2-BILL-001~004 | 未关闭 | `review/rounds/round2_compat_billing_review.md` |
| Round-3 | R3-SEC-001~008 | 未关闭 | `review/rounds/round3_security_compliance_review.md` |
| Round-4 | R4-REL-001~004 | 未关闭 | `review/rounds/round4_reliability_wargame_review.md` |

## 6. 必须整改项（若有）

| 编号 | 等级（P0/P1/P2） | 描述 | Owner | 截止日期 | 验证方式 |
|---|---|---|---|---|---|
| F-01 | P0 | 在真实 staging 环境修复 DNS 与 `API_BASE_URL` 可达性，重跑 SUP-004~SUP-007 | 李娜（PLAT）+孙悦（QA） | 2026-04-01 | `scripts/supply-gate/run_all.sh` 全绿（staging） |
| F-02 | P0 | 补齐 M-013~M-016 的 staging 实测值并达标 | 周敏（SEC）+孙悦（QA） | 2026-04-01 | `tests/supply/sec_sup_boundary_report_2026-03-30.md` 更新为 staging PASS |
| F-03 | P1 | 补齐 M-017/M-018/M-019 连续7天趋势证据 | 李娜（PLAT）+PMO | 2026-04-05 | `reports/dependency/` 与 `reports/gates/` 趋势报告 |
| F-04 | P0 | 完成 token 运行态 staging 联调取证（含审计查询与边界指标） | 王磊（ARCH）+李娜（PLAT）+周敏（SEC） | 2026-04-03 | staging 端到端验收日志 + `M-013~M-016/M-021` 实测回填 |

## 7. 条件放行项（仅当 CONDITIONAL GO）

| 编号 | 条件 | Owner | 截止日期 | 追踪路径 |
|---|---|---|---|---|
| C-01 | 完成 F-01/F-02/F-04 后，可申请“预发布环境 CONDITIONAL GO”复审 | ARCH + QA + SEC | 2026-04-03 | `reports/supply_gate_review_2026-03-31.md` |
| C-02 | 连续7天指标达标后，可申请“生产 GO”复审 | ARCH + PMO | 2026-04-05 | `review/prd_tech_planning_recheck_v3_2026-03-27.md` |

## 8. 风险接受记录（仅限非P0）

| 编号 | 风险 | 等级 | 接受人 | 日期 | 依据 |
|---|---|---|---|---|---|

规则：
1. `P0` 不允许风险接受。
2. `P1` 风险接受必须绑定整改计划与验证时间。

## 9. 会后动作清单

| 编号 | 动作 | Owner | 截止日期 | 状态 |
|---|---|---|---|---|
| A-01 | 回填会前包 `review/outputs/exp006_decision_meeting_packet_v1_2026-03-24.md` 的现场结论 | PMO | 当日 | 待执行 |
| A-02 | 创建 F-01~F-04 跟踪任务并纳入日报 | PMO | +1天 | 待执行 |
| A-03 | 发布整改计划与重审日期（生产 NO-GO） | 王磊（ARCH）+PMO | +1天 | 待执行 |

## 10. 决议签署

1. 架构负责人（签名/日期）：王磊 / 待签
2. 安全负责人（签名/日期）：周敏 / 待签
3. 合规负责人（签名/日期）：待指派 / 待签
4. SRE 负责人（签名/日期）：刘洋 / 待签
5. QA 负责人（签名/日期）：孙悦 / 待签
6. 管理层代表（签名/日期）：待指派 / 待签

## 附录：TOK-007 自动复审回填（2026-03-30_184436）

1. 自动复审来源：`/home/long/project/立交桥/review/outputs/tok007_release_recheck_2026-03-30_184436.md`
2. 自动复审结论：`CONDITIONAL_GO`
3. 说明：该候选稿用于人工审阅与签署准备，不直接替代正式签署版本。
