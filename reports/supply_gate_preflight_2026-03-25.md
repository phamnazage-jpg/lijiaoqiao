# SUP Gate 执行前检查报告（Preflight）

- 日期：2026-03-25
- 目标：确认 `SUP-004~SUP-007` 是否具备执行条件
- 执行命令：`bash scripts/supply-gate/run_all.sh`

---

## 1. 前置检查结果

| 检查项 | 结果 | 说明 |
|---|---|---|
| `curl` 可用 | PASS | 本机已安装 |
| `jq` 可用 | PASS | 本机已安装 |
| `scripts/supply-gate/.env` 存在 | PASS | 已由 `.env.example` 生成占位文件 |
| `API_BASE_URL` 可达 | FAIL | `staging.example.com` DNS 不可解析 |
| `OWNER_BEARER_TOKEN` 已配置 | WARN | 当前为占位值，需平台短期 token |
| `VIEWER_BEARER_TOKEN` 已配置 | WARN | 当前为占位值，需平台短期 token |
| `ADMIN_BEARER_TOKEN` 已配置 | WARN | 当前为占位值，需平台短期 token |

---

## 2. 阻塞证据

1. 原始日志：`tests/supply/artifacts/preflight/2026-03-25_run_all_dns_blocked.log`
2. 错误摘要：
   1. `curl: (6) Could not resolve host: staging.example.com`

---

## 3. 解锁动作（最小集合）

1. 修正 `API_BASE_URL` 为当前可达的 staging/预发地址。
2. 填充平台下发的短期 token（仅平台分发，不对外共享）。
3. 执行：
   1. `bash scripts/supply-gate/sup004_accounts.sh`
   2. `bash scripts/supply-gate/sup005_packages.sh`
   3. `bash scripts/supply-gate/sup006_settlements.sh`
   4. `bash scripts/supply-gate/sup007_boundary.sh`
4. 回填 `tests/supply/*.md` 与 `reports/supply_gate_review_2026-03-31.md`。

---

## 4. 当前门禁结论

1. `SUP-004~SUP-007`：BLOCKED
2. `SUP-008`：不通过（证据不足）
3. 结论等级：`CONDITIONAL GO`（仅限设计层），执行层 `NO-GO` 直至补齐前置。

---

## 5. 2026-03-27 本地演练补充

1. 已新增 local-mock 网关并完成 `SUP-004~SUP-007` 脚本演练。
2. 演练环境：`http://127.0.0.1:18080`。
3. 演练产物：
   1. `tests/supply/artifacts/sup004/*`
   2. `tests/supply/artifacts/sup005/*`
   3. `tests/supply/artifacts/sup006/*`
   4. `tests/supply/artifacts/sup007/*`
4. 结论：本地演练链路通过，真实 staging 仍需复核。
