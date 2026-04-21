# Gateway / Token Runtime / Supply API Contract

## P1-D-01 当前覆盖缺口

当前 CI 覆盖不足点：

1. 没有任何脚本验证 gateway 的鉴权结果与 `platform-token-runtime` introspection 结果是否一致。
2. 没有任何脚本验证吊销 token 后，gateway 与 supply-api 是否同步拒绝。
3. 没有任何脚本验证 principal 的关键字段是否沿链路保留，至少包括 `token_id`、`subject_id`、`tenant_id`、`scope`、`status`。
4. 没有任何脚本验证 token runtime 不可用时的统一失败语义。

## P1-D-02 场景 1：合法 token

### 入口

1. `platform-token-runtime`
   `POST /api/v1/platform/tokens/introspect`
2. `gateway`
   `POST /v1/chat/completions`
   说明：contract harness 需要配 mock provider，避免真实上游造成不稳定。
3. `supply-api`
   `POST /api/v1/supply/accounts/verify`

### 前置条件

1. token 为 `active`
2. principal 至少包含：
   - `token_id`
   - `subject_id`
   - `tenant_id`
   - `role`
   - `scope`
   - `issued_at`
   - `expires_at`
   - `status`

### 期望行为

1. `platform-token-runtime` 返回 `200`
2. `gateway` 返回 `200`
3. `supply-api` 返回 `200`
4. gateway 与 supply-api 的行为必须基于同一份 canonical principal，而不是各自重解释 token

## P1-D-03 场景 2：吊销 token

### 入口

1. 先执行 `POST /api/v1/platform/tokens/{tokenId}/revoke`
2. 再访问：
   - `POST /v1/chat/completions`
   - `POST /api/v1/supply/accounts/verify`

### 期望行为

1. `platform-token-runtime` introspection 返回 `status=revoked` 或拒绝该 token
2. `gateway` 返回 `401`
3. `supply-api` 返回 `401`
4. 错误语义至少要表现为 `AUTH_TOKEN_INACTIVE` 或与之等价的拒绝码，不能出现放行

## P1-D-04 场景 3：scope 不足

### 入口

1. 使用 scope 缺失的 token 访问受保护入口：
   - `POST /v1/chat/completions`
   - `POST /api/v1/supply/accounts`

### principal 关键字段

1. `token_id`
2. `subject_id`
3. `tenant_id`
4. `scope`
5. `role`

### 期望行为

1. gateway 或 supply-api 必须返回 `403`
2. 错误码应为 `AUTH_SCOPE_DENIED` 或等价拒绝码
3. 不允许把 scope 不足降级成 `500` 或静默放行

## P1-D-05 场景 4：token runtime 不可用

### 入口

1. 让 gateway 所依赖的 token runtime introspection 不可用
2. 再访问：
   - `POST /v1/chat/completions`
   - `POST /api/v1/supply/accounts/verify`

### 期望行为

1. gateway 入口必须在约定超时内失败，不能无限等待
2. 错误码必须稳定，可归并为 `AUTH_TOKEN_STATUS_UNAVAILABLE`、`TOKEN_INVALID` 或显式上游不可用约束
3. supply-api 若处于 principal consumer 路径，必须表现出同样稳定的失败语义
4. evidence 中必须记录入口、状态码、错误码和超时时长

## P1-D-06 backend-verify 执行位

设计：

1. 命令入口：
   `bash scripts/ci/backend-verify.sh --phase1-contract-gate`
2. 产物路径：
   - `reports/archive/gate_verification/contract_gate_<timestamp>.log`
   - `reports/archive/gate_verification/contract_gate_<timestamp>.md`
3. 失败语义：
   - 任一场景脚本退出非零即失败
   - 任一场景缺失 evidence 即失败
   - 任一关键断言不满足即失败

## P1-D-07 repo_integrity_check gate 入口

放置位置：

1. 在 shell 语法检查、三服务单测、仓储集成和 `supply-api` E2E 之后执行
2. 只有 service-local suites 先通过，才进入 contract gate
3. contract gate 失败时，`repo_integrity_check.sh` 必须整体失败

## P1-D-08 Phase 1 关闭条件

只有以下条件同时满足，Phase 1 才允许关闭：

1. 四个最小 contract 场景均有稳定 evidence
2. `backend-verify.sh` 已接入 contract gate
3. `repo_integrity_check.sh` 已接入 contract gate
4. 执行结果在 checklist 中逐项勾选完成
