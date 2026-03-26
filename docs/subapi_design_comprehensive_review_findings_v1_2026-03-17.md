# Subapi 集成与替换方案全面评审报告（Skills 专业评审版，v1）

- 评审日期：2026-03-17
- 评审方式：线上文档/代码证据审查（不进行线下评审）
- 评审目标：验证“可集成 subapi、可逐步替换、可达企业级商用 LLM 网关目标”的可执行性与风险闭环
- 评审技能视角：
  - `security`：安全与合规红线
  - `api-design`：协议契约与版本兼容
  - `backend`：架构可运维性与可靠性
  - `writing-clearly-and-concisely`：问题可执行表达

## 1. 评审结论

当前结论：`CONDITIONAL GO`（有条件推进）

必须先关闭以下门槛，才可进入外部专家 Round-1 正式博弈：

1. 关闭全部 P0（本报告共 2 项）。
2. 将 P1 中影响评审可信度的项（FND-P1-01/02/03/04）纳入强制整改排期与负责人。
3. 完成专家名册扩展（产品专家、重构项目专家、技术专家）并发出定向邀请。

## 2. Findings（按严重级别）

### 2.1 P0（阻断级）

| 编号 | 问题 | 证据 | 风险 | 建议整改 |
|---|---|---|---|---|
| FND-P0-01 | 设计要求了“内网隔离 + mTLS”，但两周任务单未把这两项作为显式交付任务。 | 设计要求见 `subapi_integration_compat_security_reliability_design_v1_2026-03-17.md:90-93`；任务单仅覆盖 egress 与配置硬化，未见 mTLS/内网暴露关闭任务：`subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md:58-76`。 | 若缺少东西向链路强认证与暴露面收敛，subapi 作为外部模块会成为横向移动入口，安全基线与审计责任不可闭环。 | 新增 `SEC-007`（subapi 内网隔离与公网不可达验证）+ `SEC-008`（网关<->subapi mTLS 双向证书与轮换演练），并设为 Week Gate 必过项。 |
| FND-P0-02 | query key 策略在“契约文档、设计文档、上游代码事实”三处存在边界歧义，存在误放行风险。 | 契约仍保留 `/v1beta/*` query key 兼容：`subapi_connector_contract_v1_2026-03-17.md:52,56`；我方设计要求北向入口禁止 query key、仅做内转：`subapi_integration_compat_security_reliability_design_v1_2026-03-17.md:105-107`；上游 Gemini 中间件确实允许特定路径 query key：`api_key_auth_google.go:146-157`。 | 如果网关边界策略不严，外部客户端可绕过 header-only 规范，导致密钥泄露面扩大、日志污染和审计判责困难。 | 在接口契约增加“北向/南向边界规则”专章；新增强制测试 `SEC-009`：外部 query key 全拒绝、内部改写链路可用且可审计。 |

### 2.2 P1（高优先级）

| 编号 | 问题 | 证据 | 风险 | 建议整改 |
|---|---|---|---|---|
| FND-P1-01 | 接管率 SQL 的主路径过滤与 Connector 契约存在冲突，口径有漂移风险。 | 主路径 SQL 包含 `inbound_endpoint IS NULL` 及 alias `/responses`：`router_core_takeover_metrics_sql_dashboard_v1_2026-03-17.md:75-83`；而 Connector 契约明确禁止 alias：`subapi_connector_contract_v1_2026-03-17.md:42-44`。 | 验收口径可能混入非 canonical 流量或历史脏数据，影响 `>=60% / 100%` 决策可信度。 | 固化 canonical-only 主路径过滤；把 alias 流量单独做异常面板，不计入验收分母。 |
| FND-P1-02 | `cn_platforms` 在 SQL 中仍以示例硬编码，缺少配置中心强绑定。 | 多处示例硬编码 `ARRAY['antigravity']`：`router_core_takeover_metrics_sql_dashboard_v1_2026-03-17.md:110,166,225,333`；虽有文字提示“不应硬编码”：`...:101,374`。 | 国内供应商清单变更后会导致统计失真，直接影响 `cn_takeover=100%` 验收。 | 新增配置表 `gateway_provider_taxonomy`，由 SQL 读取，禁止脚本内硬编码。 |
| FND-P1-03 | Wave Gate 没有把“路由标记覆盖率”纳入 Go 条件，评审可能在低覆盖数据上推进。 | 标记覆盖率 KPI 目标 `>=99.9%`：`router_core_takeover_metrics_sql_dashboard_v1_2026-03-17.md:384`；但 Wave-CN/Wave-Global Go 条件未包含该项：`router_core_s2_acceptance_test_cases_v1_2026-03-17.md:99-107`。 | 数据覆盖不足时，接管率结论不可靠，容易误升波或误回切。 | 在 Wave Gate 增加硬条件：`route_mark_coverage_pct >= 99.9%`，否则一律 Stop。 |
| FND-P1-04 | 专家评审轮次文件已补充预审输入，但仍缺“owner/截止日期/证据链接”的实填项。 | 预审输入已加入：`round1_architecture_review.md:6-15`、`round2_compat_billing_review.md:6-15`、`round3_security_compliance_review.md:6-15`、`round4_reliability_wargame_review.md:6-15`；正式问题清单区仍待会后填实：`round1...:24-35` 等。 | 若会后不立即回填，问题闭环会断档，影响 `EXP-006` 决议可信度。 | 会后 4 小时内必须完成 owner/due date/证据链接回填，并纳入评审秘书检查清单。 |
| FND-P1-05 | 迁移方案有技术灰度，但缺少客户成功侧“迁移中断沟通与赔付/SLA”机制。 | 迁移目标强调留存与成功率：`llm_gateway_subapi_evolution_plan_v2_2026-03-17.md:107-119`；未定义客户沟通/SLA 补救机制。 | 迁移期若出现协议回归或账务争议，商业层面容易出现升级投诉和续费风险。 | 新增 `PROD-001` 客户迁移事件分级与告知模板、`PROD-002` 账务争议 SLA 与赔付边界。 |
| FND-P1-06 | 任务单职责仍是角色占位，未落到实名与备份人。 | 角色占位说明：`subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md:20-31`。 | 发生 P0 时可能出现授权链路不清、恢复动作延迟。 | 启动会上完成实名 RACI + backup owner，并写入任务单 v1.1。 |

