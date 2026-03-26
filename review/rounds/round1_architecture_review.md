# Round-1 架构与替换路径评审输出

- 评审日期：2026-03-19
- 对应任务：`EXP-002`

## 0. Skills 预审输入（2026-03-17）

来源：`docs/subapi_design_comprehensive_review_findings_v1_2026-03-17.md`

预置问题（会前必须预读）：

1. `FND-P0-01`：内网隔离与 mTLS 未进入两周硬任务，是否允许带风险进入替换阶段。
2. `FND-P1-05`：客户迁移缺少 SLA/沟通机制，是否影响商用推进节奏。
3. `FND-P1-06`：任务 owner 未实名，是否会影响 P0 事件处置时效。
4. `CB-SSOT-01`：是否已将“需求方仅用平台凭证入站、上游凭证零外发”写入 SSOT 并可验证。

## 1. 评审结论

- [ ] GO
- [ ] CONDITIONAL GO
- [ ] NO-GO

## 2. 主要问题

| 编号 | 等级 | 问题 | Owner | 截止日期 |
|---|---|---|---|---|
| R1-ISSUE-001 | P0 | 子系统边界安全未闭环：内网隔离与 mTLS 尚未形成硬门禁任务 | `SEC` + `PLAT` | 2026-03-20 |
| R1-ISSUE-002 | P1 | 迁移方案缺少“客户受影响时的沟通/SLA/补偿”标准流程 | `产品` + `CS` + `法务` | 2026-03-24 |
| R1-ISSUE-003 | P1 | P0/P1 任务 owner 尚未实名，升级授权链路风险较高 | `PMO` + `ARCH` | 2026-03-18 |
| R1-ISSUE-004 | P1 | 接管率验收口径历史存在 canonical/alias 混算风险，需固化分母 | `ARCH` + `FIN` | 2026-03-22 |
| R1-ISSUE-005 | P1 | 评审角色需要扩展到“用户代表、测试专家、网关专家”以覆盖真实使用与质量视角 | `ARCH` + `评审秘书` | 2026-03-18 |
| R1-ISSUE-006 | P0 | 凭证边界未形成硬门禁（M-013~M-016）前，不得推进 S2 升波 | `SEC` + `PLAT` + `QA` | 2026-03-24 |

## 3. 整改要求

1. P0 项必须在 Round-2 前关闭，否则默认转入 `NO-GO` 候选。
2. 所有 P1 项必须给出可验证证据（文档、日志、演练记录）并明确 owner+backup。
3. 用户代表、测试专家、网关专家必须进入至少一轮实质评审并形成书面意见。
4. 凭证边界必须满足：`M-013=0`、`M-014=100%`、`M-015=0`、`M-016=100%`。

## 4. 证据链接

1. `/home/long/project/立交桥/docs/subapi_design_comprehensive_review_findings_v1_2026-03-17.md`
2. `/home/long/project/立交桥/docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md`
3. `/home/long/project/立交桥/review/experts_roster_2026-03-18.md`
4. `/home/long/project/立交桥/docs/llm_gateway_subapi_evolution_plan_v4_2_2026-03-24.md`
5. `/home/long/project/立交桥/docs/acceptance_gate_single_source_v1_2026-03-18.md`
