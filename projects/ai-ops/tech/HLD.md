# AI-Ops 智能运维系统 — 高层设计文档 (HLD)

> 版本：v1.0
> 负责人：TechLead
> 目标读者：后端开发、SRE、QA
> 状态：初稿

---

## 1. 设计目标与约束

### 1.1 核心目标

| 指标 | 基线值 | 目标值 | 验证方式 |
|------|--------|--------|---------|
| 核心故障 MTTR | >30 min | <10 min | 从告警触发到服务恢复的 P99 时长 |
| P1/P2 自动化处理覆盖率 | 0% | >=60% | 自愈成功事件数 / (P1+P2 总事件数) |
| 告警噪声率 | >20% | <5% | 误告警数 / 总告警数 |
| 配置回滚时间窗口 | 无 | <5 min | 回滚指令发出到验证通过的时长 |
| 审计日志保留期 | 无 | >=90 天 | 存储系统自动清理策略 |

### 1.2 技术约束（强制性）

- **语言**: Go 1.22+
- **HTTP 框架**: 标准库 `net/http` + 自定义中间件（禁止引入 Gin/Echo）
- **数据库**: PostgreSQL 15+ ，驱动 `jackc/pgx/v5`
- **缓存**: Redis，客户端 `redis/go-redis/v9`
- **配置**: YAML + Viper，环境变量覆盖敏感字段
- **日志/审计**: 结构化日志，审计事件模型与 supply-api/ 一致
- **错误码**: `{SOURCE}_{CATEGORY}_{CODE}` 格式，例如 `OPS_ALT_4001`
- **健康检查**: `/actuator/health`、`/actuator/health/live`、`/actuator/health/ready`
- **测试**: Go testing + testify，覆盖率门槛 domain >= 70%、service/handler >= 80%
- **Store 接口**: 必须包含版本控制（乐观锁）
- **条件能力**: 默认关闭，需要在 `BuildServer` / `BuildRuntime` 中显式挂载才算已交付

### 1.3 运行模式

系统必须同时支持两种运行模式：

| 模式 | 特征 | 部署方式 |
|------|------|---------|
| **独立运行** | 自有 `cmd/ai-ops/main.go`，独立数据库 schema，独立 docker-compose | `docker-compose up` 或单独容器 |
| **集成运行** | 作为 Go module 被 `gateway/` 或 `supply-api/` 引入，共享数据库连接池和配置，通过内部接口注册 | 编译时作为子模块编译，运行时挂载到立交桥主进程 |

**集成约束**：
- 独立运行时，系统提供完整的 HTTP API 和管理后台。
- 集成运行时，系统提供 `IntegrationPlugin` 接口，允许主程序通过配置开关启用/禁用各模块。
- 数据库 schema 必须使用独立的 `ai_ops_` 前缀，避免与主项目表名冲突。
- 配置文件必须支持分离加载：独立运行时读取自己的 `config.yaml`，集成运行时合并到主项目配置。

---

## 2. 系统架构总览

### 2.1 逻辑架构图

```
+---------------------+     +---------------------+     +---------------------+
|   运维控制台 (Web)    |     |   外部系统调用者    |     |   通知渠道        |
|  - 监控看板          |     |  - NewAPI/Sub2API   |     |  - Webhook          |
|  - 告警管理          |<--->|  - 企业微信/飞书    |<--->|  - 邮件            |
|  - 日志查询          |     |  - Prometheus       |     |  - 短信            |
+----------+----------+     +----------+----------+     +----------+----------+
           |                           |                           |
           v                           v                           v
+---------------------+     +---------------------+     +---------------------+
|   HTTP API Layer    |     |   /metrics (Prom)   |     |   Notification      |
|  (标准库 net/http)  |     |   /api/v1/ai-ops/   |     |   Dispatcher        |
+----------+----------+     +----------+----------+     +----------+----------+
           |                           |                           |
           v                           v                           v
+-----------------------------------------------------------------------------+
|                         AI-Ops Core Domain Layer                            |
|  +----------------+  +----------------+  +----------------+  +-----------+  |
|  | Metric Service |  | Alert Service  |  | Healing Engine |  | Capacity  |  |
|  | (指标采集/查询)  |  | (告警规则/触发) |  | (自愈动作执行)   |  | Service   |  |
|  +----------------+  +----------------+  +----------------+  +-----------+  |
|  +----------------+  +----------------+  +----------------+  +-----------+  |
|  | Audit Service  |  | Config Service |  | Log Service    |  | Authz     |  |
|  | (审计/回滚)    |  | (配置变更)     |  | (日志查询)     |  | Service   |  |
|  +----------------+  +----------------+  +----------------+  +-----------+  |
+-----------------------------------------------------------------------------+
                                    |
                                    v
+-----------------------------------------------------------------------------+
|                         Infrastructure Layer                                |
|  +----------------+  +----------------+  +----------------+  +-----------+  |
|  | Metric Store   |  | PostgreSQL     |  | Redis          |  | Time-Series|  |
|  | (Prom/Victoria)|  | (主审计/配置)  |  | (缓存/状态)   |  | DB         |  |
|  +----------------+  +----------------+  +----------------+  +-----------+  |
+-----------------------------------------------------------------------------+
                                    |
                                    v
+-----------------------------------------------------------------------------+
|                         Bridge Integration Layer                            |
|  +----------------+  +----------------+  +----------------+  +-----------+  |
|  | Token Gateway  |  | Channel Manager|  | Provider Health|  | Runtime   |  |
|  | (请求量/延迟)  |  | (供应商/路由)  |  | (健康检查)     |  | Status    |  |
|  +----------------+  +----------------+  +----------------+  +-----------+  |
+-----------------------------------------------------------------------------+
```

### 2.2 服务边界与职责

