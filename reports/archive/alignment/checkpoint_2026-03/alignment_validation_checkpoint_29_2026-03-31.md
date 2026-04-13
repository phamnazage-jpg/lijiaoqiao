# 规划设计对齐验证报告（Checkpoint-29 / STG 本地演练恢复）

- 日期：2026-03-31
- 触发条件：继续执行 STG 批次（本机开发测试口径），修复 PHASE-10 阻塞后复跑整条流水。

## 1. 结论

结论：**本阶段对齐通过。STG 本地演练流水已恢复 `PASS`，并明确保持 `local/mock` 与真实 staging 放行证据边界。**

## 2. 对齐范围

1. `scripts/ci/token_runtime_readiness_check.sh`
2. `reports/gates/staging_release_pipeline_2026-03-31_100116.md`
3. `reports/gates/superpowers_release_pipeline_2026-03-31_100120.md`
4. `reports/gates/superpowers_stage_validation_2026-03-31_100120.md`
5. `review/outputs/tok007_release_recheck_2026-03-31_100127.md`

## 3. 问题与修复

### 3.1 发现的问题

1. `PHASE-10`（M-021）在 `ENABLE_TOKEN_RUNTIME_SMOKE=1` 场景下失败。
2. 根因一：默认 smoke 端口 `18082` 被 `supply-api` 占用，冒烟请求命中错误服务（`issue` 返回 404）。
3. 根因二：脚本 smoke 分支使用 `exit 1` 直接退出，失败时无法稳定产出完整汇总输出。

### 3.2 修复动作

1. 为 M-021 冒烟新增端口自动避让：从基准端口起寻找可用端口（最多 50 次）。
2. 将 smoke 执行块改为子 Shell 返回码模型，保留失败但不中断总报告生成流程。

## 4. 核查结果

| 核查项 | 结果 | 证据 |
|---|---|---|
| M-021 脚本修复后可执行 | PASS | `reports/gates/token_runtime_readiness_2026-03-31_100017.md` |
| Superpowers 阶段验证恢复通过（PHASE-10 PASS） | PASS | `reports/gates/superpowers_stage_validation_2026-03-31_100120.md` |
| Superpowers 发布流水恢复通过 | PASS | `reports/gates/superpowers_release_pipeline_2026-03-31_100120.md` |
| STG 本地演练流水 STEP-01~03 全 PASS | PASS | `reports/gates/staging_release_pipeline_2026-03-31_100116.md` |
| TOK-007 复审结论保持 `CONDITIONAL_GO`（未误升 GO） | PASS | `review/outputs/tok007_release_recheck_2026-03-31_100127.md` |

## 5. 结论边界说明

1. 本次通过仅代表 `local/mock` 演练链路恢复，不等价真实 staging 放行。
2. `F-01/F-02/F-04` 的真实 staging 证据要求仍保持不变。

## 6. 下一步

1. 进入 STG-001：替换真实 `API_BASE_URL` 并完成可达性验证。
2. 进入 STG-002：注入真实短期 token 并复跑 `staging_release_pipeline.sh`（真实环境）。
3. 完成 STG-004：将真实证据回填至 `review/final_decision_2026-03-31.md` 与 `reports/supply_gate_review_2026-03-31.md`。
