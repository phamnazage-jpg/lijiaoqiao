# AI-Customer-Service 生产上线文档

> 版本：v1.0 | 日期：2026-05-01
> 状态：⚠️ 代码级主链已通过验证，但预生产与灰度门禁尚未闭环
> 代码基准：`3e9022a`（`upload/ai-customer-service` 分支）

---

## 1. 项目概述

**项目名**：ai-customer-service（立交桥智能客服系统）
**一句话**：当前交付物是面向生产一期的客服后端最小闭环服务，覆盖 webhook、会话、转人工工单、审计与健康检查。

**当前已验证能力**：
- 统一 Webhook 入口与按路径覆写 channel 的入口
- 基于规则的意图识别与静态 FAQ 回复
- 自动转人工工单最小闭环（创建→分配→解决→关闭）
- 审计日志持久化
- PostgreSQL 持久化、健康检查、优雅停机

**当前未完成但属于后续目标能力**：
- 真实 LLM 意图识别与多供应商 failover
- 真实 RAG 检索与知识库运营
- 完整多渠道适配器产品化
- 运营后台 UI 与完整 RBAC

---

## 2. 技术架构

**技术栈**：Go 1.22 + PostgreSQL + HTTP/REST
**二进制**：`cmd/ai-customer-service`
**模块结构**：
```
internal/
├── app/              # 应用入口、graceful shutdown
├── config/           # 配置加载（环境变量驱动）
├── domain/           # 领域模型（ticket、session、intent、message、audit）
├── http/
│   ├── router.go     # HTTP 路由注册
│   └── handlers/     # HTTP Handler（webhook/session/ticket/health/stats）
├── platform/
│   ├── httpx/        # HTTP 扩展（BodyLimit、速率限制）
│   ├── logging/      # 结构化日志（slog）
│   └── health/      # 健康检查探针
└── service/
    ├── dialog/       # 对话引擎（Process 主流程）
    ├── intent/       # 意图识别
    ├── handoff/      # 转人工
    └── reply/        # 回复生成
store/
├── memory/           # 内存存储（测试/开发用）
└── postgres/         # PostgreSQL 持久化（生产用）
```

---

## 3. API 接口清单

### 3.1 Webhook（外部消息入口）

| 方法 | 路径 | 说明 | 状态 |
|------|------|------|------|
| POST | `/api/v1/customer-service/webhook` | 统一 Webhook 入口 | ✅ 已实现 |
| POST | `/api/v1/customer-service/webhook/{channel}` | 按路径指定 channel 的 Webhook 入口 | ✅ 已实现 |

**安全特性**：HMAC-SHA256 签名校验 + 时间戳防重放 + BodyLimit 512KB + 速率限制（滑动窗口 10 req/s/IP）

### 3.2 会话管理

| 方法 | 路径 | 说明 | 状态 |
|------|------|------|------|
| POST | `/sessions/{id}/handoff` | 手动转人工 | ✅ 已实现 |
| POST | `/sessions/{id}/feedback` | 用户反馈提交 | ✅ 已实现 |

### 3.3 工单管理

| 方法 | 路径 | 说明 | 状态 |
|------|------|------|------|
| GET | `/tickets` | 工单列表（分页） | ✅ 已实现 |
| GET | `/tickets/{id}` | 工单详情 | ✅ 已实现 |
| POST | `/tickets/{id}/assign` | 工单分配（agent_id） | ✅ 已实现 |
| POST | `/tickets/{id}/resolve` | 工单解决 | ✅ 已实现 |
| POST | `/tickets/{id}/close` | 工单关闭 | ✅ 已实现（audit 已接入） |

### 3.4 运营与健康

| 方法 | 路径 | 说明 | 状态 |
|------|------|------|------|
| GET | `/actuator/health` | 综合健康检查 | ✅ 已实现 |
| GET | `/actuator/health/live` | Liveness 探针 | ✅ 已实现 |
| GET | `/actuator/health/ready` | Readiness 探针（含 DB 依赖检查） | ✅ 已实现 |
| GET | `/tickets/stats` | 工单统计（open/assigned/resolved） | ✅ 已实现 |

---

## 4. 质量验证结果

### 4.1 测试覆盖率（Phase 2 目标达成）

| 包 | 覆盖率 | Phase 2 目标 | 状态 |
|----|--------|-------------|------|
| internal/http/handlers | **87.1%** | >85% | ✅ |
| internal/service/dialog | **88.5%** | >85% | ✅ |
| internal/platform/httpx | **84.3%** | >70% | ✅ |
| internal/config | **82.4%** | >70% | ✅ |
| internal/app | **73.8%** | >70% | ✅ |
| internal/store/postgres | **62.0%** | >60% | ✅ |
| internal/store/memory | **88.3%** | >85% | ✅ |
| internal/platform/logging | **100%** | — | ✅ |
| internal/service/intent | **100%** | — | ✅ |
| internal/service/handoff | **100%** | — | ✅ |
| internal/platform/health | **100%** | — | ✅ |
| **整体覆盖率** | **77.4%** | >70% | ✅ |

