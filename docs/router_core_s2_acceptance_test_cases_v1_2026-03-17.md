# Router Core S2 验收测试用例清单（按模块展开，v1.1）

- 版本：v1.1
- 日期：2026-03-24
- 适用阶段：S2（2026-05-16 至 2026-08-15）
- 关联文档：
  - `router_core_takeover_execution_plan_v3_2026-03-17.md`
  - `router_core_takeover_metrics_sql_dashboard_v1_2026-03-17.md`

## 1. 验收范围与统一门槛

统一质量/账务/迁移门槛（与 v3 对齐）：

1. 网关附加时延 `P95 <= 60ms`。
2. 5xx 不高于基线 `+0.1%`。
3. 账务差错率 `<= 0.1%`。
4. 幂等冲突率 `<= 0.01%`。
5. `cn_takeover = 100%`。
6. `overall_takeover >= 60%`。
7. `supplier_credential_exposure_events = 0`。
8. `platform_credential_ingress_coverage_pct = 100%`。
9. `direct_supplier_call_by_consumer_events = 0`。
10. `query_key_external_reject_rate_pct = 100%`。

测试环境前置：

1. 测试库已包含 `usage_logs`、`usage_billing_dedup`、`ops_*` 表。
2. 已启用接管率统计口径（临时口径或验收口径至少一种）。
3. 准备至少 2 个账号/分组用于 failover 与并发争用验证。

## 2. Scheduler Core

| 用例ID | 前置条件 | 输入 | 测试步骤 | 预期结果 | 证据 |
|---|---|---|---|---|---|
| SCH-001 | 同分组有 A/B 两账号；A 为 previous 命中账号 | 同一会话连续请求 50 次 | 1) 首次请求命中 A 2) 持续请求保持会话上下文 3) 记录命中账号分布 | previous/session 策略优先命中 A；命中率符合配置 | 调度日志（含策略层级）+ 请求明细导出 |
| SCH-002 | session TTL 已配置（如 3600s） | 跨 TTL 边界请求 | 1) TTL 内请求 2) 超过 TTL 后再次请求 | TTL 内保持粘性；超时后允许重新选路 | session key 变化记录 + 调度 trace |
| SCH-003 | 账号负载差异明显（A 高负载，B 低负载） | 500 并发短请求 | 1) 打压 A 形成高队列 2) 发起混合流量 | load 评分生效，流量向低负载账号倾斜 | 账号负载快照 + 命中分布图 |
| SCH-004 | 存在错误账号 C（持续 429/529） | 正常请求流量 | 1) 触发 C 错误 2) 持续观测调度 | 错误账号降权/隔离，不应持续被命中 | 账号状态变更日志 + 调度拒选原因 |

## 3. Concurrency Gate

| 用例ID | 前置条件 | 输入 | 测试步骤 | 预期结果 | 证据 |
|---|---|---|---|---|---|
| CCG-001 | 用户并发上限=5 | 同用户 20 并发 | 1) 同时发起 20 请求 2) 统计状态码 | 超出上限请求进入等待或返回 429；无 500 扩散 | 压测报告 + 429 比例统计 |
| CCG-002 | 账号并发上限=3；队列上限已配置 | 多用户同打一个账号 | 1) 持续冲击单账号 2) 监控队列深度 | 账号槽位受控；队列深度不越界 | 队列深度曲线 + 账号并发快照 |
| CCG-003 | 队列超时阈值已配置 | 长耗时请求 + 短请求混流 | 1) 先占满槽位 2) 注入短请求 | 超时请求返回预期错误（429/超时码）；无槽位泄漏 | 错误分布 + 槽位计数核对 |
| CCG-004 | 支持异常中断场景 | 中途断连/超时中止请求 | 1) 发起请求后主动断连 2) 重复多轮 | 槽位自动释放；后续请求可正常获取槽位 | 槽位释放日志 + 恢复后成功率 |

## 4. Failover Orchestrator

| 用例ID | 前置条件 | 输入 | 测试步骤 | 预期结果 | 证据 |
|---|---|---|---|---|---|
| FO-001 | 配置同号重试次数=2 | 构造可重试错误（网络抖动） | 1) 注入瞬时网络错误 2) 观察重试轨迹 | 先同号重试，成功后不换号 | failover trace + 重试次数统计 |
| FO-002 | 配置换号重试上限=3 | 构造 429/529 持续错误 | 1) 让当前账号持续限流 2) 观察是否切换账号 | 按策略换号且不超过上限 | 账号切换链路日志 |
| FO-003 | 注入不可重试错误（4xx 业务错误） | 非法参数请求 | 1) 提交非法请求 2) 观察行为 | 不应触发重试/换号，直接返回标准错误 | 错误规范化结果 + 无重试证据 |
| FO-004 | 多账号可用；`max switches` 已配置 | 持续故障流量 | 1) 触发连续失败 2) 超过上限后继续请求 | 超过上限后停止切换并返回兜底错误 | failover 上限命中日志 + 告警事件 |

