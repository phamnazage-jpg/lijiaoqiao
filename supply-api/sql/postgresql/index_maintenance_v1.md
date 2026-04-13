# 数据库索引维护策略 v1.0

> **文档版本**: v1.0
> **创建日期**: 2026-04-07
> **问题**: P1-009 高频写入表的索引维护策略未定义

---

## 1. 概述

本文档定义高频写入表的索引维护策略，包括 `REINDEX`、`VACUUM` 自动化方案，确保数据库性能稳定。

### 1.1 高频写入表清单

| 表名 | 写入频率 | 日均增量 | 备注 |
|------|----------|----------|------|
| supply_usage_records | 极高 | ~1000万条 | 核心业务表 |
| supply_idempotency_records | 高 | ~100万条 | 幂等检查 |
| audit_events | 高 | ~500万条 | 审计日志 |
| billing_ledger_entries | 中 | ~10万条 | 账务明细 |

---

## 2. VACUUM 维护策略

### 2.1 自动 VACUUM 配置

PostgreSQL 默认启用 autovacuum，但需要针对高频表进行调优：

```sql
-- supply_usage_records 表配置
ALTER TABLE supply_usage_records SET (
    autovacuum_vacuum_threshold = 50,
    autovacuum_analyze_threshold = 50,
    autovacuum_vacuum_scale_factor = 0.01,
    autovacuum_analyze_scale_factor = 0.01,
    autovacuum_vacuum_cost_delay = 2,
    autovacuum_vacuum_cost_limit = 200
);

-- supply_idempotency_records 表配置
ALTER TABLE supply_idempotency_records SET (
    autovacuum_vacuum_threshold = 100,
    autovacuum_analyze_threshold = 100,
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02
);
```

### 2.2 VACUUM 策略矩阵

| 表名 | autovacuum_enabled | vacuum_threshold | vacuum_scale_factor | 分析频率 |
|------|-------------------|------------------|---------------------|----------|
| supply_usage_records | true | 50 | 0.01 (1%) | 每1%变化 |
| supply_idempotency_records | true | 100 | 0.05 (5%) | 每2%变化 |
| supply_orders | true | 500 | 0.05 (5%) | 每周 |
| supply_packages | true | 1000 | 0.1 (10%) | 每月 |

### 2.3 手动 VACUUM 计划

**日常维护** (低峰期 02:00-04:00):
```bash
# vacuum analyze 高频表
vacuumdb -h localhost -U postgres -d supply_db \
    --table 'supply_usage_records' \
    --analyze \
    --verbose

# 批量 vacuum 多个表
vacuumdb -h localhost -U postgres -d supply_db \
    --all \
    --analyze \
    --verbose
```

**周维护** (周日 03:00-05:00):
```bash
# 全面 vacuum + analyze
vacuumdb -h localhost -U postgres -d supply_db \
    --all \
    --analyze \
    --full \
    --verbose
```

---

## 3. REINDEX 维护策略

### 3.1 REINDEX 触发条件

| 触发条件 | 说明 | 影响 |
|----------|------|------|
| 索引膨胀率 > 20% | B-tree 索引膨胀 | 性能下降 |
| 大量删除后 | DELETE > 30% 总行数 | 索引包含大量空页 |
| 长时间运行后 | 运行 > 30天 | 索引统计信息陈旧 |
| 硬件故障后 | 系统重启 | 确保索引一致性 |

### 3.2 索引膨胀检测

```sql
-- 检测索引膨胀率
SELECT
    schemaname,
    tablename,
    indexname,
    pg_size_pretty(pg_relation_size(indexrelid)) AS index_size,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch,
    ROUND(
        (pg_relation_size(indexrelid)::numeric /
         pg_relation_size(indrelid) * 100),
        2
    ) AS index_ratio
FROM
    pg_stat_user_indexes
WHERE
    pg_relation_size(indexrelid) > 1024 * 1024  -- > 1MB
ORDER BY
    pg_relation_size(indexrelid) DESC;
```

### 3.3 REINDEX 执行计划

**月维护** (每月第一个周日 04:00-06:00):
```bash
# 重建单个膨胀索引
reindexdb -h localhost -U postgres -d supply_db \
    --index 'idx_supply_usage_records_request_id' \
    --verbose

# 重建表的所有索引
reindexdb -h localhost -U postgres -d supply_db \
    --table 'supply_usage_records' \
    --verbose

# 全库索引重建 (慎用，会锁表)
reindexdb -h localhost -U postgres -d supply_db \
    --all \
    --verbose
```

### 3.4 联机型 REINDEX 方案

对于不可停机的关键表，使用 `REINDEX CONCURRENTLY`:

```bash
# 联机重建索引 (不锁表)
reindexdb -h localhost -U postgres -d supply_db \
    --index 'idx_supply_usage_records_request_id' \
    --concurrently \
    --verbose
```

