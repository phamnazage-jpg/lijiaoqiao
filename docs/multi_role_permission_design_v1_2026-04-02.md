# 多角色权限设计方案（P1）

- 版本：v1.0
- 日期：2026-04-02
- 状态：设计稿（已修复）
- 依赖：
  - `docs/token_runtime_minimal_spec_v1.md`（TOK-001）
  - `docs/token_auth_middleware_design_v1_2026-03-29.md`（TOK-002）
  - `docs/llm_gateway_prd_v1_2026-03-25.md`
- 目标：实现 PRD P1 "多角色权限"需求

---

## 1. 背景与目标

### 1.1 业务背景

LLM Gateway 平台需要支持多类用户角色，满足不同的使用场景：

1. **平台管理员** - 负责组织级策略、预算、权限管理
2. **AI 应用开发者** - 负责接入模型与业务落地
3. **财务/运营负责人** - 负责成本追踪、对账与预算控制
4. **供应方** - 拥有多余LLM配额的个人或企业（平台用户）
5. **需求方** - 需要LLM调用能力的企业/开发者

### 1.2 设计目标

1. **角色扩展**：在现有 `owner/viewer/admin` 三角色基础上扩展，支持更多业务场景
2. **权限细分**：支持细粒度的 scope 权限控制
3. **层级清晰**：建立的角色继承/层级关系
4. **API兼容**：保持与现有 SUP-004~SUP-008 链路一致
5. **可扩展**：支持未来新增角色和权限

---

## 2. 现有权限模型分析

### 2.1 现有角色体系（TOK-001）

| 角色 | 等级 | 能力 | 约束 |
|------|------|------|------|
| admin | 3 | 风控与审计管理 | 仅平台内部可用 |
| owner | 2 | 管理供应侧账号、套餐、结算 | 不可读取上游凭证明文 |
| viewer | 1 | 只读查询 | 不可执行写操作 |

### 2.2 现有 JWT Token Claims 结构

```go
type TokenClaims struct {
    jwt.RegisteredClaims
    SubjectID string   `json:"subject_id"`  // 用户主体ID
    Role      string   `json:"role"`        // 角色: admin/owner/viewer
    Scope     []string `json:"scope"`        // 授权范围列表
    TenantID  int64    `json:"tenant_id"`   // 租户ID
}
```

### 2.3 现有中间件链路（TOK-002）

```
RequestIdMiddleware
    ↓
QueryKeyRejectMiddleware
    ↓
BearerExtractMiddleware
    ↓
TokenVerifyMiddleware
    ↓
TokenStatusCheckMiddleware
    ↓
ScopeRoleAuthzMiddleware     ← 权限校验
    ↓
AuditEmitMiddleware
```

---

## 3. 多角色权限设计方案

### 3.1 角色定义

#### 3.1.1 平台侧角色（Platform Roles）

| 角色 | 代码 | 层级 | 说明 | 继承关系 |
|------|------|------|------|----------|
| 超级管理员 | `super_admin` | 100 | 平台最高权限，仅平台运营方可用 | - |
| 组织管理员 | `org_admin` | 50 | 组织级管理，管理本组织所有资源 | 显式配置（拥有operator+finops+developer+viewer所有scope） |
| 运维人员 | `operator` | 30 | 系统运维与配置 | 显式配置（拥有viewer所有scope + platform:write等） |
| 开发者 | `developer` | 20 | AI应用开发者，接入模型与业务落地 | 继承 viewer |
| 财务人员 | `finops` | 20 | 成本追踪、对账与预算控制 | 继承 viewer |
| 查看者 | `viewer` | 10 | 只读查询 | - |

**说明**：
1. 继承关系仅用于权限聚合，代表"子角色拥有父角色所有scope + 自身额外scope"
2. `org_admin` 显式配置拥有 `operator` + `finops` + `developer` + `viewer` 的所有scope
3. `operator` 显式配置拥有 `viewer` 所有scope + `platform:write` 等权限
4. 层级数值仅用于权限优先级判断，不影响继承关系

