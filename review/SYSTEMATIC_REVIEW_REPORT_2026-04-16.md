# 系统性全面审查报告

**项目名称:** 立交桥（LJM Platform）  
**项目路径:** `/home/long/project/立交桥/`  
**审查日期:** 2026-04-16  
**审查范围:** supply-api / gateway / platform-token-runtime / SQL Schema / Python 组件  
**审查方法:** 静态代码分析 + 运行时测试验证 + 安全扫描 + 覆盖率测试  

---

## 一、执行摘要

本项目为 **LLM 网关 + 供应链结算平台**，包含三个 Go 微服务和一个辅助 Python 工具集。

| 服务 | 代码规模 | 测试状态 | 安全评级 | 生产就绪 |
|------|---------|---------|---------|---------|
| **supply-api** | ~300 Go 文件 | ⚠️ 部分覆盖不足 | 🟡 中等 | 需修复 KMS/JWT 问题 |
| **gateway** | ~100 Go 文件 | ✅ 17 包全通过 | 🔴 严重 | 需修复硬编码密钥/CORS |
| **platform-token-runtime** | ~30 Go 文件 | ✅ 7 包全通过 | 🟡 中等 | 需修复并发/刷新 bug |
| **SQL Schema** | 10+ 迁移文件 | N/A | 🟢 良好 | 需统一 audit_events 定义 |
| **Python 组件** | ~1692 文件 | ⚠️ 少量测试 | 🟢 良好 | 无重大风险 |

**整体结论:** 🔴 **暂不可投入生产** — 存在 2 个严重安全缺陷和 2 个关键数据完整性 bug，必须修复后才能上线。

---

## 二、严重问题（Must Fix — 阻塞上线）

### 🔴 2.1 gateway: 硬编码加密密钥回退

**文件:** `gateway/internal/config/config.go:18`

```go
encryptionKey = []byte(getEnv("PASSWORD_ENCRYPTION_KEY",
    "default-key-32-bytes-long!!!!!!!"))
```

**风险:** 生产环境若 `PASSWORD_ENCRYPTION_KEY` 未设置，所有密码将使用不安全默认值加密，可被轻易解密。

**修复建议:**
```go
// 非 dev 环境必须显式设置密钥
if env == "dev" || env == "test" {
    encryptionKey = []byte(getEnv("PASSWORD_ENCRYPTION_KEY", "default-dev-key..."))
} else {
    key := os.Getenv("PASSWORD_ENCRYPTION_KEY")
    if key == "" {
        log.Fatal("PASSWORD_ENCRYPTION_KEY must be set in production")
    }
    encryptionKey = []byte(key)
}
```

---

### 🔴 2.2 gateway: CORS 允许任意来源

**文件:** `gateway/internal/middleware/cors.go:23`

```go
AllowedOrigins: []string{"*"},
```

**风险:** 允许任何来源的跨域请求，API 极易受到 CSRF 攻击。

**修复建议:** 生产环境必须配置具体的允许域名列表。

---

### 🔴 2.3 platform-token-runtime: Refresh TTL 不持久化

**文件:** `platform-token-runtime/internal/auth/service/inmemory_runtime.go:128-147`

```go
func (r *InMemoryTokenRuntime) Refresh(token string) (*TokenRecord, error) {
    record, err := r.store.Get(token)
    if err != nil {
        return nil, err
    }
    record.ExpiresAt = time.Now().Add(r.ttl)  // ❌ 修改了内存中的 ExpiresAt
    // ❌ 但从未调用 r.store.Save(record) 保存回去
    return record, nil
}
```

**风险:** 服务重启后所有刷新的 token 恢复原始过期时间，等效于刷新操作被丢弃。

---

### 🔴 2.4 platform-token-runtime: 并发写 Map 非线程安全

**文件:** `platform-token-runtime/internal/auth/service/runtime_store.go:17-27`

```go
func (s *RuntimeStore) Save(token string, record *TokenRecord) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.data[token] = record  // ❌ 这行在 Lock 之外，实际写操作未受保护
    return nil
}
```

**风险:** 并发调用 `Save` 时可能发生数据竞争（data race），导致 map 损坏或数据丢失。

---

### 🔴 2.5 platform-token-runtime: audit-events 接口无鉴权

**文件:** `platform-token-runtime/internal/httpapi/token_api.go:326-374`

**风险:** `/v1/audit-events` 端点任何人可查询，无需任何认证。

---

## 三、高风险问题（Should Fix — 上线前强烈建议）

### 🟠 3.1 supply-api: KMS 密钥派生使用 SHA-256+固定盐

**文件:** `supply-api/internal/security/kms_service.go`

```go
key := masterKey + "supply-api-dek-salt-v1" + context
hash := sha256.Sum256([]byte(key))
```

