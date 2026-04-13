# 性能测试基线 v1.0

> **文档版本**: v1.0
> **创建日期**: 2026-04-07
> **问题**: P1-013 SLO定义了P95延迟目标，但未定义性能测试基线和测试数据规模

---

## 1. 性能测试目标

### 1.1 响应时间目标 (SLO)

| API类别 | P50 | P95 | P99 | SLO |
|---------|-----|-----|-----|-----|
| 同步API (读取) | < 50ms | < 200ms | < 500ms | 99% |
| 同步API (写入) | < 100ms | < 300ms | < 800ms | 99% |
| 异步API | < 500ms | < 1s | < 2s | 99% |
| 认证Token验证 | < 10ms | < 30ms | < 100ms | 99.9% |
| 健康检查 | < 5ms | < 10ms | < 20ms | 99.99% |

### 1.2 吞吐量目标

| 场景 | 并发数 | RPS目标 | 说明 |
|------|--------|---------|------|
| 账户查询 | 100 | 1000 | 峰值5倍 |
| 套餐列表 | 100 | 500 | 分页场景 |
| 下单创建 | 50 | 200 | 事务性操作 |
| Token验证 | 200 | 2000 | 高频调用 |
| 使用记录写入 | 100 | 1000 | 日志级别写入 |

### 1.3 资源利用率目标

| 资源 | 正常负载 | 峰值负载 | 告警阈值 |
|------|----------|----------|----------|
| CPU | < 50% | < 80% | > 80% |
| 内存 | < 60% | < 80% | > 85% |
| 数据库连接 | < 50% | < 70% | > 80% |
| Redis连接 | < 40% | < 60% | > 70% |
| 网络带宽 | < 30% | < 50% | > 70% |

---

## 2. 测试场景定义

### 2.1 基准测试场景 (Baseline Tests)

| 场景ID | 场景名称 | 描述 | 权重 |
|--------|----------|------|------|
| BL-01 | HealthCheck | /health 端点 | 10% |
| BL-02 | 账户列表查询 | GET /api/v1/accounts | 20% |
| BL-03 | 套餐详情查询 | GET /api/v1/packages/:id | 20% |
| BL-04 | Token验证 | POST /api/v1/auth/validate | 30% |
| BL-05 | 使用记录写入 | POST /api/v1/usage | 20% |

### 2.2 压力测试场景 (Stress Tests)

| 场景ID | 场景名称 | 目标 | 递增 |
|--------|----------|------|------|
| ST-01 | 线性压力 | RPS从100递增至1000 | +100/30s |
| ST-02 | 突发压力 | 50% 基础 + 200% 峰值 | 脉冲模式 |
| ST-03 | 长期压力 | 70% 峰值持续 4h | 稳定 |

### 2.3 容量测试场景 (Capacity Tests)

| 场景ID | 场景名称 | 目标 | 终止条件 |
|--------|----------|------|----------|
| CT-01 | 最大并发 | 找到最大支持并发 | P99 > 1s |
| CT-02 | 最大RPS | 找到最大支持RPS | 错误率 > 1% |
| CT-03 | 数据量增长 | 验证随数据量增长的性能 | P95 > 基线2倍 |

### 2.4 峰值测试场景 (Peak Tests)

| 场景ID | 场景名称 | 模拟 | 持续时间 |
|--------|----------|------|----------|
| PK-01 | 工作日峰值 | 9:00-12:00流量 | 3h |
| PK-02 | 活动峰值 | 限时促销活动 | 1h |
| PK-03 | 月底峰值 | 账单生成高峰 | 4h |

---

## 3. 测试数据规模

### 3.1 数据规模定义

| 级别 | 账户数 | 套餐数 | 订单数 | 使用记录 | 用途 |
|------|--------|--------|--------|----------|------|
| Small | 1,000 | 5,000 | 10,000 | 100,000 | 本地开发 |
| Medium | 10,000 | 50,000 | 100,000 | 1,000,000 | 集成测试 |
| Large | 100,000 | 500,000 | 1,000,000 | 10,000,000 | 性能测试 |
| Production | 1,000,000 | 5,000,000 | 10,000,000 | 100,000,000 | 容量测试 |

### 3.2 测试数据生成策略

