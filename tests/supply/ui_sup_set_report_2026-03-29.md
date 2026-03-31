# UI-SUP-SET 执行报告（结算提现链路）

- 日期：2026-03-29
- 覆盖用例：UI-SUP-SET-001~005
- 执行环境：local-mock (`http://127.0.0.1:18080`)

## 1. 执行结果汇总

| 用例ID | 结果 | 备注 |
|---|---|---|
| UI-SUP-SET-001 | PASS | 账单查询返回 `summary` |
| UI-SUP-SET-002 | PASS | 提现创建成功，`settlement_id=3000` |
| UI-SUP-SET-003 | PASS | 撤销成功，状态 `cancelled` |
| UI-SUP-SET-004 | PASS | 对账单返回 `download_url` |
| UI-SUP-SET-005 | PASS | 收益流水返回分页记录 |

## 2. 证据

1. 请求/响应日志路径：
   `tests/supply/artifacts/sup006/*.json`
2. 截图/录屏路径：
   本轮为 API 脚本执行，不含 UI 录屏
3. 审计事件截图路径：
   `tests/supply/artifacts/sup006/summary.txt`

## 3. 结论

- 通过率：5/5（100%）
- 是否阻断发布（是/否）：否（仅 local-mock）
- Owner：孙悦（QA）+何静（FIN）