#### 3.1.2 供应侧角色（Supply Roles）

| 角色 | 代码 | 层级 | 说明 | 继承关系 |
|------|------|------|------|----------|
| 供应方管理员 | `supply_admin` | 40 | 供应侧全面管理 | 显式配置（拥有supply_operator+supply_finops所有scope） |
| 供应方运维 | `supply_operator` | 30 | 套餐管理、额度配置 | 显式配置（拥有supply_viewer所有scope + supply:package:write等） |
| 供应方财务 | `supply_finops` | 20 | 收益结算、对账 | 继承 supply_viewer |
| 供应方查看者 | `supply_viewer` | 10 | 只读查询 | - |

#### 3.1.3 需求侧角色（Consumer Roles）

| 角色 | 代码 | 层级 | 说明 | 继承关系 |
|------|------|------|------|----------|
| 需求方管理员 | `consumer_admin` | 40 | 需求侧全面管理 | 显式配置（拥有consumer_operator所有scope） |
| 需求方运维 | `consumer_operator` | 30 | API Key管理、调用配置 | 显式配置（拥有consumer_viewer所有scope + consumer:apikey:*等） |
| 需求方查看者 | `consumer_viewer` | 10 | 只读查询 | - |

### 3.2 角色层级关系图

```
                    ┌─────────────┐
                    │ super_admin │  (层级100)
                    └──────┬──────┘
                           │ 权限聚合
                           ▼
                    ┌─────────────┐
                    │  org_admin  │  (层级50)
                    └──────┬──────┘
                           │ 显式配置（聚合operator+developer+finops+viewer scope）
              ┌────────────┼────────────┐
              ▼            ▼            ▼
       ┌──────────┐  ┌──────────┐  ┌──────────┐
       │ operator │  │developer │  │  finops  │  (层级20-30)
       └────┬─────┘  └────┬─────┘  └────┬─────┘
            │             │             │
            │ 显式配置     │ 继承        │ 继承
            │ (+viewer)   │ (+viewer)   │ (+viewer)
            ▼             ▼             ▼
       ┌──────────┐  ┌──────────┐  ┌──────────┐
       │  viewer  │  │  viewer  │  │  viewer  │  (层级10)
       └──────────┘  └──────────┘  └──────────┘

    ─────────────────────────────────────────

       ┌──────────┐     ┌──────────────┐
       │supply_ad │────│consumer_adm │
       │   min    │    │   in        │  (层级40)
       └────┬─────┘    └──────┬───────┘
            │ 显式配置         │ 显式配置
            │ (+operator      │ (+operator
            │  +finops)       │  +viewer)
            ▼                  ▼
       ┌──────────┐     ┌──────────────┐
       │supply_op │     │consumer_op  │
       │  erator  │     │  erator     │  (层级30)
       └────┬─────┘     └──────┬───────┘
            │ 显式配置          │ 显式配置
            │ (+viewer)        │ (+viewer)
            ▼                   ▼
       ┌──────────┐     ┌──────────────┐
       │supply_vi │     │consumer_vi  │
       │  ewer    │     │  ewer       │  (层级10)
       └──────────┘     └──────────────┘
```

**继承关系说明**：
- 继承 = 子角色拥有父角色所有 scope + 自身额外 scope
- 显式配置 = 直接授予特定 scope 列表（等效于显式继承但更清晰）
- supply_admin/consumer_admin = 拥有该类别下所有子角色 scope
- operator/developer/finops = 拥有 viewer 所有 scope + 各自额外 scope

### 3.3 Scope 权限定义

#### 3.3.1 Platform Scope

| Scope | 说明 | 授予角色 |
|-------|------|----------|
| `platform:read` | 读取平台配置 | viewer+ |
| `platform:write` | 修改平台配置 | operator+ |
| `platform:admin` | 平台级管理 | org_admin+ |
| `platform:audit:read` | 读取审计日志 | operator+ |
| `platform:audit:export` | 导出审计日志 | org_admin+ |

