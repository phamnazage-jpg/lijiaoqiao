# Supply API 数据字典 v1.0

> **文档版本**: v1.0
> **创建日期**: 2026-04-07
> **基于**: supply_schema_v1.sql

---

## 1. supply_accounts (供应商账户表)

| 字段名 | 类型 | 约束 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | BIGINT | PRIMARY KEY | 自增 | 账户唯一标识 |
| user_id | BIGINT | NOT NULL | - | 所属用户ID |
| platform | VARCHAR(50) | NOT NULL | - | 平台标识 (openai/anthropic/azure等) |
| account_type | VARCHAR(20) | NOT NULL | - | 账户类型: api_key/oauth |
| account_name | VARCHAR(100) | - | - | 账户显示名称 |
| encrypted_credentials | TEXT | NOT NULL | - | 加密存储的凭证 (AES-256-GCM) |
| key_id | VARCHAR(100) | - | - | 凭证密钥标识 |
| status | VARCHAR(20) | NOT NULL | 'pending' | 状态: pending/active/suspended/disabled |
| risk_level | VARCHAR(20) | NOT NULL | 'normal' | 风险等级: low/normal/high |
| total_quota | NUMERIC(20,6) | - | - | 账户总配额 |
| available_quota | NUMERIC(20,6) | - | - | 可用配额 |
| frozen_quota | NUMERIC(20,6) | NOT NULL | 0 | 冻结配额 |
| is_verified | BOOLEAN | - | FALSE | 是否已验证 |
| verified_at | TIMESTAMPTZ | - | - | 验证时间 |
| last_check_at | TIMESTAMPTZ | - | - | 最后检查时间 |
| tos_compliant | BOOLEAN | - | TRUE | TOS合规状态 |
| tos_check_result | TEXT | - | - | TOS检查结果 |
| total_requests | BIGINT | - | 0 | 累计请求数 |
| total_tokens | BIGINT | - | 0 | 累计使用Token数 |
| total_cost | NUMERIC(20,6) | - | 0 | 累计消费金额 |
| success_rate | NUMERIC(5,2) | - | 0 | 请求成功率(%) |
| risk_score | INT | - | 0 | 风险评分 (0-100) |
| risk_reason | TEXT | - | - | 风险原因 |
| is_frozen | BOOLEAN | - | FALSE | 是否被冻结 |
| frozen_reason | TEXT | - | - | 冻结原因 |
| created_at | TIMESTAMPTZ | - | CURRENT_TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMPTZ | - | CURRENT_TIMESTAMP | 更新时间 |
| created_by | BIGINT | - | - | 创建人 |
| updated_by | BIGINT | - | - | 更新人 |

**索引**:
- idx_supply_accounts_user_id (user_id)
- idx_supply_accounts_platform (platform)
- idx_supply_accounts_status (status)
- idx_supply_accounts_risk_level (risk_level)

---

## 2. supply_packages (供应套餐表)

| 字段名 | 类型 | 约束 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | BIGINT | PRIMARY KEY | 自增 | 套餐唯一标识 |
| supply_account_id | BIGINT | NOT NULL, FK | - | 关联供应商账户 |
| user_id | BIGINT | NOT NULL | - | 创建人用户ID |
| platform | VARCHAR(50) | NOT NULL | - | 平台标识 |
| model | VARCHAR(100) | NOT NULL | - | 模型标识 |
| total_quota | NUMERIC(20,6) | NOT NULL | - | 套餐总配额 |
| available_quota | NUMERIC(20,6) | NOT NULL | - | 可用配额 |
| sold_quota | NUMERIC(20,6) | - | 0 | 已售配额 |
| reserved_quota | NUMERIC(20,6) | - | 0 | 预留配额 |
| price_per_1m_input | NUMERIC(20,6) | - | - | 每百万输入Token价格 |
| price_per_1m_output | NUMERIC(20,6) | - | - | 每百万输出Token价格 |
| min_purchase | NUMERIC(20,6) | - | - | 最小购买量 |
| start_at | TIMESTAMPTZ | - | - | 生效时间 |
| end_at | TIMESTAMPTZ | - | - | 失效时间 |
| valid_days | INT | - | - | 有效天数 |
| status | VARCHAR(20) | NOT NULL | 'draft' | 状态: draft/active/paused/sold_out/expired |
| max_concurrent | INT | - | 10 | 最大并发数 |
| rate_limit_rpm | INT | - | 60 | 每分钟限流 |
| total_orders | INT | - | 0 | 累计订单数 |
| total_revenue | NUMERIC(20,6) | - | 0 | 累计收入 |
| rating | NUMERIC(3,2) | - | 0 | 平均评分 |
| rating_count | INT | - | 0 | 评分次数 |
| created_at | TIMESTAMPTZ | - | CURRENT_TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMPTZ | - | CURRENT_TIMESTAMP | 更新时间 |

