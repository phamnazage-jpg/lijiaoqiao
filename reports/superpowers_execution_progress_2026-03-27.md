> 归档状态：已归档
> 归档批次：ARCHIVE-HISTORY-2026-04-14-A01
> 归档标识：AHP-20260414-012
> 当前用途：历史快照，仅供追溯；不得作为现行门禁、发布决议或实现状态事实源。
> 当前事实源：`docs/plans/2026-04-14-repo-integrity-refactor-plan.md`、`reports/archive/gate_verification/` 与仍在维护的签署/门禁文档。

# Superpowers 执行进度台账（2026-03-27）

- 执行规范：`superpowers + executing-plans`
- 当前批次：Checkpoint-01（前 10 个子任务）
- 对应清单：`docs/plans/2026-03-25-superpowers-execution-tasklist-v1.md`

## 1. 批次执行结果（1/10 ~ 10/10）

| 序号 | Step ID | 状态 | 输出产物 | 证据 |
|---|---|---|---|---|
| 1 | A-001 | 完成 | 草案标记定位记录 | 执行前快照：`review/superpowers_comprehensive_planning_review_v1_2026-03-25.md:49` |
| 2 | A-002 | 完成 | 待拍板项提取记录（4条） | 执行前快照：`review/superpowers_comprehensive_planning_review_v1_2026-03-25.md:50` |
| 3 | A-003 | 完成 | 待拍板->决议映射表 | `docs/product/supply_prd_pending_to_decision_map_v1_2026-03-27.md` |
| 4 | A-004 | 完成 | 决议会纪要 | `review/outputs/supply_prd_decision_meeting_minutes_2026-03-27.md` |
| 5 | A-005 | 完成 | 按钮 PRD 状态变更为冻结 | `docs/supply_button_level_prd_v1_2026-03-25.md:3` |
| 6 | A-006 | 完成 | “待拍板项”替换为“已决议项” | `docs/supply_button_level_prd_v1_2026-03-25.md:236` |
| 7 | A-007 | 完成 | 任务单引用链更新为冻结状态 | `docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md:14` |
| 8 | A-008 | 完成 | 复核报告 P0-01 标注 Closed | `review/superpowers_comprehensive_planning_review_v1_2026-03-25.md:53` |
| 9 | B-001 | 完成 | `X-Request-Id` 参数组件 | `docs/supply_api_contract_openapi_draft_v1_2026-03-25.yaml:431` |
| 10 | B-002 | 完成 | `Idempotency-Key` 参数组件 | `docs/supply_api_contract_openapi_draft_v1_2026-03-25.yaml:440` |

## 2. 快速自检

1. 按钮 PRD 不再出现“草案”标记，存在“已决议项”章节。
2. P0-01 已在 superpowers 评审报告中闭环标注。
3. OpenAPI 已具备两类幂等 header 参数定义（路径挂载在下一批次 B-003~B-007 完成）。

## 3. 下一批次范围

1. B-003 ~ B-010（路径挂载 + 409/202 示例 + lint）。
2. 完成后触发 Checkpoint-02 对齐验证。

---

## 4. 批次执行结果（11/20 ~ 20/20）

| 序号 | Step ID | 状态 | 输出产物 | 证据 |
|---|---|---|---|---|
| 11 | B-003 | 完成 | `POST /api/v1/supply/accounts` 挂载双幂等头 | `docs/supply_api_contract_openapi_draft_v1_2026-03-25.yaml:46` |
| 12 | B-004 | 完成 | `POST /api/v1/supply/packages/{packageId}/publish` 挂载双幂等头 | `docs/supply_api_contract_openapi_draft_v1_2026-03-25.yaml:178` |
| 13 | B-005 | 完成 | `POST /api/v1/supply/packages/batch-price` 挂载双幂等头 | `docs/supply_api_contract_openapi_draft_v1_2026-03-25.yaml:242` |
| 14 | B-006 | 完成 | `POST /api/v1/supply/settlements/withdraw` 挂载双幂等头 | `docs/supply_api_contract_openapi_draft_v1_2026-03-25.yaml:310` |
| 15 | B-007 | 完成 | `POST /api/v1/supply/settlements/{settlementId}/cancel` 挂载双幂等头 | `docs/supply_api_contract_openapi_draft_v1_2026-03-25.yaml:339` |
| 16 | B-008 | 完成 | 409 幂等冲突示例（payload mismatch） | `docs/supply_api_contract_openapi_draft_v1_2026-03-25.yaml:500` |
| 17 | B-009 | 完成 | 202 处理中重放示例（retry_after_ms） | `docs/supply_api_contract_openapi_draft_v1_2026-03-25.yaml:510` |
| 18 | B-010 | 完成 | OpenAPI lint（YAML 解析校验） | 命令输出：`openapi_yaml_parse: PASS` |
| 19 | B-011 | 完成 | 技术增强稿标注“契约已落地” | `docs/supply_technical_design_enhanced_v1_2026-03-25.md:42` |
| 20 | B-012 | 完成 | 复核报告 P0-02 标注 Closed | `review/superpowers_comprehensive_planning_review_v1_2026-03-25.md:66` |