#### 3.3.2 Tenant Scope

| Scope | 说明 | 授予角色 | 备注 |
|-------|------|----------|------|
| `tenant:read` | 读取租户信息 | viewer+ | |
| `tenant:write` | 修改租户配置 | operator+ | |
| `tenant:member:manage` | 管理租户成员 | org_admin | |
| `tenant:billing:write` | 修改账单设置 | org_admin | |

#### 3.3.3 Supply Scope

| Scope | 说明 | 授予角色 | 备注 |
|-------|------|----------|------|
| `supply:account:read` | 读取供应账号 | supply_viewer+ | |
| `supply:account:write` | 管理供应账号 | supply_operator+ | |
| `supply:package:read` | 读取套餐信息 | supply_viewer+ | |
| `supply:package:write` | 管理套餐 | supply_operator+ | |
| `supply:package:publish` | 发布套餐 | supply_operator+ | |
| `supply:package:offline` | 下架套餐 | supply_operator+ | |
| `supply:settlement:withdraw` | 提现 | supply_admin | |
| `supply:credential:manage` | 管理凭证 | supply_admin | |

#### 3.3.4 Consumer Scope

| Scope | 说明 | 授予角色 | 备注 |
|-------|------|----------|------|
| `consumer:account:read` | 读取账户信息 | consumer_viewer+ | |
| `consumer:account:write` | 管理账户 | consumer_operator+ | |
| `consumer:apikey:create` | 创建API Key | consumer_operator+ | |
| `consumer:apikey:read` | 读取API Key | consumer_viewer+ | |
| `consumer:apikey:revoke` | 吊销API Key | consumer_operator+ | |
| `consumer:usage:read` | 读取使用量 | consumer_viewer+ | |

#### 3.3.5 Billing Scope（统一）

| Scope | 说明 | 授予角色 | user_type限定 |
|-------|------|----------|---------------|
| `billing:read` | 读取账单 | finops+, supply_finops+, consumer_viewer+ | 通过user_type区分数据范围 |
| `billing:write` | 修改账单设置 | org_admin | |

**说明**：
- 原有 `tenant:billing:read`、`supply:settlement:read`、`consumer:billing:read` 统一为 `billing:read`
- 通过 TokenClaims.user_type 字段限定数据范围：platform用户看租户账单，supply用户看供应结算，consumer用户看需求账单
- 原 scope 名称保留作为 deprecated alias

#### 3.3.6 Router Scope（网关转发）

| Scope | 说明 | 授予角色 |
|-------|------|----------|
| `router:invoke` | 调用模型 | 所有认证用户 |
| `router:model:list` | 列出可用模型 | viewer+ |
| `router:model:config` | 配置路由策略 | operator+ |

---

## 4. API 路由权限映射

### 4.1 Platform API

| API路径 | 方法 | 所需Scope | 所需角色 |
|---------|------|-----------|----------|
| `/api/v1/platform/info` | GET | `platform:read` | viewer+ |
| `/api/v1/platform/config` | GET | `platform:read` | viewer+ |
| `/api/v1/platform/config` | PUT | `platform:write` | operator+ |
| `/api/v1/platform/tenants` | GET | `tenant:read` | viewer+ |
| `/api/v1/platform/tenants` | POST | `tenant:write` | operator+ |
| `/api/v1/platform/audit/events` | GET | `platform:audit:read` | operator+ |
| `/api/v1/platform/audit/events/export` | POST | `platform:audit:export` | org_admin+ |

### 4.2 Supply API（与 SUP-004~SUP-008 保持一致）

