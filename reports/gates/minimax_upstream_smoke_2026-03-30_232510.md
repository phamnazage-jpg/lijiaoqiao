# Minimax 上游 Smoke 报告

- 时间戳：2026-03-30_232510
- 执行脚本：`scripts/supply-gate/minimax_upstream_smoke.sh`
- 环境文件：`scripts/supply-gate/.env.minimax-dev`
- API_BASE_URL：`https://api.minimaxi.com/anthropic`
- 目标路径：`/v1/messages`
- 探测 URL：`https://api.minimaxi.com/anthropic/v1/messages`
- 总体结论：**PASS_DRY_RUN**

## 1. 说明

- 本次为 dry-run，未发起任何外部网络请求。
- 用于流水联调与产物校验，不可替代真实上游验证证据。
