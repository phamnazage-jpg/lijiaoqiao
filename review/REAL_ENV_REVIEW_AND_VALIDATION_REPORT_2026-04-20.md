# 项目真实环境 Review 与全面验证报告

生成时间：2026-04-20
仓库路径：`/home/long/project/立交桥`
执行方式：只读 review + 真实运行验证
约束说明：本次未修改仓库业务代码；仅使用 `/tmp` 下临时脚本、临时辅助程序、临时进程，以及隔离的 Podman PostgreSQL 容器完成验证。

状态说明：本文记录的是 2026-04-20 的整改前真实验证基线快照。文中缺陷已拆入 [2026-04-20-remediation-tasklist-from-real-validation.md](/home/long/project/立交桥/docs/plans/2026-04-20-remediation-tasklist-from-real-validation.md)，后续修复状态应以任务单执行提交和最终验收结果为准。

后续状态更新：截至 2026-04-20 本次复核结束时，本文中已拆入整改任务单的 `13` 个已证实问题已完成修复并通过复核。`gateway` 与 `platform-token-runtime` 入口日志尚未统一为结构化 JSON，这属于任务单外新增治理项，不构成“本文已证实缺陷仍未修复”的反证。

## 1. 执行摘要

本轮验证覆盖了 `gateway`、`platform-token-runtime`、`supply-api` 三个核心后端服务，验证方式包括：

- 无缓存基线测试复跑
- 真实 PostgreSQL 落库
- 数万条业务/审计/令牌数据构造
- 真实服务启动
- 真实 HTTP 接口联调
- 条件能力与失败路径验证

结论如下：

- 当前仓库的三套后端服务可以在真实环境中成功启动。
- 当前仓库自带的核心基线测试在复跑后通过。
- 当前系统的读路径和部分管理路径可用。
- 当前系统并不能判定为“所有功能均正常”。
- 已确认 6 个确定性缺陷，涉及：
  - `platform-token-runtime` 的 PostgreSQL 刷新/撤销路径
  - `supply-api` 的幂等锁写入路径
  - `supply-api` 的套餐创建 SQL
  - `IAM` 初始化 DDL
  - `IAM` DB-backed 查询的空值扫描
  - `audit_events` 表结构与审计仓储实现不一致

## 2. 验证范围

### 2.1 核心服务

- `gateway`
- `platform-token-runtime`
- `supply-api`

### 2.2 基线验证

- [repo_integrity_check.sh](/home/long/project/立交桥/scripts/ci/repo_integrity_check.sh)
- `gateway` 无缓存 `go test -count=1 ./...`
- `platform-token-runtime` 无缓存 `go test -count=1 ./...`
- `supply-api` 无缓存 `go test -count=1 ./...`
- `supply-api` 仓储集成测试
- `supply-api` `e2e` 测试

### 2.3 真实运行验证

- 真实 PostgreSQL 容器
- 真实 schema 初始化
- 真实种子数据写入
- 真实服务启动
- 真实 API 请求验证
- 正向流程、权限校验、条件关闭、失败路径验证

## 3. 验证环境

### 3.1 数据库环境

本机 `5432` 端口存在监听，但 PostgreSQL 健康检查不可用，因此未采用本机数据库。

本次使用 Podman 启动隔离 PostgreSQL 15 容器：

- 容器名：`lijiaoqiao-review-pg`
- 映射端口：`15441`
- 验证数据库：
  - `supply_review`
  - `token_runtime_review`

### 3.2 前后端启动说明

本仓库未发现项目自有前端源码入口。当前仅发现归档竞品前端目录：

- [package.json](/home/long/project/立交桥/llm-gateway-competitors/sub2api-tar/frontend/package.json)

因此本轮只能对项目后端进行真实启动与联调，不能把不存在的前端能力写成“已验证”。

### 3.3 服务启动结果

真实启动成功的服务如下：

- `gateway`：`127.0.0.1:18080`
- `platform-token-runtime`：`127.0.0.1:18081`
- `supply-api`：`127.0.0.1:18082`
- 上游模拟服务：`127.0.0.1:19090`

健康检查结果：

- `platform-token-runtime`：`UP`
- `supply-api`：数据库与缓存检查为 `ok`

## 4. 数据规模与角色覆盖

### 4.1 数据规模

本次构造并写入的真实测试数据量如下：

- 租户：`120`
- IAM 用户：`2,120`
- 供应账号：`6,000`
- 套餐：`18,000`
- 使用记录：`50,000`
- 审计事件：`30,000`
- 告警：`2,500`
- 平台 Token：`20,000`
- Token 审计事件：`20,000`

