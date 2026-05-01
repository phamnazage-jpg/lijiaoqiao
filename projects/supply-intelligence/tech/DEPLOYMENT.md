# Supply-Intelligence 部署设计

> 版本：v1.0 | 状态：初稿

---

## 1. 部署架构

### 1.1 总体架构

```
├── Load Balancer (Nginx / 云 CLB)
    │
    ├── Supply-Intelligence API Server x 2
    │   │
    │   ├── HTTP API
    │   └── WebSocket (健康大盘实时推送)
    │
    ├── Supply-Intelligence Worker x 3
    │   │
    │   ├── Probe Worker (探针任务)
    │   ├── Discovery Worker (扫描任务)
    │   ├── Admission Worker (准入测试任务)
    │   ├── Auto-Reg Worker (自动注册任务)
    │   └── Cleanup Worker (定期清理)
    │
    └── 共享层
        │
        ├── PostgreSQL 15+ (与 supply-api 共存或独立)
        ├── Redis (缓存 + 锁 + 扫描结果缓存)
        └── 向量数据库 (PGVector / Milvus / Qdrant)
```

### 1.2 容器化部署

```yaml
services:
  supply-intel-api:
    image: supply-intelligence:latest
    command: ["./supply-intel", "api"]
    replicas: 2
    ports:
      - "8081:8080"

  supply-intel-probe:
    image: supply-intelligence:latest
    command: ["./supply-intel", "worker", "probe"]
    replicas: 1

  supply-intel-discovery:
    image: supply-intelligence:latest
    command: ["./supply-intel", "worker", "discovery"]
    replicas: 1

  supply-intel-admission:
    image: supply-intelligence:latest
    command: ["./supply-intel", "worker", "admission"]
    replicas: 1

  supply-intel-autoreg:
    image: supply-intelligence:latest
    command: ["./supply-intel", "worker", "autoreg"]
    replicas: 1
```

---

## 2. 资源需求

### 2.1 API Server

| 资源 | 需求 | 说明 |
|------|------|------|
| CPU | 1 核 | |
| 内存 | 512 MB | |
| 存储 | 无 | |

### 2.2 Worker

| Worker 类型 | CPU | 内存 | 说明 |
|------------|-----|--------|------|
| Probe | 1 核 | 512 MB | 同时发起多个 HTTP 请求 |
| Discovery | 1 核 | 1 GB | 可能涉及 Playwright 爬取 |
| Admission | 2 核 | 2 GB | 测试流水线调用 LLM API，CPU 与内存需求较高 |
| Auto-Reg | 1 核 | 512 MB | |

### 2.3 数据库

| 资源 | 需求 | 说明 |
|------|------|------|
| CPU | 2 核 | |
| 内存 | 4 GB | |
| 存储 | 100 GB | 探针历史 + 审计日志 + 定价数据库 |

### 2.4 向量数据库

| 选型 | CPU | 内存 | 存储 | 说明 |
|------|-----|--------|------|------|
| PGVector | 与 PostgreSQL 共存 | 共存 | 共存 | 推荐，无需额外部署 |
| Milvus | 2 核 | 4 GB | 50 GB | 高性能、分布式 |
| Qdrant | 1 核 | 2 GB | 30 GB | 轻量、Cloud-native |

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
1. 停止接收新 HTTP 请求
2. 等待现有请求处理完成（超时 30 秒）
3. 停止各 Worker 定时器
4. 关闭数据库连接池
5. 退出进程

### 3.3 配置管理

- 配置文件 `config.yaml` + 环境变量覆盖。
- 供应商 API Key 仅通过环境变量传入。
- 探针周期、扫描周期、测试用例集路径等可热更新。

---

## 4. 灾备设计

### 4.1 数据库灾备

| 策略 | 方案 | RTO | RPO |
|------|------|-----|-----|
| 主库故障 | 自动切换至备库 | < 5 min | < 1 min |
| 逻辑损坏 | 从备库恢复 + 审计日志回放 | < 30 min | < 1 min |

### 4.2 扫描/测试任务灾备

| 场景 | 处理 |
|------|------|
| Discovery Worker 故障 | 下一周期自动恢复，扫描任务无状态，不影响生产 |
| Admission Worker 故障 | 测试任务缓存在 Redis，恢复后继续执行 |
| Probe Worker 故障 | 探针任务缓存在 Redis，恢复后继续执行 |
| 向量数据库故障 | 知识库检索降级为文本匹配，不影响核心探针功能 |

### 4.3 多中心部署

- 当前阶段为单中心部署。
- 探针任务无状态，不依赖中心化调度。
- 未来扩展至多中心时，需要解决 PostgreSQL 分布式写入和向量数据库的同步问题。
