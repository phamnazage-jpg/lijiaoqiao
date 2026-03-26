# 供应侧技术设计增强版（XR-001）

- 版本：v1.0
- 日期：2026-03-25
- 状态：生效（实施基线）
- 目标：补齐供应侧关键写路径的幂等、并发、事务、不变量与可靠性闭环
- 关联 SSOT：
  - `llm_gateway_subapi_evolution_plan_v4_2_2026-03-24.md`
  - `acceptance_gate_single_source_v1_2026-03-18.md`
  - `supply_button_level_prd_v1_2026-03-25.md`
  - `supply_api_contract_openapi_draft_v1_2026-03-25.yaml`

---

## 1. 设计边界与约束

1. 业务链路固定为：`用户A供给 -> 平台 -> 用户B购买平台服务`。
2. 供应方上游凭证仅平台托管，任何北向接口不得回显可复用凭证片段。
3. 所有关键写操作必须支持双键幂等：`request_id + idempotency_key`。
4. 所有状态迁移必须满足“显式前置状态 + 原子落库 + 审计可追溯”。
5. 所有跨系统副作用必须通过 Outbox/Saga 触发，禁止在数据库事务中直连外部系统。

---

## 2. 关键写路径与幂等协议

## 2.1 适用操作

1. `POST /api/v1/supply/accounts`
2. `POST /api/v1/supply/packages/{id}/publish`
3. `POST /api/v1/supply/packages/batch-price`
4. `POST /api/v1/supply/settlements/withdraw`
5. `POST /api/v1/supply/settlements/{id}/cancel`

## 2.2 入站协议（MUST）

1. Header 必填：`X-Request-Id`（UUID）
2. Header 必填：`Idempotency-Key`（长度 16-128）
3. 幂等作用域：`tenant_id + operator_id + api_path + idempotency_key`
4. 幂等有效期：`24h`（提现类可扩展到 `72h`）

## 2.3 语义规范

1. 首次成功：返回业务成功码（`200/201`）并写入幂等记录。
2. 重放同参：返回同一业务结果，`idempotent_replay=true`。
3. 重放异参：返回 `409 IDEMPOTENCY_PAYLOAD_MISMATCH`。
4. 首次处理中：返回 `202 IDEMPOTENCY_IN_PROGRESS`，携带 `retry_after_ms`。

## 2.4 存储建议（PostgreSQL）

```sql
create table if not exists supply_idempotency_record (
  id bigserial primary key,
  tenant_id bigint not null,
  operator_id bigint not null,
  api_path varchar(200) not null,
  idempotency_key varchar(128) not null,
  request_id varchar(64) not null,
  payload_hash char(64) not null,
  response_code int,
  response_body jsonb,
  status varchar(20) not null, -- processing/succeeded/failed
  expires_at timestamp not null,
  created_at timestamp not null default now(),
  updated_at timestamp not null default now(),
  unique (tenant_id, operator_id, api_path, idempotency_key)
);
```

---

## 3. 并发控制策略（按领域动作）

## 3.1 账号挂载与状态迁移

1. 账号状态变更（激活/暂停/禁用）采用乐观锁：`version` 字段 CAS 更新。
2. 激活操作 SQL 需带前置状态：`where id=? and status in ('pending','suspended') and version=?`。
3. 同一账号同一时刻只允许一个状态迁移事务；冲突返回 `409 SUP_ACC_4091`。

## 3.2 套餐发布与批量调价

1. 套餐单条迁移采用乐观锁，保证 `draft -> active -> paused -> expired` 不跳态。
2. 批量调价采用“分片事务 + 明细回执”模式，单条失败不回滚全部成功项。
3. 批量任务必须落审计明细：`total/success/failed/failed_items[]`。

## 3.3 提现发起与撤销

1. 发起提现采用悲观锁：`select ... for update` 锁定供应方可提现余额行。
2. 约束：同一供应方同一时刻最多 1 笔 `processing` 提现单。
3. 唯一约束建议：

```sql
create unique index if not exists uq_settlement_supplier_processing
on supply_settlement(supplier_id)
where status = 'processing';
```

4. 余额扣减与结算单创建必须同事务提交，任一失败整体回滚。

---

## 4. 领域不变量（Invariant）

