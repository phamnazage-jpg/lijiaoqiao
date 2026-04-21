# 环境问题记录

本文档记录所有因**环境依赖**（而非纯代码实现）而未完成的优化项，以及其具体原因。

---

## P3 结构性修改 — 环境依赖型

以下 P3 项为代码硬改，但因缺少**真实 staging 环境**和**生产等效配置**而仅完成设计稿，未完成实现验证：

### P3-A: RemoteTokenRuntime HTTP timeout + cache eviction

**状态**：设计稿（代码注释标注），未落地

**具体原因**：
- `gateway/internal/middleware/remote_runtime.go` 当前使用 `http.DefaultClient`（无超时）
- 缓存 `records map[string]remoteResolvedToken` 无 TTL 淘汰机制
- 需要在 `gateway/internal/config/config.go` 添加 8 个 env var，但当前配置系统不支持热加载
- 需要一个专用的 `http.Client` builder 注入到 `buildTokenRuntime()`，涉及 bootstrap 改造

**依赖项**：
- 真实 staging environment（需要 `GATEWAY_TOKEN_RUNTIME_HTTP_TIMEOUT` 等 env var 的加载路径）
- `gateway/internal/config/config.go` 需要 `dotenv` 或 `viper` 支持（当前不支持 env var 热加载）

**下一步**：需要运维在 staging/prod 环境中验证 timeout 值，暂无自动化手段替代。

---

### P3-B: platform-token-runtime /metrics 端点

**状态**：设计稿，未落地

**具体原因**：
- `platform-token-runtime/internal/app/bootstrap.go` 只返回 `{"status":"UP"}`，无 Prometheus 指标
- 该服务使用 Go 语言，需要引入 `prometheus/client_golang` 依赖并修改 `/health` handler
- 缺少 staging 环境中的 Prometheus scrape target 配置

**依赖项**：
- `go.mod` 需要添加 `github.com/prometheus/client_golang`
- 运维需要更新 Prometheus scrape config（不在代码库管理范围内）

---

### P3-C: gateway /metrics 端点

**状态**：设计稿，未落地

**具体原因**：
- `gateway/internal/handler/handler.go` 无 metrics export
- 与 P3-A 的 `upstream_latency_ms` 指标设计耦合

**依赖项**：同 P3-A

---

### P3-D: supply-api graceful shutdown

**状态**：未开始

**具体原因**：
- `supply-api/cmd/supply-api/main.go` 未实现 signal hook，进程直接 SIGTERM
- 需要在 `main.go` 中添加 trap + context cancel 逻辑
- 需要 staging 环境的真实流量压测来验证 shutdown 不会丢请求

---

## 非环境问题（已完成）

以下优化项**不依赖外部环境**，可通过代码审查和 CI 验证完成：

| 项目 | 说明 | 状态 |
|---|---|---|
| Phase 1 Criterion 4 | contract tests 从设计稿变可执行脚本，集成到 backend-verify.sh | ✅ 已实现 |
| Phase 2 Criterion 1 | manifest.json 系统（生成+消费+硬门禁） | ✅ 已实现 |
| Phase 2 Criterion 2 | superpowers_stage_validate.sh：CONDITIONAL_GO → exit 1 | ✅ 已实现 |
| Phase 2 Criterion 3 | DEFERRED 不再作为 pass；CONDITIONAL_GO 语义清理 | ✅ 已实现 |
| Phase 2 Criterion 5 | cross_service_smoke.sh 从 DESIGN_ONLY 变可执行 | ✅ 已实现 |
| Phase 2 Criterion 4 | staging/prod 配置独立化 | ✅ 已完成（之前已落地）|

---

## 环境问题 vs 非环境问题区分原则

**非环境问题**：可通过以下方式验证
- `bash -n` 语法检查
- 纯 shell unit test（mock 网络调用）
- 代码审查确认逻辑正确性

**环境问题**：必须满足以下任一条件才能验证
- 真实 staging 环境运行
- 生产等效配置（真实的 env var、真实的数据库、真实的 sidecar）
- 运维介入（Prometheus 配置、容器编排修改）

---

*最后更新：2026-04-21*
