# Router Core 接管执行方案（v3）

- 版本：v3.0
- 日期：2026-03-17
- 目标阶段：S2（2026-05-16 至 2026-08-15）
- 关联文档：
  - `llm_gateway_subapi_evolution_plan_v2_2026-03-17.md`
  - `subapi_connector_contract_v1_2026-03-17.md`
  - `sub2api_scheduler_billing_flow_deep_dive_v2_2026-03-17.md`

## 1. 目标与边界

本方案只解决 S2 的三件事：

1. 自研 Router Core 主路径接管率（全供应商）`>= 60%`。
2. 国内 LLM 供应商主路径接管率 `= 100%`。
3. 在不牺牲稳定性与计费正确性的前提下推进迁移（可灰度、可回滚、可审计）。

边界：

1. 不在 S2 内引入机器人客户能力（归属 S3）。
2. 不在 S2 内上线低成本账号采购模块（归属 S4）。
3. S2 允许保留 `subapi connector` 作为兜底与长尾协议承接层。

## 2. 接管率口径（统一计算，避免歧义）

## 2.1 主路径定义

“主路径请求”定义为：

1. 从统一入口进入并归一到 canonical 主路径端点集合（`/v1/chat/completions`、`/v1/messages`、`/v1/responses`、`/v1beta/*`）。
   - 说明：`/responses` 等 alias 入口会在 Ingress 层归一到 `/v1/responses` 后再参与统计。
2. 经路由决策后实际发往上游模型供应商的在线请求（不含后台管理请求）。
3. 按请求数统计，不按 token 统计。

## 2.2 接管判定

当以下能力全部由自研 Router Core 执行，判定为“自研接管请求”：

1. 账号选择（scheduler）
2. 并发门控（user/account slot）
3. failover 决策
4. usage 记录与幂等计费落账

任一环节仍依赖 `subapi` 内部实现，则该请求记为“subapi 路径请求”。

## 2.3 公式

1. 全供应商接管率

`overall_takeover = self_built_main_path_requests / all_main_path_requests`

2. 国内供应商接管率

`cn_takeover = self_built_cn_provider_requests / all_cn_provider_requests`

S2 验收门槛：

1. `overall_takeover >= 60%`
2. `cn_takeover = 100%`

## 3. 模块拆分与迁移优先级

## 3.1 模块清单（按执行优先级）

| 优先级 | 模块 | S2 目标状态 | 是否必须自研 | 说明 |
|---|---|---|---|---|
| P0 | Scheduler Core | GA | 是 | 三层选择（previous/session/load） |
| P0 | Concurrency Gate | GA | 是 | user/account 双槽位 + wait queue |
| P0 | Failover Orchestrator | GA | 是 | 同号重试/换号/流式 no-replay |
| P0 | Usage & Billing Core | GA | 是 | request 级幂等落账 |
| P0 | CN Provider Adapter Pack | GA | 是 | 国内供应商 100% 走自研路径 |
| P1 | Error Normalization Engine | GA | 是 | OpenAI/Anthropic/Gemini 统一错误语义 |
| P1 | Stream Guard Layer | GA | 是 | 已写出流内容后禁止 replay |
| P1 | Observability & Audit | GA | 是 | 接管率、扣费冲突、failover 可观测 |
| P2 | subapi Connector | 保留 | 否 | 长尾协议与兜底 |
| P2 | Non-critical Protocol Compat | Beta | 否 | 可延后到 S3/S4 |

## 3.2 模块执行顺序（建议 6 个批次）

1. 批次 A（第 1-2 周）：
- Scheduler Core
- Concurrency Gate
- 基础 Observability

2. 批次 B（第 3-4 周）：
- Failover Orchestrator
- Stream Guard Layer

3. 批次 C（第 5-6 周）：
- Usage & Billing Core
- 幂等仓储与冲突告警

4. 批次 D（第 7-8 周）：
- CN Provider Adapter Pack 全量接入
- 国内供应商流量灰度到 70%

5. 批次 E（第 9 周）：
- 国内供应商流量 100% 自研接管
- `cn_takeover` 验收

6. 批次 F（第 10-12 周）：
- 全供应商流量继续切换至 60%+
- `overall_takeover` 验收

## 4. 分阶段迁移策略（含灰度门槛）

## 4.1 国内供应商迁移（必须 100%）

1. Wave-CN-1：10%
- 前置条件：P0 模块全部可用
- 观察窗口：24h
- 红线：5xx、计费冲突率、超时率

2. Wave-CN-2：40%
- 前置条件：Wave-CN-1 全部指标通过
- 观察窗口：24h

3. Wave-CN-3：70%
- 前置条件：Failover 与 Stream Guard 指标通过
- 观察窗口：48h

4. Wave-CN-4：100%
- 前置条件：连续 7 天稳定
- 动作：将国内供应商主路径全部切至自研

## 4.2 全供应商迁移（目标 >=60%）

1. Wave-Global-1：20%
2. Wave-Global-2：40%
3. Wave-Global-3：60%+

每一波都必须具备：

1. 一键回切到 `subapi connector`
2. 独立观察看板（按 provider、tenant、endpoint）
3. 账务核对通过（请求级抽样）

## 5. 验收测试矩阵（可执行）

## 5.1 模块级验收矩阵

