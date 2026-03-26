# 供应侧 UI-SUP 可执行测试清单（v1.0）

- 版本：v1.0
- 日期：2026-03-25
- 适用范围：`SUP-PAGE-001/002/003`（账号挂载、套餐发布、收益结算）
- 关联文档：
  - `supply_button_level_prd_v1_2026-03-25.md`
  - `supply_api_contract_openapi_draft_v1_2026-03-25.yaml`
  - `router_core_s2_acceptance_test_cases_v1_2026-03-17.md`
  - `acceptance_gate_single_source_v1_2026-03-18.md`

---

## 1. 测试环境与公共前置

## 1.1 环境

1. 环境：`staging`（含审计日志、告警、导出能力）。
2. 鉴权：仅允许平台凭证（header），禁止 query key 入站。
3. 时区：`Asia/Shanghai`。

## 1.2 测试账号

1. `supplier_owner_01`：供应方主账号（可创建/编辑）。
2. `supplier_viewer_01`：只读账号（仅查看）。
3. `platform_admin_01`：平台管理员（可处理风控态）。

## 1.3 测试数据

1. 供应账号初始状态：`pending`、`active`、`suspended` 各 1 条。
2. 套餐初始状态：`draft`、`active`、`paused`、`sold_out`、`expired` 各 1 条。
3. 结算单初始状态：`pending`、`processing`、`completed` 各 1 条。

## 1.4 通用断言

1. 每次关键按钮点击均产生审计事件。
2. 错误体不出现可复用上游凭证片段（映射 M-013）。
3. 所有请求 `request_id` 可追踪。

---

## 2. SUP-PAGE-001 账号挂载（UI-SUP-ACC-001~006）

## UI-SUP-ACC-001 立即验证成功路径

- 前置数据：
1. `supplier_owner_01` 已登录。
2. 页面字段 `provider/account_type/credential_input` 均可编辑。

- 步骤：
1. 选择 `provider=openai`。
2. 选择 `account_type=api_key`。
3. 输入合法凭证（测试密钥）。
4. 点击 `BTN-ACC-001 立即验证`。

- 断言：
1. 发起 `POST /api/v1/supply/accounts/verify`。
2. 返回 `verify_status=pass`。
3. 页面显示“验证通过 + 可用额度摘要”。
4. 记录审计事件 `supply.account.verify`。

## UI-SUP-ACC-002 提交挂载成功路径

- 前置数据：
1. 已完成 `UI-SUP-ACC-001` 且返回通过。
2. 勾选 `risk_ack`。

- 步骤：
1. 输入 `account_alias`。
2. 点击 `BTN-ACC-002 提交挂载`。

- 断言：
1. 发起 `POST /api/v1/supply/accounts`。
2. 返回 `201`，状态为 `pending` 或 `active`。
3. 列表出现新账号记录。
4. 记录审计事件 `supply.account.create`。

## UI-SUP-ACC-003 激活账号

- 前置数据：
1. 存在状态为 `pending` 的账号 A。
2. 当前用户有编辑权限。

- 步骤：
1. 在账号 A 行点击 `BTN-ACC-003 激活账号`。

- 断言：
1. 发起 `POST /api/v1/supply/accounts/{id}/activate`。
2. 状态从 `pending` 变为 `active`。
3. 页面即时刷新该行状态。
4. 记录审计事件 `supply.account.activate`。

## UI-SUP-ACC-004 暂停账号

- 前置数据：
1. 存在状态为 `active` 的账号 B。
2. 账号 B 无未结清风险单。

- 步骤：
1. 在账号 B 行点击 `BTN-ACC-004 暂停账号`。

- 断言：
1. 发起 `POST /api/v1/supply/accounts/{id}/suspend`。
2. 状态从 `active` 变为 `suspended`。
3. 记录审计事件 `supply.account.suspend`。

## UI-SUP-ACC-005 删除账号失败保护

- 前置数据：
1. 存在状态 `active` 的账号 C。

- 步骤：
1. 尝试触发 `BTN-ACC-005 删除账号`（若不可见则通过接口模拟）。

- 断言：
1. UI 层按钮不可见或不可点。
2. 若后端请求被触发，返回 `409` 冲突。
3. 页面提示“活跃账号不可删除”。
4. 不产生删除成功审计事件。

## UI-SUP-ACC-006 查看审计日志

- 前置数据：
1. 账号 D 已有至少 3 条历史操作记录。

- 步骤：
1. 点击 `BTN-ACC-006 查看审计`。

- 断言：
1. 发起 `GET /api/v1/supply/accounts/{id}/audit-logs`。
2. 侧边栏展示审计列表（含 operator/request_id/time）。
3. 列表分页正常（page/page_size 生效）。

---

## 3. SUP-PAGE-002 套餐发布（UI-SUP-PKG-001~006）

## UI-SUP-PKG-001 保存草稿

- 前置数据：
1. 至少存在 1 个 `active` 账号。

- 步骤：
1. 选择 `supply_account_id`、`model`。
2. 输入 `total_quota`、`price_per_1m_input`、`price_per_1m_output`、`valid_days`。
3. 点击 `BTN-PKG-001 保存草稿`。

- 断言：
1. 发起 `POST /api/v1/supply/packages/draft`。
2. 返回 `201`，状态为 `draft`。
3. 列表可查询到该草稿。
4. 记录审计事件 `supply.package.draft.save`。

## UI-SUP-PKG-002 发布上架成功

- 前置数据：
1. 存在 `draft` 套餐 E，且价格高于最低保护价。

- 步骤：
1. 点击 `BTN-PKG-002 发布上架`。

- 断言：
1. 发起 `POST /api/v1/supply/packages/{id}/publish`。
2. 状态由 `draft` 变更为 `active`。
3. 记录审计事件 `supply.package.publish`。

