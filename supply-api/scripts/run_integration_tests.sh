#!/bin/bash
# Supply API Integration Test Runner
# Usage: ./scripts/run_integration_tests.sh [package]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
DEPLOY_DIR="$PROJECT_DIR/deploy"
PACKAGE="${1:-./internal/repository}"

MODE=""
COMPOSE_CMD=()
LOCAL_ROOT=""
LOCAL_PG_BIN=""
LOCAL_PG_DATA=""
LOCAL_PG_SOCKET=""
LOCAL_PG_LOG=""
LOCAL_PG_PORT="15432"
LOCAL_PG_HOST="127.0.0.1"

SCHEMA_FILES=(
    "sql/postgresql/supply_core_schema_v2.sql"
    "sql/postgresql/partition_strategy_v1.sql"
    "sql/postgresql/outbox_pattern_v1.sql"
    "sql/postgresql/token_status_registry_v1.sql"
    "sql/postgresql/audit_alerts_v1.sql"
)

echo "=== Supply API Integration Tests ==="
echo ""

cleanup() {
    echo ""
    echo "Stopping test infrastructure..."
    case "$MODE" in
        docker)
            "${COMPOSE_CMD[@]}" -f "$DEPLOY_DIR/docker-compose.yml" down -v >/dev/null 2>&1 || true
            ;;
        local-postgres)
            if [ -n "$LOCAL_PG_BIN" ] && [ -n "$LOCAL_PG_DATA" ] && [ -d "$LOCAL_PG_DATA" ]; then
                "$LOCAL_PG_BIN/pg_ctl" -D "$LOCAL_PG_DATA" stop -m fast >/dev/null 2>&1 || true
            fi
            if [ -n "$LOCAL_ROOT" ] && [ -d "$LOCAL_ROOT" ]; then
                rm -rf "$LOCAL_ROOT"
            fi
            ;;
    esac
}

trap cleanup EXIT

resolve_compose() {
    if command -v docker-compose >/dev/null 2>&1; then
        COMPOSE_CMD=(docker-compose)
        return 0
    fi
    if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
        COMPOSE_CMD=(docker compose)
        return 0
    fi
    return 1
}

