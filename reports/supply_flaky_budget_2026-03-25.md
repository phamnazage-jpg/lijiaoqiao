# 供应侧测试 Flaky 预算与治理规则

- 版本：v1.0
- 日期：2026-03-25
- 适用范围：`UI-SUP-*`、`SEC-SUP-*`、`UI-DESIGN-QA-*`

---

## 1. 预算定义

1. 单用例 7 日 Flaky 率阈值：`<=2%`。
2. 测试套件整体 Flaky 率阈值：`<=1%`。
3. P0 用例允许 Flaky：`0`（任何抖动按失败处理）。

计算方式：
`flaky_rate = (retry_pass_count / total_executions) * 100%`

---

## 2. 处置规则

1. Flaky 率 `>2%` 且 `<5%`：进入治理 Backlog，48 小时内修复。
2. Flaky 率 `>=5%`：标记阻断项，禁止作为发布通过证据。
3. P0 用例出现 flaky：立即冻结发布，按 P0 事件处理。

---

## 3. 治理动作

1. 定位是否为环境抖动、数据污染、断言不稳定、异步等待不足。
2. 增加固定数据集与可重复前置，禁止依赖随机时序。
3. 对接口型用例增加 request_id 对账，避免误判。
4. 每次修复需附“根因 + 修复点 + 复测结果”。

---

## 4. 报告字段（每周）

1. `suite_name`
2. `case_id`
3. `execution_count`
4. `retry_pass_count`
5. `flaky_rate_pct`
6. `owner`
7. `status`（open/fixing/closed）
8. `evidence_link`
