# 2026-04-21 Env Normalization Checklist

## P2-C-01 三服务环境枚举与别名盘点

1. `gateway`
   - 当前输入源是 `GATEWAY_ENV`。
   - 共享环境判定历史上同时接受 `production`、`prod`、`online`。
   - `staging` 也被视为非开发共享环境，禁止 `inmemory` token runtime。
2. `supply-api`
   - `ResolveEnv` 只接受 `dev`、`staging`、`prod`。
   - 默认配置路径按 `config.<env>.yaml` 组装。
   - 现有 devtest 脚本曾存在 `-env=staging -config=config.dev.yaml` 的错配。
3. `platform-token-runtime`
   - `TOKEN_RUNTIME_ENV` 只接受 `dev`、`staging`、`prod`。
   - `staging/prod` 需要显式 PostgreSQL store，`dev` 才允许内存 store。
4. 本次盘点覆盖的枚举全集：
   - 规范值：`dev`、`staging`、`prod`
   - 兼容别名：`production`、`online`

## P2-C-02 统一枚举

仓库内部只保留三种规范值：

1. `dev`
2. `staging`
3. `prod`

统一规则：

1. `production`、`online` 只允许作为兼容输入存在。
2. 兼容输入一进入服务边界就必须立即折叠为 `prod`。
3. 文档、模板、脚本、报告、CI 产物里不再引入新的别名。

## P2-C-03 gateway 归一化入口

唯一入口定在 `gateway/internal/config/config.go`：

1. `LoadConfig` 读取 `GATEWAY_ENV` 后立刻归一化。
2. `ValidateAuthConfig` 与启动安全校验只消费归一化后的值。
3. `bootstrap.go` 不再维护独立的 `production/online` 判定分支。

## P2-C-04 supply-api 错配拒绝规则

拒绝条件：

1. 当 `-env` 为 `staging` 或 `prod` 时，`-config` 不得指向 `config.dev.yaml` 或 `config.dev.yml`。

错误信息草稿：

`config path "<path>" is a dev template and cannot be used with -env=<env>; use config.<env>.yaml or a <env> example template instead`

## P2-C-05 staging 模板骨架

`supply-api/config/config.staging.example.yaml` 至少保留以下段落：

1. `server`
2. `database`
3. `redis`
4. `token`
5. `audit`

## P2-C-06 prod 模板骨架

`supply-api/config/config.prod.example.yaml` 必须额外显式包含：

1. `server.default_supplier_id: 0`
2. `token.algorithm: RS256`
3. `token.public_key`
4. `settlement.withdraw_enabled`
5. `sms` 占位字段

## P2-C-07 devtest 参数设计

`scripts/devtest/start_dev_stack.sh` 改为：

1. 使用 `LIJIAOQIAO_DEVTEST_SUPPLY_CONFIG` 作为 `supply-api` 配置路径参数。
2. 默认值改为 `./config/config.staging.example.yaml`。
3. 不再把 `config.dev.yaml` 硬编码进 staging 启动链路。

## P2-C-08 环境归一化检查清单

### 启动前检查

1. 所有新文档和样例文件只出现 `dev`、`staging`、`prod` 三种规范值。
2. 兼容别名只存在于输入归一化代码，不存在于模板与脚本默认值。
3. `supply-api` 的 `staging/prod` 启动命令不引用 `config.dev.yaml`。

### 启动后检查

1. `gateway` 在 `production` 或 `online` 输入下对外表现为 `prod` 语义。
2. `supply-api` 在 `-env=staging -config=config.dev.yaml` 时必须 fail fast。
3. `platform-token-runtime` 仍然只接受 `dev/staging/prod`，不新增额外别名。

### CI 检查

1. `gateway` 运行环境归一化相关单测。
2. `supply-api` 运行命令行错配拒绝测试。
3. `scripts/devtest/start_dev_stack.sh` 至少通过 `bash -n` 语法检查。
