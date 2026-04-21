#!/usr/bin/env bash
# shellcheck disable=SC1091
# scripts/ci/lib/manifest_lib.sh
# Staging release manifest 生成和消费公共库
# 所有 staging pipeline 脚本应使用此库管理 manifest.json
set -euo pipefail

# 默认值（ROOT_DIR 必须在 MANIFEST_DIR 之前先解析）
_root_dir="${ROOT_DIR:-$(cd "$(dirname "$0")/../../.." && pwd)}"
MANIFEST_DIR="${MANIFEST_DIR:-${_root_dir}/reports/releases}"
unset _root_dir
RUN_ID="${RUN_ID:-$(date +%Y%m%d_%H%M%S)_$$}"
MANIFEST_FILE="${MANIFEST_DIR}/${RUN_ID}/manifest.json"

# ──────────────────────────────────────────────────────────────
# 生成 manifest.json
# 用法: manifest_generate [--run-id <id>] [--staging] [--prod]
# ──────────────────────────────────────────────────────────────
manifest_generate() {
  local env="staging"
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --run-id) RUN_ID="$2"; shift 2 ;;
      --staging) env="staging"; shift ;;
      --prod) env="prod"; shift ;;
      *) shift ;;
    esac
  done

  MANIFEST_FILE="${MANIFEST_DIR}/${RUN_ID}/manifest.json"

  mkdir -p "$(dirname "${MANIFEST_FILE}")"

  # 生成时间戳
  local ts
  ts="$(date -Iseconds)"

  # 基础 manifest 结构
  cat > "${MANIFEST_FILE}" <<EOF
{
  "version": "1.0",
  "run_id": "${RUN_ID}",
  "environment": "${env}",
  "created_at": "${ts}",
  "decision_inputs": {},
  "artifact_paths": {},
  "smoke_results": {},
  "contract_results": {}
}
EOF

  echo "[MANIFEST] Generated: ${MANIFEST_FILE}"
}

# ──────────────────────────────────────────────────────────────
# 向 manifest 写入 key=value（支持嵌套路径）
# 用法: manifest_set "decision_inputs.stage_validation" "GO"
#       manifest_set "artifact_paths.backend_verify" "/path/to/report.md"
# ──────────────────────────────────────────────────────────────
manifest_set() {
  local key="$1"
  local value="$2"
  local file="${3:-${MANIFEST_FILE}}"

  if [[ ! -f "${file}" ]]; then
    echo "[WARN] manifest_set: ${file} not found, skipping" >&2
    return 1
  fi

  # 将 dot-notation 路径转为 jq 路径
  local jq_path
  jq_path="$(echo "${key}" | sed 's/\./|/g' | tr '|' '.')"

  local tmp
  tmp="$(mktemp)"

  if ! jq --arg v "${value}" \
     "setpath(\"${jq_path}\" | split(\".\"); \$v)" \
     "${file}" > "${tmp}"; then
    echo "[WARN] manifest_set: jq failed for key=${key} value=${value}" >&2
    rm -f "${tmp}"
    return 1
  fi

  mv "${tmp}" "${file}"
  echo "[MANIFEST] set ${key}=${value}"
}

# ──────────────────────────────────────────────────────────────
# 从 manifest 读取 value
# 用法: manifest_get "decision_inputs.stage_validation"
#       返回值写入 stdout
# ──────────────────────────────────────────────────────────────
manifest_get() {
  local key="$1"
  local file="${2:-${MANIFEST_FILE}}"

  if [[ ! -f "${file}" ]]; then
    echo ""
    return
  fi

  local jq_path
  jq_path="$(echo "${key}" | sed 's/\./|/g' | tr '|' '.')"

  jq -r "getpath(\"${jq_path}\" | split(\".\")) // \"\" " "${file}" 2>/dev/null || true
}

# ──────────────────────────────────────────────────────────────
# 验证 manifest 完整性
# 返回 0 = 有效，1 = 无效
# 用法: manifest_validate "${MANIFEST_FILE}" || exit 1
# ──────────────────────────────────────────────────────────────
manifest_validate() {
  local file="${1:-${MANIFEST_FILE}}"

  if [[ ! -f "${file}" ]]; then
    echo "[FAIL] manifest_validate: ${file} does not exist" >&2
    return 1
  fi

  # 基础字段检查
  if ! jq -e '.run_id != "" and .environment != "" and .created_at != ""' "${file}" > /dev/null 2>&1; then
    echo "[FAIL] manifest_validate: missing required fields (run_id/environment/created_at)" >&2
    return 1
  fi

  echo "[MANIFEST] validate OK: ${file}"
  return 0
}

