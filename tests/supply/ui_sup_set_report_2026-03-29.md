# UI-SUP-SET 执行报告（结算提现链路）

- 日期：2026-03-29
- 覆盖用例：UI-SUP-SET-001~005
- 执行环境：staging

## 1. 执行结果汇总

| 用例ID | 结果 | 备注 |
|---|---|---|
| UI-SUP-SET-001 | BLOCKED | `API_BASE_URL` 不可达（DNS 解析失败） |
| UI-SUP-SET-002 | BLOCKED | `API_BASE_URL` 不可达（DNS 解析失败） |
| UI-SUP-SET-003 | BLOCKED | `API_BASE_URL` 不可达（DNS 解析失败） |
| UI-SUP-SET-004 | BLOCKED | `API_BASE_URL` 不可达（DNS 解析失败） |
| UI-SUP-SET-005 | BLOCKED | `API_BASE_URL` 不可达（DNS 解析失败） |

## 2. 证据

1. 请求/响应日志路径：
   `tests/supply/artifacts/preflight/2026-03-25_run_all_dns_blocked.log`
2. 截图/录屏路径：
   无（未进入 UI/API 执行阶段）
3. 审计事件截图路径：
   无（未进入 UI/API 执行阶段）

## 3. 结论

- 通过率：0/5（0%）
- 是否阻断发布（是/否）：是
- Owner：QA（待指派实名）