**风险:** SHA-256 不适合密钥派生，固定盐降低攻击难度。建议使用 HKDF-SHA256 或 Argon2。

---

### 🟠 3.2 supply-api: JWT 回退到 HS256

**文件:** `supply-api/internal/middleware/auth.go:501`

```go
if alg == "" {
    alg = "HS256"  // 回退到对称算法
}
```

**风险:** 算法为空时默认使用 HS256，若误将 HMAC 密钥配置为 RSA 公钥会直接解密成功。

---

### 🟠 3.3 supply-api: internal/adapter 覆盖率 0%

**文件:** `supply-api/internal/adapter/`

**风险:** 外部 API 适配层完全无测试，API 行为变更无法被测试捕获。

---

### 🟠 3.4 supply-api: internal/repository 覆盖率仅 3.1%

**文件:** `supply-api/internal/repository/`

**风险:** 数据访问层几乎没有测试，SQL 查询错误难以在开发阶段发现。

---

### 🟠 3.5 gateway: TrustedProxies 未配置

**文件:** `gateway/internal/config/config.go`

**风险:** 反向代理环境下 `RemoteAddr` 直接使用，`X-Real-IP` / `X-Forwarded-For` 未解析，导致 IP 限流和审计日志中 IP 来源不可靠。

---

### 🟠 3.6 gateway: 请求 ID 信任用户输入

**文件:** `gateway/internal/handler/handler.go:61`

```go
requestID := r.Header.Get("X-Request-ID")  // 直接信任用户输入
```

**风险:** 日志注入攻击。

---

### 🟠 3.7 gateway: 内部错误信息泄漏

**文件:** `gateway/internal/handler/handler.go:77-78`

```go
err.Error()  // 暴露给客户端
```

**风险:** 数据库连接错误等内部细节泄漏给用户。

---

## 四、中等风险问题

| ID | 服务 | 问题 | 文件位置 |
|----|------|------|---------|
| MED-01 | supply-api | `internal/domain` 覆盖率 56.7%（目标 70%） | `internal/domain/` |
| MED-02 | gateway | 弱随机数源（time.Now().UnixNano()）用于负载均衡 | `internal/router/router.go:16` |
| MED-03 | gateway | 缺少安全响应头（X-Content-Type-Options 等） | 全局 |
| MED-04 | token-runtime | 弱随机数种子用于 token 生成 | `internal/token/token.go` |
| MED-05 | SQL | `audit_events` Schema 在 3 个文件中定义不一致 | `sql/` 多个文件 |
| MED-06 | Python | `supply_gateway_mock_server.py` 路径解析未验证数组边界 | `scripts/mock/` |
| MED-07 | Python | `sandbox_executor.py` 动态 pip install 存在注入风险 | `llm-gateway-competitors/` |
| MED-08 | supply-api | 幂等中间件覆盖的字段（id/key/signal）未在 CLAUDE.md 中声明 | `internal/middleware/idempotency.go` |

---

## 五、低风险问题（可选优化）

| ID | 问题 | 建议 |
|----|------|------|
| LOW-01 | gateway router 中硬编码 "primary" 提供商名 | 改为配置驱动 |
| LOW-02 | Python mock 服务器缺少类型注解 | 添加 `typing.TYPE_CHECKING` |
| LOW-03 | SQL 迁移中 `partition_strategy_v1.sql` 使用非事务性 `DROP TABLE` | 改为 `DROP TABLE IF EXISTS` |
| LOW-04 | gateway 适配器超时硬编码 | 移至配置文件 |
| LOW-05 | Python 审计工具 `check_pnpm_audit_exceptions.py` 缺少单元测试 | 添加 pytest 测试 |

---

## 六、跨服务设计问题

### 6.1 audit_events Schema 不一致

三个服务分别定义了各自的 `audit_events` 表结构：

| 服务 | 文件 | 字段定义 |
|------|------|---------|
| supply-api | `sql/postgresql/supply_core_schema_v2.sql` | 完整字段集 |
| gateway | `llm-gateway-competitors/sub2api-tar/backend/migrations/` | 74 个迁移文件 |
| token-runtime | `sql/postgresql/token_runtime_schema_v1.sql` | 独立定义 |

**建议:** 建立统一的 audit schema 标准，所有服务遵循同一份 DDL。

---

### 6.2 字段命名不一致风险

根据 CLAUDE.md 规范，`SourceIP` 为标准命名，但需在所有服务中验证一致性。

---

## 七、测试覆盖率详情

### 7.1 supply-api

| 模块 | 覆盖率 | 目标 | 状态 |
|------|-------|------|------|
| internal/domain | 56.7% | 70% | ❌ |
| internal/audit/service | 通过 | 80% | ⚠️ |
| internal/audit/handler | 通过 | 75% | ⚠️ |
| internal/security | 通过 | 80% | ⚠️ |
| internal/middleware | 通过 | 80% | ⚠️ |
| internal/iam | 通过 | 70% | ⚠️ |
| internal/adapter | **0%** | — | ❌❌ |
| internal/repository | **3.1%** | — | ❌❌ |

