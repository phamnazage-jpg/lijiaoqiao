# 规划设计闭环执行任务清单（Superpowers 版）

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 关闭当前规划设计文档中的 P0/P1 缺口，完成从“设计对齐”到“执行可发布”的全链路闭环。  
**Architecture:** 以 SSOT 为唯一约束源，先锁定需求与契约，再打通执行环境与证据链，最后完成门禁签署与发布决策。  
**Tech Stack:** Markdown/CSV/OpenAPI YAML/Bash 脚本（`scripts/supply-gate`）/CI Gate。  

---

## 0. 需求与功能理解确认（执行前）

1. 业务模型：`用户A供给 -> 平台 -> 用户B购买平台服务`，上游凭证只托管在平台。
2. 核心功能：账号挂载、套餐发布、收益结算、凭证边界审计与门禁。
3. 最高优先级：M-013~M-016 达标 + SUP-004~SUP-007 真实证据。

---

## 1. 总体任务分组

1. `WG-A` 需求与功能冻结（P0）
2. `WG-B` 接口契约与技术规范对齐（P0）
3. `WG-C` 测试追踪矩阵与门禁一致化（P1）
4. `WG-D` 执行环境解锁与联调证据（P0）
5. `WG-E` 报告签署与发布决策（P1）
6. `WG-F` 全局功能映射补齐与一致性收尾（P1/P2）

---

## 2. 最细子任务清单（原子步骤）

