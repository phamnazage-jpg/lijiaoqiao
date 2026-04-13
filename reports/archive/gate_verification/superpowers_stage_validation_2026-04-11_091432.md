# Superpowers 阶段验证报告

- 时间戳：2026-04-11_091432
- 执行脚本：`scripts/ci/superpowers_stage_validate.sh`
- 决策：**NO_GO**
- 决策依据：at least one phase failed

## 阶段结果

| 阶段 | 结果 | 说明 | 证据 |
|---|---|---|---|
| PHASE-00 | PASS | Backend critical verification gate | /home/long/project/立交桥/tests/supply/artifacts/superpowers_stage_validation_2026-04-11_091432/phase00_backend_verify.log |
| PHASE-01 | PASS | TOK runtime code tests | /home/long/project/立交桥/tests/supply/artifacts/superpowers_stage_validation_2026-04-11_091432/phase01_go_test.log |
| PHASE-02 | PASS | SUP local-mock run_all execution | /home/long/project/立交桥/tests/supply/artifacts/superpowers_stage_validation_2026-04-11_091432/phase02_sup_run_all_mock.log |
| PHASE-03 | PASS | TOK-005 boundary dry-run on local-mock env | /home/long/project/立交桥/tests/supply/artifacts/superpowers_stage_validation_2026-04-11_091432/phase03_tok005_dryrun_mock.log |
| PHASE-04 | PASS | TOK-006 gate bundle aggregation | /home/long/project/立交桥/tests/supply/artifacts/superpowers_stage_validation_2026-04-11_091432/phase04_tok006_bundle.log |
| PHASE-05 | PASS | Dependency audit gate validation | /home/long/project/立交桥/tests/supply/artifacts/superpowers_stage_validation_2026-04-11_091432/phase05_dependency_audit.log |
| PHASE-06 | PASS | Stage gate rollback drill | /home/long/project/立交桥/tests/supply/artifacts/superpowers_stage_validation_2026-04-11_091432/phase06_stage_gate_drill.log |
| PHASE-07 | FAIL | Real staging precheck (expected deferred before real secrets) | /home/long/project/立交桥/tests/supply/artifacts/superpowers_stage_validation_2026-04-11_091432/phase07_staging_precheck.log |
| PHASE-08 | PASS | Daily metrics snapshot for M-017/M-018/M-019 | /home/long/project/立交桥/tests/supply/artifacts/superpowers_stage_validation_2026-04-11_091432/phase08_metrics_snapshot.log |
| PHASE-09 | PASS | 7-day metrics trend report generation | /home/long/project/立交桥/tests/supply/artifacts/superpowers_stage_validation_2026-04-11_091432/phase09_metrics_trend.log |
| PHASE-10 | PASS | Token runtime readiness check (M-021) | /home/long/project/立交桥/tests/supply/artifacts/superpowers_stage_validation_2026-04-11_091432/phase10_token_runtime_readiness.log |

## 说明

1. PHASE-07 为真实 staging 验证阶段，在占位凭证场景下允许 DEFERRED，不得伪造 PASS。
2. PHASE-08/09 负责 M-017/M-018/M-019 的每日快照与趋势证据生成。
3. PHASE-10 负责 M-021 token 运行态就绪度计算。
4. 其余阶段均为可执行验证，必须以命令返回码与证据文件为准。
