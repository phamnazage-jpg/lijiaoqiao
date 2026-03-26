# SEC-SUP 边界回归报告

- 日期：2026-03-30
- 覆盖用例：SEC-SUP-001~002
- 指标映射：M-013/M-014/M-015/M-016

## 1. 执行结果

| 用例ID | 结果 | 备注 |
|---|---|---|
| SEC-SUP-001 | BLOCKED | `API_BASE_URL` 不可达（DNS 解析失败） |
| SEC-SUP-002 | BLOCKED | `API_BASE_URL` 不可达（DNS 解析失败） |

## 2. 指标结果

| 指标ID | 实际值 | 目标值 | 结论 |
|---|---|---|---|
| M-013 | N/A | 0 | BLOCKED |
| M-014 | N/A | 100% | BLOCKED |
| M-015 | N/A | 0 | BLOCKED |
| M-016 | N/A | 100% | BLOCKED |

## 3. 证据

1. 脱敏扫描报告路径：
   `tests/supply/artifacts/preflight/2026-03-25_run_all_dns_blocked.log`
2. 鉴权日志路径：
   无（未进入执行）
3. 拦截日志路径：
   无（未进入执行）
4. 安全事件路径：
   无（未进入执行）

## 4. 结论

- 是否触发P0：否（当前为前置阻塞，尚未进入安全执行）
- 是否阻断发布：是
- Owner：SEC + QA（待指派实名）
