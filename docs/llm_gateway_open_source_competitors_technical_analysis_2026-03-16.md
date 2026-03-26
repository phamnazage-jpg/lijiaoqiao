# 商用 LLM 网关开源竞品技术分析（代码级证据版）

- 版本：v1.0
- 日期：2026-03-16
- 分析目标：为“商用通用 LLM 转发网关”提供可落地的技术选型与竞争策略依据
- 分析方法：以本地源码与文档为主，逐条代码证据比对

## 1. 样本与本地落地情况

本次纳入 4 个核心开源样本（与你产品定位最接近的“多模型网关/代理”）：

1. `one-api`
2. `new-api`
3. `sub2api`（你提到增长很快的项目）
4. `litellm`

### 1.1 本地路径

- one-api：`/home/long/project/立交桥/llm-gateway-competitors/one-api`
- new-api：`/home/long/project/立交桥/llm-gateway-competitors/new-api`
- sub2api：`/home/long/project/立交桥/llm-gateway-competitors/sub2api-tar`
- litellm：`/home/long/project/立交桥/llm-gateway-competitors/litellm`

### 1.2 代码规模快照（本地统计）

| 项目 | 主要语言文件量 | 测试文件量（粗略） | 文档文件量 |
|---|---:|---:|---:|
| one-api | Go 235 | 5 | 11 |
| new-api | Go 511 | 19 | 22 |
| sub2api | Go 1004, Vue 165, TS 139 | 417 | 18 |
| litellm | Python 3500 | 1846（测试目录与测试文件） | 869 |

注：对“测试覆盖率”只做“可见测试资产规模”判断，不等同于真实覆盖率。

## 2. 对比维度（商用网关视角）

本报告统一按 10 个维度做横向对比：

1. 协议与接口覆盖（OpenAI/Claude/Gemini/Realtime/Responses 等）
2. 调度与路由能力（负载、亲和、回退、重试、熔断）
3. 计费与额度一致性（预扣/后扣/退款、幂等）
4. 多租户与权限模型（用户、团队、Key、策略）
5. 限流与抗滥用（RPM/TPM、入口保护、故障策略）
6. 可观测与运营（日志、指标、后台运营）
7. 数据模型与可扩展性（Schema、状态字段、索引）
8. 工程成熟度（代码组织、测试资产、配置治理）
9. License 与商用合规风险
10. 二次开发成本与上线复杂度

## 3. 分项目技术剖析（含代码证据）

## 3.1 sub2api（生产中台倾向最强）

### 3.1.1 协议覆盖与网关组织

`sub2api` 的路由组织不是单一 OpenAI 兼容层，而是“多协议并列 + 专用平台路由”：

- `/v1` 兼容 Claude/OpenAI 风格，并提供 `messages`、`responses`、`chat/completions` 等能力  
  证据：`backend/internal/server/routes/gateway.go:40-78`
- `/v1beta` 兼容 Gemini 原生路径格式  
  证据：`backend/internal/server/routes/gateway.go:80-93`
- 提供不带 `/v1` 前缀的别名路由 `/responses`、`/chat/completions`，降低客户端迁移摩擦  
  证据：`backend/internal/server/routes/gateway.go:95-101`
- 提供 Antigravity / Sora 专用路由组（强制平台）  
  证据：`backend/internal/server/routes/gateway.go:105-157`

结论：接口层设计明显面向“多来源客户端兼容 + 业务隔离路由”。

### 3.1.2 调度能力（核心竞争点）

`OpenAIAccountScheduler` 明确实现三层选择：

- `previous_response_id` 粘性层
- `session_hash` 会话粘性层
- `load_balance` 负载层

证据：`backend/internal/service/openai_account_scheduler.go:17-21`, `224-289`

负载层不是随机，而是“多指标打分 + TopK + 加权随机顺序”：

- 指标包含优先级、负载、队列、错误率 EWMA、TTFT EWMA  
  证据：`.../openai_account_scheduler.go:656-676`
