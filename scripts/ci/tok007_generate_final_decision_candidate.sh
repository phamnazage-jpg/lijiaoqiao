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
CURRENT_POINTER_PATH="${ROOT_DIR}/${CURRENT_POINTER_FILE}"

SOURCE_FILE="${ROOT_DIR}/review/final_decision_2026-03-31.md"
OUT_FILE="${OUT_DIR}/final_decision_candidate_from_tok007_${TS}.md"
LOG_FILE="${GATE_OUT_DIR}/tok007_generate_candidate_${TS}.log"

latest_tok007_by_name() {
  shopt -s nullglob
  local matches=( "${ROOT_DIR}"/review/outputs/tok007_release_recheck_*.md )
  shopt -u nullglob
  if [[ ${#matches[@]} -eq 0 ]]; then
    return 0
  fi
  printf '%s\n' "${matches[@]}" | sort | tail -n 1
}

tok007_from_current_pointer() {
  if [[ ! -f "${CURRENT_POINTER_PATH}" ]]; then
    return 0
  fi

  local relative_path
  relative_path="$(sed -n 's/^- 当前 TOK-007 复审稿：`\([^`]*\)`/\1/p' "${CURRENT_POINTER_PATH}" | head -n 1 || true)"
  if [[ -z "${relative_path}" || "${relative_path}" == "待生成" ]]; then
    return 0
  fi
  if [[ "${relative_path}" != /* ]]; then
    relative_path="${ROOT_DIR}/${relative_path}"
  fi
  printf '%s\n' "${relative_path}"
}

TOK007_FILE="$(tok007_from_current_pointer)"
if [[ -z "${TOK007_FILE}" ]]; then
  TOK007_FILE="$(latest_tok007_by_name)"
fi

if [[ ! -f "${SOURCE_FILE}" ]]; then
  echo "[FAIL] source final decision missing: ${SOURCE_FILE}" | tee "${LOG_FILE}"
  exit 1
fi
if [[ -z "${TOK007_FILE}" || ! -f "${TOK007_FILE}" ]]; then
  echo "[FAIL] tok007 recheck report missing" | tee "${LOG_FILE}"
  exit 1
fi

DECISION="UNKNOWN"
if grep -q '机判结论：\*\*CONDITIONAL_GO\*\*' "${TOK007_FILE}"; then
  DECISION="CONDITIONAL_GO"
elif grep -q '机判结论：\*\*NO_GO\*\*' "${TOK007_FILE}"; then
  DECISION="NO_GO"
elif grep -q '机判结论：\*\*GO\*\*' "${TOK007_FILE}"; then
  DECISION="GO"
fi

if [[ "${DECISION}" == "UNKNOWN" ]]; then
  echo "[FAIL] cannot parse decision from ${TOK007_FILE}" | tee "${LOG_FILE}"
  exit 1
fi

cp "${SOURCE_FILE}" "${OUT_FILE}"

# keep generated candidate aligned with current gate evidence archive path
sed -i 's#reports/gates#reports/archive/gate_verification#g' "${OUT_FILE}"
sed -i 's#^- 机审稿引用约束：.*#- 机审稿引用约束：活文档只允许引用 `review/outputs/current_machine_review_sources.md` 中声明的现行机审稿；`review/outputs/` 下其余同类机审稿均为历史快照，不得作为当前事实源、签署依据或对外口径。#' "${OUT_FILE}"

# reset three checkboxes
sed -i 's/^- \[x\] GO/- [ ] GO/g' "${OUT_FILE}"
sed -i 's/^- \[x\] CONDITIONAL GO/- [ ] CONDITIONAL GO/g' "${OUT_FILE}"
sed -i 's/^- \[x\] NO-GO/- [ ] NO-GO/g' "${OUT_FILE}"
sed -i 's/^- \[x\] 通过/- [ ] 通过/g' "${OUT_FILE}"
sed -i 's/^- \[x\] 有条件通过/- [ ] 有条件通过/g' "${OUT_FILE}"
sed -i 's/^- \[x\] 不通过/- [ ] 不通过/g' "${OUT_FILE}"

case "${DECISION}" in
  GO)
    sed -i '0,/^- \[ \] GO/s//- [x] GO/' "${OUT_FILE}"
    ;;
  CONDITIONAL_GO)
    sed -i '0,/^- \[ \] CONDITIONAL GO/s//- [x] CONDITIONAL GO/' "${OUT_FILE}"
    ;;
  NO_GO)
    sed -i '0,/^- \[ \] NO-GO/s//- [x] NO-GO/' "${OUT_FILE}"
    ;;
esac

{
  echo
  echo "## 附录：TOK-007 自动复审回填（${TS}）"
  echo
  echo "1. 自动复审来源：\`${TOK007_FILE}\`"
  echo "2. 自动复审结论：\`${DECISION}\`"
  echo "3. 说明：该候选稿用于人工审阅与签署准备，不直接替代正式签署版本。"
  echo "4. 现行机审稿指针：\`${CURRENT_POINTER_FILE}\`"
} >> "${OUT_FILE}"

bash "${MARK_SCRIPT}" --current-tok007 "${TOK007_FILE}" --current-final "${OUT_FILE}" >/dev/null

{
  echo "[INFO] source=${SOURCE_FILE}"
  echo "[INFO] tok007=${TOK007_FILE}"
  echo "[RESULT] decision=${DECISION}"
  echo "[INFO] output=${OUT_FILE}"
  echo "[INFO] current_pointer=${ROOT_DIR}/${CURRENT_POINTER_FILE}"
} | tee "${LOG_FILE}"
