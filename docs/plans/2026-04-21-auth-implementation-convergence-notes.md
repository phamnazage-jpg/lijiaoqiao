# 2026-04-21 Auth Implementation Convergence Notes

## 1. 输入范围

- `gateway/internal/app/bootstrap.go`
- `gateway/internal/middleware/remote_runtime.go`
- `gateway/internal/config/config.go`
- `supply-api/internal/middleware/auth.go`
- `supply-api/internal/app/runtime.go`
- `supply-api/internal/app/bootstrap.go`
- `supply-api/internal/httpapi/supply_api.go`
- `supply-api/internal/middleware/ratelimit.go`

## 2. P1-C-01 gateway 入口与分支

### 2.1 token runtime 装配入口

来源：`gateway/internal/app/bootstrap.go:17`

1. `BuildServer` 在 `gateway/internal/app/bootstrap.go:42` 调用 `buildTokenRuntime(normalized.Auth)`。
2. `buildTokenRuntime` 在 `gateway/internal/app/bootstrap.go:157` 按 `cfg.TokenRuntimeMode` 分支：
   - `inmemory` 路径：`gateway/internal/app/bootstrap.go:162`
   - `remote_introspection` 路径：`gateway/internal/app/bootstrap.go:164`
3. `BuildServer` 把返回值同时注入 `Verifier` 和 `StatusResolver`，见 `gateway/internal/app/bootstrap.go:47`。

### 2.2 调用点

1. `BuildMux` 在 `gateway/internal/app/bootstrap.go:81` 和 `gateway/internal/app/bootstrap.go:82` 用 `BuildTokenAuthChain` 包装聊天与补全入口。
2. `remote_introspection` 实际 HTTP 调用发生在 `gateway/internal/middleware/remote_runtime.go:56`。
3. introspection 请求目标固定为 `/api/v1/platform/tokens/introspect`，见 `gateway/internal/middleware/remote_runtime.go:63`。

结论：

1. gateway 的 authority 选择点只在 `buildTokenRuntime` 一处，适合集中收口。
2. 运行时真正消费远程 principal 的只有 `RemoteTokenRuntime.Verify`，切换面相对小。

## 3. P1-C-02 supply-api 的 JWT 校验、状态检查与 principal 注入点

### 3.1 JWT 校验

1. `BearerExtractMiddleware` 在 `supply-api/internal/middleware/auth.go:275` 提取 Bearer token，并在 `supply-api/internal/middleware/auth.go:315` 写入 `bearerTokenKey`。
2. `TokenVerifyMiddleware` 在 `supply-api/internal/middleware/auth.go:322` 启动完整鉴权链。
3. 真正的 JWT 解析发生在 `supply-api/internal/middleware/auth.go:515`，通过 `jwt.ParseWithClaims` 解析 `TokenClaims`。

### 3.2 token 状态检查

1. `TokenVerifyMiddleware` 在 `supply-api/internal/middleware/auth.go:389` 调用 `m.checkTokenStatus(r.Context(), claims.ID)`。
2. `checkTokenStatus` 最终走 `m.tokenBackend.CheckTokenStatus`，见 `supply-api/internal/middleware/auth.go:596` 和 `supply-api/internal/middleware/auth.go:606`。

### 3.3 principal 注入

1. JWT claims 写入 context 的位置在 `supply-api/internal/middleware/auth.go:428`。
2. `tenant_id` 注入点在 `supply-api/internal/middleware/auth.go:429`。
3. `operator_id` 注入点在 `supply-api/internal/middleware/auth.go:430`。

结论：

1. `supply-api` 目前仍然自己承担 JWT authority: 解析、验签、状态查询、principal 注入都在本地完成。
2. 如果要迁到 principal consumer，关键收口点就是 `TokenVerifyMiddleware` 和 `checkTokenStatus`。

## 4. P1-C-03 supply-api token backend 装配点

### 4.1 DB-backed / memory-backed store 装配

1. `buildStoreBundle` 在 `supply-api/internal/app/runtime.go:286` 按 `db != nil` 分支：
   - DB-backed：`supply-api/internal/app/runtime.go:287`
   - memory-backed：`supply-api/internal/app/runtime.go:295`
2. `buildDBStoreBundle` 在 `supply-api/internal/app/runtime.go:302` 创建 `tokenStatusRepo`。
3. memory bundle 不创建 `tokenStatusRepo`，见 `supply-api/internal/app/runtime.go:321`。

