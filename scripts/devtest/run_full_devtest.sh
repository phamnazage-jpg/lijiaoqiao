#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STATE_DIR="${ROOT_DIR}/.tmp/devtest"
REPORT_DIR="${ROOT_DIR}/reports/devtest"
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
REPORT_FILE="${REPORT_DIR}/devtest_validation_${TIMESTAMP}.md"

mkdir -p "${REPORT_DIR}"

bash "${ROOT_DIR}/scripts/devtest/start_dev_stack.sh"

# shellcheck disable=SC1090
source "${STATE_DIR}/env.sh"

(
    cd "${ROOT_DIR}/supply-api"
    GOCACHE="${STATE_DIR}/go-cache/devtestctl-seed-supply" \
        go run ./cmd/devtestctl seed-supply \
        --dsn "${LIJIAOQIAO_DEVTEST_SUPPLY_DSN}"

    GOCACHE="${STATE_DIR}/go-cache/devtestctl-seed-token" \
        go run ./cmd/devtestctl seed-token-runtime \
        --dsn "${LIJIAOQIAO_DEVTEST_TOKEN_RUNTIME_DSN}"
)

set +e
(
    cd "${ROOT_DIR}/supply-api"
    GOCACHE="${STATE_DIR}/go-cache/devtestctl-smoke" \
        go run ./cmd/devtestctl smoke \
        --supply-base "http://127.0.0.1:18082" \
        --token-base "http://${LIJIAOQIAO_DEVTEST_TOKEN_RUNTIME_ADDR}" \
        --gateway-base "http://${LIJIAOQIAO_DEVTEST_GATEWAY_HOST}:${LIJIAOQIAO_DEVTEST_GATEWAY_PORT}" \
        --supply-dsn "${LIJIAOQIAO_DEVTEST_SUPPLY_DSN}" \
        --token-dsn "${LIJIAOQIAO_DEVTEST_TOKEN_RUNTIME_DSN}" \
        --supply-secret "${LIJIAOQIAO_DEVTEST_SUPPLY_TOKEN_SECRET_KEY}" \
        --supply-issuer "${LIJIAOQIAO_DEVTEST_SUPPLY_TOKEN_ISSUER}" \
        --report "${REPORT_FILE}"
)
SMOKE_EXIT_CODE=$?
set -e

echo "[devtest] report: ${REPORT_FILE}"
exit "${SMOKE_EXIT_CODE}"
