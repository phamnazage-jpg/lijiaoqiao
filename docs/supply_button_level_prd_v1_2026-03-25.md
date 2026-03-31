# 供应侧按钮级 PRD 分解（首批 3 个核心页面）

- 版本：v1.1（冻结）
- 日期：2026-03-27
- 适用范围：供应侧 S0/S1 首批上线页面
- 关联 SSOT：
  - `llm_gateway_subapi_evolution_plan_v4_2_2026-03-24.md`
  - `acceptance_gate_single_source_v1_2026-03-18.md`
  - `supply_side_product_design_v1_2026-03-18.md`
  - `supply_detailed_design_v1_2026-03-18.md`

---

## 1. 目标与范围

本稿用于把供应侧功能从“流程级”下钻到“按钮级”，用于：

1. 前端实现不歧义。
2. 后端接口可映射。
3. QA 可直接编写用例。
4. 审计与埋点可落地。

首批覆盖页面：

1. `SUP-PAGE-001` 供应账号挂载页。
2. `SUP-PAGE-002` 套餐发布与上下架页。
3. `SUP-PAGE-003` 收益结算与提现页。

---

## 2. 全局硬约束（所有页面生效）

1. 用户A（供应方）仅向平台提交上游凭证；用户B不可见、不可得。
2. 页面、接口、导出、错误文案均不得返回可复用上游凭证片段。
3. 凭证相关动作必须有审计事件，且支持按 `request_id/operator_id` 追踪。
4. 任何违反凭证边界的行为按 P0 处理，阻断发布（M-013~M-016）。

---

## 3. 页面一：供应账号挂载（SUP-PAGE-001）

## 3.1 页面目标

供应方完成账号挂载、验证、激活/停用，确保只在平台托管上游凭证。

## 3.2 字段规格

| 字段ID | 字段名 | 类型 | 必填 | 校验规则 | 脱敏规则 |
|---|---|---|---|---|---|
| F-001 | `provider` | 下拉 | 是 | 枚举：openai/anthropic/gemini/baidu/xfyun/tencent | 不脱敏 |
| F-002 | `account_type` | 单选 | 是 | 枚举：api_key/oauth | 不脱敏 |
| F-003 | `credential_input` | 密文输入框 | 是 | 长度、前缀、字符集校验 | UI 全程掩码，后端不回显 |
| F-004 | `account_alias` | 文本 | 否 | 1-100 字符，禁止敏感词 | 不脱敏 |
| F-005 | `min_quota_threshold` | 数值 | 否 | >=0，最多 6 位小数 | 不脱敏 |
| F-006 | `risk_ack` | 勾选框 | 是 | 必须勾选协议确认 | 不脱敏 |

## 3.3 按钮级规格

| 按钮ID | 按钮文案 | 可见条件 | 可点击条件 | 触发动作 | 成功态 | 失败态 | 审计事件 | 测试用例ID |
|---|---|---|---|---|---|---|---|---|
| BTN-ACC-001 | 立即验证 | 已选择 `provider` 且输入凭证 | F-001/F-002/F-003 合法，未处于提交中 | `POST /api/v1/supply/accounts/verify` | 展示验证通过与额度摘要 | 展示错误码与修复建议 | `supply.account.verify` | `UI-SUP-ACC-001` |
| BTN-ACC-002 | 提交挂载 | 验证通过后显示 | 风险确认已勾选 | `POST /api/v1/supply/accounts` | 新建记录 `pending/active` | 留在当前页，字段高亮报错 | `supply.account.create` | `UI-SUP-ACC-002` |
| BTN-ACC-003 | 激活账号 | 账号状态为 `pending/suspended` | 当前用户拥有供应方权限 | `POST /api/v1/supply/accounts/{id}/activate` | 状态变更 `active` | 状态不变并提示原因 | `supply.account.activate` | `UI-SUP-ACC-003` |
| BTN-ACC-004 | 暂停账号 | 账号状态为 `active` | 账号无未结清风险单 | `POST /api/v1/supply/accounts/{id}/suspend` | 状态变更 `suspended` | 状态不变并提示依赖项 | `supply.account.suspend` | `UI-SUP-ACC-004` |
| BTN-ACC-005 | 删除账号 | 账号状态非 `active` | 无关联在售套餐 | `DELETE /api/v1/supply/accounts/{id}` | 列表移除 | 删除失败并提示阻塞原因 | `supply.account.delete` | `UI-SUP-ACC-005` |
| BTN-ACC-006 | 查看审计 | 用户有审计权限 | 总是可点 | `GET /api/v1/supply/accounts/{id}/audit-logs` | 打开侧边栏 | 提示“暂无审计数据”或加载失败 | `supply.account.audit.view` | `UI-SUP-ACC-006` |