| API路径 | 方法 | 所需Scope | 所需角色 |
|---------|------|-----------|----------|
| `/api/v1/supply/accounts` | GET | `supply:account:read` | supply_viewer+ |
| `/api/v1/supply/accounts` | POST | `supply:account:write` | supply_operator+ |
| `/api/v1/supply/accounts/:id` | PUT | `supply:account:write` | supply_operator+ |
| `/api/v1/supply/accounts/:id/verify` | POST | `supply:account:write` | supply_operator+ |
| `/api/v1/supply/packages` | GET | `supply:package:read` | supply_viewer+ |
| `/api/v1/supply/packages` | POST | `supply:package:write` | supply_operator+ |
| `/api/v1/supply/packages/:id/publish` | POST | `supply:package:publish` | supply_operator+ |
| `/api/v1/supply/packages/:id/offline` | POST | `supply:package:offline` | supply_operator+ |
| `/api/v1/supply/settlements` | GET | `billing:read` | supply_finops+ |
| `/api/v1/supply/settlements/withdraw` | POST | `supply:settlement:withdraw` | supply_admin |
| `/api/v1/supply/billing` | GET | `billing:read` | supply_finops+ |

**Deprecated Alias 说明**：
- `/api/v1/supplier/*` 路径仅作为历史兼容别名保留
- 新接口禁止使用 `/supplier` 前缀
- deprecated alias 响应体应包含 `deprecation_notice` 字段提示迁移
- S2 阶段评估 alias 下线时间

### 4.3 Consumer API

| API路径 | 方法 | 所需Scope | 所需角色 |
|---------|------|-----------|----------|
| `/api/v1/consumer/account` | GET | `consumer:account:read` | consumer_viewer+ |
| `/api/v1/consumer/account` | PUT | `consumer:account:write` | consumer_operator+ |
| `/api/v1/consumer/apikeys` | GET | `consumer:apikey:read` | consumer_viewer+ |
| `/api/v1/consumer/apikeys` | POST | `consumer:apikey:create` | consumer_operator+ |
| `/api/v1/consumer/apikeys/:id/revoke` | POST | `consumer:apikey:revoke` | consumer_operator+ |
| `/api/v1/consumer/usage` | GET | `consumer:usage:read` | consumer_viewer+ |
| `/api/v1/consumer/billing` | GET | `billing:read` | consumer_viewer+ |

### 4.4 Router API（网关调用）

| API路径 | 方法 | 所需Scope | 所需角色 |
|---------|------|-----------|----------|
| `/v1/chat/completions` | POST | `router:invoke` | 所有认证用户 |
| `/v1/completions` | POST | `router:invoke` | 所有认证用户 |
| `/v1/embeddings` | POST | `router:invoke` | 所有认证用户 |
| `/v1/models` | GET | `router:model:list` | viewer+ |
| `/api/v1/router/models` | GET | `router:model:list` | viewer+ |
| `/api/v1/router/policies` | GET | `router:model:config` | operator+ |
| `/api/v1/router/policies` | PUT | `router:model:config` | operator+ |

---

## 5. 数据模型扩展

### 5.1 Role 定义表（iam_roles）

```sql
CREATE TABLE iam_roles (
    id BIGSERIAL PRIMARY KEY,
    role_code VARCHAR(50) NOT NULL UNIQUE,  -- super_admin, org_admin, operator, developer, finops, viewer
    role_name VARCHAR(100) NOT NULL,
    role_type VARCHAR(20) NOT NULL,          -- platform, supply, consumer
    parent_role_id BIGINT REFERENCES iam_roles(id),  -- 继承关系
    level INT NOT NULL DEFAULT 0,            -- 权限层级
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,

    -- 审计字段（符合 database_domain_model_and_governance v1 规范）
    request_id VARCHAR(64),                  -- 请求追踪ID
    created_ip INET,                         -- 创建者IP
    updated_ip INET,                          -- 更新者IP
    version INT DEFAULT 1,                   -- 乐观锁版本号

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_iam_roles_code ON iam_roles(role_code);
CREATE INDEX idx_iam_roles_type ON iam_roles(role_type);
CREATE INDEX idx_iam_roles_request_id ON iam_roles(request_id);
```

