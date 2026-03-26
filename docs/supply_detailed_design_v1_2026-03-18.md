# 用户供应LLM功能 - 完整详细设计（补充版）

> 本文档对"用户分享LLM供应"功能进行完整详细的设计补充，包括安全机制、账单记录等完整内容。

---

## 1. 功能架构

### 1.1 整体架构

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           供应侧功能完整架构                                      │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                        供应方侧（User A）                               │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │   │
│  │  │ 账号挂载   │  │ 套餐发布   │  │ 收益查看   │  │  提现      │   │   │
│  │  │ 模块       │  │ 模块       │  │ 模块       │  │  模块      │   │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                    │                                           │
│                                    ▼                                           │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                        平台核心层                                        │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │   │
│  │  │ 账号验证   │  │ 套餐管理   │  │ 调度引擎   │  │  计费引擎   │   │   │
│  │  │ 服务       │  │ 服务       │  │             │  │             │   │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘   │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │   │
│  │  │ 风控服务   │  │ 合规检测   │  │ 通知服务   │  │ 审计服务   │   │   │
│  │  │             │  │ 服务       │  │             │  │             │   │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                    │                                           │
│                                    ▼                                           │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                        需求方侧（User B）                               │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │   │
│  │  │ 套餐选购   │  │  调用API   │  │ 使用账单   │  │  消耗统计   │   │   │
│  │  │ 模块       │  │  模块       │  │ 模块       │  │  模块      │   │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. 安全机制详细设计

### 2.1 账号安全

#### 2.1.1 账号挂载安全流程

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        账号挂载安全验证流程                                   │
└─────────────────────────────────────────────────────────────────────────────┘

步骤1: 用户提交账号
       │
       ▼
步骤2: 账号格式校验
       │  ├── API Key 格式检查
       │  ├── OAuth 授权链接生成
       │  └── 格式不合格 → 拒绝
       ▼
步骤3: 账号有效性验证
       │  ├── 调用供应商 API 验证账号可用
       │  ├── 获取账号基本信息
       │  └── 验证失败 → 拒绝
       ▼
步骤4: 额度查询
       │  ├── 获取当前剩余额度
       │  ├── 记录额度快照
       │  └── 额度不足 → 警告
       ▼
步骤5: ToS 合规检查
       │  ├── 检查供应商 ToS 是否允许共享
       │  ├── 检查账号类型是否合规
       │  └── 不合规 → 拒绝
       ▼
步骤6: 风险评估
       │  ├── 账号历史行为分析
       │  ├── 异常检测
       │  └── 高风险 → 人工审核
       ▼
步骤7: 加密存储
       │  ├── 使用 KMS 加密
       │  ├── 生成账号唯一标识
       │  └── 存储到数据库
       ▼
步骤8: 通知用户
            ├── 挂载成功/失败通知
            └── 后续操作指引
```

#### 2.1.2 账号存储安全

| 安全措施 | 说明 | 实现 |
|----------|------|------|
| **加密存储** | API Key 必须加密存储 | AES-256-GCM 加密 |
| **KMS 集成** | 使用密钥管理服务 | AWS KMS / 自建 |
| **字段级加密** | 敏感字段单独加密 | 密钥ID + 密文 |
| **访问控制** | 最小权限原则 | RBAC 控制 |
| **审计日志** | 所有访问记录日志 | 操作人 + 时间 + IP |
| **禁止导出** | 禁止明文导出 Key | 脱敏展示 |

#### 2.1.3 凭证边界强制约束

1. 供应方上游凭证仅由平台密态托管（KMS + 字段级加密）。
2. 需求方只允许使用平台签发凭证调用统一入口。
3. 平台禁止向需求方返回供应方上游凭证（API/控制台/导出均不允许）。

### 2.2 调用安全

#### 2.2.1 请求验证流程

```
请求进入
    │
    ▼
┌─────────────────┐
│  1. API Key 验证 │ ──▶ 无效 → 401 Unauthorized
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  2. 套餐有效性  │ ──▶ 过期/售罄 → 403 Forbidden
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  3. 额度检查    │ ──▶ 额度不足 → 402 Payment Required
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  4. 风控检查    │ ──▶ 风险拦截 → 429 Too Many Requests
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  5. ToS 合规   │ ──▶ 违规拦截 → 403 Forbidden
└────────┬────────┘
         │
         ▼
    转发到上游
