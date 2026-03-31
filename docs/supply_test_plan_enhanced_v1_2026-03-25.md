# 供应侧测试方案增强版（XR-002）

- 版本：v1.1
- 日期：2026-03-27
- 状态：生效（测试执行基线）
- 目标：形成“需求-接口-测试-指标-门禁”全链路闭环，补齐并发与重放风险覆盖
- 关联文档：
  - `supply_button_level_prd_v1_2026-03-25.md`
  - `supply_api_contract_openapi_draft_v1_2026-03-25.yaml`
  - `supply_ui_test_cases_executable_v1_2026-03-25.md`
  - `supply_technical_design_enhanced_v1_2026-03-25.md`
  - `acceptance_gate_single_source_v1_2026-03-18.md`

---

## 1. 测试目标与分层策略

## 1.1 目标

1. 保障供应侧三页链路（账号、套餐、结算）功能正确且不越权。
2. 保障凭证边界红线（M-013~M-016）持续达标。
3. 保障关键写路径在并发和重复提交下无双扣、无跳态、无脏数据。
4. 保障业主口径 SLA、申诉与赔付流程可验证、可追溯、可复盘。

## 1.2 分层覆盖

1. L1 单元测试：状态机迁移、不变量校验、幂等判定、金额计算。
2. L2 契约测试：OpenAPI 请求/响应字段、错误码、枚举与脱敏约束。
3. L3 集成测试：数据库事务一致性、唯一索引冲突、Outbox 事件。
4. L4 UI+API 端到端：按钮级流程、权限态、异常态、审计事件联查。
5. L5 安全专项：凭证泄露扫描、query key 拦截、绕平台直连探测。
6. L6 可靠性/性能专项：高并发冲突率、重试重放、降级与回滚演练。

---

## 2. 测试追踪矩阵（Requirement -> API -> Test -> Metric -> Gate）

| 需求ID | 需求描述 | API | api_alias | 测试用例 | 验收指标 | 门禁映射 |
|---|---|---|---|---|---|---|
| R-ACC-001 | 账号凭证验证成功可视化 | `POST /api/v1/supply/accounts/verify` | - | UI-SUP-ACC-001 | 验证成功率 >=99.5% | SUP-004 |
| R-ACC-002 | 挂载需风险确认与审计 | `POST /api/v1/supply/accounts` | - | UI-SUP-ACC-002 | 审计覆盖率=100% | SUP-004 |
| R-ACC-003 | 账号状态不跳态 | `POST /api/v1/supply/accounts/{accountId}/activate` / `POST /api/v1/supply/accounts/{accountId}/suspend` | `POST /api/v1/supply/accounts/{id}/activate` / `POST /api/v1/supply/accounts/{id}/suspend` | UI-SUP-ACC-003/004 + INT-ACC-STATE-001 | 冲突可解释率=100% | SUP-004 |
| R-ACC-004 | 活跃账号不可删除 | `DELETE /api/v1/supply/accounts/{accountId}` | `DELETE /api/v1/supply/accounts/{id}` | UI-SUP-ACC-005 | 违规删除成功率=0 | SUP-004 |
| R-PKG-001 | 草稿保存可追踪 | `POST /api/v1/supply/packages/draft` | - | UI-SUP-PKG-001 | 保存成功率>=99.5% | SUP-005 |
| R-PKG-002 | 套餐发布满足保护价与状态约束 | `POST /api/v1/supply/packages/{packageId}/publish` | `POST /api/v1/supply/packages/{id}/publish` | UI-SUP-PKG-002 + INT-PKG-INV-001 | 保护价违规放行率=0 | SUP-005 |
| R-PKG-003 | 批量调价部分失败可回执 | `POST /api/v1/supply/packages/batch-price` | - | UI-SUP-PKG-005 | 明细完备率=100% | SUP-005 |
| R-SET-001 | 提现发起防重复防双扣 | `POST /api/v1/supply/settlements/withdraw` | - | UI-SUP-SET-002 + CON-SET-001 | M-004/M-005 达标 | SUP-006 |
| R-SET-002 | 处理中/已完成不可撤销 | `POST /api/v1/supply/settlements/{settlementId}/cancel` | `POST /api/v1/supply/settlements/{id}/cancel` | UI-SUP-SET-003 + INT-SET-STATE-001 | 跳态成功率=0 | SUP-006 |
| R-SET-003 | 对账单导出不泄露敏感信息 | `GET /api/v1/supply/settlements/{settlementId}/statement` | `GET /api/v1/supply/settlements/{id}/statement` | UI-SUP-SET-004 + SEC-SUP-001 | M-013=0 | SUP-006/SUP-007 |
| R-SEC-001 | 仅平台凭证入站 | 全部北向 API | - | SEC-SUP-002 | M-014=100% | SUP-007 |
| R-SEC-002 | 外部 query key 全拒绝 | 全部北向 API | - | SEC-SUP-002 | M-016=100% | SUP-007 |
| R-SEC-003 | 需求方不可绕平台直连 | 出网策略与告警 | - | SEC-SUP-002 + SEC-DIRECT-001 | M-015=0 | SUP-007 |
| R-UX-001 | 按钮可见性和禁用规则正确 | 三页面全部按钮 | - | UI-DESIGN-QA-001~020 | 按钮规则通过率=100% | SUP-003/SUP-008 |

