# Subapi 集成风险控制实施任务单（两周执行版，v1.5）

- 版本：v1.5
- 日期：2026-03-27
- 执行窗口：2026-03-18 至 2026-03-31（两周）
- 关联文档：
  - `subapi_integration_compat_security_reliability_design_v1_2026-03-17.md`
  - `subapi_expert_review_wargame_plan_v1_2026-03-17.md`
  - `router_core_takeover_execution_plan_v3_2026-03-17.md`
  - `router_core_takeover_metrics_sql_dashboard_v1_2026-03-17.md`
  - `router_core_s2_acceptance_test_cases_v1_2026-03-17.md`
  - `acceptance_gate_single_source_v1_2026-03-18.md`（v1.1, 2026-03-24）
  - `llm_gateway_subapi_evolution_plan_v4_2_2026-03-24.md`
  - `supply_button_level_prd_v1_2026-03-25.md`（v1.1 冻结，2026-03-27）
  - `supply_api_contract_openapi_draft_v1_2026-03-25.yaml`
  - `supply_ui_test_cases_executable_v1_2026-03-25.md`
  - `supply_gate_command_playbook_v1_2026-03-25.md`
  - `supply_technical_design_enhanced_v1_2026-03-25.md`
  - `supply_test_plan_enhanced_v1_2026-03-25.md`
  - `supply_uiux_design_spec_v1_2026-03-25.md`
  - `database_domain_model_and_governance_v1_2026-03-27.md`
  - `dependency_compatibility_audit_baseline_v1_2026-03-27.md`
  - `tests/supply/ui_design_qa_cases_v1_2026-03-25.md`
  - `reports/supply_gate_preflight_2026-03-25.md`
  - `review/multi_expert_planning_review_v1_2026-03-25.md`

## 1. 执行目标（两周必须达成）

1. 建立 subapi 升级“兼容三重 Gate”（Schema/Behavior/Performance）并接入发布前闸门。
2. 完成生产配置硬化，消除已识别高风险默认项。
3. 建立安全告警和回滚演练闭环，确保 30 分钟内可回切。
4. 将风险控制纳入日常运维流程，避免靠临时人工判断。
5. 建立“凭证边界”硬门禁：需求方仅用平台凭证，供应方上游凭证零外发。
6. 建立供应侧发布门禁链路（SUP）：账号挂载 -> 套餐发布 -> 结算提现全链路可验收。
7. 建立四专家整改发布链路（XR）：技术/测试/UIUX/业主条款与门禁统一闭环。
8. 建立 token 运行态交付链路（TOK）：从实现、部署到门禁验收可追踪闭环。

## 2. 责任角色映射（实名RACI）

| 角色 | 实名负责人（主/备） | 职责 |
|---|---|---|
| `ARCH`（架构负责人） | 王磊 / 赵凯 | 兼容策略、闸门标准、最终技术裁决 |
| `PLAT`（平台工程） | 李娜 / 陈涛 | 流水线、配置发布、网关接入改造 |
| `SEC`（安全负责人） | 周敏 / 郭强 | 安全基线、威胁验证、告警策略 |
| `SRE`（稳定性负责人） | 刘洋 / 韩雪 | 监控、演练、故障响应与回滚 |
| `QA`（测试负责人） | 孙悦 / 吴航 | 契约回归、流式边界、验收报告 |
| `FIN`（计费/数据） | 何静 / 彭程 | 对账、幂等冲突监控、成本异常告警 |

说明：任务表中的角色标识（`ARCH/PLAT/SEC/SRE/QA/FIN`）按本表实名映射执行，并纳入 on-call 值班表。

## 3. 两周里程碑（绝对日期）

| 里程碑 | 截止日期 | 验收条件 |
|---|---|---|
| M1：基线冻结 | 2026-03-20 | 风险清单冻结；硬化项可检测 |
| M2：兼容闸门联通 | 2026-03-24 | 三重 Gate 在 CI 可执行并有报告 |
| M3：安全硬化完成 | 2026-03-27 | 高风险默认项全部改为生产安全值 |
| M4：回滚演练通过 | 2026-03-30 | 升级失败自动回退演练完成 |
| M5：两周验收 | 2026-03-31 | 交付证据包齐全并评审通过 |