## 3.4 状态机

`pending -> active -> suspended -> disabled`

约束：

1. `active` 状态不可直接删除。
2. `disabled` 为平台风控态，仅管理员可恢复。

## 3.5 错误码映射（页面级）

| 错误码 | 文案 | 前端动作 |
|---|---|---|
| `SUP_ACC_4001` | 凭证格式非法 | 高亮 `credential_input` |
| `SUP_ACC_4002` | 上游连通性校验失败 | 展示“重试验证”入口 |
| `SUP_ACC_4003` | ToS 不允许该账号进入供给池 | 阻断提交并展示合规说明 |
| `SUP_ACC_5001` | 系统繁忙，请稍后再试 | 保留输入并允许重试 |

---

## 4. 页面二：套餐发布与上下架（SUP-PAGE-002）

## 4.1 页面目标

供应方发布可售套餐，执行上架/下架/暂停/恢复，受价格与风控规则约束。

## 4.2 字段规格

| 字段ID | 字段名 | 类型 | 必填 | 校验规则 | 备注 |
|---|---|---|---|---|---|
| F-101 | `supply_account_id` | 下拉 | 是 | 必须为 `active` 账号 | 来源 SUP-PAGE-001 |
| F-102 | `model` | 下拉 | 是 | 模型白名单内 | 按供应商过滤 |
| F-103 | `total_quota` | 数值 | 是 | >0 且 <= 账户可用额度 | 单位与主账务一致 |
| F-104 | `price_per_1m_input` | 数值 | 是 | >= 平台最低保护价 | 6 位小数 |
| F-105 | `price_per_1m_output` | 数值 | 是 | >= 平台最低保护价 | 6 位小数 |
| F-106 | `valid_days` | 数值 | 是 | 1-365 | 过期自动下架 |
| F-107 | `max_concurrent` | 数值 | 否 | 1-1000 | 默认 10 |
| F-108 | `rate_limit_rpm` | 数值 | 否 | 1-100000 | 默认 60 |

## 4.3 按钮级规格

| 按钮ID | 按钮文案 | 可见条件 | 可点击条件 | 触发动作 | 成功态 | 失败态 | 审计事件 | 测试用例ID |
|---|---|---|---|---|---|---|---|---|
| BTN-PKG-001 | 保存草稿 | 已进入编辑页 | 基础字段合法 | `POST /api/v1/supply/packages/draft` | 状态 `draft` 持久化 | 保留页面并提示字段错误 | `supply.package.draft.save` | `UI-SUP-PKG-001` |
| BTN-PKG-002 | 发布上架 | 套餐为 `draft/paused` | 价格、额度、有效期全部通过 | `POST /api/v1/supply/packages/{id}/publish` | 状态变更 `active` | 阻断并展示具体不满足项 | `supply.package.publish` | `UI-SUP-PKG-002` |
| BTN-PKG-003 | 暂停售卖 | 套餐为 `active` | 无平台强制冻结 | `POST /api/v1/supply/packages/{id}/pause` | 状态变更 `paused` | 状态不变并提示原因 | `supply.package.pause` | `UI-SUP-PKG-003` |
| BTN-PKG-004 | 立即下架 | 套餐为 `active/paused` | 无未完成的结算锁 | `POST /api/v1/supply/packages/{id}/unlist` | 状态变更 `expired` 或 `paused` | 状态不变并提示阻塞订单 | `supply.package.unlist` | `UI-SUP-PKG-004` |
| BTN-PKG-005 | 批量调价 | 列表页多选后显示 | 所选套餐均可编辑 | `POST /api/v1/supply/packages/batch-price` | 批量更新成功数回显 | 部分失败返回明细 | `supply.package.price.batch_update` | `UI-SUP-PKG-005` |
| BTN-PKG-006 | 复制套餐 | 任意套餐行可见 | 原套餐存在 | `POST /api/v1/supply/packages/{id}/clone` | 新增草稿套餐 | 复制失败提示 | `supply.package.clone` | `UI-SUP-PKG-006` |

