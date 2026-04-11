#!/bin/bash
# Supply API Integration Test Runner
# Usage: ./scripts/run_integration_tests.sh [package]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
DEPLOY_DIR="$PROJECT_DIR/deploy"

# Default package to test
PACKAGE="${1:-./internal/middleware/...}"

echo "=== Supply API Integration Tests ==="
echo ""

# Resolve compose command
if command -v docker-compose &> /dev/null; then
    COMPOSE_CMD=(docker-compose)
elif command -v docker &> /dev/null && docker compose version &> /dev/null; then
    COMPOSE_CMD=(docker compose)
else
    echo "ERROR: neither docker-compose nor docker compose is available"
    exit 1
fi

# Start infrastructure
echo "Starting test infrastructure..."
"${COMPOSE_CMD[@]}" -f "$DEPLOY_DIR/docker-compose.yml" up -d

# Wait for services to be healthy
echo "Waiting for PostgreSQL to be ready..."
for i in {1..30}; do
    if "${COMPOSE_CMD[@]}" -f "$DEPLOY_DIR/docker-compose.yml" exec -T postgres pg_isready -U supply_test &> /dev/null; then
        echo "PostgreSQL is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "ERROR: PostgreSQL failed to start"
        "${COMPOSE_CMD[@]}" -f "$DEPLOY_DIR/docker-compose.yml" logs postgres
        exit 1
    fi
    sleep 1
done

echo "Waiting for Redis to be ready..."
for i in {1..30}; do
    if "${COMPOSE_CMD[@]}" -f "$DEPLOY_DIR/docker-compose.yml" exec -T redis redis-cli ping &> /dev/null; then
        echo "Redis is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "ERROR: Redis failed to start"
        "${COMPOSE_CMD[@]}" -f "$DEPLOY_DIR/docker-compose.yml" logs redis
        exit 1
    fi
    sleep 1
done

echo ""
echo "Running integration tests for: $PACKAGE"
echo ""

# Run tests with integration tag
cd "$PROJECT_DIR"
export SUPPLY_API_DB_HOST="localhost"
export SUPPLY_API_DB_PORT="5432"
export SUPPLY_API_DB_USER="supply_test"
export SUPPLY_API_DB_PASSWORD="supply_test_pass"
export SUPPLY_API_DB_NAME="supply_test"
export SUPPLY_TEST_POSTGRES="postgres://supply_test:supply_test_pass@localhost:5432/supply_test?sslmode=disable"
export SUPPLY_TEST_REDIS="localhost:6379"

go test -tags=integration -v "$PACKAGE"

TEST_RESULT=$?

echo ""
echo "Stopping test infrastructure..."
"${COMPOSE_CMD[@]}" -f "$DEPLOY_DIR/docker-compose.yml" down -v

exit $TEST_RESULT