## 4. 任务清单（可直接排期）

## 4.1 Workstream A：兼容性 Gate

| 任务ID | 任务 | Owner | 截止日期 | 依赖 | 验收标准 | 证据产物 |
|---|---|---|---|---|---|---|
| COMP-001 | 固化 canonical 端点矩阵（OpenAI/Anthropic/Gemini） | `ARCH` + `QA` | 2026-03-19 | 无 | 端点矩阵冻结并评审通过 | `docs/compat/canonical_endpoint_matrix.md` |
| COMP-002 | 产出 Schema Gate 用例（请求/响应/错误结构） | `QA` | 2026-03-21 | COMP-001 | 覆盖核心 6 条链路；失败可定位到字段级 | `tests/compat/schema_gate_report.md` |
| COMP-003 | 产出 Behavior Gate 用例（流式/no-replay/重试） | `QA` | 2026-03-22 | COMP-001 | 覆盖流式边界与错误语义；无歧义 case | `tests/compat/behavior_gate_report.md` |
| COMP-004 | 产出 Performance Gate 阈值脚本（P95/5xx/账务） | `SRE` + `FIN` | 2026-03-23 | COMP-001 | 阈值可配置，支持阻断 | `scripts/gate/perf_gate_check.sh` |
| COMP-005 | 三重 Gate 接入发布流水线 | `PLAT` | 2026-03-24 | COMP-002/003/004 | 发布前自动执行，任一失败阻断发布 | CI 记录 + Gate 汇总报告 |
| COMP-006 | 定义兼容风险分级处置（P0/P1/P2） | `ARCH` | 2026-03-24 | COMP-005 | 每级别有明确响应时限与动作 | `docs/compat/risk_severity_playbook.md` |
| COMP-007 | 主路径 SQL 与 canonical 契约对齐（移除 alias/空端点歧义） | `ARCH` + `FIN` | 2026-03-22 | COMP-001 | 验收分母仅包含 canonical 主路径 | `sql/takeover_main_path_canonical.sql` |
| COMP-008 | 国内平台清单配置化（替代 SQL 硬编码） | `PLAT` + `FIN` | 2026-03-22 | COMP-007 | `cn_platforms` 来自配置表/配置中心 | `docs/compat/cn_platform_mapping.md` |
| COMP-009 | Wave Gate 增加 `route_mark_coverage>=99.9%` 硬门槛 | `ARCH` + `QA` | 2026-03-23 | COMP-007 | 覆盖率不达标自动 Stop | Wave Gate 配置快照 + 演练记录 |

## 4.2 Workstream B：安全硬化

