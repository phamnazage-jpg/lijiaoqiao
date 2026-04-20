# TDD 修复任务清单

> 基于 6 个维度评审报告汇总所有待修复问题，按优先级排序
> 生成时间: 2026-04-18

---

## P0 — 必须修复（安全/并发/编译阻塞）

| # | 问题 | 位置 | 说明 | TDD 状态 |
|---|------|------|------|---------|
| P0-01 | HTTP Client 无连接池 | `gateway/internal/adapter/openai_adapter.go:27` | `&http.Client{Timeout: 60s}` 未配置 Transport，生产环境连接耗尽 | 🔴 待修复 |
| P0-02 | SafeDSN 格式化 BUG | `supply-api/internal/config/config.go:112` | 格式字符串 4 占位符传 5 参数，d.Password 被忽略；DSN() 写死 `***` 无法显示真密码 | 🔴 待修复 |
| P0-03 | 提现 Check-And-Create 竞态 | `supply-api/internal/domain/settlement.go` | `HasPendingOrProcessingWithdraw` 和 `CreateInTx` 不在同一事务，并发提现可绕过校验 | 🔴 待修复 |
| P0-04 | Scope 切片缓存别名 bug | `gateway/internal/middleware/remote_runtime.go:100` | `result.Data.Scope` 直接存入缓存，JSON decoder  buffer 复用导致数据corruption | 🔴 待修复 |
| P0-05 | 无结构化日志 | 三服务全局 | 仅 `log.Printf`，生产环境调试困难，无法做日志聚合查询 | 🟡 优化项（不影响编译） |

---

## P1 — 重要修复（数据库/性能/安全）

| # | 问题 | 位置/说明 | TDD 状态 |
|---|------|------|---------|
| P1-01 | `supply_usage_records` 缺复合索引 | `(supplier_user_id, started_at)` 高频查询无索引，数据库全表扫描 | 🟡 迁移任务（非代码 TDD） |
| P1-02 | `supply_earnings` 缺复合索引 | `(user_id, status)` 缺索引，影响结算状态查询 | 🟡 迁移任务 |
| P1-03 | `supply_accounts` 缺审计字段 | 无 `created_by`/`updated_by`，无法追溯账户创建者 | 🟡 迁移任务 |
| P1-04 | `supply_accounts.risk_level` 缺 CHECK | 值域无限制，可写入任意值（如负数） | 🟡 迁移任务 |
| P1-05 | 无熔断器模式 | Gateway 无法防止下游故障级联，单点故障影响全局 | 🟡 优化项 |
| P1-06 | Gateway 无 Dockerfile | 无法容器化部署 | 🟡 基础设施 |

---

## P2 — 改进项（长期优化）

| # | 问题 | 说明 | 状态 |
|---|------|------|------|
| P2-01 | 接口风格不统一 | 三服务响应格式/错误码体系各异 | 📋 规划中 |
| P2-02 | 配置管理分散 | 三个服务各自读取环境变量，无统一配置中心 | 📋 规划中 |
| P2-03 | 模块边界需加强 | 防止循环依赖，整理 import 依赖图 | 📋 规划中 |
| P2-04 | RoutingMetrics 未接入 | Bootstrap 中预留但未实际收集 | 📋 规划中 |
| P2-05 | Supply API SafeDSN 密码泄露 | DSN() 中密码未正确显示/隐藏 | 🔴 待修复（见 P0-02） |

---

## TDD 实施记录

### P0-02: SafeDSN 格式化 BUG（进行中）

**问题分析**:
- `DSN()`: `fmt.Sprintf("postgres://%s:***@%s:%d/%s?sslmode=disable", d.User, d.Password, d.Host, d.Port, d.Database)`
  - 4 个占位符 (`%s`, `%s`, `%d`, `%s`) 但传了 5 个参数
  - `d.Password` 被完全忽略！实际输出 `postgres://user:***@host:5432/db`
  - `***` 是字面量，不是占位符，所以 DSN() 永远无法显示真实密码
- `SafeDSN()`: 同样有 bug——d.Password 无占位符可填

**根因**: `***` 本意是掩码，但实现时没有为 d.Password 留占位符

**修复方案**:
```go
// DSN(): 正确的 5 占位符格式
fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", d.User, d.Password, d.Host, d.Port, d.Database)

// SafeDSN(): password 参数被替换为掩码
fmt.Sprintf("postgres://%s:***@%s:%d/%s?sslmode=disable", d.User, d.Password, d.Host, d.Port, d.Database)
```
