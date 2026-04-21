# Contract Tests

## 目的

这里存放跨服务 contract tests 的说明文档与执行约束，覆盖 `gateway`、`platform-token-runtime`、`supply-api` 的共享身份链路。

## 当前状态

截至 `2026-04-21`，仓库里的 CI 仍以单服务测试为主：

- `scripts/ci/backend-verify.sh` 只跑三个服务各自的关键回归与 `supply-api` E2E 占位检查。
- `scripts/ci/repo_integrity_check.sh` 只跑 shell 语法、单服务 Go 测试、仓储集成和 `supply-api` E2E。
- 当前没有任何一个 gate 会把“同一个 token 在 gateway、platform-token-runtime、supply-api 三端的行为一致性”作为硬门禁。

## 当前未覆盖的最小链路

1. 合法 token 在 gateway 与 supply-api 两端都能通过，并带出一致 principal 字段。
2. 吊销 token 会同时在 gateway 和 supply-api 被拒绝。
3. scope 不足时拒绝行为与错误码一致。
4. token runtime 不可用时，gateway 与 supply-api 的失败语义满足统一约束。

## 合同文档

- 主场景定义：`tests/contract/gateway_token_runtime_supply_chain.md`
- Phase 1 gate checklist：`docs/plans/2026-04-21-phase1-contract-gate-checklist.md`

## 设计中的执行入口

- `bash scripts/ci/backend-verify.sh --phase1-contract-gate`
- `bash scripts/ci/repo_integrity_check.sh`

约束：

1. contract gate 的 evidence 统一写入 `reports/archive/gate_verification/contract_gate_<timestamp>.log|md`。
2. 任一场景缺证据、断言不成立或脚本非零退出，都必须让 gate 失败。