### 4.2 token backend 与鉴权中间件装配

1. `buildSecurityBundle` 在 `supply-api/internal/app/runtime.go:332` 统一装配鉴权依赖。
2. DB-backed token backend 分支在 `supply-api/internal/app/runtime.go:344` 到 `supply-api/internal/app/runtime.go:348`。
3. memory-backed token backend 分支在 `supply-api/internal/app/runtime.go:349` 到 `supply-api/internal/app/runtime.go:351`。
4. `NewAuthMiddleware` 的装配位置在 `supply-api/internal/app/runtime.go:355`。

### 4.3 HTTP 启动链路

1. 非 `dev` 环境必须有 `AuthMiddleware`，见 `supply-api/internal/app/bootstrap.go:108`。
2. 真正的 middleware 链在 `supply-api/internal/app/bootstrap.go:167` 开始构建。
3. 非 `dev` 环境当前固定接入：
   - `QueryKeyRejectMiddleware`：`supply-api/internal/app/bootstrap.go:178`
   - `BearerExtractMiddleware`：`supply-api/internal/app/bootstrap.go:177`
   - `TokenVerifyMiddleware`：`supply-api/internal/app/bootstrap.go:176`

结论：

1. `supply-api` 的 token authority 不是散落的，而是通过 `buildSecurityBundle` 和 `buildMiddlewareChain` 这两处装配成型。
2. DB-backed 与 memory-backed 的切换目前只影响 token 状态后端，不影响本地 JWT 验签语义。

## 5. P1-C-04 gateway 非 dev 禁用本地 authority 改动清单

### 5.1 `config`

1. 收紧 `gateway/internal/config/config.go:225` 的 `ValidateAuthConfig`:
   - 从“仅 `prod` / `staging` 禁用 `inmemory`”改为“只要 `env != dev` 就禁用 `inmemory`”。
   - 保证 `qa`、`pre`、`demo`、`online` 等共享环境也不能漏过。
2. 调整 `gateway/internal/config/config.go:170` 的默认值策略：
   - `dev` 允许默认 `inmemory`
   - 非 `dev` 环境必须显式配置 `remote_introspection`

### 5.2 `bootstrap`

1. 给 `gateway/internal/app/bootstrap.go:157` 的 `buildTokenRuntime` 增加环境级兜底校验，避免绕过 `ValidateAuthConfig` 后仍能落到 `inmemory` 分支。
2. 审视 `gateway/internal/app/bootstrap.go:217` 的默认值写入，避免 normalize 之后把共享环境误归到 `inmemory`。
3. 保持 `gateway/internal/middleware/remote_runtime.go:56` 为唯一非 `dev` 认证入口，不再保留第二条本地 authority 路径。

### 5.3 `tests`

1. 扩展 `gateway/internal/config/config_test.go`:
   - 增加 `qa` / `demo` / `online` 等非 `dev` 环境禁用 `inmemory` 的用例。
2. 扩展 `gateway/internal/app/bootstrap_test.go`:
   - 校验非 `dev` 环境传入 `inmemory` 时构建失败。
3. 扩展 `gateway/internal/middleware/remote_runtime_test.go`:
   - 校验远程 introspection 新字段收敛后仍能正确反序列化 principal。

## 6. P1-C-05 supply-api 从 JWT authority 迁到 principal consumer 的改动清单

### 6.1 `middleware`

1. 把 `TokenClaims` 从“JWT claims”收敛为“canonical principal”模型，降低对本地 JWT 结构的耦合。
2. 在 `supply-api/internal/middleware/auth.go` 替换本地 `verifyToken` + `checkTokenStatus` 组合：
   - 不再本地验签 JWT
   - 不再通过 `tokenBackend` 查询 token 状态
   - 改为消费来自 gateway / 上游 trusted hop 的 canonical principal
3. 保留 `QueryKeyRejectMiddleware`、scope / role 授权与 context 注入，但输入从 JWT claims 改成 principal。

### 6.2 `runtime`

1. 收缩 `supply-api/internal/app/runtime.go:332` 的 `buildSecurityBundle`:
   - 删除 DB-backed / memory-backed token status backend 的双轨装配
   - 把安全装配收敛为 principal consumer middleware + 审计适配器
2. 移除 `tokenStatusRepo` 对 auth 装配的硬依赖，让 DB 是否可用不再决定 authority 语义。

### 6.3 `HTTP handler`