```

#### 2.2.2 安全防护措施

| 防护项 | 说明 | 配置 |
|--------|------|------|
| **IP 白名单** | 限制调用来源 IP | 可配置 |
| **Referer 校验** | 限制调用来源域名 | 可配置 |
| **调用频率限制** | RPM/TPM 限制 | 按套餐配置 |
| **并发限制** | 同时请求数限制 | 按套餐配置 |
| **模型限制** | 可用模型白名单 | 按套餐配置 |
| **额度预警** | 额度低于阈值告警 | 可配置 |

### 2.3 防欺诈机制

#### 2.3.1 欺诈检测规则

| 规则 | 描述 | 动作 |
|------|------|------|
| **额度异常消耗** | 单日消耗 > 平均3倍 | 告警 + 审核 |
| **短时间大量调用** | 1分钟内 > 100次 | 限流 + 审核 |
| **新账号高额使用** | 注册24h内使用 > $100 | 审核 |
| **跨地区调用** | IP 地区突然变化 | 告警 |
| **模式异常** | 调用模式偏离历史 | 告警 |
| **账号共享检测** | 多 IP 同时使用 | 封禁 + 审核 |

#### 2.3.2 保证金机制

| 供应方类型 | 保证金要求 | 退还条件 |
|------------|------------|----------|
| 个人 | ¥500 | 最后一笔交易30天后无异常 |
| 企业 | ¥5,000 | 最后一笔交易90天后无异常 |

**扣除场景**：
- 欺诈行为
- 账号封禁导致需求方损失
- 恶意套现

---

## 3. 账单与记录详细设计

### 3.1 数据模型

执行口径说明（重要）：
1. 本节内联 SQL 用于逻辑模型说明，不作为生产执行脚本。
2. 生产执行 DDL 以 `sql/postgresql/supply_schema_v1.sql` 为唯一脚本来源（PostgreSQL 方言）。
3. 若内联示意与执行脚本冲突，以执行脚本为准。

#### 3.1.1 供应方账号表

执行 DDL：`sql/postgresql/supply_schema_v1.sql`（表：`supply_accounts`）。

关键字段：
1. 基础：`id/user_id/platform/account_type/status`。
2. 凭证：`encrypted_credentials/key_id`（仅密态托管）。
3. 风控：`risk_level/risk_score/is_frozen`。
4. 审计：`created_at/updated_at/created_by/updated_by`。

#### 3.1.2 供应套餐表

执行 DDL：`sql/postgresql/supply_schema_v1.sql`（表：`supply_packages`）。

关键字段：
1. 关联：`supply_account_id/user_id/platform/model`。
2. 定价：`price_per_1m_input/price_per_1m_output`。
3. 额度：`total_quota/available_quota/sold_quota/reserved_quota`。
4. 状态：`draft/active/paused/sold_out/expired`。

#### 3.1.3 订单表

执行 DDL：`sql/postgresql/supply_schema_v1.sql`（表：`supply_orders`）。

关键字段：
1. 订单主键：`id/order_no`。
2. 交易信息：`buyer_user_id/supplier_user_id/supply_package_id`。
3. 金额信息：`total_amount/platform_fee/supplier_earnings`。
4. 状态：`pending/paid/using/expired/refunded`。

#### 3.1.4 使用记录表（详细到每一次调用）

执行 DDL：`sql/postgresql/supply_schema_v1.sql`（表：`supply_usage_records`）。

关键字段：
1. 追踪主键：`request_id/upstream_request_id/order_id`。
2. 模型维度：`platform/model/endpoint`。
3. 计费维度：`request_tokens/response_tokens/total_tokens/total_cost`。
4. 响应维度：`response_status/latency_ms/success`。

#### 3.1.5 供应方收益表

执行 DDL：`sql/postgresql/supply_schema_v1.sql`（表：`supply_earnings`）。

关键字段：
1. 收益类型：`usage/bonus/refund`。
2. 资金状态：`pending/available/withdrawn/frozen`。
3. 金额字段：`amount/available_amount/frozen_amount/withdrawn_amount`。

#### 3.1.6 结算记录表

执行 DDL：`sql/postgresql/supply_schema_v1.sql`（表：`supply_settlements`）。

关键字段：
1. 结算状态：`pending/processing/completed/failed`。
2. 金额字段：`total_amount/fee_amount/net_amount`。
3. 周期字段：`period_start/period_end`。

### 3.2 账单查询API

#### 3.2.1 供应方账单API

```yaml
# 供应方账单查询
GET /api/v1/supplier/billing

Query Parameters:
- start_date: string (可选) 开始日期 YYYY-MM-DD
- end_date: string (可选) 结束日期 YYYY-MM-DD
- page: int (可选) 页码
- page_size: int (可选) 每页数量

