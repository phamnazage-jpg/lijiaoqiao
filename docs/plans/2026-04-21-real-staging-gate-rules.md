# 2026-04-21 Real Staging Gate Rules

## P2-B-01 当前把 `mock` / `local` / `DEFERRED` 计为通过的判定点

1. `scripts/ci/superpowers_stage_validate.sh:100`
   local/mock 环境直接把 `PHASE-07` 写成 `DEFERRED`。
2. `scripts/ci/superpowers_stage_validate.sh:110`
   `run_step_allow_deferred` 允许真实 staging 前置检查用 `DEFERRED` 退出。
3. `scripts/ci/superpowers_stage_validate.sh:166`
   `PHASE-02` 到 `PHASE-04` 继续在 `.env.local-mock` 上执行 rehearsal 流程。
4. `scripts/ci/superpowers_stage_validate.sh:233`
   只要存在 `DEFERRED` 就给出 `CONDITIONAL_GO`，没有直接把它判成失败。
5. `scripts/ci/superpowers_stage_validate.sh:238`
   local/mock 环境即使其余步骤都通过，也只降级到 `CONDITIONAL_GO`，仍可能被后续流水继续消费。
6. `scripts/ci/staging_real_readiness_check.sh:68`
   环境分类同时存在 `local-mock`、`placeholder`、`real-staging`，但输出结果仍是 `READY/BLOCKED`，没有单独的 real-pass 字段。
7. `scripts/ci/superpowers_release_pipeline.sh:78`
   发布流水当前在 `STEP-01` 之后不会校验“是否 real staging pass”，会继续执行 `STEP-02` 到 `STEP-04`。
8. `scripts/ci/final_decision_consistency_check.sh:100`
   当前只比较最终决议与机判是否一致，没有把 `DEFERRED` / rehearsal 明确排除在 release pass 之外。

## P2-B-02 rehearsal 与 real staging 术语

术语定义：

1. `rehearsal`
   任一使用 `local-mock`、`localhost`、占位 API 地址、占位 token、或无法访问真实 staging 的运行。
2. `real staging`
   使用真实外网 staging 地址、真实 owner/viewer/admin token，并通过真实连通性检查的运行。

约束：

1. rehearsal 只用于排练脚本链路与产物结构。
2. real staging 才能产生发布硬门禁证据。
3. 两者语义完全不重叠，rehearsal 结果不得冒充 real staging。

## P2-B-03 总规则

只有当同一 `run_id` 下的 real staging 检查通过，且阶段验证状态为 `PASS_REAL`，发布流水才允许输出 `PASS`。

## P2-B-04 `superpowers_stage_validate.sh` 状态枚举设计

必须只保留三种发布语义状态：

1. `PASS_REAL`
   `PHASE-07` 在真实 staging 上通过。
2. `PASS_REHEARSAL`
   rehearsal 流程通过，但 `PHASE-07` 是 local/mock/deferred。
3. `FAIL`
   任一必需阶段失败，或真实 staging 要求未满足。

映射约束：

1. 当前 `GO` 只能在未来映射为 `PASS_REAL`。
2. 当前 `CONDITIONAL_GO` 统一映射为 `PASS_REHEARSAL`。
3. `DEFERRED` 只能流向 `PASS_REHEARSAL` 或 `FAIL`，不能直接升格。

## P2-B-05 `staging_real_readiness_check.sh` 阻断输出格式

输出至少包含以下稳定字段：

- `real_staging_pass=true|false`
- `classification=REAL|REHEARSAL`
- `reason_code=<stable_code>`
- `run_id=<run_id>`

阻断规则：

1. 只要 `real_staging_pass=false`，发布链路必须立即阻断。
2. `classification=REHEARSAL` 时，无论其余检查如何，都不能视为 real-pass。

## P2-B-06 `superpowers_release_pipeline.sh` fail-fast 位置

fail-fast 放置点：

1. `STEP-01` 执行完阶段验证后立即解析其输出。
2. 若状态不是 `PASS_REAL` 且没有合法 override，立刻 `exit 1`。
3. `STEP-02` TOK-007、`STEP-03` final consistency、`STEP-04` final decision candidate 都不得在 non-real-pass 条件下继续执行。

## P2-B-07 override 机制设计

override 必填字段：

- `approver`
- `timestamp`
- `run_id`
- `reason`

约束：

1. override 必须显式落盘到本次 `run_id` 目录。
2. 没有完整四字段的 override 一律视为无效。
3. override 只能豁免发布阻断，不能把 rehearsal 改写成 real staging 事实。

## P2-B-08 `DEFERRED` 不计入完成率的规则

精确口径：

`phase_completion_rate = PASS_REAL_required_phases / total_required_phases`

规则说明：

1. 分子只统计 `PASS_REAL` 的必需阶段。
2. `DEFERRED`、`PASS_REHEARSAL`、`WARN`、`SKIP` 都不计入分子。
3. `DEFERRED` 仍占据必需阶段名额，因此会拉低完成率，不能被当作“完成一半”或“条件通过”。
