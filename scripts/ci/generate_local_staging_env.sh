#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
ENV_REL="${1:-scripts/supply-gate/.env.staging-real}"
if [[ "${ENV_REL}" == /* ]]; then
  ENV_PATH="${ENV_REL}"
else
  ENV_PATH="${ROOT_DIR}/${ENV_REL}"
fi

API_BASE_URL_VALUE="${API_BASE_URL_VALUE:-http://127.0.0.1:18080}"
TOKEN_RUNTIME_URL="${TOKEN_RUNTIME_URL:-http://127.0.0.1:18081}"
TOKEN_TTL_SECONDS="${TOKEN_TTL_SECONDS:-7200}"
TOKEN_SUBJECT_PREFIX="${TOKEN_SUBJECT_PREFIX:-local-staging-real}"
START_RUNTIME_IF_NEEDED="${START_RUNTIME_IF_NEEDED:-1}"

OUT_DIR="${ROOT_DIR}/reports/archive/gate_verification"
mkdir -p "${OUT_DIR}"
TS="$(date +%F_%H%M%S)"
RUNTIME_LOG="${OUT_DIR}/local_token_runtime_generate_env_${TS}.log"
REPORT_FILE="${OUT_DIR}/local_staging_env_generation_${TS}.md"

RUNTIME_STARTED_BY_SCRIPT=0
RUNTIME_PID=""

require_bin() {
  local b="$1"
  if ! command -v "${b}" >/dev/null 2>&1; then
    echo "[FAIL] missing required binary: ${b}"
    exit 1
  fi
}

require_bin curl
require_bin jq
require_bin date
require_bin ss
require_bin awk
require_bin sed
require_bin sha256sum

is_http_ready() {
  local url="$1"
  curl -sS -m 1 "${url}/actuator/health" 2>/dev/null | grep -q '"UP"'
}

is_port_in_use() {
  local port="$1"
  ss -ltn | awk '{print $4}' | grep -Eq "[:.]${port}$"
}

pick_free_port() {
  local base="${1:-18091}"
  local max_tries="${2:-80}"
  local p="${base}"
  local i=0
  while [[ "${i}" -lt "${max_tries}" ]]; do
    if ! is_port_in_use "${p}"; then
      echo "${p}"
      return 0
    fi
    p=$((p + 1))
    i=$((i + 1))
  done
  return 1
}

cleanup() {
  if [[ "${RUNTIME_STARTED_BY_SCRIPT}" == "1" && -n "${RUNTIME_PID}" ]]; then
    kill "${RUNTIME_PID}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

ensure_runtime() {
  if is_http_ready "${TOKEN_RUNTIME_URL}"; then
    return 0
  fi

  if [[ "${START_RUNTIME_IF_NEEDED}" != "1" ]]; then
    echo "[FAIL] token runtime not ready: ${TOKEN_RUNTIME_URL}"
    echo "[HINT] set START_RUNTIME_IF_NEEDED=1 or start token runtime manually"
    exit 1
  fi

  local go_bin="${ROOT_DIR}/.tools/go-current/bin/go"
  if [[ ! -x "${go_bin}" ]]; then
    go_bin="$(command -v go || true)"
  fi
  if [[ -z "${go_bin}" ]]; then
    echo "[FAIL] go binary not found; cannot start local token runtime"
    exit 1
  fi

  local port
  if ! port="$(pick_free_port 18091 80)"; then
    echo "[FAIL] no free port found for temporary token runtime"
    exit 1
  fi

  TOKEN_RUNTIME_URL="http://127.0.0.1:${port}"
  (
    cd "${ROOT_DIR}/platform-token-runtime"
    export PATH="$(dirname "${go_bin}"):${PATH}"
    export GOCACHE="${ROOT_DIR}/.tools/go-cache"
    export GOPATH="${ROOT_DIR}/.tools/go"
    TOKEN_RUNTIME_ADDR=":${port}" "${go_bin}" run ./cmd/platform-token-runtime
  ) >"${RUNTIME_LOG}" 2>&1 &
  RUNTIME_PID=$!
  RUNTIME_STARTED_BY_SCRIPT=1

  for _ in {1..50}; do
    if is_http_ready "${TOKEN_RUNTIME_URL}"; then
      return 0
    fi
    sleep 0.2
  done

  echo "[FAIL] temporary token runtime failed to become ready: ${TOKEN_RUNTIME_URL}"
  echo "[INFO] log: ${RUNTIME_LOG}"
  exit 1
}

issue_token() {
  local role="$1"
  local scope_json="$2"
  local req_id="req-gen-${role}-${TS}"
  local idem="idem-gen-${role}-${TS}"
  local subject="${TOKEN_SUBJECT_PREFIX}-${role}-${TS}"
  local payload
  payload="$(jq -n \
    --arg s "${subject}" \
    --arg r "${role}" \
    --argjson ttl "${TOKEN_TTL_SECONDS}" \
    --argjson sc "${scope_json}" \
    '{subject_id:$s,role:$r,ttl_seconds:$ttl,scope:$sc}')"

  local body_file
  body_file="$(mktemp)"
  local status
  status="$(curl -sS -m 8 -o "${body_file}" -w "%{http_code}" \
    -X POST "${TOKEN_RUNTIME_URL}/api/v1/platform/tokens/issue" \
    -H "Content-Type: application/json" \
    -H "X-Request-Id: ${req_id}" \
    -H "Idempotency-Key: ${idem}" \
    -d "${payload}")"

  if [[ "${status}" != "201" ]]; then
    echo "[FAIL] issue ${role} token failed, status=${status}"
    cat "${body_file}" || true
    rm -f "${body_file}"
    exit 1
  fi

  local token
  token="$(jq -r '.data.access_token // empty' "${body_file}")"
  rm -f "${body_file}"
  if [[ -z "${token}" ]]; then
    echo "[FAIL] issue ${role} token returned empty access_token"
    exit 1
  fi
  echo "${token}"
}

ensure_runtime

OWNER_TOKEN="$(issue_token "owner" "[\"supply:*\"]")"
VIEWER_TOKEN="$(issue_token "viewer" "[\"supply:read\"]")"
ADMIN_TOKEN="$(issue_token "admin" "[\"supply:*\"]")"

EXP_UTC="$(date -u -d "+${TOKEN_TTL_SECONDS} seconds" +%Y-%m-%dT%H:%M:%SZ)"
mkdir -p "$(dirname "${ENV_PATH}")"
cat > "${ENV_PATH}" <<EOF
# local staging-real(simulated) generated at $(date -u +%Y-%m-%dT%H:%M:%SZ)
# token nominal expiry: ${EXP_UTC}
# token runtime source: ${TOKEN_RUNTIME_URL}
API_BASE_URL="${API_BASE_URL_VALUE}"
OWNER_BEARER_TOKEN="${OWNER_TOKEN}"
VIEWER_BEARER_TOKEN="${VIEWER_TOKEN}"
ADMIN_BEARER_TOKEN="${ADMIN_TOKEN}"

TEST_PROVIDER="openai"
TEST_MODEL="gpt-4o"
TEST_ACCOUNT_ALIAS="sup_acc_cmd"
TEST_CREDENTIAL_INPUT="sk-test-replace-me"
TEST_PAYMENT_METHOD="alipay"
TEST_PAYMENT_ACCOUNT="tester@example.com"
TEST_SMS_CODE="123456"

SUPPLIER_DIRECT_TEST_URL=""
EOF
chmod 600 "${ENV_PATH}"

owner_hash="$(printf "%s" "${OWNER_TOKEN}" | sha256sum | awk '{print substr($1,1,12)}')"
viewer_hash="$(printf "%s" "${VIEWER_TOKEN}" | sha256sum | awk '{print substr($1,1,12)}')"
admin_hash="$(printf "%s" "${ADMIN_TOKEN}" | sha256sum | awk '{print substr($1,1,12)}')"

{
  echo "# Local Staging Env Generation"
  echo
  echo "- 时间戳：${TS}"
  echo "- 输出文件：\`${ENV_PATH}\`"
  echo "- API_BASE_URL：\`${API_BASE_URL_VALUE}\`"
  echo "- token nominal expiry(UTC)：\`${EXP_UTC}\`"
  echo "- token runtime：\`${TOKEN_RUNTIME_URL}\`"
  echo "- runtime auto-start：\`${RUNTIME_STARTED_BY_SCRIPT}\`"
  echo
  echo "## Token 摘要（不含明文）"
  echo
  echo "| role | length | sha256_12 |"
  echo "|---|---:|---|"
  echo "| owner | ${#OWNER_TOKEN} | ${owner_hash} |"
  echo "| viewer | ${#VIEWER_TOKEN} | ${viewer_hash} |"
  echo "| admin | ${#ADMIN_TOKEN} | ${admin_hash} |"
  echo
  echo "## 下一步"
  echo
  echo "1. 使用该 env 执行：\`ALLOW_LOCAL_MOCK_STAGING=1 bash scripts/ci/staging_release_pipeline.sh ${ENV_PATH}\`"
  echo "2. 若切换真实 staging，更新 \`API_BASE_URL\` 后复跑。"
} > "${REPORT_FILE}"

echo "[PASS] env generated: ${ENV_PATH}"
echo "[INFO] report: ${REPORT_FILE}"
if [[ "${RUNTIME_STARTED_BY_SCRIPT}" == "1" ]]; then
  echo "[INFO] runtime log: ${RUNTIME_LOG}"
fi