跨域映射补充：
1. 全局 P0 中预算/告警/组织级账单导出映射见：`docs/product/global_p0_to_supply_platform_mapping_v1_2026-03-27.md`。
2. 对应追踪项已并入：`reports/supply_traceability_matrix_2026-03-25.csv`（`R-PLAT-001~003`）。

---

## 3. 并发与幂等专项（P0）

## 3.1 CON-SET-001 提现并发双提防护

1. 前置：同一 `supplier_id` 可提现余额 1000，且无 processing 单。
2. 步骤：10 并发请求同一金额提现，使用不同 `request_id`，相同业务时窗。
3. 断言：
   1. 最多 1 笔进入 `processing`。
   2. 其余请求返回 `409/202`，无余额负值。
   3. 账务流水借贷平衡，`billing_conflict_rate_pct<=0.01%`。

## 3.2 CON-SET-002 提现幂等重放

1. 前置：准备固定 `Idempotency-Key`。
2. 步骤：同请求体重复发送 20 次；再发送一次异构请求体。
3. 断言：
   1. 同参重复返回同一 `settlement_id`，`idempotent_replay=true`。
   2. 异参返回 `409 IDEMPOTENCY_PAYLOAD_MISMATCH`。

## 3.3 CON-PKG-001 套餐发布冲突

1. 步骤：两个会话同时发布同一 `draft` 套餐。
2. 断言：
   1. 仅一个成功转为 `active`。
   2. 另一个返回 `409 SUP_PKG_4091`。
   3. 审计日志有冲突记录，且状态无跳变。

---

## 4. 安全与合规专项（P0）

## 4.1 凭证泄露扫描

1. 扫描对象：API 响应体、错误体、导出文件、审计日志、告警消息。
2. 扫描规则：上游 key/token 模式库 + 熵检测 + 前缀检测。
3. 通过标准：`supplier_credential_exposure_events=0`。

## 4.2 鉴权边界专项

1. 平台凭证成功链路：header bearer 访问主路径成功。
2. query key 拒绝链路：`/v1/*` 与 `/v1beta/*` 全拒绝。
3. 直连阻断链路：模拟需求方绕平台访问供应方，必须失败并告警。

## 4.3 申诉与赔付流程可测性

1. 场景：提现延迟、误扣款、导出异常、账户误冻结。
2. 断言：具备工单编号、SLA 计时、处理人、结果说明、赔付记录。
3. 指标：业主承诺时限命中率 >=99%，逾期需自动升级。

---

## 5. 性能与可靠性专项（P1）