### 5.2 Scope 定义表（iam_scopes）

```sql
CREATE TABLE iam_scopes (
    id BIGSERIAL PRIMARY KEY,
    scope_code VARCHAR(100) NOT NULL UNIQUE,  -- platform:read, supply:account:write
    scope_name VARCHAR(100) NOT NULL,
    scope_type VARCHAR(50) NOT NULL,          -- platform, supply, consumer, router
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,

    -- 审计字段（符合 database_domain_model_and_governance v1 规范）
    request_id VARCHAR(64),                  -- 请求追踪ID
    created_ip INET,                         -- 创建者IP
    updated_ip INET,                          -- 更新者IP
    version INT DEFAULT 1,                   -- 乐观锁版本号

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_iam_scopes_code ON iam_scopes(scope_code);
CREATE INDEX idx_iam_scopes_request_id ON iam_scopes(request_id);
```

### 5.3 角色-Scope 关联表（iam_role_scopes）

```sql
CREATE TABLE iam_role_scopes (
    id BIGSERIAL PRIMARY KEY,
    role_id BIGINT NOT NULL REFERENCES iam_roles(id),
    scope_id BIGINT NOT NULL REFERENCES iam_scopes(id),

    -- 审计字段（符合 database_domain_model_and_governance v1 规范）
    request_id VARCHAR(64),                  -- 请求追踪ID
    created_ip INET,                         -- 创建者IP
    version INT DEFAULT 1,                   -- 乐观锁版本号

    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(role_id, scope_id)
);

CREATE INDEX idx_iam_role_scopes_role ON iam_role_scopes(role_id);
CREATE INDEX idx_iam_role_scopes_scope ON iam_role_scopes(scope_id);
CREATE INDEX idx_iam_role_scopes_request_id ON iam_role_scopes(request_id);
```

### 5.4 用户-角色关联表（iam_user_roles）

```sql
CREATE TABLE iam_user_roles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL REFERENCES iam_roles(id),
    tenant_id BIGINT,                         -- 租户范围（NULL表示全局）
    granted_by BIGINT,
    granted_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ,                   -- 角色过期时间

    -- 审计字段（符合 database_domain_model_and_governance v1 规范）
    request_id VARCHAR(64),                  -- 请求追踪ID
    created_ip INET,                         -- 创建者IP
    updated_ip INET,                          -- 更新者IP
    version INT DEFAULT 1,                   -- 乐观锁版本号

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_iam_user_roles_user ON iam_user_roles(user_id);
CREATE INDEX idx_iam_user_roles_tenant ON iam_user_roles(tenant_id);
CREATE INDEX idx_iam_user_roles_request_id ON iam_user_roles(request_id);
CREATE UNIQUE INDEX idx_iam_user_roles_unique ON iam_user_roles(user_id, role_id, tenant_id);
```

### 5.5 扩展 Token Claims

```go
type TokenClaims struct {
    jwt.RegisteredClaims
    SubjectID   string   `json:"subject_id"`    // 用户主体ID
    Role        string   `json:"role"`          // 主角色
    Scope       []string `json:"scope"`          // 授权范围列表
    TenantID    int64    `json:"tenant_id"`     // 租户ID
    UserType    string   `json:"user_type"`     // 用户类型: platform/supply/consumer
    Permissions []string `json:"permissions"`    // 细粒度权限列表
}
```

---

## 6. 中间件设计

### 6.1 扩展 ScopeRoleAuthzMiddleware