| 编号 | 不变量 | 触发动作 | 拒绝码 |
|---|---|---|---|
| INV-ACC-001 | `active` 账号不可删除 | 删除账号 | `SUP_ACC_4092` |
| INV-ACC-002 | 账号 `disabled` 仅管理员可恢复 | 激活账号 | `SUP_ACC_4031` |
| INV-PKG-001 | `sold_out` 只能系统迁移 | 人工改状态 | `SUP_PKG_4092` |
| INV-PKG-002 | `expired` 套餐不可直接恢复 | 发布上架 | `SUP_PKG_4093` |
| INV-PKG-003 | 售价不得低于保护价 | 发布/调价 | `SUP_PKG_4001` |
| INV-SET-001 | `processing/completed` 不可撤销 | 撤销申请 | `SUP_SET_4092` |
| INV-SET-002 | 提现金额不得超过可提现余额 | 发起提现 | `SUP_SET_4001` |
| INV-SET-003 | 结算单金额与余额流水必须平衡 | 结算入账 | `SUP_SET_5002` |

说明：所有不变量失败必须写入审计事件 `invariant_violation`，并携带 `rule_code`。

---

## 5. 事务边界与副作用编排

## 5.1 本地事务内（必须原子）

1. 领域状态变更（账号/套餐/结算单）
2. 资金子账变更（冻结/解冻/可提现）
3. 幂等记录更新（`processing -> succeeded/failed`）
4. 审计日志落库（最小字段集）
5. Outbox 事件入库

## 5.2 事务外（异步执行）

1. 通知发送（站内信/邮件/短信）
2. 导出任务生成
3. 风险引擎异步评分
4. BI 聚合看板更新

## 5.3 Outbox 事件规范

1. 事件命名：`supply.{domain}.{action}.{result}`
2. 必填字段：`event_id/request_id/tenant_id/object_id/before_state/after_state`
3. 消费保障：至少一次投递 + 消费幂等（以 `event_id` 去重）

---

## 6. 失败注入与回滚策略

| 场景ID | 注入点 | 预期行为 | 验收点 |
|---|---|---|---|
| FI-001 | 提现创建后数据库超时 | 事务回滚，不产生挂单 | 余额不变、无孤儿单 |
| FI-002 | 幂等记录已存在同键异参 | 返回 409 | 不改业务状态 |
| FI-003 | 套餐发布时状态冲突 | 返回 409 | 状态不跳变 |
| FI-004 | 审计落库失败 | 主事务失败并回滚 | 无“成功但无审计” |
| FI-005 | Outbox 入库失败 | 主事务失败并回滚 | 无“状态已变更但无事件” |
| FI-006 | 导出服务不可用 | 主事务成功，异步重试 | 业务不阻塞 |
| FI-007 | 外部 query key 请求 | 网关拒绝 | M-016=100% |
| FI-008 | 响应误回显凭证片段 | 安全门禁阻断 | M-013=0 |

---

## 7. SLO 与页面动作映射

| 页面按钮 | API | SLI | SLO | Error Budget |
|---|---|---|---|---|
| BTN-ACC-001 立即验证 | `/accounts/verify` | 可用率 + P95 | 可用率 >= 99.9%，P95 <= 800ms | 月度 0.1% |
| BTN-ACC-002 提交挂载 | `/accounts` | 成功率 | 成功率 >= 99.5% | 月度 0.5% |
| BTN-PKG-002 发布上架 | `/packages/{id}/publish` | 成功率 + 冲突率 | 成功率 >= 99.5%，冲突率 <= 0.3% | 月度 0.5% |
| BTN-PKG-005 批量调价 | `/packages/batch-price` | 局部成功可解释率 | 明细可解释率 = 100% | 0 |
| BTN-SET-002 发起提现 | `/settlements/withdraw` | 一致性 + 时延 | `billing_error_rate_pct<=0.1%`，P95<=1200ms | 与 M-004 联动 |
| BTN-SET-003 撤销申请 | `/settlements/{id}/cancel` | 成功率 | 成功率 >= 99.9% | 月度 0.1% |

---

## 8. 审计与安全对齐

1. 所有关键写请求必须记录：`request_id/idempotency_key/operator_id/object_id/result_code`。
2. 错误体、导出、日志统一经过脱敏扫描；命中即触发 P0。
3. 与门禁指标映射：
   1. M-013：凭证泄露事件数=0
   2. M-014：平台凭证入站覆盖率=100%
   3. M-015：需求方绕平台直连事件=0
   4. M-016：外部 query key 拒绝率=100%

---

## 9. 实施与验收清单

1. API 网关：校验并透传 `X-Request-Id`、`Idempotency-Key`。
2. 数据库：新增幂等表、状态版本字段、提现唯一索引。
3. 服务层：统一幂等拦截器与冲突返回码。
4. 测试层：新增并发冲突、幂等重放、失败注入专项。
5. 门禁层：将 FI-001~FI-008 纳入 `SUP-*` 与 `SEC-*` Gate。
6. 证据层：执行日志、指标截图、审计抽样、签署记录齐全。

达到以上 6 项即视为 XR-001 关闭。