## 5. 下一批次范围

1. C-001 ~ C-008（测试路径一致化 + 追踪矩阵 + XR-002 验收项更新）。
2. 完成后触发 Checkpoint-03 对齐验证。

---

## 6. 独立阶段执行结果（WG-C，21/28 ~ 28/28）

| 序号 | Step ID | 状态 | 输出产物 | 证据 |
|---|---|---|---|---|
| 21 | C-001 | 完成 | 路径偏差提取（主要为 `{id}` 泛化参数） | `docs/supply_test_plan_enhanced_v1_2026-03-25.md:42`、`:45`、`:48` |
| 22 | C-002 | 完成 | accounts 路径参数改为 `accountId` | `docs/supply_test_plan_enhanced_v1_2026-03-25.md:42`、`:43` |
| 23 | C-003 | 完成 | packages 路径参数改为 `packageId` | `docs/supply_test_plan_enhanced_v1_2026-03-25.md:45` |
| 24 | C-004 | 完成 | settlements 路径参数改为 `settlementId` | `docs/supply_test_plan_enhanced_v1_2026-03-25.md:48`、`:49` |
| 25 | C-005 | 完成 | 追踪矩阵新增 `api_alias` 列 | `docs/supply_test_plan_enhanced_v1_2026-03-25.md:38` |
| 26 | C-006 | 完成 | CSV 同步 `api/api_alias` 双列口径 | `reports/supply_traceability_matrix_2026-03-25.csv:1` |
| 27 | C-007 | 完成 | 追踪矩阵生成规则文档 | `docs/supply_traceability_matrix_generation_rules_v1_2026-03-27.md` |
| 28 | C-008 | 完成 | XR-002 验收项新增“路径一致性检查” | `docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md:158` |

## 7. 下一阶段范围

1. D-001 ~ D-018（真实环境与联调证据）。
2. 若 D 阶段遇到环境或凭证阻塞，按 P0 阻塞项生成解锁清单并继续推进不依赖环境的后续项。

---

## 8. 阶段执行状态（WG-D）

| 阶段 | 状态 | 结论 | 证据 |
|---|---|---|---|
| WG-D（D-001~D-018） | DEFERRED | 开发实施阶段暂缓，待进入联调阶段后激活 | `reports/stage_d_blocker_report_2026-03-27.md` |

阻塞触发输出：
1. `[FAIL] placeholder token detected; please fill real short-lived token`
2. `RESULT:FAIL`

---

## 9. 阶段执行状态（WG-E）

| 阶段 | 状态 | 结论 | 证据 |
|---|---|---|---|
| WG-E（E-001~E-010） | DEFERRED | 随 WG-D 联调窗口一并暂缓，待真实证据阶段再执行 | `reports/stage_e_blocker_report_2026-03-27.md` |

---

## 10. 批次执行结果（F/G，39/48 ~ 48/48）

