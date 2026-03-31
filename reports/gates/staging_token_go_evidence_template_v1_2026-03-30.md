# Staging 联调证据回填模板（Token + SUP Gate）

- 模板版本：v1
- 日期：2026-03-30
- 用途：真实 staging 参数就绪后，按统一口径回填 `PHASE-07`、`M-013~M-016`、`M-021` 证据。

## 1. 环境信息

| 字段 | 值 |
|---|---|
| API_BASE_URL | |
| OWNER_BEARER_TOKEN（脱敏） | |
| VIEWER_BEARER_TOKEN（脱敏） | |
| ADMIN_BEARER_TOKEN（脱敏） | |
| 执行人 | |
| 执行时间（开始/结束） | |

## 2. 执行命令（按顺序）

1. `bash scripts/supply-gate/staging_precheck_and_run.sh scripts/supply-gate/.env`
2. `bash scripts/ci/superpowers_stage_validate.sh`
3. `bash scripts/ci/superpowers_release_pipeline.sh`

## 3. 关键结论

- [ ] PHASE-07 = PASS（不再 DEFERRED）
- [ ] M-013 = 0（staging）
- [ ] M-014 = 100%（staging）
- [ ] M-015 = 0（staging）
- [ ] M-016 = 100%（staging）
- [ ] M-021 = 100%（staging验收口径）

## 4. 指标回填

| 指标ID | 指标名 | 目标 | 实测 | 结论 | 证据 |
|---|---|---:|---:|---|---|
| M-013 | supplier_credential_exposure_events | 0 | | | |
| M-014 | platform_credential_ingress_coverage_pct | 100% | | | |
| M-015 | direct_supplier_call_by_consumer_events | 0 | | | |
| M-016 | query_key_external_reject_rate_pct | 100% | | | |
| M-021 | token_runtime_readiness_pct | 100% | | | |

## 5. 证据路径

1. `reports/gates/staging_run_*.log`
2. `reports/gates/superpowers_stage_validation_*.md`
3. `reports/gates/superpowers_release_pipeline_*.md`
4. `reports/gates/token_runtime_readiness_*.md`
5. `tests/supply/sec_sup_boundary_report_2026-03-30.md`（staging回填版）
6. `review/outputs/tok007_release_recheck_*.md`

## 6. 异常与阻塞

| 编号 | 异常描述 | 影响 | 临时措施 | 负责人 | 关闭时间 |
|---|---|---|---|---|---|
| B-01 | | | | | |

## 7. 复审建议

1. 若本模板第 3 节全部勾选，触发 `final_decision` 更新流程。
2. 若任一项未达标，维持 `CONDITIONAL_GO/NO_GO` 并回填整改计划。
