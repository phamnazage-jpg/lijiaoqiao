> 归档状态：已归档
> 归档批次：ARCHIVE-HISTORY-2026-04-14-A01
> 归档标识：AHP-20260414-021
> 当前用途：历史快照，仅供追溯；不得作为现行门禁、发布决议或实现状态事实源。
> 当前事实源：`docs/plans/2026-04-14-repo-integrity-refactor-plan.md`、`reports/archive/gate_verification/` 与仍在维护的签署/门禁文档。

# 立交桥项目设计文档全面审查报告

> 报告日期：2026-04-03
> 审查类型：设计文档全面审查（清理后新版）
> 审查范围：全部保留的设计文档
> 审查标准：行业最佳实践 + 内部一致性 + 可实施性

---

## 一、审查结论

| 维度 | 评分 | 状态 | 说明 |
|------|------|------|------|
| **总体结论** | **CONDITIONAL GO** | ⚠️ 有条件通过 | 需修复一致性问题 |
| 架构设计 | 82/100 | 良好 | 分层清晰，有少量不一致 |
| 安全设计 | 85/100 | 良好 | M-013~M-016体系完整 |
| 权限设计 | 80/100 | 良好 | 角色/Scope体系完整 |
| 路由设计 | 85/100 | 良好 | 策略模板设计完善 |
| 审计设计 | 88/100 | 优秀 | 分类体系完整，SQL可执行 |
| 合规设计 | 82/100 | 良好 | 规则库完整，CI集成完善 |
| SSO调研 | 80/100 | 良好 | 供应商对比全面 |
| **文档一致性** | **72/100** | ⚠️ 需改进 | 多处命名/接口不一致 |

---

## 二、文档清单与状态

### 2.1 核心设计文档（保留）

| 文档 | 版本 | 日期 | 状态 | 评分 |
|------|------|------|------|------|
| `llm_gateway_prd_v1_2026-03-25.md` | v1.0 | 03-25 | ✅ 冻结 | 85/100 |
| `supply_technical_design_enhanced_v1_2026-03-25.md` | v1.1 | 03-27 | ✅ 生效 | 82/100 |
| `supply_button_level_prd_v1_2026-03-25.md` | v1.1 | 03-27 | ✅ 冻结 | 80/100 |
| `database_domain_model_and_governance_v1_2026-03-27.md` | v1.0 | 03-27 | ✅ 生效 | 85/100 |
| `acceptance_gate_single_source_v1_2026-03-18.md` | v1.0 | 03-18 | ✅ 生效 | 80/100 |

### 2.2 新增优化设计文档（P1/P2）

| 文档 | 版本 | 日期 | 状态 | 评分 |
|------|------|------|------|------|
| `audit_log_enhancement_design_v1_2026-04-02.md` | v2.0 | 04-03 | ✅ 定稿 | 88/100 |
| `multi_role_permission_design_v1_2026-04-02.md` | v1.0 | 04-02 | ⚠️ 待评审 | 80/100 |
| `routing_strategy_template_design_v1_2026-04-02.md` | v1.1 | 04-02 | ✅ 定稿 | 85/100 |
| `compliance_capability_package_design_v1_2026-04-02.md` | v1.0 | 04-02 | ⚠️ 待评审 | 82/100 |
| `sso_saml_technical_research_v1_2026-04-02.md` | v1.1 | 04-02 | ✅ 完成 | 80/100 |

### 2.3 技术规格文档

| 文档 | 状态 | 评分 |
|------|------|------|
| `token_runtime_minimal_spec_v1.md` | ✅ | 82/100 |
| `token_auth_middleware_design_v1_2026-03-29.md` | ✅ | 80/100 |
| `token_lifecycle_audit_test_assertions_v1_2026-03-29.md` | ✅ | 78/100 |
| `dependency_compatibility_audit_baseline_v1_2026-03-27.md` | ✅ | 75/100 |

---

## 三、行业最佳实践评估

### 3.1 架构设计评估

| 最佳实践 | 符合度 | 说明 |
|----------|--------|------|
| 分层架构（Domain/Service/Repository） | ✅ 90% | 清晰的分层，但audit接口不统一 |
| 依赖注入 | ✅ 85% | 构造函数注入，但部分使用全局变量 |
| 接口隔离原则 | ⚠️ 70% | AuditStore有新旧两套接口 |
| 单一职责原则 | ✅ 85% | 模块职责清晰 |
| 开放封闭原则 | ✅ 80% | 策略模式支持扩展 |
| CQRS模式 | ⚠️ 60% | 读写未明确分离 |

### 3.2 安全设计评估

