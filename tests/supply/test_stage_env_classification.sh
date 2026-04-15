#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
OUT_DIR="${ROOT_DIR}/reports/archive/gate_verification"

latest_report() {
  ls -1t "${OUT_DIR}"/superpowers_stage_validation_*.md 2>/dev/null | head -n 1 || true
}

before_report="$(latest_report)"

cd "${ROOT_DIR}"
STAGING_ENV_FILE="scripts/supply-gate/.env.local-mock" \
  bash "scripts/ci/superpowers_stage_validate.sh" >/tmp/test_stage_env_classification.log 2>&1 || true

after_report="$(latest_report)"

if [[ -z "${after_report}" ]]; then
  echo "[FAIL] superpowers stage validation report not generated"
  exit 1
fi

if [[ -n "${before_report}" && "${before_report}" == "${after_report}" ]]; then
  echo "[FAIL] superpowers stage validation did not create a newer report"
  exit 1
fi

phase07_status="$(
  awk -F'|' '
    /^\| PHASE-07 / {
      value=$3
      gsub(/^ +| +$/, "", value)
      print value
      exit
    }
  ' "${after_report}"
)"

decision="$(
  sed -n 's/^- 决策：\*\*\([^*][^*]*\)\*\*$/\1/p' "${after_report}" | head -n 1
)"

if [[ "${phase07_status}" != "DEFERRED" && "${phase07_status}" != "MOCK" ]]; then
  echo "[FAIL] PHASE-07 expected DEFERRED/MOCK but got '${phase07_status:-N/A}'"
  echo "[INFO] report=${after_report}"
  exit 1
fi

if [[ "${decision}" != "CONDITIONAL_GO" ]]; then
  echo "[FAIL] expected CONDITIONAL_GO but got '${decision:-N/A}'"
  echo "[INFO] report=${after_report}"
  exit 1
fi

echo "[PASS] PHASE-07 is ${phase07_status} for local/mock env: ${after_report}"
