# Superpowers 全面规划评审报告

- 版本：v1.0
- 日期：2026-03-25
- 评审方法：`superpowers`（架构/测试/业主/UIUX 四维交叉）+ 文档一致性核对
- 评审范围：
  - PRD 与 SSOT：`llm_gateway_prd_v1_2026-03-25.md`、`llm_gateway_subapi_evolution_plan_v4_2_2026-03-24.md`
  - 门禁与任务：`acceptance_gate_single_source_v1_2026-03-18.md`、`subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md`
  - 供应侧设计链：按钮 PRD、OpenAPI、测试清单、技术增强、测试增强、UIUX 规范
  - 执行证据链：preflight、SUP Gate 汇总与专项报告

---

## 1. 结论

结论：**CONDITIONAL GO（设计理解已闭环，执行证据未闭环）**

判定依据：
1. 需求边界与凭证红线已统一（用户A -> 平台 -> 用户B，且上游凭证不外发）。
2. 供应侧三页面（账号/套餐/结算）已具备按钮级、接口级、测试级、门禁级定义。
3. 仍存在影响发布的高优先级缺口（契约冻结状态、幂等头契约缺失、执行链路阻塞）。

---

## 2. 需求与功能清单理解确认

## 2.1 需求边界（确认）

1. 业务边界：平台售卖服务能力，不售卖供应方上游凭证。
2. 安全边界：需求方仅持平台凭证入站，外部 query key 全拒绝。
3. 验收边界：以 `M-013~M-016` 和 SUP Gate 为强约束。

## 2.2 功能清单（确认）

1. 供应账号挂载（验证、创建、激活、暂停、删除、审计）。
2. 套餐发布管理（草稿、上架、暂停、下架、批量调价、复制）。
3. 收益结算提现（刷新、提现、撤销、对账单、流水）。
4. 安全与审计（脱敏扫描、凭证边界回归、事件可追踪）。

---

## 3. 发现清单（按严重级别）

## 3.1 P0 发现

### P0-01：功能文档“冻结状态”与“待拍板项”并存，无法作为最终实施输入

- 证据：
  - 按钮级 PRD 标注为“草案”：`supply_button_level_prd_v1_2026-03-25.md:3`
  - 同文档存在“待拍板项”：`supply_button_level_prd_v1_2026-03-25.md:236`
- 风险：接口和测试可能按不同口径实现，导致回归不稳定。
- 处置建议：将待拍板项迁移为“已决议记录”，文档状态改为“冻结”。
- 状态：**Closed（2026-03-27）**
- 关闭证据：
  - `docs/supply_button_level_prd_v1_2026-03-25.md`（v1.1，已改“冻结”并替换为“已决议项”）
  - `docs/product/supply_prd_pending_to_decision_map_v1_2026-03-27.md`
  - `review/outputs/supply_prd_decision_meeting_minutes_2026-03-27.md`

### P0-02：技术设计强制的幂等头未进入 OpenAPI 契约

- 证据：
  - 技术增强稿要求必填 `X-Request-Id` 与 `Idempotency-Key`：`supply_technical_design_enhanced_v1_2026-03-25.md:37`
  - OpenAPI 未定义上述 header 参数（路径定义中无对应参数）：`supply_api_contract_openapi_draft_v1_2026-03-25.yaml:22`
- 风险：客户端无法按契约实现幂等，后端幂等策略无法端到端落地。
- 处置建议：在 OpenAPI 全量写操作中加入 header 参数并标注 required。
- 状态：**Closed（2026-03-27）**
- 关闭证据：
  - `docs/supply_api_contract_openapi_draft_v1_2026-03-25.yaml`（`XRequestIdHeader`、`IdempotencyKeyHeader` 已定义并挂载到写操作）
  - `docs/supply_api_contract_openapi_draft_v1_2026-03-25.yaml`（`Conflict` 与 `AcceptedInProgress` 示例）
  - `docs/supply_technical_design_enhanced_v1_2026-03-25.md`（2.2 节标注契约已落地）

### P0-03：SUP 执行链路阻塞，发布证据不可得

- 证据：
  - preflight 失败：`API_BASE_URL` 不可达：`supply_gate_preflight_2026-03-25.md:16`
  - SUP 汇总结论“不通过”：`supply_gate_review_2026-03-31.md:9`
- 风险：`SUP-004~SUP-007` 无实测证据，无法转 GO。
- 处置建议：修复可达环境地址 + 平台短期 token，重跑并回填报告。

## 3.2 P1 发现

### P1-01：测试追踪矩阵 API 路径与 OpenAPI 实际路径不一致（缺前缀）

- 证据：
  - 测试增强矩阵写 `/accounts` 等短路径：`supply_test_plan_enhanced_v1_2026-03-25.md:40`
  - OpenAPI 实际为 `/api/v1/supply/...`：`supply_api_contract_openapi_draft_v1_2026-03-25.yaml:22`
- 风险：自动追踪或契约校验脚本难以直接关联。
- 处置建议：统一改为完整路径，或在矩阵中新增 `api_alias` 字段。

### P1-02：SUP 报告签署责任尚未实名，无法完成审计闭环

- 证据：`supply_gate_review_2026-03-31.md:15`（Owner 待实名），签署栏空缺：`:29`
- 风险：发布审计与问责链条不完整。
- 处置建议：按 RACI 回填实名+电子签署流水号。

### P1-03：PRD 的全局 P0 能力与供应侧 P0 映射仍有覆盖缺口

- 证据：
  - PRD P0 包含“预算与配额、告警与通知、账单导出”等：`llm_gateway_prd_v1_2026-03-25.md:85`
  - 当前供应侧映射聚焦账号/套餐/结算与边界：`:223`
- 风险：业务方可能误判“全局 P0 已全部按钮化”。
- 处置建议：补充“全局 P0 到供应侧/平台侧”完整映射矩阵。

## 3.3 P2 发现

### P2-01：`supplier` 与 `supply` 路径命名混用，认知成本偏高

- 证据：
  - 账单接口：`/api/v1/supplier/billing`：`supply_api_contract_openapi_draft_v1_2026-03-25.yaml:274`
  - 其余接口主要使用 `/api/v1/supply/...`
- 风险：理解成本增加，非致命但影响一致性。
- 处置建议：给出统一命名规则和兼容策略（保留 alias 或重定向）。

---

## 4. 评审后的建议优先级

1. 先关 P0：冻结状态冲突、幂等契约缺口、执行阻塞。
2. 再关 P1：追踪矩阵路径一致化、实名签署、全局功能映射补齐。
3. 最后关 P2：命名风格统一与历史兼容策略。

---

## 5. 发布准入建议

满足以下全部条件再从 `CONDITIONAL GO` 转 `GO`：
1. P0-01/P0-02/P0-03 全部关闭并有证据。
2. `SUP-004~SUP-007` 至少一次全量 PASS，且 M-013~M-016 达标。
3. SUP 汇总报告完成实名签署与审计留痕。
