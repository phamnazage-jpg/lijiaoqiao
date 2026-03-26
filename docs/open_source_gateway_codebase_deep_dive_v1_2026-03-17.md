# 开源网关代码深度理解（v1）

- 版本：v1.0
- 日期：2026-03-17
- 范围：`sub2api-tar`、`new-api`、`one-api`、`litellm`
- 目标：为“Subapi First + 自研 Router Core 接管”提供源码级认知基线。

## 1. 目录与迁移确认

当前开源仓库已位于统一项目根：

- `/home/long/project/立交桥/llm-gateway-competitors`

核心仓库路径：

1. `/home/long/project/立交桥/llm-gateway-competitors/sub2api-tar`
2. `/home/long/project/立交桥/llm-gateway-competitors/new-api`
3. `/home/long/project/立交桥/llm-gateway-competitors/one-api`
4. `/home/long/project/立交桥/llm-gateway-competitors/litellm`

## 2. 法务与商用风险快照

1. `sub2api-tar`：MIT（可商用，仍需遵守上游 ToS）
2. `new-api`：AGPLv3（对闭源商用有强约束）
3. `one-api`：MIT
4. `litellm`：仓库主体 MIT，`enterprise/` 子目录有独立授权边界

结论：你的主线“商用闭源控制面”不应以 `new-api` 作为核心代码基座。

## 3. 架构入口（代码级）

## 3.1 sub2api-tar（推荐重点）

主入口与启动装配：

1. `/backend/cmd/server/main.go`（setup 模式 + 主服务启动）
2. `/backend/internal/server/routes/gateway.go`（协议路由汇总）

核心特点：

1. 明确的多协议入口：`/v1`、`/v1beta`、`responses/chat` 别名
2. 账号调度和 failover 逻辑深（适合借鉴路由中台能力）
3. 并发槽位、等待队列、流式错误处理比较完整

建议你优先深读：

1. `/backend/internal/handler/openai_gateway_handler.go`
2. `/backend/internal/handler/gateway_handler.go`
3. `/backend/internal/handler/gemini_v1beta_handler.go`
4. `/backend/internal/service/openai_gateway_service.go`
5. `/backend/internal/service/gateway_service.go`

## 3.2 new-api（功能广但法务受限）

主入口：

1. `/main.go`
2. `/router/relay-router.go`
3. `/controller/relay.go`

核心特点：

1. 协议覆盖广（OpenAI/Claude/Gemini/Responses/Realtime）
2. `middleware.Distribute()` + `controller.Relay()` 主干清晰
3. 计费预扣/结算逻辑完善，但 AGPL 影响商用策略

可借鉴点：

1. RelayFormat 分发模型
2. 路由组织和中间件链条设计

## 3.3 one-api（经典轻量基线）

主入口：

1. `/main.go`
2. `/router/relay.go`
3. `/middleware/distributor.go`

核心特点：

1. 架构简单，易读
2. 渠道选择以“可用通道 + 随机”为主，策略深度有限
3. 适合作为最小网关实现参考，不适合直接承载企业治理能力

## 3.4 litellm（生态和工程成熟度最高）

架构说明文件：

1. `/ARCHITECTURE.md`

关键目录：

1. `/litellm`（SDK 核心）
2. `/proxy`（AI Gateway）
3. `/router.py` + `/router_strategy`（路由策略）
4. `/tests`（测试资产非常完整）

核心特点：

1. SDK 与 Gateway 分层清晰
2. Proxy 在 SDK 上叠加 auth/rate-limit/budget/routing
3. 作为“多供应商适配能力参考”价值高

## 4. 代码体量画像（按文件后缀）

1. `sub2api-tar`：Go 1004，Vue 165，TS 139
2. `new-api`：Go 511，JSX 321
3. `one-api`：Go 235，JS 242
4. `litellm`：Python 3500，TSX 844，Markdown 869

解读：

1. `sub2api` 在 Go 侧中台能力深，适合做 S1/S2 接入基座。
2. `litellm` 在多 Provider 抽象和测试体系更强，适合策略与适配层借鉴。

## 5. 与你当前路线图的对齐建议

1. S1-S2 直接对齐 `subapi connector`：以 `sub2api-tar` 路由和 handler/service 为准做契约测试。
2. S2 国内供应商 100% 接管：优先把国内 Provider 的路由与计费从 `subapi` 迁出到自研 Router Core。
3. S3 机器人能力：优先消费你自研控制面的统一事件，不直接耦合 `subapi` 内部事件格式。

## 6. 下一轮深挖计划（v2）

建议按以下顺序继续深挖并输出第二版文档：

1. `sub2api` 调度链路时序图（选账号 -> 并发槽 -> failover -> usage 入账）
2. `sub2api` 计费链路字段表（请求级 idempotency 与 ledger 对齐）
3. `litellm` Router 策略可移植点（cost/latency/success 权重）
4. `new-api/one-api` 的可借鉴中间件清单（仅借鉴，不引入 AGPL 污染）

## 7. v2 文档已完成

已完成并落盘的 v2 深挖文档：

- `/home/long/project/立交桥/docs/sub2api_scheduler_billing_flow_deep_dive_v2_2026-03-17.md`

覆盖内容：

1. `SelectAccountWithScheduler` 三层调度策略与评分机制
2. 用户/账号并发槽位获取、等待队列与释放保障
3. Failover 触发条件、同账号重试、流式 no-replay 边界
4. `submitUsageRecordTask -> UsageRecordWorkerPool -> RecordUsage -> applyUsageBilling` 全链路
5. request_id + fingerprint 幂等扣费机制与冲突语义
