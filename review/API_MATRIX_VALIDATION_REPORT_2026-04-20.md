# 接口矩阵真实验证报告

生成时间：2026-04-20
仓库路径：`/home/long/project/立交桥`
验证方式：真实启动 + 真实 PostgreSQL + 真实 HTTP 请求 + 大样本种子数据
业务代码变更：无

状态说明：本文记录的是 2026-04-20 的接口矩阵基线快照，不代表整改后的当前状态。整改执行与修复完成情况请以 [2026-04-20-remediation-tasklist-from-real-validation.md](/home/long/project/立交桥/docs/plans/2026-04-20-remediation-tasklist-from-real-validation.md) 和后续提交为准。

后续状态更新：截至 2026-04-20 当前分支收口时，本文中已拆入整改任务单的 `13` 个已证实问题已完成修复并通过复核。此前单独跟踪的结构化日志统一事项也已在三套服务入口完成，但它不属于本文当日接口缺陷矩阵范围，因此不影响本文“基线快照”属性。

关联总报告：

- [REAL_ENV_REVIEW_AND_VALIDATION_REPORT_2026-04-20.md](/home/long/project/立交桥/review/REAL_ENV_REVIEW_AND_VALIDATION_REPORT_2026-04-20.md)

原始矩阵工件：

- `/tmp/lijiaoqiao-api-matrix/api_matrix_raw_20260420_101015.md`

## 1. 执行结论

本轮接口矩阵验证覆盖了当前主启动链路中已真实挂载的核心接口，结论如下：

- `gateway`：`8/8` 接口通过，基础网关链路正常。
- `platform-token-runtime`：`6` 个接口中 `4` 个通过，`refresh` / `revoke` 两个变更接口失败。
- `supply-api`：共记录 `39` 个验证项，其中 `22` 个通过、`16` 个失败、`1` 个按设计条件关闭。
- `supply-api` 的告警接口链路在正确请求契约下完整通过。
- `supply-api` 的 IAM 接口不是“全部失败”，而是“列表/用户角色/更新/分配存在明确实现缺陷，创建/获取/删除/Scope 列表可用”。

更准确的项目状态是：

- 基础读路径和部分管理路径可用。
- 多个写路径、状态流转路径、DB-backed 审计与 IAM 路径仍然不可靠。

## 2. 验证环境与口径

### 2.1 启动环境

- 采用隔离 Podman PostgreSQL 15 容器，端口 `15441`
- 启动服务：
  - `gateway` `:18080`
  - `platform-token-runtime` `:18081`
  - `supply-api` `:18082`
  - 模拟上游 `:19090`

### 2.2 数据规模

本轮沿用真实大样本数据：

- `120` 租户
- `2,120` IAM 用户
- `6,000` 供应账号
- `18,000` 套餐
- `50,000` usage 记录
- `30,000` 审计事件
- `2,500` 告警
- `20,000` 平台 Token
- `20,000` Token 审计事件

### 2.3 判定口径

- `通过`：接口在真实请求下返回符合预期的成功结果
- `失败`：接口在正确请求契约和合理样本下返回错误，且已证实为实现缺陷或 DDL 缺陷
- `条件关闭`：接口被显式门禁关闭，属于设计内行为，不记为 bug

## 3. 关键新增发现

与上一版总报告相比，本轮接口矩阵新增或进一步坐实了以下问题。

### 3.1 P0: supply-api 账号状态流转接口存在确定性乐观锁错误

现象：

- `POST /api/v1/supply/accounts/{account_id}/activate`
- `POST /api/v1/supply/accounts/{account_id}/suspend`

两条接口都在正确样本下返回 `404`，但错误内容并非“资源不存在”，而是：

- `concurrency conflict: resource was modified by another transaction`

根因：

- 领域服务先做 `Version++`
- DB adapter 又把已经递增后的 `Version` 当作 `expectedVersion`
- Repository 再次基于 `expectedVersion + 1` 更新，并在 `WHERE version = expectedVersion` 上匹配
- 最终导致 `RowsAffected() == 0`

