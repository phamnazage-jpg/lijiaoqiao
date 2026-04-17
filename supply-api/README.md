# Supply API

> 供应链业务服务，负责账户、套餐、结算、审计、IAM、Outbox 与补偿链路。

## 当前真实状态

- 服务入口是 `cmd/supply-api/main.go`。
- PostgreSQL 可用时，会装配 DB-backed 的账户、套餐、结算、收益、审计、告警、token 状态、Outbox 与补偿链路。
- 数据库不可用时，开发模式下仍保留部分内存降级路径；当前仓库不把这种模式视为生产可用状态。
- 告警 API 在 PostgreSQL 可用时走数据库仓储；数据库不可用时才显式回退内存实现。
- IAM 路由默认不进入对外交付面；只有 `server.iam_enabled=true` 且 runtime 具备数据库依赖时才注册 `/api/v1/iam/*`。
- 依赖幂等仓储的写接口在中间件缺失时会返回 `503 SUP_HTTP_5031`，不再静默切回内联逻辑。
- Outbox processor 与补偿 worker 仅在数据库可用时启动。

## 本地运行

```bash
cd "/home/long/project/立交桥/supply-api"
go run ./cmd/supply-api -env=dev
```

如果需要本机专用数据库参数，不要直接改 `config/config.dev.yaml`，改用单独覆盖文件：

```bash
cd "/home/long/project/立交桥/supply-api"
cp ./config/config.local.example.yaml ./config/config.local.yaml
go run ./cmd/supply-api -env=dev -config ./config/config.local.yaml
```

## 当前基线 DDL

`scripts/migrate.sh` 当前应用的基线 SQL 与仓储集成测试保持一致：

- `sql/postgresql/supply_core_schema_v2.sql`
- `sql/postgresql/partition_strategy_v1.sql`
- `sql/postgresql/outbox_pattern_v1.sql`
- `sql/postgresql/token_status_registry_v1.sql`
- `sql/postgresql/audit_alerts_v1.sql`

以下文件仍保留在仓库，但不属于当前 fresh setup 基线：

- `sql/postgresql/audit_events_migration_v1_to_v2.sql`
  用于旧版 `audit_events` 表迁移，不应在全新库初始化时直接执行。
- `sql/postgresql/settlement_withdraw_constraint_v1.sql`
  说明当前提现并发约束依赖应用层事务锁，不是独立建表脚本。
- `sql/postgresql/supply_idempotency_record_v1.sql`
  历史独立幂等表方案；当前基线以 `partition_strategy_v1.sql` 中的分区定义为准。

## 验证命令

模块级单元与集成基线：

```bash
cd "/home/long/project/立交桥/supply-api"
GOCACHE=/tmp/lijiaoqiao-go-cache-supply-api go test ./...
bash scripts/run_integration_tests.sh ./internal/repository
```

仓库级统一验证：

```bash
cd "/home/long/project/立交桥"
bash scripts/ci/repo_integrity_check.sh
```

## 关键目录

- `internal/httpapi/`：供应侧业务接口与告警接口。
- `internal/repository/`：DB-backed 仓储与仓储集成测试。
- `internal/audit/`：审计事件、处理器、仓储、批量缓冲。
- `internal/outbox/`：Outbox 处理链路。
- `internal/compensation/` 与 `internal/domain/`：补偿能力与领域模型收口中的实现。
- `scripts/`：迁移、集成测试、历史生产验证脚本。