| 任务ID | 任务 | Owner | 截止日期 | 依赖 | 验收标准 | 证据产物 |
|---|---|---|---|---|---|---|
| SEC-001 | 生产配置扫描器（检测高风险默认项） | `SEC` + `PLAT` | 2026-03-20 | 无 | 可检测 run_mode/url_allowlist/private/http/trusted_proxies | `scripts/security/config_hardening_scan.sh` |
| SEC-002 | 生产环境配置硬化发布（标准模式+URL 策略） | `PLAT` | 2026-03-25 | SEC-001 | 所有环境通过扫描，无高危项 | 发布变更单 + 前后配置 diff |
| SEC-003 | 北向入口 query key 全拦截策略上线 | `PLAT` + `SEC` | 2026-03-25 | SEC-001 | 外部 query key 请求全部拒绝并告警 | 网关策略配置 + 拦截日志样本 |
| SEC-004 | Gemini 兼容请求 header 改写策略（内转） | `PLAT` | 2026-03-26 | SEC-003 | 兼容客户端可用，且不暴露 query key 通路 | 联调记录 + 回归测试报告 |
| SEC-005 | 出网 egress allowlist 与私网访问阻断 | `SEC` + `SRE` | 2026-03-27 | SEC-002 | 未授权域名/私网访问被阻断 | 防火墙/代理策略快照 + 拦截告警 |
| SEC-006 | ToS 合规审查记录归档（法务接口） | `SEC` | 2026-03-27 | 无 | 上游条款风险有书面结论 | `compliance/subapi_tos_assessment_2026-03-27.pdf` |
| SEC-007 | subapi 内网隔离与公网不可达验证 | `SEC` + `SRE` | 2026-03-20 | SEC-001 | subapi 服务不对公网开放，扫描验证通过 | 网络策略清单 + 连通性测试报告 |
| SEC-008 | 网关<->subapi mTLS 双向认证与证书轮换演练 | `PLAT` + `SEC` | 2026-03-24 | SEC-007 | 双向证书校验生效，轮换不影响可用性 | mTLS 配置 + 轮换演练报告 |
| SEC-009 | query key 外拒内转策略强制测试 | `SEC` + `QA` | 2026-03-27 | SEC-003, SEC-004 | 外部 query key 全拒绝，内部改写链路可追踪 | `tests/security/query_key_boundary_report.md` |
| SEC-010 | 供应方上游凭证泄露扫描与脱敏基线 | `SEC` + `PLAT` | 2026-03-26 | SEC-002 | `supplier_credential_exposure_events=0`，日志/报表无敏感片段 | `tests/security/credential_exposure_scan_report.md` |
| SEC-011 | 需求方绕过平台直连供应方检测策略上线 | `SEC` + `SRE` | 2026-03-27 | SEC-005 | `direct_supplier_call_by_consumer_events=0` 可观测可告警 | `docs/security/direct_supplier_call_detection_v1.md` |
| SEC-012 | 平台凭证入站覆盖率审计任务 | `PLAT` + `SEC` | 2026-03-26 | SEC-003 | `platform_credential_ingress_coverage_pct=100%` | `reports/security/platform_credential_ingress_coverage_2026-03-26.md` |

## 4.3 Workstream C：运维简单与可靠性

| 任务ID | 任务 | Owner | 截止日期 | 依赖 | 验收标准 | 证据产物 |
|---|---|---|---|---|---|---|
| REL-001 | 单一控制面发布流程定义（变更入口统一） | `ARCH` + `PLAT` | 2026-03-21 | 无 | 路由开关/灰度/熔断统一入口 | `docs/ops/unified_change_flow.md` |
| REL-002 | 安全+兼容+质量告警看板搭建 | `SRE` | 2026-03-26 | COMP-005, SEC-002 | 包含 query key、SSRF、冲突率、takeover | 看板截图 + 指标定义清单 |
| REL-003 | 回滚自动化脚本（版本回切） | `PLAT` + `SRE` | 2026-03-27 | COMP-005 | 10 分钟内触发回切，30 分钟内恢复 | `scripts/release/rollback_subapi.sh` + 演练日志 |
| REL-004 | Runbook 标准化（告警->判断->操作->验证） | `SRE` | 2026-03-28 | REL-002 | 至少覆盖 8 类高频告警 | `docs/runbook/subapi_integration_runbook_v1.md` |
| REL-005 | 一次完整演练（升级失败自动回退） | `SRE` + `QA` | 2026-03-30 | REL-003, REL-004 | 演练成功且复盘闭环 | 演练记录 + 复盘报告 |
| REL-006 | 两周验收评审与风险复盘 | `ARCH` + 全员 | 2026-03-31 | 全部任务 | 验收结论明确（通过/有条件通过/不通过） | `reports/sprint_risk_control_review_2026-03-31.md` |
| REL-007 | 凭证边界告警看板（M-013~M-016） | `SRE` + `SEC` | 2026-03-27 | SEC-010, SEC-011, SEC-012 | 凭证边界指标分钟级可观测并支持阈值告警 | 看板截图 + 告警策略快照 |

## 4.4 Workstream D：专家审核与博弈

