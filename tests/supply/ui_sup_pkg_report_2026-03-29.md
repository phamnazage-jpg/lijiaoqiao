# UI-SUP-PKG 执行报告（套餐发布链路）

- 日期：2026-03-29
- 覆盖用例：UI-SUP-PKG-001~006
- 执行环境：local-mock (`http://127.0.0.1:18080`)

## 1. 执行结果汇总

| 用例ID | 结果 | 备注 |
|---|---|---|
| UI-SUP-PKG-001 | PASS | 草稿创建成功，`package_id=2000` |
| UI-SUP-PKG-002 | PASS | 发布成功，状态 `active` |
| UI-SUP-PKG-003 | PASS | 暂停成功，状态 `paused` |
| UI-SUP-PKG-004 | PASS | 下架成功，状态 `expired` |
| UI-SUP-PKG-005 | PASS | 批量调价回执 `success=1 failed=0 total=1` |
| UI-SUP-PKG-006 | PASS | 复制成功，返回新 `package_id=2001` |

## 2. 证据

1. 请求/响应日志路径：
   `tests/supply/artifacts/sup005/*.json`
2. 截图/录屏路径：
   本轮为 API 脚本执行，不含 UI 录屏
3. 审计事件截图路径：
   `tests/supply/artifacts/sup005/summary.txt`

## 3. 结论

- 通过率：6/6（100%）
- 是否阻断发布（是/否）：否（仅 local-mock）
- Owner：孙悦（QA）