| 服务 | 职责 | 对应 PRD 场景 | 对应 AC |
|------|------|--------------|---------|
| **Metric Service** | 采集 gateway/、supply-api/、platform-token-runtime/ 的指标，提供 PromQL 查询、分钟级聚合 | A, H | AC-1, AC-2, AC-11 |
| **Alert Service** | 维护告警规则状态机，执行阈值评估，生成告警事件，负责聚合与抑制 | C, E, G | AC-3, AC-4, AC-5 |
| **Healing Engine** | 执行自愈动作：切换备用路由、限流、重启实例、触发脚本；记录执行结果 | C, D, F | AC-6 |
| **Audit Service** | 捕获所有配置变更，写入不可篡改审计日志，支持按原始操作记录回滚 | B, F, I | AC-7, AC-8 |
| **Config Service** | 管理告警规则、通知渠道、自愈策略的 CRUD，支持版本化与验证 | B, I | AC-7, AC-8 |
| **Log Service** | 按时间范围、服务、状态码、用户 ID 等维度筛选日志，支持 CSV 导出 | A, H | AC-10 |
| **Capacity Service** | 汇总过去 7 天 Token/QPS/延迟/利用率趋势，计算负载等级与增长率预测 | - | AC-9 |
| **Authz Service** | 角色鉴权：查看者/运维人员/管理员；控制台访问控制 | - | AC-12 |
| **Notification Dispatcher** | 将告警事件路由到配置的通知渠道，支持主备自动切换 | C, E | AC-4, AC-5 |

---

## 3. 核心模块设计

### 3.1 自动运维流水线 (AutoOps Pipeline)

运维流水线是系统的主干，接收指标数据，经过规则引擎评估，生成告警事件，触发自愈动作，并验证效果。

```
指标数据流
   |
   v
+-------------------+     +-------------------+     +-------------------+
| Metric Ingestor   | --> | Rule Engine       | --> | Alert Event       |
| (报文解析/格式化)   |     | (阈值评估/分级)   |     | Generator         |
+-------------------+     +-------------------+     +---------+---------+
                                                           |
                                                           v
+-------------------+     +-------------------+     +-------------------+
| Validation Loop   | <-- | Healing Engine    | <-- | Notification      |
| (2min 效果评估)   |     | (自愈动作执行)     |     | Dispatcher        |
+-------------------+     +-------------------+     +-------------------+
```

**流水线状态机**：

| 状态 | 转移条件 | 超时 |
|------|---------|------|
| `triggered` | 规则阈值被触发 | - |
| `notified` | 通知已发送 | 30s (P0/P1), 120s (P2) |
| `healing` | 自愈动作执行中 | 60s 内完成 |
| `resolved` | 监控指标回复正常 | - |
| `escalated` | 自愈失败或未配置自愈 | 立即 |
| `acknowledged` | 人工确认 | 2h 未确认则自动升级 |

### 3.2 健康探针 (Health Probe)

参考 LiteLLM 的多层级健康检查设计，对于集成运行模式提供以下端点：

| 端点 | 用途 | 检查内容 | 失败策略 |
|------|------|---------|---------|
| `/actuator/health` | 综合健康 | DB、Redis、时序库连接性 | 返回 503，触发内部告警 |
| `/actuator/health/live` | 存活探针 | 进程是否运行 | Kubernetes 重启 Pod |
| `/actuator/health/ready` | 就绪探针 | 所有依赖是否可服务 | 从负载均衡移除 |
| `/actuator/health/backlog` | 队列积压 | 告警事件队列长度 | >100 时触发内部告警 |
| `/actuator/health/datasource` | 数据源状态 | 最近 5min 内是否有新数据点 | 触发 P2 内部告警 |

独立运行时，系统自身提供以上端点。集成运行时，通过 `IntegrationPlugin` 将检查逻辑注入到主程序的健康检查中。

### 3.3 异常自动恢复 (Healing Engine)

自愈引擎的核心是动作执行器。每个动作是一个独立的可执行单元，支持沙盘模式验证。

**自愈动作类型**：

| 动作 | 说明 | 执行时间限制 | 回退策略 |
|------|------|-----------|---------|
| `switch_route` | 将流量从主路由切换到备用路由 | 30s | 自动恢复原路由，升级人工告警 |
| `throttle` | 对目标服务/供应商启动限流 | 15s | 解除限流，升级人工告警 |
| `restart_instance` | 重启异常实例（通过调用管理 API） | 45s | 不可回退，升级人工告警 |
| `invoke_script` | 执行用户配置的程序化脚本 | 60s | 脚本自身决定回退逻辑 |
| `isolate_node` | 将异常节点从负载均衡中移除 | 20s | 恢复节点到负载均衡 |

**沙盘模式**：
- 所有自愈动作必须在沙盒环境中模拟触发 >=10 次，所有次数的执行结果符合预期，才能关联到生产告警规则。
- 沙盒模式下，动作不会真正修改生产状态，而是记录 "dry-run" 结果。
- 每个动作的沙盒执行结果必须包含：预期变更、实际变更、差异说明、风险标记。

**级联故障防护**（对应 PRD 场景 F-6）：
- 每次自愈动作执行前，系统记录当前状态快照（包含相关配置版本号）。
- 若自愈动作执行后 2min 内触发新的 P1 以上告警，系统自动检测是否为级联故障。
- 检测到级联故障时，自动回退上一步操作，然后升级为 P0 人工告警。

### 3.4 规模调度与容量视图 (Capacity Board)

容量服务不执行自动扩缩容决策（当前版本 Out of Scope），仅提供量化视图与趋势预测。

**容量指标**：

| 指标 | 采集频率 | 保留时长 | 负载等级判定 |
|------|---------|---------|-----------|
| Token 消耗量 | 1 min | 7 天(原始) / 30 天(分钟级) / 90 天(小时级) | 超过日上限 80% 为警告，100% 为过载 |
| QPS | 1 min | 同上 | 超过设计值 80% 为警告，100% 为过载 |
| P99 延迟 | 1 min | 同上 | 超过 5000ms 为警告，超过 10000ms 为过载 |
| 供应商资源利用率 | 5 min | 同上 | 超过 80% 为警告，超过 95% 为过载 |

