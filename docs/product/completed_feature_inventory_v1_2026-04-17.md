# 已完成功能清单（前后端 / 原子动作级）

- 版本：v1.0
- 日期：2026-04-17
- 盘点口径：只统计当前仓库中“已挂载、已接入主链路、已有真实代码入口”的功能；未接入主链路、默认关闭或需额外外部条件的能力，不混入“已完成清单”。
- 原子动作说明：当前仓库没有一方前端控制台，所以本文中的“按钮级动作”是按最小可交互粒度抽象的动作名，用于后续页面、流程图、原型图和测试用例统一映射。

---

## 1. 盘点规则

1. 只把当前运行时会注册的 HTTP 路由或一方前端实际存在的页面/组件记入“已完成”。
2. 已实现但默认关闭、或依赖额外条件才能启用的能力，单独记入“条件性能力”。
3. 设计稿、PRD、竞品目录、实验模块、未接入主启动链路的代码，不算“已完成”。
4. 前端是否存在，以仓库内一方源码为准；不把 `llm-gateway-competitors/` 下的第三方前端算入本项目交付物。

---

## 2. 前端交付现状

### 2.1 当前结论

当前仓库 **没有一方前端应用交付物**。`gateway`、`platform-token-runtime`、`supply-api`、`docs` 目录下未检出 `.tsx` / `.jsx` / `.vue` / `.html` 的一方页面源码；仓库中出现的前端代码全部位于 `llm-gateway-competitors/` 竞品归档目录，不属于本项目交付范围。

### 2.2 页面与按钮盘点结果

| 盘点项 | 当前状态 | 证据 |
|---|---|---|
| 一方前端页面 | 无 | 仓库搜索 `gateway` / `platform-token-runtime` / `supply-api` / `docs` 未发现前端源码 |
| 一方前端按钮 | 无 | 当前仅有后端服务、测试和文档，没有可运行控制台 |
| 可直接用于 UI 设计的动作基线 | 有 | 见本文第 3 节到第 5 节的“原子动作清单” |

### 2.3 对流程梳理的直接影响

1. 当前只能先按后端原子动作梳理流程，不能按现成页面回推流程。
2. 后续如果要补控制台，建议直接以本文中的原子动作作为按钮级最小单元。
3. 供应侧旧 PRD 中的按钮定义只能作为参考版式，不能视为当前已交付 UI。

---

## 3. 后端已完成清单

## 3.1 Gateway 已完成清单

> 服务定位：OpenAI 兼容入口网关，负责入口鉴权、限流、模型聚合、上游路由和健康检查。

| 原子动作ID | 一级域 | 二级流程 | 按钮级动作 | 接口 / 入口 | 当前状态 | 证据 |
|---|---|---|---|---|---|---|
| GWT-001 | 网关入口 | 对话请求 | 发送聊天请求 | `/v1/chat/completions`、`/api/v1/chat/completions` | 已完成 | `gateway/internal/app/bootstrap.go`、`gateway/internal/handler/handler.go`、`gateway/internal/handler/handler_test.go` |
| GWT-002 | 网关入口 | 文本补全 | 发送补全文本请求 | `/v1/completions`、`/api/v1/completions` | 已完成 | `gateway/internal/app/bootstrap.go`、`gateway/internal/handler/handler.go`、`gateway/internal/handler/handler_test.go` |
| GWT-003 | 网关入口 | 模型目录 | 查看当前可用模型列表 | `/v1/models` | 已完成 | `gateway/internal/app/bootstrap.go`、`gateway/internal/handler/handler.go`、`gateway/internal/handler/handler_test.go` |
| GWT-004 | 网关运维 | 存活检查 | 查看网关健康状态 | `/health`、`/healthz`、`/readyz` | 已完成 | `gateway/internal/app/bootstrap.go`、`gateway/internal/handler/handler.go` |

### 3.1.1 Gateway 已完成流程说明

1. 请求可走 OpenAI 兼容路径和 `/api/v1/*` 兼容路径。
2. `/v1/models` 已改为返回当前注册 provider 的模型并集，不再是硬编码静态列表。
3. 主启动链路当前只接入 `latency`、`round_robin`、`weighted`、`availability` 四种策略。

## 3.2 Platform Token Runtime 已完成清单

> 服务定位：平台 token 生命周期、introspection 与审计查询服务。