| 最佳实践 | 符合度 | 说明 |
|----------|--------|------|
| OWASP Top 10防护 | ✅ 85% | 覆盖主要风险 |
| 最小权限原则 | ✅ 90% | RBAC+Scope细粒度控制 |
| 凭证管理 | ✅ 88% | 轮换/吊销/脱敏完整 |
| 审计追踪 | ✅ 90% | 完整事件分类体系 |
| 加密传输 | ✅ 85% | mTLS设计完整 |
| 速率限制 | ✅ 85% | TokenBucket+SlidingWindow |

### 3.3 权限设计评估

| 最佳实践 | 符合度 | 说明 |
|----------|--------|------|
| RBAC模型 | ✅ 90% | 角色/Scope/继承完整 |
| 最小权限 | ✅ 85% | Scope细粒度控制 |
| 权限继承 | ✅ 80% | 显式配置+继承混合 |
| 向后兼容 | ✅ 85% | 角色映射完整 |
| JWT Claims扩展 | ✅ 80% | UserType/Permissions新增 |

### 3.4 审计设计评估

| 最佳实践 | 符合度 | 说明 |
|----------|--------|------|
| 事件分类体系 | ✅ 95% | CRED/AUTH/DATA/CONFIG/SECURITY |
| 不可篡改 | ✅ 90% | UUID+时间戳+JSONB |
| 数据保留 | ✅ 85% | 365天+归档表 |
| 性能优化 | ✅ 80% | 批量写入+索引策略 |
| 合规对齐 | ✅ 90% | M-013~M-016直接映射 |

### 3.5 路由设计评估

| 最佳实践 | 符合度 | 说明 |
|----------|--------|------|
| 策略模式 | ✅ 90% | 模板+参数可配置 |
| Fallback机制 | ✅ 90% | 多级降级完整 |
| 灰度发布 | ✅ 85% | RolloutConfig完善 |
| A/B测试 | ✅ 85% | 一致性哈希分桶 |
| 可观测性 | ✅ 85% | M-006/M-007/M-008指标 |

---

## 四、文档一致性问题

### 4.1 🔴 严重不一致（阻断实施）

#### I-001: AuditStore接口不统一

**涉及文档**：
- `audit_log_enhancement_design_v1_2026-04-02.md` 定义 `AuditStoreInterface`
- `supply-api/internal/audit/audit.go` 使用 `AuditStore`（旧接口）
- `gateway/internal/middleware/audit.go` 使用 `AuditEmitter`

**问题**：
```go
// 旧接口（domain层使用）
type AuditStore interface {
    Emit(ctx context.Context, event Event) error
}

// 新接口（审计增强设计）
type AuditStoreInterface interface {
    Emit(ctx context.Context, event *model.AuditEvent) error
    Query(ctx context.Context, filter *EventFilter) ([]*model.AuditEvent, int64, error)
    GetByIdempotencyKey(ctx context.Context, key string) (*model.AuditEvent, error)
}
```

**影响**：DatabaseAuditService无法直接替换MemoryAuditStore，需要适配器

**建议**：统一接口，或明确定义适配器模式

---

#### I-002: 事件命名风格不一致

**涉及文档**：
- `audit_log_enhancement_design_v1_2026-04-02.md` 使用 `AUTH-TOKEN-OK`
- `token_auth_middleware_design_v1_2026-03-29.md` 使用 `token.authn.success`

**对齐映射**（已在审计设计中定义）：
| 审计设计 | TOK-002 |
|----------|---------|
| `AUTH-TOKEN-OK` | `token.authn.success` |
| `AUTH-TOKEN-FAIL` | `token.authn.fail` |
| `AUTH-SCOPE-DENY` | `token.authz.denied` |
| `AUTH-QUERY-REJECT` | `token.query_key.rejected` |

**建议**：在代码中统一使用一种格式，另一种作为alias

---

#### I-003: 评分权重不一致

**涉及文档**：
- `technical_architecture_optimized_v2_2026-03-18.md`: 延迟40%/可用30%/成本20%/质量10%
- `routing_strategy_template_design_v1_2026-04-02.md`: 延迟40%/可用30%/成本20%/质量10% ✅ 已对齐

**状态**：✅ 已修复（v1.1版本已对齐）

---

### 4.2 ⚠️ 中等不一致（影响实施效率）

#### I-004: 角色层级数值体系不一致

**涉及文档**：
- `multi_role_permission_design_v1_2026-04-02.md`: super_admin=100, org_admin=50, supply_admin=40
- `supply-api/internal/iam/model/role.go`: LevelSuperAdmin=100, LevelOrgAdmin=50, LevelSupplyAdmin=40 ✅ 已对齐
- `supply-api/internal/iam/middleware/scope_auth.go`: 使用独立的roleHierarchyLevels map

**问题**：scope_auth.go中定义了重复的层级map，未引用model常量

