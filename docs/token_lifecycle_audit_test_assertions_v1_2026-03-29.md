# TOK-003/TOK-004 测试断言清单（生命周期 + 审计事件）

- 版本：v1.0
- 日期：2026-03-29
- 状态：开发实施测试基线
- 适用任务：`TOK-003`、`TOK-004`

## 1. 测试范围

1. TOK-003：签发、续期、吊销、过期生命周期。
2. TOK-004：签发/校验失败/吊销/越权事件入库与可追踪。

## 2. 前置数据

1. 租户：`tenant_id=1001`
2. 主体：
   - `subject_owner=2001`
   - `subject_viewer=2002`
3. 角色策略：
   - owner: `supply:*`
   - viewer: `supply:read`
4. 观测阈值：
   - 吊销生效延迟 `<=5s`
   - 审计事件落库延迟 `<=3s`

## 3. TOK-003 生命周期断言

| 用例ID | 场景 | 步骤 | 断言 |
|---|---|---|---|
| TOK-LIFE-001 | 签发成功 | 1) 调用 `POST /tokens/issue` 2) 记录返回 | 1) `status=active` 2) `expires_at>issued_at` 3) `token_id` 唯一 |
| TOK-LIFE-002 | 签发参数非法 | 1) `ttl_seconds` 超上限 2) 调用签发 | 1) 返回 `400` 2) 不落 active token |
| TOK-LIFE-003 | 同键幂等签发重放 | 1) 相同 `Idempotency-Key` 重复提交 | 1) 返回同一 `token_id` 2) 无重复写入 |
| TOK-LIFE-004 | 续期成功 | 1) 调用 `POST /tokens/{tokenId}/refresh` | 1) `expires_at` 延后 2) `status=active` |
| TOK-LIFE-005 | 吊销成功 | 1) 调用 `POST /tokens/{tokenId}/revoke` 2) 立刻 introspect | 1) 最终 `status=revoked` 2) 生效延迟 <=5s |
| TOK-LIFE-006 | 吊销后访问受限接口 | 1) 使用被吊销 token 访问受保护路由 | 1) 返回 `401 AUTH_TOKEN_INACTIVE` |
| TOK-LIFE-007 | 过期自动失效 | 1) 签发短 TTL token 2) 等待过期 3) introspect | 1) `status=expired` 2) 返回不可用错误 |
| TOK-LIFE-008 | viewer 越权写操作 | 1) viewer token 调用写接口 | 1) 返回 `403 AUTH_SCOPE_DENIED` 2) 无写入副作用 |

## 4. TOK-004 审计事件断言

| 用例ID | 场景 | 步骤 | 断言 |
|---|---|---|---|
| TOK-AUD-001 | 签发成功事件 | 执行 TOK-LIFE-001 | 1) 存在 `token.issue.success` 2) 字段齐全 |
| TOK-AUD-002 | 签发失败事件 | 执行 TOK-LIFE-002 | 1) 存在 `token.issue.fail` 2) `result_code` 准确 |
| TOK-AUD-003 | 鉴权失败事件 | 无效 token 访问受保护路由 | 1) `token.authn.fail` 入库 2) 含 `request_id` |
| TOK-AUD-004 | 越权事件 | 执行 TOK-LIFE-008 | 1) `token.authz.denied` 入库 2) 含 `subject_id` |
| TOK-AUD-005 | 吊销事件 | 执行 TOK-LIFE-005 | 1) `token.revoke.success` 入库 2) 含 `token_id` |
| TOK-AUD-006 | query key 拒绝事件 | 使用 query key 访问接口 | 1) `token.query_key.rejected` 入库 2) 不出现敏感值 |
| TOK-AUD-007 | 事件不可篡改 | 重复读取同 `event_id` | 1) 核心字段不可变 2) 时间顺序正确 |

## 5. 字段级硬断言

每条审计事件必须包含：
1. `event_id`
2. `request_id`
3. `result_code`
4. `route`
5. `created_at`

可选字段规则：
1. `token_id`：提取失败场景可空，其余场景必填。
2. `subject_id`：匿名失败场景可空，其余场景必填。

禁止项：
1. 不得写入上游供应方凭证明文。
2. 不得写入完整 `access_token` 明文（仅允许哈希或指纹）。

## 6. 结果判定

1. TOK-003 通过标准：
   - `TOK-LIFE-*` 全通过
   - 吊销延迟阈值满足 `<=5s`
2. TOK-004 通过标准：
   - `TOK-AUD-*` 全通过
   - 审计字段完整率 `=100%`
   - 敏感数据泄露事件 `=0`
