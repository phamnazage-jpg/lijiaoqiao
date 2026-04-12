#!/usr/bin/env bash
# Test: verify mock server is accessible and staging precheck works
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

echo "=== Mock Server Health Check ==="

# Check if mock server is running
if curl -sS -m 2 "http://127.0.0.1:18080/actuator/health" >/dev/null 2>&1; then
    echo "[PASS] Mock server is running on 127.0.0.1:18080"
else
    echo "[FAIL] Mock server is not running on 127.0.0.1:18080"
    exit 1
fi

echo ""
echo "=== .env Configuration Check ==="

# Load .env
source "${ROOT_DIR}/scripts/supply-gate/.env"

echo "API_BASE_URL from .env: ${API_BASE_URL}"

# Test connectivity to API_BASE_URL
if curl -sS -m 2 -I "${API_BASE_URL}" >/dev/null 2>&1; then
    echo "[PASS] Can connect to API_BASE_URL"
else
    echo "[FAIL] Cannot connect to API_BASE_URL: ${API_BASE_URL}"
    exit 1
fi

echo ""
echo "[PASS] All connectivity checks passed"