Response:
{
  "data": {
    "period": {
      "start": "2026-03-01",
      "end": "2026-03-31"
    },
    "summary": {
      "total_revenue": 12500.00,      # 总收入
      "total_orders": 156,            # 订单数
      "total_usage": 500000000,      # 总tokens
      "total_requests": 15800,        # 总请求数
      "avg_success_rate": 99.2,      # 平均成功率
      "platform_fee": 1875.00,       # 平台服务费
      "net_earnings": 10625.00       # 净收益
    },
    "by_platform": [
      {
        "platform": "openai",
        "revenue": 8000.00,
        "orders": 100,
        "tokens": 320000000,
        "success_rate": 99.5
      },
      {
        "platform": "anthropic",
        "revenue": 4500.00,
        "orders": 56,
        "tokens": 180000000,
        "success_rate": 98.8
      }
    ],
    "by_model": [...],
    "trend": [...]
  },
  "pagination": {...}
}
```

#### 3.2.2 需求方账单API

```yaml
# 需求方账单查询
GET /api/v1/consumer/billing

Response:
{
  "data": {
    "balance": {
      "total_paid": 5000.00,         # 已支付总额
      "total_used": 3200.00,        # 已使用
      "remaining": 1800.00,        # 剩余
      "frozen": 0.00                # 冻结
    },
    "usage": {
      "this_month": {
        "tokens": 120000000,
        "cost": 960.00,
        "requests": 5200
      },
      "by_platform": [...],
      "by_model": [...]
    },
    "orders": [...]
  }
}
```

### 3.3 详细使用记录

```yaml
# 详细使用记录查询
GET /api/v1/consumer/usage-records

Query Parameters:
- start_date: string
- end_date: string
- platform: string (可选)
- model: string (可选)
- success: boolean (可选)
- page: int
- page_size: int

Response:
{
  "data": [
    {
      "id": 1001,
      "request_id": "req_abc123",
      "order_id": 500,
      "platform": "openai",
      "model": "gpt-4o",
      "endpoint": "/v1/chat/completions",
      "tokens": {
        "input": 1500,
        "output": 800,
        "total": 2300
      },
      "cost": {
        "input": 0.0075,     # $7.5/1M
        "output": 0.032,     # $32/1M
        "total": 0.0395      # $0.0395
      },
      "latency_ms": 1250,
      "status": "success",
      "timestamp": "2026-03-18T10:30:00Z"
    }
  ],
  "pagination": {...}
}
```

---

## 4. 完整业务流程

### 4.1 供应方完整流程

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          供应方完整业务流程                                  │
└─────────────────────────────────────────────────────────────────────────────┘

1. 入驻认证
   ├── 注册账号
   ├── 实名认证 (个人/企业)
   └── 缴纳保证金
        │
        ▼
2. 账号挂载
   ├── 选择供应商
   ├── 选择认证方式 (API Key / OAuth)
   ├── 提交凭证
   ├── 平台验证 (有效性/额度/ToS)
   └── 通过 → 激活账号
        │
        ▼
3. 套餐发布
   ├── 选择要共享的模型
   ├── 设置配额 (全部/部分额度)
   ├── 设置售价
   ├── 设置有效期
   └── 上架销售
        │
        ▼
4. 日常运营
   ├── 监控账号状态
   ├── 查看订单通知
   ├── 处理异常 (如有)
   └── 收益查看
        │
        ▼
5. 收益结算
   ├── 收益累积 (T+7 可提现)
   ├── 申请提现
   ├── 平台审核
   └── 到账
```

### 4.2 需求方完整流程

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          需求方完整业务流程                                  │
└─────────────────────────────────────────────────────────────────────────────┘

1. 浏览选购
   ├── 浏览可用套餐
   ├── 按供应商/模型筛选
   ├── 查看套餐详情 (额度/价格/评价)
   └── 选择购买
        │
        ▼
2. 下单购买
   ├── 选择配额数量
   ├── 确认价格
   ├── 选择支付方式
   └── 支付成功
        │
        ▼
3. 获取凭证
   ├── 获取平台 API Key（平台签发）
   ├── 不返回供应方上游凭证
   ├── 配置使用限制
   └── 开始使用
        │
        ▼
4. 使用API
   ├── 调用统一API
   ├── 实时扣减配额
   ├── 查看使用统计
   └── 配额不足 → 续费
        │
        ▼
5. 账单管理
   ├── 查看使用明细
   ├── 下载账单
   └── 对账
```

---

## 5. 调度策略

### 5.1 请求调度流程

```
请求进入 (指定模型: gpt-4o)
    │
    ▼
查找可用套餐 (gpt-4o + 有剩余配额 + 正常状态)
    │
    ├── 多个套餐可用
    │       │
    │       ▼
    │   选择策略:
    │   1. 最低价格优先
    │   2. 负载均衡 (选择负载最低)
    │   3. 轮询
    │   4. 供应商偏好
    │       │
    │       ▼
    │   选择最优套餐
    │
    └── 单个套餐可用 → 使用该套餐
        │
        ▼
检查套餐配额 (足够? → 继续; 不足? → 拒绝)
    │
    ▼
检查账户状态 (正常? → 继续; 异常? → 拒绝)
    │
    ▼
转发到上游供应商
    │
    ▼
