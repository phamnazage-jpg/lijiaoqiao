# AI-Customer-Service 生产上线执行方案

> 定位：本文件替代 demo/proto 导向的实施口径，作为小龙统筹 PM / TechLead / QA / Engineer 按生产上线标准推进的唯一执行基线。

## 1. 结论

当前 `ai-customer-service` **不具备生产上线条件**。

已完成的只是一个可运行原型，不能作为“阶段完成”或“可灰度上线”的依据。后续工作必须按生产项目方式推进，满足：
- 文档与实现一致
- 数据与审计可持久化
- 权限、签名、幂等、隔离、防重放具备
- 工单闭环真实存在
- 外部依赖真实联通并可观测
- 灰度、回滚、SLO、告警、Runbook 完整

## 2. 小龙团队职责重排

### 2.1 小龙（统筹）
负责：
- 统一生产一期范围，禁止再使用 MVP-proto 口径作为完成标准
- 建立跨角色门禁，不允许“代码能跑”替代“产品可上线”
- 每阶段只允许在 PM/TechLead/QA 共同签字后进入下一阶段
- 对“文档说有、代码没有”“测试只测 happy path”直接打回

### 2.2 PM
必须补齐：
1. 《生产一期范围与门禁定义》
2. 《客服 SLA 与升级响应规范》
3. 《工单运营闭环 SOP》
4. 《灰度发布与回滚 Runbook》
5. 《客服运营后台需求说明》
6. 《身份核验与数据权限策略》
7. 《数据合规与留存策略》
8. 《商业化与价值追踪方案》

### 2.3 TechLead
必须补齐：
1. 生产数据模型与 migration 方案
2. PostgreSQL / Redis / 外部依赖 / 配置系统接入设计
3. Webhook 签名、防重放、幂等、审计 fail-closed 方案
4. Ticket / Session / Audit / KB 真实架构
5. IntegrationPlugin / 集成运行模式设计
6. metrics / tracing / logging / health readiness 设计
7. 降级、熔断、回滚、灰度技术方案

### 2.4 QA
必须补齐：
1. 文档-实现一致性检查清单
2. 威胁建模到测试映射清单
3. AC/失败路径/安全/性能/灾备测试矩阵
4. 灰度与回滚演练检查表
5. 实施漂移检测点
6. 上线阻断条件清单

### 2.5 Engineer
必须按文档和门禁实现，不得自行降级为：
- 内存版替代持久化
- 文本文案替代真实工单
- 占位 OpenAPI 替代真实契约
- 永远 UP 的 health 替代 readiness

## 3. 当前 P0 阻塞项

### P0-1 范围口径错误
- 当前 `IMPLEMENTATION_PLAN.md` 仍使用 `MVP-proto` 口径。
- 必须废弃其“已完成即可进入下一阶段”的含义。

### P0-2 持久化与数据模型缺失
- Session / Audit / Knowledge 仍为内存实现。
- 无 PostgreSQL schema / migration / rollback。

### P0-3 Webhook 安全链路缺失
- 无签名校验、无防重放、无幂等、无限流。

### P0-4 工单闭环不存在
- 当前转人工只返回文案，没有真实 ticket 创建、分配、处理、关闭。

### P0-5 身份核验与只读业务查询缺失
- 无用户绑定、无 quota/token/error logs 真实查询。

### P0-6 权限与隔离缺失
- 无鉴权、无 RBAC、无后台权限模型、无跨用户隔离验证。

### P0-7 审计不可靠
- 审计不持久化，且当前是 fail-open。

### P0-8 可观测性与健康检查失真
- 无 metrics/tracing/structured logging。
- readiness/health 不检查依赖状态。

### P0-9 灰度/回滚不可执行
- 文档有灰度与回滚要求，但代码与部署层无对应能力。

### P0-10 契约失真
- OpenAPI / INTERFACE / router 实现明显不一致。

## 4. 分阶段执行计划

### Phase 0：收口生产一期基线（必须先完成）
交付物：
- `PRODUCTION_EXECUTION_PLAN.md`（本文件）
- 重写 `IMPLEMENTATION_PLAN.md`，去掉 proto 口径
- PM 产出生产一期范围、门禁、SLA、工单运营、灰度回滚、合规文档清单
- QA 产出上线阻断清单

退出条件：
- 不再使用“最小原型已完成”作为阶段结论
- PM / TechLead / QA 对 P0 范围达成一致

