# Staging 联调证据自动回填草稿

- 生成时间：2026-03-31_101923
- 生成脚本：`scripts/ci/staging_evidence_autofill.sh`

## 1. 自动抽取结果

| 项目 | 自动值 | 来源 |
|---|---|---|
| PHASE-07 | PASS | /home/long/project/立交桥/reports/gates/superpowers_stage_validation_2026-03-31_101919.md |
| M-013 | 0 | /home/long/project/立交桥/tests/supply/sec_sup_boundary_report_2026-03-30.md |
| M-014 | 100% | /home/long/project/立交桥/tests/supply/sec_sup_boundary_report_2026-03-30.md |
| M-015 | 0（未配置直连探测目标，未发现事件） | /home/long/project/立交桥/tests/supply/sec_sup_boundary_report_2026-03-30.md |
| M-016 | 100%（外部 query key 拒绝） | /home/long/project/立交桥/tests/supply/sec_sup_boundary_report_2026-03-30.md |
| M-021（值） | 100.00% (13/13) | /home/long/project/立交桥/reports/gates/token_runtime_readiness_2026-03-31_101922.md |
| M-021（结果） | PASS | /home/long/project/立交桥/reports/gates/token_runtime_readiness_2026-03-31_101922.md |
| TOK-007 机判 | CONDITIONAL_GO | /home/long/project/立交桥/review/outputs/tok007_release_recheck_2026-03-31_101922.md |

## 2. 证据路径清单

1. staging run：/home/long/project/立交桥/reports/gates/staging_run_2026-03-31_101920.log
2. stage validate：/home/long/project/立交桥/reports/gates/superpowers_stage_validation_2026-03-31_101919.md
3. token readiness：/home/long/project/立交桥/reports/gates/token_runtime_readiness_2026-03-31_101922.md
4. tok007 recheck：/home/long/project/立交桥/review/outputs/tok007_release_recheck_2026-03-31_101922.md
5. release pipeline：/home/long/project/立交桥/reports/gates/superpowers_release_pipeline_2026-03-31_101919.md
6. security boundary：/home/long/project/立交桥/tests/supply/sec_sup_boundary_report_2026-03-30.md

## 3. 人工确认项

1. 若 PHASE-07 仍为 DEFERRED，禁止将结论上调为 GO。
2. 若 M-013~M-016 来源为 mock，必须在 staging 复测后覆盖。
3. 若 M-021 仅为开发阶段口径，需在 staging 复跑后再次回填。