| 序号 | Step ID | 状态 | 输出产物 | 证据 |
|---|---|---|---|---|
| 39 | F-001 | 完成 | 全局 P0 -> 供应侧/平台侧映射表 | `docs/product/global_p0_to_supply_platform_mapping_v1_2026-03-27.md:1` |
| 40 | F-002 | 完成 | 预算/告警/账单导出入口映射补齐 | `docs/product/global_p0_to_supply_platform_mapping_v1_2026-03-27.md:13` |
| 41 | F-003 | 完成 | 映射项并入追踪矩阵（`R-PLAT-001~003`） | `reports/supply_traceability_matrix_2026-03-25.csv` |
| 42 | F-004 | 完成 | `/supply` vs `/supplier` 命名策略文档 | `docs/api_naming_strategy_supply_vs_supplier_v1_2026-03-27.md:1` |
| 43 | F-005 | 完成 | OpenAPI 增加 canonical + alias 兼容路径 | `docs/supply_api_contract_openapi_draft_v1_2026-03-25.yaml:293`、`:320` |
| 44 | F-006 | 完成 | OpenAPI 变更日志与注释更新 | `docs/supply_api_contract_openapi_draft_v1_2026-03-25.yaml:10` |
| 45 | F-007 | 完成 | 复核报告追加 P1/P2 收敛状态 | `review/prd_tech_planning_recheck_v3_2026-03-27.md:66` |
| 46 | G-001 | 完成 | 跨文档链接完整性检查报告 | `reports/link_integrity_check_2026-03-27.md:1` |
| 47 | G-002 | 完成 | 门禁指标一致性检查报告 | `reports/gate_metrics_consistency_check_2026-03-27.md:1` |
| 48 | G-003 | 完成 | 最终决议稿（Draft v2） | `review/final_decision_draft_v2_2026-03-27.md:1` |

---

## 11. 开发阶段补充执行（2026-03-27）

| 项目 | 状态 | 说明 | 证据 |
|---|---|---|---|
| WG-D 状态修订 | 完成 | 由 BLOCKED 调整为开发阶段暂缓（Deferred） | `reports/stage_d_blocker_report_2026-03-27.md` |
| WG-E 状态修订 | 完成 | 由 BLOCKED 调整为开发阶段暂缓（Deferred） | `reports/stage_e_blocker_report_2026-03-27.md` |
| TOK-001 最小规格 | 完成 | 新增 token 运行态最小实现规格 | `docs/token_runtime_minimal_spec_v1.md` |

---

## 12. TOK 阶段执行结果（2026-03-29）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| TOK-002 设计 | 完成 | 鉴权与 token 校验中间件设计 | `docs/token_auth_middleware_design_v1_2026-03-29.md` |
| TOK-002 契约 | 完成 | 平台 token OpenAPI 草案 | `docs/platform_token_api_contract_openapi_draft_v1_2026-03-29.yaml` |
| TOK-003/004 断言 | 完成 | 生命周期+审计事件测试断言清单 | `docs/token_lifecycle_audit_test_assertions_v1_2026-03-29.md` |
| TOK 任务链路回填 | 完成 | 任务单增加开发阶段证据口径 | `docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md` |
| 阶段对齐验证 | 完成 | Checkpoint-08 报告 | `reports/alignment_validation_checkpoint_08_2026-03-29.md` |

---

## 13. TOK 开发骨架执行结果（2026-03-29）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| TOK-002 代码骨架 | 完成 | 中间件链路骨架（request_id/query_key/bearer/status/scope/audit） | `platform-token-runtime/internal/auth/middleware/token_auth_middleware.go` |
| TOK-002 外拒骨架 | 完成 | query key 外拒中间件 | `platform-token-runtime/internal/auth/middleware/query_key_reject_middleware.go` |
| TOK-002 单测骨架 | 完成 | 鉴权路径与外拒路径测试骨架 | `platform-token-runtime/internal/auth/middleware/token_auth_middleware_test.go` |
| TOK-003 模板 | 完成 | 生命周期 `TOK-LIFE-001~008` 模板 | `platform-token-runtime/internal/token/lifecycle_test_template_test.go` |
| TOK-004 模板 | 完成 | 审计事件 `TOK-AUD-001~007` 模板 | `platform-token-runtime/internal/token/audit_test_template_test.go` |
| 阶段对齐验证 | 完成 | Checkpoint-09 报告 | `reports/alignment_validation_checkpoint_09_2026-03-29.md` |

---

## 14. TOK 最小实现推进结果（2026-03-29）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| TOK-002 运行时实现 | 完成 | 内存版 TokenVerifier/StatusResolver/RouteAuthorizer | `platform-token-runtime/internal/auth/service/inmemory_runtime.go` |
| TOK-003 可执行化（部分） | 完成 | `TOK-LIFE-001/004/005/008` 可执行测试 | `platform-token-runtime/internal/token/lifecycle_executable_test.go` |
| TOK-004 可执行化（部分） | 完成 | `TOK-AUD-003/004/006` 可执行测试 | `platform-token-runtime/internal/token/audit_executable_test.go` |
| 阶段对齐验证 | 完成 | Checkpoint-10 报告 | `reports/alignment_validation_checkpoint_10_2026-03-29.md` |

