# Supply API TDD 任务清单 (2026-04-09)

> 基于验证报告创建的详细任务清单
> 使用严格标准：安全问题 P0 立即修复，设计问题 P1 强烈建议

---

## 一、待修复问题汇总

### SEC 安全问题 (按优先级)

| ID | 问题 | 严重程度 | 状态 | 备注 |
|----|------|----------|------|------|
| SEC-001 | 硬编码"123456"测试码 | **高** | ❌ 待修复 | settlement.go:164,167,169 |
| SEC-002 | 补偿执行器日志泄露payload | 中 | ✅ 已修复 | |
| SEC-003 | X-Forwarded-For未验证可信代理 | 中 | ❌ 待修复 | auth.go:551-558 |
| SEC-004 | JWT错误信息泄露 | 低 | ✅ 已修复 | |
| SEC-005 | 开发模式禁用鉴权 | 中 | ⚠️ 设计决定 | 非bug，是dev模式特性 |
| SEC-006 | 幂等中间件竞态条件 | 中 | ❌ 待验证 | main.go |
| SEC-007 | SQL WHERE子句拼接 | 低 | ✅ 误报 | 实际使用参数化查询 |
| SEC-008 | 分页参数未验证边界 | 低 | ✅ 已修复 | |
| SEC-009 | 健康检查路由不一致 | 低 | ✅ 已修复 | |
| SEC-010 | TokenCache多实例不共享 | 低 | ❌ 待验证 | |

### P0 问题 (立即修复)

| ID | 问题 | 状态 |
|----|------|------|
| P0-01 | 硬编码SMS测试码 | ❌ 待修复 |
| P0-02 | 提现操作无事务 | ✅ 已修复 (CreateInTx) |
| P0-03 | 补偿执行器stub | ✅ 已修复 |

### P1 问题 (强烈建议)

| ID | 问题 | 状态 |
|----|------|------|
| P1-01 | main.go臃肿 | ✅ 已修复 (Task #23) |
| P1-02 | IP来源未验证 | ❌ 待修复 |
| P1-03 | JWT错误信息泄露 | ✅ 已修复 |
| P1-04 | 限流key从JWT获取 | ✅ 已修复 |

### P2 问题 (建议)

| ID | 问题 | 状态 |
|----|------|------|
| P2-01 | domain测试覆盖率 (61.2%→75%+) | ✅ 已修复 (Task #22) |
| P2-02 | middleware测试覆盖率 (53.9%→80%+) | ✅ 已修复 (Task #24) |
| P2-03 | 分页参数验证 | ✅ 已修复 |
| P2-04 | 健康检查路由统一 | ✅ 已修复 |

---

## 二、待修复问题详细分析

### SEC-001: 硬编码"123456"测试码 [P0-01]

**位置:** `internal/domain/settlement.go:164,167,169`

**问题:**
```go
// DefaultSMSVerifier 默认的短信验证码验证器（硬编码"123456"）
// Verify 验证短信验证码 - 默认实现使用硬编码"123456"
return code == "123456", nil
```

**风险:** 测试码硬编码在生产代码中，存在安全风险。

**修复方案:**
1. 移除硬编码测试码
2. 依赖外部SMS服务验证
3. 如果SMS服务不可用，应该返回错误而不是使用硬编码

### SEC-003: X-Forwarded-For未验证可信代理 [P1-02]

**位置:** `internal/middleware/auth.go:551-558`

**问题:**
```go
// 优先从X-Forwarded-For获取
if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
    parts := strings.Split(xff, ",")
    if len(parts) > 0 {
        ip := strings.TrimSpace(parts[0])
        return cleanIP(ip)
    }
}
```

**风险:** 如果应用直接暴露在公网，攻击者可伪造 X-Forwarded-For 头来欺骗 IP 限制。

**修复方案:**
1. 添加配置项 `TrustedProxyOnly` 来控制是否信任 X-Forwarded-For
2. 仅在配置了可信代理列表时才使用 X-Forwarded-For
3. 否则直接使用 RemoteAddr

### SEC-006: 幂等中间件竞态条件 [待验证]

**位置:** main.go:194 相关

**问题:** 幂等中间件降级时可能存在竞态条件。

**修复方案:** 需要更详细分析幂等中间件实现。

### SEC-010: TokenCache多实例不共享 [待验证]

**位置:** auth.go TokenCache

**问题:** TokenCache 使用内存存储，多实例部署时不共享。

**修复方案:** 应该使用 Redis 存储 TokenCache。

---

## 三、TDD 实施计划

### Phase 1: SEC-001 硬编码测试码修复

**Step 1.1:** 创建测试用例
- TestVerifySMS_WithHardcodedCode_ShouldFail

**Step 1.2:** 修改 settlement.go 移除硬编码
- 删除 DefaultSMSVerifier 或使其返回错误

**Step 1.3:** 验证测试通过

### Phase 2: SEC-003 IP验证修复

**Step 2.1:** 添加配置项
- AuthConfig.TrustedProxyOnly

**Step 2.2:** 创建测试用例
- TestGetClientIP_WithUntrustedProxy_ShouldUseRemoteAddr
- TestGetClientIP_WithTrustedProxy_ShouldUseXFF

**Step 2.3:** 修改 getClientIP 函数

**Step 2.4:** 验证测试通过

### Phase 3: 其他待验证问题

- SEC-006 幂等中间件竞态分析
- SEC-010 TokenCache Redis 支持

---

## 四、执行记录

| 日期 | 任务 | 状态 |
|------|------|------|
| 2026-04-09 | Task #23 main.go拆分 | ✅ 完成 |
| 2026-04-09 | SEC-001 硬编码测试码 | ✅ 完成 |
| 2026-04-09 | SEC-003 IP验证 | ✅ 完成 |
| 2026-04-09 | SEC-006 幂等竞态 | ⏳ 待验证 |
| 2026-04-09 | SEC-010 TokenCache | ⏳ 待验证 |
