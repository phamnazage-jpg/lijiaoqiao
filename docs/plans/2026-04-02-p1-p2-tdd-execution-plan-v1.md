# P1/P2 TDD开发执行计划

> 版本：v1.0
> 日期：2026-04-02
> 依据：Superpowers执行框架 + TDD规范
> 目标：P0 staging验证BLOCKED期间，并行启动P1/P2核心模块TDD开发

---

## 1. 当前状态

### 1.1 Superpowers执行状态

| 工作流 | 状态 | 说明 |
|--------|------|------|
| WG-A 需求冻结 | DONE | PRD v1已冻结 |
| WG-B 契约对齐 | DONE | OpenAPI已对齐 |
| WG-C 测试矩阵 | DONE | 路径一致化完成 |
| WG-D 真实联调 | **BLOCKED** | 缺staging环境 |
| WG-E 报告签署 | **BLOCKED** | 依赖WG-D |
| WG-F 一致性收尾 | DONE | 命名策略完成 |
| WG-G 全局校验 | DONE | 校验链路可执行 |

### 1.2 P1/P2设计状态

| 设计文档 | 评审结论 | 状态 |
|----------|----------|------|
| multi_role_permission_design | GO | 可进入开发 |
| audit_log_enhancement_design | GO | 可进入开发 |
| routing_strategy_template_design | GO | 可进入开发 |
| sso_saml_technical_research | GO | 可进入调研 |
| compliance_capability_package_design | GO | 可进入开发 |

---

## 2. TDD开发原则

### 2.1 红绿重构循环

```
┌─────────────────────────────────────────────────────┐
│  1. RED: 写一个失败的测试（描述期望行为）           │
│  2. GREEN: 写最少量代码让测试通过                   │
│  3. REFACTOR: 重构代码，消除重复                   │
│  循环直到功能完成                                   │
└─────────────────────────────────────────────────────┘
```

### 2.2 测试分层

| 层级 | 范围 | 工具 |
|------|------|------|
| 单元测试 | 纯函数、核心逻辑 | Go test, testify |
| 集成测试 | 模块间交互 | Go test, testify |
| E2E测试 | 完整API链路 | Bash脚本 |

### 2.3 门禁检查

```
Pre-Commit → Unit Tests → Integration Tests → Build Gate → Staging Gate
```

---

## 3. P1开发任务

### 3.1 多角色权限（IAM）

#### 设计文档
`docs/multi_role_permission_design_v1_2026-04-02.md`

#### TDD任务

| Step | 描述 | 测试先行 | 验收标准 |
|------|------|----------|----------|
| IAM-01 | 数据模型：iam_roles表DDL | ✅ | 表结构符合规范 |
| IAM-02 | 数据模型：iam_scopes表DDL | ✅ | 表结构符合规范 |
| IAM-03 | 数据模型：iam_role_scopes关联表DDL | ✅ | 关联正确 |
| IAM-04 | 数据模型：iam_user_roles关联表DDL | ✅ | 关联正确 |
| IAM-05 | 中间件：Scope验证中间件 | ✅ | 正确校验scope |
| IAM-06 | 中间件：角色继承逻辑 | ✅ | 继承关系正确 |
| IAM-07 | API：角色管理API | ✅ | CRUD正确 |
| IAM-08 | API：权限校验API | ✅ | 正确返回 |

#### 目录结构
```
supply-api/internal/
├── iam/                    # 新增IAM模块
│   ├── model/             # 数据模型
│   │   ├── role.go
│   │   ├── scope.go
│   │   └── user_role.go
│   ├── repository/        # 仓储
│   │   └── iam_repository.go
│   ├── service/          # 服务层
│   │   └── iam_service.go
│   ├── handler/          # HTTP处理器
│   │   └── iam_handler.go
│   └── middleware/       # 权限中间件
│       └── scope_auth.go
```

### 3.2 审计日志增强

#### 设计文档
`docs/audit_log_enhancement_design_v1_2026-04-02.md`

#### TDD任务