| Step ID | Workstream | 原子动作（2-5分钟） | 输入 | 输出 | Owner | DoD |
|---|---|---|---|---|---|---|
| A-001 | WG-A | 打开按钮级 PRD 并定位“草案”标记 | `docs/supply_button_level_prd_v1_2026-03-25.md` | 标记行号记录 | 产品 | 已记录证据 |
| A-002 | WG-A | 打开按钮级 PRD 并定位“待拍板项” | 同上 | 待决议项清单 | 产品 | 4条待决议被提取 |
| A-003 | WG-A | 建立“待拍板->决议”映射表 | A-002 | 决议表草案 | 产品+ARCH | 每条有决议动作 |
| A-004 | WG-A | 召开 30 分钟决议会确认 4 条待拍板项 | A-003 | 会议纪要 | 产品+ARCH+FIN+QA | 4条全定稿 |
| A-005 | WG-A | 将按钮级 PRD 状态改为“冻结” | A-004 | 文档状态更新 | 产品 | 文档不再含“草案” |
| A-006 | WG-A | 删除/替换“待拍板项”章节为“已决议项” | A-004 | 已决议章节 | 产品 | 无未决事项残留 |
| A-007 | WG-A | 在任务单中引用最新冻结 PRD | A-005 | 引用链更新 | PMO | 任务单链接可追踪 |
| A-008 | WG-A | 在复核报告中更新 P0-01 关闭状态 | A-005/A-006 | 复核报告修订 | ARCH | P0-01 标注 Closed |
| B-001 | WG-B | 在 OpenAPI 中新增 `X-Request-Id` 参数定义 | `docs/supply_api_contract_openapi_draft_v1_2026-03-25.yaml` | 参数组件 | PLAT | 参数 schema 完整 |
| B-002 | WG-B | 在 OpenAPI 中新增 `Idempotency-Key` 参数定义 | 同上 | 参数组件 | PLAT | 参数 schema 完整 |
| B-003 | WG-B | 将两类 header 挂到 `POST /supply/accounts` | B-001/B-002 | 路径参数更新 | PLAT | required=true |
| B-004 | WG-B | 将两类 header 挂到 `POST /packages/{id}/publish` | B-001/B-002 | 路径参数更新 | PLAT | required=true |
| B-005 | WG-B | 将两类 header 挂到 `POST /packages/batch-price` | B-001/B-002 | 路径参数更新 | PLAT | required=true |
| B-006 | WG-B | 将两类 header 挂到 `POST /settlements/withdraw` | B-001/B-002 | 路径参数更新 | PLAT | required=true |
| B-007 | WG-B | 将两类 header 挂到 `POST /settlements/{id}/cancel` | B-001/B-002 | 路径参数更新 | PLAT | required=true |
| B-008 | WG-B | 为幂等冲突补充 `409` 错误码示例 | B-003~B-007 | 错误响应示例 | PLAT | 含 payload mismatch 场景 |
| B-009 | WG-B | 为处理中重放补充 `202` 示例 | B-003~B-007 | 错误响应示例 | PLAT | 含 retry_after_ms |
| B-010 | WG-B | 运行 OpenAPI lint 校验 | B-001~B-009 | lint 结果 | PLAT | 无语法错误 |
| B-011 | WG-B | 更新技术增强稿中“契约已落地”状态 | B-010 | 文档状态更新 | ARCH | XR-001 对齐 |
| B-012 | WG-B | 在复核报告中更新 P0-02 关闭状态 | B-010 | 复核报告修订 | ARCH | P0-02 Closed |
| C-001 | WG-C | 打开测试增强文档，提取短路径条目 | `docs/supply_test_plan_enhanced_v1_2026-03-25.md` | 待改路径清单 | QA | 清单完整 |
| C-002 | WG-C | 将 `/accounts` 替换为完整 `/api/v1/supply/accounts` | C-001 | 路径修正 | QA | 与 OpenAPI 一致 |
| C-003 | WG-C | 将 `/packages` 类路径替换为完整路径 | C-001 | 路径修正 | QA | 与 OpenAPI 一致 |
| C-004 | WG-C | 将 `/settlements` 类路径替换为完整路径 | C-001 | 路径修正 | QA | 与 OpenAPI 一致 |
| C-005 | WG-C | 在矩阵中新增 `api_alias` 列（如需兼容） | C-002~C-004 | 矩阵字段扩展 | QA | 自动匹配无歧义 |
| C-006 | WG-C | 同步更新 `reports/supply_traceability_matrix_2026-03-25.csv` | C-002~C-005 | CSV 更新 | QA | 行列一致 |
| C-007 | WG-C | 补充追踪矩阵生成规则文档 | C-006 | 生成说明 | QA | 新人可复跑 |
| C-008 | WG-C | 在 XR-002 验收项加入“路径一致性检查” | C-006 | 任务单更新 | ARCH+QA | 验收标准明确 |
| D-001 | WG-D | 与平台确认可达 staging 域名 | Preflight 报告 | 可达域名 | PLAT/SRE | 域名可解析 |
| D-002 | WG-D | 在 `.env` 更新 `API_BASE_URL` | `scripts/supply-gate/.env` | 新地址配置 | PLAT | 配置已保存 |
| D-003 | WG-D | 向平台申请短期 owner token | 安全流程 | token 值 | SEC+PLAT | 获取成功 |
| D-004 | WG-D | 向平台申请短期 viewer token | 安全流程 | token 值 | SEC+PLAT | 获取成功 |
| D-005 | WG-D | 向平台申请短期 admin token | 安全流程 | token 值 | SEC+PLAT | 获取成功 |
| D-006 | WG-D | 写入 `.env` 三类 token | D-003~D-005 | 完整 `.env` | PLAT | 无占位值 |
| D-007 | WG-D | 执行 `sup004_accounts.sh` | D-006 | `tests/supply/artifacts/sup004/*` | QA | 产物齐全 |
| D-008 | WG-D | 校验 `sup004` 关键字段（account_id/status） | D-007 | 校验记录 | QA | 断言通过 |
| D-009 | WG-D | 执行 `sup005_packages.sh` | D-006 | `tests/supply/artifacts/sup005/*` | QA | 产物齐全 |
| D-010 | WG-D | 校验 `sup005` 批量调价明细 | D-009 | 校验记录 | QA | success+failed=total |
| D-011 | WG-D | 执行 `sup006_settlements.sh` | D-006 | `tests/supply/artifacts/sup006/*` | QA+FIN | 产物齐全 |
| D-012 | WG-D | 校验提现状态机与金额回退 | D-011 | 校验记录 | FIN+QA | 无跳态/无双扣 |
| D-013 | WG-D | 执行 `sup007_boundary.sh` | D-006 | `tests/supply/artifacts/sup007/*` | SEC+QA | 产物齐全 |
| D-014 | WG-D | 校验 M-013 脱敏扫描结果 | D-013 | 指标断言 | SEC | 必须为 0 |
| D-015 | WG-D | 校验 M-014 入站覆盖率 | D-013 | 指标断言 | SEC+PLAT | 必须 100% |
| D-016 | WG-D | 校验 M-015 直连绕过事件 | D-013 | 指标断言 | SEC+SRE | 必须 0 |
| D-017 | WG-D | 校验 M-016 query key 拒绝率 | D-013 | 指标断言 | SEC+PLAT | 必须 100% |
| D-018 | WG-D | 生成新 preflight 报告（PASS） | D-007~D-017 | preflight 更新 | QA | 状态 PASS |
| E-001 | WG-E | 回填 ACC 报告每条用例 PASS/FAIL | D-007/D-008 | ACC 报告 | QA | 6条有结论 |
| E-002 | WG-E | 回填 PKG 报告每条用例 PASS/FAIL | D-009/D-010 | PKG 报告 | QA | 6条有结论 |
| E-003 | WG-E | 回填 SET 报告每条用例 PASS/FAIL | D-011/D-012 | SET 报告 | QA+FIN | 5条有结论 |
| E-004 | WG-E | 回填 SEC 报告与 M-013~M-016 实测值 | D-013~D-017 | SEC 报告 | SEC+QA | 指标完整 |
| E-005 | WG-E | 更新 SUP 汇总报告分项结论 | E-001~E-004 | 汇总表更新 | ARCH+QA | 4项闭环 |
| E-006 | WG-E | 勾选汇总结论（通过/有条件通过/不通过） | E-005 | 结论勾选 | ARCH | 单选且合理 |
| E-007 | WG-E | 回填实名 Owner（非“待实名”） | E-005 | Owner 完整 | PMO | 全实名 |
| E-008 | WG-E | 回填 4 方签署人 + 日期 +签署编号 | E-007 | 签署页完整 | PMO | 可审计 |
| E-009 | WG-E | 更新复核报告未关闭风险章节 | E-006~E-008 | 复核报告更新 | ARCH | 状态同步 |
| E-010 | WG-E | 在任务单标记 SUP-004~SUP-008 状态 | E-006 | 任务单更新 | PMO | 状态一致 |
| F-001 | WG-F | 创建“全局 P0 -> 供应侧/平台侧”映射表 | `llm_gateway_prd_v1_2026-03-25.md` | 新映射文档 | 产品 | 覆盖 PRD 全量 P0 |
| F-002 | WG-F | 将“预算/告警/账单导出”映射到按钮或页面入口 | F-001 | 映射补充 | 产品+UIUX | 无遗漏 |
| F-003 | WG-F | 将映射表接入测试追踪矩阵 | F-001/F-002 | 矩阵扩展 | QA | 需求可测 |
| F-004 | WG-F | 定义 `/supply` vs `/supplier` 命名策略 | OpenAPI | 命名决议 | ARCH+PLAT | 有兼容方案 |
| F-005 | WG-F | 若保留双路径，补充 alias/重定向规则 | F-004 | 契约补充 | PLAT | 客户端可迁移 |
| F-006 | WG-F | 更新 API 规范注释与变更日志 | F-004/F-005 | 变更记录 | PLAT | 变更可追溯 |
| F-007 | WG-F | 在复核报告追加 P1/P2 收敛状态 | F-001~F-006 | 复核更新 | ARCH | 收敛结论明确 |
| G-001 | 全局 | 执行一次跨文档链接完整性检查 | 全部更新文档 | 检查记录 | PMO | 无失效引用 |
| G-002 | 全局 | 执行一次门禁指标与报告一致性检查 | 指标报告 | 对齐记录 | QA+SEC | 指标一致 |
| G-003 | 全局 | 输出最终“GO/CONDITIONAL GO/NO-GO”决议稿 | E/F/G 完成项 | 决议文档 | ARCH | 管理层可签发 |

---

## 3. 里程碑与退出标准

1. 里程碑 M-A（需求冻结完成）：A-001~A-008 全完成。
2. 里程碑 M-B（契约对齐完成）：B-001~B-012、C-001~C-008 全完成。
3. 里程碑 M-C（执行证据完成）：D-001~D-018、E-001~E-010 全完成。
4. 里程碑 M-D（一致性收尾）：F-001~F-007、G-001~G-003 全完成。

退出标准（可判定 GO）：
1. 所有 P0 项已关闭。
2. `SUP-004~SUP-007` 真实执行证据完整。
3. M-013~M-016 全达标且有签署。
4. 汇总评审从“不通过/阻塞”转为“通过/有条件通过（带关闭计划）”。
