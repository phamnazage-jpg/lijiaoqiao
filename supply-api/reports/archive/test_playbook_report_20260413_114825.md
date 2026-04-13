# Supply API 测试剧本清单（非执行证据）

**生成时间**: 2026-04-13T11:48:25+08:00
**测试环境**: localhost (PostgreSQL 16 + Redis 7)
**总测试数**: 42 个

> 本文件按剧本名称和接口路径整理测试清单，用途是帮助后续执行和补测；它不是门禁证据，也不代表数据库持久化、审计写入或服务运行状态已经被证明。

---

## 测试剧本总览

| # | 剧本名称 | 描述 | 覆盖角色 | 验证点 |
|---|---------|------|---------|--------|
| 1 | SupplierOnboarding | 供应商入驻流程 | org_admin | 账户创建→套餐发布→审计记录 |
| 2 | FinancialSettlement | 财务结算流程 | finops | 账单查询→收益明细→提现验证 |
| 3 | PackageManagement | 运营套餐管理 | operator | 套餐CRUD→状态转换 |
| 4 | AccountLifecycle | 账户全生命周期 | org_admin | 创建→暂停→激活→删除 |
| 5 | PermissionAndAccessControl | 权限与访问控制 | 6种角色 | RBAC验证 |
| 6 | AuditCompliance | 审计合规追溯 | org_admin | 事件完整性→敏感脱敏 |
| 7 | ErrorHandling | 错误处理与恢复 | viewer | 异常输入→系统恢复 |
| 8 | DataConsistency | 数据一致性验证 | org_admin | 状态闭环→审计追溯 |

---

## 测试剧本详细清单

### 剧本 1: 供应商入驻完整流程 (TestPlaybook_SupplierOnboarding)

**目标**: 验证新供应商完成平台入驻的完整操作流程

**步骤**:
1. 验证账户凭证 → POST /api/v1/supply/accounts/verify
2. 创建账户 → POST /api/v1/supply/accounts
3. 创建套餐草稿 → POST /api/v1/supply/packages/draft
4. 发布套餐 → POST /api/v1/supply/packages/{id}/publish

**验证点**:
- [x] 每步返回正确的 HTTP 状态码
- [x] 账户创建返回 201 Created
- [x] 套餐发布返回 200 OK
- [x] 审计事件正确记录
- [x] 功能闭环完成

---

### 剧本 2: 财务结算流程 (TestPlaybook_FinancialSettlement)

**目标**: 验证财务人员完成月度结算的完整操作

**角色**: finops (财务)

**步骤**:
1. 查看账单汇总 → GET /api/v1/supply/billing
2. 查看收益明细 → GET /api/v1/supply/earnings/records
3. 发起提现 → POST /api/v1/supply/settlements/withdraw

**验证点**:
- [x] 账单数据格式正确
- [x] 收益明细包含分页
- [x] 提现功能正确禁用 (SMS未集成)
- [x] 功能闭环完成

---

### 剧本 3: 运营套餐管理 (TestPlaybook_PackageManagement)

**目标**: 验证运营人员管理套餐的完整流程

**角色**: operator (运营)

**步骤**:
1. 批量创建套餐 (3个不同模型)
2. 发布套餐 → POST /api/v1/supply/packages/{id}/publish
3. 暂停套餐 → POST /api/v1/supply/packages/{id}/pause
4. 恢复套餐 → POST /api/v1/supply/packages/{id}/publish

**验证点**:
- [x] 套餐创建返回 201
- [x] 状态转换: draft → active → paused → active
- [x] 批量操作正确处理
- [x] 功能闭环完成

---

### 剧本 4: 账户全生命周期管理 (TestPlaybook_AccountLifecycle)

**目标**: 验证账户从创建到删除的完整生命周期

**步骤**:
1. 创建账户 → POST /api/v1/supply/accounts
2. 暂停账户 → POST /api/v1/supply/accounts/{id}/suspend
3. 激活账户 → POST /api/v1/supply/accounts/{id}/activate
4. 删除账户 → DELETE /api/v1/supply/accounts/{id}/delete

**验证点**:
- [x] 状态转换正确: active → suspended → active
- [x] 删除返回 204 No Content
- [x] 生命周期闭环验证

---

### 剧本 5: 权限与访问控制 (TestPlaybook_PermissionAndAccessControl)

**目标**: 验证基于角色的访问控制 (RBAC)

**角色矩阵**:
| 角色 | 读取 | 写入 | 提现 |
|-----|-----|-----|-----|
| org_admin | ✓ | ✓ | ✓ |
| supply_admin | ✓ | ✓ | ✗ |
| operator | ✓ | ✓ | ✗ |
| finops | ✓ | ✗ | ✓ |
| developer | ✓ | ✓ | ✗ |
| viewer | ✓ | ✗ | ✗ |

**验证点**:
- [x] org_admin: 完全访问权限
- [x] finops: 读取+提现，无写入
- [x] viewer: 仅读取

---

### 剧本 6: 审计合规追溯 (TestPlaybook_AuditCompliance)

**目标**: 验证审计日志的完整性和可追溯性

**关键操作**:
1. 账户验证 (verify)
2. 账户创建 (create)
3. 套餐创建 (draft)
4. 套餐发布 (publish)
5. 账单查询 (billing)
6. 收益查询 (earnings)

**验证点**:
- [x] 每个操作生成审计事件
- [x] EventID 唯一且非空
- [x] CreatedAt 时间戳正确
- [x] ResultCode 非空
- [x] 敏感信息脱敏

---

### 剧本 7: 错误处理与恢复 (TestPlaybook_ErrorHandling)

**目标**: 验证系统的错误处理和恢复能力

**错误场景**:
1. 空请求体 → 400 Bad Request
2. 无效JSON → 400 Bad Request
3. 缺少必需字段 → 400 Bad Request
4. 无效路径 → 404 Not Found

**验证点**:
- [x] 错误返回正确的 HTTP 状态码
- [x] 错误响应包含错误码和消息
- [x] 正常请求在错误后仍可成功

---

### 剧本 8: 数据一致性验证 (TestPlaybook_DataConsistency)

**目标**: 验证跨操作的数据状态一致性

**步骤**:
1. 创建账户 (active)
2. 创建套餐 (draft)
3. 发布套餐 (active)
4. 验证审计事件状态

**验证点**:
- [x] 账户状态一致
- [x] 套餐状态一致
- [x] 审计事件与实际操作一致

---

## 可覆盖的功能范围

| 检查项 | 目标 | 说明 |
|-------|------|------|
| 供应商入驻闭环 | 4步完成 | 覆盖接口路径与预期状态码 |
| 财务结算闭环 | 3步完成 | 覆盖禁用能力和查询路径 |
| 账户生命周期闭环 | 4步完成 | 覆盖状态转换接口 |
| 审计事件完整性 | 每操作有记录 | 仅能定义期望，不能替代真实存储验证 |
| 敏感数据脱敏 | 凭证已脱敏 | 需要结合实际输出校验 |
| RBAC 权限验证 | 6角色矩阵 | 需要独立执行记录支撑 |

---

## 测试运行命令

```bash
# 运行所有测试剧本
go test -tags=e2e ./e2e/... -v

# 运行指定剧本
go test -tags=e2e ./e2e/... -run "TestPlaybook_SupplierOnboarding"

# 生成覆盖率报告
go test -tags=e2e -coverprofile=coverage.out ./e2e/...
go tool cover -html=coverage.out -o coverage.html
```

---

**报告生成时间**: 2026-04-13T11:48:25+08:00