# ──────────────────────────────────────────────────────────────
# 运行 backend-verify 并将结果写入 manifest
# 用法: manifest_run_backend_verify [--manifest-file <path>]
# ──────────────────────────────────────────────────────────────
manifest_run_backend_verify() {
  local manifest_file="${MANIFEST_FILE}"
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --manifest-file) manifest_file="$2"; shift 2 ;;
      *) shift ;;
    esac
  done

  local bv_log="${OUT_DIR:-/tmp}/backend_verify_$(date +%F_%H%M%S).log"
  local bv_report="${OUT_DIR:-/tmp}/backend_verify_$(date +%F_%H%M%S).md"

  if bash "${ROOT_DIR}/scripts/ci/backend-verify.sh" \
    > >(tee "${bv_log}") 2>&1; then
    manifest_set "artifact_paths.backend_verify" "${bv_report}" "${manifest_file}"
    manifest_set "contract_results.backend_verify" "PASS" "${manifest_file}"
    echo "[MANIFEST] backend_verify: PASS"
  else
    manifest_set "artifact_paths.backend_verify" "${bv_log}" "${manifest_file}"
    manifest_set "contract_results.backend_verify" "FAIL" "${manifest_file}"
    echo "[MANIFEST] backend_verify: FAIL"
    return 1
  fi
}

# ──────────────────────────────────────────────────────────────
# 运行 contract gate 并将结果写入 manifest
# 用法: manifest_run_contract_gate [--manifest-file <path>]
# ──────────────────────────────────────────────────────────────
manifest_run_contract_gate() {
  local manifest_file="${MANIFEST_FILE}"
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --manifest-file) manifest_file="$2"; shift 2 ;;
      *) shift ;;
    esac
  done

  local cg_log="${OUT_DIR:-/tmp}/contract_gate_$(date +%F_%H%M%S).log"
  local cg_report="${OUT_DIR:-/tmp}/contract_gate_$(date +%F_%H%M%S).md"

  if bash "${ROOT_DIR}/scripts/ci/backend-verify.sh" --phase1-contract-gate \
    > >(tee "${cg_log}") 2>&1; then
    manifest_set "artifact_paths.contract_gate" "${cg_report}" "${manifest_file}"
    manifest_set "contract_results.contract_gate" "PASS" "${manifest_file}"
    echo "[MANIFEST] contract_gate: PASS"
  else
    manifest_set "artifact_paths.contract_gate" "${cg_log}" "${manifest_file}"
    manifest_set "contract_results.contract_gate" "FAIL" "${manifest_file}"
    echo "[MANIFEST] contract_gate: FAIL"
    return 1
  fi
}

# ──────────────────────────────────────────────────────────────
# 运行 superpowers_stage_validate 并将结果写入 manifest
# 用法: manifest_run_stage_validation [--manifest-file <path>]
# ──────────────────────────────────────────────────────────────
manifest_run_stage_validation() {
  local manifest_file="${MANIFEST_FILE}"
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --manifest-file) manifest_file="$2"; shift 2 ;;
      *) shift ;;
    esac
  done

  local sp_log="${OUT_DIR:-/tmp}/superpowers_stage_validation_$(date +%F_%H%M%S).log"
  local sp_report="${OUT_DIR:-/tmp}/superpowers_stage_validation_$(date +%F_%H%M%S).md"

  if bash "${ROOT_DIR}/scripts/ci/superpowers_stage_validate.sh" \
    > >(tee "${sp_log}") 2>&1; then
    manifest_set "decision_inputs.stage_validation" "PASS" "${manifest_file}"
    manifest_set "artifact_paths.stage_validation" "${sp_report}" "${manifest_file}"
    echo "[MANIFEST] stage_validation: PASS"
  else
    manifest_set "decision_inputs.stage_validation" "FAIL" "${manifest_file}"
    manifest_set "artifact_paths.stage_validation" "${sp_report}" "${manifest_file}"
    echo "[MANIFEST] stage_validation: FAIL"
    return 1
  fi
}

# ──────────────────────────────────────────────────────────────
# 检查 manifest 中的 run_id 是否非空（硬门禁）
# 用法: manifest_hard_gate_run_id [--manifest-file <path>]
# ──────────────────────────────────────────────────────────────
manifest_hard_gate_run_id() {
  local file="${1:-${MANIFEST_FILE}}"

  local run_id
  run_id="$(manifest_get "run_id" "${file}")"

  if [[ -z "${run_id}" ]]; then
    echo "[GATE FAIL] run_id is empty in manifest — hard gate blocked" >&2
    echo "[GATE FAIL] manifest: ${file}" >&2
    return 1
  fi

  echo "[GATE OK] run_id=${run_id}"
  return 0
}

# ──────────────────────────────────────────────────────────────
# 打印 manifest 摘要
# 用法: manifest_summary [--manifest-file <path>]
# ──────────────────────────────────────────────────────────────
manifest_summary() {
  local file="${1:-${MANIFEST_FILE}}"

  if [[ ! -f "${file}" ]]; then
    echo "[WARN] manifest not found: ${file}"
    return
  fi

  echo "=== Manifest: ${file} ==="
  jq -r '
    to_entries | .[] |
    if .value | type == "object" then
      "\(.key):"
    elif .value | type == "array" then
      "\(.key): \(.value | join(", "))"
    else
      "\(.key): \(.value}"
    end
  ' "${file}" 2>/dev/null || cat "${file}"
}