证据位置：

- [account.go](/home/long/project/立交桥/supply-api/internal/domain/account.go#L219)
- [account.go](/home/long/project/立交桥/supply-api/internal/domain/account.go#L221)
- [adapter.go](/home/long/project/立交桥/supply-api/internal/adapter/adapter.go#L150)
- [account.go](/home/long/project/立交桥/supply-api/internal/repository/account.go#L129)
- [account.go](/home/long/project/立交桥/supply-api/internal/repository/account.go#L143)
- [account.go](/home/long/project/立交桥/supply-api/internal/repository/account.go#L160)

影响：

- 账号激活、暂停等更新型接口在 DB-backed 运行时下不可靠

### 3.2 P0: supply-api 套餐状态流转接口存在双重问题

现象：

- `POST /api/v1/supply/packages/{package_id}/publish`
- `POST /api/v1/supply/packages/{package_id}/pause`
- `POST /api/v1/supply/packages/{package_id}/unlist`

在正确样本下统一返回并发冲突类错误。

根因一：

- 与账号状态流转相同，领域层先 `Version++`，adapter 再把递增值作为 `expectedVersion` 传给 repository，导致更新条件天然不匹配

根因二：

- `GetByID` 查询列顺序为 `id, supply_account_id, user_id`
- 但扫描目标却是 `pkg.ID, pkg.SupplierID, pkg.AccountID`
- 导致 `SupplierID` 和 `AccountID` 映射反了，后续更新 `WHERE user_id = pkg.SupplierID` 进一步放大失败概率

证据位置：

- [package.go](/home/long/project/立交桥/supply-api/internal/domain/package.go#L192)
- [package.go](/home/long/project/立交桥/supply-api/internal/domain/package.go#L194)
- [package.go](/home/long/project/立交桥/supply-api/internal/domain/package.go#L221)
- [package.go](/home/long/project/立交桥/supply-api/internal/domain/package.go#L223)
- [package.go](/home/long/project/立交桥/supply-api/internal/domain/package.go#L246)
- [package.go](/home/long/project/立交桥/supply-api/internal/domain/package.go#L248)
- [adapter.go](/home/long/project/立交桥/supply-api/internal/adapter/adapter.go#L176)
- [package.go](/home/long/project/立交桥/supply-api/internal/repository/package.go#L73)
- [package.go](/home/long/project/立交桥/supply-api/internal/repository/package.go#L88)
- [package.go](/home/long/project/立交桥/supply-api/internal/repository/package.go#L116)
- [package.go](/home/long/project/立交桥/supply-api/internal/repository/package.go#L131)

影响：

- 套餐发布、暂停、下架三条状态流转链路在当前 DB-backed 实现下不可用

### 3.3 P0: supply-api 套餐创建 / 克隆链路被同一 SQL 缺陷阻断

现象：

- `POST /api/v1/supply/packages/draft` 返回 `422`
- `POST /api/v1/supply/packages/{package_id}/clone` 返回 `404` 包装错误，但内层仍是创建失败

根因：

- `supply_packages` 的 `INSERT` 目标列数多于占位符数

证据位置：

- [package.go](/home/long/project/立交桥/supply-api/internal/repository/package.go#L27)
- [package.go](/home/long/project/立交桥/supply-api/internal/repository/package.go#L37)
- [package.go](/home/long/project/立交桥/supply-api/internal/repository/package.go#L60)

### 3.4 P1: 审计事件读路径也被表结构不一致破坏

现象：

- `GET /api/v1/supply/accounts/{account_id}/audit-logs` 返回 `500`
- `GET /api/v1/audit/events/{event_id}` 返回 `500`

根因：

- 审计仓储查询和写入都假定 `audit_events` 表存在 `trace_id`、`span_id`
- 当前分区 schema 实际没有这两个列

证据位置：

- [audit_repository.go](/home/long/project/立交桥/supply-api/internal/audit/repository/audit_repository.go#L333)
- [audit_repository.go](/home/long/project/立交桥/supply-api/internal/audit/repository/audit_repository.go#L339)
- [partition_strategy_v1.sql](/home/long/project/立交桥/supply-api/sql/postgresql/partition_strategy_v1.sql#L7)
- [partition_strategy_v1.sql](/home/long/project/立交桥/supply-api/sql/postgresql/partition_strategy_v1.sql#L15)

说明：

- 这不是只影响“审计写入”的问题，读路径也已经被破坏

### 3.5 P1: IAM 更新接口把空字符串写入 INET 列

现象：

- `PUT /api/v1/iam/roles/{role_code}` 返回 `500`
- PostgreSQL 报错：`invalid input syntax for type inet: ""`

根因：

- 仓储层直接把 `role.UpdatedIP` 写入 `updated_ip`
- 当前模型字段是字符串零值，更新时传入空字符串而非 `NULL`

证据位置：

- [role.go](/home/long/project/立交桥/supply-api/internal/iam/model/role.go#L77)
- [iam_repository.go](/home/long/project/立交桥/supply-api/internal/iam/repository/iam_repository.go#L155)
- [iam_repository.go](/home/long/project/立交桥/supply-api/internal/iam/repository/iam_repository.go#L162)

### 3.6 P1: IAM 分配角色接口未填 `granted_by`，直接触发外键错误

现象：

- `POST /api/v1/iam/users/{user_id}/roles` 返回 `500`
- 数据库错误指向 `iam_user_roles_granted_by_fkey`

根因：

- HTTP handler 构造 `AssignRoleRequest` 时只传 `UserID / RoleCode / TenantID`
- 服务层原样把 `GrantedBy=0` 写入 `iam_user_roles`
- PostgreSQL 外键拒绝该值

证据位置：

- [iam_handler.go](/home/long/project/立交桥/supply-api/internal/iam/handler/iam_handler.go#L330)
- [iam_service_db.go](/home/long/project/立交桥/supply-api/internal/iam/service/iam_service_db.go#L188)
- [iam_repository.go](/home/long/project/立交桥/supply-api/internal/iam/repository/iam_repository.go#L416)

### 3.7 P1: IAM 列表与用户角色查询的空值扫描问题已在真实接口层复现

现象：

- `GET /api/v1/iam/roles` 返回 `500`
- `GET /api/v1/iam/users/{user_id}/roles` 返回 `500`

根因：

- 可空字段被直接扫描到非空基本类型

证据位置：

- [iam_repository.go](/home/long/project/立交桥/supply-api/internal/iam/repository/iam_repository.go#L131)
- [iam_repository.go](/home/long/project/立交桥/supply-api/internal/iam/repository/iam_repository.go#L483)
- [iam_repository.go](/home/long/project/立交桥/supply-api/internal/iam/repository/iam_repository.go#L513)

## 4. 服务级汇总

| 服务 | 总项数 | 通过 | 失败 | 条件关闭 | 备注 |
| --- | ---: | ---: | ---: | ---: | --- |
| `gateway` | 8 | 8 | 0 | 0 | 当前主启动链路正常 |
| `platform-token-runtime` | 6 | 4 | 2 | 0 | 查询链路可用，变更链路失败 |
| `supply-api` | 39 | 22 | 16 | 1 | 包含 1 个 DDL 前置失败项 |

## 5. 接口矩阵

### 5.1 gateway

| 方法 | 路径 | 判定 | HTTP | 说明 |
| --- | --- | --- | --- | --- |
| `GET` | `/health` | 通过 | `200` | 健康检查通过 |
| `GET` | `/healthz` | 通过 | `200` | 健康检查通过 |
| `GET` | `/readyz` | 通过 | `200` | 健康检查通过 |
| `GET` | `/v1/models` | 通过 | `200` | 返回 `5` 个模型 |
| `POST` | `/v1/chat/completions` | 通过 | `200` | 真实上游联调成功 |
| `POST` | `/api/v1/chat/completions` | 通过 | `200` | 别名路由正常 |
| `POST` | `/v1/completions` | 通过 | `200` | 真实上游联调成功 |
| `POST` | `/api/v1/completions` | 通过 | `200` | 别名路由正常 |

### 5.2 platform-token-runtime

| 方法 | 路径 | 判定 | HTTP | 说明 |
| --- | --- | --- | --- | --- |
| `POST` | `/api/v1/platform/tokens/issue` | 通过 | `201` | 成功签发真实 Token |
| `GET` | `/actuator/health` | 通过 | `200` | 健康检查正常 |
| `POST` | `/api/v1/platform/tokens/introspect` | 通过 | `200` | 返回 `active/admin` |
| `GET` | `/api/v1/platform/tokens/audit-events` | 通过 | `200` | 返回审计事件 |
| `POST` | `/api/v1/platform/tokens/{token_id}/refresh` | 失败 | `422` | `BUSINESS_ERROR` |
| `POST` | `/api/v1/platform/tokens/{token_id}/revoke` | 失败 | `422` | `BUSINESS_ERROR` |

### 5.3 supply-api

#### 前置 DDL

| 方法 | 路径 | 判定 | HTTP | 说明 |
| --- | --- | --- | --- | --- |
| `DDL` | `/sql/postgresql/iam_schema_v1.sql` | 失败 | `psql` | `'*'` 违反 `chk_scope_code_format` |

#### 健康与基础查询

| 方法 | 路径 | 判定 | HTTP | 说明 |
| --- | --- | --- | --- | --- |
| `GET` | `/actuator/health` | 通过 | `200` | 数据库与缓存均为 `ok` |
| `GET` | `/actuator/health/ready` | 通过 | `200` | readiness 正常 |
| `GET` | `/actuator/health/live` | 通过 | `200` | liveness 正常 |
| `GET` | `/api/v1/supply/billing` | 通过 | `200` | 账单汇总接口可用 |
| `GET` | `/api/v1/supplier/billing` | 通过 | `200` | 兼容别名可用 |
| `GET` | `/api/v1/supply/earnings/records` | 通过 | `200` | 返回分页总数 `417` |

#### 账号接口

| 方法 | 路径 | 判定 | HTTP | 说明 |
| --- | --- | --- | --- | --- |
| `POST` | `/api/v1/supply/accounts/verify` | 通过 | `200` | 校验接口可用 |
| `POST` | `/api/v1/supply/accounts` | 失败 | `500` | `IDEMPOTENCY_LOCK_FAILED` |
| `POST` | `/api/v1/supply/accounts/{account_id}/activate` | 失败 | `404` | 外部返回 404，内层为并发冲突 |
| `POST` | `/api/v1/supply/accounts/{account_id}/suspend` | 失败 | `404` | 外部返回 404，内层为并发冲突 |
| `DELETE` | `/api/v1/supply/accounts/{account_id}/delete` | 通过 | `204` | 删除链路可用，但审计写入报错 |
| `GET` | `/api/v1/supply/accounts/{account_id}/audit-logs` | 失败 | `500` | `trace_id` 列不存在 |

#### 套餐接口

| 方法 | 路径 | 判定 | HTTP | 说明 |
| --- | --- | --- | --- | --- |
| `POST` | `/api/v1/supply/packages/draft` | 失败 | `422` | `SUP_HTTP_5002` |
| `POST` | `/api/v1/supply/packages/batch-price` | 通过 | `200` | 批量调价可用 |
| `POST` | `/api/v1/supply/packages/{package_id}/publish` | 失败 | `404` | 外部返回 404，内层为并发冲突 |
| `POST` | `/api/v1/supply/packages/{package_id}/pause` | 失败 | `404` | 外部返回 404，内层为并发冲突 |
| `POST` | `/api/v1/supply/packages/{package_id}/unlist` | 失败 | `404` | 外部返回 404，内层为并发冲突 |
| `POST` | `/api/v1/supply/packages/{package_id}/clone` | 失败 | `404` | 内层实际是创建 SQL 失败 |

#### 结算与审计接口

| 方法 | 路径 | 判定 | HTTP | 说明 |
| --- | --- | --- | --- | --- |
| `POST` | `/api/v1/supply/settlements/withdraw` | 条件关闭 | `503` | SMS 未就绪，属于设计内 fail-closed |
| `POST` | `/api/v1/supply/settlements/{settlement_id}/cancel` | 通过 | `200` | 业务动作成功，但审计写入报错 |
| `GET` | `/api/v1/supply/settlements/{settlement_id}/statement` | 通过 | `200` | 结算单下载地址正常返回 |
| `GET` | `/api/v1/audit/events/{event_id}` | 失败 | `500` | 审计仓储查询字段与表结构不一致 |

#### 告警接口

| 方法 | 路径 | 判定 | HTTP | 说明 |
| --- | --- | --- | --- | --- |
| `POST` | `/api/v1/audit/alerts` | 通过 | `201` | 成功创建 `ALT-e70a7615` |
| `GET` | `/api/v1/audit/alerts` | 通过 | `200` | 列表查询正常 |
| `GET` | `/api/v1/audit/alerts/{alert_id}` | 通过 | `200` | 详情查询正常 |
| `PUT` | `/api/v1/audit/alerts/{alert_id}` | 通过 | `200` | 状态更新为 `acknowledged` |
| `POST` | `/api/v1/audit/alerts/{alert_id}/resolve` | 通过 | `200` | 状态更新为 `resolved` |
| `DELETE` | `/api/v1/audit/alerts/{alert_id}` | 通过 | `204` | 删除正常 |

#### IAM 接口

| 方法 | 路径 | 判定 | HTTP | 说明 |
| --- | --- | --- | --- | --- |
| `GET` | `/api/v1/iam/roles` | 失败 | `500` | 空值扫描失败 |
| `POST` | `/api/v1/iam/roles` | 通过 | `201` | `matrix_role` 创建成功 |
| `GET` | `/api/v1/iam/roles/{role_code}` | 通过 | `200` | 单角色查询正常 |
| `PUT` | `/api/v1/iam/roles/{role_code}` | 失败 | `500` | 空字符串写入 `INET` 列 |
| `GET` | `/api/v1/iam/scopes` | 通过 | `200` | 返回 `29` 个 Scope |
| `GET` | `/api/v1/iam/users/{user_id}/roles` | 失败 | `500` | 空值扫描失败 |
| `POST` | `/api/v1/iam/users/{user_id}/roles` | 失败 | `500` | `granted_by` 外键失败 |
| `DELETE` | `/api/v1/iam/users/{user_id}/roles/{role_code}` | 失败 | `404` | 上一步分配失败后的级联结果 |
| `DELETE` | `/api/v1/iam/roles/{role_code}` | 通过 | `200` | 删除角色正常 |
| `GET` | `/api/v1/iam/check-scope` | 通过 | `200` | 校验接口可用 |

## 6. 最终判断

这轮矩阵验证后的最终判断是：

- `gateway` 可以视为当前最稳定的已交付模块。
- `platform-token-runtime` 仍然不能宣称“生命周期完整可用”，因为刷新和撤销链路在 PostgreSQL-backed 模式下失败。
- `supply-api` 不能宣称“功能完整正常”，因为它的状态流转、创建链路、审计读写一致性、IAM 部分 DB-backed 能力都存在确定性缺陷。

如果按交付成熟度排序：

- 第一梯队：`gateway`
- 第二梯队：`platform-token-runtime` 的查询能力、`supply-api` 的读路径与告警模块
- 需要优先整改：`platform-token-runtime` 变更链路、`supply-api` 写路径、审计仓储、IAM DB-backed 路径