---

## 15. TOK 全量可执行化结果（2026-03-29）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| Go 工具链安装 | 完成 | 项目内本地 Go 1.26.1 | `/.tools/go-current/bin/go version` |
| TOK-003 全量可执行 | 完成 | `TOK-LIFE-001~008` 执行实现（含幂等/吊销/过期） | `platform-token-runtime/internal/token/lifecycle_executable_test.go` |
| TOK-004 全量可执行 | 完成 | `TOK-AUD-001~007` 执行实现（含事件必填与不可篡改） | `platform-token-runtime/internal/token/audit_executable_test.go` |
| Idempotency 语义实现 | 完成 | 同键重放返回同 token，冲突载荷拒绝 | `platform-token-runtime/internal/auth/service/inmemory_runtime.go` |
| 本地测试验证 | 完成 | `go test ./...` 全通过 | `platform-token-runtime` 测试输出 |
| 阶段对齐验证 | 完成 | Checkpoint-11 报告 | `reports/alignment_validation_checkpoint_11_2026-03-29.md` |

---

## 16. TOK-005 Dry-Run 门禁并入（2026-03-30）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| TOK-005 干跑脚本 | 完成 | 凭证边界 dry-run 执行脚本 | `scripts/supply-gate/tok005_boundary_dryrun.sh` |
| staging 预检接入 | 完成 | 预检脚本接入 TOK-005 dry-run（可开关） | `scripts/supply-gate/staging_precheck_and_run.sh` |
| Dry-run 执行证据 | 完成 | 门禁报告 + 原始日志 + go test 输出 | `reports/gates/tok005_dryrun_2026-03-30_090146.md` + `tests/supply/artifacts/tok005_dryrun_2026-03-30_090146/go_test_output.txt` |
| 命令手册更新 | 完成 | 新增 TOK-005 干跑执行章节 | `docs/supply_gate_command_playbook_v1_2026-03-25.md` |
| 任务单证据口径更新 | 完成 | TOK-005 增加开发阶段证据路径 | `docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md` |
| 阶段对齐验证 | 完成 | Checkpoint-12 报告 | `reports/alignment_validation_checkpoint_12_2026-03-30.md` |

---

## 17. TOK-006 统一 Gate 汇总落地（2026-03-30）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| TOK-006 汇总脚本 | 完成 | 统一汇总 TOK-005 + SUP-004~007 并输出单页判定 | `scripts/supply-gate/tok006_gate_bundle.sh` |
| TOK-006 实跑证据 | 完成 | 汇总报告 + 原始日志（本轮结论 CONDITIONAL_GO） | `reports/gates/tok006_gate_bundle_2026-03-30_091849.md` + `.log` |
| 单页判定模板 | 完成 | 发布判定 one-pager 模板 | `reports/gates/tok006_release_decision_onepager_template_v1_2026-03-30.md` |
| 命令手册更新 | 完成 | 增加 TOK-006 执行章节 | `docs/supply_gate_command_playbook_v1_2026-03-25.md` |
| 任务单证据口径更新 | 完成 | TOK-006 增加开发阶段证据路径 | `docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md` |
| 阶段对齐验证 | 完成 | Checkpoint-13 报告 | `reports/alignment_validation_checkpoint_13_2026-03-30.md` |

---

## 18. Superpowers 严格阶段验证执行（2026-03-30）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| 分阶段验证脚本 | 完成 | 统一阶段验证脚本（PHASE-01~09） | `scripts/ci/superpowers_stage_validate.sh` |
| 阶段实跑证据 | 完成 | 阶段验证报告与分阶段日志 | `reports/gates/superpowers_stage_validation_2026-03-30_120619.md` + `tests/supply/artifacts/superpowers_stage_validation_2026-03-30_120619/phase*.log` |
| 结果判定 | 完成 | 当前结论 `CONDITIONAL_GO`（staging 阶段 DEFERRED） | `reports/gates/superpowers_stage_validation_2026-03-30_120619.md` |
| 命令手册更新 | 完成 | 增加 Superpowers 严格分阶段验证章节 | `docs/supply_gate_command_playbook_v1_2026-03-25.md` |
| 任务单证据口径更新 | 完成 | TOK-006 增加 superpowers 阶段验证证据路径 | `docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md` |
| 阶段对齐验证 | 完成 | Checkpoint-14 报告 | `reports/alignment_validation_checkpoint_14_2026-03-30.md` |