**增长率预测算法**：
- 采用简单线性回归，基于过去 7 天的分钟级数据计算日均增长率。
- 计算公式：`days_to_limit = (limit - current) / daily_growth`，其中 `daily_growth = (latest - earliest) / 7`。
- ⚠️ **免责声明**：结果仅供**参考，不作为扩容决策依据**。线性回归无法捕捉季节性波动和突增流量（如大促、热点事件），实际容量规划应以人工判断为主。
- 建议在 UI 界面上也同步显示同样免责声明，控制台显示为 "预计 X 天达到上限（仅供参考，不作为扩容决策依据）"。

### 3.5 知识库管理 (审计与回滚)

审计服务是运维系统的可信基础。所有生产配置变更必须被捕获并不可篡改地存储。

**审计事件模型**（与 supply-api/ 审计规范一致）：

```go
type AuditEvent struct {
    EventID     string         `json:"event_id"`
    TenantID    string         `json:"tenant_id"`        // 工作区 ID
    ObjectType  string         `json:"object_type"`      // 例如 "alert_rule", "route_policy"
    ObjectID    string         `json:"object_id"`
    Action      string         `json:"action"`           // "create", "update", "delete", "rollback"
    BeforeState map[string]any `json:"before_state"`
    AfterState  map[string]any `json:"after_state"`
    RequestID   string         `json:"request_id"`
    ResultCode  string         `json:"result_code"`      // "OK", "OPS_AUD_4001"
    SourceIP    string         `json:"source_ip"`
    ActorID     string         `json:"actor_id"`         // 操作人 ID
    CreatedAt   time.Time      `json:"created_at"`
}
```

**高风险变更检测**（对应 PRD 场景 I）：
- 对于每次配置变更，系统计算 "影响面分数"。
- 影响面计算方式：变更后将导致被拒绝的请求占比。若估算拒绝率 > 50%，标记为高风险。
- 高风险变更在执行前必须弹出二次确认窗口，管理员角色才能继续。

**回滚机制**：
- 回滚操作不是简单的 "恢复原值"，而是一个新的审计事件（Action="rollback"），生成新的版本。
- 回滚前必须检查目标资源是否仍然存在。若不存在，返回错误码 `OPS_AUD_4101` (对应 PRD 中的 `AUDIT_ROLLBACK_TARGET_LOST`)。
- 回滚执行前必须显示将被覆盖的子资源列表，并要求管理员二次确认。
- 回滚必须在 60s 内完成并通过验证。

---

## 4. 数据模型设计

### 4.1 核心实体关系图 (ER)

```
+----------------+       +----------------+       +----------------+
| ai_ops_rules   |<----->| ai_ops_alerts  |<----->| ai_ops_healings|
+----------------+       +----------------+       +----------------+
        |                        |                         |
        |                        v                         |
        |               +----------------+                 |
        |               | ai_ops_events  |                 |
        |               +----------------+                 |
        |                        ^                         |
        v                        |                         v
+----------------+       +----------------+       +----------------+
| ai_ops_channels|<----->| ai_ops_notifys |       | ai_ops_snapshots|
+----------------+       +----------------+       +----------------+
        |
        v
+----------------+       +----------------+       +----------------+
| ai_ops_audits  |       | ai_ops_configs |       | ai_ops_metrics |
+----------------+       +----------------+       +----------------+
```

### 4.2 数据表结构