| 任务ID | 任务 | Owner | 截止日期 | 依赖 | 验收标准 | 证据产物 |
|---|---|---|---|---|---|---|
| EXP-001 | 确认专家名单与角色回避规则 | `ARCH` + `SEC` | 2026-03-18 | 无 | 专家名单冻结（含用户代表/测试专家/网关专家），独立性规则确认 | `review/experts_roster_2026-03-18.md` |
| EXP-002 | Round-1 架构与替换路径评审 | `ARCH` | 2026-03-19 | EXP-001 | 形成问题清单与决策建议 | `review/rounds/round1_architecture_review.md` |
| EXP-003 | Round-2 兼容与账务一致性评审 | `QA` + `FIN` | 2026-03-22 | COMP-002, COMP-003 | 兼容差异与账务风险可追踪 | `review/rounds/round2_compat_billing_review.md` |
| EXP-004 | Round-3 安全与合规攻防评审 | `SEC` | 2026-03-25 | SEC-002, SEC-003 | 安全/合规 P0 是否清零有明确结论 | `review/rounds/round3_security_compliance_review.md` |
| EXP-005 | Round-4 可靠性与回滚演练评审 | `SRE` | 2026-03-29 | REL-003, REL-004 | 演练满足 30 分钟恢复目标 | `review/rounds/round4_reliability_wargame_review.md` |
| EXP-006 | 专家最终决议（GO/CONDITIONAL GO/NO-GO） | `ARCH` + 管理层 | 2026-03-31 | EXP-002~005 | 决议与风险接受记录齐全 | `review/final_decision_2026-03-31.md` |

## 4.5 Workstream E：产品与项目治理闭环（新增）

| 任务ID | 任务 | Owner | 截止日期 | 依赖 | 验收标准 | 证据产物 |
|---|---|---|---|---|---|---|
| PROD-001 | 迁移异常客户沟通模板与分级机制 | `产品` + `CS` | 2026-03-24 | EXP-002 | 迁移异常 30 分钟内有标准对外沟通 | `docs/product/migration_incident_comms_v1.md` |
| PROD-002 | 账务争议 SLA 与补偿边界定义 | `产品` + `FIN` + `法务` | 2026-03-24 | EXP-003 | 客户争议处理时限与补偿规则可执行 | `docs/product/billing_dispute_sla_v1.md` |
| PMO-001 | 任务实名 RACI 与备份负责人落地 | `PMO` + `ARCH` | 2026-03-18 | 无 | 所有 P0/P1 任务均有 owner+backup | `reports/raci_snapshot_2026-03-18.md` |

## 4.6 Workstream F：三角色联合评审落地（用户/测试/网关）

| 任务ID | 任务 | Owner | 截止日期 | 依赖 | 验收标准 | 证据产物 |
|---|---|---|---|---|---|---|
| UXR-001 | 用户代表迁移旅程验收走查（含告警通知链路） | `产品` + `CS` + `用户代表` | 2026-03-25 | PROD-001 | 迁移异常场景 15 分钟内通知链路实测通过 | `reports/user_representative_migration_walkthrough_2026-03-25.md` |
| UXR-002 | 用户代表账务争议流程演练与反馈闭环 | `产品` + `FIN` + `用户代表` | 2026-03-25 | PROD-002 | 争议流程演练通过且用户侧反馈关闭 | `reports/user_billing_dispute_drill_2026-03-25.md` |
| TST-001 | 契约漂移检测接入 CI 阻断 | `QA` + `PLAT` | 2026-03-25 | COMP-005 | 漂移失败自动阻断发布 | `tests/compat/contract_drift_ci_report.md` |
| TST-002 | 流式+Failover 高压回归套件落地 | `QA` + `SRE` | 2026-03-27 | COMP-003, REL-002 | no-replay 与切换策略在高压下稳定通过 | `tests/compat/stream_failover_stress_report.md` |
| TST-003 | 升波证据包模板标准化 | `QA` + `SRE` | 2026-03-23 | COMP-009 | 每次升波均产出统一证据包 | `evidence/*/wave_gate_bundle.md` |
| TST-004 | 凭证边界回归测试（平台凭证入站/上游凭证不外发） | `QA` + `SEC` | 2026-03-27 | SEC-010, SEC-012 | 用例失败自动阻断发布 | `tests/security/credential_boundary_regression_report.md` |
| GAT-001 | Provider 能力矩阵与缺口清单 | `ARCH` + `PLAT` | 2026-03-22 | COMP-001 | 已接入供应商能力矩阵覆盖率 100% | `docs/gateway/provider_capability_matrix_v1.md` |
| GAT-002 | 三层降级策略与演练脚本 | `ARCH` + `SRE` | 2026-03-28 | REL-003 | 演练可在 30 分钟内止血恢复 | `docs/gateway/degrade_playbook_v1.md` |
| GAT-003 | Adapter SPI 版本兼容规范 | `ARCH` | 2026-03-26 | GAT-001 | 新增适配器均有 SPI 兼容校验 | `docs/gateway/adapter_spi_versioning_v1.md` |
| EXP-007 | 三角色联合复审（用户/测试/网关） | `ARCH` + `QA` + `产品` | 2026-03-27 | UXR-001, TST-001, GAT-001 | 形成联合复审结论并决定是否继续升波 | `docs/subapi_role_based_review_wargame_optimization_v1_2026-03-18.md` |

