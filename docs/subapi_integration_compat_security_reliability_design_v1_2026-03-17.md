# Subapi 集成兼容性、安全与运维可靠性设计（v1.1）

- 版本：v1.1
- 日期：2026-03-24
- 适用阶段：S1-S2
- 关联文档：
  - `subapi_connector_contract_v1_2026-03-17.md`
  - `sub2api_integration_readiness_checklist_2026-03-16.md`
  - `router_core_takeover_execution_plan_v3_2026-03-17.md`
  - `acceptance_gate_single_source_v1_2026-03-18.md`（v1.1）
  - `llm_gateway_subapi_evolution_plan_v4_2_2026-03-24.md`

## 1. 结论先行（回答四个核心问题）

1. 我们是否考虑了与 subapi 的技术兼容性：
- 已考虑了“契约层兼容”和“灰度回滚”，但还缺“版本漂移自动检测 + 协议回归自动阻断 + 兼容风险分级”的闭环。

2. 我们是否了解 subapi 的安全风险：
- 已识别部分风险，但需要把风险从“清单级”升级到“强制控制级”。
- 典型高风险包括：URL allowlist 默认关闭、私网/HTTP 默认放开、Gemini 路径保留 query key 兼容、Simple 模式会跳过关键计费校验。

3. 集成 subapi 如何防止风险：
- 必须采用“外部服务模块化 + 网络隔离 + 配置硬化 + 双层鉴权 + 契约测试闸门 + 安全告警”组合拳，不接受单点措施。

4. 架构是否兼顾运维简单和可靠性：
- 当前方向正确（灰度、回滚、观测已有），但还需补齐“最小化运维复杂度”的具体机制：单一控制面、统一配置发布、标准化 runbook、以及故障域隔离（cell 化）。

## 2. 兼容性设计（防止实施期协议翻车）

## 2.1 已有基础（可复用）

1. Connector 契约已定义 canonical 端点、错误归一、流式边界、重试约束。
2. S2 迁移路径已定义 Wave 灰度与 stop/go 条件。
3. 已有接管率与验收用例文档可做质量门禁基础。

## 2.2 仍需补齐的兼容闭环

新增三层闸门（必须全部启用）：

1. **Schema Gate（接口形态）**
- 校验请求/响应 JSON 结构、必填字段、字段类型、错误码结构。
- 重点接口：`/v1/messages`、`/v1/chat/completions`、`/v1/responses`、`/v1beta/models/*`。

2. **Behavior Gate（行为语义）**
- 校验 streaming 行为、首 token 前后错误处理、no-replay 规则。
- 校验 header 优先级、模型映射、会话粘性、fallback 行为。

3. **Performance Gate（性能与稳定）**
- 校验 P95、5xx、超时率、账务差错率、幂等冲突率。
- 不达标直接阻断升级，回退上一稳定版本。

## 2.3 兼容性风险分级（建议）

| 等级 | 定义 | 示例 | 处理策略 |
|---|---|---|---|
| P0 | 会导致错误计费、协议不可用或大面积失败 | 流式 replay、错误码语义变化导致无限重试 | 立即阻断上线，强制回退 |
| P1 | 影响部分场景或部分租户 | 某端点字段新增导致旧 SDK 解析失败 | 灰度暂停，48h 内修复 |
| P2 | 非核心功能偏差 | 次要元数据缺失 | 记录并进入下个迭代 |

## 3. subapi 安全风险台账（针对当前代码事实）

## 3.1 配置默认值风险

1. `security.url_allowlist.enabled=false`（默认关闭）。
2. `allow_private_hosts=true`、`allow_insecure_http=true`（默认放开）。
3. 这意味着若不硬化，SSRF/内网访问风险面会扩大。

## 3.2 认证与参数风险

1. 常规 API 已拒绝 query key（`key`/`api_key`）。
2. 但 Gemini 路径保留了 `query key` 兼容（`/v1beta*`），需要在我方入口再做强制拦截与改写策略。

## 3.3 运行模式风险

1. `run_mode=simple` 下，认证后会跳过部分计费/订阅校验流程。
2. 生产若误用 simple，会带来额度与计费控制失效风险。

## 3.4 代理与来源 IP 风险

1. `server.trusted_proxies` 默认空数组。
2. 虽有告警日志，但若部署链路未正确设置，来源 IP 信任链可能失真，影响风控与审计。

## 3.5 合规与法律风险

1. 上游仓库 README 明确提示 ToS 风险与研究用途声明。
2. MIT 许可仅覆盖开源代码使用，不覆盖上游模型服务条款合规。

## 4. 风险防护设计（集成必须项）

## 4.1 网络与边界隔离

1. subapi 只允许内网访问，不直接暴露公网。
2. 主网关与 subapi 之间启用 mTLS（至少双向证书校验）。
3. 出网层做 egress allowlist（域名/IP 双层），禁直连内网和不受信目的地。

## 4.2 配置硬化基线（生产强制）

1. `run_mode=standard`。
2. `security.url_allowlist.enabled=true`。
3. `security.url_allowlist.allow_private_hosts=false`。
4. `security.url_allowlist.allow_insecure_http=false`。
5. `billing.circuit_breaker.enabled=true`。
6. `server.trusted_proxies` 必须显式配置。