---

## 19. TOK-007 复审自动化执行（2026-03-30）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| TOK-007 复审脚本 | 完成 | 复审自动化汇总脚本 | `scripts/ci/tok007_release_recheck.sh` |
| TOK-007 实跑证据 | 完成 | 复审报告 + 执行日志 | `review/outputs/tok007_release_recheck_2026-03-30_121727.md` + `reports/gates/tok007_release_recheck_2026-03-30_121727.log` |
| 命令手册更新 | 完成 | 增加 TOK-007 执行章节 | `docs/supply_gate_command_playbook_v1_2026-03-25.md` |
| 任务单证据口径更新 | 完成 | TOK-007 增加开发阶段证据路径 | `docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md` |
| 阶段对齐验证 | 完成 | Checkpoint-15 报告 | `reports/alignment_validation_checkpoint_15_2026-03-30.md` |

---

## 20. TOK-007 决议一致性校验（2026-03-30）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| 一致性校验脚本 | 完成 | final_decision vs tok007 vs superpowers 一致性校验 | `scripts/ci/final_decision_consistency_check.sh` |
| 一致性校验实跑 | 完成 | 本轮结果 `WARN`（final=NO_GO, tok007=CONDITIONAL_GO） | `reports/gates/final_decision_consistency_2026-03-30_123320.md` |
| 命令手册更新 | 完成 | 增加一致性校验章节 | `docs/supply_gate_command_playbook_v1_2026-03-25.md` |
| 任务单证据口径更新 | 完成 | TOK-007 增加一致性校验证据路径 | `docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md` |
| 阶段对齐验证 | 完成 | Checkpoint-16 报告 | `reports/alignment_validation_checkpoint_16_2026-03-30.md` |

---

## 21. TOK-007 候选决议稿生成（2026-03-30）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| 候选稿生成脚本 | 完成 | 自动生成 final_decision 候选稿（不覆盖原件） | `scripts/ci/tok007_generate_final_decision_candidate.sh` |
| 候选稿实跑证据 | 完成 | 候选稿 + 生成日志 | `review/outputs/final_decision_candidate_from_tok007_2026-03-30_123719.md` + `reports/gates/tok007_generate_candidate_2026-03-30_123719.log` |
| 命令手册更新 | 完成 | 增加候选稿生成章节 | `docs/supply_gate_command_playbook_v1_2026-03-25.md` |
| 任务单证据口径更新 | 完成 | TOK-007 增加候选稿证据路径 | `docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md` |
| 阶段对齐验证 | 完成 | Checkpoint-17 报告 | `reports/alignment_validation_checkpoint_17_2026-03-30.md` |

---

## 22. M-017/M-018/M-019 指标修复与复跑（2026-03-30）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| M-018 统计修复 | 完成 | 修复阶段统计正则，恢复 `pass_steps/total_steps` 正确计算 | `scripts/ci/metrics_daily_snapshot.sh` |
| debug 数据隔离 | 完成 | 快照脚本自动剔除 `*-debug` 行；趋势脚本仅统计标准日期 | `scripts/ci/metrics_daily_snapshot.sh` + `scripts/ci/metrics_trend_report.sh` |
| 每日快照复跑 | 完成 | 生成修复后快照（`M-018=88.89%`） | `reports/gates/metrics_daily_snapshot_2026-03-30.md` |
| 7日趋势复跑 | 完成 | 趋势报告不再纳入 debug 行 | `reports/gates/metrics_trend_7d_2026-03-30.md` |
| Superpowers 阶段复跑 | 完成 | PHASE-08/09 均 PASS，整体 `CONDITIONAL_GO` | `reports/gates/superpowers_stage_validation_2026-03-30_154103.md` |
| TOK-007 全链复跑 | 完成 | 复审/一致性/候选稿证据链重建 | `review/outputs/tok007_release_recheck_2026-03-30_154104.md` + `reports/gates/final_decision_consistency_2026-03-30_154104.md` + `review/outputs/final_decision_candidate_from_tok007_2026-03-30_154104.md` |
| 总控流水验证 | 完成 | STEP-01~04 全部 PASS | `reports/gates/superpowers_release_pipeline_2026-03-30_154103.md` |
| 阶段对齐验证 | 完成 | Checkpoint-18 报告 | `reports/alignment_validation_checkpoint_18_2026-03-30.md` |

