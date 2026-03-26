#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/common.sh" "${1:-}"

require_bin curl
require_bin jq
require_var API_BASE_URL
require_var OWNER_BEARER_TOKEN
require_var TEST_PAYMENT_METHOD
require_var TEST_PAYMENT_ACCOUNT
require_var TEST_SMS_CODE

ART_DIR="$(init_artifact_dir "sup006")"

BILLING_RESP="$(curl_json GET "${API_BASE_URL}/api/v1/supplier/billing?page=1&page_size=20" "${OWNER_BEARER_TOKEN}")"
echo "${BILLING_RESP}" > "${ART_DIR}/01_billing.json"

WITHDRAW_REQ="$(jq -n \
  --arg pm "${TEST_PAYMENT_METHOD}" \
  --arg pa "${TEST_PAYMENT_ACCOUNT}" \
  --arg sms "${TEST_SMS_CODE}" \
  '{withdraw_amount:10,payment_method:$pm,payment_account:$pa,sms_code:$sms}')"

WITHDRAW_RESP="$(curl_json POST "${API_BASE_URL}/api/v1/supply/settlements/withdraw" "${OWNER_BEARER_TOKEN}" "${WITHDRAW_REQ}")"
echo "${WITHDRAW_RESP}" > "${ART_DIR}/02_withdraw_create.json"
SETTLEMENT_ID="$(echo "${WITHDRAW_RESP}" | json_get '.data.settlement_id')"
if [[ -z "${SETTLEMENT_ID}" ]]; then
  echo "failed to create settlement withdraw"
  exit 1
fi

CANCEL_RESP="$(curl_json POST "${API_BASE_URL}/api/v1/supply/settlements/${SETTLEMENT_ID}/cancel" "${OWNER_BEARER_TOKEN}")"
echo "${CANCEL_RESP}" > "${ART_DIR}/03_withdraw_cancel.json"

STATEMENT_RESP="$(curl_json GET "${API_BASE_URL}/api/v1/supply/settlements/${SETTLEMENT_ID}/statement" "${OWNER_BEARER_TOKEN}")"
echo "${STATEMENT_RESP}" > "${ART_DIR}/04_statement.json"

EARNINGS_RESP="$(curl_json GET "${API_BASE_URL}/api/v1/supply/earnings/records?page=1&page_size=20" "${OWNER_BEARER_TOKEN}")"
echo "${EARNINGS_RESP}" > "${ART_DIR}/05_earnings_records.json"

cat > "${ART_DIR}/summary.txt" <<EOF
SUP-006 settlement flow executed.
settlement_id=${SETTLEMENT_ID}
artifacts:
  ${ART_DIR}/01_billing.json
  ${ART_DIR}/02_withdraw_create.json
  ${ART_DIR}/03_withdraw_cancel.json
  ${ART_DIR}/04_statement.json
  ${ART_DIR}/05_earnings_records.json
EOF

echo "done: ${ART_DIR}"