## 5. Stream Guard Layer

| 用例ID | 前置条件 | 输入 | 测试步骤 | 预期结果 | 证据 |
|---|---|---|---|---|---|
| SG-001 | 流式链路可控注入错误 | 首包前上游失败 | 1) 在首 token 前注入可重试错误 2) 观察策略 | 可按策略重试，不产生重复输出 | 流式事件序列 + 重试日志 |
| SG-002 | 流式输出已开始 | 首 token 后注入上游失败 | 1) 首 token 已发送 2) 注入上游失败 | 禁止 replay；返回单次终止，不出现双流拼接 | SSE/WS 抓包 + stream guard 日志 |
| SG-003 | 客户端断流可观测 | 客户端中途取消 | 1) 流式进行中取消连接 2) 观察后端行为 | 后端及时收敛，不继续写流，不泄漏资源 | 连接生命周期日志 + 资源占用快照 |

## 6. Usage & Billing Core

| 用例ID | 前置条件 | 输入 | 测试步骤 | 预期结果 | 证据 |
|---|---|---|---|---|---|
| UB-001 | 幂等键生效（`request_id + api_key_id`） | 相同 request_id 重放 2 次 | 1) 发送首次请求 2) 原样重放 | 只扣费一次；重复请求命中幂等 | 账务明细 + `usage_billing_dedup` 记录 |
| UB-002 | 启用指纹冲突检测 | 同 request_id 不同 payload | 1) 首次成功请求 2) 修改 payload 重放 | 触发冲突告警；不允许静默重复扣费 | 冲突告警事件 + 审计日志 |
| UB-003 | failover 打开 | 单请求多次尝试后成功 | 1) 前几次失败 2) 最终一次成功 | 全链路只产生一条有效扣费记录 | request 级账务对账报告 |
| UB-004 | 冷归档任务可执行 | 运行 dedup 归档任务 | 1) 执行归档 2) 抽样验证历史请求去重 | 归档后热表缩小且去重能力不退化 | 归档任务日志 + 抽样重放结果 |

## 7. CN Provider Adapter Pack

| 用例ID | 前置条件 | 输入 | 测试步骤 | 预期结果 | 证据 |
|---|---|---|---|---|---|
| CN-001 | 已接入国内供应商账号集 | 标准文本请求 | 1) 发起兼容请求 2) 核对路由和鉴权 | 鉴权成功，路由到目标供应商，无协议回退 | adapter 请求日志 + 上游响应 |
| CN-002 | 模型映射表已配置 | 兼容模型名请求 | 1) 使用统一模型名 2) 检查映射后的上游模型 | 映射正确，计费口径不丢失 | 模型映射日志 + usage 样本 |
| CN-003 | 国内供应商多账号可用 | 注入单账号故障 | 1) 让主账号故障 2) 验证同平台切换 | 在国内供应商集合内切换成功，链路不回退到 subapi | failover trace + router_engine 统计 |
| CN-004 | 已开启 CN 全量灰度阶段 | 24h 连续流量 | 1) 持续运行 2) 统计接管率 | `cn_takeover=100%`，若非 100 立即阻断升波 | CN 接管率 SQL 输出 + 告警记录 |

## 8. Error Normalization Engine

| 用例ID | 前置条件 | 输入 | 测试步骤 | 预期结果 | 证据 |
|---|---|---|---|---|---|
| EN-001 | OpenAI/Anthropic/Gemini 适配器可用 | 三平台等价限流错误 | 1) 各平台触发 429 2) 比较返回结构 | 统一 `category/code/retryable` 语义一致 | 契约测试报告 |
| EN-002 | 非重试错误映射规则就绪 | 参数错误/鉴权错误 | 1) 触发 400/401 2) 观察重试标记 | `retryable=false`，无误重试 | 错误返回样本 + failover 日志 |
| EN-003 | passthrough 白名单规则已配置 | 指定平台原始错误 | 1) 命中白名单规则 2) 校验透传字段 | 仅白名单字段透传，敏感信息不泄漏 | 规则命中日志 + 安全审计记录 |

## 9. Observability & Audit

| 用例ID | 前置条件 | 输入 | 测试步骤 | 预期结果 | 证据 |
|---|---|---|---|---|---|
| OBS-001 | 全链路打点开启 | 正常/异常混合请求 | 1) 抽样 100 请求 2) 逐条回溯 request_id | request_id 可追踪率 100% | trace 检查清单 |
| OBS-002 | 接管率看板已上线 | 固定时间窗查询 | 1) 执行 SQL 2) 对照看板值 | SQL 与看板偏差在容忍范围内（建议 <=0.1pp） | SQL 输出存档 + 看板截图 |
| OBS-003 | 告警规则已启用 | 人工触发阈值场景 | 1) 构造超阈值 2) 观察告警生命周期 | 告警按规则触发、抑制、恢复 | `ops_alert_events` 记录 |
| OBS-004 | 审计日志不可篡改策略生效 | 管理操作与重试操作 | 1) 执行管理动作 2) 验证审计条目 | 关键动作均有审计证据，字段完整 | 审计导出报表 |

