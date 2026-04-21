# P4-C 分析：IAM tenant/operator/scope 闭环

生成时间: 2026-04-21
Phase: P4-C

---

## 一、当前 IAM 系统全貌

### 1.1 Claims 结构 (scope_auth.go:27-37)

```go
type IAMTokenClaims struct {
    SubjectID   string   `json:"subject_id"`   // 操作者唯一标识
    Role        string   `json:"role"`         // 角色: super_admin/org_admin/supply_admin/...
    Scope       []string `json:"scope"`        // 细粒度权限列表
    TenantID    int64    `json:"tenant_id"`    // 租户ID（0=平台级）
    UserType    string   `json:"user_type"`    // platform/supply/consumer
    Permissions []string `json:"permissions"`   // 备用细粒度权限列表
    Version     int      `json:"version,omitempty"`
}
```

### 1.2 角色层级 (role.go:32-46)

| 角色 | Level | 类型 |
|------|-------|------|
| super_admin | 100 | platform |
| org_admin | 50 | platform |
| supply_admin | 40 | supply |
| consumer_admin | 40 | consumer |
| operator | 30 | platform |
| supply_operator | 30 | supply |
| consumer_operator | 30 | consumer |
| developer | 20 | platform |
| finops | 20 | billing |
| viewer | 10 | platform |

### 1.3 预定义 Scope (scope.go:167-210)

共 30+ 预定义 scope，覆盖：
- **platform**: platform:read/write/admin, tenant:read/write/member:manage/billing:write
- **supply**: supply:account:read/write, supply:package:read/write/publish/offline, supply:settlement:withdraw, supply:credential:manage
- **consumer**: consumer:account:read/write, consumer:apikey:create/read/revoke, consumer:usage:read
- **billing**: billing:read/write
- **router**: router:invoke/model:list/model:config
- **wildcard**: `*`

---

## 二、闭环分析

### 2.1 什么是"闭环"

tenant/operator/scope 闭环 = **三个维度互相验证，任何一个维度被破坏都被拦截**：

```
Tenant A 的 operator
  只能操作 Tenant A 的资源
  只能使用 Tenant A 类型匹配的 Scope
  不能跨 Tenant 操作
  不能使用不属于自己的 Role 对应的 Scope
```

### 2.2 三个闭环检查点

#### 闭环点 1: Tenant 隔离（TenantID 校验）

**预期**：每个请求的 TenantID 必须与操作资源的 TenantID 一致。

**当前实现**：
- `IAMTokenClaims.TenantID` 存在于 JWT claims 中
- `ScopeAuthMiddleware` 的 `RequireScope` 等中间件**不检查 TenantID**

**缺口**：无法确认 operator 操作的资源是否属于同一个租户。

```
举例：operator 持有 TenantID=5 的 token
     访问 GET /api/accounts/999 （account属于TenantID=3）
     当前系统：返回 200（有scope即可）
     期望行为：返回 403（资源不属于同一租户）
```

#### 闭环点 2: Operator 身份校验（SubjectID）

**预期**：所有写操作必须记录 SubjectID（操作者），用于审计。

**当前实现**：
- `IAMTokenClaims.SubjectID` 存在
- 审计日志写入时使用 `RequestID` 作为追踪（background.go 等处）

**缺口**：
- 审计字段有 `RequestID` 但 `SubjectID` 审计字段未确认是否在所有写路径填充
- 需要grep确认 audit store 的 `CreatedBy`/`UpdatedBy` 是否用 SubjectID 填充

#### 闭环点 3: Scope-UserType 匹配校验

**预期**：supply 类型 operator 只能使用 supply:* scope，不能使用 consumer:* scope。

**当前实现**：
- `scope.go:GetScopeTypeFromCode()` 可以从 scope code 推断 type
- 但 `ScopeAuthMiddleware` 的 `RequireScope` 不检查 UserType 与 scope type 的一致性

