> 归档状态：已归档
> 归档批次：ARCHIVE-HISTORY-2026-04-14-A01
> 归档标识：AHR-20260414-005
> 当前用途：历史快照，仅供追溯；不得作为现行门禁、发布决议或实现状态事实源。
> 当前事实源：`docs/plans/2026-04-14-repo-integrity-refactor-plan.md`、`reports/archive/gate_verification/` 与仍在维护的签署/门禁文档。

# 数据库基线执行验证报告（2026-03-27）

- 执行环境：PostgreSQL 15 (`127.0.0.1:34603`)
- 执行账号：`mosquito`
- 验证库：`lijiaoqiao_design_review_20260327`
- 执行人：Codex

---

## 1. 执行 SQL 清单

1. `sql/postgresql/platform_core_schema_v1.sql`
2. `sql/postgresql/supply_schema_v1.sql`
3. `sql/postgresql/supply_schema_v1_patch_2026-03-27.sql`

原始日志：
1. `reports/db/sql_apply_2026-03-27.log`

---

## 2. 执行结果

1. 三份 SQL 均执行成功（全部到 `COMMIT`）。
2. 表总数：`15`
3. 索引总数：`82`
4. 关键字段命中数：`37`

结构快照：
1. `reports/db/tables_2026-03-27.txt`
2. `reports/db/indexes_2026-03-27.txt`
3. `reports/db/key_columns_2026-03-27.txt`

---

## 3. 关键验收点核对

1. 跨域核心表（Core/IAM/Auth/Billing/Routing/Security/Audit）已创建。
2. 供应域 patch 中加密字段已生效：
   - `credential_cipher_algo`
   - `credential_kms_key_alias`
   - `credential_key_version`
   - `credential_fingerprint`
3. 单位字段已生效：
   - `quota_unit`
   - `price_unit`
   - `amount_unit`
   - `currency_code`
4. 审计与幂等字段已生效：
   - `request_id`
   - `idempotency_key`
   - `audit_trace_id`
   - `version`
5. 关键组合索引与部分索引已创建（含 `uq_supply_settlements_user_processing`）。

---

## 4. 问题与修复记录

1. 首次执行失败原因：新增 SQL 文件字符串默认值引号丢失。
2. 修复动作：重写 `platform_core_schema_v1.sql` 与 `supply_schema_v1_patch_2026-03-27.sql`，统一字符串字面量语法。
3. 修复后复跑结果：全部通过。

---

## 5. 结论

结论：**通过（设计层 SQL 可执行）**。

后续建议：
1. 在目标测试环境执行同样脚本并对比 `EXPLAIN` 计划。
2. 将执行日志纳入 `SUP-008` 与 `GO` 决策证据包。