## 10. Credential Boundary（凭证边界）

| 用例ID | 前置条件 | 输入 | 测试步骤 | 预期结果 | 证据 |
|---|---|---|---|---|---|
| CB-001 | 平台凭证鉴权已开启 | 使用平台凭证访问主路径接口 | 1) 用平台凭证发起请求 2) 校验日志字段 | 请求通过；记录平台凭证上下文；`platform_credential_ingress_coverage_pct` 统计为有效样本 | 鉴权日志 + 指标快照 |
| CB-002 | 已开启凭证脱敏与导出审计 | 触发常见错误/导出账单 | 1) 构造4xx/5xx错误 2) 导出账单和审计 | 错误体、报表、导出中无可复用供应方上游凭证；`supplier_credential_exposure_events=0` | 错误样本 + 导出样本 + 脱敏扫描报告 |
| CB-003 | 出网审计与告警已开启 | 构造需求方绕过平台直连上游尝试 | 1) 从需求方网络直接访问上游 2) 校验告警与阻断 | 直连被阻断并告警；`direct_supplier_call_by_consumer_events` 记录并闭环处置 | 出网策略命中日志 + 安全事件记录 |
| CB-004 | query key 外部拦截策略已开启 | 外部 query key 请求（含 `/v1beta/*`） | 1) 发送带 query key 请求 2) 校验响应与计数 | 外部 query key 全拒绝；`query_key_external_reject_rate_pct=100%` | 网关拦截日志 + 指标快照 |

## 11. 分波次 Gate（Wave-CN / Wave-Global）

| Wave | 流量目标 | 观察窗口 | Go 条件（全部满足） | Stop 条件（任一触发） |
|---|---|---|---|---|
| Wave-CN-1 | CN 10% | 24h | P0 模块用例全绿；`cn_takeover=100%`（窗口内）；`route_mark_coverage>=99.9%`；5xx 与时延达标；`platform_credential_ingress_coverage_pct=100%` | 5xx > 基线+0.1% 或 账务差错率>0.1% 或 幂等冲突率>0.01% 或 `route_mark_coverage<99.9%` 或 `platform_credential_ingress_coverage_pct<100%` |
| Wave-CN-2 | CN 40% | 24h | 延续 Wave-CN-1 全绿；CN adapter 用例全绿；`route_mark_coverage>=99.9%`；`supplier_credential_exposure_events=0` | 同上，或出现协议不兼容回退，或 `supplier_credential_exposure_events>0` |
| Wave-CN-3 | CN 70% | 48h | Failover+Stream Guard 用例全绿；连续稳定；`route_mark_coverage>=99.9%`；`direct_supplier_call_by_consumer_events=0` | 同上，或出现流式 replay 异常，或 `direct_supplier_call_by_consumer_events>0` |
| Wave-CN-4 | CN 100% | 连续 7 天 | `cn_takeover=100%` 连续成立；账务核对通过；`route_mark_coverage>=99.9%`；`query_key_external_reject_rate_pct=100%` | 任一时段 `cn_takeover<100%` 或 `route_mark_coverage<99.9%` 或 `query_key_external_reject_rate_pct<100%` 立即降级 |
| Wave-Global-1 | 全量 20% | 24h | P0/P1 模块核心用例全绿；口径稳定；`route_mark_coverage>=99.9%`；`platform_credential_ingress_coverage_pct=100%` | 质量或账务任一红线触发，或 `route_mark_coverage<99.9%`，或 `platform_credential_ingress_coverage_pct<100%` |
| Wave-Global-2 | 全量 40% | 24h | 关键告警稳定；按租户拆分无异常尖刺；`route_mark_coverage>=99.9%`；`supplier_credential_exposure_events=0` | 同上，或 `supplier_credential_exposure_events>0` |
| Wave-Global-3 | 全量 60%+ | 连续 7 天 | `overall_takeover>=60%` 连续成立；抽样对账通过；`route_mark_coverage>=99.9%`；`supplier_credential_exposure_events=0`；`direct_supplier_call_by_consumer_events=0`；`query_key_external_reject_rate_pct=100%` | `overall_takeover<60%` 或任一红线触发，或 `route_mark_coverage<99.9%`，或任一凭证边界指标失效 |

## 12. 执行与出具报告要求

每轮 Wave 必交付以下证据包：

1. 模块测试报告（含失败重测记录）。
2. 接管率 SQL 原始结果（overall/cn + 趋势）。
3. 质量指标快照（P95、5xx、队列深度）。
4. 账务对账报告（请求级抽样，至少覆盖 Top 租户）。
5. 异常与回滚记录（如发生）。

验收结论模板：

1. 通过：满足当前 Wave 全部 Go 条件。
2. 有条件通过：存在已知风险但不触发 Stop，需明确补救期限。
3. 不通过：触发任一 Stop 条件，必须回切并复盘后重试。
