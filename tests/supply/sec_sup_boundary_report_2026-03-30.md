# SEC-SUP 边界回归报告

- 日期：2026-03-30
- 覆盖用例：SEC-SUP-001~002
- 指标映射：M-013/M-014/M-015/M-016
- 执行环境：local-mock (`http://127.0.0.1:18080`)

## 1. 执行结果

| 用例ID | 结果 | 备注 |
|---|---|---|
| SEC-SUP-001 | PASS | 平台凭证主路径可用，脱敏扫描通过 |
| SEC-SUP-002 | PASS | 外部 query key 被拒绝（HTTP 403） |

## 2. 指标结果

| 指标ID | 实际值 | 目标值 | 结论 |
|---|---|---|---|
| M-013 | 0 | 0 | PASS |
| M-014 | 100% | 100% | PASS |
| M-015 | 0（未配置直连探测目标，未发现事件） | 0 | PASS（mock） |
| M-016 | 100%（外部 query key 拒绝） | 100% | PASS |

## 3. 证据

1. 脱敏扫描报告路径：
   `tests/supply/artifacts/sup007/04_redaction_scan.txt`
2. 鉴权日志路径：
   `tests/supply/artifacts/sup007/01_main_path_with_platform_token.json`
3. 拦截日志路径：
   `tests/supply/artifacts/sup007/02_external_query_key_attempt.txt`
4. 安全事件路径：
   本轮未发现安全事件

## 4. 结论

- 是否触发P0：否
- 是否阻断发布：否（仅 local-mock）
- Owner：周敏（SEC）+孙悦（QA）
