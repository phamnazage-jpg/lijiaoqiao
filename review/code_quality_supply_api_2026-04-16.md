# Supply-API 代码质量与安全审查报告

**项目路径:** `/home/long/project/立交桥/supply-api/`
**审查日期:** 2026-04-16
**审查范围:** 代码质量、安全性、测试覆盖率、数据库架构

---

## 一、执行摘要

Supply-API 是一个基于 Go 的供应商结算微服务，提供账户管理、套餐管理、结算提现等功能。整体代码结构清晰，采用分层架构，但存在若干安全隐患和测试覆盖率不足的问题。

| 维度 | 评级 | 说明 |
|------|------|------|
| 代码质量 | 中等 | 架构合理，但测试覆盖率不足 |
| 安全性 | 中等 | 存在潜在密钥派生和幂等性设计问题 |
| 测试覆盖 | 不足 | 整体覆盖率 ~50%，部分模块为 0% |
| 数据库设计 | 良好 | 合理使用锁机制和分区策略 |

---

## 二、安全审查

### 2.1 [中风险] KMS 密钥派生算法安全性不足

**文件:** `internal/security/kms_service.go`

```go
func (s *kmsService) deriveDEK(masterKey, context string) ([]byte, error) {
    // P0-SEC-003修复: 添加盐值防止彩虹表攻击
    // 问题: 使用固定盐 "supply-api-dek-salt-v1"
    key := masterKey + "supply-api-dek-salt-v1" + context
    hash := sha256.Sum256([]byte(key))
    return hash[:], nil
}
```

**问题分析:**
- 使用 SHA-256 简单级联后哈希，密钥派生强度不足
- 固定盐值降低了攻击难度
- 建议: 使用 PBKDF2/Argon2 等专业密钥派生算法

**建议修复:**
```go
func (s *kmsService) deriveDEK(masterKey, context string) ([]byte, error) {
    // 使用 HKDF-SHA256 实现密钥派生
    salt := []byte(context)
    info := []byte("supply-api-dek-v1")
    return hkdf(sha256.New, []byte(masterKey), salt, info, 32)
}
```

---

### 2.2 [中风险] JWT 默认算法回退到 HS256

**文件:** `internal/middleware/auth.go` 第 501 行

```go
if alg == "" {
    alg = "HS256" // 默认算法
}
```

**问题:** 空算法时默认使用对称签名算法 HS256，若配置错误可能导致签名验证绕过。

**缓解因素:** 代码中有注释说明"默认 HS256"，表明这是有意设计，但应确保配置管理严格。

---

### 2.3 [低风险] 幂等性中间件 Payload 哈希碰撞风险

**文件:** `internal/middleware/idempotency.go`

幂等记录使用 SHA-256 哈希校验 payload 相同性，存在理论碰撞风险（实际可忽略）。更大的问题是：

1. **幂等键长度验证** (16-128 字符) 可能过于宽松，建议增加熵要求
2. **处理中超时重试** 依赖 TTL 配置，需确保合理设置

---

### 2.4 [低风险] 默认 SMS 验证器安全设计

**文件:** `internal/domain/sms_verifier_sec_test.go`

`DefaultSMSVerifier` 正确拒绝所有验证码（无真实 SMS 服务时），这是安全设计。但存在测试依赖实际 SMS 实现的潜在问题。

---

### 2.5 [低风险] 审计日志缺失源 IP 收集

**问题:** `AuditEvent` 结构中的 `SourceIP` 字段在部分代码路径中可能为空，未始终记录客户端真实 IP。

**检查代码:**
- `audit/model/event.go` 定义了 `SourceIP` 字段
- HTTP handler 应使用 `GetClientIP()` 提取真实 IP

---

### 2.6 [低风险] SQL 注入防护

**检查结果:** 未发现明显 SQL 注入风险。所有查询使用参数化查询（pgx 驱动），但需注意：

- `supply_core_schema_v2.sql` 中 `request_id` 字段为 VARCHAR(100)，需确保外部输入长度限制

---

### 2.7 [良好实践] 身份验证与授权

- JWT 支持多种算法（HS256/HS384/HS512, RS256/RS384/RS512）
- Token 缓存机制防止暴力破解
- 账户状态机强制执行（pending → active → suspended/disabled）
- 活跃账户不可删除（INV-ACC-001）

---

### 2.8 [良好实践] 数据脱敏

**文件:** `internal/audit/sanitizer/sanitizer.go`

Credential Scanner 正确实现，支持检测：
- OpenAI API Key (`sk-[a-zA-Z0-9]{20,}`)
- AWS Access Key ID (`AKIA[0-9A-Z]{16}`)
- 私钥文件（CRITICAL）
- 通用 API Key

---

## 三、代码质量审查

### 3.1 测试覆盖率