- 先选 TopK 再加权顺序，减少单账号长期垄断  
  证据：`.../openai_account_scheduler.go:416-449`, `506-559`, `678-687`

运行时反馈机制：

- 实时维护 `errorRateEWMABits`、`ttftEWMABits`  
  证据：`.../openai_account_scheduler.go:107-109`, `148-179`

结论：这是“调度系统”而非“随机通道器”。

### 3.1.3 限流与安全策略

认证高风险入口（注册/登录/2FA/验证码等）采用 Redis 限流且明确 `fail-close`：

- `auth-register`/`auth-login`/... 均配置 `FailureMode: RateLimitFailClose`  
  证据：`backend/internal/server/routes/auth.go:30-64`
- 限流器 Lua 原子计数 + TTL 修复，避免并发下 TTL 丢失  
  证据：`backend/internal/middleware/rate_limiter.go:28-39`, `98-121`

结论：安全设计偏保守，适合企业场景。

### 3.1.4 数据模型与运营可控性

`Account` schema 字段覆盖“可调度状态 + 限流恢复 + 过载恢复 + 会话窗口”：

- `schedulable`, `rate_limit_reset_at`, `overload_until`, `temp_unschedulable_until`  
  证据：`backend/ent/schema/account.go:143-180`

`APIKey` schema 含多窗口额度限制（5h/1d/7d）与窗口起点：

- `rate_limit_5h/1d/7d`, `usage_5h/1d/7d`, `window_*_start`  
  证据：`backend/ent/schema/api_key.go:78-117`

配置层默认值非常细，包含：

- billing circuit breaker  
  证据：`backend/internal/config/config.go:1180-1184`
- Redis 连接池与 DB 连接池高并发参数  
  证据：`.../config.go:1212-1227`
- OpenAI WS sticky TTL、TopK、scheduler 权重  
  证据：`.../config.go:1371-1383`
- 对 scheduler 权重合法性做校验（非负且总和>0）  
  证据：`.../config.go:2091-2105`

结论：`sub2api` 在“运营可控 + 调度深度 + 安全策略”上最接近企业中台。

## 3.2 new-api（协议覆盖强、功能扩展快）

### 3.2.1 协议覆盖面广

`new-api` 路由覆盖非常广，包含：

- OpenAI Chat/Completions/Embeddings/Audio/Responses/Realtime
- Claude messages
- Gemini `/v1beta/models/*`
- Midjourney、Suno、Video 等任务型接口

证据：`router/relay-router.go:69-201`, `168-223`

结论：协议与场景覆盖是其主要优势。

### 3.2.2 通道选择与重试策略

`Distribute` 层能力包含：

- token 级模型白名单限制
- auto 分组选择
- 通道亲和策略
- 重试通道重新选择

证据：`middleware/distributor.go:55-151`

`Relay` 主流程包含：

- 预扣费
- 请求重试循环（记录 used channel）
- 错误归一化
- 自动封禁异常通道（auto ban）

证据：`controller/relay.go:160-177`, `180-241`, `350-357`

### 3.2.3 计费一致性设计

`PreConsumeBilling` + `SettleBilling` 实现预扣与结算分离，支持 delta（补扣/返还）：

- `delta = actualQuota - preConsumedQuota`
- 统一会话对象执行 `Settle` / `Refund`

证据：`service/billing.go:17-26`, `32-78`

### 3.2.4 通道模型复杂度

`ChannelInfo` 支持 multi-key、轮询/随机、多 key 状态与禁用原因追踪：

证据：`model/channel.go:60-68`, `105-190`

结论：`new-api` 适合“快速接入多协议 + 高迭代需求”，但需要更强工程治理。

### 3.2.5 商用合规风险

License 明确 AGPLv3，并提示若组织不能接受需联系商业授权：

证据：`README.md:446-452`, `README.zh_CN.md:446-452`

结论：对闭源商用产品是显著法律约束点。

## 3.3 one-api（经典、简洁、低门槛）

