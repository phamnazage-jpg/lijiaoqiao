#!/bin/bash
# Supply API Database Migration Script
# 只应用当前 fresh setup 需要的基线 DDL。

set -euo pipefail

DB_HOST="${SUPPLY_DB_HOST:-localhost}"
DB_PORT="${SUPPLY_DB_PORT:-5432}"
DB_USER="${SUPPLY_DB_USER:-postgres}"
DB_NAME="${SUPPLY_DB_NAME:-supply_db}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SQL_DIR="$(dirname "$SCRIPT_DIR")/sql/postgresql"

echo "============================================"
echo "Supply API Database Migration"
echo "============================================"
echo "Host: $DB_HOST:$DB_PORT"
echo "Database: $DB_NAME"
echo "User: $DB_USER"
echo ""

# 迁移顺序（固定）
# 与 scripts/run_integration_tests.sh 的 schema baseline 保持一致。
MIGRATIONS=(
    "supply_core_schema_v2.sql"
    "partition_strategy_v1.sql"
    "outbox_pattern_v1.sql"
    "token_status_registry_v1.sql"
    "audit_alerts_v1.sql"
)

# 检查PGPASSWORD
if [ -z "${SUPPLY_DB_PASSWORD:-}" ]; then
    echo "Warning: SUPPLY_DB_PASSWORD not set"
fi

# 执行迁移
for migration in "${MIGRATIONS[@]}"; do
    sql_file="$SQL_DIR/$migration"
    if [ ! -f "$sql_file" ]; then
        echo "ERROR: Migration file not found: $sql_file"
        exit 1
    fi

    echo "Executing: $migration"
    echo "--------------------------------------------"

    PGPASSWORD="${SUPPLY_DB_PASSWORD:-}" psql \
        -h "$DB_HOST" \
        -p "$DB_PORT" \
        -U "$DB_USER" \
        -d "$DB_NAME" \
        -f "$sql_file" \
        --set ON_ERROR_STOP=on

    echo "SUCCESS: $migration"
    echo ""
done

echo "============================================"
echo "All migrations completed successfully!"
echo "============================================"
