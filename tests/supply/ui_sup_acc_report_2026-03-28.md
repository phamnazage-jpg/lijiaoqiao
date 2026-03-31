# UI-SUP-ACC 执行报告（账号挂载链路）

- 日期：2026-03-28
- 覆盖用例：UI-SUP-ACC-001~006
- 执行环境：local-mock (`http://127.0.0.1:18080`)

## 1. 执行结果汇总

| 用例ID | 结果 | 备注 |
|---|---|---|
| UI-SUP-ACC-001 | PASS | `verify_status=pass`，返回风险等级 |
| UI-SUP-ACC-002 | PASS | 创建账号成功，返回 `account_id=1000` |
| UI-SUP-ACC-003 | PASS | 激活成功，状态 `active` |
| UI-SUP-ACC-004 | PASS | 暂停成功，状态 `suspended` |
| UI-SUP-ACC-005 | PASS | 审计日志可查，返回 `request_id` |
| UI-SUP-ACC-006 | PASS | 全链路执行成功，产物齐全 |

## 2. 证据

1. 请求/响应日志路径：
   `tests/supply/artifacts/sup004/*.json`
2. 截图/录屏路径：
   本轮为 API 脚本执行，不含 UI 录屏
3. 审计事件截图路径：
   `tests/supply/artifacts/sup004/05_audit_logs.json`

## 3. 结论

- 通过率：6/6（100%）
- 是否阻断发布（是/否）：否（仅 local-mock）
- Owner：孙悦（QA）