### 4.2 当前门禁结论

| 门禁层级 | 状态 | 说明 |
|---------|------|------|
| 代码级门禁 | ✅ 通过 | `go test ./...`、`go test -race ./...`、`go build ./...` 通过 |
| 预生产门禁 | ⚠️ 未闭环 | 真实环境 DB/migration/webhook/audit/ticket 入库验证仍需证据化 |
| 灰度门禁 | ❌ 未通过 | 鉴权、最小监控、灰度阈值、回滚演练未闭环 |

**当前解释口径**：仓库内测试通过，只能证明现有实现稳定，不等于“PRD 功能已完成”或“可直接灰度发布”。 

### 4.3 安全审计

| 检查项 | 结果 |
|--------|------|
| 硬编码密钥/Token | ✅ 未发现 |
| SQL 注入 | ✅ 参数化查询，无拼接 |
| Audit 写入失败 | ✅ P0 标准：只 log，不阻流 |
| Context 超时 | ✅ 有限 timeout 上下文 |
| 数据竞争 | ✅ gorace 无警告 |

### 4.4 死代码修复记录

**问题**：`auditTicketChange`（ticket_handler.go:104）定义但从未调用，导致 Assign/Resolve/Close 状态变更缺少 handler 层审计。
**修复**：将 `auditTicketChange` 接入 Assign/Resolve/Close 成功后调用，新增 `actorID` 参数。
**Commit**：`3e9022a`

---

## 5. 部署指南

### 5.1 构建 Docker 镜像

```bash
# 项目根目录执行
docker build -t ai-customer-service:v1.0.0 .

# 或使用 Makefile
make test    # 运行测试
make run     # 本地运行（go run）
```

### 5.2 环境变量配置

| 变量 | 说明 | 示例 |
|------|------|------|
| `AI_CS_RUNTIME_ENV` | 运行环境 | `production` |
| `AI_CS_ADDR` | HTTP 监听地址 | `:8080` |
| `AI_CS_POSTGRES_ENABLED` | 是否启用 PostgreSQL store | `true` |
| `AI_CS_POSTGRES_DSN` | PostgreSQL 连接串 | `postgres://ai_cs:***@localhost:5432/ai_customer_service?sslmode=disable` |
| `AI_CS_POSTGRES_MIGRATION_DIR` | migration 目录 | `db/migration` |
| `AI_CS_WEBHOOK_SECRET` | Webhook HMAC 密钥 | — |
| `AI_CS_WEBHOOK_TIMESTAMP_HEADER` | 时间戳请求头 | `X-CS-Timestamp` |
| `AI_CS_WEBHOOK_SIGNATURE_HEADER` | 签名请求头 | `X-CS-Signature` |
| `AI_CS_WEBHOOK_MAX_SKEW_SECONDS` | 最大时钟偏差（秒） | `300` |

### 5.3 数据库初始化

```bash
# 执行 migration（项目 db/ 目录）
psql "$AI_CS_POSTGRES_DSN" -f db/migration/0001_init.up.sql
```

### 5.4 健康检查

```bash
# Readiness（含 DB 依赖检查）
curl http://localhost:8080/actuator/health/ready

# Liveness
curl http://localhost:8080/actuator/health/live

# 综合健康
curl http://localhost:8080/actuator/health
```

---

## 6. 仓库分布

| Remote | 仓库地址 | 分支 | 最新 Commit |
|--------|---------|------|------------|
| GitHub | `https://github.com/phamnazage-jpg/lijiaoqiao` | `upload/ai-customer-service` | `3e9022a` ✅ |
| Gitea | `http://localhost:3000/shenyi/lijiaoqiao` | `upload/ai-customer-service` | `3e9022a` ✅ |
| TKSea | `https://tksea.top/niuniu/lijiaoqiao` | `upload/ai-customer-service` | `3e9022a` ✅ |

**三端已同步。**

---

## 7. 已知限制（P1 后续迭代）

以下功能在本版本未实现，如需请在 P1 中补充：

| 功能 | 优先级 | 说明 |
|------|--------|------|
| 真实多渠道适配器产品化 | P1 | 当前只有统一 webhook 模型与路径覆写 channel |
| 人工回复用户链路 | P1 | 只有工单创建，无回复闭环 |
| 排队位置查询 | P1 | 无此 API |
| 真实 LLM / RAG | P1 | 当前为规则识别 + 静态 FAQ |
| 安全拒绝事件审计（签名失败/非法 body） | P0 | 此类事件暂未写审计 |
| metrics / tracing / SLO | P1 | 暂无可观测基础设施 |
| 灰度/回滚 Runbook | P1 | 需完成演练与证据化验证 |

---

## 8. 关键联系人

- **项目负责人**：小龙团队（Hermes Review 完成）
- **代码基准**：`3e9022a`
- **Phase 2 覆盖率目标**：✅ 已达成（77.4% > 70%）