## 4.7 Workstream G：供应侧发布门禁链路（SUP，新增）

| 任务ID | 任务 | Owner | 截止日期 | 依赖 | 验收标准 | 证据产物 |
|---|---|---|---|---|---|---|
| SUP-001 | 供应侧按钮级 PRD 冻结（3 页面） | `产品` + `ARCH` | 2026-03-26 | 无 | 页面字段、按钮、状态机、错误码冻结 | `docs/supply_button_level_prd_v1_2026-03-25.md`（v1.1 冻结） |
| SUP-002 | 供应侧 OpenAPI 契约冻结（3 页面） | `PLAT` + `ARCH` | 2026-03-26 | SUP-001 | 请求/响应字段、枚举、错误码冻结 | `docs/supply_api_contract_openapi_draft_v1_2026-03-25.yaml` |
| SUP-003 | UI-SUP 可执行用例评审通过 | `QA` + `产品` | 2026-03-27 | SUP-001, SUP-002 | `UI-SUP-*` + `UI-DESIGN-QA-*` 全量可执行，覆盖按钮/状态/权限/可访问性 | `docs/supply_ui_test_cases_executable_v1_2026-03-25.md` + `tests/supply/ui_design_qa_cases_v1_2026-03-25.md` |
| SUP-004 | 账号挂载链路联调（验证/创建/激活/暂停） | `PLAT` + `QA` | 2026-03-28 | SUP-002, SUP-003 | `UI-SUP-ACC-001~006` 通过率 100% | `scripts/supply-gate/sup004_accounts.sh` + `tests/supply/ui_sup_acc_report_2026-03-28.md` |
| SUP-005 | 套餐发布链路联调（草稿/上架/暂停/下架/复制） | `PLAT` + `QA` | 2026-03-29 | SUP-002, SUP-003 | `UI-SUP-PKG-001~006` 通过率 100% | `scripts/supply-gate/sup005_packages.sh` + `tests/supply/ui_sup_pkg_report_2026-03-29.md` |
| SUP-006 | 结算提现链路联调（刷新/提现/撤销/导出） | `PLAT` + `FIN` + `QA` | 2026-03-29 | SUP-002, SUP-003 | `UI-SUP-SET-001~005` 通过率 100%，状态机无跳态 | `scripts/supply-gate/sup006_settlements.sh` + `tests/supply/ui_sup_set_report_2026-03-29.md` |
| SUP-007 | 供应侧凭证边界专项回归（SEC-SUP） | `SEC` + `QA` | 2026-03-30 | SUP-004, SUP-005, SUP-006 | `SEC-SUP-001~002` 通过，M-013~M-016 持续达标 | `scripts/supply-gate/sup007_boundary.sh` + `tests/supply/sec_sup_boundary_report_2026-03-30.md` |
| SUP-008 | 供应侧 Gate 汇总与发布结论 | `ARCH` + `QA` + `产品` | 2026-03-31 | SUP-004~SUP-007 | SUP Gate 结论为通过或有条件通过 | `reports/supply_gate_review_2026-03-31.md` |