### 4.2 角色与调用方

本次覆盖的真实调用方包括：

- 匿名请求方
- 网关 Bearer Token 调用方
- 平台 Token 管理侧
- 供应侧组织管理员
- 结算侧租户管理员

### 4.3 模型覆盖

本次按系统真实注册模型进行验证，未虚构不存在模型。已联调模型如下：

- `gpt-4o`
- `gpt-4.1`
- `gpt-4.1-mini`
- `claude-3-7-sonnet`
- `deepseek-chat`

## 5. 已通过项

### 5.1 基线测试

以下验证在复跑后通过：

- `bash scripts/ci/repo_integrity_check.sh`
- `cd supply-api && bash scripts/run_integration_tests.sh ./internal/repository`
- `cd supply-api && go test -count=1 -tags=e2e ./e2e`

说明：

- 首次执行统一校验时，`supply-api/internal/domain` 出现过一次未复现失败。
- 随后的单独复跑、无缓存多次复跑、整仓复跑均通过。
- 当前更合理的判断是“存在稳定性风险信号，但尚未形成可复现确定性缺陷”。

### 5.2 gateway

已确认通过：

- `/v1/models`
- `/v1/chat/completions`
- `/v1/completions`
- 缺失 Bearer Token 时的 `401` 返回

### 5.3 platform-token-runtime

已确认通过：

- Token 签发
- Token introspect
- Audit events 查询

### 5.4 supply-api

已确认通过：

- 健康检查
- 账单查询
- 收益记录查询
- 告警创建
- 告警获取
- 告警列表
- 告警更新
- 告警解决
- 告警删除
- 结算单下载
- 提现能力条件关闭

其中提现关闭行为是设计内结果，不是 bug。对应逻辑位于：