### 2.3 P2（建议优化）

| 编号 | 问题 | 证据 | 风险 | 建议整改 |
|---|---|---|---|---|
| FND-P2-01 | 术语“subapi/sub2api”混用，跨文档追踪成本偏高。 | 多份文档并存两种命名：例如 `sub2api_integration_readiness_checklist_2026-03-16.md:1-4` 与 `llm_gateway_subapi_evolution_plan_v2_2026-03-17.md:15-16`。 | 评审/实施时易产生“同名异义”沟通误差。 | 统一术语字典：对外叫 `subapi`，代码与仓库名保持 `sub2api`，在文档页首固定声明。 |
| FND-P2-02 | 专家执行台账字段粒度不足，难以快速看清“优先级+邀请状态+轮次覆盖”。 | 旧名册/台账字段偏基础：`review/experts_roster_2026-03-18.md:9-20`、`review/invitation_dispatch_tracker_2026-03-17.md:8-19`。 | 邀请执行过程不够透明，不利于秘书做催办与替补。 | 扩展为“专家类型/优先级/邀请状态/预计轮次/是否已签 NDA”。（本次已更新） |

## 3. 整改任务映射（新增与绑定）

| Finding | 任务ID | 类型 | 截止日期 | 验收标准 |
|---|---|---|---|---|
| FND-P0-01 | SEC-007 | 新增 | 2026-03-20 | subapi 对公网不可达，且有网络策略与扫描证据 |
| FND-P0-01 | SEC-008 | 新增 | 2026-03-24 | mTLS 双向认证联通，证书轮换演练通过 |
| FND-P0-02 | SEC-009 | 新增 | 2026-03-21 | 外部 query key 全拒绝；内部改写链路可观测 |
| FND-P1-01 | COMP-007 | 新增 | 2026-03-22 | 主路径 SQL 与 canonical 契约一致，alias 单列监控 |
| FND-P1-02 | COMP-008 | 新增 | 2026-03-22 | `cn_platforms` 改为配置表驱动 |
| FND-P1-03 | COMP-009 | 新增 | 2026-03-23 | Wave Gate 加入覆盖率硬门槛并生效 |
| FND-P1-04 | EXP-002~005 | 绑定 | 2026-03-19 起 | 四轮评审均有会前问题清单与会后闭环 |
| FND-P1-05 | PROD-001/002 | 新增 | 2026-03-24 | 发布客户沟通与赔付/SLA 流程文档 |
| FND-P1-06 | PMO-001 | 新增 | 2026-03-18 | 任务单实名 owner + backup owner 完成 |

## 4. 专家博弈输入建议（会前统一口径）

1. 先审 P0，再谈路线优选：P0 不闭环时不进入扩流讨论。
2. Round-1 必问三题：锁定风险、止血路径、替换可逆性。
3. Round-2 必核数据口径：canonical 分母、覆盖率门槛、CN 平台清单来源。
4. Round-3 必做攻防演练：query key 绕过、SSRF、内网访问阻断、审计追踪。
5. Round-4 必做恢复演练：10 分钟触发回切、30 分钟恢复、账务一致性验收。

## 5. 当前执行状态（本次已完成）

1. 已完成 skills 内部全面评审并形成分级 findings。
2. 已更新专家名册与邀请跟踪字段，显式纳入产品专家、重构项目专家、技术专家。
3. 已生成逐人邀请文本，可直接发送。

## 6. 三角色联合复审回链（2026-03-18）

1. 已补充三角色联合评审文档：`subapi_role_based_review_wargame_optimization_v1_2026-03-18.md`。
2. 已将用户代表、测试专家、网关专家纳入专家名册与邀请跟踪（E13/E14/E15）。
3. 已将联合评审任务映射到执行任务单（`UXR-*`、`TST-*`、`GAT-*`、`EXP-007`）。
