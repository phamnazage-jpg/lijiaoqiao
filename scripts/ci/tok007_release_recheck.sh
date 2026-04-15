#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
TS="$(date +%F_%H%M%S)"
OUT_DIR="${ROOT_DIR}/review/outputs"
mkdir -p "${OUT_DIR}"
GATE_OUT_DIR="${ROOT_DIR}/reports/archive/gate_verification"
mkdir -p "${GATE_OUT_DIR}"
MARK_SCRIPT="${ROOT_DIR}/scripts/ci/mark_historical_snapshots.sh"
CURRENT_POINTER_FILE="review/outputs/current_machine_review_sources.md"
OUT_FILE="${OUT_DIR}/tok007_release_recheck_${TS}.md"
LOG_FILE="${GATE_OUT_DIR}/tok007_release_recheck_${TS}.log"

log() {
  echo "$1" | tee -a "${LOG_FILE}"
}

latest_file_or_empty() {
  local pattern="$1"
  local latest
  latest="$(ls -1t ${pattern} 2>/dev/null | head -n 1 || true)"
  echo "${latest}"
}

extract_md_checkbox_conclusion() {
  local file="$1"
  local go="0"
  local cgo="0"
  local nogo="0"
  if [[ ! -f "${file}" ]]; then
    echo "UNKNOWN"
    return
  fi
  grep -Eq '^- \[x\] (通过|GO)' "${file}" && go="1" || true
  grep -Eq '^- \[x\] (有条件通过|CONDITIONAL GO)' "${file}" && cgo="1" || true
  grep -Eq '^- \[x\] (不通过|NO-GO)' "${file}" && nogo="1" || true

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

extract_bold_decision() {
  local file="$1"
  if [[ ! -f "${file}" ]]; then
    echo "UNKNOWN"
    return
  fi
  local row
  row="$(grep -E '^- (决策|判定)：\*\*' "${file}" | head -n 1 || true)"
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

extract_superpowers_decision() {
  local file="$1"
  extract_bold_decision "${file}"
}

extract_pass_fail_result() {
  local file="$1"
  if [[ ! -f "${file}" ]]; then
    echo "UNKNOWN"
    return
  fi
  if grep -Eq '^- 结果：\*\*PASS\*\*' "${file}"; then
    echo "PASS"
    return
  fi
  if grep -Eq '^- 结果：\*\*FAIL\*\*' "${file}"; then
    echo "FAIL"
    return
  fi
  echo "UNKNOWN"
}

TOK006_REPORT="$(latest_file_or_empty "${GATE_OUT_DIR}/tok006_gate_bundle_*.md")"
SP_REPORT="$(latest_file_or_empty "${GATE_OUT_DIR}/superpowers_stage_validation_*.md")"
TOK_RUNTIME_READINESS_REPORT="$(latest_file_or_empty "${GATE_OUT_DIR}/token_runtime_readiness_*.md")"
SUP_REVIEW_REPORT="${ROOT_DIR}/reports/supply_gate_review_2026-03-31.md"
FINAL_DECISION_REPORT="${ROOT_DIR}/review/final_decision_2026-03-31.md"

TOK006_DECISION="$(extract_bold_decision "${TOK006_REPORT}")"
SP_DECISION="$(extract_superpowers_decision "${SP_REPORT}")"
TOK_RUNTIME_READINESS_RESULT="$(extract_pass_fail_result "${TOK_RUNTIME_READINESS_REPORT}")"
SUP_DECISION="$(extract_md_checkbox_conclusion "${SUP_REVIEW_REPORT}")"
FINAL_DECISION_CURRENT="$(extract_md_checkbox_conclusion "${FINAL_DECISION_REPORT}")"

has_unknown=0
if [[ "${TOK006_DECISION}" == "UNKNOWN" || "${SP_DECISION}" == "UNKNOWN" || "${TOK_RUNTIME_READINESS_RESULT}" == "UNKNOWN" || "${SUP_DECISION}" == "UNKNOWN" ]]; then
  has_unknown=1
fi

DECISION="CONDITIONAL_GO"
DECISION_REASON="all available checks are non-failing but at least one source is conditional/mock/deferred"
if [[ "${TOK006_DECISION}" == "NO_GO" || "${SP_DECISION}" == "NO_GO" || "${TOK_RUNTIME_READINESS_RESULT}" == "FAIL" || "${SUP_DECISION}" == "NO_GO" ]]; then
  DECISION="NO_GO"
  DECISION_REASON="at least one upstream gate is NO_GO"
elif [[ "${TOK006_DECISION}" == "GO" && "${SP_DECISION}" == "GO" && "${TOK_RUNTIME_READINESS_RESULT}" == "PASS" && "${SUP_DECISION}" == "GO" ]]; then
  DECISION="GO"
  DECISION_REASON="all upstream gates report GO"
elif [[ "${has_unknown}" -eq 1 ]]; then
  DECISION="NO_GO"
  DECISION_REASON="missing/unknown upstream decision source"
fi

RECOMMEND_ACTION_1="补齐真实 staging 参数后执行 scripts/supply-gate/staging_precheck_and_run.sh"
RECOMMEND_ACTION_2="重跑 scripts/ci/superpowers_stage_validate.sh 并确认 PHASE-07=PASS"
RECOMMEND_ACTION_3="更新 reports/supply_gate_review_2026-03-31.md 与 review/final_decision_2026-03-31.md 签署页"

cat > "${OUT_FILE}" <<EOF
# TOK-007 发布门禁复审报告

- 时间戳：${TS}
- 生成脚本：\`scripts/ci/tok007_release_recheck.sh\`
- 现行机审稿指针：\`${CURRENT_POINTER_FILE}\`

## 1. 输入证据

| 来源 | 路径 | 判定 |
|---|---|---|
| TOK-006 Gate 汇总 | ${TOK006_REPORT:-N/A} | ${TOK006_DECISION} |
| Superpowers 阶段验证 | ${SP_REPORT:-N/A} | ${SP_DECISION} |
| Token Runtime Readiness (M-021) | ${TOK_RUNTIME_READINESS_REPORT:-N/A} | ${TOK_RUNTIME_READINESS_RESULT} |
| SUP Gate 汇总评审 | ${SUP_REVIEW_REPORT} | ${SUP_DECISION} |
| 当前最终决议文档 | ${FINAL_DECISION_REPORT} | ${FINAL_DECISION_CURRENT} |

## 2. 复审结论

- [ ] GO
- [ ] CONDITIONAL GO
- [ ] NO-GO

- 机判结论：**${DECISION}**
- 结论依据：${DECISION_REASON}

## 3. 状态建议

1. ${RECOMMEND_ACTION_1}
2. ${RECOMMEND_ACTION_2}
3. ${RECOMMEND_ACTION_3}
EOF

case "${DECISION}" in
  GO)
    sed -i 's/^- \[ \] GO/- [x] GO/' "${OUT_FILE}"
    ;;
  CONDITIONAL_GO)
    sed -i 's/^- \[ \] CONDITIONAL GO/- [x] CONDITIONAL GO/' "${OUT_FILE}"
    ;;
  NO_GO)
    sed -i 's/^- \[ \] NO-GO/- [x] NO-GO/' "${OUT_FILE}"
    ;;
esac

bash "${MARK_SCRIPT}" --current-tok007 "${OUT_FILE}" >/dev/null

log "[INFO] TOK006=${TOK006_DECISION}, SP=${SP_DECISION}, M021=${TOK_RUNTIME_READINESS_RESULT}, SUP=${SUP_DECISION}, FINAL_CURRENT=${FINAL_DECISION_CURRENT}"
log "[RESULT] ${DECISION}"
log "[INFO] output=${OUT_FILE}"
log "[INFO] current_pointer=${ROOT_DIR}/${CURRENT_POINTER_FILE}"

if [[ "${DECISION}" == "NO_GO" ]]; then
  exit 1
fi