| 模块 | 测试类型 | 核心用例 | 通过标准 | 证据产物 |
|---|---|---|---|---|
| Scheduler Core | 单元+集成 | previous/session/load 三层选择 | 选择层命中率符合预期；错误账号不重复命中 | 调度决策日志 + 测试报告 |
| Concurrency Gate | 压测+集成 | user/account 双槽位争用 | 无槽位泄漏；等待队列上限生效；超限返回 429 | 并发压测报告 + Redis key 观测 |
| Failover Orchestrator | 集成 | 同号重试、换号重试、切换上限 | 不超过 max switches；重试策略符合配置 | failover trace |
| Stream Guard Layer | 集成+回归 | 流已写出后上游错误 | 禁止 replay；无双流拼接 | 流式回归用例结果 |
| Usage & Billing Core | 集成+一致性 | 重复 request_id、冲突 fingerprint | 重复不重复扣费；冲突可告警可追踪 | 账务对账报表 |
| CN Adapter Pack | 端到端 | 全国内供应商请求链路 | 路由、鉴权、错误映射、计费全通过 | provider e2e 报告 |
| Error Normalization | 契约 | OpenAI/Anthropic/Gemini 错误归一 | category/code/retryable 一致 | 契约测试报告 |
| Observability & Audit | 集成 | request_id 全链路、接管率统计 | 可追踪率 100%；接管率计算一致 | dashboard 截图 + SQL 校验 |

## 5.2 阶段性门槛

1. 质量门槛
- 网关附加时延 P95 <= 60ms
- 5xx 不高于基线 + 0.1%

2. 账务门槛
- 账务差错率 <= 0.1%
- 幂等冲突率 <= 0.01%（超阈值即阻断继续灰度）

3. 迁移门槛
- `cn_takeover = 100%`
- `overall_takeover >= 60%`

## 6. 里程碑与交付物

| 里程碑 | 时间窗 | 交付物 |
|---|---|---|
| M1 基础接管能力可用 | 第 2 周末 | P0 模块上线灰度 |
| M2 稳定 failover 与流式边界 | 第 4 周末 | failover/stream guard 回归通过 |
| M3 幂等计费闭环 | 第 6 周末 | 账务一致性报告 |
| M4 国内供应商 100% 接管 | 第 9 周末 | `cn_takeover` 验收报告 |
| M5 全供应商 60%+ 接管 | 第 12 周末 | `overall_takeover` 验收报告 |

## 7. 风险与应对

1. 风险：接管率统计口径前后不一致
- 应对：统一 SQL 统计脚本与看板口径，验收前做双系统对账

2. 风险：流式边界处理不一致导致客户端异常
- 应对：将“写出后禁止 replay”抽象为统一中间层并覆盖回归

3. 风险：国内供应商适配细节差异过大
- 应对：Provider Adapter Pack 先做最小公共面，再逐家补差异策略

4. 风险：计费幂等冲突上升
- 应对：request_id 生成策略收敛 + fingerprint 冲突报警 + 快速止血回切

## 8. 本周执行清单（可直接开工）

1. 固化接管率统计 SQL（overall/cn 两套）并接入 dashboard。
2. 拉出 P0 四模块的接口清单与 owner，建立每日燃尽图。
3. 为 Stream Guard 建立跨协议回归用例（OpenAI/Anthropic/Gemini）。
4. 为 Usage & Billing 建立“重复请求/冲突指纹”专项压测与告警规则。
5. 按国内供应商清单建立 Adapter 接入优先级（先高频模型再长尾）。

## 9. 与现有文档的关系

1. 本文档是 S2 执行层文档，补充了 v2 演进稿中“方向有了但执行口径不够细”的部分。
2. `subapi connector` 契约继续有效，S2 作为兜底与长尾承接；不再承担国内供应商主路径。
3. 本文档可作为每周项目例会的唯一追踪基线（接管率、质量、账务三条主线）。

## 10. 实施附件（新增）

为保证 S2 可执行与可验收，新增以下实施附件：

1. `router_core_takeover_metrics_sql_dashboard_v1_2026-03-17.md`
   - 固化接管率 SQL（overall/cn/趋势）
   - 明确看板字段与告警阈值
2. `router_core_s2_acceptance_test_cases_v1_2026-03-17.md`
   - 按模块展开验收用例
   - 固化 Wave-CN / Wave-Global 的 stop/go 条件

## 11. 兼容与安全运维设计附件（新增）

为降低 S2 实施期兼容与安全事故风险，新增设计文档：

1. `subapi_integration_compat_security_reliability_design_v1_2026-03-17.md`
   - 明确兼容三重 Gate（Schema/Behavior/Performance）
   - 固化 subapi 集成安全风险台账与防护基线
   - 补齐“运维简单 + 高可靠”目标架构与两周落地动作

## 12. 两周执行任务单（新增）

为确保上述风险控制设计可落地执行，新增：

1. `subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md`
   - 两周任务排期（里程碑、owner、截止日期）
   - 明确任务级验收标准与证据包要求
   - 固化 daily/weekly gate 与 P0/P1/P2 阻断规则

## 13. 专家审核与博弈机制（新增）

为确保“可实现集成 + 可实现替换 + 企业级可商用”目标可被独立验证，新增：

1. `subapi_expert_review_wargame_plan_v1_2026-03-17.md`
   - 定义专家组成、独立性与回避规则
   - 采用 Red vs Blue 对抗式评审与四轮审核流程
   - 固化评分模型、GO/CONDITIONAL GO/NO-GO 决策与一票否决条件

## 14. 三角色联合评审输入（新增，2026-03-18）

为强化 S2 接管执行的用户可接受性、测试阻断能力与网关替换可逆性，新增：

1. `subapi_role_based_review_wargame_optimization_v1_2026-03-18.md`
   - 用户代表：迁移通知与争议 SLA 门槛
   - 测试专家：契约漂移/流式 failover/升波证据包门槛
   - 网关专家：Provider 能力矩阵与降级策略门槛
   - 相关新增任务：`UXR-*`、`TST-*`、`GAT-*`、`EXP-007`
