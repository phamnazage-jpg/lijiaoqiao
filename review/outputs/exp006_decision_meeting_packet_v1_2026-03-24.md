# EXP-006 决议会可填写包（GO/CONDITIONAL GO/NO-GO）

- 版本：v1.1
- 生成日期：2026-03-24
- 适用会议：2026-03-31 `EXP-006` 最终决议会
- 使用方式：会前准备证据，会中逐项勾选，会后归档签署
- SSOT：
  - `docs/llm_gateway_subapi_evolution_plan_v4_2_2026-03-24.md`
  - `docs/acceptance_gate_single_source_v1_2026-03-18.md`
  - `docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md`

---

## 1. 会议信息

| 项目 | 内容 |
|---|---|
| 会议名称 | EXP-006 最终决议会 |
| 会议时间 | 2026-03-31 `__ : __ - __ : __` |
| 主持人 |  |
| 记录人 |  |
| 参会角色 | 架构、安全、合规、SRE、QA、产品、管理层 |
| 关联任务 | `EXP-002` ~ `EXP-006`、`EXP-007` |

---

## 2. 会前材料清单（必须齐套）

| 编号 | 文档 | 状态（已准备/缺失） | 备注 |
|---|---|---|---|
| D-01 | `docs/llm_gateway_subapi_evolution_plan_v4_2_2026-03-24.md` |  |  |
| D-02 | `docs/acceptance_gate_single_source_v1_2026-03-18.md`（v1.1） |  |  |
| D-03 | `docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md`（v1.1） |  |  |
| D-04 | `docs/subapi_integration_compat_security_reliability_design_v1_2026-03-17.md`（v1.1） |  |  |
| D-05 | `docs/router_core_s2_acceptance_test_cases_v1_2026-03-17.md`（v1.1） |  |  |
| D-06 | `review/rounds/round1_architecture_review.md` |  |  |
| D-07 | `review/rounds/round2_compat_billing_review.md` |  |  |
| D-08 | `review/rounds/round3_security_compliance_review.md` |  |  |
| D-09 | `review/rounds/round4_reliability_wargame_review.md` |  |  |
| D-10 | `review/final_decision_2026-03-31.md` |  |  |
| D-11 | `review/comprehensive_review_report_v3_1_addendum_2026-03-24.md` |  |  |

规则：
1. 任一 `D-01~D-10` 缺失，会议只允许开“补件会”，不得出 GO 结论。

---

## 3. 硬门槛核对单（会议主表）

> 结论规则：任一项“不通过”即 `NO-GO`；凭证边界项（M-013~M-016）任一不通过按 `P0` 处理。

| 指标ID | 指标名 | 目标值 | 实际值 | 结论（通过/不通过） | 证据路径 | 核对人 |
|---|---|---|---|---|---|---|
| M-004 | billing_error_rate_pct | <=0.1% |  |  |  |  |
| M-005 | billing_conflict_rate_pct | <=0.01% |  |  |  |  |
| M-006 | overall_takeover_pct | >=60% |  |  |  |  |
| M-007 | cn_takeover_pct | =100% |  |  |  |  |
| M-008 | route_mark_coverage_pct | >=99.9% |  |  |  |  |
| M-013 | supplier_credential_exposure_events | =0 |  |  |  |  |
| M-014 | platform_credential_ingress_coverage_pct | =100% |  |  |  |  |
| M-015 | direct_supplier_call_by_consumer_events | =0 |  |  |  |  |
| M-016 | query_key_external_reject_rate_pct | =100% |  |  |  |  |

---

## 4. 凭证边界专项核对（必填）

| 核对项 | 预期 | 结果（是/否） | 证据路径 | 备注 |
|---|---|---|---|---|
| 需求方仅使用平台凭证入站 | 是 |  |  |  |
| 错误体/报表/导出无可复用上游凭证 | 是 |  |  |  |
| 需求方绕过平台直连供应方被阻断并告警 | 是 |  |  |  |
| 外部 query key 全拒绝（含 `/v1beta/*`） | 是 |  |  |  |

任一“否”即动作：
1. 标记 `P0`。
2. 立即冻结升波。
3. 转入整改闭环，不得给 GO。

---