- [runtime.go](/home/long/project/立交桥/supply-api/internal/app/runtime.go#L407)

## 6. 已证实缺陷

### 6.1 P0: platform-token-runtime 的 PostgreSQL 刷新与撤销路径失效

现象：

- `/api/v1/platform/tokens/refresh` 返回数据库错误
- `/api/v1/platform/tokens/revoke` 返回数据库错误
- Token 状态未变更为 `revoked`
- `gateway` 仍接受原 token

根因：

- PostgreSQL store 的保存逻辑在 `INSERT` 阶段先执行 `NULLIF($2, '')`
- 刷新/撤销路径调用 `Save` 时没有传回有效 access token
- 因 `token_fingerprint` 为 `NULL` 触发 `NOT NULL` 约束
- 请求在进入 `ON CONFLICT DO UPDATE` 之前即失败

证据位置：

- [postgres_runtime_store.go](/home/long/project/立交桥/platform-token-runtime/internal/auth/service/postgres_runtime_store.go#L73)
- [postgres_runtime_store.go](/home/long/project/立交桥/platform-token-runtime/internal/auth/service/postgres_runtime_store.go#L91)
- [inmemory_runtime.go](/home/long/project/立交桥/platform-token-runtime/internal/auth/service/inmemory_runtime.go#L164)
- [inmemory_runtime.go](/home/long/project/立交桥/platform-token-runtime/internal/auth/service/inmemory_runtime.go#L190)

影响判断：

- PostgreSQL-backed token runtime 不具备可靠的刷新/撤销能力
- 网关依赖该运行时做远程 introspection 时，撤销链路存在失效风险

### 6.2 P0: supply-api 幂等锁实现与表结构冲突，账号创建被阻塞

现象：

- 创建账号接口返回 `IDEMPOTENCY_LOCK_FAILED`
- PostgreSQL 明确报错：`there is no unique or exclusion constraint matching the ON CONFLICT specification`

根因：

- 仓储层按 `(tenant_id, operator_id, api_path, idempotency_key)` 做 `ON CONFLICT`
- 表定义没有该唯一约束

证据位置：

- [idempotency.go](/home/long/project/立交桥/supply-api/internal/repository/idempotency.go#L196)
- [idempotency.go](/home/long/project/立交桥/supply-api/internal/repository/idempotency.go#L217)
- [partition_strategy_v1.sql](/home/long/project/立交桥/supply-api/sql/postgresql/partition_strategy_v1.sql#L110)
- [partition_strategy_v1.sql](/home/long/project/立交桥/supply-api/sql/postgresql/partition_strategy_v1.sql#L124)

影响判断：

- 所有依赖该幂等锁的 DB-backed 写接口都可能被阻塞

### 6.3 P0: supply-api 套餐创建 SQL 占位符数量错误

现象：

- 创建套餐接口返回 SQL 语法层错误：`INSERT has more target columns than expressions`

根因：

- `INSERT INTO supply_packages` 定义了 29 个目标列
- `VALUES` 只提供了 28 个占位符

证据位置：

- [package.go](/home/long/project/立交桥/supply-api/internal/repository/package.go#L27)
- [package.go](/home/long/project/立交桥/supply-api/internal/repository/package.go#L37)
- [package.go](/home/long/project/立交桥/supply-api/internal/repository/package.go#L60)

影响判断：

- 套餐创建链路在真实数据库下不可用

### 6.4 P1: IAM schema 初始化脚本在全新数据库上失败

现象：

- 执行 [iam_schema_v1.sql](/home/long/project/立交桥/sql/postgresql/iam_schema_v1.sql) 时失败
- 默认 scope 数据中的 `'*'` 违反自身格式约束
- 整个事务回滚

证据位置：

- [iam_schema_v1.sql](/home/long/project/立交桥/sql/postgresql/iam_schema_v1.sql#L60)

影响判断：

- 仓库自带 IAM schema 不能直接用于干净环境初始化

### 6.5 P1: DB-backed IAM 查询对空值不安全

现象：

- 角色列表查询可能触发 `cannot scan NULL into *string`
- 用户角色查询可能触发 `cannot scan NULL into *int64`

根因：

- 可空列被直接扫描进非空基本类型

证据位置：

- [iam_repository.go](/home/long/project/立交桥/supply-api/internal/iam/repository/iam_repository.go#L131)
- [iam_repository.go](/home/long/project/立交桥/supply-api/internal/iam/repository/iam_repository.go#L483)
- [iam_repository.go](/home/long/project/立交桥/supply-api/internal/iam/repository/iam_repository.go#L513)

影响判断：

- DB-backed IAM 在真实数据下不稳定

### 6.6 P1: 审计仓储实现与 audit_events 表结构不一致

现象：

- 结算取消接口返回业务成功
- 但服务日志记录 `trace_id` 列不存在

根因：

- 审计仓储插入 `trace_id`、`span_id`
- 当前分区表定义没有这两个列

证据位置：

- [audit_repository.go](/home/long/project/立交桥/supply-api/internal/audit/repository/audit_repository.go#L105)
- [audit_repository.go](/home/long/project/立交桥/supply-api/internal/audit/repository/audit_repository.go#L109)
- [partition_strategy_v1.sql](/home/long/project/立交桥/supply-api/sql/postgresql/partition_strategy_v1.sql#L7)
- [partition_strategy_v1.sql](/home/long/project/立交桥/supply-api/sql/postgresql/partition_strategy_v1.sql#L15)

影响判断：

- 业务操作可能成功，但审计持久化静默失败

## 7. 当前状态判断

按真实运行结果判断：

- `gateway`：基础能力可用
- `platform-token-runtime`：查询链路可用，刷新/撤销链路不可判定为可用
- `supply-api`：读路径和部分管理路径可用，关键写路径存在明确缺陷

更准确的项目状态是：

- 不是“无法运行”
- 也不是“功能已全部正常”
- 属于“基础链路可跑，但关键变更路径仍有明确阻塞缺陷”

## 8. 结论与建议

### 8.1 结论

本次真实验证确认：

- 项目当前具备真实启动与部分真实业务联调能力
- 项目当前不具备“全功能正常”的结论基础
- 当前最需要优先修复的是写路径、PostgreSQL-backed token 变更路径、IAM schema 初始化和审计落库一致性

### 8.2 建议优先级

- `P0`
  - 修复 `platform-token-runtime` 的 PostgreSQL `refresh/revoke`
  - 修复 `supply-api` 幂等锁唯一约束与仓储实现不一致
  - 修复 `supply-api` 套餐创建 SQL 占位符错误
- `P1`
  - 修复 IAM schema 初始化脚本
  - 修复 IAM 仓储对可空字段的扫描
  - 统一 audit repository 与 `audit_events` 表结构
- `P2`
  - 继续跟踪 `supply-api/internal/domain` 的一次性不稳定失败
  - 在缺陷修复后重新执行全量接口矩阵验证

## 9. 报告边界

本报告只写本轮已证实事实，不写未验证推测。以下事项未纳入“已通过”结论：

- 不存在于当前仓库的项目自有前端
- 未真实挂载或未真实联调的接口
- 未复现的偶发性异常
