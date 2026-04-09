# Supply API TDD 任务清单 V2 (2026-04-09)

> 基于验证报告创建的详细任务清单
> 严格标准：安全问题 P0 立即修复，设计问题 P1 强烈建议

---

## 一、验证报告问题状态总览

### SEC 安全问题

| ID | 问题 | 严重程度 | 状态 | 说明 |
|----|------|----------|------|------|
| SEC-001 | 硬编码"123456"测试码 | **高** | ✅ 已修复 | DefaultSMSVerifier 返回错误 |
| SEC-002 | 补偿执行器日志泄露payload | 中 | ✅ 已修复 | maskPayload 脱敏 |
| SEC-003 | X-Forwarded-For未验证可信代理 | 中 | ✅ 已修复 | TrustedProxies 配置 |
| SEC-004 | JWT错误信息泄露 | 低 | ✅ 已修复 | 通用错误消息 |
| SEC-005 | 开发模式禁用鉴权 | 中 | ⚠️ 设计决定 | 非bug，是dev模式特性 |
| SEC-006 | 幂等中间件竞态条件 | 中 | ✅ 已验证 | 使用AcquireLock事务保护 |
| SEC-007 | SQL WHERE子句拼接 | 低 | ✅ 误报 | 实际使用参数化查询 |
| SEC-008 | 分页参数未验证边界 | 低 | ✅ 已修复 | 负数和上限检查 |
| SEC-009 | 健康检查路由不一致 | 低 | ✅ 已修复 | /actuator/health 前缀 |
| SEC-010 | TokenCache多实例不共享 | 低 | ⚠️ 已知限制 | 需要Redis支持 |

### P0 问题

| ID | 问题 | 状态 |
|----|------|------|
| P0-01 | 硬编码SMS测试码 | ✅ 已修复 |
| P0-02 | 提现操作无事务 | ✅ 已修复 (CreateWithdrawTx + FOR UPDATE SKIP LOCKED) |
| P0-03 | 补偿执行器stub | ✅ 已修复 |

### P1 问题