1. 保持 handler 只依赖 context 中的 principal 字段，不依赖 JWT 细节：
   - `supply-api/internal/httpapi/supply_api.go:111` 的 `resolveSupplierID`
   - `supply-api/internal/middleware/ratelimit.go:233` 的租户限流键
   - `supply-api/internal/middleware/idempotency.go:280` 的 tenant/operator 注入消费者
2. 审核所有直接读取 `GetTokenClaims` 的调用点，改为读取统一 principal / tenant / operator 上下文。

### 6.4 `tests`

1. `supply-api/internal/middleware/auth_test.go`
   - 从 JWT 验签场景改为 principal 消费场景
   - 增加缺 principal、principal 不完整、scope / role 拒绝的断言
2. `supply-api/internal/app/runtime_test.go`
   - 删除 token backend 双轨装配前提
   - 新增 principal consumer middleware 装配断言
3. `supply-api/internal/httpapi/supply_api_test.go`
   - 验证 handler 继续只依赖 context 中的 tenant / operator

## 7. P1-C-06 过渡期兼容策略

策略：`单写 + 双读短窗 + 一次性切断旧 JWT`

### 7.1 单写

1. token 生命周期只允许 `platform-token-runtime` 写入和解释。
2. gateway 只通过 `remote_introspection` 获取 canonical principal。

### 7.2 双读短窗

1. 过渡期让 `supply-api` 优先读取新 principal 通道。
2. 仅在 trusted internal 流量下保留旧 JWT 读取兜底，用于灰度和回滚。
3. 双读期间禁止新增任何本地 token 签发 / 刷新 / 吊销逻辑。

### 7.3 一次性切断

1. 当 gateway 与 supply-api 的 principal 通道通过回归验证后，移除：
   - `verifyToken`
   - `tokenBackend`
   - 本地 JWT claims 依赖
2. 切断后仅保留 principal consumer 路径，避免长期双轨。

为什么不用长期双轨：

1. 长期双轨会让 authority 再次分叉，和 Phase 1 目标冲突。
2. 运维上会多一套签名密钥、状态后端和回归矩阵，违背“运维更简单”。

## 8. P1-C-07 回滚条件

回滚目标契约：`兼容窗口契约 v1`

定义：

1. `platform-token-runtime` 仍是唯一 token 生命周期写入方。
2. gateway 仍固定使用 `remote_introspection`。
3. supply-api 回退到“新 principal 优先、旧 JWT 兜底”的双读兼容版本，不回退到本地签发 / 刷新 / 吊销 token。

触发回滚的条件：

1. 合法 token contract 场景出现持续失败，且 15 分钟内无法通过配置修复。
2. 吊销 token 在 gateway 或 supply-api 侧出现放行，形成安全回归。
3. principal 字段缺失导致租户、operator、scope 任一关键上下文为空，影响写接口正确性。
4. token runtime 不可用时，错误码或超时行为偏离 gate 约束，导致上游无法稳定降级。
5. 发布后出现无法通过兼容窗口热修的 P0/P1 认证事故。

回滚动作：

1. 立即停止“只读 principal”后的切流，恢复到双读兼容版本。
2. 保留 gateway `remote_introspection`，禁止把 `inmemory` 重新带回共享环境。
3. 回滚到最近一个通过 Phase 1 contract gate 的兼容窗口版本，而不是回滚到旧的多 authority 设计。

## 9. P1-C-08 README / ADR 兼容窗口说明草稿

### 9.1 上线前

在兼容窗口开始前，`platform-token-runtime` 已是唯一 token authority，gateway 非 `dev` 环境只允许 `remote_introspection`。supply-api 将进入“新 principal 优先、旧 JWT 兜底”的短期双读阶段，用于灰度验证 principal 通道，不再新增任何本地 token 生命周期逻辑。

### 9.2 上线中

兼容窗口期间，gateway 持续从 `platform-token-runtime` 拉取 canonical principal。supply-api 优先消费 principal 通道；若仅在 trusted internal 流量下发现旧 JWT 仍被依赖，可临时走兜底分支，但必须保持单写、禁止扩散双轨，并以 contract gate 结果作为是否继续切流的唯一依据。

### 9.3 上线后

当 contract tests、回归验证和运行观测都稳定后，移除 supply-api 的旧 JWT 兜底路径，只保留 principal consumer 实现。上线后文档、README 和 ADR 必须统一声明：共享环境不存在本地 token authority，token 生命周期、状态解释和 introspection 字段都以 `platform-token-runtime` 为单一真源。
