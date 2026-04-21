#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
ENV_FILE_REL="${1:-scripts/supply-gate/.env}"
if [[ "${ENV_FILE_REL}" == /* ]]; then
  ENV_FILE="${ENV_FILE_REL}"
else
  ENV_FILE="${ROOT_DIR}/${ENV_FILE_REL}"
fi
TS="$(date +%F_%H%M%S)"
OUT_DIR="${ROOT_DIR}/reports/archive/gate_verification"
RELEASES_DIR="${ROOT_DIR}/reports/releases"
mkdir -p "${OUT_DIR}"

REPORT_FILE="${OUT_DIR}/staging_release_pipeline_${TS}.md"
LOG_FILE="${OUT_DIR}/staging_release_pipeline_${TS}.log"
ALLOW_LOCAL_MOCK_STAGING="${ALLOW_LOCAL_MOCK_STAGING:-0}"

# Manifest migration design:
# - run_id format: YYYYMMDD_HHMMSS_<shortsha>_<env>[-rNN]
# - release root: ${RELEASES_DIR}/<run_id>/
# - manifest path: ${RELEASES_DIR}/<run_id>/manifest.json
# - this script becomes the manifest seed writer and must pass the resolved manifest path
#   to downstream scripts instead of relying on latest_file_or_empty().

log() {
  echo "$1" | tee -a "${LOG_FILE}"
}

latest_file_or_empty() {
  local pattern="$1"
  local latest
  latest="$(ls -1t ${pattern} 2>/dev/null | head -n 1 || true)"
  echo "${latest}"
}

read_env_api_base_url() {
  local env_path="$1"
  grep -E '^API_BASE_URL=' "${env_path}" | head -n 1 | cut -d'=' -f2- | tr -d '\"' || true
}

is_mock_staging_env() {
  local env_path="$1"
  if echo "${env_path}" | grep -Eiq 'local-mock'; then
    return 0
  fi
  if [[ ! -f "${env_path}" ]]; then
    return 1
  fi
  local api_base
  api_base="$(read_env_api_base_url "${env_path}")"
  if echo "${api_base}" | grep -Eiq '127\.0\.0\.1|localhost|staging\.example\.com'; then
    return 0
  fi
  return 1
}

if [[ ! -f "${ENV_FILE}" ]]; then
  log "[FAIL] env file not found: ${ENV_FILE}"
  exit 1
fi

MOCK_SERVER_PID=""
ENV_CLASSIFICATION="REAL_STAGING"
if is_mock_staging_env "${ENV_FILE}"; then
  ENV_CLASSIFICATION="LOCAL_MOCK"
  if [[ "${ALLOW_LOCAL_MOCK_STAGING}" != "1" ]]; then
    log "[FAIL] local/mock env detected (${ENV_FILE_REL})."
    log "[FAIL] for safety, set ALLOW_LOCAL_MOCK_STAGING=1 to run this rehearsal explicitly."
    exit 1
  fi
  log "[WARN] local/mock env acknowledged by ALLOW_LOCAL_MOCK_STAGING=1; result cannot be used as real staging evidence."
fi

if [[ "${ENV_CLASSIFICATION}" == "LOCAL_MOCK" ]]; then
  API_BASE_URL="$(read_env_api_base_url "${ENV_FILE}")"
  if [[ -n "${API_BASE_URL}" ]] && echo "${API_BASE_URL}" | grep -Eiq '127\.0\.0\.1|localhost'; then
    if ! curl -sS -m 2 -I "${API_BASE_URL}" >/dev/null 2>&1; then
      log "[INFO] local/mock API unreachable, starting mock server for rehearsal."
      nohup python3 "${ROOT_DIR}/scripts/mock/supply_gateway_mock_server.py" \
        > "${OUT_DIR}/staging_mock_server_${TS}.log" 2>&1 &
      MOCK_SERVER_PID=$!
      for _ in {1..20}; do
        if curl -sS -m 2 -I "${API_BASE_URL}" >/dev/null 2>&1; then
          break
        fi
        sleep 0.2
      done
      if ! curl -sS -m 2 -I "${API_BASE_URL}" >/dev/null 2>&1; then
        log "[FAIL] cannot start local/mock server for ${API_BASE_URL}"
        exit 1
      fi
      log "[INFO] local/mock server started pid=${MOCK_SERVER_PID}"
      trap 'kill "${MOCK_SERVER_PID}" >/dev/null 2>&1 || true' EXIT
    else
      log "[INFO] local/mock API already reachable: ${API_BASE_URL}"
    fi
  fi
fi

STEP_RESULTS=()

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

  if [[ ${rc} -eq 0 ]]; then
    STEP_RESULTS+=("${step_id}|PASS|${title}|${out_file}")
    log "[PASS] ${step_id} rc=${rc}"
  else
    STEP_RESULTS+=("${step_id}|FAIL|${title}|${out_file}")
    log "[FAIL] ${step_id} rc=${rc}"
  fi
}

run_step \
  "STEP-01" \
  "Staging precheck and run_all" \
  "cd \"${ROOT_DIR}\" && bash \"scripts/supply-gate/staging_precheck_and_run.sh\" \"${ENV_FILE}\""

run_step \
  "STEP-02" \
  "Superpowers release pipeline with staging env" \
  "cd \"${ROOT_DIR}\" && STAGING_ENV_FILE=\"${ENV_FILE_REL}\" bash \"scripts/ci/superpowers_release_pipeline.sh\""

# Planned manifest inputs for staging_evidence_autofill.sh:
# - decision_inputs.staging_run_log
# - decision_inputs.stage_report
# - decision_inputs.token_runtime_readiness_report
# - decision_inputs.tok007_recheck_report
# - artifact_paths.superpowers_release_pipeline_report
LATEST_STAGING_RUN_LOG="$(latest_file_or_empty "${OUT_DIR}/staging_run_*.log")"
LATEST_STAGE_REPORT="$(latest_file_or_empty "${OUT_DIR}/superpowers_stage_validation_*.md")"
LATEST_TOKEN_READINESS="$(latest_file_or_empty "${OUT_DIR}/token_runtime_readiness_*.md")"
LATEST_TOK007_REPORT="$(latest_file_or_empty "${ROOT_DIR}/review/outputs/tok007_release_recheck_*.md")"
LATEST_PIPELINE_REPORT="$(latest_file_or_empty "${OUT_DIR}/superpowers_release_pipeline_*.md")"
SEC_REPORT="${ROOT_DIR}/tests/supply/sec_sup_boundary_report_2026-03-30.md"

run_step \
  "STEP-03" \
  "Staging evidence autofill" \
  "cd \"${ROOT_DIR}\" && bash \"scripts/ci/staging_evidence_autofill.sh\" \
    --staging-run-log \"${LATEST_STAGING_RUN_LOG}\" \
    --stage-report \"${LATEST_STAGE_REPORT}\" \
    --token-readiness \"${LATEST_TOKEN_READINESS}\" \
    --tok007-report \"${LATEST_TOK007_REPORT}\" \
    --pipeline-report \"${LATEST_PIPELINE_REPORT}\" \
    --sec-report \"${SEC_REPORT}\""

HAS_FAIL=0
for row in "${STEP_RESULTS[@]}"; do
  status="$(echo "${row}" | awk -F'|' '{print $2}')"
  if [[ "${status}" == "FAIL" ]]; then
    HAS_FAIL=1
  fi
done

RESULT="PASS"
NOTE="all steps finished"
if [[ "${HAS_FAIL}" -eq 1 ]]; then
  RESULT="FAIL"
  NOTE="at least one step failed"
fi

{
  echo "# Staging 发布流水报告"
  echo
  echo "- 时间戳：${TS}"
  echo "- 执行脚本：\`scripts/ci/staging_release_pipeline.sh\`"
  echo "- 环境文件：\`${ENV_FILE_REL}\`"
  echo "- 环境分类：\`${ENV_CLASSIFICATION}\`"
  echo "- local/mock 显式确认：\`${ALLOW_LOCAL_MOCK_STAGING}\`"
  echo "- 结果：**${RESULT}**"
  echo "- 说明：${NOTE}"
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

log "[INFO] report=${REPORT_FILE}"
log "[RESULT] ${RESULT}"

if [[ "${RESULT}" == "FAIL" ]]; then
  exit 1
fi