## 4.8 Workstream H：四专家整改与复核链路（XR，新增）

| 任务ID | 任务 | Owner | 截止日期 | 依赖 | 验收标准 | 证据产物 |
|---|---|---|---|---|---|---|
| XR-001 | 供应侧技术设计增强落地（幂等/并发/不变量/事务） | `ARCH` + `PLAT` | 2026-03-26 | SUP-002 | 关键写路径均具备双键幂等和冲突语义 | `docs/supply_technical_design_enhanced_v1_2026-03-25.md` |
| XR-002 | 供应侧测试方案增强落地（追踪矩阵+并发重放） | `QA` + `ARCH` | 2026-03-27 | XR-001 | Requirement->API->Test->Metric->Gate 全量可追踪，且路径一致性检查通过 | `docs/supply_test_plan_enhanced_v1_2026-03-25.md` + `reports/supply_traceability_matrix_2026-03-25.csv` + `docs/supply_traceability_matrix_generation_rules_v1_2026-03-27.md` + `reports/supply_flaky_budget_2026-03-25.md` |
| XR-003 | 供应侧 UI/UX 规范与设计验收清单落地 | `产品` + `UIUX` + `QA` | 2026-03-27 | SUP-003 | DQA P0=0，P1 通过率>=95% | `docs/supply_uiux_design_spec_v1_2026-03-25.md` |
| XR-004 | 业主 SLA/申诉/赔付条款并入门禁验收 | `产品` + `CS` + `FIN` | 2026-03-28 | XR-002, XR-003 | 条款可执行可测且签字确认 | `docs/product/owner_sla_dispute_compensation_rules_v1.md` |
| XR-005 | 四专家再次对齐复核并形成发布结论 | `ARCH` + `QA` + `产品` + `UIUX` | 2026-03-28 | XR-001~XR-004 | 复核结论明确（GO/CONDITIONAL GO/NO-GO） | `review/multi_expert_alignment_recheck_v1_2026-03-25.md` |

## 4.9 Workstream I：数据库与依赖质量闭环（新增）

| 任务ID | 任务 | Owner | 截止日期 | 依赖 | 验收标准 | 证据产物 |
|---|---|---|---|---|---|---|
| DB-001 | 跨域核心表基线落地（Core/IAM/Auth/Billing/Audit） | `ARCH` + `PLAT` | 2026-03-27 | XR-001 | `platform_core_schema_v1.sql` 可执行且评审通过 | `sql/postgresql/platform_core_schema_v1.sql` |
| DB-002 | 供应域加密/单位/审计字段与索引补齐 | `PLAT` + `QA` | 2026-03-28 | DB-001 | patch 可幂等执行，关键查询计划不回退 | `sql/postgresql/supply_schema_v1_patch_2026-03-27.sql` |
| DB-003 | 数据模型与迁移策略文档并入 SSOT | `ARCH` | 2026-03-28 | DB-001, DB-002 | 迁移顺序、回滚策略、验收清单完整 | `docs/database_domain_model_and_governance_v1_2026-03-27.md` |
| DEP-001 | 依赖兼容审计四件套接入发布流程 | `PLAT` + `SEC` | 2026-03-28 | COMP-005 | SBOM/锁差异/兼容矩阵/风险清单缺一阻断 | `docs/dependency_compatibility_audit_baseline_v1_2026-03-27.md` |
| DEP-002 | 分阶段质量门禁（G0-G5）接入 CI | `QA` + `PLAT` | 2026-03-29 | DEP-001, XR-002 | `M-018` 与 `M-019` 自动计算并阻断 | CI 记录 + Gate 汇总 |
| DEP-003 | 需求-设计-测试漂移日检机制上线 | `PMO` + `QA` | 2026-03-29 | DEP-002 | 发现漂移 24h 内闭环，周报可追踪 | `reports/design_drift_daily_*.md` |

