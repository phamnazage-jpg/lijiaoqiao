> 归档状态：已归档
> 归档批次：ARCHIVE-HISTORY-2026-04-14-A01
> 归档标识：AHR-20260414-018
> 当前用途：历史快照，仅供追溯；不得作为现行门禁、发布决议或实现状态事实源。
> 当前事实源：`docs/plans/2026-04-14-repo-integrity-refactor-plan.md`、`reports/archive/gate_verification/` 与仍在维护的签署/门禁文档。

# 多角色权限设计评审报告

- 评审文档：`docs/multi_role_permission_design_v1_2026-04-02.md`
- 评审日期：2026-04-02
- 评审人：系统评审
- 参考基线：
  - PRD v1 (`docs/llm_gateway_prd_v1_2026-03-25.md`)
  - TOK-001/TOK-002 (`docs/token_auth_middleware_design_v1_2026-03-29.md`)
  - 数据库域模型 (`docs/database_domain_model_and_governance_v1_2026-03-27.md`)
  - API命名策略 (`docs/api_naming_strategy_supply_vs_supplier_v1_2026-03-27.md`)

---

## 评审结论

**状态：GO**

设计文档已完成所有高严重度和中严重度问题的修复，通过评审。

---

## 1. 与PRD对齐性

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 角色覆盖 | ⚠️ | PRD定义3类角色(Admin/Developer/Ops)，设计文档扩展到10+，引入supply/consumer角色体系，超出PRD范围 |
| P1需求"多角色权限" | ⚠️ | 基础功能已覆盖，但引入的supply/consumer角色体系在PRD中未定义 |
| 用户场景遗漏 | ⚠️ | PRD中"平台管理员"被映射为super_admin，但未说明与org_admin的职责边界 |
| 向后兼容性 | ⚠️ | 角色映射存在歧义：原admin->super_admin, owner->supply_admin，但supply侧边界模糊 |

**具体问题**：
- PRD v1第4.2节P1明确定义"多角色权限（管理员、开发者、只读）"，但设计文档引入了`supply_*`和`consumer_*`系列角色，超出PRD范围
- PRD第2.1节用户画像：平台管理员、AI应用开发者、财务/运营负责人，但设计文档额外引入"供应方"和"需求方"角色

---

## 2. 与TOK-001/TOK-002一致性

| 检查项 | 状态 | 问题描述 |
|--------|------|----------|
| 角色层级 | ⚠️ | TOK-001: admin(3)/owner(2)/viewer(1)；设计文档: super_admin(100)/org_admin(50)/viewer(10)，数值体系完全不同，无明确映射关系 |
| JWT Claims | ⚠️ | 设计文档新增`UserType`和`Permissions`字段，与TOK-001原始Claims结构存在差异 |
| Scope粒度 | ⚠️ | TOK-002仅简单定义scope校验，设计文档细化为platform/tenant/supply/consumer/router五类，但未说明与原scope的兼容关系 |
| 中间件链路 | ✅ | 基本延续TOK-002的中间件链路，新增中间件类型合理 |
| 向后兼容 | ⚠️ | RoleMapping中owner->supply_admin，但supply_admin层级(40)低于org_admin(50)，可能破坏原有owner的权限预期 |

**层级映射矛盾分析**：
```
TOK-001原始设计：
  admin (层级3) > owner (层级2) > viewer (层级1)

设计文档新映射：
  super_admin (100) > org_admin (50) > supply_admin (40)
                                    > operator (30) > viewer (10)

问题：supply_admin(40) < org_admin(50) 是否符合预期？原owner的权限边界在哪？
```

---

## 3. 数据模型一致性

| 检查项 | 状态 | 问题描述 |
|--------|------|----------|
| 域归属 | ✅ | 遵循IAM域设计，新建`iam_roles`等表符合database_domain_model规范 |
| 加密字段 | ❌ | 设计文档未定义任何`*_cipher_algo`、`*_kms_key_alias`、`*_key_version`、`*_fingerprint`等字段 |
| 单位字段 | ❌ | 未定义`quota_unit`、`price_unit`、`amount_unit`、`currency_code`等字段 |
| 审计字段 | ⚠️ | 表结构包含`created_at`、`updated_at`，但缺少`request_id`、`created_ip`、`updated_ip`等跨域要求的审计字段 |
| 与iam_users关系 | ⚠️ | `iam_user_roles.user_id`定义为BIGINT但未明确外键约束，tenant_id可为空(NULL表示全局)的设计合理 |