| 原子动作ID | 一级域 | 二级流程 | 按钮级动作 | 接口 / 入口 | 当前状态 | 证据 |
|---|---|---|---|---|---|---|
| TOK-001 | 平台鉴权 | Token 生命周期 | 签发平台 token | `POST /api/v1/platform/tokens/issue` | 已完成 | `platform-token-runtime/internal/httpapi/token_api.go`、`platform-token-runtime/internal/token/lifecycle_executable_test.go` |
| TOK-002 | 平台鉴权 | Token 生命周期 | 刷新 token TTL | `POST /api/v1/platform/tokens/{token_id}/refresh` | 已完成 | `platform-token-runtime/internal/httpapi/token_api.go`、`platform-token-runtime/internal/token/lifecycle_executable_test.go` |
| TOK-003 | 平台鉴权 | Token 生命周期 | 撤销 token | `POST /api/v1/platform/tokens/{token_id}/revoke` | 已完成 | `platform-token-runtime/internal/httpapi/token_api.go`、`platform-token-runtime/internal/token/lifecycle_executable_test.go` |
| TOK-004 | 平台鉴权 | Token 生命周期 | 校验 token 有效性 | `POST /api/v1/platform/tokens/introspect` | 已完成 | `platform-token-runtime/internal/httpapi/token_api.go`、`platform-token-runtime/internal/httpapi/token_api_test.go` |
| TOK-005 | 平台鉴权 | 审计查询 | 查询 token 审计事件 | `GET /api/v1/platform/tokens/audit-events` | 已完成 | `platform-token-runtime/internal/httpapi/token_api.go`、`platform-token-runtime/internal/httpapi/token_api_test.go`、`platform-token-runtime/internal/token/audit_executable_test.go` |
| TOK-006 | 平台运维 | 存活检查 | 查看 token runtime 健康状态 | `/actuator/health` | 已完成 | `platform-token-runtime/internal/app/bootstrap.go` |

### 3.2.1 Token Runtime 已完成流程说明

1. 已覆盖最小生命周期闭环：签发、刷新、撤销、校验、审计查询。
2. `audit-events` 当前必须带 Bearer Token 才能访问。
3. `dev` 环境默认可用内存 store；`staging/prod` 已支持 PostgreSQL store 装配。

## 3.3 Supply API 已完成清单

> 服务定位：供应侧账号挂载、套餐发布、账单收益、审计与告警能力。

### 3.3.1 供应账号

| 原子动作ID | 一级域 | 二级流程 | 按钮级动作 | 接口 / 入口 | 当前状态 | 证据 |
|---|---|---|---|---|---|---|
| SUP-ACC-001 | 供应侧 | 账号挂载 | 验证上游账号凭证 | `POST /api/v1/supply/accounts/verify` | 已完成 | `supply-api/internal/httpapi/supply_api.go`、`supply-api/e2e/production_flow_test.go` |
| SUP-ACC-002 | 供应侧 | 账号挂载 | 创建供应账号 | `POST /api/v1/supply/accounts` | 已完成 | `supply-api/internal/httpapi/supply_api.go`、`supply-api/e2e/production_flow_test.go` |
| SUP-ACC-003 | 供应侧 | 账号管理 | 激活账号 | `POST /api/v1/supply/accounts/{account_id}/activate` | 已完成 | `supply-api/internal/httpapi/supply_api.go` |
| SUP-ACC-004 | 供应侧 | 账号管理 | 暂停账号 | `POST /api/v1/supply/accounts/{account_id}/suspend` | 已完成 | `supply-api/internal/httpapi/supply_api.go` |
| SUP-ACC-005 | 供应侧 | 账号管理 | 删除账号 | `DELETE /api/v1/supply/accounts/{account_id}/delete` | 已完成 | `supply-api/internal/httpapi/supply_api.go` |
| SUP-ACC-006 | 供应侧 | 审计追溯 | 查看账号审计日志 | `GET /api/v1/supply/accounts/{account_id}/audit-logs` | 已完成 | `supply-api/internal/httpapi/supply_api.go` |

### 3.3.2 套餐发布

