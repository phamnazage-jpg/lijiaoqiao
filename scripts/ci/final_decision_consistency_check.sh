#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
TS="$(date +%F_%H%M%S)"
OUT_DIR="${ROOT_DIR}/reports/archive/gate_verification"
RELEASES_DIR="${ROOT_DIR}/reports/releases"
mkdir -p "${OUT_DIR}"

REPORT_FILE="${OUT_DIR}/final_decision_consistency_${TS}.md"
LOG_FILE="${OUT_DIR}/final_decision_consistency_${TS}.log"

# Manifest migration design:
# - preferred entry: --manifest <reports/releases/<run_id>/manifest.json>
# - this script should bind final_decision, tok007_recheck and stage validation inputs
#   to one run_id and stop reading historical "latest" outputs

latest_file_or_empty() {
  local pattern="$1"
  local latest
  latest="$(ls -1t ${pattern} 2>/dev/null | head -n 1 || true)"
  echo "${latest}"
}

parse_checkbox_decision() {
  local file="$1"
  local go="0"
  local cgo="0"
  local nogo="0"

  if [[ ! -f "${file}" ]]; then
    echo "UNKNOWN"
    return
  fi

  grep -Eq '^- \[x\] (GO|通过)' "${file}" && go="1" || true
  grep -Eq '^- \[x\] (CONDITIONAL GO|有条件通过)' "${file}" && cgo="1" || true
  grep -Eq '^- \[x\] (NO-GO|不通过)' "${file}" && nogo="1" || true

  if [[ "${go}" == "1" ]]; then
    echo "GO"
    return
  fi
  if [[ "${cgo}" == "1" ]]; then
    echo "CONDITIONAL_GO"
    return
  fi
  if [[ "${nogo}" == "1" ]]; then
    echo "NO_GO"
    return
  fi
  echo "UNKNOWN"
}

parse_machine_decision() {
  local file="$1"
  if [[ ! -f "${file}" ]]; then
    echo "UNKNOWN"
    return
  fi
  local row
  row="$(grep -E '^\- (机判结论|决策)：\*\*' "${file}" | head -n 1 || true)"
  if [[ -z "${row}" ]]; then
    echo "UNKNOWN"
    return
  fi
  if echo "${row}" | grep -q 'NO_GO'; then
    echo "NO_GO"
    return
  fi
  if echo "${row}" | grep -q 'CONDITIONAL_GO'; then
    echo "CONDITIONAL_GO"
    return
  fi
  if echo "${row}" | grep -q 'GO'; then
    echo "GO"
    return
  fi
  echo "UNKNOWN"
}

FINAL_DECISION_FILE="${ROOT_DIR}/review/final_decision_2026-03-31.md"
# Planned manifest keys:
# - decision_inputs.final_decision_report
# - decision_inputs.tok007_recheck_report
# - decision_inputs.superpowers_stage_validation_report
TOK007_FILE="$(latest_file_or_empty "${ROOT_DIR}/review/outputs/tok007_release_recheck_*.md")"
SP_FILE="$(latest_file_or_empty "${OUT_DIR}/superpowers_stage_validation_*.md")"

FINAL_DECISION="$(parse_checkbox_decision "${FINAL_DECISION_FILE}")"
TOK007_DECISION="$(parse_machine_decision "${TOK007_FILE}")"
SP_DECISION="$(parse_machine_decision "${SP_FILE}")"

CONSISTENCY_STATUS="PASS"
CONSISTENCY_NOTE="final decision is aligned with latest machine recheck"

if [[ "${FINAL_DECISION}" == "UNKNOWN" || "${TOK007_DECISION}" == "UNKNOWN" || "${SP_DECISION}" == "UNKNOWN" ]]; then
  CONSISTENCY_STATUS="FAIL"
  CONSISTENCY_NOTE="cannot parse one or more decision sources"
elif [[ "${FINAL_DECISION}" != "${TOK007_DECISION}" ]]; then
  CONSISTENCY_STATUS="WARN"
  CONSISTENCY_NOTE="final signed decision lags latest machine recheck; requires manual review update"
fi

cat > "${REPORT_FILE}" <<EOF
# Final Decision Consistency Check

- 时间戳：${TS}
- 执行脚本：\`scripts/ci/final_decision_consistency_check.sh\`

## 1. 输入源

| 来源 | 路径 | 解析结论 |
|---|---|---|
| final_decision | ${FINAL_DECISION_FILE} | ${FINAL_DECISION} |
| tok007_recheck | ${TOK007_FILE:-N/A} | ${TOK007_DECISION} |
| superpowers_stage_validation | ${SP_FILE:-N/A} | ${SP_DECISION} |

## 2. 一致性结果

- 状态：**${CONSISTENCY_STATUS}**
- 说明：${CONSISTENCY_NOTE}

## 3. 建议动作

1. 若状态为 WARN：人工确认是否需要更新 \`review/final_decision_2026-03-31.md\` 的勾选与签署记录。
2. 若状态为 FAIL：先修复报告来源或解析格式，再重新执行本检查。
3. staging 真值就绪后，按顺序重跑：
   1. \`scripts/ci/superpowers_stage_validate.sh\`
   2. \`scripts/ci/tok007_release_recheck.sh\`
   3. \`scripts/ci/final_decision_consistency_check.sh\`
EOF

{
  echo "[INFO] FINAL_DECISION=${FINAL_DECISION}"
  echo "[INFO] TOK007_DECISION=${TOK007_DECISION}"
  echo "[INFO] SP_DECISION=${SP_DECISION}"
  echo "[RESULT] ${CONSISTENCY_STATUS}"
  echo "[INFO] REPORT=${REPORT_FILE}"
} | tee "${LOG_FILE}"

if [[ "${CONSISTENCY_STATUS}" == "FAIL" ]]; then
  exit 1
fi
