# 整改完成确认单

- 项目：立交桥
- 路径：`/home/long/project/立交桥`
- 日期：2026-04-17
- 依据：
  - `review/REPORT_CORRECTION_2026-04-17.md`
  - `docs/plans/2026-04-17-remediation-execution-checklist.md`

---

## 一、结论

本轮整改执行清单已经全部完成。

当前结论分三层：

1. 纠偏报告中确认的阻塞性代码问题，已经全部修复。
2. 当前仓库已经恢复到“可验证、可回归、可继续交付”的状态。
3. 仍然存在的后续事项，不应再归类为“本轮未修复缺陷”，而应拆分为：
   - 上线条件
   - 长期治理项

---

## 二、已修复

以下问题已经在本轮整改中完成修复，并通过当前仓库验证链确认：

### 2.1 构建与测试链路

1. `supply-api` 的 HKDF 编译阻塞已修复，整仓 `go test -count=1 ./...` 已恢复。
2. `gateway` 的 `BuildMux` 签名失配和错误脱敏断言失配已修复。
3. `platform-token-runtime` 的中间件签名失配和 `audit-events` 鉴权测试失配已修复。
4. `scripts/ci/repo_integrity_check.sh` 已升级为可信基线，默认使用无缓存测试，并纳入 `supply-api` 仓储集成。

### 2.2 运行时能力闭环

1. `platform-token-runtime` 已引入统一 store 抽象，并落地 PostgreSQL-backed 的最小 runtime / audit store。
2. `supply-api` 默认补偿执行器已改为 fail-closed，不再出现“假成功”。
3. `supply-api` 的 IAM 路由已经收敛为显式能力：默认关闭，满足条件才挂载。
4. `supply-api` 的提现能力已经收敛为显式 readiness 门禁：SMS 未就绪时保持关闭并返回明确错误。

### 2.3 配置与对外行为

1. `gateway` 在生产环境下已禁止默认密钥与默认 `*` CORS。
2. `gateway /v1/models` 已改为基于已注册 provider 动态聚合输出。
3. `gateway` 已明确高级路由策略目前不进入主启动链路。

### 2.4 SQL 与文档边界

1. `supply-api/sql/postgresql/supply_core_schema_v2.sql` 已按当前真实领域状态补齐 `CHECK` 约束。
2. `sql/postgresql/platform_core_schema_v1.sql`、`sql/postgresql/token_runtime_schema_v1.sql` 的服务边界已写清。
3. `ClientIP` / `SourceIP` 的统一方向已经固化为规范，不再作为阻塞缺陷处理。

---

## 三、属于上线条件

以下事项不是“代码未修复”，而是正式上线时必须满足的环境或接入条件：

### 3.1 `supply-api` 提现能力

1. 只有 `settlement.withdraw_enabled=true` 且 SMS 配置齐备，并实际注入真实 `SMSVerifier` 时，提现才会开放。
2. 若 SMS 未就绪，系统会继续保持关闭态并返回 `SMS is not ready`。

### 3.2 `gateway` 生产配置

1. 生产环境必须显式提供 `PASSWORD_ENCRYPTION_KEY`。
2. 生产环境必须显式提供 `GATEWAY_CORS_ALLOW_ORIGINS`。
3. 这类要求是故意设计的 fail-closed 机制，不属于缺陷残留。

### 3.3 `platform-token-runtime` 部署条件

1. PostgreSQL-backed runtime 已经具备，但正式部署仍需要真实数据库实例。
2. 正式部署仍需要执行对应 schema 初始化。
3. 正式部署仍需要正确提供数据库连接环境变量与运行时装配。

---

## 四、属于长期治理项

以下事项真实存在，但不应继续与“本轮整改未完成”混为一谈：

### 4.1 命名一致性

1. 仓库中仍存在历史存量 `ClientIP` / `SourceIP` 并存。
2. 当前规则已经明确：
   - HTTP / middleware 局部上下文允许保留 `ClientIP`
   - 审计 / 持久化 / 对外字段统一 `SourceIP`
3. 后续如果要做全仓统一，应单独作为维护性重构推进。

### 4.2 测试层级差异

1. `supply-api` 的 e2e 仍以 HTTP 流程回归为主。
2. 它不能等价为真实短信、消息总线、第三方支付等外部依赖已全部联通。
3. 这属于测试覆盖层级边界，不是当前代码断裂。

### 4.3 实验性能力

1. `gateway` 中的 `cost_based`、`cost_aware`、`fallback` 仍属于实验性模块。
2. 当前文档已明确它们未接入主启动链路。
3. 若后续要进入主链路，应单独立项、补设计、补验证，而不是视为“本轮漏修”。

### 4.4 审查产物治理

1. 2026-04-16 原始审查报告仍保留在仓库中。
2. 这些报告可作为历史输入，但不应再直接作为当前状态依据。
3. 当前状态判断应以以下两份文档为准：
   - `review/REPORT_CORRECTION_2026-04-17.md`
   - `review/REMEDIATION_COMPLETION_CONFIRMATION_2026-04-17.md`

---

## 五、最终验证

本轮整改完成后，已实际通过以下验证：

1. `cd gateway && go test -count=1 ./...`
2. `cd platform-token-runtime && go test -count=1 ./...`
3. `cd supply-api && go test -count=1 ./...`
4. `cd supply-api && bash scripts/run_integration_tests.sh ./internal/repository`
5. `bash scripts/ci/repo_integrity_check.sh`
6. `git diff --check`

---

## 六、确认结论

如果只保留一句话，当前最准确的表述是：

**整改执行清单已经全部完成；纠偏报告中确认的阻塞性代码问题已经全部修复；剩余事项属于上线条件或长期治理项，不再属于本轮未完成缺陷。**
