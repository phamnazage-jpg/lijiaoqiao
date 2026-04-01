#!/bin/bash
# Supply API Database Migration Script
# 执行顺序固定，按 XR-001 + database_domain_model_and_governance_v1_2026-03-27.md

set -e

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
MIGRATIONS=(
    "platform_core_schema_v1.sql"
    "supply_schema_v1.sql"
    "supply_schema_v1_patch_2026-03-27.sql"
    "supply_idempotency_record_v1.sql"
)

# 检查PGPASSWORD
if [ -z "$SUPPLY_DB_PASSWORD" ]; then
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

    PGPASSWORD="$SUPPLY_DB_PASSWORD" psql \
        -h "$DB_HOST" \
        -p "$DB_PORT" \
        -U "$DB_USER" \
        -d "$DB_NAME" \
        -f "$sql_file" \
        --set ON_ERROR_STOP=on

    if [ $? -eq 0 ]; then
        echo "SUCCESS: $migration"
    else
        echo "FAILED: $migration"
        exit 1
    fi
    echo ""
done

echo "============================================"
echo "All migrations completed successfully!"
echo "============================================"