**索引**:
- idx_supply_packages_supply_account_id (supply_account_id)
- idx_supply_packages_user_id (user_id)
- idx_supply_packages_platform_model (platform, model)
- idx_supply_packages_status (status)

---

## 3. supply_orders (供应订单表)

| 字段名 | 类型 | 约束 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | BIGINT | PRIMARY KEY | 自增 | 订单唯一标识 |
| order_no | VARCHAR(64) | NOT NULL, UNIQUE | - | 订单编号 |
| buyer_user_id | BIGINT | NOT NULL | - | 买家用户ID |
| buyer_team_id | BIGINT | - | - | 买家团队ID |
| supply_account_id | BIGINT | NOT NULL, FK | - | 供应商账户ID |
| supplier_user_id | BIGINT | NOT NULL | - | 供应商用户ID |
| supply_package_id | BIGINT | NOT NULL, FK | - | 套餐ID |
| platform | VARCHAR(50) | NOT NULL | - | 平台标识 |
| model | VARCHAR(100) | NOT NULL | - | 模型标识 |
| quota_amount | NUMERIC(20,6) | NOT NULL | - | 购买配额量 |
| quota_tokens | BIGINT | - | - | 购买Token配额 |
| unit_price | NUMERIC(20,6) | NOT NULL | - | 单价 |
| total_amount | NUMERIC(20,6) | NOT NULL | - | 总金额 |
| platform_fee | NUMERIC(20,6) | NOT NULL | - | 平台手续费 |
| supplier_earnings | NUMERIC(20,6) | NOT NULL | - | 供应商实收 |
| status | VARCHAR(20) | NOT NULL | 'pending' | 状态: pending/paid/using/expired/refunded |
| used_quota | NUMERIC(20,6) | - | 0 | 已使用配额 |
| remaining_quota | NUMERIC(20,6) | - | - | 剩余配额 |
| expired_at | TIMESTAMPTZ | - | - | 过期时间 |
| payment_method | VARCHAR(20) | - | - | 支付方式 |
| paid_at | TIMESTAMPTZ | - | - | 支付时间 |
| payment_transaction_id | VARCHAR(100) | - | - | 支付流水号 |
| created_at | TIMESTAMPTZ | - | CURRENT_TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMPTZ | - | CURRENT_TIMESTAMP | 更新时间 |

**索引**:
- idx_supply_orders_buyer_user_id (buyer_user_id)
- idx_supply_orders_supplier_user_id (supplier_user_id)
- idx_supply_orders_supply_package_id (supply_package_id)
- idx_supply_orders_status (status)

---

## 4. supply_usage_records (使用记录表)

| 字段名 | 类型 | 约束 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | BIGINT | PRIMARY KEY | 自增 | 记录唯一标识 |
| order_id | BIGINT | NOT NULL, FK | - | 关联订单ID |
| buyer_user_id | BIGINT | NOT NULL | - | 买家用户ID |
| supply_account_id | BIGINT | NOT NULL, FK | - | 供应商账户ID |
| supplier_user_id | BIGINT | NOT NULL | - | 供应商用户ID |
| request_id | VARCHAR(64) | NOT NULL | - | 请求唯一ID |
| upstream_request_id | VARCHAR(128) | - | - | 上游请求ID |
| api_key_id | BIGINT | - | - | API Key ID |
| platform | VARCHAR(50) | NOT NULL | - | 平台标识 |
| model | VARCHAR(100) | NOT NULL | - | 模型标识 |
| endpoint | VARCHAR(100) | NOT NULL | - | API端点 |
| request_tokens | BIGINT | - | - | 请求Token数 |
| response_tokens | BIGINT | - | - | 响应Token数 |
| total_tokens | BIGINT | STORED GENERATED | - | 总Token数 (计算字段) |
| input_cost | NUMERIC(20,6) | - | - | 输入费用 |
| output_cost | NUMERIC(20,6) | - | - | 输出费用 |
| total_cost | NUMERIC(20,6) | NOT NULL | - | 总费用 |
| unit_price | NUMERIC(20,6) | NOT NULL | - | 单价 |
| response_status | INT | - | - | 响应状态码 |
| latency_ms | INT | - | - | 延迟(毫秒) |
| error_message | TEXT | - | - | 错误信息 |
| success | BOOLEAN | - | TRUE | 是否成功 |
| started_at | TIMESTAMPTZ | NOT NULL | - | 开始时间 |
| completed_at | TIMESTAMPTZ | - | - | 完成时间 |
| created_at | TIMESTAMPTZ | - | CURRENT_TIMESTAMP | 创建时间 |