## UI-SUP-PKG-003 暂停售卖

- 前置数据：
1. 存在 `active` 套餐 F。

- 步骤：
1. 点击 `BTN-PKG-003 暂停售卖`。

- 断言：
1. 发起 `POST /api/v1/supply/packages/{id}/pause`。
2. 状态变更为 `paused`。
3. 记录审计事件 `supply.package.pause`。

## UI-SUP-PKG-004 下架套餐

- 前置数据：
1. 存在 `active` 或 `paused` 套餐 G。
2. 套餐 G 不存在未完成结算锁。

- 步骤：
1. 点击 `BTN-PKG-004 立即下架`。

- 断言：
1. 发起 `POST /api/v1/supply/packages/{id}/unlist`。
2. 状态变更为 `expired`（或按策略回到 `paused`）。
3. 记录审计事件 `supply.package.unlist`。

## UI-SUP-PKG-005 批量调价部分失败

- 前置数据：
1. 选择 3 个套餐：2 个可编辑，1 个不可编辑（状态冲突）。

- 步骤：
1. 触发 `BTN-PKG-005 批量调价`，提交统一调价参数。

- 断言：
1. 发起 `POST /api/v1/supply/packages/batch-price`。
2. 返回 `total=3`，`success_count=2`，`failed_count=1`。
3. 失败项含明确 `package_id` 与 `error_code`。
4. 成功项价格实际更新。

## UI-SUP-PKG-006 复制套餐

- 前置数据：
1. 存在任意套餐 H。

- 步骤：
1. 点击 `BTN-PKG-006 复制套餐`。

- 断言：
1. 发起 `POST /api/v1/supply/packages/{id}/clone`。
2. 返回 `201`，新套餐状态为 `draft`。
3. 新套餐字段默认值与原套餐一致（除状态、创建时间）。
4. 记录审计事件 `supply.package.clone`。

---

## 4. SUP-PAGE-003 结算提现（UI-SUP-SET-001~005）

## UI-SUP-SET-001 刷新收益数据

- 前置数据：
1. 供应方账号存在账单数据。

- 步骤：
1. 点击 `BTN-SET-001 刷新收益`。

- 断言：
1. 发起 `GET /api/v1/supplier/billing`。
2. 汇总卡片与趋势图刷新。
3. 刷新失败时显示可重试提示，不清空旧数据。

## UI-SUP-SET-002 发起提现成功

- 前置数据：
1. `available_amount > 0`。
2. 当前无 `processing` 结算单。

- 步骤：
1. 输入提现金额、收款方式、收款账户、验证码。
2. 点击 `BTN-SET-002 发起提现`。

- 断言：
1. 发起 `POST /api/v1/supply/settlements/withdraw`。
2. 返回 `201`，结算状态为 `pending`。
3. 提现金额从可提现余额冻结。
4. 记录审计事件 `supply.settlement.withdraw.create`。

## UI-SUP-SET-003 撤销提现申请

- 前置数据：
1. 存在 `pending` 状态结算单 I。

- 步骤：
1. 点击 `BTN-SET-003 撤销申请`。

- 断言：
1. 发起 `POST /api/v1/supply/settlements/{id}/cancel`。
2. 状态变更为 `failed/cancelled`。
3. 冻结金额回退。
4. 记录审计事件 `supply.settlement.withdraw.cancel`。

## UI-SUP-SET-004 下载对账单

- 前置数据：
1. 存在任意结算单 J。

- 步骤：
1. 点击 `BTN-SET-004 下载对账单`。

- 断言：
1. 发起 `GET /api/v1/supply/settlements/{id}/statement`。
2. 返回可下载链接与过期时间。
3. 下载文件成功，文件命名符合规范。
4. 记录审计事件 `supply.settlement.statement.export`。

## UI-SUP-SET-005 查看收益流水

- 前置数据：
1. 已生成多条收益流水记录。

- 步骤：
1. 点击 `BTN-SET-005 查看流水明细`。
2. 分别使用时间区间和分页参数查询。

- 断言：
1. 发起 `GET /api/v1/supply/earnings/records`。
2. 明细返回 `earnings_type/status/amount/earned_at` 字段完整。
3. 分页逻辑正确，无重复/漏项。

---

## 5. 安全专项（SEC-SUP-001~002）

## SEC-SUP-001 错误体与导出脱敏检查

- 前置数据：
1. 人为构造账号验证失败、发布失败、提现失败场景。
2. 准备账单导出与对账单导出样本。

- 步骤：
1. 触发上述失败场景并抓取错误响应。
2. 执行对账单下载。
3. 对错误体、导出文件运行脱敏扫描。

- 断言：
1. 任一结果中均不出现可复用上游凭证片段。
2. 命中敏感片段则标记 P0，对应 M-013 失败。

## SEC-SUP-002 凭证边界回归（对齐 CB-001~CB-004）

- 前置数据：
1. 平台鉴权与拦截策略已开启。
2. 出网审计与告警已开启。

- 步骤：
1. 使用平台凭证访问主路径，确认可通过。
2. 构造外部 query key 请求（含 `/v1beta/*`）。
3. 构造需求方绕过平台直连上游尝试。

- 断言：
1. `platform_credential_ingress_coverage_pct=100%`（M-014）。
2. `query_key_external_reject_rate_pct=100%`（M-016）。
3. `direct_supplier_call_by_consumer_events=0`（M-015）。
4. `supplier_credential_exposure_events=0`（M-013）。

---

## 6. 执行产物要求

每条用例至少产出：

1. 原始请求/响应日志（含 request_id）。
2. 页面录屏或关键截图。
3. 审计事件截图或导出。
4. 用例结论（PASS/FAIL/BLOCKED）与责任人签字。
