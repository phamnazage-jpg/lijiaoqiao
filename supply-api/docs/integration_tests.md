# Supply API 集成测试指南

## 概述

本项目包含单元测试和集成测试。集成测试需要真实的基础设施（PostgreSQL 和 Redis）。

## 测试类型

| 类型 | 运行方式 | 基础设施 |
|------|----------|----------|
| 单元测试 | `go test ./...` | Mock |
| 集成测试 | `go test -tags=integration ./...` | 真实 DB + Redis |

## 快速开始

### 1. 启动测试基础设施

```bash
docker-compose -f deploy/docker-compose.yml up -d
```

### 2. 运行集成测试

```bash
# 使用提供的脚本（推荐）
./scripts/run_integration_tests.sh

# 或手动运行
export SUPPLY_API_DB_HOST="localhost"
export SUPPLY_API_DB_PORT="5432"
export SUPPLY_API_DB_USER="supply_test"
export SUPPLY_API_DB_PASSWORD="supply_test_pass"
export SUPPLY_API_DB_NAME="supply_test"
export SUPPLY_TEST_POSTGRES="postgres://supply_test:supply_test_pass@localhost:5432/supply_test?sslmode=disable"
export SUPPLY_TEST_REDIS="localhost:6379"
go test -tags=integration -v ./internal/repository ./internal/middleware/...
```

### 3. 停止基础设施

```bash
docker-compose -f deploy/docker-compose.yml down -v
```

## 测试覆盖

### 当前覆盖率

| 模块 | 单元测试覆盖率 | 集成测试覆盖 |
|------|----------------|--------------|
| middleware | **80.4%** | 完整覆盖 |
| - db_token_backend.go | 92%+ | 真实 DB + Redis |
| - token_revocation_service.go | 71%+ | Redis Pub/Sub |

### 集成测试覆盖的场景

1. **CheckTokenStatus_CacheHit** - Redis 缓存命中
2. **RevokeToken** - 真实数据库和 Redis 联动
3. **RevokeBySubjectID** - 批量吊销
4. **RevocationService** - Redis Pub/Sub 发布/订阅

## 文件结构

```
supply-api/
├── deploy/
│   └── docker-compose.yml          # 测试基础设施
├── scripts/
│   └── run_integration_tests.sh    # 集成测试运行脚本
├── internal/
│   └── middleware/
│       ├── db_token_backend_integration_test.go  # 集成测试
│       └── *_test.go               # 单元测试
└── docs/
    └── integration_tests.md        # 本文档
```

## 常见问题

### Q: 集成测试需要什么？

**A:** Docker 和 Docker Compose。需要 PostgreSQL 15+ 和 Redis 7+。

### Q: 可以在 CI 中运行集成测试吗？

**A:** 可以。CI 环境需要：
1. Docker-in-Docker (DinD) 支持
2. 使用 `docker-compose up -d` 启动基础设施
3. 为 `repository` 测试设置 `SUPPLY_API_DB_*` 环境变量
4. 为 `middleware` 测试设置 `SUPPLY_TEST_POSTGRES` 和 `SUPPLY_TEST_REDIS`
5. 运行 `go test -tags=integration ./...`

### Q: 为什么某些测试需要集成测试？

**A:** 某些代码路径依赖真实的基础设施：
- Redis Pub/Sub 发布/订阅功能
- PostgreSQL 事务和约束
- 网络超时和重试逻辑
- 连接池行为

单元测试使用 mock 无法完全模拟这些行为。

## 清理

```bash
# 停止并清理容器和数据卷
docker-compose -f deploy/docker-compose.yml down -v
```
