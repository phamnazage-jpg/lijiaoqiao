#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
OUT_DIR="${ROOT_DIR}/reports/archive/gate_verification"
TS="$(date +%F_%H%M%S)"
LOG_FILE="${OUT_DIR}/cross_service_smoke_${TS}.log"
REPORT_FILE="${OUT_DIR}/cross_service_smoke_${TS}.md"

SMOKE_GATEWAY_BASE_URL="${SMOKE_GATEWAY_BASE_URL:-http://127.0.0.1:18080}"
SMOKE_TOKEN_RUNTIME_BASE_URL="${SMOKE_TOKEN_RUNTIME_BASE_URL:-http://127.0.0.1:18081}"
SMOKE_SUPPLY_API_BASE_URL="${SMOKE_SUPPLY_API_BASE_URL:-http://127.0.0.1:18082}"
SMOKE_BEARER_TOKEN="${SMOKE_BEARER_TOKEN:-placeholder-token}"
SMOKE_EXPECTED_SCOPE="${SMOKE_EXPECTED_SCOPE:-supply:read}"
SMOKE_EXPECTED_MODEL="${SMOKE_EXPECTED_MODEL:-gpt-4o-mini}"
SMOKE_ALLOW_LOCAL_PLACEHOLDER="${SMOKE_ALLOW_LOCAL_PLACEHOLDER:-0}"

mkdir -p "${OUT_DIR}"
: > "${LOG_FILE}"

cat > "${REPORT_FILE}" <<EOF
# Cross-Service Smoke Design Report

- 时间戳：${TS}
- 状态：**DESIGN_ONLY**
- gateway：${SMOKE_GATEWAY_BASE_URL}
- token-runtime：${SMOKE_TOKEN_RUNTIME_BASE_URL}
- supply-api：${SMOKE_SUPPLY_API_BASE_URL}
- expected_scope：${SMOKE_EXPECTED_SCOPE}
- expected_model：${SMOKE_EXPECTED_MODEL}
- allow_local_placeholder：${SMOKE_ALLOW_LOCAL_PLACEHOLDER}

## Planned Chain

1. gateway health
2. token-runtime health
3. supply-api health
4. protected request through gateway with real bearer token
5. verify gateway -> token-runtime -> supply-api chain evidence

## Planned Result Contract

- \`PASS\`: real staging smoke passed
- \`SKIP_LOCAL_PLACEHOLDER\`: local/mock/placeholder inputs only
- \`FAIL_REAL_SMOKE\`: real inputs supplied but chain failed

## Note

This script is a Phase P2-D design stub. It defines input/output contracts and artifact paths,
but it must not be treated as completed release evidence yet.
EOF

{
  echo "[INFO] cross-service smoke design stub"
  echo "[INFO] report: ${REPORT_FILE}"
  echo "[INFO] log: ${LOG_FILE}"
  echo "[INFO] status: DESIGN_ONLY"
} | tee -a "${LOG_FILE}"

exit 2
