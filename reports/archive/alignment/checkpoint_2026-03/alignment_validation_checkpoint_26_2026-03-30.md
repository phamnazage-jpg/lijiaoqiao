# 规划设计对齐验证报告（Checkpoint-26 / Minimax 上游独立 Smoke 落地）

- 日期：2026-03-30
- 触发条件：新增“上游 Minimax 独立验证”能力，避免与 SUP 发布门禁链路耦合

## 1. 结论

结论：**本阶段对齐通过。已新增独立上游 smoke 脚本并完成实测，Minimax active 探测返回 200；SUP 发布门禁仍保持独立判定边界。**

## 2. 对齐范围

1. `scripts/supply-gate/minimax_upstream_smoke.sh`
2. `docs/supply_gate_command_playbook_v1_2026-03-25.md`（新增第 20 节）
3. `reports/gates/minimax_upstream_smoke_2026-03-30_231930.md`
4. `tests/supply/artifacts/minimax_smoke_2026-03-30_231930/02_active_probe_body.json`

## 3. 核查结果

| 核查项 | 结果 | 证据 |
|---|---|---|
| 独立 smoke 脚本可执行（语法 + 运行） | PASS | `minimax_upstream_smoke_2026-03-30_231930.md` |
| Base 连通探测可达 | PASS | http_code=404（base 探测） |
| Active 鉴权探测到达业务层并成功返回 | PASS | http_code=200，见 active probe body |
| 结果分类与失败边界清晰 | PASS | 报告中 `PASS/PASS_AUTH_REACHED/FAIL_*` 规则 |
| 与 SUP-004~SUP-007 门禁链路解耦 | PASS | 命令手册第20节说明“不可替代 SUP 门禁” |

## 4. 关键说明

1. `API_BASE_URL=https://api.minimaxi.com/anthropic` 在 base 地址上返回 404 属于可预期，不影响 active 路径探测。
2. active 路径 `.../v1/messages` 返回 200，说明该 token 在当前 smoke 路径下可用。
3. 该结果仅证明“上游可达 + 鉴权可用”，不等价于 SUP 平台业务契约通过。

## 5. 下一步

1. 继续默认使用 local/mock 跑 SUP 开发门禁。
2. 如需持续监控 Minimax 上游可用性，可将 `minimax_upstream_smoke.sh` 挂入定时健康检查。
3. 等平台 staging 网关地址就绪后，再执行真实 SUP 门禁闭环。
