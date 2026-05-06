# DO-P1-1：最小监控与告警闭环

> 状态：✅ 已定义，待在真实共享预生产/灰度环境接入  
> 负责人：TechLead / DevOps  
> 基准：Gate B 已完成本地/容器化预演，Gate C 前必须落地最小观察面

---

## 1. 目标

生产一期灰度阶段不追求“全量可观测平台一次到位”，只要求有一套**最小、可执行、能支持放量/回滚决策**的监控闭环。

本轮最小监控集只覆盖 8 个指标：

1. `webhook 5xx`
2. `webhook reject 数`
3. `ticket 创建量`
4. `handoff 比率`
5. `audit 写入失败数`
6. `readiness down 次数`
7. `postgres 连接异常`
8. `单实例重启次数`

---

## 2. 最小指标定义

| 指标 | 定义 | 最低数据来源 | 说明 |
|------|------|--------------|------|
| Webhook 5xx | `POST /api/v1/customer-service/webhook*` 返回 5xx 的比例 | API 网关/Ingress 访问日志或应用日志 | 灰度放量的首要阻断指标 |
| Webhook reject 数 | 因签名、时间戳、非法 body 被拒绝的请求数 | `CS_AUTH_4031/4032/4033/4034`、`CS_REQ_*` 日志或审计 | 区分“攻击/误配置”和“服务不可用” |
| Ticket 创建量 | 每 5 分钟新建工单数 | `cs_tickets` 表或应用埋点 | 与 handoff 比率配合判断主链健康 |
| Handoff 比率 | `handoff=true` 会话数 / 总 webhook 请求数 | webhook 结果日志、审计或 DB | 反映机器人有效性与故障降级情况 |
| Audit 写入失败数 | audit 写入失败事件数 | 应用 ERROR 日志 | 任一增长都需要关注 |
| Readiness down 次数 | `ready` 探针失败次数 | K8s probe / LB 健康检查 / 外部探测 | 用于摘流与自动回滚判断 |
| PostgreSQL 连接异常 | DB ping/query error 次数 | `ready` 检查、应用 ERROR、连接池错误 | Phase 1 的核心依赖告警 |
| 单实例重启次数 | 单个实例在窗口期内重启次数 | K8s event / systemd / 容器平台 | 判断二进制稳定性和资源问题 |

---

## 3. 告警阈值与动作

### 3.1 必须可执行的阈值

| 指标 | 阈值 | 持续时间 | 级别 | 动作 |
|------|------|----------|------|------|
| Webhook 5xx | `> 1%` | 5 分钟 | P1 | 立即停止继续放量，触发回滚评估 |
| Webhook 5xx | `> 5%` | 5 分钟 | P0 | 立即回滚当前灰度版本 |
| Webhook reject 数 | `> 5%` 且以 `4031/4034` 为主 | 10 分钟 | P2 | 检查上游签名配置，不自动回滚 |
| Webhook reject 数 | `> 20%` | 10 分钟 | P1 | 暂停放量，升级为渠道接入故障 |
| Ticket 创建量 | 灰度期内 handoff 明显存在，但连续 10 分钟 `ticket 创建量 = 0` | 10 分钟 | P1 | 判定工单主链异常，停止放量 |
| Handoff 比率 | `> 25%` 或高于过去 24h 基线 `2x` | 30 分钟 | P2 | 检查意图识别/依赖故障/降级路径 |
| Audit 写入失败数 | `> 0` | 5 分钟 | P1 | 停止放量，优先排查审计链路 |
| Readiness down 次数 | 单实例连续 3 次失败 | 3 个探针周期 | P1 | 从灰度池摘流量 |
| PostgreSQL 连接异常 | `> 0` 且影响 ready | 1 分钟 | P0 | 立即停止放量，必要时回滚 |
| 单实例重启次数 | 单实例 `> 2` 次 | 10 分钟 | P2 | 冻结当前比例，排查资源/崩溃问题 |

### 3.2 放量前置条件

进入下一个灰度档位前，必须同时满足：

1. 最近一个观察窗口内 `webhook 5xx <= 0.5%`
2. `audit 写入失败数 = 0`
3. `postgres 连接异常 = 0`
4. 没有实例因 `readiness down` 被持续摘流
5. `ticket 创建量` 与 `handoff 比率` 没有出现异常偏移

---

## 4. 指标落地方式

当前仓库还没有 Prometheus 指标端点，因此本轮按“两层实现”定义：

### 4.1 Gate C 前最低可接受方案

- Ingress / API Gateway access log 统计：
  - webhook 请求总量
  - webhook 5xx
- 应用日志统计：
  - `CS_AUTH_403*`
  - `audit write failed`
  - `webhook process failed`
  - `postgres` 相关错误
- 数据库 SQL 统计：
  - `cs_tickets` 新增量
  - `cs_audit_logs` 指定 action 数量
  - `cs_message_dedup` 去重记录数
- 探针统计：
  - `live`
  - `ready`

### 4.2 推荐目标方案

后续在不改变本轮门禁的前提下，可以升级为：

- Prometheus metrics
- Alertmanager 路由
- Grafana 灰度大盘
- Loki / ELK 日志聚合

---

## 5. 最小告警路由

| 事件 | 通知对象 | 方式 | 时限 |
|------|----------|------|------|
| P0：DB 异常 / 5xx > 5% | 值班工程师 + TechLead | 电话 + 飞书 | 5 分钟内响应 |
| P1：5xx > 1% / audit 失败 / readiness 异常 | 值班工程师 | 飞书 + 工单 | 15 分钟内响应 |
| P2：handoff 异常升高 / reject 异常 | 值班工程师 + 产品/运营 | 飞书 | 30 分钟内响应 |

---

## 6. 当前落地状态

| 项目 | 当前状态 | 结论 |
|------|----------|------|
| 指标定义 | 已完成 | ✅ |
| 告警阈值 | 已完成 | ✅ |
| Grafana/Prometheus 接入 | 未完成 | ⚠️ Gate C 前需至少完成最低可接受方案 |
| 真共享预生产环境监控联调 | 未完成 | ⚠️ |
| 回滚联动门禁 | 已定义，未演练 | ⚠️ |

---

## 7. 与灰度放量的关系

这份文档不是泛化监控说明，而是**灰度放量门禁文档**。  
任何放量决策都必须引用：

- [GRAY_DASHBOARD_MINIMUM.md](/home/long/project/立交桥/projects/ai-customer-service/docs/GRAY_DASHBOARD_MINIMUM.md)
- [SERVICE_SLA.md](/home/long/project/立交桥/projects/ai-customer-service/prd/SERVICE_SLA.md)
- [GRAY_RELEASE_ROLLBACK_RUNBOOK.md](/home/long/project/立交桥/projects/ai-customer-service/prd/GRAY_RELEASE_ROLLBACK_RUNBOOK.md)

