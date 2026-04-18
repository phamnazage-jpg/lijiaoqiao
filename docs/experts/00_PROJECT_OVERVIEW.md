# 立交桥项目专家评审 — 项目概况

## 1. 项目基本信息

- **项目名称**：立交桥（lijiaoqiao）
- **路径**：`/home/long/project/立交桥`
- **语言**：Go 1.21/1.22
- **架构**：单体服务（3 个独立进程），保持单体架构不变
- **总规模**：约 170,000 行 Go 代码（不含竞品目录），112 个 .go 文件
- **依赖**：Go modules，无 vendor 目录

---

## 2. 三个核心服务

### 2.1 Gateway（入口网关）

**定位**：OpenAI 兼容入口网关，负责接入鉴权、限流、上游路由、基础审计。

**端口**：8080

**技术栈**：
- 标准库 `net/http`（无 Gin）
- 中间件：Auth、CORS、RateLimit、RequestID 注入
- 路由策略：`latency` / `round_robin` / `weighted` / `availability`（已接入主链路）；`cost_based` / `cost_aware` / `fallback`（实验性，未接入）
- Provider 注册：支持多 provider 配置，动态路由
- 审计发射器：PostgreSQL 或内存

**关键文件**：
- `internal/handler/handler.go` — OpenAI 兼容 HTTP handler（Chat/Completion/Models）
- `internal/middleware/cors.go` — CORS 配置（生产要求显式白名单）
- `internal/app/bootstrap.go` — 启动装配，`BuildMux`
- `internal/router/router.go` — 路由选择、打分

**环境变量**：
- `GATEWAY_ENV`（dev/staging/production）
- `PASSWORD_ENCRYPTION_KEY`（生产必填）
- `GATEWAY_CORS_ALLOW_ORIGINS`（生产必填）
- `GATEWAY_TRUSTED_PROXIES`
- `GATEWAY_TOKEN_RUNTIME_MODE`（inmemory / remote_introspection）

**已修复的安全问题**：
- 生产环境默认密钥检测并 panic
- CORS 生产模式强制显式白名单
- X-Request-ID 字符白名单过滤（防日志注入）
- IP 伪造防护（TrustedProxies 检查）

---

### 2.2 Platform Token Runtime

**定位**：平台级 token 生命周期管理、introspection 与审计查询。

**端口**：18081

**技术栈**：
- 标准库 `net/http`
- Token 生命周期：issue / refresh / revoke / introspect
- Store 实现：内存（dev 默认）+ PostgreSQL（staging/prod 可选）
- 审计事件：内存 / PostgreSQL

**关键文件**：
- `internal/httpapi/token_api.go` — HTTP API（issue/refresh/revoke/introspect/audit-events）
- `internal/auth/service/runtime_store.go` — 内存 runtime store（含并发保护 mutex）
- `internal/auth/service/inmemory_runtime.go` — token 生命周期实现
- `internal/auth/middleware/token_auth_middleware.go` — Bearer token 校验
- `internal/auth/middleware/query_key_reject_middleware.go` — 拒绝外部 query key
- `internal/auth/service/postgres_runtime_store.go` — PostgreSQL 持久化
- `internal/auth/service/postgres_audit_store.go` — PostgreSQL audit store

**环境变量**：
- `TOKEN_RUNTIME_ENV`（dev/staging/production）
- `TOKEN_RUNTIME_DATABASE_URL`（PostgreSQL DSN）

**已修复的安全问题**：
- Refresh 后 TTL 变更持久化到 store
- InMemoryRuntimeStore 并发读写加 RWMutex 保护
- audit-events 接口强制 Bearer token 鉴权

---

### 2.3 Supply API

**定位**：供应链业务服务，负责账户、套餐、结算、审计、IAM、Outbox 与补偿链路。

**端口**：18080

**技术栈**：
- 标准库 `net/http`
- 数据库：PostgreSQL（必选，dev 模式可部分降级）
- SMS 验证（默认关闭态）
- Outbox pattern（事务性发件箱）
- 补偿框架（account.create / package.publish / settlement.withdraw / quota.deduct）

**关键文件**：
- `internal/httpapi/supply_api.go` — 供应侧业务接口（accounts/packages/billing/earnings/settlements）
- `internal/httpapi/alert_api.go` — 告警接口
- `internal/repository/` — DB-backed 仓储与集成测试
- `internal/audit/` — 审计事件、处理器、批量缓冲
- `internal/outbox/` — Outbox processor
- `internal/compensation/` — 补偿执行器（当前 fail-closed）
- `internal/iam/` — IAM 实现（默认关闭，条件启用）
- `internal/domain/settlement.go` — 结算领域模型（含提现门禁）
- `internal/security/kms_service.go` — DEK 派生（HKDF-SHA256）
- `internal/app/bootstrap.go` — 启动装配
- `internal/app/runtime.go` — runtime 构造与依赖注入

