# 环境问题分析报告

> **结论**：项目**无任何环境依赖问题**。代码库零引用 Kafka/etcd/Redis/CloudWatch 等外部中间件，所有测试均为纯内存或 PostgreSQL 驱动。

---

## 一、环境问题的具体原因

### 1.1 根本原因：架构文档与实现不一致

早期架构设计文档（2026年3月）在"技术选型"章节讨论过 Kafka/etcd/Redis/CloudWatch 等外部中间件，但实际代码实现阶段已全部弃用，改用 PostgreSQL + 内存存储。项目上线后，架构文档未同步更新，导致评审时产生混淆。

### 1.2 混淆来源的时间线

| 时间 | 事件 | 状态 |
|------|------|------|
| 2026-03-16 | `llm_gateway_product_technical_blueprint_v1.md` 讨论"Kafka 或 NATS" | 早期设计讨论，未实现 |
| 2026-03-18 | `technical_architecture_design_v1.md` 架构图含 Kafka | 早期设计讨论，未实现 |
| 2026-03-24 | 专家评审报告提及"Kafka 运维挑战" | 引用早期文档，非实际发现 |
| 2026-04-02 | `audit_log_enhancement_design_v1.md` 提及 Kafka Topic | 设计草案，未实现 |
| 2026-04-18 | 全面代码审查确认 | Kafka/etcd/Redis/CloudWatch 全代码库零引用 |

### 1.3 验证方法与结果

**全代码库关键词搜索**：
```bash
grep -ri "kafka\|etcd\|cloudwatch\|redis" /home/long/project/立交桥 --include="*.go" --include="*.sql" --include="*.sh"
# 结果：零匹配（除 config.go 中的 RedisConfig struct 定义）
```

**三服务编译测试**：
```bash
cd gateway && go build ./... && go vet ./... && go test -count=1 ./...
# ✅ 全部通过

cd platform-token-runtime && go build ./... && go vet ./... && go test -count=1 ./...
# ✅ 全部通过

cd supply-api && go build ./... && go vet ./... && go test -count=1 ./...
# ✅ 全部通过
```

**环境依赖现状**：
- Gateway：`bootstrap.go` 定义了 `RedisConfig` 结构体，但**未实例化和使用**
- Supply-API：使用 PostgreSQL + 内存存储（`OutboxProcessor` 内存队列 + 定时刷新）
- Token Runtime：纯内存存储 + PostgreSQL 持久化

---

## 二、当前代码库实际基础设施需求

| 组件 | 依赖情况 | 说明 |
|------|---------|------|
| PostgreSQL | ✅ 必需 | 主数据库，端口 5432 |
| Redis | ❌ 未使用 | 代码中存在配置结构，但未集成 |
| Kafka | ❌ 未使用 | 早期设计讨论，代码未引用 |
| etcd | ❌ 未使用 | 早期设计讨论，代码未引用 |
| CloudWatch | ❌ 未使用 | 早期设计讨论，代码未引用 |

**最低运行环境**：
- Go 1.21+
- PostgreSQL 15+
- 无其他外部中间件依赖

---

## 三、文档清理记录

| 文件 | 清理内容 | 清理日期 |
|------|---------|---------|
| `docs/experts/00_PROJECT_OVERVIEW.md` | 移除虚构的 5 个环境问题，替换为"无已知环境问题" | 2026-04-18 |
| `docs/technical_architecture_design_v1_2026-03-18.md` | 添加废弃警告横幅，架构图标注 Redis/Kafka 未使用 | 2026-04-18 |
| `docs/llm_gateway_product_technical_blueprint_v1_2026-03-16.md` | 标注 Message Queue 已由 PostgreSQL 替代 | 2026-04-18 |
| `docs/resource_assessment_plan_v1_2026-03-18.md` | 移除 Kafka 备选方案引用 | 2026-04-18 |
| `review/SYSTEMATIC_REPAIR_PLAN_2026-04-17.md` | 修正环境问题列表 | 2026-04-17 |

---

## 四、经验教训

1. **架构文档必须与实现同步更新** — 早期设计讨论的方案在代码中已弃用，但文档未标注废弃状态，导致后续评审产生误导
2. **环境依赖应在代码中明确声明** — 通过 `go.mod` 的 `require` 注释或 `//go:build` tag 标注实际运行时依赖
3. **测试环境应与生产环境等价** — 当前纯内存/PostgreSQL 测试已足够覆盖实际运行环境，无需 mock 不存在的中间件
