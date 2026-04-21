# 2026-04-21 Project Quality / Performance / Ops Optimization Execution Log

## P1-A-01 当前身份入口扫描结果

执行命令：

```bash
rg -n "inmemory|remote_introspection|token runtime|JWT|Bearer" gateway supply-api platform-token-runtime
```

结论：

1. `gateway` 当前同时保留两种 token runtime 装配路径：
   - `gateway/internal/app/bootstrap.go`
   - `gateway/internal/config/config.go`
   - `gateway/internal/middleware/chain.go`
   - `gateway/README.md`

2. `platform-token-runtime` 当前承载独立的 token issue / refresh / revoke / introspect / audit-events 语义：
   - `platform-token-runtime/internal/app/bootstrap.go`
   - `platform-token-runtime/internal/httpapi/token_api.go`
   - `platform-token-runtime/internal/auth/middleware/token_auth_middleware.go`
   - `platform-token-runtime/README.md`

3. `supply-api` 当前仍保留独立 JWT 校验、Bearer 提取和 token 状态后端：
   - `supply-api/internal/middleware/auth.go`
   - `supply-api/internal/app/bootstrap.go`
   - `supply-api/internal/middleware/ratelimit.go`
   - `supply-api/internal/middleware/token_format_test.go`

4. 当前身份链路的关键事实：
   - `gateway` 的远程模式依赖 `platform-token-runtime` 做 introspection。
   - `supply-api` 仍以 JWT claims 为认证入口。
   - 三个服务对“谁是 authority”仍未收敛为单一真源。

## P1-A-04 OpenAPI 与 canonical principal 差异记录

执行命令：

```bash
rg -n "IntrospectTokenResponse|tenant_id|project_id|operator_id|metadata|IssueTokenRequest" docs/platform_token_api_contract_openapi_draft_v1_2026-03-29.yaml
```

结论：

1. `IssueTokenRequest` 已定义 `metadata`，但 `IntrospectTokenResponse.data` 尚未暴露 `tenant_id`。
2. 现有 introspection 响应字段只有 `token_id`、`subject_id`、`role`、`status`、`scope`、`issued_at`、`expires_at`，与 canonical principal 最小字段清单相比缺少 `tenant_id`。
3. `project_id`、`operator_id`、`metadata` 目前也未出现在 introspection 响应中，但它们尚未进入最小 canonical principal 强制字段，保留为 Phase 1 后续 schema / 审计收敛项。

## P1-A-07 supply-api README 统一 principal 说明

结论：

1. 已在 `supply-api/README.md` 明确 `supply-api` 只消费 canonical principal，不自持独立 token authority。
2. 文档现在把 token 状态与权限判断的唯一来源收束到 `platform-token-runtime` 的 introspection 结果。

## P1-A-08 platform-token-runtime README 唯一 authority 与字段边界

结论：

1. 已在 `platform-token-runtime/README.md` 明确 `platform-token-runtime` 是唯一 token authority。
2. 文档现在把 canonical principal 的最小字段边界写死为 `token_id`、`subject_id`、`tenant_id`、`role`、`scope`、`issued_at`、`expires_at`、`status`。
3. 文档同时要求未来扩展字段必须同步更新 DDL、OpenAPI、存储模型和审计字段，避免边界漂移。

## P1-B token runtime schema 对齐决策

执行结果：

1. 已创建 `docs/plans/2026-04-21-token-runtime-schema-alignment-notes.md`，记录 schema、model、runtime store、audit store 的字段差异。
2. 单一决策为“保留字段并贯穿”，不采用删除字段 / shrink SQL 路线。
3. 后续实现顺序固定为：`model -> store -> API -> audit -> tests`。

## P1-C 身份实现收敛策略

执行结果：

1. 已创建 `docs/plans/2026-04-21-auth-implementation-convergence-notes.md`，记录 gateway 与 supply-api 的身份入口、装配点和迁移清单。
2. gateway 侧后续只保留 `remote_introspection` 作为非 `dev` 环境 authority 入口，本地 `inmemory` 仅允许 `dev`。
3. supply-api 侧过渡策略固定为：`单写 + 双读短窗 + 一次性切断旧 JWT`。
4. 回滚目标契约固定为“兼容窗口契约 v1”，不回滚到旧的多 authority 设计。

## P1-D contract gate 设计完成

执行结果：

1. 已创建 `tests/contract/README.md` 与 `tests/contract/gateway_token_runtime_supply_chain.md`，明确当前 CI 覆盖缺口与四个最小 contract 场景。
2. 已创建 `docs/plans/2026-04-21-phase1-contract-gate-checklist.md`，把 Phase 1 关闭条件绑定到 contract gate。
3. 已在 `scripts/ci/backend-verify.sh` 和 `scripts/ci/repo_integrity_check.sh` 写明 contract gate 执行位、产物路径和失败语义。

## P2-A release manifest 合同设计完成

执行结果：

1. 已创建 `docs/plans/2026-04-21-release-manifest-contract.md`，记录 `latest_file_or_empty` 依赖入口、`run_id` 规则、目录结构与 `manifest.json` 必填字段。
2. 已创建 `reports/releases/.gitkeep`，为后续 `<run_id>` 工件目录预留稳定路径。
3. 已在四个脚本中补入 manifest 迁移设计说明，明确后续必须从 `decision_inputs` / `artifact_paths` 读取本次 run 的证据。

## P2-B 真实 staging 硬门禁规则设计完成

执行命令：

```bash
rg -n 'P2-B-0[1-8]|PASS_REAL|PASS_REHEARSAL|FAIL|override|DEFERRED|real_staging_pass' docs/plans/2026-04-21-real-staging-gate-rules.md
bash -n scripts/ci/superpowers_stage_validate.sh
bash -n scripts/ci/staging_real_readiness_check.sh
bash -n scripts/ci/superpowers_release_pipeline.sh
bash -n scripts/ci/final_decision_consistency_check.sh
git diff --check
```

执行结果：

1. 已创建 `docs/plans/2026-04-21-real-staging-gate-rules.md`，逐项覆盖 `P2-B-01` 到 `P2-B-08`，写死 rehearsal / real staging 术语边界、`PASS_REAL|PASS_REHEARSAL|FAIL` 状态枚举、override 约束和完成率口径。
2. 已在 `scripts/ci/superpowers_stage_validate.sh`、`scripts/ci/staging_real_readiness_check.sh`、`scripts/ci/superpowers_release_pipeline.sh`、`scripts/ci/final_decision_consistency_check.sh` 补入迁移设计注释，明确真实 staging 是唯一 release 硬门禁，且 `DEFERRED` / rehearsal 不得计入 release pass。
3. 四个脚本 `bash -n` 通过，且 `git diff --check` 无格式错误；本批次仅落设计规则与迁移约束，没有伪装成已完成实现。