## 4.10 Workstream J：token 运行态实现与验收闭环（TOK，新增）

| 任务ID | 任务 | Owner | 截止日期 | 依赖 | 验收标准 | 证据产物 |
|---|---|---|---|---|---|---|
| TOK-001 | token 能力最小实现清单冻结（签发/校验/吊销/续期/审计） | `ARCH` + `SEC` + `PLAT` | 2026-03-28 | SUP-002 | 功能边界、接口与状态机冻结，禁止再口头变更 | `docs/token_runtime_minimal_spec_v1.md` |
| TOK-002 | 平台鉴权与 token 校验中间件实现（仅平台凭证入站） | `PLAT` + `SEC` | 2026-03-30 | TOK-001 | 外部请求必须通过平台凭证校验，覆盖率=100% | 开发阶段：`docs/token_auth_middleware_design_v1_2026-03-29.md` + `docs/platform_token_api_contract_openapi_draft_v1_2026-03-29.yaml`；联调阶段：实现代码 + 单测报告 |
| TOK-003 | token 生命周期实现（签发/短期TTL/吊销/轮换） | `PLAT` | 2026-03-31 | TOK-001 | 生命周期状态可追踪，吊销生效延迟满足阈值 | 开发阶段：`docs/token_lifecycle_audit_test_assertions_v1_2026-03-29.md`；联调阶段：实现代码 + 集成测试报告 |
| TOK-004 | 安全审计与事件入库（签发/鉴权失败/吊销/越权） | `SEC` + `PLAT` | 2026-03-31 | TOK-002, TOK-003 | 审计事件完整入库，可按租户/角色追踪 | 开发阶段：`docs/token_lifecycle_audit_test_assertions_v1_2026-03-29.md`；联调阶段：审计表样例 + 查询结果 |
| TOK-005 | 凭证边界联调（SUP-007 合并复测） | `SEC` + `QA` | 2026-04-01 | TOK-002~TOK-004 | M-013~M-016 在 staging 实测全部达标 | 开发阶段：`scripts/supply-gate/tok005_boundary_dryrun.sh` + `reports/archive/gate_verification/tok005_dryrun_*.md`；联调阶段：`tests/supply/sec_sup_boundary_report_2026-03-30.md`（staging回填） |
| TOK-006 | staging 一键回归（SUP-004~SUP-007 + TOK） | `QA` + `PLAT` | 2026-04-01 | TOK-005 | 全链路通过且无 mock 依赖 | 开发阶段：`scripts/supply-gate/tok006_gate_bundle.sh` + `scripts/ci/superpowers_stage_validate.sh` + `reports/archive/gate_verification/tok006_gate_bundle_*.md` + `reports/archive/gate_verification/superpowers_stage_validation_*.md` + `reports/archive/gate_verification/tok006_release_decision_onepager_template_v1_2026-03-30.md`；联调阶段：`reports/archive/gate_verification/sup_run_all_staging_*.log` + 实测单页判定报告 |
| TOK-007 | 发布门禁复审（并入 EXP-006 决议） | `ARCH` + `QA` + `SEC` | 2026-04-03 | TOK-006 | F-04 关闭，生产决议可重新评估 | 开发阶段：`scripts/ci/tok007_release_recheck.sh` + `scripts/ci/final_decision_consistency_check.sh` + `scripts/ci/tok007_generate_final_decision_candidate.sh` + `review/outputs/tok007_release_recheck_*.md`（活文档只允许引用当时明确标记为现行稿的最新一版，旧批次一律按历史快照处理） + `review/outputs/final_decision_candidate_from_tok007_*.md`（同前） + `reports/archive/gate_verification/final_decision_consistency_*.md` + `reports/archive/gate_verification/tok007_generate_candidate_*.log`；联调阶段：`review/final_decision_2026-03-31.md`（复审回填） |