```sql
-- 生成Large级别测试数据
-- 执行时间: ~30分钟

-- 1. 生成用户 (100,000)
INSERT INTO iam_users (username, email, role, created_at)
SELECT
    'user_' || generate_series,
    'user_' || generate_series || '@test.com',
    (ARRAY['admin', 'operator', 'viewer'])[floor(random() * 3 + 1)],
    NOW() - interval '365 days' * random()
FROM generate_series(1, 100000);

-- 2. 生成供应账户 (500,000, 每个用户5个)
INSERT INTO supply_accounts (user_id, platform, status, created_at)
SELECT
    (random() * 99999 + 1)::bigint,
    (ARRAY['openai', 'anthropic', 'azure', 'google'])[floor(random() * 4 + 1)],
    (ARRAY['active', 'pending', 'suspended'])[floor(random() * 3 + 1)],
    NOW() - interval '180 days' * random()
FROM generate_series(1, 500000);

-- 3. 生成套餐 (500,000)
INSERT INTO supply_packages (
    supply_account_id, user_id, platform, model,
    total_quota, available_quota, status, created_at
)
SELECT
    generate_series,
    (random() * 99999 + 1)::bigint,
    (ARRAY['openai', 'anthropic', 'azure'])[floor(random() * 3 + 1)],
    (ARRAY['gpt-4', 'gpt-3.5', 'claude-3', 'claude-2'])[floor(random() * 4 + 1)],
    (random() * 1000000)::bigint + 100000,
    (random() * 500000)::bigint + 100000,
    'active',
    NOW() - interval '90 days' * random()
FROM generate_series(1, 500000);

-- 4. 创建索引
CREATE INDEX CONCURRENTLY idx_test_accounts_user ON supply_accounts(user_id);
CREATE INDEX CONCURRENTLY idx_test_packages_account ON supply_packages(supply_account_id);
```

### 3.3 数据刷新策略

```bash
#!/bin/bash
# refresh_test_data.sh - 刷新测试数据
# 每周执行一次，保持数据新鲜度

set -e

psql -h localhost -U postgres -d supply_test <<-EOSQL
    -- 更新订单时间分布
    UPDATE supply_orders
    SET created_at = NOW() - (random() * interval '30 days')
    WHERE created_at < NOW() - interval '30 days';

    -- 更新使用记录时间分布
    UPDATE supply_usage_records
    SET started_at = NOW() - (random() * interval '7 days')
    WHERE started_at < NOW() - interval '7 days';

    -- 重新生成部分数据
    DELETE FROM supply_usage_records WHERE id > 1000000;
    \i generate_usage_records.sql
EOSQL
```

---

## 4. 性能测试工具

### 4.1 工具选型

| 工具 | 用途 | 优势 | 劣势 |
|------|------|------|------|
| k6 | 基准测试、压力测试 | 脚本简单，输出丰富 | 分布式能力弱 |
| wrk | 基准测试 | 性能高，Lua脚本 | 无分布式 |
| locust | 复杂场景 | Python脚本，分布式 | 学习曲线 |
| Artillery | API测试 | YAML配置，云集成 | 并发有限 |
| Vegeta | 恒定RPS测试 | Go实现，高性能 | 脚本能力弱 |

### 4.2 k6 测试脚本示例

```javascript
// baseline_test.js - 基准测试脚本
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// 自定义指标
const errorRate = new Rate('errors');
const latency = new Trend('latency');

export const options = {
    stages: [
        { duration: '2m', target: 100 },   // 预热
        { duration: '5m', target: 100 },   // 基准负载
        { duration: '2m', target: 0 },     // 冷却
    ],
    thresholds: {
        'http_req_duration': ['p(95)<500'],
        'errors': ['rate<0.01'],
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const TOKEN = __ENV.TOKEN || 'test-token';

export default function() {
    // Token验证
    const validateRes = http.post(
        `${BASE_URL}/api/v1/auth/validate`,
        JSON.stringify({ token: TOKEN }),
        { headers: { 'Content-Type': 'application/json' } }
    );
    latency.add(validateRes.timings.duration);
    check(validateRes, {
        'validate status 200': (r) => r.status === 200,
        'validate latency < 30ms': (r) => r.timings.duration < 30,
    }) || errorRate.add(1);

    // 账户查询
    const accountsRes = http.get(
        `${BASE_URL}/api/v1/accounts`,
        { headers: { 'Authorization': `Bearer ${TOKEN}` } }
    );
    latency.add(accountsRes.timings.duration);
    check(accountsRes, {
        'accounts status 200': (r) => r.status === 200,
        'accounts latency < 200ms': (r) => r.timings.duration < 200,
    }) || errorRate.add(1);

    sleep(1);
}
```

