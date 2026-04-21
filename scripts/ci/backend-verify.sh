#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
OUT_DIR="${ROOT_DIR}/reports/archive/gate_verification"
TS="$(date +%F_%H%M%S)"
LOG_FILE="${OUT_DIR}/backend_verify_${TS}.log"
REPORT_FILE="${OUT_DIR}/backend_verify_${TS}.md"
LIB_FILE="${ROOT_DIR}/scripts/ci/lib/verification_common.sh"
CONTRACT_GATE_DOC="${ROOT_DIR}/tests/contract/gateway_token_runtime_supply_chain.md"
CONTRACT_GATE_CHECKLIST="${ROOT_DIR}/docs/plans/2026-04-21-phase1-contract-gate-checklist.md"
CONTRACT_GATE_LOG="${OUT_DIR}/contract_gate_${TS}.log"
CONTRACT_GATE_REPORT="${OUT_DIR}/contract_gate_${TS}.md"
# shellcheck disable=SC1091
source "${LIB_FILE}"

mkdir -p "${OUT_DIR}"
: > "${LOG_FILE}"

GO_BIN="$(resolve_go_bin "${ROOT_DIR}" || true)"
if [[ -z "${GO_BIN}" ]]; then
  echo "[FAIL] go binary not found" | tee -a "${LOG_FILE}"
  exit 1
fi

setup_go_env "${GO_BIN}" "${ROOT_DIR}/.tools/go-cache"

STEP_RESULTS=()

log() {
  echo "$1" | tee -a "${LOG_FILE}"
}

run_step() {
  local step_id="$1"
  local title="$2"
  local cmd="$3"
  local out_file="${OUT_DIR}/${step_id,,}_${TS}.out.log"

  log "[INFO] ${step_id} ${title} start"
  set +e
  bash -lc "${cmd}" > "${out_file}" 2>&1
  local rc=$?
  set -e

  if [[ "${rc}" -eq 0 ]]; then
    log "[PASS] ${step_id} rc=${rc}"
    write_step_result STEP_RESULTS "${step_id}" "PASS" "${title}" "${out_file}"
  else
    log "[FAIL] ${step_id} rc=${rc}"
    write_step_result STEP_RESULTS "${step_id}" "FAIL" "${title}" "${out_file}"
  fi
}

run_e2e_skip_gate() {
  local step_id="$1"
  local title="$2"
  local out_file="${OUT_DIR}/${step_id,,}_${TS}.out.log"

  log "[INFO] ${step_id} ${title} start"
  set +e
  bash -lc "cd \"${ROOT_DIR}/supply-api\" && \"${GO_BIN}\" test -tags=e2e -v ./e2e/..." > "${out_file}" 2>&1
  local rc=$?
  set -e

  if grep -Eiq 'SKIP|需要完整环境运行 E2E 测试|Skipping E2E test' "${out_file}"; then
    log "[FAIL] ${step_id} placeholder E2E detected"
    write_step_result STEP_RESULTS "${step_id}" "FAIL" "${title}" "${out_file}"
    return
  fi

  if [[ "${rc}" -eq 0 ]]; then
    log "[PASS] ${step_id} rc=${rc}"
    write_step_result STEP_RESULTS "${step_id}" "PASS" "${title}" "${out_file}"
  else
    log "[FAIL] ${step_id} rc=${rc}"
    write_step_result STEP_RESULTS "${step_id}" "FAIL" "${title}" "${out_file}"
  fi
}

run_step \
  "STEP-01" \
  "supply-api critical regression suite" \
  "cd \"${ROOT_DIR}/supply-api\" && \"${GO_BIN}\" test ./cmd/supply-api ./internal/config ./internal/httpapi ./internal/middleware ./internal/outbox ./internal/repository"

run_step \
  "STEP-02" \
  "gateway critical regression suite" \
  "cd \"${ROOT_DIR}/gateway\" && \"${GO_BIN}\" test ./cmd/gateway ./internal/config ./internal/middleware"

run_step \
  "STEP-03" \
  "platform-token-runtime critical regression suite" \
  "cd \"${ROOT_DIR}/platform-token-runtime\" && \"${GO_BIN}\" test ./cmd/platform-token-runtime ./internal/httpapi ./internal/token ./internal/auth/..."

run_e2e_skip_gate \
  "STEP-04" \
  "supply-api E2E gate must not contain placeholder skip"

# Phase 1 contract gate execution slot (design only at this stage):
# - command entry: bash "${ROOT_DIR}/scripts/ci/backend-verify.sh" --phase1-contract-gate
# - contract spec: ${CONTRACT_GATE_DOC}
# - gate checklist: ${CONTRACT_GATE_CHECKLIST}
# - planned artifacts: ${CONTRACT_GATE_LOG} and ${CONTRACT_GATE_REPORT}
# - failure semantics: any scenario mismatch, missing required evidence, or non-zero command exit
#   must mark the backend verify result as FAIL.

HAS_FAIL=0
for row in "${STEP_RESULTS[@]}"; do
  status="$(echo "${row}" | awk -F'|' '{print $2}')"
  if [[ "${status}" == "FAIL" ]]; then
    HAS_FAIL=1
  fi
done

RESULT="PASS"
NOTE="all backend release gates passed"
if [[ "${HAS_FAIL}" -eq 1 ]]; then
  RESULT="FAIL"
  NOTE="at least one backend release gate failed"
fi

{
  echo "# Backend Verify Report"
  echo
  echo "- 时间戳：${TS}"
  echo "- 结果：**${RESULT}**"
  echo "- 说明：${NOTE}"
  echo
  echo "| 步骤 | 结果 | 说明 | 证据 |"
  echo "|---|---|---|---|"
  for row in "${STEP_RESULTS[@]}"; do
    step_id="$(echo "${row}" | awk -F'|' '{print $1}')"
    status="$(echo "${row}" | awk -F'|' '{print $2}')"
    title="$(echo "${row}" | awk -F'|' '{print $3}')"
    evidence="$(echo "${row}" | awk -F'|' '{print $4}')"
    echo "| ${step_id} | ${status} | ${title} | ${evidence} |"
  done
} > "${REPORT_FILE}"

log "[INFO] report generated: ${REPORT_FILE}"
log "[RESULT] ${RESULT}"

if [[ "${RESULT}" != "PASS" ]]; then
  exit 1
fi
