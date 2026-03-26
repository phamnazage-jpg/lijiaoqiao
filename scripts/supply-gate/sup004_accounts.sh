#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/common.sh" "${1:-}"

require_bin curl
require_bin jq
require_var API_BASE_URL
require_var OWNER_BEARER_TOKEN
require_var TEST_PROVIDER
require_var TEST_CREDENTIAL_INPUT

ART_DIR="$(init_artifact_dir "sup004")"

VERIFY_REQ="$(jq -n \
  --arg p "${TEST_PROVIDER}" \
  --arg ct "api_key" \
  --arg cred "${TEST_CREDENTIAL_INPUT}" \
  '{provider:$p,account_type:$ct,credential_input:$cred}')"

VERIFY_RESP="$(curl_json POST "${API_BASE_URL}/api/v1/supply/accounts/verify" "${OWNER_BEARER_TOKEN}" "${VERIFY_REQ}")"
echo "${VERIFY_RESP}" > "${ART_DIR}/01_verify.json"

CREATE_REQ="$(jq -n \
  --arg p "${TEST_PROVIDER}" \
  --arg ct "api_key" \
  --arg cred "${TEST_CREDENTIAL_INPUT}" \
  --arg alias "${TEST_ACCOUNT_ALIAS:-sup_acc_cmd}" \
  '{provider:$p,account_type:$ct,credential_input:$cred,account_alias:$alias,risk_ack:true}')"

CREATE_RESP="$(curl_json POST "${API_BASE_URL}/api/v1/supply/accounts" "${OWNER_BEARER_TOKEN}" "${CREATE_REQ}")"
echo "${CREATE_RESP}" > "${ART_DIR}/02_create.json"
ACCOUNT_ID="$(echo "${CREATE_RESP}" | json_get '.data.account_id')"

if [[ -z "${ACCOUNT_ID}" ]]; then
  echo "create account failed: missing account_id"
  exit 1
fi

ACTIVATE_RESP="$(curl_json POST "${API_BASE_URL}/api/v1/supply/accounts/${ACCOUNT_ID}/activate" "${OWNER_BEARER_TOKEN}")"
echo "${ACTIVATE_RESP}" > "${ART_DIR}/03_activate.json"

SUSPEND_RESP="$(curl_json POST "${API_BASE_URL}/api/v1/supply/accounts/${ACCOUNT_ID}/suspend" "${OWNER_BEARER_TOKEN}")"
echo "${SUSPEND_RESP}" > "${ART_DIR}/04_suspend.json"

AUDIT_RESP="$(curl_json GET "${API_BASE_URL}/api/v1/supply/accounts/${ACCOUNT_ID}/audit-logs?page=1&page_size=20" "${OWNER_BEARER_TOKEN}")"
echo "${AUDIT_RESP}" > "${ART_DIR}/05_audit_logs.json"

cat > "${ART_DIR}/summary.txt" <<EOF
SUP-004 account flow executed.
account_id=${ACCOUNT_ID}
artifacts:
  ${ART_DIR}/01_verify.json
  ${ART_DIR}/02_create.json
  ${ART_DIR}/03_activate.json
  ${ART_DIR}/04_suspend.json
  ${ART_DIR}/05_audit_logs.json
EOF

echo "done: ${ART_DIR}"