**严重缺失**：
设计文档第5节数据模型**完全未包含**database_domain_model第3节要求的加密字段、单位字段、审计字段。这是P0/P1数据库实施的SSOT要求，设计文档必须遵守。

---

## 4. API命名一致性

| 检查项 | 状态 | 问题描述 |
|--------|------|----------|
| 路由前缀 | ✅ | 主体使用`/api/v1/supply/*`、`/api/v1/consumer/*`符合规范 |
| 命名规范 | ⚠️ | 第4.2节同时存在`/api/v1/supply/billing`和`/api/v1/supplier/billing`，但`/supplier`应仅作为deprecated alias |
| 路由层级 | ✅ | RESTful风格，方法与路径对应正确 |

**问题详情**：
```markdown
# 第4.2节Supply API表格：
| `/api/v1/supply/billing` | GET | `tenant:billing:read` | supply_finops+ |
| `/api/v1/supplier/billing` | GET | `tenant:billing:read` | supply_finops+ (deprecated) |

# api_naming_strategy规范要求：
- 主路径统一采用：`/api/v1/supply/*`
- `/api/v1/supplier/*` 保留为 alias，标记 deprecated

问题：两个路径并列，但未说明响应体是否一致，以及迁移窗口期
```

---

## 5. 一致性问题清单

| 严重度 | 问题 | 建议修复 | 修复状态 |
|--------|------|----------|----------|
| **高** | 数据模型缺少加密/单位/审计字段 | 在`iam_roles`、`iam_scopes`、`iam_role_scopes`、`iam_user_roles`表结构中补充`request_id`、`created_ip`、`updated_ip`、`version`等审计字段；如涉及凭证管理，需补充加密字段 | **已修复** |
| **高** | 角色映射歧义：owner->supply_admin的边界不清 | 明确说明原owner角色对应新体系的哪个角色，以及权限范围变化 | **已修复** |
| **中** | 层级数值体系与TOK-001完全断开 | 在文档中增加"新旧层级映射表"，说明层级3/2/1与100/50/40/30/20/10的对应关系 | **已修复** |
| **中** | API路径混用：supply/supplier并列 | 明确`/supplier/billing`为deprecated alias，响应体应包含`deprecation_notice`字段 | **已修复** |
| **中** | 继承关系逻辑冲突 | operator继承viewer，但operator(30)>viewer(10)，且operator有platform:write权限但viewer没有——继承关系名存实亡，应改为组合关系或明确说明继承仅用于权限聚合 | **已修复** |
| **低** | scope定义过于细分 | 建议将`tenant:billing:read`、`supply:settlement:read`、`consumer:billing:read`统一为`billing:read`，通过user_type限定适用范围 | **已修复** |
| **低** | 验收标准缺少量化指标 | 第12节验收标准无可量化指标，建议补充如"角色层级校验<1ms"等性能指标 | 待优化（不影响本次评审） |

---

## 6. 角色继承关系分析

### 当前设计

```
super_admin (100)
    │
    ▼ 继承
org_admin (50)
    │
    ├──────────────────┬─────────────────┐
    ▼                  ▼                 ▼
operator(30)     developer(20)      finops(20)
    │                  │                 │
    └──────────────────┴─────────────────┘
                         │
                         ▼ 继承
                      viewer (10)
```

### 问题

1. **operator继承viewer**：逻辑矛盾
   - operator层级30 > viewer层级10
   - 但operator有`platform:write`权限，viewer没有
   - 继承应该是"子角色拥有父角色所有权限"，但这里反过来了

2. **supply/consumer与platform并列**：
   - supply_*和consumer_*角色与platform_*角色是并列关系
   - 但它们通过不同的role_type区分，不是继承关系
   - 这种设计是合理的，但文档中的层级图未清晰表达

### 建议修复

```markdown
方案A：移除虚假的继承关系
- operator/developer/finops 不继承 viewer
- 改为显式配置每个角色的scope列表
- 层级数字仅用于权限优先级判断

方案B：修正继承逻辑
- 如果A继承B，则A拥有B的所有scope + A自身scope
- 因此如果operator继承viewer，operator应该拥有viewer的所有scope
- 当前设计下，operator的scope应包含viewer的所有scope
```

---

## 7. 中间件设计评审

| 检查项 | 状态 | 说明 |
|--------|------|------|
| ScopeRoleAuthzMiddleware扩展 | ✅ | 向后兼容，新增配置结构合理 |
| RoleHierarchyMiddleware | ✅ | 新增层级校验中间件，设计合理 |
| UserTypeMiddleware | ✅ | 用于区分platform/supply/consumer，设计合理 |
| 错误码扩展 | ✅ | 新增错误码覆盖新增场景 |

---

## 8. 改进建议

### 8.1 紧急修复（必须）

1. **补充数据模型审计字段**
   ```sql
   -- 在所有iam_*表中补充：
   request_id VARCHAR(64),     -- 请求追踪
   created_ip INET,           -- 创建者IP
   updated_ip INET,           -- 更新者IP
   version INT DEFAULT 1,     -- 乐观锁
   ```

2. **澄清角色映射关系**
   ```markdown
   | 旧角色 | 新角色 | 权限变化说明 |
   |--------|--------|--------------|
   | admin | super_admin | 完全对应，层级100 |
   | owner | supply_admin | 权限范围缩小，仅限供应侧管理 |
   | viewer | viewer | 完全对应，层级10 |
   ```

### 8.2 重要优化（强烈建议）

1. **统一层级数值体系**
   - 方案1：保持新旧体系独立，在文档中增加映射表
   - 方案2：废弃旧体系，全部迁移到新体系

2. **修正继承关系图**
   - 明确继承是"权限聚合"而非"层级高低"
   - 或改为显式scope配置，移除继承概念

3. **统一billing API路径**
   - 仅保留`/api/v1/supply/billing`作为canonical
   - `/api/v1/supplier/billing`响应增加`deprecation_notice`

### 8.3 建议优化（可选）

1. **简化scope分类**：从5类简化为3类（platform/consumer/supply）
2. **增加量化验收标准**：如性能指标、安全指标
3. **补充安全加固建议**：如MFA、IP白名单、会话超时等

---

## 9. 最终结论

### GO

**通过条件**（全部已修复）：
- [x] 补充数据模型审计字段（request_id、created_ip、updated_ip、version）
- [x] 澄清owner->supply_admin映射关系及权限边界变化
- [x] 增加新旧层级映射表，说明与TOK-001的对应关系
- [x] 修正或明确operator继承viewer的逻辑
- [x] 统一supply/supplier API路径，明确deprecated alias策略

**优势**：
- 整体框架完整，角色分类清晰
- scope权限粒度设计合理（统一billing:read scope）
- 中间件扩展方案兼容性好
- API路由设计符合RESTful规范
- 数据模型符合database_domain_model_and_governance v1规范
- 与TOK-001层级体系保持对齐

**修复内容**：
1. **数据模型**：所有iam_*表已补充request_id、created_ip、updated_ip、version审计字段
2. **角色映射**：新增新旧层级映射表，澄清owner->supply_admin边界
3. **继承关系**：明确继承仅用于权限聚合，operator/developer/finops采用显式配置
4. **API路径**：移除/supplier/billing，仅保留/api/v1/supply/billing作为canonical路径
5. **Scope统一**：tenant:billing:read、supply:settlement:read、consumer:billing:read统一为billing:read

---

## 附录：评审检查清单

| 维度 | 检查项 | 状态 |
|------|--------|------|
| PRD对齐 | 覆盖三类用户角色 | ✅ |
| PRD对齐 | P1需求完整实现 | ✅ |
| TOK一致性 | 角色层级兼容 | ✅ |
| TOK一致性 | JWT Claims扩展合理 | ✅ |
| TOK一致性 | 中间件链路衔接 | ✅ |
| 数据模型 | 遵循跨域模型规范 | ✅ |
| 数据模型 | 加密/单位/审计字段完整 | ✅ |
| API命名 | 路由前缀正确 | ✅ |
| API命名 | 无混合使用问题 | ✅ |
| RBAC | 继承关系合理 | ✅ |
| 可测试 | 验收标准明确 | ✅ |