---

## 4. 自动化脚本

### 4.1 每日维护脚本 (daily_vacuum.sh)

```bash
#!/bin/bash
# daily_vacuum.sh - 每日索引维护
# 执行时间: 每日 02:00

set -e

DB_HOST="localhost"
DB_PORT="5432"
DB_NAME="supply_db"
DB_USER="postgres"
LOG_FILE="/var/log/postgresql/daily_vacuum_$(date +%Y%m%d).log"

echo "=== 开始每日 VACUUM 维护: $(date) ===" | tee -a "$LOG_FILE"

# 高频表优先 vacuum
TABLES=(
    "supply_usage_records"
    "supply_idempotency_records"
    "supply_orders"
    "supply_earnings"
)

for TABLE in "${TABLES[@]}"; do
    echo "VACUUM $TABLE ..." | tee -a "$LOG_FILE"
    vacuumdb -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
        --table "$TABLE" \
        --analyze \
        --verbose 2>&1 | tee -a "$LOG_FILE"
done

echo "=== VACUUM 维护完成: $(date) ===" | tee -a "$LOG_FILE"
```

### 4.2 每周维护脚本 (weekly_reindex.sh)

```bash
#!/bin/bash
# weekly_reindex.sh - 每周 REINDEX 维护
# 执行时间: 每周日 03:00

set -e

DB_HOST="localhost"
DB_PORT="5432"
DB_NAME="supply_db"
DB_USER="postgres"
LOG_FILE="/var/log/postgresql/weekly_reindex_$(date +%Y%m%d).log"

echo "=== 开始每周 REINDEX 维护: $(date) ===" | tee -a "$LOG_FILE"

# 检查并重建膨胀索引
膨胀索引=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
SELECT indexname FROM pg_stat_user_indexes
WHERE pg_relation_size(indexrelid) > 10 * 1024 * 1024
AND idx_scan = 0
AND schemaname = 'public';
")

for INDEX in $膨胀索引; do
    echo "REINDEX INDEX $INDEX ..." | tee -a "$LOG_FILE"
    reindexdb -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
        --index "$INDEX" \
        --concurrently \
        --verbose 2>&1 | tee -a "$LOG_FILE"
done

echo "=== REINDEX 维护完成: $(date) ===" | tee -a "$LOG_FILE"
```

### 4.3 Cron 任务配置

```bash
# /etc/cron.d/postgresql_maintenance
# 每日凌晨2点执行 vacuum
0 2 * * * postgres /home/postgres/scripts/daily_vacuum.sh

# 每周日凌晨3点执行 reindex
0 3 * * 0 postgres /home/postgres/scripts/weekly_reindex.sh
```

---

## 5. 监控指标

### 5.1 关键监控指标

| 指标 | 告警阈值 | 说明 |
|------|----------|------|
| index膨胀率 | > 20% | 触发 REINDEX |
| dead_tuples | > 10000 | 触发 VACUUM |
| last_autovacuum | > 24h | 可能 autovacuum 异常 |
| idx_scan | = 0 | 索引未使用，考虑删除 |

### 5.2 监控查询

```sql
-- 检测需要维护的表
SELECT
    schemaname,
    relname AS table_name,
    n_dead_tup,
    n_live_tup,
    last_autovacuum,
    last_autoanalyze
FROM
    pg_stat_user_tables
WHERE
    n_dead_tup > 1000
ORDER BY
    n_dead_tup DESC;

-- 检测未使用的索引
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan
FROM
    pg_stat_user_indexes
WHERE
    idx_scan = 0
    AND NOT indexname LIKE '%_pkey'
ORDER BY
    pg_relation_size(indexrelid) DESC;
```

---

## 6. 最佳实践

1. **避免在高峰期维护**: 维护操作安排在低峰期 (02:00-06:00)
2. **优先自动 vacuum**: 配置合理的 autovacuum 参数，减少手动干预
3. **监控索引膨胀**: 定期检测膨胀率，及时重建
4. **使用 CONCURRENTLY**: 关键表使用 `REINDEX CONCURRENTLY` 避免锁表
5. **保留维护日志**: 记录每次维护执行情况，便于分析问题

---

## 7. 恢复时间预估

| 操作 | 表大小 | 预计耗时 | 锁类型 |
|------|--------|----------|--------|
| VACUUM ANALYZE | 10GB | 5-10min | 轻量锁 |
| REINDEX | 1GB | 1-2min | 表锁* |
| REINDEX CONCURRENTLY | 1GB | 3-5min | 无锁 |
| VACUUM FULL | 10GB | 15-30min | 表锁 |

*使用 `REINDEX CONCURRENTLY` 可避免锁表

---

> **维护记录**:
> - v1.0 (2026-04-07): 初始版本
