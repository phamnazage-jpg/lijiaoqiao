# SUP Gate 命令级执行清单（SUP-004~SUP-007）

- 版本：v1.0
- 日期：2026-03-25
- 目标：为测试团队提供可直接执行的命令清单，输出可回填证据
- 关联任务：`SUP-004`、`SUP-005`、`SUP-006`、`SUP-007`
- 关联报告模板：
  - `tests/supply/ui_sup_acc_report_2026-03-28.md`
  - `tests/supply/ui_sup_pkg_report_2026-03-29.md`
  - `tests/supply/ui_sup_set_report_2026-03-29.md`
  - `tests/supply/sec_sup_boundary_report_2026-03-30.md`

---

## 1. 执行前准备

## 1.1 环境变量

在项目根目录创建并编辑 `scripts/supply-gate/.env`：

```bash
API_BASE_URL="https://staging.example.com"
OWNER_BEARER_TOKEN="replace-me-owner-token"
VIEWER_BEARER_TOKEN="replace-me-viewer-token"
ADMIN_BEARER_TOKEN="replace-me-admin-token"

# 测试数据（可按环境替换）
TEST_PROVIDER="openai"
TEST_MODEL="gpt-4o"
TEST_ACCOUNT_ALIAS="sup_acc_cmd"
TEST_CREDENTIAL_INPUT="sk-test-replace-me"
TEST_PAYMENT_METHOD="alipay"
TEST_PAYMENT_ACCOUNT="tester@example.com"
TEST_SMS_CODE="123456"

# 可选：绕过平台直连供应方探测目标
SUPPLIER_DIRECT_TEST_URL=""
```

## 1.2 依赖检查

```bash
command -v curl >/dev/null
command -v jq >/dev/null
```

## 1.3 执行入口

```bash
cd "/home/long/project/立交桥"
bash "scripts/supply-gate/run_all.sh"
```

---

## 2. SUP-004 账号挂载链路（UI-SUP-ACC-001~006）

执行脚本：

```bash
cd "/home/long/project/立交桥"
bash "scripts/supply-gate/sup004_accounts.sh"
```

最低断言：

1. 验证接口返回 `verify_status=pass/review_required`。
2. 创建账号成功并返回 `account_id`。
3. 激活/暂停接口返回状态迁移成功。
4. 审计日志接口可查询并返回 `request_id`。

证据输出：

1. `tests/supply/artifacts/sup004/*.json`
2. `tests/supply/artifacts/sup004/summary.txt`

---

## 3. SUP-005 套餐发布链路（UI-SUP-PKG-001~006）

执行脚本：

```bash
cd "/home/long/project/立交桥"
bash "scripts/supply-gate/sup005_packages.sh"
```

最低断言：

1. 草稿创建成功并返回 `package_id`。
2. 上架后状态为 `active`。
3. 暂停后状态为 `paused`。
4. 下架返回成功（`expired/paused` 合法）。
5. 批量调价返回 `success_count + failed_count = total`。
6. 复制成功并返回新的 `package_id`。

证据输出：

1. `tests/supply/artifacts/sup005/*.json`
2. `tests/supply/artifacts/sup005/summary.txt`

---

## 4. SUP-006 结算提现链路（UI-SUP-SET-001~005）

执行脚本：

```bash
cd "/home/long/project/立交桥"
bash "scripts/supply-gate/sup006_settlements.sh"
```

最低断言：

1. 账单查询成功返回 `summary`。
2. 提现申请成功返回 `settlement_id` 且状态 `pending`。
3. 撤销申请接口返回状态变更。
4. 对账单下载接口返回 `download_url`。
5. 收益流水接口返回分页与记录字段。

证据输出：

1. `tests/supply/artifacts/sup006/*.json`
2. `tests/supply/artifacts/sup006/summary.txt`

---

## 5. SUP-007 凭证边界专项（SEC-SUP-001~002）

执行脚本：

```bash
cd "/home/long/project/立交桥"
bash "scripts/supply-gate/sup007_boundary.sh"
```

最低断言：

1. 平台凭证访问主路径成功（映射 M-014）。
2. 外部 query key 请求被拒绝（映射 M-016）。
3. 响应/导出样本脱敏扫描无可复用凭证片段（映射 M-013）。
4. 若配置 `SUPPLIER_DIRECT_TEST_URL`，直连探测应失败或被阻断（映射 M-015）。

证据输出：

1. `tests/supply/artifacts/sup007/*.json`
2. `tests/supply/artifacts/sup007/summary.txt`

---

## 6. 回填要求

执行完成后，必须回填：

1. `tests/supply/ui_sup_acc_report_2026-03-28.md`
2. `tests/supply/ui_sup_pkg_report_2026-03-29.md`
3. `tests/supply/ui_sup_set_report_2026-03-29.md`
4. `tests/supply/sec_sup_boundary_report_2026-03-30.md`
5. `reports/supply_gate_review_2026-03-31.md`

所有回填项需要包含：

1. 结论（PASS/FAIL/BLOCKED）
2. 证据路径（json/screenshot/log）
3. 责任人签字