## 4.4 状态机

`draft -> active -> paused -> sold_out -> expired`

约束：

1. `sold_out` 仅系统自动迁移，不可人工强制设置。
2. `expired` 后仅允许“复制套餐”，不允许直接恢复。

## 4.5 错误码映射（页面级）

| 错误码 | 文案 | 前端动作 |
|---|---|---|
| `SUP_PKG_4001` | 售价低于保护价 | 锁定发布按钮并高亮价格字段 |
| `SUP_PKG_4002` | 可用额度不足 | 高亮额度字段并提示可用值 |
| `SUP_PKG_4003` | 账号状态不可发布套餐 | 跳转账号页处理 |
| `SUP_PKG_4091` | 套餐状态冲突，请刷新后重试 | 强制刷新当前行 |

---

## 5. 页面三：收益结算与提现（SUP-PAGE-003）

## 5.1 页面目标

供应方查看收益、发起提现、追踪结算状态，形成可审计资金链路。

## 5.2 字段规格

| 字段ID | 字段名 | 类型 | 必填 | 校验规则 | 备注 |
|---|---|---|---|---|---|
| F-201 | `available_amount` | 只读金额 | 是 | >=0 | 后端计算 |
| F-202 | `withdraw_amount` | 数值输入 | 是 | >0 且 <= `available_amount` | 2 位小数 |
| F-203 | `payment_method` | 单选 | 是 | 枚举：bank/alipay/wechat | 与账户一致 |
| F-204 | `payment_account` | 文本 | 是 | 按通道校验账号格式 | 敏感信息掩码展示 |
| F-205 | `sms_code` | 验证码 | 是 | 时效 5 分钟 | 提交后失效 |

## 5.3 按钮级规格

| 按钮ID | 按钮文案 | 可见条件 | 可点击条件 | 触发动作 | 成功态 | 失败态 | 审计事件 | 测试用例ID |
|---|---|---|---|---|---|---|---|---|
| BTN-SET-001 | 刷新收益 | 页面可见 | 总是可点 | `GET /api/v1/supplier/billing` | 卡片与趋势图更新 | 提示“刷新失败，请稍后重试” | `supply.settlement.refresh` | `UI-SUP-SET-001` |
| BTN-SET-002 | 发起提现 | 有可提现金额 | 金额、账户、验证码合法 | `POST /api/v1/supply/settlements/withdraw` | 生成结算单 `pending` | 错误提示并保留输入 | `supply.settlement.withdraw.create` | `UI-SUP-SET-002` |
| BTN-SET-003 | 撤销申请 | 结算单状态为 `pending` | 申请属于本人 | `POST /api/v1/supply/settlements/{id}/cancel` | 状态变更 `failed/cancelled` | 提示不可撤销原因 | `supply.settlement.withdraw.cancel` | `UI-SUP-SET-003` |
| BTN-SET-004 | 下载对账单 | 有结算记录 | 总是可点 | `GET /api/v1/supply/settlements/{id}/statement` | 下载文件成功 | 弹窗提示失败原因 | `supply.settlement.statement.export` | `UI-SUP-SET-004` |
| BTN-SET-005 | 查看流水明细 | 页面可见 | 总是可点 | `GET /api/v1/supply/earnings/records` | 打开明细抽屉 | 提示无数据/加载失败 | `supply.earnings.records.view` | `UI-SUP-SET-005` |

## 5.4 状态机

`pending -> processing -> completed/failed`

约束：

1. `completed` 状态不可撤销。
2. `processing` 状态禁止重复提交提现申请。

## 5.5 错误码映射（页面级）

