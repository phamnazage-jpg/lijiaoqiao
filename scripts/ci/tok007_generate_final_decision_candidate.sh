#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
TS="$(date +%F_%H%M%S)"
OUT_DIR="${ROOT_DIR}/review/outputs"
mkdir -p "${OUT_DIR}"

SOURCE_FILE="${ROOT_DIR}/review/final_decision_2026-03-31.md"
TOK007_FILE="$(ls -1t ${ROOT_DIR}/review/outputs/tok007_release_recheck_*.md 2>/dev/null | head -n 1 || true)"
OUT_FILE="${OUT_DIR}/final_decision_candidate_from_tok007_${TS}.md"
LOG_FILE="${ROOT_DIR}/reports/gates/tok007_generate_candidate_${TS}.log"

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
} >> "${OUT_FILE}"

{
  echo "[INFO] source=${SOURCE_FILE}"
  echo "[INFO] tok007=${TOK007_FILE}"
  echo "[RESULT] decision=${DECISION}"
  echo "[INFO] output=${OUT_FILE}"
} | tee "${LOG_FILE}"

