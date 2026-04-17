# LJM Platform 系统性修复计划

**项目:** 立交桥（LJM Platform）
**路径:** `/home/long/project/立交桥/`
**编制日期:** 2026-04-17
**依据:** SYSTEMATIC_REVIEW_REPORT + 4份专项报告 (2026-04-16)
**状态:** 🟡 部分完成 — 今日已修复 3 项，剩余 P0/P1 待处理

---

## 修复概览

| 优先级 | 数量 | 已完成 | 待修复 |
|--------|------|--------|--------|
| **P0 阻塞上线** | 6 | 1 (IP spoofing) | 5 |
| **P1 强烈建议** | 7 | 1 (BruteForce激活) | 6 |
| **P2 建议优化** | 8 | 0 | 8 |
| **总计** | **21** | **2** | **19** |

---

## P0 — 必须修复（阻塞上线）

### ✅ P0-0: IP Spoofing (platform-token-runtime) — **已完成**
- **文件:** `internal/auth/middleware/token_auth_middleware.go`, `query_key_reject_middleware.go`
- **问题:** `extractClientIP` 直接信任 `X-Forwarded-For`，任意客户端可伪造 IP
- **修复:** 参考 gateway 的安全实现，添加 `TrustedProxies []string` 参数，仅在来自可信代理时信任 XFF
- **验证:** 5 处调用点全部更新，BuildTokenAuthChain 传递 TrustedProxies

---

### P0-1: gateway 硬编码加密密钥回退 🔴
- **文件:** `gateway/internal/config/config.go:18`
- **问题:**
  ```go
  encryptionKey = []byte(getEnv("PASSWORD_ENCRYPTION_KEY",
      "default-key-32-bytes-long!!!!!!!"))
  ```
  生产环境若未设置 `PASSWORD_ENCRYPTION_KEY`，所有密码使用不安全默认值加密
- **修复:** 非 dev/test 环境必须显式设置，否则 `log.Fatal`
- **工时:** 0.5h
- **状态:** ⬜ 待修复

---

### P0-2: gateway CORS 允许任意来源 🔴
- **文件:** `gateway/internal/middleware/cors.go:23`
- **问题:** `AllowOrigins: []string{"*"}` 允许所有来源跨域请求
- **修复:** 默认改为空或 restrictive，配置驱动
- **工时:** 0.5h
- **状态:** ⬜ 待修复

---

### P0-3: token-runtime Refresh TTL 不持久化 🔴
- **文件:** `platform-token-runtime/internal/auth/service/inmemory_runtime.go:128-147`
- **问题:** `Refresh` 修改了内存中记录的 `ExpiresAt`，但从未调用 `store.Save()` 回写
- **修复:** `Refresh` 成功后调用 `r.store.Save()` 或添加 `UpdateExpiresAt` 方法
- **工时:** 1h
- **状态:** ⬜ 待修复

---

### P0-4: token-runtime 并发写 Map 非线程安全 🔴
- **文件:** `platform-token-runtime/internal/auth/service/runtime_store.go:17-27`
- **问题:** `Save` 方法的 map 写操作在 `s.mu.Lock()` 之外
- **修复:** 将 map 写操作纳入 mutex 保护范围
- **工时:** 1h
- **状态:** ⬜ 待修复

---

### P0-5: token-runtime audit-events 无鉴权 🔴
- **文件:** `platform-token-runtime/internal/httpapi/token_api.go:326-374`
- **问题:** `/v1/audit-events` 端点无需任何认证即可查询
- **修复:** 要求 bearer token 鉴权，或限制内网访问
- **工时:** 1h
- **状态:** ⬜ 待修复

---

## P1 — 强烈建议（上线前完成）

