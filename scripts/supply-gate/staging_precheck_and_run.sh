#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="${1:-${SCRIPT_DIR}/.env}"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
OUT_DIR="${ROOT_DIR}/reports/gates"
mkdir -p "${OUT_DIR}"
TS="$(date +%F_%H%M%S)"
LOG_FILE="${OUT_DIR}/staging_run_${TS}.log"

# shellcheck disable=SC1090
source "${ENV_FILE}"
ENABLE_TOK005_DRYRUN="${ENABLE_TOK005_DRYRUN:-1}"
ENABLE_M021_PRECHECK="${ENABLE_M021_PRECHECK:-1}"

required=(API_BASE_URL OWNER_BEARER_TOKEN VIEWER_BEARER_TOKEN ADMIN_BEARER_TOKEN)
for v in "${required[@]}"; do
  if [[ -z "${!v:-}" ]]; then
    echo "[FAIL] missing env var: ${v}"
    exit 1
  fi
done

for t in "${OWNER_BEARER_TOKEN}" "${VIEWER_BEARER_TOKEN}" "${ADMIN_BEARER_TOKEN}"; do
  if [[ "${t}" == replace-me-* ]]; then
    echo "[FAIL] placeholder token detected; please fill real short-lived token"
    exit 1
  fi
done

if [[ "${API_BASE_URL}" == *"staging.example.com"* ]]; then
  echo "[FAIL] placeholder API_BASE_URL detected: ${API_BASE_URL}"
  exit 1
fi

echo "[INFO] precheck pass, API_BASE_URL=${API_BASE_URL}" | tee "${LOG_FILE}"

if [[ "${ENABLE_M021_PRECHECK}" == "1" ]]; then
  echo "[INFO] run M-021 token runtime readiness precheck" | tee -a "${LOG_FILE}"
  bash "${ROOT_DIR}/scripts/ci/token_runtime_readiness_check.sh" "$(date +%F)" | tee -a "${LOG_FILE}"
else
  echo "[INFO] skip M-021 precheck by ENABLE_M021_PRECHECK=${ENABLE_M021_PRECHECK}" | tee -a "${LOG_FILE}"
fi

if [[ "${ENABLE_TOK005_DRYRUN}" == "1" ]]; then
  echo "[INFO] run TOK-005 dry-run gate first" | tee -a "${LOG_FILE}"
  bash "${SCRIPT_DIR}/tok005_boundary_dryrun.sh" "${ENV_FILE}" | tee -a "${LOG_FILE}"
else
  echo "[INFO] skip TOK-005 dry-run gate by ENABLE_TOK005_DRYRUN=${ENABLE_TOK005_DRYRUN}" | tee -a "${LOG_FILE}"
fi

if ! curl -sS -m 5 -I "${API_BASE_URL}" >/dev/null; then
  echo "[FAIL] API_BASE_URL unreachable: ${API_BASE_URL}" | tee -a "${LOG_FILE}"
  exit 1
fi

echo "[INFO] reachable, start SUP run_all" | tee -a "${LOG_FILE}"
{
  echo "== run_all begin =="
  bash "${SCRIPT_DIR}/run_all.sh" "${ENV_FILE}"
  echo "== run_all end =="
} | tee -a "${LOG_FILE}"

echo "[PASS] staging run complete: ${LOG_FILE}" | tee -a "${LOG_FILE}"
