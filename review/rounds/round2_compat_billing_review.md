# Round-2 兼容与计费一致性评审输出

- 评审日期：2026-03-22
- 对应任务：`EXP-003`

## 0. Skills 预审输入（2026-03-17）

来源：`docs/subapi_design_comprehensive_review_findings_v1_2026-03-17.md`
补充来源：`docs/subapi_role_based_review_wargame_optimization_v1_2026-03-18.md`

预置问题（会前必须预读）：

1. `FND-P1-01`：主路径 SQL 包含 alias/空端点，是否与 canonical 契约冲突。
2. `FND-P1-02`：`cn_platforms` 硬编码示例未配置化，是否影响 `cn_takeover=100%` 判断。
3. `FND-P1-03`：Wave Gate 缺少 `route_mark_coverage_pct>=99.9%` 的硬门槛。
4. `TST-001/TST-002`：契约漂移与流式高压回归是否已具备阻断能力。
5. `GAT-001`：Provider 能力矩阵是否已覆盖全部已接入供应商。
6. `CB-002`：需求方调用是否 100% 使用平台凭证，且无供应方上游凭证透出。

## 1. 评审结论

- [ ] GO
- [x] CONDITIONAL GO（预审建议，待会议确认）
- [ ] NO-GO

## 2. 兼容差异清单

| 编号 | 协议/端点 | 问题描述 | 风险等级 | Owner |
|---|---|---|---|---|
| R2-COMP-001 | 主路径统计口径（canonical） | 接管率分母需严格限定 canonical 端点，禁止混入 alias/空端点历史数据 | P1 | `ARCH` + `FIN` |
| R2-COMP-002 | CN 平台识别口径 | `cn_platforms` 必须从配置中心/配置表读取，禁止 SQL 硬编码 | P1 | `PLAT` + `FIN` |
| R2-COMP-003 | 契约漂移检测 | 升级前必须有契约漂移 CI 阻断，失败即停止发布 | P0 | `QA` + `PLAT` |
| R2-COMP-004 | 流式与 Failover 组合场景 | 高压场景下 no-replay + 切换策略需有固定回归报告 | P0 | `QA` + `SRE` |
| R2-COMP-005 | Provider 能力矩阵 | 已接入供应商能力矩阵未全量固化时，不得扩接新供应商 | P1 | `ARCH` + `PLAT` |
| R2-COMP-006 | 入站凭证覆盖率 | `platform_credential_ingress_coverage_pct` 必须持续等于 100% | P0 | `PLAT` + `SEC` |
| R2-COMP-007 | 凭证泄露防护 | 错误体/报表/导出不得出现可复用上游凭证 | P0 | `SEC` + `QA` |

## 3. 账务风险清单

| 编号 | 场景 | 风险描述 | 风险等级 | Owner |
|---|---|---|---|---|
| R2-BILL-001 | 幂等冲突告警 | 冲突告警已定义，但需验证是否能阻断继续升波 | P0 | `FIN` + `SRE` |
| R2-BILL-002 | 账务争议处理 | 用户侧争议 SLA 与补偿边界需形成对外可执行文本 | P1 | `产品` + `FIN` + `法务` |
| R2-BILL-003 | 升波证据包 | 升波审批缺少标准化账务抽样与 trace 证据包模板 | P1 | `QA` + `FIN` |
| R2-BILL-004 | 凭证边界证据包 | 每次升波需提交 M-013~M-016 指标快照与日志证据 | P0 | `QA` + `SEC` + `SRE` |

## 4. 证据链接

1. `/home/long/project/立交桥/docs/subapi_design_comprehensive_review_findings_v1_2026-03-17.md`
2. `/home/long/project/立交桥/docs/router_core_takeover_metrics_sql_dashboard_v1_2026-03-17.md`
3. `/home/long/project/立交桥/docs/router_core_s2_acceptance_test_cases_v1_2026-03-17.md`
4. `/home/long/project/立交桥/docs/subapi_role_based_review_wargame_optimization_v1_2026-03-18.md`
5. `/home/long/project/立交桥/docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md`
6. `/home/long/project/立交桥/docs/llm_gateway_subapi_evolution_plan_v4_2_2026-03-24.md`
7. `/home/long/project/立交桥/docs/acceptance_gate_single_source_v1_2026-03-18.md`