| ID | 问题 | 状态 |
|----|------|------|
| P1-01 | main.go臃肿 | ✅ 已修复 (Task #23) |
| P1-02 | IP来源未验证 | ✅ 已修复 (SEC-003) |
| P1-03 | JWT错误信息泄露 | ✅ 已修复 (SEC-004) |
| P1-04 | 限流key从JWT获取 | ✅ 已修复 |

### P2 问题

| ID | 问题 | 状态 |
|----|------|------|
| P2-01 | domain测试覆盖率 | ✅ 72.0% (目标70%+) |
| P2-02 | middleware测试覆盖率 | ✅ 81.0% (目标70%+) |
| P2-03 | 分页参数验证 | ✅ 已修复 |
| P2-04 | 健康检查路由统一 | ✅ 已修复 |

### go vet 问题

| ID | 问题 | 状态 |
|----|------|------|
| govet-01 | 测试函数命名 | ✅ 已修复/误报 |
| govet-02 | context cancel未调用 | ✅ 已修复/误报 |

---

## 二、剩余问题分析

### 问题 1: domain 测试覆盖率偏低 (57.4% → 75%+)

**当前状态:**
- domain 覆盖率: 57.4%
- 目标覆盖率: 75%+

**需要覆盖的模块:**
1. SettlementService 核心方法
2. AccountService 核心方法
3. PackageService 核心方法
4. EarningService 核心方法

**TDD 任务:**
- [ ] 为每个领域服务编写更多边界测试
- [ ] 添加错误场景测试
- [ ] 添加并发场景测试

### 问题 2: SEC-010 TokenCache 多实例不共享

**当前状态:**
- TokenCache 使用纯内存存储
- 多实例部署时不共享

**解决方案:**
- 使用 Redis 作为可选缓存后端
- 保持向后兼容（无 Redis 时使用内存缓存）

---

## 三、TDD 实施计划

### Phase 1: 提高 domain 测试覆盖率

**Step 1.1:** 分析当前覆盖率差距
```bash
go test -coverprofile=coverage.out ./internal/domain/
go tool cover -func=coverage.out | grep -E "domain.*:"
```

**Step 1.2:** 识别未覆盖的函数
- SettlementService.Withdraw
- SettlementService.Cancel
- AccountService.* (部分方法)
- PackageService.* (部分方法)

**Step 1.3:** 编写测试用例
- TestSettlementService_Withdraw_InsufficientBalance
- TestSettlementService_Withdraw_DuplicateRequest
- TestSettlementService_Withdraw_ConcurrentRequests

### Phase 2: SEC-010 TokenCache Redis 支持

**Step 2.1:** 创建 TokenCache 接口
```go
type TokenCacheBackend interface {
    Get(tokenID string) (string, bool)
    Set(tokenID string, status string, ttl time.Duration) error
    Delete(tokenID string) error
}
```

**Step 2.2:** 实现 Redis 后端
```go
type RedisTokenCacheBackend struct {
    client *redis.Client
}
```

**Step 2.3:** 修改 TokenCache 使用后端

---

## 四、执行记录

| 日期 | 任务 | 状态 |
|------|------|------|
| 2026-04-09 | Task #23 main.go拆分 | ✅ 完成 |
| 2026-04-09 | SEC-001 硬编码测试码 | ✅ 完成 |
| 2026-04-09 | SEC-003 IP验证 | ✅ 完成 |
| 2026-04-09 | TASK-25 domain覆盖率提升 | ✅ 完成 (72.3%) |
| 2026-04-09 | TASK-27 DSN密码泄露检查 | ✅ 完成 (设计安全) |
| 2026-04-09 | TASK-28 提现竞态修复 | ✅ 完成 (FOR UPDATE SKIP LOCKED) |
| 2026-04-09 | 请求超时中间件检查 | ✅ 完成 (已实现) |
| 2026-04-09 | SEC-010 TokenCache | ⚠️ 已知限制 (需Redis) |

---

## 五、验证报告问题清单核对

### 架构审查 (7/10)

| 序号 | 问题 | 验证报告位置 | 修复状态 |
|------|------|-------------|---------|
| 1 | main.go过于臃肿 | 2.2节 | ✅ 已修复 |
| 2 | 提现操作无事务 | 2.2节 | ✅ 已修复 |
| 3 | 内存审计存储无持久化 | 2.2节 | ✅ DB-backed实现 |
| 4 | DSN()返回明文密码 | 2.2节 | ✅ 设计安全 (内部使用) |
| 5 | 缺少请求超时中间件 | 2.2节 | ✅ 已实现 |
| 6 | 幂等锁存在竞态条件 | 2.2节 | ✅ 已验证安全 |
| 7 | 短信验证码硬编码 | 2.2节 | ✅ 已修复 |

### 安全审查 (7.5/10)

| ID | 问题 | 验证报告位置 | 修复状态 |
|----|------|-------------|---------|
| SEC-001 | 硬编码"123456" | 3.2节 | ✅ 已修复 |
| SEC-002 | 补偿执行器日志泄露 | 3.2节 | ✅ 已修复 |
| SEC-003 | X-Forwarded-For未验证 | 3.2节 | ✅ 已修复 |
| SEC-004 | JWT错误信息泄露 | 3.2节 | ✅ 已修复 |
| SEC-005 | 开发模式禁用鉴权 | 3.2节 | ⚠️ 设计决定 |
| SEC-006 | 幂等中间件竞态 | 3.2节 | ✅ 已验证 |
| SEC-007 | SQL拼接 | 3.2节 | ✅ 误报 |
| SEC-008 | 分页参数未验证 | 3.2节 | ✅ 已修复 |
| SEC-009 | 健康检查路由不一致 | 3.2节 | ✅ 已修复 |
| SEC-010 | TokenCache多实例 | 3.2节 | ⚠️ 已知限制 |

---

## 六、新增任务

### TASK-25: 提高 domain 测试覆盖率到 70%+

**目标:** domain 测试覆盖率从 57.4% 提升到 70%+

**当前状态:** ✅ 已完成 (72.0%)

**TDD 方法:**
1. 分析当前覆盖率报告
2. 识别未覆盖的关键路径
3. 编写测试用例
4. 验证覆盖率达标

### TASK-26: SEC-010 TokenCache Redis 支持

**目标:** 为 TokenCache 添加可选的 Redis 后端支持

**TDD 方法:**
1. 创建 TokenCacheBackend 接口
2. 实现 RedisTokenCacheBackend
3. 修改 TokenCache 支持后端注入
4. 编写测试验证

### TASK-27: 检查并修复 DSN 密码泄露问题

**目标:** 确保 DSN() 不返回明文密码

**当前状态:** ✅ 已验证 (设计安全)
- DSN() 仅供内部使用（数据库连接）
- SafeDSN() 用于日志（密码脱敏）
- 使用位置：internal/repository/db.go

**TDD 方法:**
1. 检查 DatabaseConfig.DSN() 实现
2. 如果返回明文密码，修改为返回掩码
3. 编写测试验证