---

## 23. TOK-REAL-001/002/003 开发收敛与 M-021 接入（2026-03-30）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| Token HTTP 服务入口 | 完成 | 可执行服务主程序（health + token API） | `platform-token-runtime/cmd/platform-token-runtime/main.go` |
| Token API 实现 | 完成 | `issue/refresh/revoke/introspect` 路由处理 | `platform-token-runtime/internal/httpapi/token_api.go` |
| 审计查询实现 | 完成 | `audit-events` 查询接口（代码+契约） | `platform-token-runtime/internal/httpapi/token_api.go` + `docs/platform_token_api_contract_openapi_draft_v1_2026-03-29.yaml` |
| Token API 单测 | 完成 | API 级可执行测试（含幂等冲突与头校验） | `platform-token-runtime/internal/httpapi/token_api_test.go` |
| 运行态接口补齐 | 完成 | 运行时新增 `Lookup(token_id)` 能力供 API 使用 | `platform-token-runtime/internal/auth/service/inmemory_runtime.go` |
| 可部署工件 | 完成 | runtime 镜像构建工件（Dockerfile） | `platform-token-runtime/Dockerfile` |
| M-021 门禁脚本 | 完成 | Token runtime readiness 检查脚本 | `scripts/ci/token_runtime_readiness_check.sh` |
| M-021 实测结果 | 完成 | `token_runtime_readiness_pct=100%`（开发阶段口径，13项，含本地冒烟） | `reports/gates/token_runtime_readiness_2026-03-30_173728.md` |
| Superpowers 阶段验证 | 完成 | PHASE-10（M-021）通过，整体 `CONDITIONAL_GO` | `reports/gates/superpowers_stage_validation_2026-03-30_173726.md` |
| 总控流水复跑 | 完成 | STEP-01 口径为 PHASE-01~10 并 PASS | `reports/gates/superpowers_release_pipeline_2026-03-30_173726.md` |
| TOK-007 全链复跑 | 完成 | 复审/一致性/候选稿证据更新（含 M-021 复审输入） | `review/outputs/tok007_release_recheck_2026-03-30_173728.md` + `reports/gates/final_decision_consistency_2026-03-30_173728.md` + `review/outputs/final_decision_candidate_from_tok007_2026-03-30_173728.md` |
| 阶段对齐验证 | 完成 | Checkpoint-20 报告 | `reports/alignment_validation_checkpoint_20_2026-03-30.md` |

---

## 24. 联调前收口与决议口径同步（2026-03-30）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| staging 预检增强 | 完成 | `staging_precheck_and_run.sh` 增加 M-021 预检 | `scripts/supply-gate/staging_precheck_and_run.sh` |
| 联调回填模板 | 完成 | staging 证据回填模板（M-013~M-016/M-021） | `reports/gates/staging_token_go_evidence_template_v1_2026-03-30.md` |
| 命令手册更新 | 完成 | 增加 M-021 开关与审计查询执行说明 | `docs/supply_gate_command_playbook_v1_2026-03-25.md` |
| 最终决议口径更新 | 完成 | `M-021` 与 `F-04` 调整为“开发收敛+staging待验” | `review/final_decision_2026-03-31.md` |
| SUP 汇总风险口径更新 | 完成 | token 风险更新为“staging 取证缺口” | `reports/supply_gate_review_2026-03-31.md` |
| TOK 差距复审更新 | 完成 | 引用最新 M-021 和阶段报告证据 | `reports/token_runtime_implementation_gap_review_2026-03-30.md` |
| TOK-007 复审链复跑 | 完成 | 复审/一致性/候选稿证据更新 | `review/outputs/tok007_release_recheck_2026-03-30_182149.md` + `reports/gates/final_decision_consistency_2026-03-30_182149.md` + `review/outputs/final_decision_candidate_from_tok007_2026-03-30_182149.md` |
| 总控流水复跑 | 完成 | STEP-01~04 全 PASS | `reports/gates/superpowers_release_pipeline_2026-03-30_181925.md` |
| 阶段对齐验证 | 完成 | Checkpoint-21 报告 | `reports/alignment_validation_checkpoint_21_2026-03-30.md` |