#### 4.2.1 `ai_ops_rules` — 告警规则

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | UUID | PK, 默认 gen_random_uuid() | 规则唯一标识 |
| `name` | VARCHAR(128) | NOT NULL, UNIQUE | 规则名称 |
| `metric_source` | VARCHAR(64) | NOT NULL | 指标来源：gateway/supply-api/platform-token-runtime |
| `metric_name` | VARCHAR(128) | NOT NULL | 指标名称：qps/latency_p99/error_rate/… |
| `threshold_type` | VARCHAR(16) | NOT NULL, CHECK IN ('>', '<', '=', 'regex') | 阈值类型 |
| `threshold_value` | TEXT | NOT NULL | 阈值（支持正则表达式） |
| `duration_min` | INT | NOT NULL, DEFAULT 1, CHECK >=1 | 持续触发时长（分钟） |
| `level` | VARCHAR(8) | NOT NULL, CHECK IN ('P0','P1','P2','P3') | 告警级别 |
| `channel_ids` | UUID[] | NOT NULL, DEFAULT '{}' | 关联通知渠道 ID 列表 |
| `healing_action` | VARCHAR(32) | DEFAULT NULL | 自愈动作类型（可选） |
| `healing_config` | JSONB | DEFAULT NULL | 自愈动作参数 |
| `is_sandboxed` | BOOLEAN | NOT NULL, DEFAULT FALSE | 是否已通过沙盒验证 |
| `enabled` | BOOLEAN | NOT NULL, DEFAULT TRUE | 是否启用 |
| `created_by` | VARCHAR(64) | NOT NULL | 创建人 |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | 创建时间 |
| `updated_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | 更新时间 |
| `version` | INT | NOT NULL, DEFAULT 1 | 乐观锁版本 |

**索引**：`CREATE INDEX idx_rules_enabled ON ai_ops_rules(enabled);`

#### 4.2.2 `ai_ops_alerts` — 告警事件

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | UUID | PK | 告警事件 ID |
| `rule_id` | UUID | NOT NULL, FK -> ai_ops_rules | 触发规则 |
| `level` | VARCHAR(8) | NOT NULL | 告警级别（可能升级） |
| `resource_type` | VARCHAR(64) | NOT NULL | 资源类型：service/provider/model |
| `resource_id` | VARCHAR(128) | NOT NULL | 资源标识 |
| `current_value` | TEXT | NOT NULL | 触发时的实际值 |
| `threshold_value` | TEXT | NOT NULL | 触发时的阈值 |
| `status` | VARCHAR(16) | NOT NULL, DEFAULT 'triggered' | triggered/notified/healing/resolved/escalated/acknowledged |
| `is_aggregated` | BOOLEAN | NOT NULL, DEFAULT FALSE | 是否为聚合告警 |
| `aggregated_count` | INT | DEFAULT 0 | 聚合的子告警数量 |
| `parent_alert_id` | UUID | NULL, FK -> ai_ops_alerts | 父聚合告警 ID |
| `started_at` | TIMESTAMPTZ | NOT NULL | 开始时间 |
| `resolved_at` | TIMESTAMPTZ | NULL | 解除时间 |
| `acknowledged_by` | VARCHAR(64) | NULL | 确认人 |
| `acknowledged_at` | TIMESTAMPTZ | NULL | 确认时间 |

**索引**：
```sql
CREATE INDEX idx_alerts_status ON ai_ops_alerts(status);
CREATE INDEX idx_alerts_started_at ON ai_ops_alerts(started_at DESC);
CREATE INDEX idx_alerts_resource ON ai_ops_alerts(resource_type, resource_id);
```

#### 4.2.3 `ai_ops_healings` — 自愈执行记录

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | UUID | PK | 自愈执行 ID |
| `alert_id` | UUID | NOT NULL, FK -> ai_ops_alerts | 关联告警 |
| `action_type` | VARCHAR(32) | NOT NULL | switch_route/throttle/restart_instance/invoke_script/isolate_node |
| `config` | JSONB | NOT NULL | 执行时的参数快照 |
| `status` | VARCHAR(16) | NOT NULL, DEFAULT 'pending' | pending/succeeded/failed/rolled_back |
| `dry_run` | BOOLEAN | NOT NULL, DEFAULT FALSE | 是否沙盒执行 |
| `result_detail` | JSONB | NULL | 执行结果详情 |
| `error_code` | VARCHAR(16) | NULL | 失败时的错误码 |
| `started_at` | TIMESTAMPTZ | NOT NULL | 开始时间 |
| `completed_at` | TIMESTAMPTZ | NULL | 完成时间 |

#### 4.2.4 `ai_ops_channels` — 通知渠道

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | UUID | PK | 渠道 ID |
| `name` | VARCHAR(128) | NOT NULL | 渠道名称 |
| `channel_type` | VARCHAR(32) | NOT NULL, CHECK IN ('webhook','email','feishu','wechat','sms') | 渠道类型 |
| `config` | JSONB | NOT NULL | 渠道配置（URL/密钥/接收人等） |
| `priority` | INT | NOT NULL, DEFAULT 1 | 优先级（低数 = 高优先） |
| `enabled` | BOOLEAN | NOT NULL, DEFAULT TRUE | 是否启用 |
| `created_at` | TIMESTAMPTZ | NOT NULL | 创建时间 |

#### 4.2.5 `ai_ops_audits` — 审计日志

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | UUID | PK | 审计事件 ID |
| `tenant_id` | VARCHAR(64) | NOT NULL | 工作区 ID |
| `object_type` | VARCHAR(64) | NOT NULL | 目标资源类型 |
| `object_id` | VARCHAR(128) | NOT NULL | 目标资源 ID |
| `action` | VARCHAR(32) | NOT NULL | create/update/delete/rollback |
| `before_state` | JSONB | NULL | 变更前状态 |
| `after_state` | JSONB | NULL | 变更后状态 |
| `request_id` | VARCHAR(64) | NOT NULL | HTTP 请求 ID |
| `result_code` | VARCHAR(16) | NOT NULL | OK 或错误码 |
| `source_ip` | VARCHAR(45) | NOT NULL | 操作人 IP |
| `actor_id` | VARCHAR(64) | NOT NULL | 操作人 ID |
| `risk_level` | VARCHAR(8) | NOT NULL, DEFAULT 'normal' | normal/high/critical |
| `parent_audit_id` | UUID | NULL, FK -> ai_ops_audits | 回滚时关联原始审计 |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | 创建时间 |

**索引**：
```sql
CREATE INDEX idx_audits_tenant_created ON ai_ops_audits(tenant_id, created_at DESC);
CREATE INDEX idx_audits_object ON ai_ops_audits(object_type, object_id);
CREATE INDEX idx_audits_actor ON ai_ops_audits(actor_id, created_at DESC);
CREATE INDEX idx_audits_request ON ai_ops_audits(request_id);
```

#### 4.2.6 `ai_ops_metrics` — 时序指标缓存

该表仅在未接入独立时序数据库时作为落地缓存，主时序数据仍然推荐存储在 Prometheus/VictoriaMetrics 中。

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | BIGSERIAL | PK | 自增 ID |
| `metric_name` | VARCHAR(128) | NOT NULL | 指标名称 |
| `labels` | JSONB | NOT NULL, DEFAULT '{}' | 标签（service/path/supplier 等） |
| `value` | DOUBLE PRECISION | NOT NULL | 指标值 |
| `recorded_at` | TIMESTAMPTZ | NOT NULL | 采集时间 |

**索引**：`CREATE INDEX idx_metrics_name_time ON ai_ops_metrics(metric_name, recorded_at DESC);`

**分区策略**：按 `recorded_at` 分区，每日一个分区，自动删除 > 7 天的分区。

### 4.3 实体关系说明

- **Rule -> Alert** (1:N)：一条规则在不同时间可触发多个告警事件。
- **Alert -> Healing** (1:1)：每个告警事件最多执行一次自愈动作（失败后升级人工处理）。
- **Alert -> Alert** (1:N, 聚合)：父告警聚合多个子告警。
- **Audit -> Audit** (1:1, 回滚)：回滚审计记录通过 `parent_audit_id` 关联原始记录。
- **Rule -> Channel** (N:M)：通过 `channel_ids` 数组实现多对多关系。

---

## 5. 关键流程设计

### 5.1 异常检测 → 诊断 → 恢复 → 验证 → 回复

```
  Metric Ingestor          Rule Engine           Alert Service         Healing Engine        Validation Loop
       |                        |                       |                      |                    |
       | 1. 推送指标数据          |                       |                      |                    |
       |---------------------->|                       |                      |                    |
       |                        | 2. 评估阈值规则        |                      |                    |
       |                        |---------------------->|                      |                    |
       |                        |                       | 3. 生成告警事件        |                      |
       |                        |                       |--------------------->|                      |
       |                        |                       | 4. 检查自愈配置         |                      |
       |                        |                       |--------------------->|                      |
       |                        |                       |                      | 5. 执行自愈动作      |
       |                        |                       |                      |--------------------->|
       |                        |                       |                      | 6. 记录执行结果     |
       |                        |                       |<---------------------|                      |
       |                        |                       | 7. 发送通知            |                      |
       |                        |                       |------------------------------------------------>|
       |                        |                       |                      |                      | 8. 2min 后验证
       |                        |                       |                      |<---------------------|
       |                        |                       | 9a. 解除告警          |                      |
       |                        |                       |<---------------------|                      |
       |                        |                       | 9b. 升级人工告警      |                      |
       |                        |                       |<---------------------|                      |
