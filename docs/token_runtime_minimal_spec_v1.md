# Token 运行态最小实现规格（TOK-001）

- 版本：v1.0
- 日期：2026-03-27
- 状态：开发实施基线
- 对应任务：`TOK-001`

## 1. 目标

在不依赖真实 staging 参数的前提下，定义可落地的 token 运行态最小能力集，为后续 TOK-002~TOK-007 提供统一实施输入。

## 2. 最小能力范围（MVP）

1. 平台签发：短期访问 token（owner/viewer/admin）。
2. 入站校验：仅平台凭证有效，拒绝 query key 外部入站。
3. 生命周期：签发、续期、吊销、过期。
4. 边界审计：签发/校验失败/吊销/越权事件全量入审计。
5. 指标可观测：可计算 M-013~M-016 与 M-021。

## 3. 角色与权限

| 角色 | 能力 | 约束 |
|---|---|---|
| owner | 管理供应侧账号、套餐、结算 | 不可读取上游凭证明文 |
| viewer | 只读查询 | 不可执行写操作 |
| admin | 风控与审计管理 | 仅平台内部可用 |

## 4. Token 数据模型（最小字段）

| 字段 | 类型 | 说明 |
|---|---|---|
| token_id | string | 平台内部唯一标识 |
| subject_id | string | 用户/服务主体ID |
| role | string | owner/viewer/admin |
| issued_at | datetime | 签发时间 |
| expires_at | datetime | 过期时间 |
| status | string | active/revoked/expired |
| scope | string[] | 授权范围 |
| request_id | string | 请求追踪ID |
| revoked_reason | string | 吊销原因（可空） |

## 5. 生命周期状态机

`active -> revoked -> expired`

规则：
1. `revoked` 不可恢复为 `active`，需重新签发。
2. `expires_at` 到期自动进入 `expired`。
3. 续期只能对 `active` token 生效。

## 6. 核心接口（草案）

1. `POST /api/v1/platform/tokens/issue`
2. `POST /api/v1/platform/tokens/{tokenId}/refresh`
3. `POST /api/v1/platform/tokens/{tokenId}/revoke`
4. `POST /api/v1/platform/tokens/introspect`
5. `GET /api/v1/platform/tokens/audit-events`

返回要求：
1. 不回传任何上游供应方凭证。
2. 错误码需区分：无效、过期、越权、吊销。
3. 审计查询接口仅返回审计字段，不返回 access token 或任何上游凭证明文。

## 7. 安全约束

1. token 存储需采用哈希或加密指纹，禁止明文落库。
2. 校验路径必须记录 `request_id` 与调用来源。
3. 外部 query key 入站请求必须拒绝并记录事件。
4. 任一泄露事件触发 P0。

## 8. 审计事件最小集

1. `token.issue.success/fail`
2. `token.introspect.success/fail`
3. `token.refresh.success/fail`
4. `token.revoke.success/fail`
5. `token.authz.denied`

审计字段：
1. `event_id`
2. `request_id`
3. `operator_id`
4. `subject_id`
5. `token_id`
6. `result_code`
7. `created_at`

## 9. 验收标准（TOK-001 关闭条件）

1. 本规格被 `ARCH + SEC + PLAT` 确认并引用到执行任务单。
2. 后续 TOK-002~TOK-004 的实现字段与本规格一致。
3. 不得新增“直接向终端用户分发上游 token”的路径。