### ✅ P1-0: BruteForceProtection 激活 (supply-api) — **已完成**
- **文件:** `internal/middleware/auth.go`, `internal/app/runtime.go`
- **问题:** `BruteForceProtection` 结构体已定义但从未初始化，`bruteForce` 字段始终为 nil
- **修复:** 在 `AuthConfig` 添加 `BruteForceMaxAttempts` 和 `BruteForceLockoutDuration` 字段，
  `runtime.go` 中初始化时设置默认值（5次/15分钟锁定）
- **工时:** 0.5h
- **状态:** ✅ 完成

---

### P1-1: supply-api KMS 密钥派生算法升级 🟠
- **文件:** `supply-api/internal/security/kms_service.go`
- **问题:** 使用 `SHA-256(concat)` 简单哈希作为密钥派生，固定盐值降低攻击难度
- **修复:** 改用 `crypto/hkdf` + SHA-256，实现 proper KDF
- **工时:** 2h
- **状态:** ⬜ 待修复

---

### P1-2: supply-api JWT 禁止 HS256 回退 🟠
- **文件:** `supply-api/internal/middleware/auth.go:501`
- **问题:** `alg == ""` 时回退到 HS256，配置错误可能导致签名验证绕过
- **修复:** 非 dev 环境禁止空算法回退，要求显式配置
- **工时:** 1h
- **状态:** ⬜ 待修复

---

### P1-3: supply-api adapter 层添加测试 🟠
- **文件:** `supply-api/internal/adapter/`
- **问题:** 测试覆盖率 0%，API 适配层完全无测试
- **修复:** 补充外部 API 适配层的 mock 测试
- **工时:** 4h
- **状态:** ⬜ 待修复

---

### P1-4: supply-api repository 层覆盖率提升 🟠
- **文件:** `supply-api/internal/repository/`
- **问题:** 覆盖率仅 3.1%，SQL 查询错误难以在开发阶段发现
- **修复:** 补充 repository 集成测试（可用 sqlmock）
- **工时:** 8h
- **状态:** ⬜ 待修复

---

### P1-5: gateway TrustedProxies 配置 🟠
- **文件:** `gateway/internal/config/config.go`, `internal/app/bootstrap.go`
- **问题:** `TrustedProxies` 从未设置，反向代理环境下 `extractClientIP` 始终用 RemoteAddr
- **修复:** 添加配置文件选项，在 bootstrap 时传入 auth chain
- **工时:** 1h
- **状态:** ⬜ 待修复

---

### P1-6: gateway 请求 ID 信任用户输入 🟠
- **文件:** `gateway/internal/handler/handler.go:61`
- **问题:** `requestID := r.Header.Get("X-Request-ID")` 直接信任用户输入，存在日志注入风险
- **修复:** 对用户提供的 request ID 做长度/字符校验或直接重新生成
- **工时:** 0.5h
- **状态:** ⬜ 待修复

---

### P1-7: gateway 内部错误信息泄漏 🟠
- **文件:** `gateway/internal/handler/handler.go:77-78`
- **问题:** `err.Error()` 直接暴露给客户端，内部细节泄漏
- **修复:** 通用错误消息，日志记录详细错误
- **工时:** 1h
- **状态:** ⬜ 待修复

---

## P2 — 建议优化（本次迭代后完成）

| ID | 服务 | 问题 | 文件 | 工时 |
|----|------|------|------|------|
| P2-1 | supply-api | domain 层覆盖率 56.7% → 70% | internal/domain/ | 8h |
| P2-2 | 全局 | audit_events Schema 在 3 服务中定义不一致 | sql/ | 4h |
| P2-3 | Python | sandbox_executor pip install 注入风险 | llm-gateway-competitors/ | 2h |
| P2-4 | gateway | 缺少安全响应头 (X-Content-Type-Options 等) | 全局中间件 | 1h |
| P2-5 | gateway | adapter 超时硬编码 60s | internal/adapter/ | 1h |
| P2-6 | token-runtime | 弱随机数种子用于 token 生成 | internal/token/token.go | 1h |
| P2-7 | gateway | 弱随机数用于负载均衡 | internal/router/router.go:16 | 0.5h |
| P2-8 | supply-api | IP 字段命名不一致 (SourceIP vs ClientIP) | 多文件 | 1h |