```

**流程说明**：

1. **指标采集** (<=15s): Metric Ingestor 每 15s 拉取一次 Prometheus 数据，或通过 Pushgateway 接收推送数据。
2. **规则评估** (<=5s): Rule Engine 对每个启用的规则评估阈值条件。触发条件时，检查是否已在当前持续时间窗口内已存在未关闭的同类告警（抑制重复触发）。
3. **告警生成** (<=1s): 创建 Alert 记录，状态为 `triggered`。
4. **自愈检查** (<=1s): 检查规则是否配置了自愈动作，且已通过沙盒验证。
5. **自愈执行** (<=60s): 执行自愈动作，包含最多 1 次重试。
6. **结果记录** (<=1s): 将自愈执行结果写入 Healing 表，更新 Alert 状态为 `healing`。
7. **通知发送** (P0/P1 <=30s, P2 <=120s): Notification Dispatcher 路由到配置的通知渠道。
8. **效果验证** (2min 后): Validation Loop 查询监控指标，检查告警条件是否仍然满足。
9. **终态处理**:
   - 9a. 若指标恢复正常，Alert 状态变为 `resolved`。
   - 9b. 若指标仍未恢复，Alert 状态变为 `escalated`，通知升级为 P0 人工告警。

### 5.2 告警聚合流程

```
Alert Service
      |
      | 1. 检测到新告警
      v
+-----------+     +----------------+     +----------------+
| 同一资源  | --> | 1min 内数量 >20 | --> | 生成集群告警    |
| 在 1min   |     | 条?             |     | (is_aggregated) |
| 内的告警  |     +----------------+     +--------+-------+
+-----------+                                    |
                                                 | 2. 将子告警关联到父告警
                                                 v
                                        +--------+-------+
                                        | 停止单条通知  |
                                        | 发送，只发集群 |
                                        +----------------+
```

**聚合规则**：
- 触发条件：同一 `resource_type` + `resource_id` 在 60s 内触发 > 20 条告警。
- 聚合行为：生成一条新的 Alert，`is_aggregated=TRUE`，`aggregated_count=N`，将所有子告警的 `parent_alert_id` 设为该聚合告警 ID。
- 通知行为：只发送一条集群告警通知，包含涉及的规则列表和时间范围。
- 抑制周期：同一规则同一目标在 5min 内只发送 1 次通知（除非级别升级）。

### 5.3 配置回滚流程

```
Admin Console
      |
      | 1. 选择审计记录，点击回滚
      v
Audit Service
      |
      | 2. 检查目标资源是否存在
      v
+-----------+     +----------------+     +----------------+
| 目标存在?  | --> | 是              | --> | 显示子资源影响面 |
+-----------+     +----------------+     +--------+-------+
      |                                           |
      | 否                                        | 3. 管理员确认
      v                                           v
+-----------+                            +--------+-------+
| 返回错误  |                            | 执行回滚      |
| OPS_AUD_  |                            | (BeforeState   |
| 4101      |                            | -> current)    |
+-----------+                            +--------+-------+
                                                  |
                                                  | 4. 生成新审计记录
                                                  v
                                         +--------+-------+
                                         | 验证回滚后  |
                                         | 状态，返回结果  |
                                         +----------------+