## 5. 验收门禁（每日/每周）

## 5.1 Daily Gate（每日 18:00）

1. 高危配置扫描是否全部通过。
2. 兼容 Gate 失败数是否为 0。
3. 账务冲突率是否 <= 0.01%。
4. `query_key_external_reject_rate_pct` 是否 = 100%（否则即 P0）。
5. `platform_credential_ingress_coverage_pct` 是否 = 100%（否则即 P0）。
6. `supplier_credential_exposure_events` 是否 = 0（非0即 P0）。
7. `direct_supplier_call_by_consumer_events` 是否 = 0（非0即 P0）。
8. `route_mark_coverage_pct` 是否 >= 99.9%（不足即禁止升波）。
9. 迁移异常是否在 15 分钟内完成用户通知（未达标即 P1）。
10. 契约漂移检测是否通过（未通过即阻断发布）。
11. 供应侧 UI Gate 是否全绿（`UI-SUP-ACC-* / UI-SUP-PKG-* / UI-SUP-SET-*`）。
12. 供应侧凭证边界专项（`SEC-SUP-*`）是否全绿（失败即 P0）。
13. 四专家整改链路（XR-001~XR-003）是否全绿（未完成即禁止进入 SUP-008 结论环节）。
14. 数据库补丁任务（DB-001~DB-003）是否按阶段达成（未完成即禁止升波）。
15. 依赖兼容审计四件套是否完整（缺任一项即阻断发布）。
16. 分阶段质量门禁 `M-018/M-019` 是否持续 = 100%（否则回退到失败阶段）。
17. token 运行态链路（TOK-002~TOK-006）是否完成（未完成即禁止生产 GO）。

## 5.2 Weekly Gate（2026-03-24 / 2026-03-31）

1. 是否满足 M2 / M5 里程碑验收条件。
2. 是否触发 P0 事件（触发则冻结升级）。
3. 凭证边界指标（M-013~M-016）是否连续 7 天达标。
4. 是否完成回滚演练并达到 30 分钟恢复目标。
5. 是否完成当周专家评审并关闭必须整改项。
6. 供应侧 Gate（SUP-004~SUP-008）是否完成并出具结论。
7. 四专家复核链路（XR-001~XR-005）是否完成并形成签署结论。
8. DB/依赖质量链路（DB-* / DEP-*）是否全量关闭。
9. 依赖兼容审计指标 `M-017` 是否连续 7 天达标。
10. 阶段质量与追踪覆盖指标 `M-018/M-019` 是否连续 7 天达标。
11. token 运行态审计缺口（`TOK-REAL-001~003`）是否全部关闭。

## 6. 风险与阻断规则

| 触发条件 | 等级 | 处理动作 |
|---|---|---|
| 账务错误、双流拼接、大面积协议失败 | P0 | 立即回滚，冻结发布，24h 内复盘 |
| 上游凭证泄露、需求方绕过平台直连供应方、平台凭证入站覆盖不足 | P0 | 立即回滚，冻结发布，启动安全应急并完成法务告警 |
| 供应侧结算状态跳态、提现资金对不平、按钮权限越权 | P0 | 冻结供应侧发布链路，执行资金核对与权限审计 |
| 兼容回归影响部分租户 | P1 | 暂停升波，48h 内修复并补测 |
| 非关键指标偏差 | P2 | 记录到 backlog，下迭代修复 |

## 7. 证据包目录规范（建议）

```text
立交桥/
  evidence/
    2026-03-31-risk-control/
      gate-reports/
      supply-gate/
      security-scans/
      rollback-drill/
      dashboards/
      review/
```

所有任务验收必须至少提供：

1. 原始执行日志。
2. 指标/截图证据。
3. 结论与责任人签字（电子审批记录）。

## 8. 启动会议议程（30 分钟模板）

1. 确认实名 RACI 与 on-call（按第2章映射执行）。
2. 确认 P0 红线（含凭证边界）及回滚授权链路。
3. 确认 M1-M5 日期不变。
4. 锁定每日站会与每周评审时间。