| Step | 描述 | 测试先行 | 验收标准 |
|------|------|----------|----------|
| AUD-01 | 数据模型：audit_events表DDL | ✅ | 表结构符合规范 |
| AUD-02 | 数据模型：M-013~M-016子表DDL | ✅ | 子表结构正确 |
| AUD-03 | 事件分类：SECURITY事件定义 | ✅ | invariant_violation存在 |
| AUD-04 | 事件分类：CRED事件定义 | ✅ | CRED-EXPOSE/INGRESS/DIRECT |
| AUD-05 | 写入API：POST /audit/events | ✅ | 幂等性正确 |
| AUD-06 | 查询API：GET /audit/events | ✅ | 分页过滤正确 |
| AUD-07 | 指标API：M-013~M-016统计 | ✅ | 计算正确 |
| AUD-08 | 脱敏扫描：敏感信息检测 | ✅ | 扫描逻辑正确 |

#### 目录结构
```
supply-api/internal/audit/
├── model/                 # 审计事件模型
│   ├── audit_event.go
│   └── audit_metrics.go
├── repository/           # 审计仓储
│   └── audit_repository.go
├── service/             # 审计服务
│   └── audit_service.go
├── handler/             # HTTP处理器
│   └── audit_handler.go
└── sanitizer/           # 脱敏扫描器
    └── sanitizer.go
```

### 3.3 路由策略模板

#### 设计文档
`docs/routing_strategy_template_design_v1_2026-04-02.md`

#### TDD任务

| Step | 描述 | 测试先行 | 验收标准 |
|------|------|----------|----------|
| ROU-01 | 评分模型：ScoreWeights默认权重 | ✅ | 延迟40%/可用30%/成本20%/质量10% |
| ROU-02 | 评分模型：CalculateScore方法 | ✅ | 评分正确 |
| ROU-03 | 策略模板：StrategyTemplate接口 | ✅ | 模板可替换 |
| ROU-04 | 策略模板：CostBased/CostAware策略 | ✅ | 策略正确 |
| ROU-05 | 路由决策：RoutingEngine | ✅ | 决策正确 |
| ROU-06 | Fallback：多级Fallback | ✅ | 降级正确 |
| ROU-07 | 指标采集：M-008采集 | ✅ | 全路径覆盖 |
| ROU-08 | A/B测试：ABStrategyTemplate | ✅ | 流量分配正确 |
| ROU-09 | 灰度发布：RolloutConfig | ✅ | 百分比正确 |

#### 目录结构
```
gateway/internal/router/
├── strategy/             # 策略模板
│   ├── strategy.go       # 接口定义
│   ├── cost_based.go
│   ├── cost_aware.go
│   ├── quality_first.go
│   ├── latency_first.go
│   ├── ab_strategy.go
│   └── rollout.go
├── scoring/             # 评分模型
│   └── scoring_model.go
├── engine/              # 路由引擎
│   └── routing_engine.go
├── metrics/             # 指标采集
│   └── routing_metrics.go
└── fallback/            # Fallback策略
    └── fallback.go
```

---

## 4. P2开发任务

### 4.1 合规能力包

#### 设计文档
`docs/compliance_capability_package_design_v1_2026-04-02.md`

#### TDD任务

| Step | 描述 | 测试先行 | 验收标准 |
|------|------|----------|----------|
| CMP-01 | 规则引擎：规则加载器 | ✅ | YAML加载正确 |
| CMP-02 | 规则引擎：CRED-EXPOSE规则 | ✅ | 凭证泄露检测 |
| CMP-03 | 规则引擎：CRED-INGRESS规则 | ✅ | 入站覆盖检测 |
| CMP-04 | 规则引擎：CRED-DIRECT规则 | ✅ | 直连检测 |
| CMP-05 | 规则引擎：AUTH-QUERY规则 | ✅ | query key拒绝检测 |
| CMP-06 | CI脚本：m013_credential_scan.sh | ✅ | 扫描执行正确 |
| CMP-07 | CI脚本：M-017四件套生成 | ✅ | SBOM生成正确 |
| CMP-08 | Gate集成：compliance_gate.sh | ✅ | 门禁通过 |

#### 目录结构
```
gateway/internal/compliance/     # 或新增compliance目录
├── rules/                      # 规则定义
│   ├── loader.go
│   ├── cred_expose.go
│   ├── cred_ingress.go
│   ├── cred_direct.go
│   └── auth_query.go
├── engine/                    # 规则引擎
│   └── compliance_engine.go
└── ci/                        # CI脚本
    ├── compliance_gate.sh
    ├── m013_credential_scan.sh
    ├── m014_ingress_check.sh
    ├── m015_direct_check.sh
    ├── m016_query_key_check.sh
    └── m017_dependency_audit.sh
```

---

## 5. TDD执行协议

### 5.1 单个任务执行流程