获取响应
    │
    ▼
记录使用记录
    │
    ├── 更新订单剩余配额
    ├── 更新套餐已售配额
    ├── 计算费用
    └── 更新供应方收益
        │
        ▼
返回响应给需求方
```

### 5.2 调度策略配置

| 策略 | 说明 | 适用场景 |
|------|------|----------|
| **最低价格** | 选择售价最低的套餐 | 成本优先 |
| **负载均衡** | 选择负载最低的套餐 | 性能优先 |
| **轮询** | 依次选择各套餐 | 公平使用 |
| **供应商偏好** | 优先特定供应商 | 稳定性优先 |
| **混合** | 综合考虑价格/负载/偏好 | 默认 |

---

## 6. 监控与告警

### 6.1 监控指标

#### 6.1.1 供应方监控

| 指标 | 说明 | 告警阈值 |
|------|------|----------|
| 账号可用率 | 账号正常比例 | < 99% |
| 成功率 | 请求成功率 | < 95% |
| 平均延迟 | 平均响应时间 | > 5000ms |
| 配额消耗速度 | 配额日消耗比例 | > 80%/天 |
| 异常请求比例 | 失败请求比例 | > 10% |
| 收益异常 | 收益突增/突降 | 偏离 > 50% |

#### 6.1.2 套餐监控

| 指标 | 说明 | 告警阈值 |
|------|------|----------|
| 剩余配额 | 可用配额 | < 10% |
| 订单量 | 新增订单 | 突降 > 30% |
| 评分 | 用户评分 | < 4.0 |
| 投诉 | 用户投诉 | > 3次/周 |

### 6.2 告警通知

| 告警类型 | 通知对象 | 通知方式 |
|----------|----------|----------|
| 账号异常 | 供应方 | 站内信/邮件 |
| 配额不足 | 供应方 | 站内信/短信 |
| 收益到账 | 供应方 | 站内信/邮件 |
| 异常订单 | 平台运营 | 邮件/短信 |
| ToS 违规 | 平台运营 | 邮件 |

---

## 7. 报表导出

### 7.1 供应方报表

| 报表 | 内容 | 格式 |
|------|------|------|
| 账单汇总 | 收入/订单/费用 | CSV/Excel |
| 使用明细 | 每笔调用详情 | CSV/Excel |
| 账户流水 | 收益/提现/冻结 | CSV/Excel |
| 对账单 | 平台与供应方对账 | PDF |

### 7.2 需求方报表

| 报表 | 内容 | 格式 |
|------|------|------|
| 消费账单 | 消费汇总 | CSV/Excel |
| 使用明细 | 调用明细 | CSV/Excel |
| 成本分析 | 按模型/部门分析 | PDF |

---

## 8. 数据统计SQL示例

### 8.1 供应方收入统计

```sql
SELECT
    sa.user_id,
    u.username,
    sa.platform,
    SUM(sur.total_cost) as total_revenue,
    SUM(sur.total_cost) * :platform_fee_rate as platform_fee,
    SUM(sur.total_cost) * :supplier_settlement_rate as supplier_earnings,
    COUNT(DISTINCT sur.order_id) as order_count,
    SUM(sur.total_tokens) as total_tokens,
    AVG(sur.success) as avg_success_rate
FROM supply_usage_records sur
JOIN supply_accounts sa ON sur.supply_account_id = sa.id
JOIN users u ON sa.user_id = u.id
WHERE sur.created_at >= '2026-03-01' AND sur.created_at < '2026-04-01'
GROUP BY sa.user_id, u.username, sa.platform
ORDER BY total_revenue DESC;
```

### 8.2 套餐消耗统计

```sql
SELECT
    sp.id as package_id,
    sp.model,
    sp.platform,
    sp.total_quota,
    sp.available_quota,
    sp.sold_quota,
    (sp.sold_quota / sp.total_quota * 100) as sold_rate,
    COUNT(DISTINCT so.id) as order_count,
    SUM(sur.total_cost) as total_revenue,
    AVG(sur.latency_ms) as avg_latency,
    AVG(sur.success) as success_rate
FROM supply_packages sp
LEFT JOIN supply_orders so ON sp.id = so.supply_package_id
LEFT JOIN supply_usage_records sur
       ON sur.supply_account_id = sp.supply_account_id
      AND sur.model = sp.model
WHERE sp.created_at >= '2026-03-01'
GROUP BY sp.id, sp.model, sp.platform, sp.total_quota, sp.available_quota, sp.sold_quota
ORDER BY total_revenue DESC;
```

---

**文档状态**：完整详细设计（已补 PostgreSQL 执行口径与 SQL 示例修正）
**关联文档**：
- `supply_side_product_design_v1_2026-03-18.md`
- `supply_feature_technical_analysis_v1_2026-03-18.md`