| 模块 | 覆盖率 | 目标 | 状态 |
|------|--------|------|------|
| cmd/supply-api | 9.3% | 50% | 不达标 |
| internal/adapter | 0.0% | 50% | 不达标 |
| internal/app | 51.6% | 70% | 接近 |
| internal/audit/events | 97.6% | 80% | 达标 |
| internal/audit/handler | 79.6% | 80% | 接近 |
| internal/audit/model | 93.8% | 80% | 达标 |
| internal/audit/repository | 1.5% | 30% | 不达标 |
| internal/audit/sanitizer | 84.3% | 80% | 达标 |
| internal/audit/service | 83.5% | 80% | 达标 |
| internal/cache | 50.0% | 60% | 接近 |
| internal/compensation | 51.6% | 60% | 接近 |
| internal/config | 86.4% | 80% | 达标 |
| internal/domain | 56.7% | 70% | 不达标 |
| internal/httpapi | 60.5% | 70% | 接近 |
| internal/iam | 93.2% | 80% | 达标 |
| internal/middleware | 68.9% | 70% | 接近 |
| internal/repository | 3.1% | 30% | 不达标 |
| internal/security | 88.8% | 80% | 达标 |
| internal/sms | 44.9% | 60% | 接近 |

**关键问题:**
1. `internal/adapter` 覆盖率 0% — 需要单元测试
2. `internal/repository` 覆盖率 3.1% — 严重不足，需要集成测试
3. `internal/domain` 覆盖率 56.7% — 需要补充关键业务逻辑测试

---

### 3.2 领域模型设计

**文件:** `internal/domain/`

**优点:**
- 清晰的状态机定义（账户、套餐、结算单）
- 不变量检查器（InvariantChecker）强制业务规则
- 审计日志统一发射点

**问题:**
- `domain/invariants.go` 中 `CheckPackagePrice` 硬编码最小价格 `0.01`，应从配置读取
- 状态转换验证函数返回 bool 而非 error，信息丢失

---

### 3.3 命名一致性

| 问题类型 | 发现位置 | 说明 |
|----------|----------|------|
| IP 字段命名不一致 | `AuditEvent.SourceIP` vs `audit/model` | 部分使用 `ClientIP` |
| 账户 ID 字段混淆 | `Package.AccountID` | JSON tag 为 `account_id` 但语义为 supply_account_id |

---

## 四、数据库架构审查

### 4.1 Schema 设计

**文件:** `sql/postgresql/supply_core_schema_v2.sql`

**优点:**
- 乐观锁实现（`version` 字段）
- 唯一约束防止重复 (`supply_accounts`, `supply_packages`)
- 部分索引支持幂等查询
- INET 类型存储 IP 地址
- JSONB 字段支持灵活扩展

**问题:**
1. **外键缺失:** 表之间无外键约束，依赖应用层验证（P0-09 修复引入 `ForeignKeyValidator`）
2. **索引优化空间:** `supply_settlements` 的 `user_id + status` 复合索引可进一步优化

---

### 4.2 审计事件分区策略

**文件:** `sql/postgresql/audit_events_migration_v1_to_v2.sql`

- 按月分区设计合理，支持数据保留策略
- 12 个月历史 + 3 个月未来的分区预创建

**问题:**
- 迁移脚本直接 `DROP TABLE`，虽有备份但生产环境风险高
- 建议: 增加条件判断和数据量检查

---

## 五、并发控制

### 5.1 锁策略分析

**文件:** `internal/repository/settlement.go`

| 方法 | 锁类型 | 适用场景 |
|------|--------|----------|
| `GetForUpdate` | 悲观锁 (FOR UPDATE) | 低并发、数据一致性要求高 |
| `GetForUpdateNoWait` | 悲观锁 NOWAIT | 高并发、不等待锁 |
| `GetProcessing` | FOR UPDATE SKIP LOCKED | 确保单一 processing 记录 |
| `Update` | 乐观锁 (version) | 大多数场景推荐 |

**评估:** 锁策略选择合理，满足不同并发场景需求。

---

## 六、问题汇总

### 6.1 安全性问题

| ID | 严重程度 | 位置 | 问题描述 |
|----|----------|------|----------|
| SEC-001 | 中 | kms_service.go | 密钥派生算法强度不足 |
| SEC-002 | 中 | auth.go | JWT 默认算法回退 |
| SEC-003 | 低 | idempotency.go | 幂等键熵要求缺失 |

### 6.2 代码质量问题

| ID | 严重程度 | 位置 | 问题描述 |
|----|----------|------|----------|
| QUAL-001 | 高 | internal/adapter | 测试覆盖率 0% |
| QUAL-002 | 高 | internal/repository | 测试覆盖率 3.1% |
| QUAL-003 | 中 | internal/domain | 覆盖率 56.7%，低于 70% 目标 |
| QUAL-004 | 低 | invariants.go | 硬编码最小价格值 |
| QUAL-005 | 低 | 多处 | 命名不一致（IP 字段） |

---

## 七、建议修复优先级

### P0（紧急）
1. **KMS 密钥派生算法升级** — 当前实现存在被暴力破解风险
2. **Adapter 模块测试覆盖** — 0% 覆盖率可能导致生产问题

### P1（重要）
1. **Repository 集成测试补充** — 3.1% 覆盖率过低
2. **Domain 层测试增强** — 补充不变量和状态转换测试

### P2（一般）
1. 幂等键熵验证要求
2. 配置化最小价格阈值
3. IP 字段命名统一

---

## 八、结论

Supply-API 项目整体架构设计合理，安全性措施部分到位（Credential Scanner、审计日志、幂等中间件），但密钥派生算法和测试覆盖率需要改进。建议优先处理 P0 级别问题后，逐步完善测试体系。

---

*报告生成时间: 2026-04-16*
*审查工具: go test -cover ./...*