| 原子动作ID | 一级域 | 二级流程 | 按钮级动作 | 接口 / 入口 | 当前状态 | 证据 |
|---|---|---|---|---|---|---|
| SUP-PKG-001 | 供应侧 | 套餐编辑 | 创建套餐草稿 | `POST /api/v1/supply/packages/draft` | 已完成 | `supply-api/internal/httpapi/supply_api.go`、`supply-api/e2e/production_flow_test.go` |
| SUP-PKG-002 | 供应侧 | 套餐管理 | 发布套餐 | `POST /api/v1/supply/packages/{package_id}/publish` | 已完成 | `supply-api/internal/httpapi/supply_api.go`、`supply-api/e2e/production_flow_test.go` |
| SUP-PKG-003 | 供应侧 | 套餐管理 | 暂停套餐 | `POST /api/v1/supply/packages/{package_id}/pause` | 已完成 | `supply-api/internal/httpapi/supply_api.go` |
| SUP-PKG-004 | 供应侧 | 套餐管理 | 下架套餐 | `POST /api/v1/supply/packages/{package_id}/unlist` | 已完成 | `supply-api/internal/httpapi/supply_api.go` |
| SUP-PKG-005 | 供应侧 | 套餐管理 | 复制套餐 | `POST /api/v1/supply/packages/{package_id}/clone` | 已完成 | `supply-api/internal/httpapi/supply_api.go` |
| SUP-PKG-006 | 供应侧 | 套餐管理 | 批量调价 | `POST /api/v1/supply/packages/batch-price` | 已完成 | `supply-api/internal/httpapi/supply_api.go` |

### 3.3.3 账单、收益、结算查询

| 原子动作ID | 一级域 | 二级流程 | 按钮级动作 | 接口 / 入口 | 当前状态 | 证据 |
|---|---|---|---|---|---|---|
| SUP-BIL-001 | 供应侧 | 账单汇总 | 查看账单汇总 | `GET /api/v1/supply/billing` | 已完成 | `supply-api/internal/httpapi/supply_api.go`、`supply-api/e2e/production_flow_test.go` |
| SUP-BIL-002 | 供应侧 | 账单汇总 | 查看账单汇总（兼容别名） | `GET /api/v1/supplier/billing` | 已完成 | `supply-api/internal/httpapi/supply_api.go` |
| SUP-EAR-001 | 供应侧 | 收益流水 | 查看收益记录 | `GET /api/v1/supply/earnings/records` | 已完成 | `supply-api/internal/httpapi/supply_api.go`、`supply-api/e2e/production_flow_test.go` |
| SUP-SET-001 | 供应侧 | 结算单管理 | 取消结算单 | `POST /api/v1/supply/settlements/{settlement_id}/cancel` | 已完成 | `supply-api/internal/httpapi/supply_api.go` |
| SUP-SET-002 | 供应侧 | 结算单管理 | 下载结算单 | `GET /api/v1/supply/settlements/{settlement_id}/statement` | 已完成 | `supply-api/internal/httpapi/supply_api.go` |

### 3.3.4 审计与告警

| 原子动作ID | 一级域 | 二级流程 | 按钮级动作 | 接口 / 入口 | 当前状态 | 证据 |
|---|---|---|---|---|---|---|
| SUP-AUD-001 | 审计 | 事件追溯 | 查看单个审计事件 | `GET /api/v1/audit/events/{event_id}` | 已完成 | `supply-api/internal/httpapi/supply_api.go` |
| SUP-ALT-001 | 告警 | 告警管理 | 创建告警 | `POST /api/v1/audit/alerts` | 已完成 | `supply-api/internal/httpapi/alert_api.go`、`supply-api/internal/audit/handler/alert_handler.go` |
| SUP-ALT-002 | 告警 | 告警管理 | 查看告警列表 | `GET /api/v1/audit/alerts` | 已完成 | `supply-api/internal/httpapi/alert_api.go`、`supply-api/internal/audit/handler/alert_handler.go` |
| SUP-ALT-003 | 告警 | 告警管理 | 查看告警详情 | `GET /api/v1/audit/alerts/{alert_id}` | 已完成 | `supply-api/internal/httpapi/alert_api.go`、`supply-api/internal/audit/handler/alert_handler.go` |
| SUP-ALT-004 | 告警 | 告警管理 | 更新告警 | `PUT /api/v1/audit/alerts/{alert_id}` | 已完成 | `supply-api/internal/httpapi/alert_api.go`、`supply-api/internal/audit/handler/alert_handler.go` |
| SUP-ALT-005 | 告警 | 告警管理 | 删除告警 | `DELETE /api/v1/audit/alerts/{alert_id}` | 已完成 | `supply-api/internal/httpapi/alert_api.go`、`supply-api/internal/audit/handler/alert_handler.go` |
| SUP-ALT-006 | 告警 | 告警处置 | 标记告警已解决 | `POST /api/v1/audit/alerts/{alert_id}/resolve` | 已完成 | `supply-api/internal/httpapi/alert_api.go`、`supply-api/internal/audit/handler/alert_handler.go` |