| 用例ID | 场景 | 负载 | 目标 |
|---|---|---|---|
| PERF-ACC-001 | 账号验证峰值 | 100 RPS, 10min | P95 <= 800ms，错误率 <0.5% |
| PERF-PKG-001 | 套餐批量调价 | 2000 套餐/批 | 全量回执，超时率 <1% |
| PERF-SET-001 | 提现高峰并发 | 50 并发/供应方 | 无双扣，余额不为负 |
| REL-SET-001 | 结算服务实例重启 | 执行中重启一次 | 无状态跳变，幂等可恢复 |
| REL-SEC-001 | 网关规则热更新 | 更新 query key 拦截规则 | M-016 不下降 |

---

## 6. 测试数据治理

## 6.1 数据分层

1. 固定样本：用于回归（可重复、可比较）。
2. 脱敏样本：用于安全扫描与导出验证。
3. 回放样本：来自线上脱敏事件，验证真实边界场景。

## 6.2 数据规则

1. 测试数据必须通过脱敏策略，不得包含真实凭证。
2. 每次执行必须记录 `dataset_version`。
3. 幂等与并发用例必须复位余额和状态，防止脏数据串案。

---

## 7. CI 与门禁编排

## 7.1 执行顺序

1. `Contract Gate`：OpenAPI 漂移检测（阻断）。
2. `Core Integration Gate`：事务与不变量校验（阻断）。
3. `UI-SUP Gate`：按钮级 E2E（阻断）。
4. `SEC-SUP Gate`：凭证边界与泄露扫描（阻断）。
5. `PERF/REL Gate`：每晚定时跑，异常进入发布前强制复核。

## 7.3 分阶段质量门禁（防偏航）

1. G0 Requirement Gate：检查 PRD/OpenAPI/按钮清单版本一致，任一漂移阻断开发。
2. G1 Design Gate：检查 DDL、状态机、不变量、审计字段齐套，缺一阻断联调。
3. G2 Dev Gate：单测与契约测试达标后才允许合并。
4. G3 Integration Gate：DB/Redis/Outbox/权限链路通过后才允许提测。
5. G4 Release Gate：SUP-004~SUP-007 与安全门禁全绿才允许发布。
6. G5 Post Gate：发布后 24h 观察窗口出现 P0/P1 立即冻结后续升波。
7. 指标约束：`M-018=100%` 且 `M-019=100%`，否则回退到失败阶段整改。

## 7.2 失败策略

1. P0 用例失败：立即阻断发布 + 当日复盘。
2. P1 用例失败：冻结升波，48h 内修复并补测。
3. Flaky 管理：单用例 7 日 flaky 率 >2% 必须治理，禁止“无限重试掩盖失败”。

---

## 8. 准入与退出标准

## 8.1 准入（Entry）

1. PRD 按钮级规格冻结。
2. OpenAPI 字段冻结。
3. 技术增强稿（XR-001）已落地。
4. 路径一致性检查通过（API 字段与 OpenAPI 主路径一致，alias 映射完整）。

## 8.2 退出（Exit）

1. 追踪矩阵全部有执行结果与证据链接。
2. P0 用例通过率 100%，P1 用例通过率 >=98%。
3. M-013~M-016 全部达标，且无未关闭 P0 缺陷。
4. 业主验收条款（SLA/申诉/赔付）签字通过。

---

## 9. 交付物清单

1. `tests/supply/ui_sup_acc_report_2026-03-28.md`
2. `tests/supply/ui_sup_pkg_report_2026-03-29.md`
3. `tests/supply/ui_sup_set_report_2026-03-29.md`
4. `tests/supply/sec_sup_boundary_report_2026-03-30.md`
5. `reports/supply_gate_review_2026-03-31.md`
6. `reports/supply_traceability_matrix_2026-03-25.csv`（新增）
7. `reports/supply_flaky_budget_2026-03-25.md`（新增）

完成以上 7 项即视为 XR-002 关闭。
