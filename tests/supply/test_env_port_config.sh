#!/usr/bin/env bash
# Test: verify .env port configuration matches mock server
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Expected mock server port
EXPECTED_PORT="18080"
# Expected mock server host
EXPECTED_HOST="127.0.0.1"

# Load .env file
ENV_FILE="${ROOT_DIR}/scripts/supply-gate/.env"
source "${ENV_FILE}"

# Parse API_BASE_URL
API_HOST="$(echo "${API_BASE_URL}" | sed -E 's|http://([^/:]+).*|\1|')"
API_PORT="$(echo "${API_BASE_URL}" | sed -E 's|http://[^/:]+:([0-9]+).*|\1|')"

echo "=== Port Configuration Test ==="
echo "Expected mock server: ${EXPECTED_HOST}:${EXPECTED_PORT}"
echo "Loaded from .env:    ${API_HOST}:${API_PORT}"

FAILED=0

if [[ "${API_HOST}" != "${EXPECTED_HOST}" ]]; then
    echo "[FAIL] API_HOST mismatch: expected ${EXPECTED_HOST}, got ${API_HOST}"
    FAILED=1
fi

if [[ "${API_PORT}" != "${EXPECTED_PORT}" ]]; then
    echo "[FAIL] API_PORT mismatch: expected ${EXPECTED_PORT}, got ${API_PORT}"
    FAILED=1
fi

if [[ ${FAILED} -eq 0 ]]; then
    echo "[PASS] Port configuration matches mock server"
else
    echo "[FAIL] Port configuration does NOT match mock server"
fi

# Test connectivity
echo ""
echo "=== Connectivity Test ==="
if curl -sS -m 2 -I "${API_BASE_URL}" >/dev/null 2>&1; then
    echo "[PASS] Can connect to ${API_BASE_URL}"
else
    echo "[FAIL] Cannot connect to ${API_BASE_URL}"
    FAILED=1
fi

exit ${FAILED}