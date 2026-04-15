#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
ENV_FILE="${1:-${SCRIPT_DIR}/.env.minimax-dev}"
OUT_DIR="${ROOT_DIR}/reports/archive/gate_verification"
ART_DIR_BASE="${ROOT_DIR}/tests/supply/artifacts"
TS="$(date +%F_%H%M%S)"

mkdir -p "${OUT_DIR}" "${ART_DIR_BASE}"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "[FAIL] missing env file: ${ENV_FILE}"
  exit 1
fi

# shellcheck disable=SC1090
source "${ENV_FILE}"

require_var() {
  local n="$1"
  if [[ -z "${!n:-}" ]]; then
    echo "[FAIL] missing required env var: ${n}"
    exit 1
  fi
}

require_bin() {
  local b="$1"
  if ! command -v "${b}" >/dev/null 2>&1; then
    echo "[FAIL] missing required binary: ${b}"
    exit 1
  fi
}

join_url() {
  local base="$1"
  local path="$2"
  base="${base%/}"
  if [[ "${path}" != /* ]]; then
    path="/${path}"
  fi
  echo "${base}${path}"
}

classify_http_code() {
  local code="$1"
  case "${code}" in
    200|201|202)
      echo "PASS"
      ;;
    400|422|429)
      echo "PASS_AUTH_REACHED"
      ;;
    401|403)
      echo "FAIL_AUTH"
      ;;
    404|405)
      echo "FAIL_PATH"
      ;;
    000)
      echo "FAIL_NETWORK"
      ;;
    *)
      echo "FAIL_OTHER"
      ;;
  esac
}

require_var API_BASE_URL
require_var OWNER_BEARER_TOKEN
require_bin curl
require_bin jq

MINIMAX_SMOKE_PATH="${MINIMAX_SMOKE_PATH:-/v1/messages}"
MINIMAX_SMOKE_MODEL="${MINIMAX_SMOKE_MODEL:-minimax-smoke-model}"
MINIMAX_TIMEOUT_SECONDS="${MINIMAX_TIMEOUT_SECONDS:-20}"
MINIMAX_SMOKE_DRY_RUN="${MINIMAX_SMOKE_DRY_RUN:-0}"

TARGET_URL="$(join_url "${API_BASE_URL}" "${MINIMAX_SMOKE_PATH}")"
ART_DIR="${ART_DIR_BASE}/minimax_smoke_${TS}"
mkdir -p "${ART_DIR}"

BASE_RESP_FILE="${ART_DIR}/01_base_probe_body.txt"
BASE_ERR_FILE="${ART_DIR}/01_base_probe_stderr.log"
ACTIVE_RESP_FILE="${ART_DIR}/02_active_probe_body.json"
ACTIVE_ERR_FILE="${ART_DIR}/02_active_probe_stderr.log"
REPORT_FILE="${OUT_DIR}/minimax_upstream_smoke_${TS}.md"
LOG_FILE="${OUT_DIR}/minimax_upstream_smoke_${TS}.log"

echo "[INFO] minimax smoke started ts=${TS}" | tee "${LOG_FILE}"
echo "[INFO] env_file=${ENV_FILE}" | tee -a "${LOG_FILE}"
echo "[INFO] api_base_url=${API_BASE_URL}" | tee -a "${LOG_FILE}"
echo "[INFO] target_url=${TARGET_URL}" | tee -a "${LOG_FILE}"
echo "[INFO] dry_run=${MINIMAX_SMOKE_DRY_RUN}" | tee -a "${LOG_FILE}"

if [[ "${MINIMAX_SMOKE_DRY_RUN}" == "1" ]]; then
  {
    echo "# Minimax 上游 Smoke 报告"
    echo
    echo "- 时间戳：${TS}"
    echo "- 执行脚本：\`scripts/supply-gate/minimax_upstream_smoke.sh\`"
    echo "- 环境文件：\`${ENV_FILE}\`"
    echo "- API_BASE_URL：\`${API_BASE_URL}\`"
    echo "- 目标路径：\`${MINIMAX_SMOKE_PATH}\`"
    echo "- 探测 URL：\`${TARGET_URL}\`"
    echo "- 总体结论：**PASS_DRY_RUN**"
    echo
    echo "## 1. 说明"
    echo
    echo "- 本次为 dry-run，未发起任何外部网络请求。"
    echo "- 用于流水联调与产物校验，不可替代真实上游验证证据。"
  } > "${REPORT_FILE}"
  {
    echo "[INFO] report=${REPORT_FILE}"
    echo "[RESULT] PASS_DRY_RUN"
  } | tee -a "${LOG_FILE}"
  exit 0
fi

BASE_HTTP_CODE="000"
BASE_RC=0
BASE_HTTP_CODE="$(curl -sS -m "${MINIMAX_TIMEOUT_SECONDS}" \
  -o "${BASE_RESP_FILE}" \
  -w '%{http_code}' \
  "${API_BASE_URL}" 2>"${BASE_ERR_FILE}")" || BASE_RC=$?

ACTIVE_HTTP_CODE="000"
ACTIVE_RC=0
ACTIVE_PAYLOAD_FILE="${ART_DIR}/02_active_probe_request.json"
jq -n \
  --arg model "${MINIMAX_SMOKE_MODEL}" \
  '{model:$model,max_tokens:1,messages:[{role:"user",content:"ping"}]}' > "${ACTIVE_PAYLOAD_FILE}"

ACTIVE_HTTP_CODE="$(curl -sS -m "${MINIMAX_TIMEOUT_SECONDS}" \
  -o "${ACTIVE_RESP_FILE}" \
  -w '%{http_code}' \
  -X POST "${TARGET_URL}" \
  -H "Authorization: Bearer ${OWNER_BEARER_TOKEN}" \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  --data @"${ACTIVE_PAYLOAD_FILE}" 2>"${ACTIVE_ERR_FILE}")" || ACTIVE_RC=$?

BASE_CLASS="PASS_CONNECTIVITY"
if [[ "${BASE_RC}" -ne 0 ]]; then
  BASE_CLASS="FAIL_NETWORK"
elif [[ "${BASE_HTTP_CODE}" == "000" ]]; then
  BASE_CLASS="FAIL_NETWORK"
fi

ACTIVE_CLASS="FAIL_OTHER"
if [[ "${ACTIVE_RC}" -ne 0 ]]; then
  ACTIVE_CLASS="FAIL_NETWORK"
else
  ACTIVE_CLASS="$(classify_http_code "${ACTIVE_HTTP_CODE}")"
fi

OVERALL="PASS"
if [[ "${BASE_CLASS}" == FAIL_* ]] || [[ "${ACTIVE_CLASS}" == FAIL_* ]]; then
  OVERALL="FAIL"
fi

{
  echo "# Minimax 上游 Smoke 报告"
  echo
  echo "- 时间戳：${TS}"
  echo "- 执行脚本：\`scripts/supply-gate/minimax_upstream_smoke.sh\`"
  echo "- 环境文件：\`${ENV_FILE}\`"
  echo "- API_BASE_URL：\`${API_BASE_URL}\`"
  echo "- 目标路径：\`${MINIMAX_SMOKE_PATH}\`"
  echo "- 探测 URL：\`${TARGET_URL}\`"
  echo "- 总体结论：**${OVERALL}**"
  echo
  echo "## 1. Base 连通探测"
  echo
  echo "- curl rc：${BASE_RC}"
  echo "- http_code：${BASE_HTTP_CODE}"
  echo "- 分类：**${BASE_CLASS}**"
  echo "- 产物：\`${BASE_RESP_FILE}\` / \`${BASE_ERR_FILE}\`"
  echo
  echo "## 2. Active 鉴权探测"
  echo
  echo "- curl rc：${ACTIVE_RC}"
  echo "- http_code：${ACTIVE_HTTP_CODE}"
  echo "- 分类：**${ACTIVE_CLASS}**"
  echo "- 产物：\`${ACTIVE_PAYLOAD_FILE}\` / \`${ACTIVE_RESP_FILE}\` / \`${ACTIVE_ERR_FILE}\`"
  echo
  echo "## 3. 判定规则"
  echo
  echo "1. Base 探测仅判断连通：curl 成功且非 \`000\` 记为 \`PASS_CONNECTIVITY\`。"
  echo "2. Active 探测 \`2xx\` => PASS（请求成功）。"
  echo "3. Active 探测 \`400/422/429\` => PASS_AUTH_REACHED（已到达业务层，通常说明鉴权头被接收）。"
  echo "4. Active 探测 \`401/403\` => FAIL_AUTH（鉴权失败）。"
  echo "5. Active 探测 \`404/405\` => FAIL_PATH（路径或方法不匹配）。"
  echo "6. 任一探测 \`000\` 或 curl 非零 => FAIL_NETWORK（网络/解析/连接失败）。"
} > "${REPORT_FILE}"

{
  echo "[INFO] report=${REPORT_FILE}"
  echo "[INFO] base_http=${BASE_HTTP_CODE} active_http=${ACTIVE_HTTP_CODE}"
  echo "[RESULT] ${OVERALL}"
} | tee -a "${LOG_FILE}"

if [[ "${OVERALL}" == "FAIL" ]]; then
  exit 1
fi
