#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
TS="$(date +%F_%H%M%S)"
TODAY_TAG="$(date +%F)"
OUT_DIR="${ROOT_DIR}/reports/archive/gate_verification"
mkdir -p "${OUT_DIR}"

LOG_FILE="${OUT_DIR}/superpowers_release_pipeline_${TS}.log"
REPORT_FILE="${OUT_DIR}/superpowers_release_pipeline_${TS}.md"
ENABLE_MINIMAX_MONITORING="${ENABLE_MINIMAX_MONITORING:-0}"
MINIMAX_ENV_FILE="${MINIMAX_ENV_FILE:-scripts/supply-gate/.env.minimax-dev}"
MINIMAX_RUN_ACTIVE_SMOKE="${MINIMAX_RUN_ACTIVE_SMOKE:-0}"

# Planned real-staging hard gate:
# - fail fast immediately after STEP-01 if stage validation is not PASS_REAL
# - STEP-02 onward must not run release-path decisions on PASS_REHEARSAL/DEFERRED
# - override, if ever allowed, must require approver, timestamp, run_id, and reason

log() {
  echo "$1" | tee -a "${LOG_FILE}"
}

STEP_RESULTS=()

run_step() {
  local step_id="$1"
  local title="$2"
  local cmd="$3"

  log "[INFO] ${step_id} ${title} start"
  set +e
  bash -lc "${cmd}" > "${OUT_DIR}/${step_id,,}_${TS}.out.log" 2>&1
  local rc=$?
  set -e
  local evidence="${OUT_DIR}/${step_id,,}_${TS}.out.log"

  if [[ "${rc}" -eq 0 ]]; then
    log "[PASS] ${step_id} rc=${rc}"
    STEP_RESULTS+=("${step_id}|PASS|${title}|${evidence}")
  else
    if [[ "${step_id}" == "STEP-03" ]]; then
      # final decision consistency check can return WARN via exit 0; non-zero means parse failure only.
      log "[FAIL] ${step_id} rc=${rc}"
      STEP_RESULTS+=("${step_id}|FAIL|${title}|${evidence}")
    else
      log "[FAIL] ${step_id} rc=${rc}"
      STEP_RESULTS+=("${step_id}|FAIL|${title}|${evidence}")
    fi
  fi
}

run_optional_step_non_blocking() {
  local step_id="$1"
  local title="$2"
  local enabled="$3"
  local cmd="$4"

  if [[ "${enabled}" != "1" ]]; then
    log "[SKIP] ${step_id} not enabled"
    STEP_RESULTS+=("${step_id}|SKIP|${title}|not enabled")
    return
  fi

  log "[INFO] ${step_id} ${title} start"
  set +e
  bash -lc "${cmd}" > "${OUT_DIR}/${step_id,,}_${TS}.out.log" 2>&1
  local rc=$?
  set -e
  local evidence="${OUT_DIR}/${step_id,,}_${TS}.out.log"

  if [[ "${rc}" -eq 0 ]]; then
    log "[PASS] ${step_id} rc=${rc}"
    STEP_RESULTS+=("${step_id}|PASS|${title}|${evidence}")
  else
    # optional monitor step should not block release pipeline
    log "[WARN] ${step_id} rc=${rc} (non-blocking)"
    STEP_RESULTS+=("${step_id}|WARN|${title}|${evidence}")
  fi
}

run_step \
  "STEP-01" \
  "Superpowers stage validation (PHASE-01~10)" \
  "cd \"${ROOT_DIR}\" && bash \"scripts/ci/superpowers_stage_validate.sh\""

run_step \
  "STEP-02" \
  "TOK-007 release recheck" \
  "cd \"${ROOT_DIR}\" && bash \"scripts/ci/tok007_release_recheck.sh\""

run_step \
  "STEP-03" \
  "Final decision consistency check" \
  "cd \"${ROOT_DIR}\" && bash \"scripts/ci/final_decision_consistency_check.sh\""

run_step \
  "STEP-04" \
  "Generate final decision candidate from TOK-007" \
  "cd \"${ROOT_DIR}\" && bash \"scripts/ci/tok007_generate_final_decision_candidate.sh\""

run_optional_step_non_blocking \
  "STEP-05" \
  "Optional Minimax upstream monitoring snapshot+trend" \
  "${ENABLE_MINIMAX_MONITORING}" \
  "cd \"${ROOT_DIR}\" && RUN_ACTIVE_SMOKE=\"${MINIMAX_RUN_ACTIVE_SMOKE}\" bash \"scripts/ci/minimax_upstream_daily_snapshot.sh\" \"${TODAY_TAG}\" \"${MINIMAX_ENV_FILE}\" && bash \"scripts/ci/minimax_upstream_trend_report.sh\" \"${TODAY_TAG}\""

# Planned fail-fast insertion point:
# - parse STEP-01 report here
# - if real staging status != PASS_REAL and no valid override record exists, exit 1 before STEP-02

has_fail=0
for row in "${STEP_RESULTS[@]}"; do
  status="$(echo "${row}" | awk -F'|' '{print $2}')"
  if [[ "${status}" == "FAIL" ]]; then
    has_fail=1
  fi
done

PIPELINE_RESULT="PASS"
PIPELINE_NOTE="all steps finished"
if [[ "${has_fail}" -eq 1 ]]; then
  PIPELINE_RESULT="FAIL"
  PIPELINE_NOTE="at least one step failed"
fi

{
  echo "# Superpowers 发布流水执行报告"
  echo
  echo "- 时间戳：${TS}"
  echo "- 执行脚本：\`scripts/ci/superpowers_release_pipeline.sh\`"
  echo "- 结果：**${PIPELINE_RESULT}**"
  echo "- 说明：${PIPELINE_NOTE}"
  echo "- Minimax 监控步开关：\`${ENABLE_MINIMAX_MONITORING}\`（非阻断）"
  echo "- Minimax 监控环境：\`${MINIMAX_ENV_FILE}\`"
  echo "- Minimax 实时探测：\`${MINIMAX_RUN_ACTIVE_SMOKE}\`"
  echo
  echo "## 步骤结果"
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

log "[INFO] pipeline report generated: ${REPORT_FILE}"
log "[RESULT] ${PIPELINE_RESULT}"

if [[ "${PIPELINE_RESULT}" == "FAIL" ]]; then
  exit 1
fi