## 5. Round 闭环核对

| Round | 必须关闭项 | 当前状态（已关闭/未关闭） | 证据路径 | 风险等级 |
|---|---|---|---|---|
| Round-1 | R1-ISSUE-001~006 |  |  |  |
| Round-2 | R2-COMP-001~007, R2-BILL-001~004 |  |  |  |
| Round-3 | R3-SEC-001~008 |  |  |  |
| Round-4 | R4-REL-001~004 |  |  |  |

判定：
1. 仍有 P0 未关闭 -> `NO-GO`。
2. 无 P0 且仅 P1 可接受 -> 进入 `CONDITIONAL GO` 讨论。

---

## 6. 证据包目录核对（可复跑）

| 编号 | 证据类别 | 必要性 | 路径 | 已提供（是/否） |
|---|---|---|---|---|
| E-01 | Gate 报告（Schema/Behavior/Performance） | 必需 | `tests/compat/schema_gate_report.md`<br>`tests/compat/behavior_gate_report.md`<br>`tests/compat/stream_failover_stress_report.md` |  |
| E-02 | 凭证边界回归报告（CB-001~CB-004） | 必需 | `tests/security/credential_boundary_regression_report.md`<br>`tests/security/query_key_boundary_report.md` |  |
| E-03 | 安全扫描报告（凭证泄露/脱敏） | 必需 | `tests/security/credential_exposure_scan_report.md` |  |
| E-04 | 出网阻断与告警记录 | 必需 | `docs/security/direct_supplier_call_detection_v1.md`<br>`evidence/2026-03-31-risk-control/security-scans/` |  |
| E-05 | 波次指标快照（M-004~M-016） | 必需 | `reports/security/platform_credential_ingress_coverage_2026-03-26.md`<br>`evidence/2026-03-31-risk-control/dashboards/` |  |
| E-06 | 回滚演练与复盘报告 | 必需 | `scripts/release/rollback_subapi.sh`<br>`evidence/2026-03-31-risk-control/rollback-drill/`<br>`reports/sprint_risk_control_review_2026-03-31.md` |  |
| E-07 | 法务结论与风险留档 | 必需 | `compliance/subapi_tos_assessment_2026-03-27.pdf` |  |

路径口径规则：
1. `tests/*`、`reports/*`、`docs/*` 为任务产物主路径（以任务单为准）。
2. `evidence/2026-03-31-risk-control/*` 为会前归档路径（用于会审复盘与留痕）。
3. 同一证据允许“双路径并存”，但内容哈希必须一致。

---

## 7. 会议决策区（现场填写）

### 7.1 决策结论

- [ ] GO
- [ ] CONDITIONAL GO
- [ ] NO-GO

### 7.2 决策依据摘要

1.  
2.  
3.  

### 7.3 若为 CONDITIONAL GO：附条件清单

| 编号 | 条件 | Owner | 截止日期 | 验证方式 |
|---|---|---|---|---|
| C-01 |  |  |  |  |
| C-02 |  |  |  |  |
| C-03 |  |  |  |  |

---

## 8. 风险接受记录（仅限非P0）

| 编号 | 风险描述 | 级别 | 接受人 | 接受日期 | 依据文档 |
|---|---|---|---|---|---|
| R-01 |  |  |  |  |  |
| R-02 |  |  |  |  |  |

规则：
1. `P0` 不允许风险接受。
2. `P1` 风险接受必须有补救计划与验证时间。

---

## 9. 签署区

1. 架构负责人（签名/日期）：
2. 安全负责人（签名/日期）：
3. 合规负责人（签名/日期）：
4. SRE 负责人（签名/日期）：
5. QA 负责人（签名/日期）：
6. 管理层代表（签名/日期）：

---

## 10. 会后动作清单

| 编号 | 动作 | Owner | 截止日期 | 状态 |
|---|---|---|---|---|
| A-01 | 将会议结论回填 `review/final_decision_2026-03-31.md` | 记录人 | 当日 |  |
| A-02 | 若为 CONDITIONAL GO，创建条件项跟踪任务 | PMO | +1天 |  |
| A-03 | 若为 NO-GO，发布整改计划与重审日期 | ARCH + PMO | +1天 |  |
