# AI-Ops 部署设计

> 版本：v1.0 | 状态：初稿

---

## 1. 部署架构

### 1.1 总体架构

```
├── Load Balancer (Nginx / 云 CLB)
    │
    ├── AI-Ops API Server x 2 (主备)
    │   │
    │   ├── HTTP API (标准库 net/http)
    │   └── WebSocket (告警推送)
    │
    ├── AI-Ops Worker x 2 (后台任务)
    │   │
    │   ├── 指标采集器
    │   ├── 告警评估器
    │   ├── 自愈执行器
    │   └── 审计清理器
    │
    └── 共享层
        │
        ├── PostgreSQL 15+ (主库 + 备库)
        ├── Redis (缓存 + 会话 + 锁)
        ├── Prometheus (时序数据)
        └── Grafana (监控可视化)
```

### 1.2 容器化部署

使用 Docker Compose 或 Kubernetes：

```yaml
# docker-compose.yml 抽象
services:
  ai-ops-api:
    image: ai-ops:latest
    command: ["./ai-ops", "api"]
    replicas: 2
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=postgres
      - REDIS_HOST=redis
      - PROMETHEUS_HOST=prometheus

  ai-ops-worker:
    image: ai-ops:latest
    command: ["./ai-ops", "worker"]
    replicas: 2
    environment:
      - DB_HOST=postgres
      - REDIS_HOST=redis
      - PROMETHEUS_HOST=prometheus

  postgres:
    image: postgres:15
    volumes:
      - pg_data:/var/lib/postgresql/data

  redis:
    image: redis:7

  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml

  grafana:
    image: grafana/grafana:latest
```

---

## 2. 资源需求

### 2.1 API Server

| 资源 | 需求 | 说明 |
|------|------|------|
| CPU | 2 核 | Go 服务主要为 IO 密集型 |
| 内存 | 1 GB | 含连接池缓存 |
| 存储 | 无 | 状态外部化 |
| 网络 | 内网 100Mbps | 调用内部服务 |

### 2.2 Worker

| 资源 | 需求 | 说明 |
|------|------|------|
| CPU | 1 核 | 定时任务，CPU 需求低 |
| 内存 | 512 MB | |
| 存储 | 无 | |

### 2.3 数据库

| 资源 | 需求 | 说明 |
|------|------|------|
| CPU | 2 核 | |
| 内存 | 4 GB | 索引与缓冲 |
| 存储 | 200 GB | 90 天审计日志 + 时序数据 |
| 网络 | 内网 1Gbps | |

### 2.4 Prometheus

| 资源 | 需求 | 说明 |
|------|------|------|
| CPU | 1 核 | |
| 内存 | 2 GB | |
| 存储 | 100 GB | 时序数据保留 90 天 |

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
3. Prometheus 启动完成
4. Worker 启动（执行 migration）
5. API Server 启动

**关闭顺序**:
1. 停止接收新 HTTP 请求（健康检查返回非 200）
2. 等待现有请求处理完成（超时 30 秒）
3. 停止 Worker 定时器
4. 关闭数据库连接池
5. 退出进程

### 3.3 配置管理

- 配置文件 `config.yaml` + 环境变量覆盖。
- 敏感字段（密钥、密码）仅通过环境变量传入，不落地配置文件。
- 支持热更新的配置：告警规则、通知渠道。

---

## 4. 灾备设计

### 4.1 数据库灾备

| 策略 | 方案 | RTO | RPO |
|------|------|-----|-----|
| 主库故障 | 自动切换至备库 | < 5 min | < 1 min |
| 逻辑损坏 | 从备库恢复 + 审计日志回放 | < 30 min | < 1 min |
| 全库损坏 | 每日冷备份恢复 | < 2 h | < 24 h |

### 4.2 应用层灾备

| 场景 | 处理 |
|------|------|
| API Server 单机故障 | 负载均衡自动移除，剩余节点继续服务 |
| Worker 单机故障 | 剩余 Worker 继续执行定时任务，某些任务可能延迟 |
| Redis 故障 | 审计日志落地 PostgreSQL，告警缓存失效不影响核心功能 |
| Prometheus 故障 | 实时指标采集中断，告警引擎依赖本地缓存继续运行 |

### 4.3 多中心部署

- 当前阶段为单中心部署。
- 备份中心仅用于数据库备份恢复，不提供活跃服务。
- 未来扩展至多中心时，需要解决 PostgreSQL 的分布式写入和 Prometheus 的联邦查询问题。