## 4.3 认证与密钥策略

1. 我方北向入口禁止 query key；统一只接收 header 鉴权。
2. 对 Gemini 兼容流量，在入口层将 query key 转换为 header 后再转发，外部请求直接带 query key 一律拒绝。
3. API Key 与上游凭证分离管理，凭证仅在 Adapter 层短时解密。
4. 需求方仅可使用平台签发凭证访问平台入口，`platform_credential_ingress_coverage_pct` 必须为 100%。
5. 禁止向需求方返回供应方上游凭证（API/控制台/导出/错误信息均不允许）。
6. 需求方绕过平台直连供应方视为 P0 安全事件，必须可观测、可告警、可阻断。

## 4.4 版本与发布治理

1. 固定 subapi 精确版本（`vX.Y.Z`），禁止漂移。
2. 每次升级必须通过 Schema/Behavior/Performance 三重 Gate。
3. 灰度比例：5% -> 20% -> 50% -> 100%。
4. 任一 P0 风险触发，自动回退上一稳定版本。

## 4.5 观测与审计

1. 强制 request_id 全链路透传。
2. 记录 `router_engine`、`inbound_endpoint`、`upstream_endpoint`、`request_type`。
3. 安全告警纳入统一事件中心：
- query key 拦截次数
- 非法上游域名命中
- 私网地址访问拦截
- 账务冲突率突增
- 供应方上游凭证泄露事件（`supplier_credential_exposure_events`）
- 需求方绕过平台直连供应方事件（`direct_supplier_call_by_consumer_events`）
- 平台凭证入站覆盖率下降（`platform_credential_ingress_coverage_pct < 100%`）

## 5. 运维简单 + 可靠性架构（目标态）

## 5.1 运维简单（减少人肉操作）

1. **单一控制面**：所有路由开关、灰度比例、熔断阈值在我方控制面发布。
2. **单一发布流水线**：subapi 升级与 Router Core 升级共享同一套 Gate 与回滚动作。
3. **标准化运行手册**：按“告警 -> 判断 -> 操作 -> 验证”四段式 Runbook 固化。

## 5.2 高可靠（避免级联故障）

1. **故障域隔离（Cell）**：按租户或区域切分 subapi 实例池，避免单点故障扩散。
2. **双通路兜底**：自研主路径 + subapi connector 兜底，并支持一键回切。
3. **幂等与补偿**：请求级幂等扣费 + 冲突告警 + 对账补偿任务。
4. **流式保护**：首字输出后禁止 replay，防止双流拼接与重复扣费。

## 5.3 SLO 与错误预算（建议）

1. 可用性 SLO：99.9%（网关维度）。
2. 附加延迟 SLO：P95 <= 60ms。
3. 账务正确性 SLO：差错率 <= 0.1%，冲突率 <= 0.01%。
4. 每周审查 error budget；超预算自动冻结升波。

## 6. 两周内可落地动作（最小闭环）

1. 新增“兼容三重 Gate”流水线，并接入升级流程。
2. 生产配置硬化巡检（按 4.2 清单逐项验收）。
3. 在入口层落地“query key 全拦截（含 Gemini 兼容改写）”。
4. 建立安全告警面板（SSRF 拦截、query key 拦截、账务冲突）。
5. 增加凭证边界专项检查（上游凭证零外发 + 平台凭证入站覆盖率100%）。
6. 完成一次“升级 + 灰度 + 自动回滚”演练并沉淀复盘。

## 7. 验收标准（本设计是否落地）

1. 任一 subapi 升级都能产出 Gate 报告并可追溯。
2. 生产环境不存在宽松高风险配置（4.2 全部满足）。
3. 发生兼容或安全异常时，30 分钟内可回切到稳定版本。
4. 需求方仅使用平台凭证入站，`platform_credential_ingress_coverage_pct = 100%`。
5. `supplier_credential_exposure_events = 0` 且 `direct_supplier_call_by_consumer_events = 0`。
6. 运维团队按 Runbook 可独立处置常见告警，无需临时拍脑袋决策。

## 8. 代码证据（用于本设计判断）

1. 默认安全配置与告警日志：
`backend/internal/config/config.go`
2. API Key 鉴权与 simple mode 逻辑：
`backend/internal/server/middleware/api_key_auth.go`
3. Gemini 认证优先级与 query key 兼容：
`backend/internal/server/middleware/api_key_auth_google.go`
4. URL 校验器（含 allowlist/private/insecure 与 DNS rebinding 注释）：
`backend/internal/util/urlvalidator/validator.go`
5. trusted proxies 与 release 模式告警：
`backend/internal/server/http.go`
6. 上游 ToS 风险提示：
`README.md`（sub2api 仓库）

## 9. 执行任务单（新增）

为将本设计转换为可执行排期，新增任务单：

1. `subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md`
   - 两周里程碑（2026-03-18 至 2026-03-31）
   - 任务 ID / 责任角色 / 截止日期 / 验收标准 / 证据产物
   - Daily Gate / Weekly Gate / 回滚演练闭环
2. `subapi_expert_review_wargame_plan_v1_2026-03-17.md`
   - 专家组成、对抗式博弈规则、评分模型与一票否决条件
   - 四轮评审（架构/兼容计费/安全合规/可靠性演练）与最终决议机制
