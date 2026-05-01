# AI-Customer-Service 部署设计

> 版本：v1.0 | 状态：初稿

---

## 1. 部署架构

### 1.1 总体架构

```
├── Load Balancer (Nginx / 云 CLB)
    │
    ├── AI-CS API Server x 2
    │   │
    │   ├── HTTP API
    │   └── WebSocket (实时对话)
    │
    ├── AI-CS Worker x 2
    │   │
    │   ├── 知识库索引更新 Worker
    │   └── 清理 Worker (过期会话清理)
    │
    └── 共享层
        │
        ├── PostgreSQL 15+ (独立 schema: cs_*)
        ├── Redis (会话 + 缓存 + 锁 + 频率限制)
        └── 向量数据库 (PGVector / Milvus / Qdrant)
```

### 1.2 容器化部署

```yaml
services:
  ai-cs-api:
    image: ai-customer-service:latest
    command: ["./ai-cs", "api"]
    replicas: 2
    ports:
      - "8082:8080"
    environment:
      - DB_HOST=postgres
      - REDIS_HOST=redis
      - VECTOR_DB_HOST=pgvector

  ai-cs-worker:
    image: ai-customer-service:latest
    command: ["./ai-cs", "worker"]
    replicas: 2
    environment:
      - DB_HOST=postgres
      - REDIS_HOST=redis
      - VECTOR_DB_HOST=pgvector

  postgres:
    image: postgres:15
    volumes:
      - pg_data:/var/lib/postgresql/data

  redis:
    image: redis:7

  pgvector:
    image: ankane/pgvector:latest
    # 或使用独立 Milvus/Qdrant 容器
```

---

## 2. 资源需求

### 2.1 API Server

| 资源 | 需求 | 说明 |
|------|------|------|
| CPU | 2 核 | 含意图识别、知识库检索、LLM 调用 |
| 内存 | 2 GB | 连接池 + 向量检索缓存 |
| 存储 | 无 | |
| 网络 | 内网 100Mbps | 调用 LLM API、内部服务 |

### 2.2 Worker

| 资源 | 需求 | 说明 |
|------|------|------|
| CPU | 1 核 | |
| 内存 | 1 GB | 知识库索引更新时需要 |
| 存储 | 无 | |

### 2.3 数据库

| 资源 | 需求 | 说明 |
|------|------|------|
| CPU | 2 核 | |
| 内存 | 4 GB | 索引与缓冲 |
| 存储 | 100 GB | 会话 + 消息 + 工单 + 审计日志 |

### 2.4 向量数据库

| 选型 | CPU | 内存 | 存储 | 说明 |
|------|-----|--------|------|------|
| PGVector | 与 PostgreSQL 共存 | 共存 | 共存 | 推荐，无需额外部署 |
| Milvus | 2 核 | 4 GB | 30 GB | 高性能、分布式 |
| Qdrant | 1 核 | 2 GB | 20 GB | 轻量、Cloud-native |

---

## 3. 监控与运维钩子

### 3.1 健康检查

| 端点 | 路径 | 预期响应 | 失败行为 |
|------|------|----------|---------|
| 存活检查 | `/actuator/health/live` | HTTP 200 | 容器重启 |
| 就绪检查 | `/actuator/health/ready` | HTTP 200 | 从负载均衡移除 |
| 综合检查 | `/actuator/health` | HTTP 200 + JSON | 触发告警 |

### 3.2 启动/关闭顺序

**启动顺序**:
1. PostgreSQL 启动完成
2. Redis 启动完成
3. 向量数据库启动完成
4. Worker 启动（执行 migration）
5. API Server 启动

**关闭顺序**:
1. 停止接收新 HTTP 请求和 WebSocket 连接
2. 等待现有请求处理完成（超时 30 秒）
3. 停止 Worker
4. 关闭数据库连接池
5. 退出进程

### 3.3 配置管理

- 配置文件 `config.yaml` + 环境变量覆盖。
- LLM API Key 仅通过环境变量传入。
- 模型供应商配置、意图置信度阈值、转人工触发条件等可热更新。

---

## 4. 灾备设计

### 4.1 数据库灾备

| 策略 | 方案 | RTO | RPO |
|------|------|-----|-----|
| 主库故障 | 自动切换至备库 | < 5 min | < 1 min |
| 逻辑损坏 | 从备库恢复 + 审计日志回放 | < 30 min | < 1 min |

### 4.2 应用层灾备

| 场景 | 处理 |
|------|------|
| API Server 单机故障 | 负载均衡自动移除，剩余节点继续服务 |
| LLM 主供应商故障 | 5 秒内切换至备用供应商 |
| 双 LLM 故障 | 返回兑底回复 + 自动生成工单 |
| Redis 故障 | 会话状态丢失，用户需要重新发起会话（接受） |
| 向量数据库故障 | 知识库检索降级为关键词匹配，不影响核心对话 |
| 数据库连接池耗尽 | 进入降级模式：仅返回静态 FAQ 链接 |

### 4.3 多中心部署

- 当前阶段为单中心部署。
- 未来扩展至多中心时，需要解决 PostgreSQL 分布式写入、Redis 主从同步和 WebSocket 连接的跨中心问题。