**缺口**：
```
举例：UserType=supply 的 token，scope=[consumer:account:read]
     访问 GET /consumer/accounts
     当前系统：返回 200（scope存在即放行）
     期望行为：返回 403（supply operator 不能用 consumer scope）
```

---

## 三、缺口优先级分析

| 缺口 | 风险等级 | 说明 | 修复方式 |
|------|---------|------|---------|
| Tenant 资源隔离 | **P0** | 跨租户数据访问 | 在 service 层加 TenantID 校验 |
| Scope-UserType 校验 | **P1** | supply operator 可用 consumer scope | 中间件扩展：校验 scope type 与 UserType 匹配 |
| SubjectID 审计填充 | **P2** | 审计日志缺少操作者 | 审计中间件注入 SubjectID |

---

## 四、P4-C-01~08 执行计划

### P4-C-01: 确认 SubjectID 审计填充率

```
目标：grep audit store 的 Create/Update 方法，确认 CreatedBy/UpdatedBy 字段
命令：grep -rn "CreatedBy\|UpdatedBy\|AuditModel" supply-api/internal --include="*.go" | grep -v "_test.go" | head -20
完成标准：所有 domain service 写操作写入 SubjectID
```

### P4-C-02: 补充 TenantAware 接口

```go
// supply-api/internal/iam/model/tenant.go（新建）
type TenantAware interface {
    GetTenantID() int64
}

// 为所有资源模型实现此接口
func (a *Account) GetTenantID() int64 { return a.TenantID }
```

### P4-C-03: service 层添加 TenantID 校验

在所有读操作的 service 方法中，添加：
```go
if resource.GetTenantID() != claims.TenantID {
    return ErrTenantAccessDenied
}
```

### P4-C-04: ScopeType 推断函数

```go
// scope_auth.go 新增
func ValidateScopeTypeMatch(claims *IAMTokenClaims, requiredScope string) bool {
    // supply:* scope → UserType 必须是 supply/consumer/platform（含 platform 兼容性）
    // consumer:* scope → UserType 必须是 consumer/platform
    // platform:* / tenant:* / billing:* → UserType 必须是 platform
    scopeType := model.GetScopeTypeFromCode(requiredScope)
    return validateUserTypeScopeMatch(claims.UserType, scopeType)
}
```

### P4-C-05: 中间件扩展 RequireScopeWithUserType

```go
func (m *ScopeAuthMiddleware) RequireScopeWithUserType(requiredScope string) func(http.Handler) http.Handler {
    // 原有 RequireScope 逻辑 + scope type 匹配校验
}
```

### P4-C-06: P2-01 通配符 scope 审计日志（已有）

`scope_auth.go:237-254` 已有 `logWildcardScopeAccess()` 实现（P2-01 已完成）。

### P4-C-07: 写路径 SubjectID 注入中间件

```go
// 在 auth 中间件最后，将 SubjectID 注入 request context
ctx = context.WithValue(ctx, SubjectIDKey, claims.SubjectID)
r = r.WithContext(ctx)
```

### P4-C-08: 集成测试

```bash
# 跨租户访问测试
# supply operator 访问其他租户资源 → 期望 403

# scope type 匹配测试
# UserType=supply + scope=consumer:account:read → 期望 403

# SubjectID 审计测试
# 写操作后查 audit 表，CreatedBy == SubjectID
```

---

## 五、已完成的 IAM 能力（无需重复实现）

- ✅ IAMTokenClaims 定义（SubjectID/Role/Scope/TenantID/UserType）
- ✅ RoleHierarchyLevels 角色层级
- ✅ PredefinedScopes 30+ 预定义 scope
- ✅ ScopeAuthMiddleware（RequireScope/RequireAllScopes/RequireAnyScope/RequireRole/RequireMinLevel）
- ✅ P2-01 通配符 scope 审计日志（logWildcardScopeAccess）
- ✅ Claims 版本迁移（MigrateClaims）
- ✅ Claims 完整性校验（ValidateClaims）
- ✅ 角色层级中间件（RequireMinLevel）
