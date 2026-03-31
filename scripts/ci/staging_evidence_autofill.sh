#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
OUT_DIR="${ROOT_DIR}/reports/gates"
TS="$(date +%F_%H%M%S)"
OUT_FILE="${OUT_DIR}/staging_token_go_evidence_autofill_${TS}.md"
LOG_FILE="${OUT_DIR}/staging_token_go_evidence_autofill_${TS}.log"

mkdir -p "${OUT_DIR}"

usage() {
  cat <<'EOF'
Usage:
  bash scripts/ci/staging_evidence_autofill.sh [options]

Options:
  --staging-run-log <path>     指定 staging_run_*.log
  --stage-report <path>        指定 superpowers_stage_validation_*.md
  --token-readiness <path>     指定 token_runtime_readiness_*.md
  --tok007-report <path>       指定 tok007_release_recheck_*.md
  --pipeline-report <path>     指定 superpowers_release_pipeline_*.md
  --sec-report <path>          指定 sec_sup_boundary_report_*.md
  --out-file <path>            指定输出 markdown 文件路径
  -h, --help                   查看帮助
EOF
}

resolve_path() {
  local value="$1"
  if [[ -z "${value}" ]]; then
    echo ""
    return
  fi
  if [[ "${value}" == /* ]]; then
    echo "${value}"
  else
    echo "${ROOT_DIR}/${value}"
  fi
}

require_arg() {
  local opt="$1"
  local value="${2:-}"
  if [[ -z "${value}" ]]; then
    echo "[FAIL] missing value for ${opt}" >&2
    usage >&2
    exit 1
  fi
}

latest_file_or_empty() {
  local pattern="$1"
  local latest
  latest="$(ls -1t ${pattern} 2>/dev/null | head -n 1 || true)"
  echo "${latest}"
}

extract_phase_status() {
  local file="$1"
  local phase="$2"
  if [[ ! -f "${file}" ]]; then
    echo "N/A"
    return
  fi
  awk -F'|' -v p="${phase}" '
    {
      f2=$2
      gsub(/^ +| +$/, "", f2)
      if (f2 == p) {
        f3=$3
        gsub(/^ +| +$/, "", f3)
        print f3
        found=1
        exit
      }
    }
    END { if (!found) print "N/A" }
  ' "${file}"
}

extract_metric_from_sec_report() {
  local file="$1"
  local metric="$2"
  if [[ ! -f "${file}" ]]; then
    echo "N/A"
    return
  fi
  awk -F'|' -v m="${metric}" '
    {
      f2=$2
      gsub(/^ +| +$/, "", f2)
      if (f2 == m) {
        f3=$3
        gsub(/^ +| +$/, "", f3)
        print f3
        found=1
        exit
      }
    }
    END { if (!found) print "N/A" }
  ' "${file}"
}

extract_m021_value() {
  local file="$1"
  if [[ ! -f "${file}" ]]; then
    echo "N/A"
    return
  fi
  local row
  row="$(grep -E '^- 数值：' "${file}" | head -n 1 || true)"
  if [[ -z "${row}" ]]; then
    echo "N/A"
    return
  fi
  echo "${row#- 数值：}"
}

extract_m021_result() {
  local file="$1"
  if [[ ! -f "${file}" ]]; then
    echo "N/A"
    return
  fi
  local row
  row="$(grep -E '^- 结果：\*\*' "${file}" | head -n 1 || true)"
  if [[ -z "${row}" ]]; then
    echo "N/A"
    return
  fi
  if echo "${row}" | grep -q 'PASS'; then
    echo "PASS"
    return
  fi
  if echo "${row}" | grep -q 'FAIL'; then
    echo "FAIL"
    return
  fi
  echo "N/A"
}

extract_tok007_machine_decision() {
  local file="$1"
  if [[ ! -f "${file}" ]]; then
    echo "N/A"
    return
  fi
  local row
  row="$(grep -E '^- 机判结论：\*\*' "${file}" | head -n 1 || true)"
  if [[ -z "${row}" ]]; then
    echo "N/A"
    return
  fi
  echo "${row}" | sed -E 's/^- 机判结论：\*\*([^*]+)\*\*$/\1/'
}

STAGING_RUN_LOG=""
SP_REPORT=""
TOK021_REPORT=""
TOK007_REPORT=""
PIPELINE_REPORT=""
SEC_REPORT="${ROOT_DIR}/tests/supply/sec_sup_boundary_report_2026-03-30.md"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --staging-run-log)
      require_arg "$1" "${2:-}"
      STAGING_RUN_LOG="$(resolve_path "$2")"
      shift 2
      ;;
    --stage-report)
      require_arg "$1" "${2:-}"
      SP_REPORT="$(resolve_path "$2")"
      shift 2
      ;;
    --token-readiness)
      require_arg "$1" "${2:-}"
      TOK021_REPORT="$(resolve_path "$2")"
      shift 2
      ;;
    --tok007-report)
      require_arg "$1" "${2:-}"
      TOK007_REPORT="$(resolve_path "$2")"
      shift 2
      ;;
    --pipeline-report)
      require_arg "$1" "${2:-}"
      PIPELINE_REPORT="$(resolve_path "$2")"
      shift 2
      ;;
    --sec-report)
      require_arg "$1" "${2:-}"
      SEC_REPORT="$(resolve_path "$2")"
      shift 2
      ;;
    --out-file)
      require_arg "$1" "${2:-}"
      OUT_FILE="$(resolve_path "$2")"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "[FAIL] unknown arg: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "${STAGING_RUN_LOG}" ]]; then
  STAGING_RUN_LOG="$(latest_file_or_empty "${ROOT_DIR}/reports/gates/staging_run_*.log")"
fi
if [[ -z "${SP_REPORT}" ]]; then
  SP_REPORT="$(latest_file_or_empty "${ROOT_DIR}/reports/gates/superpowers_stage_validation_*.md")"
fi
if [[ -z "${TOK021_REPORT}" ]]; then
  TOK021_REPORT="$(latest_file_or_empty "${ROOT_DIR}/reports/gates/token_runtime_readiness_*.md")"
fi
if [[ -z "${TOK007_REPORT}" ]]; then
  TOK007_REPORT="$(latest_file_or_empty "${ROOT_DIR}/review/outputs/tok007_release_recheck_*.md")"
fi
if [[ -z "${PIPELINE_REPORT}" ]]; then
  PIPELINE_REPORT="$(latest_file_or_empty "${ROOT_DIR}/reports/gates/superpowers_release_pipeline_*.md")"
fi

LOG_FILE="${OUT_DIR}/staging_token_go_evidence_autofill_${TS}.log"

PHASE07="$(extract_phase_status "${SP_REPORT}" "PHASE-07")"
M013="$(extract_metric_from_sec_report "${SEC_REPORT}" "M-013")"
M014="$(extract_metric_from_sec_report "${SEC_REPORT}" "M-014")"
M015="$(extract_metric_from_sec_report "${SEC_REPORT}" "M-015")"
M016="$(extract_metric_from_sec_report "${SEC_REPORT}" "M-016")"
M021_VALUE="$(extract_m021_value "${TOK021_REPORT}")"
M021_RESULT="$(extract_m021_result "${TOK021_REPORT}")"
TOK007_DECISION="$(extract_tok007_machine_decision "${TOK007_REPORT}")"

{
  echo "# Staging 联调证据自动回填草稿"
  echo
  echo "- 生成时间：${TS}"
  echo "- 生成脚本：\`scripts/ci/staging_evidence_autofill.sh\`"
  echo
  echo "## 1. 自动抽取结果"
  echo
  echo "| 项目 | 自动值 | 来源 |"
  echo "|---|---|---|"
  echo "| PHASE-07 | ${PHASE07} | ${SP_REPORT:-N/A} |"
  echo "| M-013 | ${M013} | ${SEC_REPORT} |"
  echo "| M-014 | ${M014} | ${SEC_REPORT} |"
  echo "| M-015 | ${M015} | ${SEC_REPORT} |"
  echo "| M-016 | ${M016} | ${SEC_REPORT} |"
  echo "| M-021（值） | ${M021_VALUE} | ${TOK021_REPORT:-N/A} |"
  echo "| M-021（结果） | ${M021_RESULT} | ${TOK021_REPORT:-N/A} |"
  echo "| TOK-007 机判 | ${TOK007_DECISION} | ${TOK007_REPORT:-N/A} |"
  echo
  echo "## 2. 证据路径清单"
  echo
  echo "1. staging run：${STAGING_RUN_LOG:-N/A}"
  echo "2. stage validate：${SP_REPORT:-N/A}"
  echo "3. token readiness：${TOK021_REPORT:-N/A}"
  echo "4. tok007 recheck：${TOK007_REPORT:-N/A}"
  echo "5. release pipeline：${PIPELINE_REPORT:-N/A}"
  echo "6. security boundary：${SEC_REPORT}"
  echo
  echo "## 3. 人工确认项"
  echo
  echo "1. 若 PHASE-07 仍为 DEFERRED，禁止将结论上调为 GO。"
  echo "2. 若 M-013~M-016 来源为 mock，必须在 staging 复测后覆盖。"
  echo "3. 若 M-021 仅为开发阶段口径，需在 staging 复跑后再次回填。"
} > "${OUT_FILE}"

{
  echo "[INFO] output=${OUT_FILE}"
  echo "[INFO] PHASE-07=${PHASE07}, M021_RESULT=${M021_RESULT}, TOK007=${TOK007_DECISION}"
  echo "[RESULT] PASS"
} | tee -a "${LOG_FILE}"