**建议**：scope_auth.go引用model包的常量

---

#### I-005: Token Claims结构不一致

**涉及文档**：
- `token_runtime_minimal_spec_v1.md`: 基础Claims（SubjectID, Role, Scope, TenantID）
- `multi_role_permission_design_v1_2026-04-02.md`: 扩展Claims（+UserType, Permissions）

**状态**：✅ 设计上是扩展关系，但需要明确迁移路径

---

#### I-006: 错误码体系分散

**涉及文档**：
- `audit_log_enhancement_design_v1_2026-04-02.md`: 定义了错误码对照表
- `supply-api/internal/iam/middleware/scope_auth.go`: 使用 `AUTH_SCOPE_DENIED` 等
- `supply-api/internal/middleware/auth.go`: 使用 `AUTH_MISSING_BEARER` 等

**状态**：⚠️ 有对照表但未集中管理

**建议**：创建统一的错误码定义文件

---

### 4.3 🟡 轻微不一致（文档层面）

#### I-007: 供应商命名不一致

**涉及文档**：
- `api_naming_strategy_supply_vs_supplier_v1_2026-03-27.md`: 规定使用 `supply` 前缀
- 部分文档仍使用 `supplier` 前缀

**状态**：⚠️ 有命名策略文档，但部分旧文档未更新

---

#### I-008: 数据库表命名风格

**涉及文档**：
- `audit_log_enhancement_design_v1_2026-04-02.md`: `audit_events`（下划线）
- `multi_role_permission_design_v1_2026-04-02.md`: `iam_roles`（下划线）
- `database_domain_model_and_governance_v1_2026-03-27.md`: 应统一定义

**状态**：✅ 基本一致，都使用下划线命名

---

## 五、设计完整性评估

### 5.1 已覆盖的设计领域

| 领域 | 覆盖度 | 文档 |
|------|--------|------|
| 产品需求 | ✅ 100% | PRD v1 |
| 架构设计 | ✅ 95% | 技术架构v2 |
| 供应侧设计 | ✅ 95% | 增强技术设计+按钮级PRD |
| 数据库设计 | ✅ 90% | 领域模型+治理 |
| 安全设计 | ✅ 90% | 安全方案+审计增强 |
| 权限设计 | ✅ 90% | 多角色权限设计 |
| 路由设计 | ✅ 90% | 路由策略模板 |
| 合规设计 | ✅ 85% | 合规能力包 |
| API设计 | ✅ 85% | API解决方案 |
| 测试设计 | ✅ 80% | 测试计划 |
| SSO集成 | ✅ 80% | 技术调研 |

### 5.2 缺失的设计领域

| 领域 | 优先级 | 说明 |
|------|--------|------|
| 前端UI/UX详细设计 | P1 | 仅有supply_uiux_design_spec_v1 |
| 部署架构详细设计 | P1 | 缺少K8s/Helm配置 |
| 监控告警详细设计 | P1 | 仅有路由告警框架 |
| 数据迁移方案 | P2 | 从subapi迁移的详细方案 |
| 灾备设计 | P2 | 多区域部署方案 |
| 性能基准测试方案 | P2 | 压测场景定义 |

---

## 六、设计质量评估

### 6.1 审计日志增强设计（88/100）

**优点**：
1. ✅ 事件分类体系完整（CRED/AUTH/DATA/CONFIG/SECURITY）
2. ✅ M-013~M-016直接映射，SQL可执行
3. ✅ 索引策略完整，性能考虑周到
4. ✅ CI/CD Gate脚本生产级（重试/超时/错误处理）
5. ✅ 幂等性协议完整（201/202/409/200）
6. ✅ TPS达成路径清晰（3K→8K，无Kafka）

**问题**：
1. ⚠️ 分区表设计存在但不启用（数据量超过1000万再分）
2. ⚠️ 4个专用事件表增加复杂度，可考虑JSONB存储

### 6.2 多角色权限设计（80/100）

**优点**：
1. ✅ 角色体系完整（平台/供应/需求三侧）
2. ✅ Scope细粒度控制
3. ✅ 继承关系清晰（显式配置+继承混合）
4. ✅ 向后兼容方案完整
5. ✅ API路由权限映射完整

**问题**：
1. ⚠️ 角色层级数值与scope_auth.go重复定义
2. ⚠️ 缺少跨租户权限校验设计
3. ⚠️ 角色变更的审计事件定义不完整

### 6.3 路由策略模板设计（85/100）

**优点**：
1. ✅ 策略类型完整（成本/质量/延迟/模型/复合）
2. ✅ Fallback多级架构完善
3. ✅ 灰度发布+A/B测试支持
4. ✅ 与RateLimit/Alert集成设计完整
5. ✅ M-006/M-007/M-008指标采集完整
6. ✅ 评分权重与技术架构对齐

