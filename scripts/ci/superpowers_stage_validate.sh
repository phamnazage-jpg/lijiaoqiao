#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
LIB_FILE="${ROOT_DIR}/scripts/ci/lib/verification_common.sh"
# shellcheck disable=SC1091
source "${LIB_FILE}"
TS="$(date +%F_%H%M%S)"
OUT_DIR="${ROOT_DIR}/reports/archive/gate_verification"
ART_DIR="${ROOT_DIR}/tests/supply/artifacts/superpowers_stage_validation_${TS}"
REPORT_FILE="${OUT_DIR}/superpowers_stage_validation_${TS}.md"
LOG_FILE="${OUT_DIR}/superpowers_stage_validation_${TS}.log"
DEP_AUDIT_DATE="${DEP_AUDIT_DATE:-2026-03-27}"
STAGE_DRILL_DATE="${STAGE_DRILL_DATE:-$(date +%F)}"
STAGING_ENV_FILE="${STAGING_ENV_FILE:-scripts/supply-gate/.env}"

mkdir -p "${OUT_DIR}" "${ART_DIR}"
: > "${LOG_FILE}"

log() {
  echo "$1" | tee -a "${LOG_FILE}"
}

is_mock_staging_env() {
  local env_path="$1"
  if [[ -z "${env_path}" ]]; then
    return 1
  fi
  if [[ "${env_path}" != /* ]]; then
    env_path="${ROOT_DIR}/${env_path}"
  fi
  if [[ ! -f "${env_path}" ]]; then
    return 1
  fi
  if echo "${env_path}" | grep -Eiq 'local-mock'; then
    return 0
  fi
  local api_base
  api_base="$(grep -E '^API_BASE_URL=' "${env_path}" | head -n 1 | cut -d'=' -f2- | tr -d '\"' || true)"
  if echo "${api_base}" | grep -Eiq '127\.0\.0\.1|localhost'; then
    return 0
  fi
  return 1
}

STEP_RESULTS=()

run_step() {
  local step_id="$1"
  local title="$2"
  local cmd="$3"
  local out_file="$4"

  log "[INFO] ${step_id} ${title} start"
  set +e
  bash -lc "${cmd}" > "${out_file}" 2>&1
  local rc=$?
  set -e
  if [[ ${rc} -eq 0 ]]; then
    log "[PASS] ${step_id} rc=${rc}"
    write_step_result STEP_RESULTS "${step_id}" "PASS" "${title}" "${out_file}"
  else
    log "[FAIL] ${step_id} rc=${rc}"
    write_step_result STEP_RESULTS "${step_id}" "FAIL" "${title}" "${out_file}"
  fi
}

run_step_allow_deferred() {
  local step_id="$1"
  local title="$2"
  local cmd="$3"
  local out_file="$4"
  local deferred_pattern="$5"

  log "[INFO] ${step_id} ${title} start"
  set +e
  bash -lc "${cmd}" > "${out_file}" 2>&1
  local rc=$?
  set -e

  if [[ ${rc} -eq 0 ]]; then
    log "[PASS] ${step_id} rc=${rc}"
    write_step_result STEP_RESULTS "${step_id}" "PASS" "${title}" "${out_file}"
    return
  fi

  if grep -Eiq "${deferred_pattern}" "${out_file}"; then
    log "[DEFERRED] ${step_id} rc=${rc} matched expected pattern"
    write_step_result STEP_RESULTS "${step_id}" "DEFERRED" "${title}" "${out_file}"
    return
  fi

  log "[FAIL] ${step_id} rc=${rc}"
  write_step_result STEP_RESULTS "${step_id}" "FAIL" "${title}" "${out_file}"
}

run_phase07() {
  local out_file="$1"

  if is_mock_staging_env "${STAGING_ENV_FILE}"; then
    {
      echo "[INFO] env_class=local-mock"
      echo "[DEFERRED] PHASE-07 requires real staging inputs"
    } > "${out_file}"
    log "[DEFERRED] PHASE-07 local/mock staging env detected"
    write_step_result STEP_RESULTS "PHASE-07" "DEFERRED" "Real staging precheck (local/mock env)" "${out_file}"
    return
  fi

  run_step_allow_deferred \
    "PHASE-07" \
    "Real staging precheck (expected deferred before real secrets)" \
    "cd \"${ROOT_DIR}\" && bash \"scripts/supply-gate/staging_precheck_and_run.sh\" \"${STAGING_ENV_FILE}\"" \
    "${out_file}" \
    "env_class=local-mock|placeholder token detected|placeholder API_BASE_URL|missing env var|API_BASE_URL unreachable"
}

# Real staging decision design:
# - PASS_REAL: PHASE-07 runs against real staging and succeeds.
# - PASS_REHEARSAL: all executable rehearsal phases pass, but PHASE-07 is local/mock/deferred.
# - FAIL: any required phase fails, or PHASE-07 is not real-pass when release flow requests hard gate.
# - DEFERRED must never be promoted to PASS_REAL.

ensure_mock_server() {
  if curl -sS -m 2 "http://127.0.0.1:18080/actuator/health" >/dev/null 2>&1; then
    echo "already_running"
    return
  fi
  nohup python3 "${ROOT_DIR}/scripts/mock/supply_gateway_mock_server.py" > "${ART_DIR}/mock_server.log" 2>&1 &
  local pid=$!
  for _ in {1..20}; do
    if curl -sS -m 2 "http://127.0.0.1:18080/actuator/health" >/dev/null 2>&1; then
      echo "${pid}"
      return
    fi
    sleep 0.2
  done
  echo "failed"
}

MOCK_PID="$(ensure_mock_server)"
if [[ "${MOCK_PID}" == "failed" ]]; then
  log "[FAIL] cannot start mock server on 127.0.0.1:18080"
  exit 1
fi
if [[ "${MOCK_PID}" != "already_running" ]]; then
  log "[INFO] mock server started with pid=${MOCK_PID}"
  trap 'kill "${MOCK_PID}" >/dev/null 2>&1 || true' EXIT
else
  log "[INFO] mock server already running"
fi

GO_BIN="$(resolve_go_bin "${ROOT_DIR}" || true)"
if [[ -z "${GO_BIN}" ]]; then
  log "[FAIL] go binary not found"
  exit 1
fi
setup_go_env "${GO_BIN}" "${ROOT_DIR}/.tools/go-cache"

run_step \
  "PHASE-00" \
  "Backend critical verification gate" \
  "cd \"${ROOT_DIR}\" && bash \"scripts/ci/backend-verify.sh\"" \
  "${ART_DIR}/phase00_backend_verify.log"

run_step \
  "PHASE-01" \
  "TOK runtime code tests" \
  "cd \"${ROOT_DIR}/platform-token-runtime\" && \"${GO_BIN}\" test ./..." \
  "${ART_DIR}/phase01_go_test.log"

run_step \
  "PHASE-02" \
  "SUP local-mock run_all execution" \
  "cd \"${ROOT_DIR}\" && bash \"scripts/supply-gate/run_all.sh\" \"scripts/supply-gate/.env.local-mock\"" \
  "${ART_DIR}/phase02_sup_run_all_mock.log"

run_step \
  "PHASE-03" \
  "TOK-005 boundary dry-run on local-mock env" \
  "cd \"${ROOT_DIR}\" && bash \"scripts/supply-gate/tok005_boundary_dryrun.sh\" \"scripts/supply-gate/.env.local-mock\"" \
  "${ART_DIR}/phase03_tok005_dryrun_mock.log"

run_step \
  "PHASE-04" \
  "TOK-006 gate bundle aggregation" \
  "cd \"${ROOT_DIR}\" && ENABLE_SUP_RUN=0 ENABLE_TOK005_DRYRUN=1 bash \"scripts/supply-gate/tok006_gate_bundle.sh\" \"scripts/supply-gate/.env.local-mock\"" \
  "${ART_DIR}/phase04_tok006_bundle.log"

run_step \
  "PHASE-05" \
  "Dependency audit gate validation" \
  "cd \"${ROOT_DIR}\" && bash \"scripts/ci/dependency-audit-check.sh\" \"${DEP_AUDIT_DATE}\"" \
  "${ART_DIR}/phase05_dependency_audit.log"

run_step \
  "PHASE-06" \
  "Stage gate rollback drill" \
  "cd \"${ROOT_DIR}\" && bash \"scripts/ci/stage-gate-drill.sh\" \"G3\" \"${STAGE_DRILL_DATE}\"" \
  "${ART_DIR}/phase06_stage_gate_drill.log"

run_phase07 "${ART_DIR}/phase07_staging_precheck.log"

run_step \
  "PHASE-08" \
  "Daily metrics snapshot for M-017/M-018/M-019" \
  "cd \"${ROOT_DIR}\" && bash \"scripts/ci/metrics_daily_snapshot.sh\" \"$(date +%F)\"" \
  "${ART_DIR}/phase08_metrics_snapshot.log"

run_step \
  "PHASE-09" \
  "7-day metrics trend report generation" \
  "cd \"${ROOT_DIR}\" && bash \"scripts/ci/metrics_trend_report.sh\" \"$(date +%F)\"" \
  "${ART_DIR}/phase09_metrics_trend.log"

run_step \
  "PHASE-10" \
  "Token runtime readiness check (M-021)" \
  "cd \"${ROOT_DIR}\" && ENABLE_TOKEN_RUNTIME_SMOKE=1 bash \"scripts/ci/token_runtime_readiness_check.sh\" \"$(date +%F)\"" \
  "${ART_DIR}/phase10_token_runtime_readiness.log"

has_fail=0
has_deferred=0
for row in "${STEP_RESULTS[@]}"; do
  status="$(echo "${row}" | awk -F'|' '{print $2}')"
  if [[ "${status}" == "FAIL" ]]; then
    has_fail=1
  fi
  if [[ "${status}" == "DEFERRED" ]]; then
    has_deferred=1
  fi
done

DECISION="GO"
DECISION_REASON="all phases passed"
if [[ "${has_fail}" -eq 1 ]]; then
  DECISION="NO_GO"
  DECISION_REASON="at least one phase failed"
elif [[ "${has_deferred}" -eq 1 ]]; then
  DECISION="CONDITIONAL_GO"
  DECISION_REASON="all executable phases passed but real staging phase is deferred"
fi

if is_mock_staging_env "${STAGING_ENV_FILE}" && [[ "${DECISION}" == "GO" ]]; then
  DECISION="CONDITIONAL_GO"
  DECISION_REASON="all phases passed but PHASE-07 used local/mock staging env"
fi

# Future release hard-gate mapping:
# - GO maps to PASS_REAL only when PHASE-07 is real-staging PASS.
# - CONDITIONAL_GO maps to PASS_REHEARSAL and must not satisfy release pass.
# - NO_GO maps to FAIL.

{
  echo "# Superpowers 阶段验证报告"
  echo
  echo "- 时间戳：${TS}"
  echo "- 执行脚本：\`scripts/ci/superpowers_stage_validate.sh\`"
  echo "- 决策：**${DECISION}**"
  echo "- 决策依据：${DECISION_REASON}"
  echo
  echo "## 阶段结果"
  echo
  echo "| 阶段 | 结果 | 说明 | 证据 |"
  echo "|---|---|---|---|"
  for row in "${STEP_RESULTS[@]}"; do
    step_id="$(echo "${row}" | awk -F'|' '{print $1}')"
    status="$(echo "${row}" | awk -F'|' '{print $2}')"
    title="$(echo "${row}" | awk -F'|' '{print $3}')"
    evidence="$(echo "${row}" | awk -F'|' '{print $4}')"
    echo "| ${step_id} | ${status} | ${title} | ${evidence} |"
  done
  echo
  echo "## 说明"
  echo
  echo "1. PHASE-07 为真实 staging 验证阶段；local/mock 或占位输入只能记为 DEFERRED，不得伪造 PASS。"
  echo "2. PHASE-08/09 负责 M-017/M-018/M-019 的每日快照与趋势证据生成。"
  echo "3. PHASE-10 负责 M-021 token 运行态就绪度计算。"
  echo "4. 其余阶段均为可执行验证，必须以命令返回码与证据文件为准。"
} > "${REPORT_FILE}"

log "[INFO] report generated: ${REPORT_FILE}"
log "[RESULT] ${DECISION}"

if [[ "${DECISION}" == "NO_GO" || "${DECISION}" == "CONDITIONAL_GO" ]]; then
  exit 1
fi
