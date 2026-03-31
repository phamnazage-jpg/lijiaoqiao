# 规划设计对齐验证报告（Checkpoint-24 / 真实参数验证回归）

- 日期：2026-03-30
- 触发条件：使用真实 `API_BASE_URL + token` 执行 staging 发布流水验证

## 1. 结论

结论：**本阶段对齐未通过（NO_GO）。根因不是脚本执行框架，而是接口契约不匹配：当前 URL 指向上游提供方接口，不是 SUP-004~SUP-007 预期的平台 API。**

## 2. 对齐范围

1. `scripts/supply-gate/.env`（真实值注入）
2. `scripts/ci/staging_release_pipeline.sh`
3. `scripts/supply-gate/staging_precheck_and_run.sh`
4. `scripts/supply-gate/run_all.sh`
5. `scripts/supply-gate/sup004_accounts.sh`
6. `reports/gates/staging_release_pipeline_2026-03-30_205035.md`
7. `reports/gates/step-01_2026-03-30_205035.out.log`
8. `tests/supply/artifacts/sup004/01_verify.json`
9. `tests/supply/artifacts/sup004/02_create.json`
10. `reports/gates/superpowers_release_pipeline_2026-03-30_205037.md`

## 3. 核查结果

| 核查项 | 结果 | 证据 |
|---|---|---|
| API_BASE_URL 连通性（HEAD） | PASS | `step-01_2026-03-30_205035.out.log` 中 `reachable` |
| TOK-005 dry-run + M-021 readiness | PASS | `step-01_2026-03-30_205035.out.log`（readiness 100%） |
| SUP-004 首个业务接口返回契约可解析 JSON | FAIL | `tests/supply/artifacts/sup004/01_verify.json` 为 nginx `404 Not Found` HTML |
| staging_release_pipeline 总结果 | FAIL | `staging_release_pipeline_2026-03-30_205035.md` |
| superpowers_release_pipeline 总结果 | FAIL | `superpowers_release_pipeline_2026-03-30_205037.md` |

## 4. 根因分析

1. `sup004_accounts.sh` 固定访问：`{API_BASE_URL}/api/v1/supply/accounts/verify`。
2. 当前提供的 `API_BASE_URL=https://api.minimaxi.com/anthropic`，拼接后为：
   `https://api.minimaxi.com/anthropic/api/v1/supply/accounts/verify`。
3. 该地址返回 HTML 404（非平台契约 JSON），导致 `jq` 解析失败并中断 `run_all`。
4. 因此当前失败判定为：**环境地址与 SUP 契约不匹配**，并非单纯 token 占位或脚本逻辑缺陷。

## 5. 影响评估

1. 不能据此判定 token 本身有效/无效（未命中正确业务契约）。
2. 当前发布门禁链路维持 FAIL/NO_GO 是正确行为，防止误放行。
3. 若继续沿用该 URL，SUP-004~007 全链路都会因契约错位失败。

## 6. 修复建议（下一步）

1. 提供“平台 SUP API 网关”基地址（应与 `/api/v1/supply/*` 契约匹配）。
2. 若目标仅验证 Minimax token，请走独立“上游直连 smoke”脚本，不应复用 SUP 门禁脚本。
3. 拿到正确平台地址后，重跑：
   `bash scripts/ci/staging_release_pipeline.sh scripts/supply-gate/.env`