```

---

## 6. 技术选型与备选方案

### 6.1 时序数据库

| 方案 | 选择 | 理由 | 备选 |
|------|------|------|------|
| Prometheus | 推荐 | 已为 PRD 假设依赖，生态成熟，支持 PromQL，与 NewAPI/Sub2API 的 `/metrics` 集成自然 | VictoriaMetrics（更高性能，更低资源占用） |
| PostgreSQL 时序表 | 落地缓存 | 作为 Prometheus 不可用时的降级方案，保存最近 7 天原始指标 | - |

**决策理由**：
- 主指标存储使用 Prometheus，提供 `/metrics` 端点供外部 scrape。
- 在 PostgreSQL 中保存分钟级聚合指标（用于控制台快速查询）。
- 若 Prometheus 丢失，系统进入只读降级模式，告警引擎依赖本地缓存持续运行。

### 6.2 告警状态缓存

| 方案 | 选择 | 理由 | 备选 |
|------|------|------|------|
| Redis + 本地内存 (DualCache) | 推荐 | 参考 LiteLLM 的 DualCache 模式，Redis 保证多实例共享状态，本地内存降低延迟 | 单纯 Redis |

**设计细节**：
- 告警抑制状态存储在 Redis 中，TTL 为 5min。
- 告警聚合计数器存储在 Redis 中，TTL 为 1min。
- 本地内存作为 L1 缓存，命中失败时才访问 Redis（L2）。

### 6.3 告警批量处理

| 方案 | 选择 | 理由 | 备选 |
|------|------|------|------|
| 内存批量队列 + 定时刷盘 | 推荐 | 参考 LiteLLM CustomBatchLogger，每 10s 或队列长度 > 50 时刷盘，避免告警爆炸时的 IO 瓶颈 | 单条同步发送 |

### 6.4 通知渠道

| 渠道 | 优先级 | 备份策略 |
|------|--------|---------|
| Webhook | 1 | 失败时降级到邮件 |
| 邮件 | 2 | 失败时降级到飞书/企业微信 |
| 飞书/企业微信 | 3 | 失败时降级到短信 |
| 短信 | 4 | 失败时通知 TechLead |

---

## 7. 与立交桥主系统的集成点

### 7.1 Token Gateway (gateway/)

**数据提供**：
- gateway/ 需要通过 Prometheus 指标暴露以下数据：
  - `gateway_requests_total` (标签: path, method, status)
  - `gateway_request_duration_seconds` (标签: path, method, quantile)
  - `gateway_error_rate_5xx` (标签: path)
  - `gateway_degradation_hits_total` (标签: rule_id)

**控制接口**：
- gateway/ 提供内部 HTTP 接口供 AI-Ops 调用：
  - `POST /internal/gateway/throttle` — 启动/解除限流
  - `POST /internal/gateway/switch-route` — 切换路由规则
  - `POST /internal/gateway/restart` — 触发实例重启

**集成方式**：
- 独立运行时：通过配置文件 `gateway.internal_endpoint` 指定地址，使用 API Key 鉴权。
- 集成运行时：通过 `IntegrationPlugin` 直接调用 gateway/ 的内部方法，跳过 HTTP 层。

### 7.2 Channel Manager (supply-api/)

**数据提供**：
- supply-api/ 需要暴露以下接口：
  - `GET /internal/suppliers/health` — 供应商健康状态
  - `GET /internal/audit/events` — 审计事件查询
  - `GET /internal/usage/token-stats` — Token 消耗统计

**控制接口**：
- supply-api/ 提供内部接口供 AI-Ops 调用：
  - `POST /internal/suppliers/switch` — 切换主供应商
  - `POST /internal/suppliers/isolate` — 隔离异常供应商

**审计事件对接**：
- AI-Ops 的审计事件格式与 supply-api/ 保持一致。
- 集成运行时，可选择复用 supply-api/ 的 AuditStore 接口，或使用独立的 `ai_ops_audits` 表（推荐独立表，避免 schema 冲突）。

### 7.3 Platform Token Runtime

**数据提供**：
- platform-token-runtime/ 需要暴露：
  - `GET /internal/tokens/status` — 令牌消耗状态
  - `GET /internal/tokens/quota` — 配额余量
  - `GET /internal/tokens/recovery` — 异常恢复周期

---

## 8. 安全设计

### 8.1 角色与权限控制 (RBAC)

| 角色 | 监控查看 | 日志查询 | 告警确认/忽略 | 告警规则管理 | 配置回滚 | 高风险变更 |
|------|---------|---------|-------------|-------------|---------|-----------|
| 查看者 (viewer) | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| 运维人员 (operator) | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| 管理员 (admin) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

**实现方案**：
- 独立运行时，系统自带角色表 `ai_ops_roles`。
- 集成运行时，通过 `IntegrationPlugin` 接口从主程序获取当前用户角色，或复用主程序的 IAM 系统。
- 每个 HTTP 请求必须经过 Authz Middleware 检查，在响应头中返回 `X-Permitted-Actions` 列表。

### 8.2 审计与日志安全

- 审计日志必须使用只读存储，禁止任何用户/管理员直接修改 `ai_ops_audits` 表。
- 审计日志保留期 >= 90 天，通过 PostgreSQL 分区表 + 自动清理实现。
- 敏感字段脱敏：审计日志中的 `BeforeState` / `AfterState` 包含密钥、密码时，必须通过 Sanitizer 脱敏处理。
- 所有管理端点必须记录访问日志，包含操作人 IP、时间戳、操作类型。

### 8.3 数据隔离

- 所有数据查询必须带有 `tenant_id` / `workspace_id` 过滤条件，防止跨租户数据泄露。
- 数据库层面使用 Row Level Security (RLS) 作为最后一道防线（可选，根据性能决策）。

---

## 9. 性能考量

### 9.1 并发能力

| 指标 | 目标值 | 验证方式 |
|------|--------|---------|
| 告警规则评估吞吐量 | >= 50 条规则 / 15s | 压力测试 |
| 并发告警处理 | >= 100 事件/s | 压力测试 |
| 控制台首页加载 | < 2s | 性能测试 |
| 日志查询首页返回 | < 3s | 性能测试 |
| 审计日志查询 | < 3s | 性能测试 |

### 9.2 扩展性

- **水平扩展**：AI-Ops 服务无状态（状态存储在 Redis/PostgreSQL），可通过增加 Pod 数量水平扩展。
- **告警引擎分片**：当规则数量 > 200 条时，可将规则按 `metric_source` 分片到不同的评估器实例。
- **时序库扩展**：Prometheus 采用 Remote Write 到 VictoriaMetrics 或 Thanos，支持长期存储扩展。

### 9.3 存储估算

**指标数据**（以 Prometheus 为主存储）：
- 假设 10 个指标，每个指标 10 个标签组合，采集频率 15s。
- 每天数据量: 10 * 10 * (86400/15) * 8 bytes = 4.6 MB/天
- 7 天原始数据: ~32 MB
- 30 天分钟级聚合: ~200 MB
- 90 天小时级聚合: ~150 MB

**审计日志**（PostgreSQL）：
- 假设每天 1000 次配置变更，每条记录平均 2 KB。
- 每天: 2 MB
- 90 天: ~180 MB

**告警事件**（PostgreSQL）：
- 假设每天 500 条告警，每条记录平均 1 KB。
- 每天: 500 KB
- 90 天: ~45 MB

**总存储估算**：
- 指标时序库：500 MB（含小时级聚合）
- PostgreSQL (审计+告警+配置): 500 MB
- Redis (状态缓存): 100 MB
- 总计: ~1.1 GB（无压缩），实际生产环境建议预畔 5 GB 磁盘空间。

---

## 10. 风险评估与缓解策略

| 风险编号 | 风险描述 | 严重级别 | 发生概率 | 缓解策略 |
|---------|---------|---------|---------|---------|
| R-1 | 自愈规则设计不当导致正常流量被截断或重定向 | 高 | 中 | 沙盒模式强制验证；高风险变更二次确认；自愈引擎支持一键关闭 |
| R-2 | 告警规则过于敏感或缺乏抑制，导致噪音爆炸 | 高 | 中 | 告警聚合机制；抑制周期 5min；噪声率监控与自动告警；间隔 2h 未确认自动升级避免麻木 |
| R-3 | 回滚操作不当导致配置状态更深层次损坏 | 中 | 低 | 回滚前显示子资源影响面；二次确认；回滚后自动验证；高风险变更二次确认 |
| R-4 | 审计日志丢失导致故障定责和合规审查受阻 | 中 | 低 | 主备双写；异步文件缓存作为降级；90 天保留期；存储监控与预警 |
| R-5 | 时序数据库全面中断 | 高 | 低 | 控制台降级为只读模式；告警引擎依赖本地缓存持续运行；PostgreSQL 落地缓存作为最后防线 |
| R-6 | 通知渠道全部失效 | 中 | 低 | 主备自动切换机制；4 层降级；最终通知 TechLead；通知失败记录保留在事件中 |

### 10.1 威胁建模

| 威胁场景 | 攻击/故障路径 | 影响 | 控制措施 | 验证要求 |
|---------|---------------|------|---------|---------|
| 自愈误触发 | 错误规则或坏数据触发切流/限流/重启 | 生产流量中断、雪崩放大 | 沙盒演练、双人确认、高风险动作默认关闭、回滚快照 | 每个高风险动作必须有沙盒验证和回滚演练 |
| 告警洪泛 | 外部噪声或错误规则导致告警风暴 | 值班麻木、真实故障被淹没 | 聚合、抑制、静默窗口、升级策略、噪声率告警 | 压测和回放验证 50 条并发规则下噪声可控 |
| 越权运维操作 | 低权限用户执行回滚/规则修改/高风险变更 | 生产配置被误改 | RBAC、二次确认、审计、资源级鉴权、响应头返回 permitted actions | QA 必测 viewer/operator/admin 差异权限 |
| 审计链路失真 | 审计未先写入或被篡改 | 无法追责、回滚依据失效 | 审计先写后执行业务；审计存储防篡改；失败阻断高风险操作 | 审计写失败时高风险变更必须拒绝 |
| 外部适配层被滥用 | `/metrics`、Webhook、管理 API 适配暴露过多能力 | 信息泄露、被动放大攻击面 | 最小暴露面、签名校验、限流、只读隔离、错误码映射 | 合同测试覆盖外部接口鉴权与字段边界 |

### 10.2 设计阶段门控结论

**结论：REQUEST_CHANGES（补齐关键门禁描述后，方可进入开发）**

**放行前必须满足：**
- 自愈、回滚、告警、审计、权限五条核心链路都要在后续实现中提供真实挂载点与验证命令。
- `BuildServer` / `BuildRuntime` 显式挂载约束必须落实为 QA 的阻断检查项，避免“定义了但没接入”被误判为完成。
- 独立运行 / 集成运行 / IntegrationPlugin / OpenAPI / 适配层要求必须进入测试阻断矩阵。
- 对高风险变更必须规定 fail-closed，不允许“看起来能跑”替代验证通过。

**阻断条件：**
- 自愈动作没有沙盒、快照与回滚闭环。
- 审计日志不能保证先写审计再执行业务。
- 无法证明集成模式中路由、worker、健康检查全部真实挂载。

---

## 11. 可重用的设计模式

| 设计模式 | 来源 | 应用场景 |
|---------|------|---------|
| **CustomBatchLogger** | LiteLLM | 告警事件批量处理，避免高并发下的 IO 瓶颈 |
| **DualCache** | LiteLLM | 告警状态缓存（内存 + Redis），确保告警可靠性 |
| **DigestEntry** | LiteLLM | 告警聚合，避免滥发 |
| **AlertType + AlertTypeConfig** | LiteLLM | 可扩展的告警类型系统，支持按类型配置不同策略 |
| **OutageModel + ProviderRegionOutageModel** | LiteLLM | 故障状态机，支持模型级和区域级故障检测 |
| **Cooldown 机制** | LiteLLM | 故障部署自动移除，作为自愈动作的一种 |
| **FreeRide SupplierChain** | FreeRide (OpenClaw) | 供应商多级 Fallback 链 + 冷却期，防止震荡 |
| **SupplierProbe + ELOHistory** | FreeRide (OpenClaw) | 供应商探针定时任务 + 质量趋势记录 |
| **Repository + Service + Handler** | Bridge 主项目 | 分层架构，领域层定义接口，应用层实现业务逻辑，HTTP 层处理协议转换 |
| **Optimistic Locking** | supply-api/ | 配置变更时防止并发覆盖，Store 接口必须包含 expectedVersion |
| **Circuit Breaker** | 行业实践 | 自愈动作执行失败时，避免连续重试导致级联故障 |
| **Snapshot + Rollback** | 行业实践 | 自愈动作执行前记录状态快照，支持自动回退 |

---

## X 技术选型（前端）

### 前端技术栈
- **框架**：React 18+（或与 gateway 现有前端保持一致）
- **组件库**：Tailwind CSS + Headless UI（或现有 UI 框架）
- **图表**：ECharts 5.x（已在功能清单中使用）
- **构建工具**：Vite
- **状态管理**：React Query（用于 API 数据获取和缓存）

### 前端工作范围
- 监控首页（6 个指标卡片 + 实时刷新）
- 指标下钻页（ECharts 趋势图 + 维度筛选）
- 日志查询页（表格 + 分页 + 导出）
- 告警规则管理页（CRUD 表单）
- 告警事件列表页（状态 Tab + 集群聚合）
- 配置审计与回滚页
- 容量主板（多图表 + 预测卡片）

### 约束
- 前端不做后端逻辑，所有数据通过 `/api/v1/ai-ops/` REST 接口获取
- 前端与后端通过 JWT Token 认证，Token 由后端签发

---

## 12. 技术栈与集成约束

### 12.1 统一技术栈
本项目必须与立交桥主项目保持一致：
- **语言**: Go 1.22+
- **HTTP框架**: 标准库 `net/http` + 自定义中间件（禁止引入 Gin/Echo 等第三方框架，保持与 gateway/ 和 supply-api/ 的一致性）
- **数据库**: PostgreSQL 15+ ，驱动 `jackc/pgx/v5`
- **缓存**: Redis，客户端 `redis/go-redis/v9`
- **配置**: YAML + Viper，环境变量覆盖敏感字段
- **日志/审计**: 结构化日志，审计事件模型与 supply-api/ 一致
- **错误码**: `{SOURCE}_{CATEGORY}_{CODE}` 格式，例如 `OPS_ALT_4001`
- **健康检查**: `/actuator/health` 、 `/actuator/health/live` 、 `/actuator/health/ready`
- **测试**: Go testing + testify，覆盖率门槛 domain ≥ 70%、service/handler ≥ 80%

### 12.2 独立运行与集成运行
本系统必须同时支持两种运行模式：

| 模式 | 特征 | 部署方式 | 适用场景 |
|------|------|---------|---------|
| **独立运行** | 自有 `cmd/ai-ops/main.go`，独立数据库 schema，独立 docker-compose | `docker-compose up` 或单独容器 | 外部用户只需要运维能力，不想接入立交桥全套 |
| **集成运行** | 作为 Go module 被 `gateway/` 或 `supply-api/` 引入，共享数据库连接池和配置，通过内部接口注册 | 编译时作为子模块编译，运行时挂载到立交桥主进程 | 立交桥用户希望获得一体化运维能力 |

**集成约束**:
- 独立运行时，系统必须提供完整的 HTTP API 和管理后台。
- 集成运行时，系统必须提供 `IntegrationPlugin` 接口，允许主程序通过配置开关启用/禁用各模块。
- 数据库 schema 必须使用独立的 `ai_ops_` 前缀，避免与主项目表名冲突。
- 配置文件必须支持分离加载：独立运行时读取自己的 `config.yaml`，集成运行时合并到主项目配置。

### 12.3 NewAPI / Sub2API 适配支持
本系统的核心能力必须能够对接 NewAPI 和 Sub2API 系统：
- **监控数据推送**: 提供 Prometheus 格式的 `/metrics` 接口，NewAPI/Sub2API 可通过 Prometheus scrape 获取运维数据。
- **告警回调**: 支持 Webhook 告警通知，NewAPI/Sub2API 可配置接收本系统的告警事件。
- **自愈脚本扩展**: 自愈动作中的“触发程序化脚本”支持调用 NewAPI/Sub2API 的管理 API（如切换供应商、限流配置、重启实例）。
- **独立部署时**: 通过配置文件指定 NewAPI/Sub2API 的管理端点地址和鉴权信息，本系统通过适配层与之交互。
- **集成部署时**: 若立交桥 gateway/ 已接入 NewAPI/Sub2API，本系统通过 gateway/ 的内部路由接口操作上游状态。

### 12.4 对外接口契约
- 必须提供 OpenAPI 3.0 接口文档，确保 NewAPI/Sub2API 开发者可以独立接入。
- 接口路径前缀默认为 `/api/v1/ai-ops/`，集成运行时可通过配置改为 `/internal/ai-ops/`。

---

## 13. 变更日志

| 版本 | 日期 | 修改人 | 内容 |
|------|------|--------|------|
| v1.0 | 2026-04-27 | TechLead | 初稿：完成系统架构、模块设计、数据模型、流程设计、技术选型、集成点、安全、性能、风险、设计模式 |

---

## 附录 Y：参考文档与外部依赖

| 参考项目 | 版本/日期 | URL | 用途 |
|---------|---------|-----|------|
| LiteLLM | v1.40.0 (2026-03) | https://docs.litellm.ai/ | 模型接口标准化、健康检查设计 |
| Sub2API | main分支 (2026-04) | https://github.com/WeI-Shaw/sub2api | 公告系统、用户体系参考 |
| Intercom | - | https://www.intercom.com/ | 客服体验对标 |
| Prometheus | 3.x (2026-Q1) | https://prometheus.io/ | 时序数据存储 |
| VictoriaMetrics | 1.100.x (2026-Q1) | https://victoriametrics.com/ | 时序数据备选存储 |
| Playwright | 1.50.x (2026-Q1) | https://playwright.dev/ | 浏览器自动化 |
| Qdrant | 1.12.x (2026-Q1) | https://qdrant.tech/ | 向量数据库备选 |
| PGVector | 0.8.x (2026-Q1) | https://github.com/pgvector/pgvector | PostgreSQL向量扩展 |

注：以上版本号为评审时（2026-04-28）的最新稳定版，随着项目开发应定期更新。