### 3.3.1 网关结构与能力边界

`/v1` 路由具备 OpenAI 基础接口，但大量高级接口为 `RelayNotImplemented`：

证据：`router/relay.go:20-73`

### 3.3.2 分发策略

核心选择逻辑是“按 group+model 找可用通道 + 同优先级随机”：

- `CacheGetRandomSatisfiedChannel(...)`
- 同一优先级集合中 `rand.Intn`

证据：`middleware/distributor.go:45-59`, `model/cache.go:227-255`

### 3.3.3 计费模型

存在预扣返还与后扣逻辑，结构直接易懂：

证据：`relay/billing/billing.go:11-21`, `23-48`

### 3.3.4 商用注意

项目 LICENSE 文件是 MIT，但 README 对署名提出额外要求说明：

证据：`README.md:476-480`, `LICENSE`

结论：`one-api` 优势在“学习/部署快”，短板在“调度深度与企业治理能力”。

## 3.4 litellm（生态广、路由策略丰富、平台化强）

### 3.4.1 样本来源说明

本地分析样本来自 `litellm` 完整仓库代码（含 `tests/`、`docs/`、`enterprise/`、`proxy`）。

- 仓库 LICENSE 为 MIT  
  证据：`LICENSE`

### 3.4.2 Provider 与协议覆盖

Provider 列表非常长（含 OpenAI、Anthropic、Gemini、Bedrock、Azure、Groq、DeepSeek、OpenRouter 等）：

- `LITELLM_CHAT_PROVIDERS` 列表  
  证据：`litellm/constants.py:479-568`
- OpenAI 兼容 provider 列表  
  证据：`litellm/constants.py:728-783`

Proxy 端点具备：

- `/v1/models`
- `/v1/chat/completions`
- `/v1/embeddings`
- `/v1/realtime` WebSocket

证据：
- `litellm/proxy/proxy_server.py:6479-6709`
- `.../proxy_server.py:7074-7098`
- `.../proxy_server.py:7691-7704`

### 3.4.3 路由与可靠性策略

Router 构造参数内建：

- retries / fallbacks / content-policy fallbacks
- 多路由策略（least-busy/usage/latency/cost）
- provider budget config

证据：`litellm/router.py:242-299`, `516-539`, `617-642`, `756-833`

并发与限速控制：

- 每部署 `max_parallel_requests` semaphore
- 预调用检查 RPM/TPM

证据：`litellm/router.py:2057-2075`

`usage-based-routing-v2` 的跨实例限速逻辑明确依赖 Redis 计数：

- 设计注释“Meant to work across instances”
- `increment_cache` + TTL

证据：`litellm/router_strategy/lowest_tpm_rpm_v2.py:33-43`, `115-119`

### 3.4.4 预算与费用治理

`RouterBudgetLimiting` 支持按：

- provider budget
- deployment budget
- request tag budget

过滤超预算部署：

证据：`litellm/router_strategy/budget_limiter.py:116-189`, `191-280`

Proxy 端有大量 spend/budget 管理组件与后台任务：

- spend 更新、budget 重置任务、spend log 清理

证据：`litellm/proxy/proxy_server.py:910-933`, `5758-5781`, `5915-5945`

### 3.4.5 平台化能力

`proxy_server.py` 中 `include_router` 规模很大，涵盖 key/team/budget/model/fallback/compliance/analytics/guardrails 等：

证据：`litellm/proxy/proxy_server.py:13249-13318`

结论：`litellm` 在“生态接入广度 + 平台化管理能力”上非常强，适合作为“路由内核或能力参考源”。

## 4. 竞品对比矩阵（技术+商用）

评分区间：1（弱）~5（强）