**问题**：
1. ⚠️ 策略配置热更新机制未详细说明
2. ⚠️ 缺少策略版本管理设计

### 6.4 合规能力包设计（82/100）

**优点**：
1. ✅ M-013~M-017规则化定义完整
2. ✅ CI/CD集成方案详细
3. ✅ SBOM+锁文件diff+兼容矩阵+风险登记册四件套
4. ✅ 与审计日志设计对齐

**问题**：
1. ⚠️ 合规报告自动化程度未明确
2. ⚠️ 缺少合规规则版本管理

### 6.5 SSO/SAML技术调研（80/100）

**优点**：
1. ✅ 供应商对比全面（Keycloak/Auth0/Okta/Casdoor/Ory/Azure AD）
2. ✅ 成本分析详细
3. ✅ Go集成方案明确
4. ✅ 分阶段实施建议合理

**问题**：
1. ⚠️ 缺少与现有Token体系的集成设计
2. ⚠️ 缺少数据迁移方案

---

## 七、与代码实现对齐检查

### 7.1 已实现 vs 设计

| 设计模块 | 设计文档 | 代码实现 | 对齐度 | 说明 |
|----------|----------|----------|--------|------|
| 审计事件分类 | audit_log_enhancement | ✅ 已实现 | 85% | 分类完整，存储未持久化 |
| 多角色权限 | multi_role_permission | ✅ 已实现 | 80% | 模型/中间件/Handler完整 |
| 路由策略 | routing_strategy_template | ⚠️ 部分实现 | 60% | 基础策略有，模板未实现 |
| 合规能力包 | compliance_capability | ⚠️ 部分实现 | 50% | 规则定义有，CI未集成 |
| SSO集成 | sso_saml_research | 🔴 未实现 | 0% | 仅调研 |

### 7.2 TODO清理状态

| TODO位置 | 内容 | 优先级 | 状态 |
|----------|------|--------|------|
| main.go:115 | DB-backed幂等 | P1 | 待实施 |
| main.go:474 | GetWithdrawableBalance | P0 | 待实施 |
| main.go:484 | ListRecords | P0 | 待实施 |
| main.go:489 | GetBillingSummary | P0 | 待实施 |
| main.go:495 | memoryTokenBackend | P1 | 临时实现 |

---

## 八、改进建议

### 8.1 立即修复（P0）

| 编号 | 问题 | 建议 | 影响文档 |
|------|------|------|----------|
| FIX-001 | AuditStore接口不统一 | 统一接口或定义适配器 | audit_log_enhancement |
| FIX-002 | 事件命名风格不一致 | 统一使用一种格式 | audit_log + token_auth |
| FIX-003 | 角色层级重复定义 | 引用model常量 | multi_role_permission |

### 8.2 短期改进（P1）

| 编号 | 问题 | 建议 |
|------|------|------|
| IMP-001 | 错误码分散 | 创建统一错误码定义文件 |
| IMP-002 | 供应商命名不一致 | 更新所有旧文档 |
| IMP-003 | Token Claims迁移路径 | 明确迁移步骤文档 |
| IMP-004 | 策略热更新机制 | 补充设计文档 |

### 8.3 中期改进（P2）

| 编号 | 问题 | 建议 |
|------|------|------|
| IMP-005 | 缺失部署架构设计 | 补充K8s/Helm设计 |
| IMP-006 | 缺失监控告警详细设计 | 补充Prometheus/Grafana设计 |
| IMP-007 | 缺失数据迁移方案 | 补充subapi迁移方案 |

---

## 九、总结

### 9.1 设计质量总评

| 维度 | 评分 | 评价 |
|------|------|------|
| 完整性 | 85/100 | 核心领域全覆盖，缺少部署/监控详细设计 |
| 一致性 | 72/100 | 存在接口/命名/权重不一致 |
| 可实施性 | 80/100 | 大部分设计可指导实施，部分需补充细节 |
| 行业对标 | 85/100 | 符合行业最佳实践 |
| 可扩展性 | 82/100 | 策略模式/接口设计支持扩展 |

### 9.2 最终结论

**CONDITIONAL GO** - 设计文档整体质量良好，符合行业最佳实践，但需修复3个P0一致性问题后方可作为实施基线。

### 9.3 下一步行动

1. **修复P0不一致性**：统一AuditStore接口、事件命名、角色层级
2. **补充缺失设计**：部署架构、监控告警、数据迁移
3. **代码对齐检查**：确保实现与设计一致
4. **建立文档治理机制**：定期审查一致性

---

**审查人**：多角色专家联合审查
**审查日期**：2026-04-03
**下次审查**：P0问题修复后