### 7.2 gateway

**17 个包全部通过测试**，但部分模块仅有 smoke test，无深度覆盖。

### 7.3 platform-token-runtime

**7 个包全部通过测试**，`go vet ./...` 和 `go build ./...` 均无问题。

---

## 八、数据库设计评估

| 维度 | 评级 | 说明 |
|------|------|------|
| 索引覆盖 | 🟢 良好 | 有效使用 partial indexes |
| 约束完整性 | 🟡 中等 | `supply_core_schema_v2.sql` 缺少 status CHECK 约束（v1 有） |
| 迁移安全性 | 🟡 中等 | partition_strategy 使用非事务性 DROP TABLE |
| 敏感数据加密 | 🟢 良好 | credential 字段使用 AES-GCM 加密 |
| 锁策略 | 🟢 良好 | 正确使用乐观锁/悲观锁 |

---

## 九、安全扫描结果

```
✅ supply-api go.mod: 依赖正常，无已知 CVE
✅ platform-token-runtime: 仅使用 stdlib，攻击面极小
⚠️  gateway: 依赖较少，但需关注配置密钥问题
⚠️  Python: 无 requirements.txt，工具脚本临时使用外部包
```

---

## 十、修复优先级

### P0（必须修复，阻塞上线）

| 优先级 | 问题 | 服务 | 预计工时 |
|--------|------|------|---------|
| P0-1 | 硬编码加密密钥回退 | gateway | 0.5h |
| P0-2 | CORS 任意来源 | gateway | 0.5h |
| P0-3 | Refresh TTL 不持久化 | token-runtime | 1h |
| P0-4 | 并发写 Map 非线程安全 | token-runtime | 1h |
| P0-5 | audit-events 无鉴权 | token-runtime | 1h |

### P1（强烈建议，上线前完成）

| 优先级 | 问题 | 服务 | 预计工时 |
|--------|------|------|---------|
| P1-1 | KMS 密钥派生算法升级 | supply-api | 2h |
| P1-2 | JWT 禁止 HS256 回退 | supply-api | 1h |
| P1-3 | adapter 层添加测试 | supply-api | 4h |
| P1-4 | repository 层覆盖率提升 | supply-api | 8h |
| P1-5 | TrustedProxies 配置 | gateway | 1h |
| P1-6 | 请求 ID 输入校验 | gateway | 0.5h |
| P1-7 | 内部错误信息脱敏 | gateway | 1h |

### P2（建议，本迭代后完成）

| 优先级 | 问题 | 服务 | 预计工时 |
|--------|------|------|---------|
| P2-1 | domain 层覆盖率提升至 70% | supply-api | 8h |
| P2-2 | audit_events Schema 统一 | 所有服务 | 4h |
| P2-3 | Python sandbox_executor 注入风险 | llm-gateway-competitors | 2h |
| P2-4 | 安全响应头添加 | gateway | 1h |

---

## 十一、总体评分

| 维度 | 得分 | 满分 | 评级 |
|------|------|------|------|
| 代码质量 | 68/100 | 100 | 🟡 中等 |
| 安全性 | 55/100 | 100 | 🟠 较差 |
| 测试覆盖 | 52/100 | 100 | 🟠 较差 |
| 架构设计 | 78/100 | 100 | 🟢 良好 |
| 数据库设计 | 72/100 | 100 | 🟢 良好 |
| **综合** | **65/100** | **100** | **🟠 较差 — 需修复 P0 后上线** |

---

## 十二、审查清单

### 分项报告索引

| 报告 | 路径 | 大小 |
|------|------|------|
| Supply-API 详细报告 | `review/code_quality_supply_api_2026-04-16.md` | 8.9KB |
| Gateway 详细报告 | `review/code_quality_gateway_2026-04-16.md` | 13.2KB |
| Token-Runtime 详细报告 | `review/code_quality_token_runtime_2026-04-16.md` | 15.5KB |
| SQL Schema 详细报告 | `review/code_quality_sql_2026-04-16.md` | ~12KB |
| Python 组件详细报告 | `review/code_quality_python_2026-04-16.md` | ~4KB |

### 审查方法

- **静态分析:** `go vet`, `go build`, `go test -short`
- **安全扫描:** 代码模式扫描（硬编码凭证、不安全函数调用）
- **覆盖率测试:** `go test -cover`
- **Schema 审查:** DDL 逐文件分析
- **依赖审查:** `go.mod` / `requirements*.txt` 核查

---

*本报告由 Hermes Agent 自动生成，审查时间 2026-04-16。子报告位于 `review/code_quality_*_2026-04-16.md`。*