### Phase 1：生产底座
交付物：
- PostgreSQL schema + migration + rollback
- Redis 方案
- 配置系统（YAML + env）
- 结构化日志、metrics、trace id
- health/live/ready 真实区分
- graceful shutdown

退出条件：
- 服务重启不丢核心状态
- 多实例可运行
- readiness 能真实阻断坏实例接流量

### Phase 2：入口安全与契约
交付物：
- webhook 签名校验
- 防重放
- 幂等表与重复消息处理语义
- body limit / schema validation
- 完整 OpenAPI
- 统一错误码

退出条件：
- 外部恶意/重复/畸形请求不能造成假成功
- QA 契约测试通过

### Phase 3：核心业务闭环
交付物：
- Session / Message / Ticket / Audit 持久化
- 真实工单状态机
- 转人工创建/分配/关闭链路
- 身份核验与账户绑定
- quota/token/error logs 只读查询
- 审计 fail-closed

退出条件：
- 查询、转人工、审计、人工处理形成真实闭环
- 不再存在“文案假装已转人工”

### Phase 4：运营后台与知识库
交付物：
- 工单后台 API
- 知识库 CRUD / 发布 / 审核 / 引用统计
- FAQ 命中与未命中回流
- 运营指标看板

退出条件：
- 客服与运营团队可实际接管系统

### Phase 5：依赖联调、灰度、回滚
交付物：
- supply-api / token-runtime / gateway / NewAPI/Sub2API 联调结果
- 灰度策略开关
- 回滚脚本与 Runbook
- 压测/安全/灾备报告
- 发布检查单

退出条件：
- QA 签字通过
- 小龙批准进入灰度

## 5. 生产级门禁

### Gate A：允许开始实现前
- [ ] 生产一期范围清晰，不含 proto/demo 表述
- [ ] PM 文档补齐到可执行程度
- [ ] QA 阻断项建立完成
- [ ] TechLead 生产架构方案冻结

### Gate B：允许联调前
- [ ] 持久化、签名、防重放、幂等、鉴权、审计已具备
- [ ] OpenAPI 与实现一致
- [ ] 真实健康检查可工作
- [ ] 关键失败路径自动化测试存在
- [x] **Phase 1 真实范围已定义**：6 个接口（P0-A~C + P1-D~E）+ 错误码统一
- [x] **16+ 漂移接口已明确分类**：GET tickets/{id} / POST sessions/{id}/handoff / POST sessions/{id}/feedback / GET tickets/stats → Phase 1；KB 全系 / admin 全系 / 会话查询类 → Phase 2
- [ ] **GET /tickets/{id}** 已实现并测试通过
- [ ] **POST /sessions/{id}/handoff** 已实现并测试通过（手动转人工）
- [ ] **POST /sessions/{id}/feedback** 已实现并测试通过
- [ ] **GET /tickets/stats** 已实现并测试通过
- [ ] **错误码全局统一**：无 hardcode 散落，统一使用 `internal/domain/error/` 包

### Gate C：允许灰度前
- [ ] 工单闭环真实可用
- [ ] 身份核验与只读查询真实可用
- [ ] 监控、告警、SLO 仪表板上线
- [ ] 灰度/回滚 Runbook 完成并演练
- [ ] 压测/安全/灾备测试通过

### Gate D：允许全量前
- [ ] 灰度期间投诉率、错误率、转人工率、SLA 达标
- [ ] 无 P0/P1 未关闭缺陷
- [ ] PM/TechLead/QA/小龙联合签字

## 6. 当前立即执行项（本轮）

1. 废弃 demo 口径：重写 `IMPLEMENTATION_PLAN.md`
2. 以生产底座为先，优先落地：
   - PostgreSQL migration
   - 持久化 Session/Audit/Ticket 基础模型
   - 配置系统
   - readiness/health 改造
   - HTTP 超时/请求体限制/优雅停机/结构化日志基础设施
3. 并行补齐 PM/QA 文档，不允许只有代码没有上线规则

## 7. 纪律要求

- 不允许再把“代码能运行”汇报成“项目可上线”。
- 不允许拿 mock/内存版冒充生产闭环完成。
- 不允许 QA 在没有真实依赖、真实工单、真实权限边界验证的情况下放行。
- 任何阶段发现文档与实现漂移，立即回退到上一门禁。 
