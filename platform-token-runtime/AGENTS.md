# Platform-Token-Runtime 模块规则

## 模块定位

`platform-token-runtime` 是 token 生命周期、introspection 与审计查询的真源服务。这里承载的是身份与授权边界，不是普通业务接口。默认必须以 authority 的严肃程度来设计、修改和验证。

任何在这里的错误，都可能直接影响鉴权正确性、审计可信性和整个系统的安全边界。

## 第一原则

1. authority 必须单一真源。
token 的签发、刷新、撤销、状态解释和 introspection 语义必须在这里集中收口，不能让其他服务复制或发散这些语义。

2. 字段边界必须稳定。
canonical principal 的字段集合、含义、缺省行为和响应格式都是契约。变更默认是高风险。

3. 安全默认值优先。
涉及 token、审计、身份边界时，默认 fail-closed；不能用“返回空”“假成功”“先兼容一下”代替明确拒绝。

4. 明文敏感数据绝不外泄。
无论是响应、日志、错误、审计还是调试输出，都不能暴露 access token 明文。

## 变更分类

### 协议契约变更

- `issue` / `refresh` / `revoke` / `introspect` / `audit-events`
- principal 字段
- 状态枚举
- 错误码/错误响应

这些改动默认必须视为外部契约变更。

### 存储层变更

- runtime store
- audit store
- PostgreSQL schema / DDL
- 内存实现与数据库实现的行为一致性

这些改动必须同时考虑迁移、安全、兼容与查询语义。

## 验证要求

### 至少覆盖

- token 生命周期主路径
- 无效 token / 过期 token / 撤销 token 路径
- `dev` 与 `staging/prod` 下 store 装配差异
- 数据库未配置时的行为
- 审计查询返回语义

### 涉及 principal 字段时

- 必须同步检查 DDL、存储模型、HTTP 输出、OpenAPI 或文档说明
- 必须验证不会因字段漂移导致 `gateway` 解析错误

### 涉及存储时

- 必须确认内存实现与 PostgreSQL 实现的关键行为一致
- 不能只修一个 backend

## 文档规则

- 只记录当前真实 authority 行为
- 明确哪些接口、字段和边界是 canonical
- 对环境差异、快速失败条件、默认监听端口和装配逻辑要写清楚

## 禁止事项

- 不要在任何输出中泄露 token 明文
- 不要把 query key、api_key 等旁路鉴权方式偷偷加回来
- 不要让 `staging/prod` 在缺少关键依赖时静默回退到内存实现
- 不要在未同步下游契约的前提下调整 principal 边界