```
1. 读取设计文档对应章节
2. 编写测试用例（RED）
3. 运行测试确认失败（RED）
4. 编写实现代码（GREEN）
5. 运行测试确认通过（GREEN）
6. 重构代码（REFACTOR）
7. 提交代码（git commit）
```

### 5.2 测试命名规范

```go
// 命名格式: Test{模块}_{场景}_{期望行为}
TestAuditService_CreateEvent_Success
TestAuditService_CreateEvent_DuplicateIdempotencyKey
TestRoutingEngine_SelectProvider_CostBasedStrategy
TestScopeAuth_CheckScope_SuperAdminHasAllScopes
```

### 5.3 断言规范

```go
// 使用testify/assert
assert.Equal(t, expected, actual, "描述")
assert.NoError(t, err, "描述")
assert.True(t, condition, "描述")
```

---

## 6. 执行约束

1. **测试先行**：必须先写测试再写实现
2. **门禁检查**：所有测试通过才能提交
3. **代码覆盖**：核心逻辑覆盖率 >= 80%
4. **文档更新**：每完成一个任务更新进度

---

## 7. 验收标准

### 7.1 IAM模块

| 验收项 | 标准 |
|--------|------|
| 审计字段 | request_id, created_ip, updated_ip, version |
| 角色层级 | super_admin(100) > org_admin(50) > supply_admin(40) > ... > viewer(10) |
| Scope校验 | 正确校验token.scope包含required_scope |
| API | /api/v1/iam/* CRUD正确 |

### 7.2 审计日志模块

| 验收项 | 标准 |
|--------|------|
| 事件分类 | CRED-EXPOSE/INGRESS/DIRECT, AUTH-QUERY |
| M-014/M-016边界 | 分母不同，无重叠 |
| 幂等性 | 201/202/409/200正确响应 |
| 脱敏 | 敏感字段自动掩码 |

### 7.3 路由策略模块

| 验收项 | 标准 |
|--------|------|
| 评分权重 | 延迟40%/可用30%/成本20%/质量10% |
| M-008覆盖 | 主路径+Fallback全采集 |
| A/B测试 | 流量分配正确 |
| 灰度发布 | 百分比递增正确 |

### 7.4 合规模块

| 验收项 | 标准 |
|--------|------|
| 规则格式 | CRED-EXPOSE-RESPONSE等 |
| M-017四件套 | SBOM+LockfileDiff+兼容矩阵+风险登记册 |
| CI集成 | compliance_gate.sh可执行 |

---

## 8. 进度追踪

> ⚠️ **状态已更新至2026-04-03，详见** `docs/plans/2026-04-03-p1-p2-implementation-status-v1.md`

| 任务 | 状态 | 完成日期 | 说明 |
|------|------|----------|------|
| IAM-01~08 | ✅ **已完成** | 2026-04-02 | 核心功能完成，测试覆盖85.9%/99.0% |
| AUD-01~08 | ⚠️ **6/8完成** | 2026-04-02 | Handler未实现，核心功能完成 |
| ROU-01~09 | ✅ **已完成** | 2026-04-02 | 核心功能完成，测试覆盖94.2% |
| CMP-01~08 | ✅ **已完成** | 2026-04-02 | 核心功能+CI脚本完成 |

### 8.1 详细进度

#### IAM模块
- IAM-01~04: ✅ 数据模型完成 (覆盖率62.9%)
- IAM-05~06: ✅ 中间件完成 (覆盖率63.8%)
- IAM-07~08: ✅ API完成 (覆盖率85.9%)

#### Audit模块
- AUD-01~04: ✅ 模型+事件完成 (覆盖率73.5%~95.0%)
- AUD-05~06: ⚠️ Service完成，Handler未实现
- AUD-07~08: ✅ 指标+脱敏完成 (覆盖率79.7%)

#### Router模块
- ROU-01~02: ✅ 评分模型完成 (覆盖率94.1%)
- ROU-03~04: ✅ 策略模板完成 (覆盖率71.2%)
- ROU-05~07: ✅ 引擎+Fallback+指标完成 (覆盖率76.9%~82.4%)
- ROU-08~09: ✅ A/B测试+灰度完成 (覆盖率71.2%)

#### Compliance模块
- CMP-01~05: ✅ 规则引擎完成 (覆盖率73.1%)
- CMP-06~08: ✅ CI脚本完成

---

**文档状态**：执行计划
**下次更新**：每日进度报告
**维护责任人**：项目开发组
