# 规划设计对齐验证报告（Checkpoint-25 / 开发阶段切回本地地址）

- 日期：2026-03-30
- 触发条件：按指示“开发阶段先用本地地址跑通，Minimax URL/token 仅作开发测试参考”

## 1. 结论

结论：**本阶段对齐通过。默认执行环境已切回本地 mock，开发门禁流水恢复可执行，且仍保持 `CONDITIONAL_GO` 防误判。**

## 2. 对齐范围

1. `scripts/supply-gate/.env`（已切回 local-mock 值）
2. `scripts/supply-gate/.env.minimax-dev`（保留此前 Minimax 测试值）
3. `scripts/ci/staging_release_pipeline.sh`
4. `reports/gates/staging_release_pipeline_2026-03-30_212424.md`
5. `reports/gates/superpowers_stage_validation_2026-03-30_212426.md`
6. `review/outputs/tok007_release_recheck_2026-03-30_212430.md`
7. `reports/gates/staging_token_go_evidence_autofill_2026-03-30_212430.md`

## 3. 核查结果

| 核查项 | 结果 | 证据 |
|---|---|---|
| 默认 env 已可用于本地演练（local mock） | PASS | `staging_release_pipeline_2026-03-30_212424.md` |
| staging 三步流水（STEP-01~03） | PASS | 同上报告，3/3 PASS |
| superpowers 分阶段验证 | PASS（决策为 `CONDITIONAL_GO`） | `superpowers_stage_validation_2026-03-30_212426.md` |
| TOK-007 复审机判 | PASS（机判 `CONDITIONAL_GO`） | `tok007_release_recheck_2026-03-30_212430.md` |
| staging 自动回填草稿产出 | PASS | `staging_token_go_evidence_autofill_2026-03-30_212430.md` |

## 4. 说明

1. `CONDITIONAL_GO` 是预期：当前为 local/mock 演练证据，不可上调为真实 staging `GO`。
2. Minimax URL/token 不能直接替代 SUP 平台契约地址（`/api/v1/supply/*`），此前已在 Checkpoint-24 记录。
3. 当前做法是：
   - 开发门禁与流程联调用 local mock；
   - 上游 Minimax 能力验证应走独立 smoke（不混入 SUP 发布门禁判定）。

## 5. 下一步

1. 需要时可新增 `scripts/supply-gate/minimax_upstream_smoke.sh`，单独校验 Minimax token 可用性。
2. 当平台 staging API 网关地址可用后，恢复真实 env 并重跑完整门禁链路。