---

## 25. 联调自动化补齐与双口径决议表（2026-03-30）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| 证据自动回填脚本 | 完成 | 自动抽取 `PHASE-07/M-013~M-016/M-021/TOK-007` 草稿 | `scripts/ci/staging_evidence_autofill.sh` + `reports/gates/staging_token_go_evidence_autofill_2026-03-30_182910.md` |
| 一键 staging 流水脚本 | 完成 | staging 预检 + 总控流水 + 自动回填三步串联 | `scripts/ci/staging_release_pipeline.sh` |
| PHASE-07 环境文件可配置 | 完成 | `superpowers_stage_validate.sh` 支持 `STAGING_ENV_FILE` | `scripts/ci/superpowers_stage_validate.sh` |
| final_decision 双口径表 | 完成 | 指标表新增“开发阶段口径/staging口径”双列 | `review/final_decision_2026-03-31.md` |
| 候选稿同步双口径 | 完成 | TOK-007 候选稿继承双口径字段 | `review/outputs/final_decision_candidate_from_tok007_2026-03-30_182830.md` |
| 复审链最新证据 | 完成 | TOK-007 + 一致性 + 候选稿更新 | `review/outputs/tok007_release_recheck_2026-03-30_182830.md` + `reports/gates/final_decision_consistency_2026-03-30_182830.md` + `review/outputs/final_decision_candidate_from_tok007_2026-03-30_182830.md` |
| 总控流水复跑 | 完成 | STEP-01~04 全 PASS | `reports/gates/superpowers_release_pipeline_2026-03-30_182827.md` |
| 阶段对齐验证 | 完成 | Checkpoint-22 报告 | `reports/alignment_validation_checkpoint_22_2026-03-30.md` |

---

## 26. Minimax 趋势化监控并入总控流水（2026-03-30）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| Minimax 7日趋势脚本 | 完成 | 上游趋势报告生成脚本 | `scripts/ci/minimax_upstream_trend_report.sh` |
| 趋势报告首轮产出 | 完成 | 生成趋势报告（当前 1 天样本，`INSUFFICIENT_DATA`） | `reports/gates/minimax_upstream_trend_7d_2026-03-30.md` |
| 总控流水可选监控接入 | 完成 | `superpowers_release_pipeline` 新增 `STEP-05` | `scripts/ci/superpowers_release_pipeline.sh` |
| 总控流水复跑验证 | 完成 | `STEP-01~STEP-05` 全 PASS | `reports/gates/superpowers_release_pipeline_2026-03-30_235224.md` |
| 命令手册更新 | 完成 | 增加 Minimax 7 日趋势与可选监控说明 | `docs/supply_gate_command_playbook_v1_2026-03-25.md` |
| 阶段对齐验证 | 完成 | Checkpoint-28 报告 | `reports/alignment_validation_checkpoint_28_2026-03-30.md` |

---

## 27. STG 本地批次续跑与 M-021 阻塞修复（2026-03-31）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| STG 本地续跑（首次） | 完成 | 识别 STEP-02 失败，定位到 PHASE-10 失败 | `reports/gates/staging_release_pipeline_2026-03-31_095302.md` |
| PHASE-10 根因定位 | 完成 | `18082` 被占用导致 smoke 命中错误服务，且脚本 `exit 1` 提前退出 | `reports/gates/token_runtime_smoke_2026-03-31_095638.log` |
| M-021 脚本修复 | 完成 | 端口自动避让 + smoke 子 Shell 返回码回传 | `scripts/ci/token_runtime_readiness_check.sh` |
| M-021 修复验证 | 完成 | 冒烟开启场景下就绪度恢复 100% | `reports/gates/token_runtime_readiness_2026-03-31_100017.md` |
| STG 本地续跑（复跑） | 完成 | STEP-01~03 全 PASS | `reports/gates/staging_release_pipeline_2026-03-31_100116.md` |
| Superpowers 总控复跑 | 完成 | STEP-01~05（含可选监控位）主链 PASS | `reports/gates/superpowers_release_pipeline_2026-03-31_100120.md` |
| TOK-007 复审结果 | 完成 | 机判维持 `CONDITIONAL_GO`（未误升） | `review/outputs/tok007_release_recheck_2026-03-31_100127.md` |
| 阶段对齐验证 | 完成 | Checkpoint-29 报告 | `reports/alignment_validation_checkpoint_29_2026-03-31.md` |

