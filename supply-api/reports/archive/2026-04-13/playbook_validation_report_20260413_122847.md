# 测试剧本人工核对记录（非门禁证据）

**验证时间**: 2026-04-13T12:28:47+08:00
**测试环境**: localhost (PostgreSQL 16 + Redis 7)
**服务状态**: 待独立验证

> 本文件保留人工核对结论，但它不是原始执行证据。若无对应的 `go test` 输出、HTTP 响应记录和 SQL 查询结果，不能把其中的 `PASS` 结论当成发布门禁。

---

## 测试剧本人工核对结果

| # | 剧本名称 | 当前判断 | 备注 |
|---|---------|---------|------|
| 1 | SupplierOnboarding | ⚠️ 部分成立 | 接口路径可用，但审计写入未证实 |
| 2 | FinancialSettlement | ⚠️ 部分成立 | 查询和禁用逻辑可核对，数据库闭环未证实 |
| 3 | PackageManagement | ⚠️ 部分成立 | 状态流转需配合真实存储验证 |
| 4 | AccountLifecycle | ⚠️ 部分成立 | 生命周期接口存在，持久化未证实 |
| 5 | PermissionAndAccessControl | ⚠️ 部分成立 | 权限矩阵需要原始请求证据 |
| 6 | AuditCompliance | ❌ 未证实 | 缺少真实审计存储证据 |
| 7 | ErrorHandling | ⚠️ 部分成立 | 输入验证仍有边界缺口 |
| 8 | DataConsistency | ❌ 未证实 | 数据一致性需要真实数据库查询 |

---

## 当前确认的问题

### 问题 1: 审计事件未记录到存储 ⚠️

**严重程度**: 中

**现象**:
- 测试剧本执行后，`audit_events` 表数量为 0
- MemoryAuditStore 查询返回 0 个事件
- 审计中间件可能未正确连接到审计存储

**影响**:
- 无法进行审计合规追溯
- 安全事件无法追踪
- 操作记录不完整

**可能原因**:
1. 审计中间件未启用
2. AuditEmitterAdapter 未正确配置
3. 异步写入失败被静默忽略

**建议修复**:
```go
// 检查 AuditEmitterAdapter 配置
auditEmitter := adapter.NewAuditEmitterAdapter(auditStore)
mux.Handle("/api/v1/supply/accounts", authMiddleware.Wrap(auditEmitter.Emit(handleAccountAction)))
```

---

### 问题 2: 数据未持久化到数据库 ⚠️

**严重程度**: 高

**现象**:
- 创建账户后，`supply_accounts` 表数量仍为 0
- 创建套餐后，`supply_packages` 表数量仍为 0
- 当前归档中没有足够证据证明剧本走到了真实数据库

**影响**:
- 测试不能验证真实数据库操作
- 功能闭环无法验证数据正确性
- 无法进行端到端的数据一致性验证

**可能原因**:
1. E2E 测试使用的是内存 mock 服务
2. Repository 层未正确连接到真实数据库
3. 事务未提交或回滚

**建议修复**:
- 在 E2E 测试中使用真实 Repository
- 或增加集成测试验证数据库操作

---

### 问题 3: 输入验证不够严格 ⚠️

**严重程度**: 中

**现象**:
- 缺少 `provider` 字段时返回 201 (创建成功) 而不是 400 (参数错误)
- 错误信息 `"risk_ack is required"` 可能暴露内部实现

**当前行为**:
```
POST /api/v1/supply/accounts
Body: {"account_type":"resource","credential_input":"sk-test"}
Response: 201 Created
```

**期望行为**:
```
Response: 400 Bad Request
{"error": {"code": "SUP_HTTP_4003", "message": "missing required field: provider"}}
```

**建议修复**:
```go
// 在 handleCreateAccount 中添加验证
if req.Provider == "" {
    writeError(w, http.StatusBadRequest, CodeMissingParam, "provider is required")
    return
}
```

---

### 问题 4: 错误信息暴露内部细节 ⚠️

**严重程度**: 低

**现象**:
- 错误信息 `"risk_ack is required"` 暴露了内部字段名
- 错误信息 `"SUP_HTTP_5002"` 格式不够清晰

**期望行为**:
```
{"error": {"code": "SUP_HTTP_4003", "message": "missing required field: risk_ack"}}
```

**建议修复**:
- 统一错误码格式
- 用户友好错误信息
- 不暴露内部实现细节

---

## 功能闭环验证

### 仅能确认接口级闭环

| 流程 | 步骤 | 状态 |
|-----|------|------|
| 账户生命周期 | 创建→暂停→激活→删除 | ⚠️ 仅确认 API 响应路径 |
| 套餐生命周期 | 创建草稿→发布→暂停→恢复 | ⚠️ 仅确认 API 响应路径 |
| 结算流程 | 账单查询→收益明细→提现检查 | ⚠️ 仅确认接口与禁用逻辑 |

### 未被证实的部分

| 流程 | 问题 |
|-----|------|
| 审计追溯 | 审计事件未记录 |
| 数据持久化 | 数据库无数据 |
| 输入验证 | 缺少字段返回 201 |

---

## 下一步建议

### P0 (必须修复)

1. **启用审计存储**: 确保审计事件正确记录到 audit_events 表
2. **修复输入验证**: 缺少必需字段应返回 400
3. **补齐真实数据库验证**: 使用 SQL 查询证明创建/更新正确持久化

### P1 (应该修复)

1. **统一错误码格式**: 符合 `SUP_{DOMAIN}_{CODE}` 规范
2. **增强 E2E/集成边界**: 明确哪些场景使用真实 Repository，哪些是 mock
3. **添加数据库验证**: 在测试中验证数据正确性

### P2 (可以修复)

1. **添加性能基准**: P99 < 200ms SLA
2. **增强限流**: 当前无明确的限流响应
3. **添加健康检查详情**: /actuator/health 应包含依赖检查

---

## 测试命令

```bash
# 运行测试剧本
go test -tags=e2e ./e2e/... -v -run "TestPlaybook"

# 检查数据库状态
psql -U long -h /var/run/postgresql -d supply_api -c "SELECT * FROM supply_accounts;"
psql -U long -h /var/run/postgresql -d supply_api -c "SELECT * FROM audit_events;"

# 检查服务健康
curl http://localhost:18082/actuator/health
```

---

**报告生成时间**: 2026-04-13T12:28:47+08:00