**索引**:
- idx_supply_usage_records_request_id (request_id)
- idx_supply_usage_records_order_id (order_id)
- idx_supply_usage_records_supply_account_id (supply_account_id)
- idx_supply_usage_records_platform_model (platform, model)
- idx_supply_usage_records_started_at (started_at)

---

## 5. supply_earnings (收益表)

| 字段名 | 类型 | 约束 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | BIGINT | PRIMARY KEY | 自增 | 收益记录ID |
| user_id | BIGINT | NOT NULL | - | 用户ID |
| supply_account_id | BIGINT | FK | - | 供应商账户ID |
| order_id | BIGINT | FK | - | 关联订单ID |
| usage_record_id | BIGINT | FK | - | 使用记录ID |
| earnings_type | VARCHAR(20) | NOT NULL | - | 类型: usage/bonus/refund |
| amount | NUMERIC(20,6) | NOT NULL | - | 收益金额 |
| currency | VARCHAR(10) | - | 'CNY' | 币种 |
| status | VARCHAR(20) | NOT NULL | 'pending' | 状态: pending/available/withdrawn/frozen |
| available_amount | NUMERIC(20,6) | - | 0 | 可用金额 |
| frozen_amount | NUMERIC(20,6) | - | 0 | 冻结金额 |
| withdrawn_amount | NUMERIC(20,6) | - | 0 | 已提现金额 |
| description | TEXT | - | - | 描述 |
| earned_at | TIMESTAMPTZ | - | CURRENT_TIMESTAMP | 收益时间 |
| available_at | TIMESTAMPTZ | - | - | 可用时间 |
| created_at | TIMESTAMPTZ | - | CURRENT_TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMPTZ | - | CURRENT_TIMESTAMP | 更新时间 |

**索引**:
- idx_supply_earnings_user_id (user_id)
- idx_supply_earnings_status (status)
- idx_supply_earnings_earned_at (earned_at)

---

## 6. supply_settlements (结算表)

| 字段名 | 类型 | 约束 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | BIGINT | PRIMARY KEY | 自增 | 结算单ID |
| settlement_no | VARCHAR(64) | NOT NULL, UNIQUE | - | 结算单号 |
| user_id | BIGINT | NOT NULL | - | 用户ID (供应商) |
| total_amount | NUMERIC(20,6) | NOT NULL | - | 总金额 |
| fee_amount | NUMERIC(20,6) | - | 0 | 手续费 |
| net_amount | NUMERIC(20,6) | NOT NULL | - | 净金额 |
| status | VARCHAR(20) | NOT NULL | 'pending' | 状态: pending/processing/completed/failed |
| payment_method | VARCHAR(20) | - | - | 支付方式 |
| payment_account | VARCHAR(100) | - | - | 支付账户 |
| payment_transaction_id | VARCHAR(100) | - | - | 支付流水号 |
| paid_at | TIMESTAMPTZ | - | - | 支付时间 |
| period_start | DATE | NOT NULL | - | 结算周期开始 |
| period_end | DATE | NOT NULL | - | 结算周期结束 |
| total_orders | INT | - | 0 | 关联订单数 |
| total_usage_records | INT | - | 0 | 关联使用记录数 |
| version | INT | - | 0 | 乐观锁版本号 |
| created_at | TIMESTAMPTZ | - | CURRENT_TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMPTZ | - | CURRENT_TIMESTAMP | 更新时间 |

**索引**:
- idx_supply_settlements_user_id (user_id)
- idx_supply_settlements_status (status)
- idx_supply_settlements_period (period_start, period_end)