---

## 28. 本机冲突进程清理与端口基线固化（2026-03-31）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| 冲突进程清理 | 完成 | 清理蚊子残留与 STG 冲突进程（8080/5176/5177/18081/18082） | `reports/gates/local_dev_port_baseline_2026-03-31.md` |
| 端口基线固化 | 完成 | 输出端口基线报告与复核命令 | `reports/gates/local_dev_port_baseline_2026-03-31.md` |
| 清理后 STG 复测 | 完成 | local/mock 流水复测 PASS（STEP-01~03） | `reports/gates/staging_release_pipeline_2026-03-31_100942.md` |
| 清理后总控复测 | 完成 | Superpowers 发布流水 PASS | `reports/gates/superpowers_release_pipeline_2026-03-31_100943.md` |
| 阶段对齐验证 | 完成 | Checkpoint-30 报告 | `reports/alignment_validation_checkpoint_30_2026-03-31.md` |

---

## 29. 真实 STG 前置自动化补齐（2026-03-31）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| 本地 STG env 一键生成 | 完成 | 自动签发 owner/viewer/admin 并写入 `.env.staging-real` | `scripts/ci/generate_local_staging_env.sh` + `reports/gates/local_staging_env_generation_2026-03-31_105620.md` |
| 本地 STG env 联调验证 | 完成 | 使用 `.env.staging-real` 复跑 local/mock 流水 PASS | `reports/gates/staging_release_pipeline_2026-03-31_105633.md` |
| 真实 STG 就绪度检查脚本 | 完成 | 地址+token+可达性自动判定 | `scripts/ci/staging_real_readiness_check.sh` |
| 当前配置真实就绪判定 | 完成 | 当前仍为 `BLOCKED`（本地地址 + 不可达） | `reports/gates/staging_real_readiness_2026-03-31_110213.md` |
| 命令手册更新 | 完成 | 新增第 23/24 节执行说明 | `docs/supply_gate_command_playbook_v1_2026-03-25.md` |
| 阶段对齐验证 | 完成 | Checkpoint-31 报告 | `reports/alignment_validation_checkpoint_31_2026-03-31.md` |

---

## 30. 完整开发测试批次续跑（2026-03-31）

| 项目 | 状态 | 输出产物 | 证据 |
|---|---|---|---|
| 本地 STG env 重生成 | 完成 | `.env.staging-real` 重新签发 owner/viewer/admin 三类 token（非占位） | `reports/gates/local_staging_env_generation_2026-03-31_123102.md` |
| STG 本地流水续跑 | 完成 | `staging_release_pipeline`（local/mock）STEP-01~03 全 PASS | `reports/gates/staging_release_pipeline_2026-03-31_123148.md` |
| Superpowers 总控续跑 | 完成 | 发布流水 STEP-01~04 PASS，阶段结论保持 `CONDITIONAL_GO` | `reports/gates/superpowers_release_pipeline_2026-03-31_123150.md` + `reports/gates/superpowers_stage_validation_2026-03-31_123150.md` |
| TOK-007 复审续跑 | 完成 | 复审结论维持 `CONDITIONAL_GO`，与 local/mock 边界一致 | `review/outputs/tok007_release_recheck_2026-03-31_123153.md` |
| 真实 STG 就绪度复核 | 完成 | 就绪度仍为 `BLOCKED`（`STG-RDY-004/008`） | `reports/gates/staging_real_readiness_2026-03-31_123159.md` |
| Minimax 上游 smoke 复核 | 完成 | Base 404 + Active 200，结论 `PASS` | `reports/gates/minimax_upstream_smoke_2026-03-31_123210.md` |
| 阶段对齐验证 | 完成 | Checkpoint-32 报告 | `reports/alignment_validation_checkpoint_32_2026-03-31.md` |
