# Minimax 上游 Smoke 报告

- 时间戳：2026-03-31_103303
- 执行脚本：`scripts/supply-gate/minimax_upstream_smoke.sh`
- 环境文件：`scripts/supply-gate/.env.minimax-dev`
- API_BASE_URL：`https://api.minimaxi.com/anthropic`
- 目标路径：`/v1/messages`
- 探测 URL：`https://api.minimaxi.com/anthropic/v1/messages`
- 总体结论：**PASS**

## 1. Base 连通探测

- curl rc：0
- http_code：404
- 分类：**PASS_CONNECTIVITY**
- 产物：`/home/long/project/立交桥/tests/supply/artifacts/minimax_smoke_2026-03-31_103303/01_base_probe_body.txt` / `/home/long/project/立交桥/tests/supply/artifacts/minimax_smoke_2026-03-31_103303/01_base_probe_stderr.log`

## 2. Active 鉴权探测

- curl rc：0
- http_code：200
- 分类：**PASS**
- 产物：`/home/long/project/立交桥/tests/supply/artifacts/minimax_smoke_2026-03-31_103303/02_active_probe_request.json` / `/home/long/project/立交桥/tests/supply/artifacts/minimax_smoke_2026-03-31_103303/02_active_probe_body.json` / `/home/long/project/立交桥/tests/supply/artifacts/minimax_smoke_2026-03-31_103303/02_active_probe_stderr.log`

## 3. 判定规则

1. Base 探测仅判断连通：curl 成功且非 `000` 记为 `PASS_CONNECTIVITY`。
2. Active 探测 `2xx` => PASS（请求成功）。
3. Active 探测 `400/422/429` => PASS_AUTH_REACHED（已到达业务层，通常说明鉴权头被接收）。
4. Active 探测 `401/403` => FAIL_AUTH（鉴权失败）。
5. Active 探测 `404/405` => FAIL_PATH（路径或方法不匹配）。
6. 任一探测 `000` 或 curl 非零 => FAIL_NETWORK（网络/解析/连接失败）。
