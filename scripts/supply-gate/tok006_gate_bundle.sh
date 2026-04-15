#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
ENV_FILE="${1:-${SCRIPT_DIR}/.env}"
OUT_DIR="${ROOT_DIR}/reports/archive/gate_verification"
TS="$(date +%F_%H%M%S)"
BUNDLE_ID="tok006_gate_bundle_${TS}"
REPORT_FILE="${OUT_DIR}/${BUNDLE_ID}.md"
LOG_FILE="${OUT_DIR}/${BUNDLE_ID}.log"

mkdir -p "${OUT_DIR}"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "[FAIL] env file not found: ${ENV_FILE}"
  exit 1
fi

# shellcheck disable=SC1090
source "${ENV_FILE}"
ENABLE_TOK005_DRYRUN="${ENABLE_TOK005_DRYRUN:-1}"
ENABLE_SUP_RUN="${ENABLE_SUP_RUN:-0}"

log() {
  local msg="$1"
  echo "${msg}" | tee -a "${LOG_FILE}"
}

latest_file_or_empty() {
  local pattern="$1"
  local latest
  latest="$(ls -1t ${pattern} 2>/dev/null | head -n 1 || true)"
  echo "${latest}"
}

status_from_report() {
  local file="$1"
  if [[ -z "${file}" || ! -f "${file}" ]]; then
    echo "BLOCKED"
    return
  fi
  if grep -Eq '\bFAIL\b' "${file}"; then
    echo "FAIL"
    return
  fi
  if grep -Eq '\bPASS\b' "${file}"; then
    echo "PASS"
    return
  fi
  echo "BLOCKED"
}

env_from_report() {
  local file="$1"
  if [[ -z "${file}" || ! -f "${file}" ]]; then
    echo "-"
    return
  fi
  if grep -Eiq 'local-mock|mock' "${file}"; then
    echo "mock"
    return
  fi
  if grep -Eiq 'staging' "${file}"; then
    echo "staging"
    return
  fi
  echo "unknown"
}

extract_tok005_staging_readiness() {
  local tok005_report="$1"
  if [[ -z "${tok005_report}" || ! -f "${tok005_report}" ]]; then
    echo "UNKNOWN|tok005 report missing"
    return
  fi
  local row
  row="$(grep -E '^\| staging 实测就绪性 \|' "${tok005_report}" | head -n 1 || true)"
  if [[ -z "${row}" ]]; then
    echo "UNKNOWN|staging readiness row missing"
    return
  fi
  local ready reason
  ready="$(echo "${row}" | awk -F'|' '{gsub(/[[:space:]]/, "", $3); print $3}')"
  reason="$(echo "${row}" | awk -F'|' '{gsub(/^[[:space:]]+|[[:space:]]+$/, "", $4); print $4}')"
  if [[ -z "${ready}" ]]; then
    ready="UNKNOWN"
  fi
  echo "${ready}|${reason}"
}

any_fail=0
any_blocked=0
any_mock=0

log "[INFO] TOK-006 gate bundle started at ${TS}"
log "[INFO] env file: ${ENV_FILE}"
log "[INFO] ENABLE_TOK005_DRYRUN=${ENABLE_TOK005_DRYRUN}, ENABLE_SUP_RUN=${ENABLE_SUP_RUN}"

TOK005_STDOUT_LOG="${OUT_DIR}/${BUNDLE_ID}_tok005.stdout.log"
if [[ "${ENABLE_TOK005_DRYRUN}" == "1" ]]; then
  set +e
  bash "${SCRIPT_DIR}/tok005_boundary_dryrun.sh" "${ENV_FILE}" > "${TOK005_STDOUT_LOG}" 2>&1
  tok005_rc=$?
  set -e
  log "[INFO] TOK-005 dry-run executed with rc=${tok005_rc}, stdout=${TOK005_STDOUT_LOG}"
else
  log "[INFO] TOK-005 dry-run skipped by switch"
fi

if [[ "${ENABLE_SUP_RUN}" == "1" ]]; then
  set +e
  bash "${SCRIPT_DIR}/run_all.sh" "${ENV_FILE}" > "${OUT_DIR}/${BUNDLE_ID}_sup_run_all.stdout.log" 2>&1
  sup_run_rc=$?
  set -e
  log "[INFO] SUP run_all executed with rc=${sup_run_rc}"
else
  log "[INFO] SUP run_all skipped by switch"
fi

TOK005_REPORT="$(latest_file_or_empty "${OUT_DIR}/tok005_dryrun_*.md")"
SUP004_REPORT="$(latest_file_or_empty "${ROOT_DIR}/tests/supply/ui_sup_acc_report_*.md")"
SUP005_REPORT="$(latest_file_or_empty "${ROOT_DIR}/tests/supply/ui_sup_pkg_report_*.md")"
SUP006_REPORT="$(latest_file_or_empty "${ROOT_DIR}/tests/supply/ui_sup_set_report_*.md")"
SUP007_REPORT="$(latest_file_or_empty "${ROOT_DIR}/tests/supply/sec_sup_boundary_report_*.md")"

