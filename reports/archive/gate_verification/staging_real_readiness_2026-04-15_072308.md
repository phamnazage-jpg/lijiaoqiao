# 真实 STG 就绪度检查

- 时间戳：2026-04-15_072308
- 输入环境：`scripts/supply-gate/.env.local-mock`
- 环境分类：`local-mock`
- 结果：**BLOCKED**
- 说明：at least one required check failed

| 检查项 | 结果 | 说明 | 证据 |
|---|---|---|---|
| STG-RDY-001 | PASS | 环境文件存在 | /home/long/project/立交桥/scripts/supply-gate/.env.local-mock |
| STG-RDY-002 | PASS | API_BASE_URL 已配置 | http://127.0.0.1:18080 |
| STG-RDY-003 | PASS | API_BASE_URL 非占位值 | http://127.0.0.1:18080 |
| STG-RDY-004 | FAIL | API_BASE_URL 为真实外网 STG 地址 | http://127.0.0.1:18080 (local) |
| STG-RDY-005 | PASS | owner/viewer/admin token 已配置 | all present |
| STG-RDY-006 | PASS | token 非占位值 | ok |
| STG-RDY-007 | PASS | 三类 token 建议区分角色 | distinct tokens |
| STG-RDY-008 | FAIL | API_BASE_URL 可达性 | http_code=000 |

## 结论

1. 该检查用于判定“是否具备真实 STG 放行验证前提”。
2. 若结果为 BLOCKED，不应执行真实放行口径判定。
