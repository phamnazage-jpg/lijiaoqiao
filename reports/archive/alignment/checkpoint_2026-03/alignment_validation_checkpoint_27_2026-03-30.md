# 规划设计对齐验证报告（Checkpoint-27 / Minimax 监控化增强）

- 日期：2026-03-30
- 触发条件：将 Minimax 上游独立 smoke 进一步纳入“可持续执行”的日常快照链路

## 1. 结论

结论：**本阶段对齐通过。已完成 Minimax smoke 判定口径修正、dry-run 能力补齐、每日快照脚本落地，满足“开发期可持续执行 + 不误入 SUP 发布门禁”的要求。**

## 2. 对齐范围

1. `scripts/supply-gate/minimax_upstream_smoke.sh`
2. `scripts/ci/minimax_upstream_daily_snapshot.sh`
3. `docs/supply_gate_command_playbook_v1_2026-03-25.md`
4. `reports/gates/minimax_upstream_smoke_2026-03-30_232510.md`
5. `reports/gates/minimax_upstream_daily_snapshot_2026-03-30.md`
6. `reports/gates/minimax_upstream_daily_snapshots.csv`

## 3. 核查结果

| 核查项 | 结果 | 证据 |
|---|---|---|
| smoke 脚本支持 `MINIMAX_SMOKE_DRY_RUN=1`（不发网络请求） | PASS | `minimax_upstream_smoke_2026-03-30_232510.md` |
| smoke 判定口径修正（base=连通、active=业务状态） | PASS | `scripts/supply-gate/minimax_upstream_smoke.sh` 判定规则段 |
| 每日快照脚本可执行并产生日报 | PASS | `minimax_upstream_daily_snapshot_2026-03-30.md` |
| 每日快照 CSV 可更新覆盖当日数据 | PASS | `minimax_upstream_daily_snapshots.csv` |
| 快照默认优先引用非 dry-run 报告 | PASS | 2026-03-30 快照证据指向 `...231930.md`（active=200） |
| 文档已补齐第 21 节命令与断言 | PASS | `docs/supply_gate_command_playbook_v1_2026-03-25.md` |

## 4. 关键变更

1. `minimax_upstream_smoke.sh` 新增 dry-run 模式，适配“先联调再开真实请求”的执行策略。
2. `minimax_upstream_smoke.sh` 的规则描述与结果口径保持一致，避免 404 base 探测与规则冲突。
3. 新增 `scripts/ci/minimax_upstream_daily_snapshot.sh`，支持：
   - `RUN_ACTIVE_SMOKE=0`：仅汇总（默认）；
   - `RUN_ACTIVE_SMOKE=1`：实时探测后汇总。
4. 快照脚本默认优先选取“非 PASS_DRY_RUN”最新报告，降低误判风险。

## 5. 限制与说明

1. Minimax 快照仅用于上游可达性趋势，不可替代 SUP-004~SUP-007 门禁结论。
2. 当前开发主链仍应使用 local/mock 维持持续迭代；真实 staging 仍待平台网关地址就绪。

## 6. 下一步

1. 如你同意，我可继续把 `minimax_upstream_daily_snapshot.sh` 接入 `superpowers_release_pipeline.sh` 的“可选监控步”（默认关闭）。
2. 也可新增 7 日趋势脚本（类似 M-017~019）用于上游稳定性周报。
