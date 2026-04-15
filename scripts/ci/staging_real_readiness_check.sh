#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
ENV_REL="${1:-scripts/supply-gate/.env.staging-real}"
if [[ "${ENV_REL}" == /* ]]; then
  ENV_FILE="${ENV_REL}"
else
  ENV_FILE="${ROOT_DIR}/${ENV_REL}"
fi

OUT_DIR="${ROOT_DIR}/reports/archive/gate_verification"
mkdir -p "${OUT_DIR}"
TS="$(date +%F_%H%M%S)"
REPORT_FILE="${OUT_DIR}/staging_real_readiness_${TS}.md"
LOG_FILE="${OUT_DIR}/staging_real_readiness_${TS}.log"

CHECK_IDS=()
CHECK_STATUS=()
CHECK_DESC=()
CHECK_EVIDENCE=()

add_check() {
  CHECK_IDS+=("$1")
  CHECK_STATUS+=("$2")
  CHECK_DESC+=("$3")
  CHECK_EVIDENCE+=("$4")
}

log() {
  echo "$1" | tee -a "${LOG_FILE}" >/dev/null
}

if [[ ! -f "${ENV_FILE}" ]]; then
  add_check "STG-RDY-001" "FAIL" "环境文件存在" "${ENV_FILE} (missing)"
else
  add_check "STG-RDY-001" "PASS" "环境文件存在" "${ENV_FILE}"
fi

if [[ ! -f "${ENV_FILE}" ]]; then
  {
    echo "# 真实 STG 就绪度检查"
    echo
    echo "- 时间戳：${TS}"
    echo "- 输入环境：\`${ENV_REL}\`"
    echo "- 环境分类：\`${ENV_CLASS}\`"
    echo "- 结果：**BLOCKED**"
    echo
    echo "| 检查项 | 结果 | 说明 | 证据 |"
    echo "|---|---|---|---|"
    for i in "${!CHECK_IDS[@]}"; do
      echo "| ${CHECK_IDS[$i]} | ${CHECK_STATUS[$i]} | ${CHECK_DESC[$i]} | ${CHECK_EVIDENCE[$i]} |"
    done
  } > "${REPORT_FILE}"
  echo "[RESULT] BLOCKED" | tee -a "${LOG_FILE}" >/dev/null
  echo "[INFO] report=${REPORT_FILE}"
  exit 1
fi

# shellcheck disable=SC1090
source "${ENV_FILE}"

API_BASE_URL_VALUE="${API_BASE_URL:-}"
OWNER_TOKEN_VALUE="${OWNER_BEARER_TOKEN:-}"
VIEWER_TOKEN_VALUE="${VIEWER_BEARER_TOKEN:-}"
ADMIN_TOKEN_VALUE="${ADMIN_BEARER_TOKEN:-}"

classify_env() {
  if [[ "${ENV_FILE}" == *".env.local-mock"* ]]; then
    echo "local-mock"
    return
  fi

  if [[ -z "${API_BASE_URL_VALUE}" || "${API_BASE_URL_VALUE}" == *"staging.example.com"* ]]; then
    echo "placeholder"
    return
  fi

  if echo "${API_BASE_URL_VALUE}" | grep -Eiq '127\.0\.0\.1|localhost'; then
    echo "local-mock"
    return
  fi

  for token in "${OWNER_TOKEN_VALUE}" "${VIEWER_TOKEN_VALUE}" "${ADMIN_TOKEN_VALUE}"; do
    if [[ -z "${token}" || "${token}" == replace-me-* || "${token}" == placeholder* ]]; then
      echo "placeholder"
      return
    fi
  done

  echo "real-staging"
}

ENV_CLASS="$(classify_env)"
log "[INFO] env_class=${ENV_CLASS}"

if [[ -n "${API_BASE_URL_VALUE}" ]]; then
  add_check "STG-RDY-002" "PASS" "API_BASE_URL 已配置" "${API_BASE_URL_VALUE}"
else
  add_check "STG-RDY-002" "FAIL" "API_BASE_URL 已配置" "empty"
fi

if [[ "${API_BASE_URL_VALUE}" == *"staging.example.com"* ]]; then
  add_check "STG-RDY-003" "FAIL" "API_BASE_URL 非占位值" "${API_BASE_URL_VALUE}"
elif [[ -n "${API_BASE_URL_VALUE}" ]]; then
  add_check "STG-RDY-003" "PASS" "API_BASE_URL 非占位值" "${API_BASE_URL_VALUE}"
else
  add_check "STG-RDY-003" "FAIL" "API_BASE_URL 非占位值" "empty"
fi

if echo "${API_BASE_URL_VALUE}" | grep -Eiq '127\.0\.0\.1|localhost'; then
  add_check "STG-RDY-004" "FAIL" "API_BASE_URL 为真实外网 STG 地址" "${API_BASE_URL_VALUE} (local)"
else
  add_check "STG-RDY-004" "PASS" "API_BASE_URL 为真实外网 STG 地址" "${API_BASE_URL_VALUE}"
fi

if [[ -n "${OWNER_TOKEN_VALUE}" && -n "${VIEWER_TOKEN_VALUE}" && -n "${ADMIN_TOKEN_VALUE}" ]]; then
  add_check "STG-RDY-005" "PASS" "owner/viewer/admin token 已配置" "all present"
else
  add_check "STG-RDY-005" "FAIL" "owner/viewer/admin token 已配置" "missing one or more token"
fi

has_placeholder=0
for t in "${OWNER_TOKEN_VALUE}" "${VIEWER_TOKEN_VALUE}" "${ADMIN_TOKEN_VALUE}"; do
  if [[ "${t}" == replace-me-* || "${t}" == placeholder* || -z "${t}" ]]; then
    has_placeholder=1
    break
  fi
done
if [[ "${has_placeholder}" == "1" ]]; then
  add_check "STG-RDY-006" "FAIL" "token 非占位值" "placeholder/empty detected"
else
  add_check "STG-RDY-006" "PASS" "token 非占位值" "ok"
fi

if [[ "${OWNER_TOKEN_VALUE}" == "${VIEWER_TOKEN_VALUE}" || "${OWNER_TOKEN_VALUE}" == "${ADMIN_TOKEN_VALUE}" || "${VIEWER_TOKEN_VALUE}" == "${ADMIN_TOKEN_VALUE}" ]]; then
  add_check "STG-RDY-007" "WARN" "三类 token 建议区分角色" "at least two tokens are identical"
else
  add_check "STG-RDY-007" "PASS" "三类 token 建议区分角色" "distinct tokens"
fi

reachable_status="000"
if [[ -n "${API_BASE_URL_VALUE}" ]]; then
  reachable_status="$(curl -sS -m 5 -o /dev/null -w "%{http_code}" -I "${API_BASE_URL_VALUE}" 2>/dev/null || true)"
fi
if [[ "${reachable_status}" == "000" ]]; then
  add_check "STG-RDY-008" "FAIL" "API_BASE_URL 可达性" "http_code=000"
else
  add_check "STG-RDY-008" "PASS" "API_BASE_URL 可达性" "http_code=${reachable_status}"
fi

has_fail=0
for s in "${CHECK_STATUS[@]}"; do
  if [[ "${s}" == "FAIL" ]]; then
    has_fail=1
    break
  fi
done

RESULT="READY"
NOTE="all required checks passed"
if [[ "${has_fail}" == "1" ]]; then
  RESULT="BLOCKED"
  NOTE="at least one required check failed"
fi

{
  echo "# 真实 STG 就绪度检查"
  echo
  echo "- 时间戳：${TS}"
  echo "- 输入环境：\`${ENV_REL}\`"
  echo "- 环境分类：\`${ENV_CLASS}\`"
  echo "- 结果：**${RESULT}**"
  echo "- 说明：${NOTE}"
  echo
  echo "| 检查项 | 结果 | 说明 | 证据 |"
  echo "|---|---|---|---|"
  for i in "${!CHECK_IDS[@]}"; do
    echo "| ${CHECK_IDS[$i]} | ${CHECK_STATUS[$i]} | ${CHECK_DESC[$i]} | ${CHECK_EVIDENCE[$i]} |"
  done
  echo
  echo "## 结论"
  echo
  echo "1. 该检查用于判定“是否具备真实 STG 放行验证前提”。"
  echo "2. 若结果为 BLOCKED，不应执行真实放行口径判定。"
} > "${REPORT_FILE}"

echo "[INFO] report=${REPORT_FILE}" | tee -a "${LOG_FILE}" >/dev/null
echo "[RESULT] ${RESULT}" | tee -a "${LOG_FILE}" >/dev/null

if [[ "${RESULT}" != "READY" ]]; then
  exit 1
fi
