# Round-3 安全与合规攻防评审输出

- 评审日期：2026-03-25
- 对应任务：`EXP-004`

## 0. Skills 预审输入（2026-03-17）

来源：`docs/subapi_design_comprehensive_review_findings_v1_2026-03-17.md`
补充来源：`docs/subapi_role_based_review_wargame_optimization_v1_2026-03-18.md`

预置问题（会前必须预读）：

1. `FND-P0-01`：内网隔离+mTLS 未明确排期，是否满足上线安全红线。
2. `FND-P0-02`：query key 边界策略存在歧义，是否存在外部绕过路径。
3. `FND-P1-04`：安全轮次是否具备可执行的问题清单与闭环责任人。
4. `TST-001`：安全相关契约漂移是否接入 CI 阻断。
5. `UXR-001`：安全事件是否具备 15 分钟用户通知机制。
6. `CB-SEC-01`：凭证边界是否可验证（M-013~M-016）。

## 1. 评审结论

- [ ] GO
- [x] CONDITIONAL GO（预审建议，待会议确认）
- [ ] NO-GO

## 2. 安全问题清单

| 编号 | 风险点 | 等级 | 影响范围 | Owner | 截止日期 |
|---|---|---|---|---|---|
| R3-SEC-001 | subapi 内网隔离与公网不可达未完成验证 | P0 | 数据面与控制面边界 | `SEC` + `SRE` | 2026-03-20 |
| R3-SEC-002 | 网关<->subapi mTLS 双向认证和轮换未完成演练 | P0 | 东西向链路安全 | `PLAT` + `SEC` | 2026-03-24 |
| R3-SEC-003 | query key 外拒内转边界未完成全链路强测 | P0 | 鉴权与审计链路 | `SEC` + `QA` | 2026-03-21 |
| R3-SEC-004 | 契约漂移 CI 阻断未形成强制门禁 | P1 | 发布流程 | `QA` + `PLAT` | 2026-03-22 |
| R3-SEC-005 | 安全事件 15 分钟用户通知链路待实测 | P1 | 用户与客户成功 | `产品` + `CS` | 2026-03-22 |
| R3-SEC-006 | 供应方上游凭证泄露检测与脱敏基线未完成 | P0 | 凭证与日志安全 | `SEC` + `PLAT` | 2026-03-26 |
| R3-SEC-007 | 需求方绕过平台直连供应方检测未上线 | P0 | 出网与边界安全 | `SEC` + `SRE` | 2026-03-27 |
| R3-SEC-008 | 平台凭证入站覆盖率未达 100% | P0 | 北向鉴权链路 | `PLAT` + `SEC` | 2026-03-26 |

## 3. 合规结论

1. ToS 审查结论：待法务确认（SEC-006）。
2. 数据审计结论：待补充查询链路与导出证据样本。
3. 需要法务补充项：低成本账号模块边界与用户告知条款一致性。
4. 凭证边界合规结论：需求方仅使用平台凭证，供应方上游凭证零外发（待证据包确认）。

## 4. 一票否决项检查

| 项目 | 结果 | 说明 |
|---|---|---|
| 安全 P0 清零 | 否（预审） | `R3-SEC-001/002/003` 未关闭前默认不通过 |
| 合规 P0 清零 | 待确认 | 依赖 `SEC-006` 法务结论与留档证据 |
| 凭证边界 P0 清零 | 待确认 | 依赖 `R3-SEC-006/007/008` 与 `M-013~M-016` 达标 |

## 5. 证据链接

1. `/home/long/project/立交桥/docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md`
2. `/home/long/project/立交桥/docs/subapi_integration_compat_security_reliability_design_v1_2026-03-17.md`
3. `/home/long/project/立交桥/docs/acceptance_gate_single_source_v1_2026-03-18.md`
4. `/home/long/project/立交桥/docs/llm_gateway_subapi_evolution_plan_v4_2_2026-03-24.md`