```go
// 扩展后的权限校验逻辑
type AuthzConfig struct {
    // 路由-角色映射
    RouteRolePolicies map[string]RolePolicy
    // 路由-Scope映射
    RouteScopePolicies map[string][]string
    // 角色层级
    RoleHierarchy map[string]int
}

type RolePolicy struct {
    RequiredLevel int
    RequiredRole string
    RequiredScope []string
}

// 权限校验逻辑
func (m *AuthMiddleware) ScopeRoleAuthzMiddleware(requiredScope string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims, ok := r.Context().Value(tokenClaimsKey).(*TokenClaims)
            if !ok {
                writeAuthError(w, http.StatusUnauthorized, "AUTH_CONTEXT_MISSING", "")
                return
            }

            // 1. Scope 校验
            if requiredScope != "" && !containsScope(claims.Scope, requiredScope) {
                m.emitAuditAndReject(r, w, "AUTH_SCOPE_DENIED", requiredScope, claims)
                return
            }

            // 2. 角色层级校验（如果配置了角色要求）
            if policy, exists := getRoutePolicy(r.URL.Path); exists {
                if !checkRolePolicy(claims, policy) {
                    m.emitAuditAndReject(r, w, "AUTH_ROLE_DENIED", "", claims)
                    return
                }
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

### 6.2 新增角色层级中间件

```go
// RoleHierarchyMiddleware 角色层级校验中间件
// 用于需要特定角色层级的操作
func RoleHierarchyMiddleware(minLevel int) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims := GetTokenClaims(r.Context())
            if claims == nil {
                writeAuthError(w, http.StatusUnauthorized, "AUTH_CONTEXT_MISSING", "")
                return
            }

            if getRoleLevel(claims.Role) < minLevel {
                writeAuthError(w, http.StatusForbidden, "AUTH_ROLE_LEVEL_DENIED",
                    fmt.Sprintf("required role level %d", minLevel))
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

### 6.3 新增跨类型校验中间件

```go
// UserTypeMiddleware 用户类型校验中间件
// 用于区分 platform/supply/consumer 用户
func UserTypeMiddleware(allowedTypes ...string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims := GetTokenClaims(r.Context())
            if claims == nil {
                writeAuthError(w, http.StatusUnauthorized, "AUTH_CONTEXT_MISSING", "")
                return
            }

            if !containsString(allowedTypes, claims.UserType) {
                writeAuthError(w, http.StatusForbidden, "AUTH_USER_TYPE_DENIED",
                    fmt.Sprintf("allowed user types: %v", allowedTypes))
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

---

## 7. 错误码扩展

| 错误码 | HTTP状态 | 说明 |
|--------|----------|------|
| `AUTH_SCOPE_DENIED` | 403 | Scope权限不足 |
| `AUTH_ROLE_DENIED` | 403 | 角色权限不足 |
| `AUTH_ROLE_LEVEL_DENIED` | 403 | 角色层级不足 |
| `AUTH_USER_TYPE_DENIED` | 403 | 用户类型不允许 |
| `AUTH_TENANT_MISMATCH` | 403 | 租户上下文不匹配 |
| `AUTH_RESOURCE_OWNER_DENIED` | 403 | 资源所有权校验失败 |

---

## 8. 审计事件扩展

| 事件名 | 说明 | 触发场景 |
|--------|------|----------|
| `role.assign` | 角色分配 | 给用户分配角色 |
| `role.revoke` | 角色吊销 | 吊销用户角色 |
| `role.scope.denied` | Scope权限拒绝 | Scope校验失败 |
| `role.hierarchy.denied` | 角色层级拒绝 | 角色层级校验失败 |
| `usertype.denied` | 用户类型拒绝 | 用户类型校验失败 |

---

## 9. API 契约更新

### 9.1 新增角色管理 API

#### GET /api/v1/iam/roles

获取角色列表

**响应：**
```json
{
  "roles": [
    {
      "role_code": "org_admin",
      "role_name": "组织管理员",
      "role_type": "platform",
      "level": 50,
      "scopes": ["platform:read", "tenant:read", "tenant:write"]
    }
  ]
}
```

#### POST /api/v1/iam/users/:userId/roles

分配角色给用户

**请求：**
```json
{
  "role_code": "developer",
  "tenant_id": 123,
  "expires_at": "2026-12-31T23:59:59Z"
}
```

#### DELETE /api/v1/iam/users/:userId/roles/:roleCode

吊销用户角色

### 9.2 新增 Scope 查询 API

#### GET /api/v1/iam/scopes

获取所有可用Scope

---

## 10. 向后兼容方案

### 10.1 新旧层级映射表（与TOK-001对齐）

| TOK-001旧层级 | 旧角色代码 | 新角色代码 | 新层级 | 权限变化说明 |
|---------------|------------|------------|--------|--------------|
| 3 | admin | `super_admin` | 100 | 完全对应，平台最高权限 |
| 2 | owner | `supply_admin` | 40 | 权限范围明确为供应侧管理，不含平台运营权限 |
| 1 | viewer | `viewer` | 10 | 完全对应 |

**说明**：
- TOK-001 新角色体系（super_admin/org_admin/operator）专属于平台侧管理
- 原 owner 角色对应 supply_admin（供应侧管理员），职责边界清晰
- 层级数值用于优先级判断，新旧体系独立运作

### 10.2 现有角色映射

| 旧角色 | 新角色 | 说明 |
|--------|--------|------|
| `admin` | `super_admin` | 完全对应，层级100 |
| `owner` | `supply_admin` | 权限范围重新定义为供应侧，不含平台运营权限 |
| `viewer` | `viewer` | 完全对应，层级10 |

**权限边界变化说明**：
- 原 owner 可管理供应侧账号、套餐、结算（对应 supply_admin）
- 原 owner 不可执行平台级操作（由 org_admin/super_admin 专属）
- supply_admin(40) < org_admin(50) 是合理设计，因为 org_admin 管理范围更广

### 10.3 Token 兼容处理

```go
// RoleMapping 旧角色到新角色的映射
var RoleMapping = map[string]string{
    "admin": "super_admin",
    "owner": "supply_admin",
    // viewer 保持不变
}

// 在Token验证时自动转换
func normalizeRole(role string) string {
    if newRole, exists := RoleMapping[role]; exists {
        return newRole
    }
    return role
}
```

---

## 11. 实施计划

### 11.1 Phase 1: 数据模型扩展

1. 创建 `iam_roles`, `iam_scopes`, `iam_role_scopes`, `iam_user_roles` 表
2. 初始化预定义角色和Scope数据
3. 提供数据迁移脚本

### 11.2 Phase 2: 中间件扩展

1. 扩展 `ScopeRoleAuthzMiddleware` 支持新角色层级
2. 新增 `RoleHierarchyMiddleware`
3. 新增 `UserTypeMiddleware`
4. 更新 Token Claims 结构

### 11.3 Phase 3: API 实现

1. 实现角色管理 API
2. 实现 Scope 查询 API
3. 更新现有 API 的权限校验

### 11.4 Phase 4: 向后兼容

1. 实现角色映射逻辑
2. 提供迁移指导文档

---

## 12. 验收标准

1. [ ] 角色层级清晰：super_admin > org_admin > operator/developer/finops > viewer
2. [ ] Scope权限校验正确：精确匹配路由与所需Scope
3. [ ] 继承关系正确：子角色自动继承父角色Scope
4. [ ] 向后兼容：现有 owner/viewer/admin 角色正常工作
5. [ ] 审计完整：角色变更和权限拒绝事件全量记录
6. [ ] API契约更新：新增角色管理API符合RESTful规范

---

## 13. 关联文档

- `docs/token_runtime_minimal_spec_v1.md`（TOK-001）
- `docs/token_auth_middleware_design_v1_2026-03-29.md`（TOK-002）
- `docs/llm_gateway_prd_v1_2026-03-25.md`
- `docs/database_domain_model_and_governance_v1_2026-03-27.md`
- `docs/api_naming_strategy_supply_vs_supplier_v1_2026-03-27.md`

---

**文档状态**：设计稿（待评审）
**下一步**：提交评审，根据反馈修订后进入实施阶段