| 维度 | sub2api | new-api | one-api | litellm |
|---|---:|---:|---:|---:|
| 协议覆盖 | 4.5 | 5.0 | 3.0 | 5.0 |
| 调度与路由深度 | 5.0 | 4.0 | 2.5 | 4.5 |
| 计费一致性 | 4.5 | 4.5 | 3.5 | 4.5 |
| 多租户与权限治理 | 4.5 | 4.0 | 3.0 | 4.5 |
| 限流与抗滥用 | 4.5 | 4.0 | 3.0 | 4.0 |
| 可观测与运营能力 | 4.5 | 4.0 | 3.0 | 4.5 |
| 工程成熟度（代码/测试/配置） | 4.7 | 4.0 | 3.2 | 4.2 |
| License 商用友好度 | 4.5 (MIT) | 2.0 (AGPLv3) | 4.0 (MIT+README附加说明) | 4.5 (MIT) |
| 二开难度（低分=容易） | 2.5 | 3.0 | 4.5 | 2.8 |

解读：

1. 如果你要做“高可信商用中台”：`sub2api` 的调度与治理思路最值得吸收。
2. 如果你要做“快速多协议覆盖”：`new-api` 与 `litellm` 的路由广度更有参考价值。
3. 如果你要“极快 MVP”：`one-api` 仍是最低心智负担底座，但后续重构成本高。
4. `new-api` 的 AGPLv3 需尽早法务评估，不建议直接深度嵌入闭源核心。

## 5. 对你产品的可执行技术路线（90 天）

目标：构建“企业可采购”的通用 LLM 网关，不陷入纯转发同质化。

## 阶段 A（第 1-3 周）：稳定底座

1. 统一北向协议：先收敛到 OpenAI + Anthropic + Gemini 三套核心接口。
2. 路由内核：实现 `fallback + retry + health + affinity` 四件套。
3. 计费一致性：上线 `pre-consume / settle / refund` 全链路，所有扣费具备 request_id 幂等键。

验收指标：

- P95 延迟、错误率、切换成功率可观测
- 扣费差错率 < 0.1%

## 阶段 B（第 4-8 周）：成本与治理

1. 按租户/团队/API Key/标签做预算体系。
2. 上线“策略层”：模型白名单、限流、敏感词、回退策略模板化。
3. 控制面增加运营能力：账号池状态、异常封禁、手动降级开关。

验收指标：

- 预算超限拦截准确率 > 99.9%
- 故障演练中自动切换恢复时间 < 30s

## 阶段 C（第 9-12 周）：企业化与差异化

1. 审计与合规：全链路审计日志、角色权限、密钥托管策略。
2. FinOps 面板：按团队/模型/供应商展示成本归因 + 优化建议。
3. 企业集成：Webhook、SIEM、账单导出、告警路由（Slack/飞书/钉钉）。

验收指标：

- 支持企业 PoC 的最小安全审计清单
- 提供“降本证明报表”（对比基线）

## 6. 关键风险与规避

1. **License 风险**：AGPLv3 组件若进入核心链路，闭源商用风险高。  
   规避：核心路径优先 MIT/Apache 组件，自研关键模块。
2. **转发同质化**：只做 API 兼容会被价格战挤压。  
   规避：把“成本优化 + 策略治理 + 合规审计”作为主卖点。
3. **计费可信度风险**：多模型多重试导致“账实不一致”。  
   规避：所有计费事件幂等化 + 对账作业 + 失败补偿队列。
4. **调度复杂度风险**：过早引入复杂路由导致系统不稳。  
   规避：先实现可解释策略，再逐步引入多指标打分。

## 7. 最终建议（针对你的项目定位）

结论性建议：

1. 参考 `sub2api` 的“调度中台化设计”作为核心能力蓝本。
2. 参考 `litellm` 的“多策略路由 + provider 生态接入”提升覆盖速度。
3. 参考 `new-api` 的协议覆盖清单，但避免 AGPL 风险进入闭源核心。
4. 不建议以 `one-api` 作为长期架构终态，可作为过渡 MVP 参考。

一句话战略：

**你的商用壁垒不在“接了多少模型”，而在“企业如何可控地、可审计地、持续降本地用模型”。**
