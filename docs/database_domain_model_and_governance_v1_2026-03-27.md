# 数据库跨域模型与治理基线（v1.0）

- 版本：v1.0
- 日期：2026-03-27
- 状态：生效（数据库设计 SSOT 补丁）
- 适用范围：S0-S2 执行与验收
- 关联文档：
  - `docs/llm_gateway_prd_v1_2026-03-25.md`
  - `docs/supply_technical_design_enhanced_v1_2026-03-25.md`
  - `docs/technical_architecture_optimized_v2_2026-03-18.md`
  - `sql/postgresql/supply_schema_v1.sql`
  - `sql/postgresql/supply_schema_v1_patch_2026-03-27.sql`
  - `sql/postgresql/platform_core_schema_v1.sql`

---

## 1. 本次补齐的缺口

1. 仅有 `supply_*` 表，缺少 PRD P0/P1 的核心域（租户/项目/鉴权 key/账务总账/审计事件）。
2. 供应域缺少统一加密元数据字段，无法审计算法、KMS Key 版本与轮换状态。
3. 缺少统一单位字段（quota/cost/amount unit），跨域统计口径不稳定。
4. 审计字段不完整（request_id、trace_id、IP、operator、version）。
5. 索引以单列为主，未覆盖高频组合查询（租户+状态+时间）。

---

## 2. 最小跨域表模型（按 PRD P0/P1）

| 域 | 表 | 说明 |
|---|---|---|
| Core | `core_tenants` | 组织/租户主实体 |
| Core | `core_projects` | 项目/成本归因单元 |
| IAM | `iam_users` | 用户身份与角色 |
| Auth | `auth_platform_api_keys` | 平台签发凭证（仅 hash，不存明文） |
| Billing | `billing_accounts` | 预算账户与余额 |
| Billing | `billing_ledger_entries` | 借贷分录与请求级对账 |
| Routing | `routing_policies` | 策略版本、优先级、生效窗口 |
| Security | `security_kms_key_registry` | KMS Key 与加密算法版本登记 |
| Audit | `audit_events` | 全域审计事件（配置/账务/安全） |

DDL：`sql/postgresql/platform_core_schema_v1.sql`

---

## 3. 供应域字段补齐（在 v1 基础上增量）

### 3.1 加密字段（必须）

1. `*_cipher_algo`：默认 `AES-256-GCM`
2. `*_kms_key_alias`：KMS key alias（非 key 明文）
3. `*_key_version`：key 版本号
4. `*_fingerprint`：凭证摘要（不可逆）
5. `last_rotation_at`：上次轮换时间

### 3.2 单位与币种字段（必须）

1. `quota_unit`：`token/request/credit`
2. `price_unit`：`per_1m_tokens` 等
3. `amount_unit`：`minor`（分/厘）
4. `currency_code`：ISO 4217 三位码

### 3.3 审计与并发字段（必须）

1. `request_id`
2. `idempotency_key`
3. `audit_trace_id`
4. `created_ip` / `updated_ip`
5. `version`（乐观锁）

DDL：`sql/postgresql/supply_schema_v1_patch_2026-03-27.sql`

---

## 4. 索引策略（高频查询优先）

### 4.1 组合索引

1. `supply_accounts(user_id, status, updated_at desc)`
2. `supply_packages(user_id, status, updated_at desc)`
3. `supply_orders(buyer_user_id, status, created_at desc)`
4. `supply_settlements(user_id, status, updated_at desc)`
5. `billing_ledger_entries(billing_account_id, occurred_at desc)`

### 4.2 部分索引

1. `supply_packages` 的 active 查询（仅 `status=active`）
2. `supply_settlements` 的处理中唯一约束（仅 `status=processing`）

### 4.3 可观测索引

1. `request_id`
2. `trace_id`
3. `audit_trace_id`

说明：所有关键事件必须具备 request 级反查路径，满足“从告警到原始账务分录”单跳可达。

---

## 5. 迁移顺序与回滚策略

1. Phase-A：执行 `platform_core_schema_v1.sql`（新增表，无破坏性）。
2. Phase-B：执行 `supply_schema_v1_patch_2026-03-27.sql`（增列+增索引）。
3. Phase-C：灰度写入新字段（双写，不读取）。
4. Phase-D：回填历史数据（按日批，带校验）。
5. Phase-E：切换读路径到新字段并开启质量门禁。

回滚原则：
1. 新字段只增不删，读路径可切回旧字段。
2. 新索引可独立回退，不影响主流程事务。
3. 任一阶段失败立即冻结下一阶段，不跨阶段带病推进。

---

## 6. 质量验收清单（DB）

1. 结构验收：新增表/列/索引全部存在，且命名符合规范。
2. 安全验收：无明文凭证列，hash/指纹字段可用。
3. 一致性验收：账务分录借贷平衡，提现处理中单一约束生效。
4. 审计验收：关键写接口 100% 带 `request_id + trace_id`。
5. 性能验收：高频查询 P95 无劣化（对比 patch 前后）。

---

## 7. 约束声明

1. 本文与两个 SQL 文件共同构成数据库实施 SSOT。
2. 任何新增业务功能必须先选择所属域，再定义表/字段/索引，不允许“先代码后补库”。
3. 未通过本清单第 6 章，禁止进入发布门禁 `SUP-008` 与全局 `GO` 评审。
