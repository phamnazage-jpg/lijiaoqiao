# Supply API 生产验证报告（历史快照，已校正）
**日期**: 2026-04-13
**状态**: 历史记录保留，不能直接作为当前门禁证据

---

## 文档定位

本文件保留 2026-04-13 的本地排查上下文，但以下条目已经被后续代码复核证伪或降级：

- “Unix socket 配置有解析问题”不再成立。当前代码已经在 `internal/config/config.go` 中对 `host` 以 `/` 开头的 Unix socket DSN 做了显式处理。
- “HTTP 服务器没有监听端口，怀疑 goroutine 调度问题”不再成立。当前代码已经在 `cmd/supply-api/main.go` 中显式启动 goroutine 并调用 `ListenAndServe`。
- “E2E 测试通过”只能说明测试场景在当时通过，不能直接推出“真实 PostgreSQL 持久化已被验证”。

因此，本文件只能作为**历史排查快照**保存，不能继续作为“当前真实状态报告”或“发布门禁依据”引用。

---

## 校正后的执行摘要

| 项目 | 状态 | 说明 |
|------|------|------|
| 数据库 Schema 迁移 | ⚠️ 待独立验证 | 文档声称已完成，但本轮未重新执行数据库校验命令 |
| 数据库连接配置 | ✅ 代码已支持 | 当前代码显式支持 Unix socket DSN |
| HTTP 服务启动路径 | ✅ 代码已存在 | 当前代码显式创建 goroutine 并调用 `ListenAndServe` |
| 审计事件持久化 | ❌ 仍未证实 | 需要真实运行服务并查询 PostgreSQL |
| E2E 测试 | ⚠️ 历史通过记录 | 不能替代数据库持久化验证 |
| 数据持久化 | ❌ 仍未证实 | 需要真实 Repository 路径验证 |

---

## 1. 数据库 Schema 迁移

### 问题描述
审计事件表 `audit_events` 的 schema 与代码中的 model 不匹配：
- **旧版 schema**: `domain_code`, `action_code`, `severity`, `client_ip`
- **代码期望**: `event_name`, `event_category`, `operator_id`, `source_ip`

### 修复措施
创建并执行迁移脚本 `sql/postgresql/audit_events_migration_v1_to_v2.sql`：
- 备份现有数据
- 删除旧表
- 创建新表（基于 partition_strategy_v1.sql）
- 创建 16 个月度分区
- 创建必要的索引

### 验证
```bash
psql -c "\d audit_events"
# 输出显示新列: event_id, event_name, event_category, timestamp_ms, action 等
```

---

## 2. 数据库连接问题（已校正）

### 问题描述
原始排查记录认为 `config.dev.yaml` 中的 Unix socket 路径解析有问题。该判断已经失效，因为当前代码路径已对 Unix socket 明确分支处理。
```
host=/var/run/postgresql user=long database=var/run/postgresql:5432/supply_api
```
更准确的结论应为：当时的本机配置和样例配置被混用了，导致排查记录把“本机配置污染”误写成了“框架解析缺陷”。

### 当前状态
当前代码中的真实情况是：

- `DatabaseConfig.DSN()` 对 `host` 以 `/` 开头的情况返回 `host=%s user=%s dbname=%s sslmode=disable`
- `DatabaseConfig.SafeDSN()` 使用同一分支进行脱敏输出
- `config.dev.yaml` 当前仍然包含本机 Unix socket 配置，这会让样例测试失败，但这属于样例文件污染，不属于 DSN 逻辑缺失

因此应把待修复项改写为：

1. 恢复仓库样例配置与本机配置的边界。
2. 使用 `config.local.*` 或环境变量承载本机 Unix socket 参数。

历史日志如下，仅作排查记录保留：
```
connected to database at /var/run/postgresql:5432
审计存储: 使用PostgreSQL (DB-backed)
```

---

## 3. HTTP 服务器启动问题（已校正）

### 问题描述
原始排查记录认为服务没有监听端口 18082，并怀疑 goroutine 调度问题。该判断已经失效。

### 日志输出
```
starting supply-api in dev mode
connected to database at /var/run/postgresql:5432
connected to redis at localhost:6379
审计存储: 使用PostgreSQL (DB-backed)
外键校验器: 已初始化 (PostgreSQL-backed)
Token状态后端: 使用PostgreSQL (DB-backed)
```
当前代码已显式包含：

```go
go func() {
    jsonLogger.Infof("starting HTTP server on %s", cfg.Server.Addr)
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        serverErrCh <- err
    }
}()
```

更合理的历史解释是：

- 当时采集到的日志并不完整，不能据此断言 HTTP 服务器未启动。
- 若要验证监听状态，应保留 `ss -ltn`、`curl /actuator/health` 或进程启动输出作为原始证据。

---

## 4. E2E 测试结果（解释修正）

文档保留的结论是“生产流测试通过”，但这只能解释为**历史测试记录**：
- `TestProductionFlow_SupplierOnboarding` - ✅ 4 步供应商入驻流程
- `TestProductionFlow_CompleteSettlementCycle` - ✅ 结算周期验证
- `TestProductionFlow_AuditTrailCompliance` - ✅ 审计追溯合规

### 注意
E2E 测试若使用内存审计存储或 mock 依赖，则不能推出以下结论：

- PostgreSQL `audit_events` 已被真实写入
- `supply_accounts` / `supply_packages` 已被真实持久化
- 线上环境依赖已经闭环验证

---

## 5. 当前仍待验证的问题

### P0 - 必须修复

1. **审计事件是否写入真实 PostgreSQL**
   - 原因：本轮只做了代码复核，未复跑数据库查询
   - 影响：无法确认 `audit_events` 表是否接收真实业务事件

2. **供应账户与套餐是否走真实 Repository 持久化**
   - 原因：当前归档里缺少对应 SQL 查询证据
   - 影响：功能闭环仍停留在接口/测试层

### P1 - 应该修复

1. **样例配置与本机配置混用**
   - `config.dev.yaml` 已被本机 Unix socket 值污染
   - 需要恢复样例配置，并新增本机覆盖方式

---

## 6. 建议修复步骤

### 步骤 1: 恢复配置边界
1. 让 `config.dev.yaml` 回到仓库样例值
2. 新增本机覆盖文件或环境变量说明
3. 运行 `go test ./internal/config`

### 步骤 2: 验证审计事件持久化
1. 启动服务
2. 调用 API（如 `/api/v1/supply/accounts/verify`）
3. 查询数据库 `SELECT COUNT(*) FROM audit_events;`

### 步骤 3: 验证真实持久化
1. 创建账户和套餐
2. 查询 `supply_accounts` 与 `supply_packages`
3. 将原始 SQL 输出与请求日志一起归档

---

## 7. 建议验证命令

```bash
# 检查审计事件表结构
PGPASSWORD="" psql -U long -d supply_api -h /var/run/postgresql -c "\d audit_events"

# 检查审计事件数量
PGPASSWORD="" psql -U long -d supply_api -h /var/run/postgresql -c "SELECT COUNT(*) FROM audit_events;"

# 检查账户数量
PGPASSWORD="" psql -U long -d supply_api -h /var/run/postgresql -c "SELECT COUNT(*) FROM supply_accounts;"

# 检查套餐数量
PGPASSWORD="" psql -U long -d supply_api -h /var/run/postgresql -c "SELECT COUNT(*) FROM supply_packages;"
```

---

**报告生成时间**: 2026-04-13T20:10:00+08:00