### 3.3.5 健康检查

| 原子动作ID | 一级域 | 二级流程 | 按钮级动作 | 接口 / 入口 | 当前状态 | 证据 |
|---|---|---|---|---|---|---|
| SUP-OPS-001 | 运维 | 健康检查 | 查看总健康状态 | `GET /actuator/health` | 已完成 | `supply-api/internal/httpapi/healthcheck.go`、`supply-api/internal/app/bootstrap.go` |
| SUP-OPS-002 | 运维 | 健康检查 | 查看 readiness 状态 | `GET /actuator/health/ready` | 已完成 | `supply-api/internal/httpapi/healthcheck.go`、`supply-api/internal/app/bootstrap.go` |
| SUP-OPS-003 | 运维 | 健康检查 | 查看 liveness 状态 | `GET /actuator/health/live` | 已完成 | `supply-api/internal/httpapi/healthcheck.go`、`supply-api/internal/app/bootstrap.go` |

---

## 4. 条件性能力（已实现，但不计入默认完成清单）

> 这部分不是“未实现”，而是“代码已实现，但默认关闭或依赖外部条件才能启用”。为避免误判，不混入第 3 节的完成清单。

### 4.1 Supply API 条件性能力

| 原子动作ID | 一级域 | 二级流程 | 按钮级动作 | 接口 / 入口 | 当前状态 | 启用条件 | 证据 |
|---|---|---|---|---|---|---|---|
| SUP-SET-C01 | 供应侧 | 提现 | 发起提现申请 | `POST /api/v1/supply/settlements/withdraw` | 条件启用 | `settlement.withdraw_enabled=true` 且 SMS 配置 ready 且 SMSVerifier 已注入 | `supply-api/internal/httpapi/supply_api.go`、`supply-api/internal/app/runtime.go`、`supply-api/internal/config/config.go` |
| SUP-IAM-C01 | IAM | 角色管理 | 查看角色列表 | `GET /api/v1/iam/roles` | 条件启用 | `server.iam_enabled=true` 且 runtime 为 DB-backed | `supply-api/internal/iam/handler/iam_handler.go`、`supply-api/internal/app/runtime.go` |
| SUP-IAM-C02 | IAM | 角色管理 | 创建角色 | `POST /api/v1/iam/roles` | 条件启用 | 同上 | `supply-api/internal/iam/handler/iam_handler.go`、`supply-api/internal/app/runtime.go` |
| SUP-IAM-C03 | IAM | 角色管理 | 查看单个角色 | `GET /api/v1/iam/roles/{role_code}` | 条件启用 | 同上 | `supply-api/internal/iam/handler/iam_handler.go`、`supply-api/internal/app/runtime.go` |
| SUP-IAM-C04 | IAM | 角色管理 | 更新角色 | `PUT /api/v1/iam/roles/{role_code}` | 条件启用 | 同上 | `supply-api/internal/iam/handler/iam_handler.go`、`supply-api/internal/app/runtime.go` |
| SUP-IAM-C05 | IAM | 角色管理 | 删除角色 | `DELETE /api/v1/iam/roles/{role_code}` | 条件启用 | 同上 | `supply-api/internal/iam/handler/iam_handler.go`、`supply-api/internal/app/runtime.go` |
| SUP-IAM-C06 | IAM | Scope 管理 | 查看 scope 列表 | `GET /api/v1/iam/scopes` | 条件启用 | 同上 | `supply-api/internal/iam/handler/iam_handler.go`、`supply-api/internal/app/runtime.go` |
| SUP-IAM-C07 | IAM | 用户角色 | 查看用户角色 | `GET /api/v1/iam/users/{user_id}/roles` | 条件启用 | 同上 | `supply-api/internal/iam/handler/iam_handler.go`、`supply-api/internal/app/runtime.go`、`supply-api/internal/iam/handler/iam_handler_real_test.go` |
| SUP-IAM-C08 | IAM | 用户角色 | 分配角色 | `POST /api/v1/iam/users/{user_id}/roles` | 条件启用 | 同上 | `supply-api/internal/iam/handler/iam_handler.go`、`supply-api/internal/app/runtime.go`、`supply-api/internal/iam/handler/iam_handler_real_test.go` |
| SUP-IAM-C09 | IAM | 用户角色 | 撤销角色 | `DELETE /api/v1/iam/users/{user_id}/roles/{role_code}` | 条件启用 | 同上 | `supply-api/internal/iam/handler/iam_handler.go`、`supply-api/internal/app/runtime.go`、`supply-api/internal/iam/handler/iam_handler_real_test.go` |
| SUP-IAM-C10 | IAM | Scope 校验 | 检查用户是否拥有 scope | `GET /api/v1/iam/check-scope` | 条件启用 | 同上 | `supply-api/internal/iam/handler/iam_handler.go`、`supply-api/internal/app/runtime.go` |

