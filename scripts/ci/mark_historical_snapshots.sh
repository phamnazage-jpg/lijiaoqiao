#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
OUT_DIR="${ROOT_DIR}/review/outputs"
CURRENT_POINTER_FILE="${OUT_DIR}/current_machine_review_sources.md"

CURRENT_TOK007=""
CURRENT_FINAL=""

usage() {
  cat <<'EOF'
Usage:
  bash scripts/ci/mark_historical_snapshots.sh [--current-tok007 PATH] [--current-final PATH]

Description:
  1. 维护 review/outputs/current_machine_review_sources.md 现行机审稿指针。
  2. 将 review/outputs/ 下非现行的 tok007 / final candidate 文件统一标记为历史快照。
EOF
}

resolve_path() {
  local path="$1"
  if [[ -z "${path}" ]]; then
    return 0
  fi
  if [[ "${path}" != /* ]]; then
    path="${ROOT_DIR}/${path}"
  fi
  printf '%s\n' "${path}"
}

latest_matching() {
  local pattern="$1"
  shopt -s nullglob
  local matches=( ${pattern} )
  shopt -u nullglob
  if [[ ${#matches[@]} -eq 0 ]]; then
    return 0
  fi
  printf '%s\n' "${matches[@]}" | sort | tail -n 1
}

to_relative_path() {
  local path="$1"
  if [[ "${path}" == "${ROOT_DIR}/"* ]]; then
    printf '%s\n' "${path#"${ROOT_DIR}/"}"
    return
  fi
  printf '%s\n' "${path}"
}

strip_archive_header() {
  local file="$1"
  if head -n 1 "${file}" | grep -q '^> 归档状态：已归档'; then
    tail -n +6 "${file}" | sed '/./,$!d'
    return
  fi
  cat "${file}"
}

write_current_file() {
  local file="$1"
  if [[ -z "${file}" || ! -f "${file}" ]]; then
    return
  fi

  if ! head -n 1 "${file}" | grep -q '^> 归档状态：已归档'; then
    return
  fi

  local tmp_file
  tmp_file="$(mktemp)"
  strip_archive_header "${file}" > "${tmp_file}"
  mv "${tmp_file}" "${file}"
}

write_historical_file() {
  local file="$1"
  local archive_id="$2"
  local tmp_file
  tmp_file="$(mktemp)"

  {
    echo "> 归档状态：已归档"
    echo "> 归档批次：ARCHIVE-HISTORY-AUTO"
    echo "> 归档标识：${archive_id}"
    echo "> 当前用途：历史机审快照，仅供追溯；不得作为现行门禁、发布决议或实现状态事实源。"
    echo "> 当前事实源：\`review/outputs/current_machine_review_sources.md\`、\`docs/plans/2026-04-14-repo-integrity-refactor-plan.md\` 与 \`reports/archive/gate_verification/\`。"
    echo
    strip_archive_header "${file}"
  } > "${tmp_file}"

  mv "${tmp_file}" "${file}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --current-tok007)
      CURRENT_TOK007="${2:-}"
      shift 2
      ;;
    --current-final)
      CURRENT_FINAL="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "[FAIL] unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

mkdir -p "${OUT_DIR}"

CURRENT_TOK007="$(resolve_path "${CURRENT_TOK007}")"
CURRENT_FINAL="$(resolve_path "${CURRENT_FINAL}")"

if [[ -z "${CURRENT_TOK007}" ]]; then
  CURRENT_TOK007="$(latest_matching "${OUT_DIR}/tok007_release_recheck_*.md")"
fi
if [[ -z "${CURRENT_FINAL}" ]]; then
  CURRENT_FINAL="$(latest_matching "${OUT_DIR}/final_decision_candidate_from_tok007_*.md")"
fi

if [[ -n "${CURRENT_TOK007}" && ! -f "${CURRENT_TOK007}" ]]; then
  echo "[FAIL] current tok007 file missing: ${CURRENT_TOK007}" >&2
  exit 1
fi
if [[ -n "${CURRENT_FINAL}" && ! -f "${CURRENT_FINAL}" ]]; then
  echo "[FAIL] current final candidate missing: ${CURRENT_FINAL}" >&2
  exit 1
fi
if [[ -z "${CURRENT_TOK007}" && -z "${CURRENT_FINAL}" ]]; then
  echo "[FAIL] no machine-review source files found under ${OUT_DIR}" >&2
  exit 1
fi

CURRENT_TOK007_REL="待生成"
CURRENT_FINAL_REL="待生成"
if [[ -n "${CURRENT_TOK007}" ]]; then
  CURRENT_TOK007_REL="$(to_relative_path "${CURRENT_TOK007}")"
fi
if [[ -n "${CURRENT_FINAL}" ]]; then
  CURRENT_FINAL_REL="$(to_relative_path "${CURRENT_FINAL}")"
fi

{
  echo "# 现行机审稿指针"
  echo
  echo "- 生成时间：$(date +%F_%H%M%S)"
  echo "- 维护脚本：\`scripts/ci/mark_historical_snapshots.sh\`"
  echo "- 活文档引用规则：只允许引用本文件列出的现行机审稿；\`review/outputs/\` 下其余 \`tok007_release_recheck_*\` / \`final_decision_candidate_from_tok007_*\` 一律按历史快照处理。"
  echo "- 当前 TOK-007 复审稿：\`${CURRENT_TOK007_REL}\`"
  echo "- 当前最终决议候选稿：\`${CURRENT_FINAL_REL}\`"
} > "${CURRENT_POINTER_FILE}"

write_current_file "${CURRENT_TOK007}"
write_current_file "${CURRENT_FINAL}"

archive_index=1
shopt -s nullglob
snapshot_files=( "${OUT_DIR}"/tok007_release_recheck_*.md "${OUT_DIR}"/final_decision_candidate_from_tok007_*.md )
shopt -u nullglob

if [[ ${#snapshot_files[@]} -gt 0 ]]; then
  mapfile -t snapshot_files < <(printf '%s\n' "${snapshot_files[@]}" | sort)
fi

for file in "${snapshot_files[@]}"; do
  if [[ "${file}" == "${CURRENT_TOK007}" || "${file}" == "${CURRENT_FINAL}" ]]; then
    continue
  fi
  write_historical_file "${file}" "$(printf 'AHR-AUTO-%03d' "${archive_index}")"
  archive_index=$((archive_index + 1))
done

echo "[INFO] current pointer updated: ${CURRENT_POINTER_FILE}"
echo "[INFO] current tok007: ${CURRENT_TOK007_REL}"
echo "[INFO] current final candidate: ${CURRENT_FINAL_REL}"
