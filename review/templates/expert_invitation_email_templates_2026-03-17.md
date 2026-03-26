# 专家评审邀请函模板（邮件）

- 模板版本：v1.0
- 日期：2026-03-17

## 1. 总体邀请（首封）

**邮件主题：**【邀请评审】商用 LLM 网关 subapi 集成与替换路径专家审核（2026-03-19~2026-03-31）

**正文模板：**

各位专家好，

我们将启动“Subapi 集成与替换路径”专家审核与博弈机制，目标是确保：

1. 能稳定集成 subapi 到现有系统。
2. 能按阶段平滑替换 subapi 关键能力。
3. 最终达成企业级商用 LLM 网关目标（稳定、可审计、可运维、可合规）。

评审方式为 Red vs Blue 对抗式审核，共四轮：

1. Round-1（2026-03-19）：架构与替换路径
2. Round-2（2026-03-22）：兼容与计费一致性
3. Round-3（2026-03-25）：安全与合规攻防
4. Round-4（2026-03-29）：可靠性与回滚演练

请确认是否接受邀请，并于会前 24 小时完成预读材料。

本次评审输出将形成 GO / CONDITIONAL GO / NO-GO 决议。

谢谢。

发起人：{姓名}
日期：{日期}

## 2. Round-1 邀请模板（架构）

**邮件主题：**【Round-1】架构与替换路径评审（2026-03-19）

请重点关注：

1. 是否存在被 subapi 锁死的架构路径。
2. 接管率目标（全供应商 >=60%、国内供应商 100%）是否可达。
3. 失败场景下 30 分钟止血路径是否清晰。

附件：

1. `llm_gateway_subapi_evolution_plan_v2_2026-03-17.md`
2. `router_core_takeover_execution_plan_v3_2026-03-17.md`
3. `subapi_expert_review_wargame_plan_v1_2026-03-17.md`

## 3. Round-2 邀请模板（兼容+计费）

**邮件主题：**【Round-2】兼容性与计费一致性评审（2026-03-22）

请重点关注：

1. 协议兼容（OpenAI/Anthropic/Gemini）是否有盲区。
2. 流式 no-replay 与错误归一语义是否一致。
3. 幂等扣费、冲突告警与对账闭环是否完整。

附件：

1. `subapi_connector_contract_v1_2026-03-17.md`
2. `router_core_takeover_metrics_sql_dashboard_v1_2026-03-17.md`
3. `router_core_s2_acceptance_test_cases_v1_2026-03-17.md`

## 4. Round-3 邀请模板（安全+合规）

**邮件主题：**【Round-3】安全与合规攻防评审（2026-03-25）

请重点关注：

1. URL allowlist、query key、run_mode、trusted_proxies 风险是否收敛。
2. 出网策略、密钥管理、审计可追溯是否达企业标准。
3. ToS 风险是否有明确可接受结论。

附件：

1. `subapi_integration_compat_security_reliability_design_v1_2026-03-17.md`
2. `sub2api_integration_readiness_checklist_2026-03-16.md`

## 5. Round-4 邀请模板（可靠性演练）

**邮件主题：**【Round-4】可靠性与回滚演练评审（2026-03-29）

请重点关注：

1. 失败升级是否可自动回退。
2. 是否能在 30 分钟内恢复服务。
3. Runbook 是否可由值班团队独立执行。

附件：

1. 回滚演练记录
2. 告警看板快照
3. Runbook v1