---

## 已修复汇总 (2026-04-17)

| ID | 问题 | 修复方式 |
|----|------|---------|
| P0-0 | IP Spoofing (token-runtime) | 添加 TrustedProxies 过滤，参考 gateway 安全实现 |
| P1-0 | BruteForceProtection 从未激活 | 添加配置字段 + runtime.go 初始化 |
| DUP-1 | splitPath 两处重复实现 | 合并到 internal/pkg/pathutil/path.go |

---

## 执行顺序建议

```
第1轮 (P0 — 1天)
  → P0-1 硬编码密钥 (0.5h) + P0-2 CORS (0.5h) + P0-3 Refresh持久化 (1h)
  → P0-4 并发安全 (1h) + P0-5 audit-events鉴权 (1h)
第2轮 (P1 — 2天)
  → P1-1 KMS升级 (2h) + P1-2 JWT回退 (1h)
  → P1-5 TrustedProxies (1h) + P1-6 请求ID (0.5h) + P1-7 错误泄漏 (1h)
  → P1-3 adapter测试 (4h) — 可并行
第3轮 (P2 — 3天)
  → P2 测试覆盖率提升 + Schema统一 + 安全头
```

---

## 已修复汇总 (2026-04-17)

| ID | 问题 | 修复方式 |
|----|------|---------|
| P0-0 | IP Spoofing (token-runtime) | 添加 TrustedProxies 过滤，参考 gateway 安全实现 |
| P1-0 | BruteForceProtection 从未激活 | 添加配置字段 + runtime.go 初始化 |
| DUP-1 | splitPath 两处重复实现 | 合并到 internal/pkg/pathutil/path.go |

---

## 环境问题说明

> 以下为 **环境配置问题**，非代码缺陷，无需修改代码，通过运维/部署配置解决。

### ENV-1: supply-api go build 失败 — module not found

**现象:**
```
go: module lijiaoqiao/supply-api: not found and --buildvcs=false
```

**原因:** `go env GOPATH` 未配置为支持 `lijiaoqiao/<module>` 风格的模块路径。
Go 1.22 中，go.mod 声明 `module lijiaoqiao/supply-api`，但 GOPATH 未包含对应的 `lijiaoqiao/` 目录结构。

**解决:** 在 Makefile 或 CI 中设置正确的 GOPATH，或使用 `go work` 挂载所有服务模块。

---

### ENV-2: go vet / go test 报告假性错误

**现象:**
```
package lijiaoqiao/platform-token-runtime/... is not in std (...)
```

**原因:** 同 ENV-1 — Go 工具链按 GOPATH 查找本地模块，GOPATH 缺失时将 module path 当作 stdlib 路径处理。

**解决:** 与 ENV-1 相同，配置正确 GOPATH 后自动消失。

---

### ENV-3: 立苌桥 tests 无法在 CI 中独立运行

**现象:** 部分测试依赖真实数据库连接或外部 API mock，单独运行会 panic。

**原因:** 测试设计为在完整环境（有数据库、IAM handler mock）中运行，而非隔离单元测试。

**解决:** 补充测试 fixture 和 mock，或在 CI 中使用 docker-compose 启动完整依赖环境。

---

### ENV-4: 立苌桥部分 Python 工具脚本缺少依赖说明

**现象:** `scripts/mock/supply_gateway_mock_server.py` 等工具运行时缺少类型注解依赖包。

**原因:** 工具脚本使用 `typing.TYPE_CHECKING` 但未在文档中说明运行时不需要这些包。

**解决:** 在工具脚本顶部添加 `if TYPE_CHECKING:` 块说明，或创建 `requirements-dev.txt`。

---

*本计划由 Hermes Agent 根据 2026-04-16 系统性审查报告编制*