---

## 5. 性能基线报告

### 5.1 基线报告模板

```markdown
# 性能测试报告 - {日期}

## 测试环境
- CPU: Intel Xeon 2.4GHz x 8
- 内存: 16GB DDR4
- 数据库: PostgreSQL 15 (4核8GB)
- Redis: 7.0 (2核4GB)

## 测试配置
- 持续时间: 5分钟
- 并发用户: 100
- 总请求数: 30,000

## 结果摘要

| 指标 | 目标 | 实际 | 状态 |
|------|------|------|------|
| P95延迟 | < 200ms | 156ms | ✅ 通过 |
| P99延迟 | < 500ms | 423ms | ✅ 通过 |
| 错误率 | < 1% | 0.02% | ✅ 通过 |
| RPS | > 100 | 128 | ✅ 通过 |

## 详细指标

### 响应时间分布
- P50: 45ms
- P90: 120ms
- P95: 156ms
- P99: 423ms
- Max: 1.2s

### 吞吐量
- 平均RPS: 128
- 峰值RPS: 156

### 错误分析
- 总错误: 6
- 超时错误: 3
- 服务端错误: 3
```

### 5.2 性能回归检测

```bash
#!/bin/bash
# compare_baseline.sh - 基线对比

CURRENT=$(cat perf_report_latest.json)
BASELINE=$(cat perf_report_baseline.json)

# 比较P95延迟
CURRENT_P95=$(echo "$CURRENT" | jq '.latency.p95')
BASELINE_P95=$(echo "$BASELINE" | jq '.latency.p95')

REGRESSION=$(echo "$CURRENT_P95 > $BASELINE_P95 * 1.1" | bc)

if [ "$REGRESSION" = "1" ]; then
    echo "⚠️ 警告: P95延迟回归检测到"
    echo "基线: ${BASELINE_P95}ms"
    echo "当前: ${CURRENT_P95}ms"
    exit 1
fi

echo "✅ 性能无回归"
```

---

## 6. 性能测试执行计划

### 6.1 执行频率

| 测试类型 | 频率 | 触发条件 |
|----------|------|----------|
| 基准测试 | 每日 | 代码提交后自动执行 |
| 压力测试 | 每周 | 手动触发 |
| 容量测试 | 每月 | 发布前执行 |
| 峰值测试 | 每季度 | 重大活动前 |

### 6.2 性能测试流程

```
代码提交
    │
    ▼
┌─────────────┐
│ 自动化构建  │──失败──► 返回修改
└─────────────┘
    │成功
    ▼
┌─────────────┐
│ 基准测试    │──失败──► 创建Bug
└─────────────┘
    │通过
    ▼
┌─────────────┐
│ 代码审查    │
└─────────────┘
    │通过
    ▼
┌─────────────┐
│ 集成测试    │
└─────────────┘
    │通过
    ▼
    合并
```

---

## 7. 性能问题诊断

### 7.1 常见性能问题

| 症状 | 可能原因 | 诊断方法 |
|------|----------|----------|
| P99延迟高 | 数据库索引缺失 | EXPLAIN ANALYZE |
| RPS低 | 线程池配置不当 | jstack分析 |
| 内存增长 | 内存泄漏 | heap profile |
| 连接池耗尽 | 连接泄漏 | 连接数监控 |

### 7.2 诊断工具

```bash
# 数据库慢查询
psql -c "SELECT query, calls, mean_time FROM pg_stat_statements ORDER BY mean_time DESC LIMIT 10;"

# Redis命令统计
redis-cli INFO commandstats | grep -E "cmdstat_get|cmdstat_set"

# Go pprof
go tool pprof http://localhost:6060/debug/pprof/heap
```

---

## 8. 性能优化建议

### 8.1 优化优先级

1. **P0 (立即优化)**: P99 > SLO目标
2. **P1 (本周优化)**: P95 > SLO目标 150%
3. **P2 (本月优化)**: RPS < 目标 70%

### 8.2 常见优化手段

| 问题 | 优化方案 |
|------|----------|
| 数据库查询慢 | 添加索引、优化SQL |
| 序列化开销 | 使用更快的序列化库 |
| GC压力大 | 对象池、减少分配 |
| 连接池耗尽 | 增加连接数、优化使用 |

---

> **维护记录**:
> - v1.0 (2026-04-07): 初始版本