find_pg_bin() {
    local dir
    for dir in /usr/lib/postgresql/*/bin; do
        if [ -x "$dir/initdb" ] && [ -x "$dir/pg_ctl" ] && [ -x "$dir/psql" ]; then
            echo "$dir"
            return 0
        fi
    done
    return 1
}

start_docker_infra() {
    if ! resolve_compose; then
        return 1
    fi

    echo "Starting test infrastructure with docker..."
    if ! "${COMPOSE_CMD[@]}" -f "$DEPLOY_DIR/docker-compose.yml" up -d >/dev/null 2>&1; then
        return 1
    fi

    MODE="docker"

    echo "Waiting for PostgreSQL to be ready..."
    for i in {1..30}; do
        if "${COMPOSE_CMD[@]}" -f "$DEPLOY_DIR/docker-compose.yml" exec -T postgres pg_isready -U supply_test >/dev/null 2>&1; then
            echo "PostgreSQL is ready"
            break
        fi
        if [ "$i" -eq 30 ]; then
            echo "ERROR: PostgreSQL failed to start"
            "${COMPOSE_CMD[@]}" -f "$DEPLOY_DIR/docker-compose.yml" logs postgres || true
            return 1
        fi
        sleep 1
    done

    echo "Waiting for Redis to be ready..."
    for i in {1..30}; do
        if "${COMPOSE_CMD[@]}" -f "$DEPLOY_DIR/docker-compose.yml" exec -T redis redis-cli ping >/dev/null 2>&1; then
            echo "Redis is ready"
            break
        fi
        if [ "$i" -eq 30 ]; then
            echo "ERROR: Redis failed to start"
            "${COMPOSE_CMD[@]}" -f "$DEPLOY_DIR/docker-compose.yml" logs redis || true
            return 1
        fi
        sleep 1
    done

    return 0
}

start_local_postgres() {
    LOCAL_PG_BIN="$(find_pg_bin)" || {
        echo "ERROR: docker 不可用，且未找到本机 PostgreSQL 二进制"
        return 1
    }

    MODE="local-postgres"
    LOCAL_ROOT="$(mktemp -d /tmp/lijiaoqiao-supply-integration.XXXXXX)"
    LOCAL_PG_DATA="$LOCAL_ROOT/postgres-data"
    LOCAL_PG_SOCKET="$LOCAL_ROOT/postgres-socket"
    LOCAL_PG_LOG="$LOCAL_ROOT/postgres.log"

    mkdir -p "$LOCAL_PG_SOCKET"

    echo "Starting local PostgreSQL test infrastructure..."
    "$LOCAL_PG_BIN/initdb" -D "$LOCAL_PG_DATA" -U supply_test --auth=trust >/dev/null
    "$LOCAL_PG_BIN/pg_ctl" -D "$LOCAL_PG_DATA" -l "$LOCAL_PG_LOG" -o "-k $LOCAL_PG_SOCKET -h $LOCAL_PG_HOST -p $LOCAL_PG_PORT" start >/dev/null

    for i in {1..30}; do
        if "$LOCAL_PG_BIN/pg_isready" -h "$LOCAL_PG_HOST" -p "$LOCAL_PG_PORT" -U supply_test >/dev/null 2>&1; then
            echo "PostgreSQL is ready"
            break
        fi
        if [ "$i" -eq 30 ]; then
            echo "ERROR: local PostgreSQL failed to start"
            cat "$LOCAL_PG_LOG" || true
            return 1
        fi
        sleep 1
    done

    "$LOCAL_PG_BIN/createdb" -h "$LOCAL_PG_HOST" -p "$LOCAL_PG_PORT" -U supply_test supply_test
    echo "Redis is not required for package $PACKAGE"
    return 0
}

apply_schema() {
    echo ""
    echo "Applying schema baseline..."
    local schema
    for schema in "${SCHEMA_FILES[@]}"; do
        echo "  - $(basename "$schema")"
        case "$MODE" in
            docker)
                "${COMPOSE_CMD[@]}" -f "$DEPLOY_DIR/docker-compose.yml" exec -T postgres \
                    psql -v ON_ERROR_STOP=1 -U supply_test -d supply_test \
                    -f "/docker-entrypoint-initdb.d/$(basename "$schema")" >/dev/null
                ;;
            local-postgres)
                "$LOCAL_PG_BIN/psql" -v ON_ERROR_STOP=1 -h "$LOCAL_PG_HOST" -p "$LOCAL_PG_PORT" -U supply_test -d supply_test \
                    -f "$PROJECT_DIR/$schema" >/dev/null
                ;;
            *)
                echo "ERROR: unknown infrastructure mode"
                return 1
                ;;
        esac
    done
}

export_test_env() {
    case "$MODE" in
        docker)
            export SUPPLY_API_DB_HOST="localhost"
            export SUPPLY_API_DB_PORT="5432"
            export SUPPLY_TEST_REDIS="localhost:6379"
            ;;
        local-postgres)
            export SUPPLY_API_DB_HOST="$LOCAL_PG_HOST"
            export SUPPLY_API_DB_PORT="$LOCAL_PG_PORT"
            export SUPPLY_TEST_REDIS=""
            ;;
        *)
            echo "ERROR: unknown infrastructure mode"
            return 1
            ;;
    esac

    export SUPPLY_API_DB_USER="supply_test"
    export SUPPLY_API_DB_PASSWORD=""
    export SUPPLY_API_DB_NAME="supply_test"
    export GOCACHE="${GOCACHE:-/tmp/lijiaoqiao-go-cache-supply-integration}"
}

if ! start_docker_infra; then
    echo "Docker infrastructure unavailable, falling back to local PostgreSQL binaries..."
    start_local_postgres
fi

apply_schema

echo ""
echo "Running integration tests for: $PACKAGE"
echo ""

cd "$PROJECT_DIR"
export_test_env
go test -count=1 -tags=integration -v "$PACKAGE"