---

## 7. supply_idempotency_records (幂等记录表)

| 字段名 | 类型 | 约束 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | BIGINT | PRIMARY KEY | 自增 | 记录ID |
| tenant_id | BIGINT | NOT NULL | - | 租户ID |
| operator_id | BIGINT | NOT NULL | - | 操作人ID |
| api_path | VARCHAR(200) | NOT NULL | - | API路径 |
| idempotency_key | VARCHAR(128) | NOT NULL | - | 幂等键 |
| request_id | VARCHAR(64) | NOT NULL | - | 请求ID |
| payload_hash | CHAR(64) | NOT NULL | - | 请求体SHA256摘要 |
| response_code | INT | - | - | 响应码 |
| response_body | JSONB | - | - | 响应体 |
| status | VARCHAR(20) | NOT NULL | 'processing' | 状态: processing/succeeded/failed |
| expires_at | TIMESTAMPTZ | NOT NULL | - | 过期时间 (默认24h,提现72h) |
| created_at | TIMESTAMPTZ | - | CURRENT_TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMPTZ | - | CURRENT_TIMESTAMP | 更新时间 |

**索引**:
- UNIQUE (tenant_id, operator_id, api_path, idempotency_key)
- idx_idempotency_tenant_operator_path_key (tenant_id, operator_id, api_path, idempotency_key) WHERE expires_at > CURRENT_TIMESTAMP
- idx_idempotency_request_id (request_id)
- idx_idempotency_expires_at (expires_at) WHERE status != 'processing'
- idx_idempotency_status_expires (status, expires_at)

---

## 8. 枚举类型定义

### account_type (账户类型)
| 值 | 说明 |
|----|------|
| api_key | API Key认证 |
| oauth | OAuth认证 |

### account_status (账户状态)
| 值 | 说明 |
|----|------|
| pending | 待验证 |
| active | 激活 |
| suspended | 暂停 |
| disabled | 禁用 |

### risk_level (风险等级)
| 值 | 说明 |
|----|------|
| low | 低风险 |
| normal | 正常 |
| high | 高风险 |

### package_status (套餐状态)
| 值 | 说明 |
|----|------|
| draft | 草稿 |
| active | 生效中 |
| paused | 已暂停 |
| sold_out | 售罄 |
| expired | 已过期 |

### order_status (订单状态)
| 值 | 说明 |
|----|------|
| pending | 待支付 |
| paid | 已支付 |
| using | 使用中 |
| expired | 已过期 |
| refunded | 已退款 |

### settlement_status (结算状态)
| 值 | 说明 |
|----|------|
| pending | 待处理 |
| processing | 处理中 |
| completed | 已完成 |
| failed | 失败 |

### earnings_type (收益类型)
| 值 | 说明 |
|----|------|
| usage | 使用收益 |
| bonus | 奖励 |
| refund | 退款 |

### earnings_status (收益状态)
| 值 | 说明 |
|----|------|
| pending | 待确认 |
| available | 可提现 |
| withdrawn | 已提现 |
| frozen | 冻结中 |

---

## 9. 数据类型说明

| 类型 | 说明 |
|------|------|
| BIGINT | 64位整数，主键和外部键使用 |
| VARCHAR(n) | 变长字符串，最大n字符 |
| TEXT | 无长度限制的文本 |
| NUMERIC(p,s) | 精确数值，p为总位数，s为小数位 |
| BOOLEAN | 布尔值 |
| TIMESTAMPTZ | 带时区的时间戳 |
| DATE | 日期 |
| JSONB | JSON二进制格式，支持索引 |
| CHAR(n) | 定长字符串 |

---

## 10. 字段命名规范

| 前缀/后缀 | 说明 | 示例 |
|-----------|------|------|
| _id | ID字段 | user_id, order_id |
| _at | 时间字段 | created_at, paid_at |
| _amount | 金额字段 | total_amount, net_amount |
| is_ | 布尔字段 | is_verified, is_frozen |
| total_ | 累计字段 | total_requests, total_cost |
| available_ | 可用字段 | available_quota, available_amount |
| encrypted_ | 加密字段 | encrypted_credentials |

---

> **维护记录**:
> - v1.0 (2026-04-07): 初始版本，基于supply_schema_v1.sql