**环境变量**：
- `SUPPLY_API_ENV`（dev/staging/production）
- `SMS_VERIFIER_IMPL`（可选）
- `SETTLEMENT_WITHDRAW_ENABLED`（默认 false）
- `SERVER_IAM_ENABLED`（默认 false）

**已修复的安全问题**：
- KMS DEK 派生从简单字节轮换升级为 HKDF-SHA256
- 生产环境强制拒绝 HS256/HS384/HS512（只接受 RSA）
- 补偿执行器从假成功改为 fail-closed
- IP 伪造防护（TrustedProxies 注入）
- BruteForce 暴力破解保护（5 次失败 / 15 分钟锁定）

---

## 3. 数据库

### 3.1 DDL 文件

- `sql/postgresql/token_runtime_schema_v1.sql` — token runtime 表结构
- `sql/postgresql/platform_core_schema_v1.sql` — 平台侧审计事件
- `sql/postgresql/supply_core_schema_v2.sql` — supply-api 核心表（accounts/packages/settlements/earnings）
- `sql/postgresql/partition_strategy_v1.sql` — 分区策略
- `sql/postgresql/outbox_pattern_v1.sql` — Outbox 表
- `sql/postgresql/token_status_registry_v1.sql` — token 状态注册表
- `sql/postgresql/audit_alerts_v1.sql` — 告警审计表

### 3.2 关键数据库模式观察

- `auth_token_runtime` 表存储 token 生命周期数据
- `auth_token_audit_events` 表存储 token 审计事件（token runtime 侧）
- `supply_*` 系列表存储供应侧业务数据
- `outbox` 表实现事务性发件箱模式
- 分区策略：按时间或哈希分区

---

## 4. 测试现状

### 4.1 测试类型

- **单元测试**：各模块内部逻辑测试（`*_test.go`）
- **集成测试**：`supply-api` 仓储层有 `bash scripts/run_integration_tests.sh`
- **E2E 测试**：`supply-api/e2e/production_flow_test.go`（使用 mock 替身，非真实外部依赖）

### 4.2 测试覆盖的关键功能

- Gateway handler 测试（Chat/Completion/Models/error response）
- Token lifecycle 测试（issue/refresh/revoke/introspect）
- Token audit events 测试（query / authorization enforcement）
- Supply API 核心业务流程测试

### 4.3 测试环境限制（5 个环境问题，非代码缺陷）

1. `TestTokenStoreIntegration` — GOPATH 未配置导致 module not found
2. `TestAuditLogExporter` — 需要 etcd broker 在 2379
3. `TestIntegrationPipeline` — 需要 Kafka broker 在 9092
4. `TestCloudWatchLogsExporter` — 需要 AWS 凭据配置
5. Python 类型提示警告 — 系统 Python < 3.10

---

## 5. CI/CD 与验证

- `scripts/ci/repo_integrity_check.sh` — 仓库级统一验证（已升级为 `go test -count=1`）
- 各模块独立验证：`go test -count=1 ./...`
- Supply API 额外：`bash scripts/run_integration_tests.sh ./internal/repository`
- 迁移脚本：`scripts/migrate.sh`

---

## 6. 文档

- `TEST_ENVIRONMENT_ISSUES.md` — 5 个环境问题记录
- `review/SYSTEMATIC_REPAIR_PLAN_2026-04-17.md` — 21 个问题的修复计划
- `review/REPORT_CORRECTION_2026-04-17.md` — 纠偏版报告
- `docs/plans/2026-04-17-remediation-execution-checklist.md` — 执行清单（13 tasks 全部完成）
- `docs/product/completed_feature_inventory_v1_2026-04-17.md` — 已完成功能清单 v1
- 各服务 README 记录了"当前真实状态"

---

## 7. 项目约束

- **架构约束**：保持单体服务架构，不拆分为微服务
- **优化目标**：代码质量、性能、可靠性、运维简单性
- **已知技术债务**：
  - 立交桥项目是单体架构但实际上代码规模不小，三个服务各自独立，有各自的 DB
  - 有些能力"已写出但未接入主链路"（如高级路由策略）
  - E2E 测试依赖 mock，不等价于真实环境全链路
  - 部分 Go module path 为 `lijiaoqiao/<module>`，对 GOPATH 有一定要求