### 4.2 条件性能力的决策提示

1. 提现能力当前不能算“默认可用功能”，因为生产态必须满足 SMS readiness。
2. IAM 不能算“默认开放能力”，因为只有显式开关打开且依赖是数据库实现时才挂载路由。
3. 条件性能力适合纳入“二期启用决策”或“环境就绪清单”，不适合写进默认上线范围。

---

## 5. 未纳入本次完成清单的项

| 项目 | 归类 | 不纳入原因 | 证据 |
|---|---|---|---|
| 一方前端页面 / 按钮 | 未交付 | 仓库内没有一方控制台源码 | 仓库目录盘点结果 |
| `gateway` 的 `cost_based` / `cost_aware` / `fallback` 实验策略 | 未接入主链路 | 代码存在，但 `BuildServer` 未接入运行时装配 | `gateway/internal/app/bootstrap.go`、`gateway/README.md` |
| 竞品目录里的 Vue 前端 | 非本项目交付物 | 位于 `llm-gateway-competitors/`，仅用于参考/归档 | 仓库目录盘点结果 |

---

## 6. 已打通的组合流程

> 这一节只列当前有可执行测试或明确主链路证据的组合流程，方便做流程图梳理与优先级决策。

| 流程ID | 流程名称 | 当前已打通步骤 | 当前边界 | 证据 |
|---|---|---|---|---|
| FLOW-SUP-001 | 供应商入驻 | 验证账号 -> 创建账号 -> 创建套餐草稿 -> 发布套餐 | 只覆盖后端流程，不含一方前端页面 | `supply-api/e2e/production_flow_test.go` |
| FLOW-SUP-002 | 结算查询 | 查看账单汇总 -> 查看收益记录 | 提现默认关闭，单独归为条件性能力 | `supply-api/e2e/production_flow_test.go` |
| FLOW-SUP-003 | 审计追溯 | 执行业务动作 -> 写入审计事件 -> 审计存储查询 | 以 API / Store 视角验证，无前端审计页 | `supply-api/e2e/production_flow_test.go` |
| FLOW-TOK-001 | Token 生命周期 | 签发 -> 刷新 -> 撤销 -> introspect | 当前是一套独立平台服务，不含管理 UI | `platform-token-runtime/internal/token/lifecycle_executable_test.go` |
| FLOW-TOK-002 | Token 审计 | 业务动作 -> 记录审计事件 -> 查询审计事件 | `audit-events` 需要 Bearer Token | `platform-token-runtime/internal/token/audit_executable_test.go` |
| FLOW-GWT-001 | Gateway OpenAI 兼容入口 | 聊天请求 -> 上游 provider 调用 -> 返回 OpenAI 兼容响应 | 当前不含控制台，只含 API 入口 | `gateway/internal/handler/handler_test.go` |

---

## 7. 给流程梳理和产品决策的直接建议

1. 如果下一步要画“页面流程图”，请先把第 3 节与第 4 节的原子动作转成页面按钮，不要反过来从旧 PRD 倒推实现状态。
2. 如果下一步要确定“首批控制台范围”，建议优先覆盖 `SUP-ACC-*`、`SUP-PKG-*`、`SUP-BIL-*`、`SUP-EAR-*`、`SUP-ALT-*`，因为它们已经是默认挂载能力。
3. `提现` 和 `IAM` 不建议放进“默认已交付页面范围”，应作为“条件启用页”或“环境就绪后开放”的模块。
4. `gateway` 和 `platform-token-runtime` 当前更适合作为 API 产品和运维能力来梳理，而不是控制台页面来梳理。