TOK005_STATUS="$(status_from_report "${TOK005_REPORT}")"
SUP004_STATUS="$(status_from_report "${SUP004_REPORT}")"
SUP005_STATUS="$(status_from_report "${SUP005_REPORT}")"
SUP006_STATUS="$(status_from_report "${SUP006_REPORT}")"
SUP007_STATUS="$(status_from_report "${SUP007_REPORT}")"

TOK005_ENV="$(env_from_report "${TOK005_REPORT}")"
SUP004_ENV="$(env_from_report "${SUP004_REPORT}")"
SUP005_ENV="$(env_from_report "${SUP005_REPORT}")"
SUP006_ENV="$(env_from_report "${SUP006_REPORT}")"
SUP007_ENV="$(env_from_report "${SUP007_REPORT}")"

for status in "${TOK005_STATUS}" "${SUP004_STATUS}" "${SUP005_STATUS}" "${SUP006_STATUS}" "${SUP007_STATUS}"; do
  if [[ "${status}" == "FAIL" ]]; then
    any_fail=1
  fi
  if [[ "${status}" == "BLOCKED" ]]; then
    any_blocked=1
  fi
done

for env_name in "${TOK005_ENV}" "${SUP004_ENV}" "${SUP005_ENV}" "${SUP006_ENV}" "${SUP007_ENV}"; do
  if [[ "${env_name}" == "mock" ]]; then
    any_mock=1
  fi
done

readiness_pair="$(extract_tok005_staging_readiness "${TOK005_REPORT}")"
TOK005_STAGING_READY="${readiness_pair%%|*}"
TOK005_STAGING_REASON="${readiness_pair#*|}"

DECISION="CONDITIONAL_GO"
DECISION_REASON="all gates pass but include mock evidence or staging readiness is not YES"
if [[ "${any_fail}" -eq 1 || "${any_blocked}" -eq 1 ]]; then
  DECISION="NO_GO"
  DECISION_REASON="at least one gate failed or blocked"
elif [[ "${TOK005_STAGING_READY}" == "YES" && "${any_mock}" -eq 0 ]]; then
  DECISION="GO"
  DECISION_REASON="all gates pass with non-mock evidence and staging readiness is YES"
fi

cat > "${REPORT_FILE}" <<EOF
# TOK-006 统一 Gate 汇总报告

- 时间戳：${TS}
- 执行入口：\`scripts/supply-gate/tok006_gate_bundle.sh\`
- 环境文件：${ENV_FILE}

## 1. Gate 矩阵

| Gate | 状态 | 环境 | 证据 |
|---|---|---|---|
| TOK-005 dry-run | ${TOK005_STATUS} | ${TOK005_ENV} | ${TOK005_REPORT:-N/A} |
| SUP-004 账号挂载 | ${SUP004_STATUS} | ${SUP004_ENV} | ${SUP004_REPORT:-N/A} |
| SUP-005 套餐发布 | ${SUP005_STATUS} | ${SUP005_ENV} | ${SUP005_REPORT:-N/A} |
| SUP-006 结算提现 | ${SUP006_STATUS} | ${SUP006_ENV} | ${SUP006_REPORT:-N/A} |
| SUP-007 边界专项 | ${SUP007_STATUS} | ${SUP007_ENV} | ${SUP007_REPORT:-N/A} |

## 2. 关键约束检查

| 项目 | 值 | 说明 |
|---|---|---|
| TOK-005 staging readiness | ${TOK005_STAGING_READY} | ${TOK005_STAGING_REASON} |
| 是否存在 FAIL | ${any_fail} | 1=是, 0=否 |
| 是否存在 BLOCKED | ${any_blocked} | 1=是, 0=否 |
| 是否包含 mock 证据 | ${any_mock} | 1=是, 0=否 |

## 3. 发布判定（单页）

- 判定：**${DECISION}**
- 判定依据：${DECISION_REASON}
- 说明：
  - GO：全部 gate 通过，且非 mock，且 staging readiness=YES。
  - CONDITIONAL_GO：全部 gate 通过，但存在 mock 证据或 staging readiness!=YES。
  - NO_GO：存在 FAIL/BLOCKED。

## 4. 下一步动作

1. 若判定为 CONDITIONAL_GO/NO_GO，优先补齐真实 staging 参数并执行：
   \`bash scripts/supply-gate/staging_precheck_and_run.sh scripts/supply-gate/.env\`
2. 联调完成后回填：
   \`tests/supply/sec_sup_boundary_report_2026-03-30.md\`、\`reports/supply_gate_review_2026-03-31.md\`。
EOF

log "[INFO] bundle report generated: ${REPORT_FILE}"
log "[RESULT] ${DECISION}"

if [[ "${DECISION}" == "NO_GO" ]]; then
  exit 1
fi