| 错误码 | 文案 | 前端动作 |
|---|---|---|
| `SUP_SET_4001` | 可提现余额不足 | 高亮金额并提示可提现额度 |
| `SUP_SET_4002` | 收款账户校验失败 | 高亮账户字段 |
| `SUP_SET_4003` | 验证码失效或错误 | 清空验证码并可重新获取 |
| `SUP_SET_4091` | 当前有处理中提现单 | 禁用“发起提现”按钮 |

---

## 6. 页面级埋点与审计最小集

## 6.1 埋点事件（分析）

1. `sup_page_view`：页面访问。
2. `sup_button_click`：按钮点击（含按钮 ID）。
3. `sup_submit_success`：关键提交成功。
4. `sup_submit_fail`：关键提交失败（含错误码）。

## 6.2 审计事件（合规）

1. 账号相关：创建、激活、暂停、删除、查看审计日志。
2. 套餐相关：发布、调价、暂停、下架、复制。
3. 结算相关：提现发起、提现撤销、对账单导出。

审计最小字段：

1. `event_id`
2. `operator_id`
3. `tenant_id`
4. `object_type`
5. `object_id`
6. `before_state`
7. `after_state`
8. `request_id`
9. `created_at`

---

## 7. 与凭证边界门禁映射

| 约束 | 页面控制点 | 对应 Gate/指标 |
|---|---|---|
| 供应方上游凭证不外发 | SUP-PAGE-001 的 `credential_input` 只入不出；所有页面禁止回显原文 | M-013 |
| 需求方仅平台凭证入站 | 页面与接口文案统一“平台凭证”，不提供上游凭证下载入口 | M-014 |
| 禁止需求方绕过平台直连上游 | 无任何“上游直连参数配置”入口；异常由安全策略阻断 | M-015 |
| 外部 query key 全拒绝 | 页面帮助文档和 SDK 示例仅给 header 鉴权方式 | M-016 |

---

## 8. 测试用例骨架（新增）

| 用例ID | 页面 | 关注点 | 期望 |
|---|---|---|---|
| UI-SUP-ACC-001~006 | SUP-PAGE-001 | 按钮可见性、禁用态、状态迁移、审计 | 与按钮规格一致 |
| UI-SUP-PKG-001~006 | SUP-PAGE-002 | 发布前校验、状态机、批量操作 | 不越权、不跳态 |
| UI-SUP-SET-001~005 | SUP-PAGE-003 | 提现流程、并发防重、撤销边界 | 资金状态一致 |
| SEC-SUP-001 | 全局 | 错误体/导出脱敏 | 不出现可复用上游凭证 |
| SEC-SUP-002 | 全局 | 凭证边界回归（与 CB-001~CB-004 对齐） | M-013~M-016 达标 |

---

## 9. 已决议项（2026-03-27）

决议依据：
1. `docs/product/supply_prd_pending_to_decision_map_v1_2026-03-27.md`
2. `review/outputs/supply_prd_decision_meeting_minutes_2026-03-27.md`

| 决议ID | 原待拍板项 | 决议结论 | 执行动作 |
|---|---|---|---|
| DEC-001 | `POST /api/v1/supply/*` 系列接口是否按本稿命名冻结 | 冻结 `/api/v1/supply/*` 为供应侧主路径；`/api/v1/supplier/billing` 作为兼容路径保留，待 F 阶段统一命名策略 | 在 OpenAPI 变更日志记录主路径与兼容路径策略 |
| DEC-002 | 提现金额风控阈值（单笔/单日）与冷却期 | S1 阶段阈值冻结：单笔 `<= 50,000 CNY`，单日累计 `<= 200,000 CNY`，同账户提现冷却期 `15 分钟` | 在结算风控与测试用例中同步阈值断言 |
| DEC-003 | 套餐“下架”与“暂停”的财务影响口径是否一致 | 不一致：`暂停`仅阻断新购，存量订单不变；`下架`阻断新购并触发 T+1 财务核对任务 | 在结算页与审计事件中区分 `pause/unlist` 财务语义 |
| DEC-004 | 供应方是否允许批量导入账号 | 不允许进入 S0/S1 主路径；改为 S2 评审项，仅可在受控灰度与白名单下试点 | 移出当前发布门禁范围，纳入后续路线图 |
