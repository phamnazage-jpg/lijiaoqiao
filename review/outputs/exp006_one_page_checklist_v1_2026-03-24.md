# EXP-006 会前一页纸 Checklist（2026-03-31）

- 用途：会中 5-10 分钟快速判断 `GO / CONDITIONAL GO / NO-GO`
- SSOT：
  - `docs/llm_gateway_subapi_evolution_plan_v4_2_2026-03-24.md`
  - `docs/acceptance_gate_single_source_v1_2026-03-18.md`
  - `review/outputs/exp006_decision_meeting_packet_v1_2026-03-24.md`

## 1. 材料齐套（缺一即不开决议）

- [ ] D-01 基线 v4.2 在场
- [ ] D-02 门禁 SSOT（v1.1）在场
- [ ] D-03 执行任务单（v1.1）在场
- [ ] D-04 设计文档（v1.1）在场
- [ ] D-05 S2 验收用例（含 CB-001~CB-004）在场
- [ ] D-06~D-09 四轮 Round 评审在场
- [ ] D-10 最终决议模板在场
- [ ] D-11 纠偏附录在场

## 2. 硬门槛（任一不通过即 NO-GO）

| 指标ID | 目标值 | 实际值 | 结论 |
|---|---|---|---|
| M-004 | `<=0.1%` |  |  |
| M-005 | `<=0.01%` |  |  |
| M-006 | `>=60%` |  |  |
| M-007 | `=100%` |  |  |
| M-008 | `>=99.9%` |  |  |
| M-013 | `=0` |  |  |
| M-014 | `=100%` |  |  |
| M-015 | `=0` |  |  |
| M-016 | `=100%` |  |  |

## 3. 凭证边界专项（任一“否”按 P0）

- [ ] 用户A仅向平台供给上游凭证，不向用户B分发
- [ ] 用户B仅使用平台凭证入站，不持有供应方上游凭证
- [ ] 错误体/报表/导出无可复用上游凭证
- [ ] 绕过平台直连上游可阻断、可告警、可追溯
- [ ] 外部 query key（含 `/v1beta/*`）全拒绝

## 4. 证据路径速查（缺任一条目不得 GO）

- [ ] `tests/compat/schema_gate_report.md`
- [ ] `tests/compat/behavior_gate_report.md`
- [ ] `tests/security/credential_boundary_regression_report.md`
- [ ] `tests/security/query_key_boundary_report.md`
- [ ] `tests/security/credential_exposure_scan_report.md`
- [ ] `reports/security/platform_credential_ingress_coverage_2026-03-26.md`
- [ ] `reports/sprint_risk_control_review_2026-03-31.md`
- [ ] `compliance/subapi_tos_assessment_2026-03-27.pdf`

## 5. 决策口径

1. 任一硬门槛失败 -> `NO-GO`。
2. 任一凭证边界项失败 -> `P0 + NO-GO + 冻结升波`。
3. 无 P0，且仅剩可接受 P1 -> 可讨论 `CONDITIONAL GO`，必须附条件清单、Owner、截止日期、验证方式。
