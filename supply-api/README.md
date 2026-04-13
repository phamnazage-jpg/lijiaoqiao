# Supply API

> 供应链管理 API 服务

## 项目概述

Supply API 是一个基于 Go 的微服务，提供供应链管理功能，包括：

- **账户管理** - 供应商和消费者账户的 CRUD 操作
- **套餐管理** - 供应链套餐的发布、下架和管理
- **结算服务** - 供应链结算和提现处理
- **收益服务** - 收益记录和账单汇总
- **审计日志** - 完整的审计日志记录和查询
- **IAM (身份和访问管理)** - 多角色权限系统

## 技术栈

- **语言**: Go 1.21+
- **数据库**: PostgreSQL 15+
- **缓存**: Redis
- **框架**: 标准库 + 自定义中间件
- **测试**: Go testing + testify

## 项目结构

```
supply-api/
├── cmd/
│   └── supply-api/           # 主程序入口
│       └── main.go
├── internal/
│   ├── audit/                # 审计日志模块
│   │   ├── model/           # 审计事件模型
│   │   ├── service/         # 审计服务
│   │   ├── handler/         # HTTP 处理器
│   │   ├── repository/      # 数据库仓储 (R-09)
│   │   ├── sanitizer/      # 敏感信息脱敏
│   │   └── events/         # 事件定义 (CRED, SECURITY)
│   ├── iam/                 # IAM 模块
│   │   ├── model/          # 角色、权限模型
│   │   ├── service/         # IAM 服务
│   │   ├── handler/         # HTTP 处理器
│   │   ├── middleware/      # 权限中间件
│   │   └── repository/     # 数据库仓储 (R-08)
│   ├── domain/              # 领域模型
│   ├── middleware/           # HTTP 中间件
│   ├── repository/           # 通用数据仓储
│   ├── cache/                # Redis 缓存
│   └── config/               # 配置管理
├── sql/
│   └── postgresql/           # 数据库 DDL 脚本
│       ├── platform_core_schema_v1.sql
│       ├── iam_schema_v1.sql           # IAM 表 (R-07)
│       └── supply_idempotency_record_v1.sql
└── scripts/
    └── migrate.sh            # 数据库迁移脚本
```

## 模块说明

### IAM 模块 (多角色权限)

| 功能 | 说明 |
|------|------|
| 角色管理 | super_admin, org_admin, supply_admin, operator, developer, finops, viewer |
| 权限范围 | 细粒度 scope 权限控制 |
| 角色继承 | 支持角色层级继承 |
| 中间件验证 | ScopeAuth 中间件 |

**文件**:
- `internal/iam/model/` - 角色、权限模型
- `internal/iam/service/` - IAM 服务层
- `internal/iam/middleware/` - 权限验证中间件

### Audit 模块 (审计日志)

| 功能 | 说明 |
|------|------|
| 事件记录 | CRED/AUTH/DATA/SECURITY 事件分类 |
| 幂等性保证 | IdempotencyKey 支持 |
| 敏感信息脱敏 | 自动扫描和掩码 |
| 指标统计 | M-013/M-014/M-015/M-016 |

**文件**:
- `internal/audit/model/` - 审计事件模型
- `internal/audit/service/` - 审计服务
- `internal/audit/handler/` - HTTP API
- `internal/audit/sanitizer/` - 敏感信息脱敏

### Domain 模块

| Store | 说明 |
|-------|------|
| AccountStore | 账户 CRUD |
| PackageStore | 套餐管理 |
| SettlementStore | 结算处理 |
| EarningStore | 收益记录 |

## API 端点

### 审计 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/audit/events | 创建审计事件 |
| GET | /api/v1/audit/events | 查询事件列表 |

### IAM API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/iam/roles | 创建角色 |
| GET | /api/v1/iam/roles | 列出角色 |
| GET | /api/v1/iam/roles/:code | 获取角色详情 |
| PUT | /api/v1/iam/roles/:code | 更新角色 |
| DELETE | /api/v1/iam/roles/:code | 删除角色 |
| POST | /api/v1/iam/roles/:code/scopes | 分配权限 |
| DELETE | /api/v1/iam/roles/:code/scopes/:scope | 移除权限 |

## 配置

配置文件位于 `config/` 目录：

```yaml
# config/config.dev.yaml
database:
  host: localhost
  port: 5432
  user: supply
  password: ""
  database: supply_db
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 5m

redis:
  host: localhost
  port: 6379
  password: ""
  db: 0
```

本机专用配置不要直接写入 `config/config.dev.yaml`。如果需要使用 Unix socket、本机数据库名或本机用户，请改用单独的本机覆盖文件：

```bash
cp ./config/config.local.example.yaml ./config/config.local.yaml
go run ./cmd/supply-api -env=dev -config ./config/config.local.yaml
```

仓库中的 `config/config.dev.yaml` 保持为可复现样例，`config/config.local.yaml` 只用于本机环境，已加入忽略规则。

## 构建和运行

```bash
# 构建
go build -o supply-api ./cmd/supply-api/

# 运行
./supply-api -env=dev

# 测试
go test ./... -count=1
```

## 测试覆盖率

| 模块 | 覆盖率 |
|------|--------|
| audit/events | 73.5% |
| audit/handler | 83.0% |
| audit/model | 95.0% |
| audit/sanitizer | 79.7% |
| audit/service | 75.3% |
| iam/handler | 85.9% |
| iam/middleware | 83.5% |
| iam/model | 62.9% |
| iam/service | 99.0% |

## 数据库迁移

```bash
# 运行迁移
./scripts/migrate.sh -env=dev
```

## 文档

- [实施状态](./docs/plans/2026-04-03-p1-p2-implementation-status-v1.md)
- [设计文档](./docs/)

## License

Proprietary
