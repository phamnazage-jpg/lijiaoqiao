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
require_var TEST_MODEL
require_var TEST_CREDENTIAL_INPUT

ART_DIR="$(init_artifact_dir "sup005")"

# ensure an account exists
CREATE_ACC_REQ="$(jq -n \
  --arg p "${TEST_PROVIDER}" \
  --arg ct "api_key" \
  --arg cred "${TEST_CREDENTIAL_INPUT}" \
  '{provider:$p,account_type:$ct,credential_input:$cred,account_alias:"sup_pkg_acc",risk_ack:true}')"
CREATE_ACC_RESP="$(curl_json POST "${API_BASE_URL}/api/v1/supply/accounts" "${OWNER_BEARER_TOKEN}" "${CREATE_ACC_REQ}")"
echo "${CREATE_ACC_RESP}" > "${ART_DIR}/00_create_account.json"
ACCOUNT_ID="$(echo "${CREATE_ACC_RESP}" | json_get '.data.account_id')"
if [[ -z "${ACCOUNT_ID}" ]]; then
  echo "failed to create account for package flow"
  exit 1
fi

DRAFT_REQ="$(jq -n \
  --argjson sid "${ACCOUNT_ID}" \
  --arg model "${TEST_MODEL}" \
  '{supply_account_id:$sid,model:$model,total_quota:1000,price_per_1m_input:5,price_per_1m_output:10,valid_days:30,max_concurrent:10,rate_limit_rpm:60}')"

DRAFT_RESP="$(curl_json POST "${API_BASE_URL}/api/v1/supply/packages/draft" "${OWNER_BEARER_TOKEN}" "${DRAFT_REQ}")"
echo "${DRAFT_RESP}" > "${ART_DIR}/01_draft.json"
PACKAGE_ID="$(echo "${DRAFT_RESP}" | json_get '.data.package_id')"
if [[ -z "${PACKAGE_ID}" ]]; then
  echo "failed to create package draft"
  exit 1
fi

PUBLISH_RESP="$(curl_json POST "${API_BASE_URL}/api/v1/supply/packages/${PACKAGE_ID}/publish" "${OWNER_BEARER_TOKEN}")"
echo "${PUBLISH_RESP}" > "${ART_DIR}/02_publish.json"

PAUSE_RESP="$(curl_json POST "${API_BASE_URL}/api/v1/supply/packages/${PACKAGE_ID}/pause" "${OWNER_BEARER_TOKEN}")"
echo "${PAUSE_RESP}" > "${ART_DIR}/03_pause.json"

UNLIST_RESP="$(curl_json POST "${API_BASE_URL}/api/v1/supply/packages/${PACKAGE_ID}/unlist" "${OWNER_BEARER_TOKEN}")"
echo "${UNLIST_RESP}" > "${ART_DIR}/04_unlist.json"

BATCH_REQ="$(jq -n \
  --argjson pid "${PACKAGE_ID}" \
  '{items:[{package_id:$pid,price_per_1m_input:6,price_per_1m_output:12}]}')"
BATCH_RESP="$(curl_json POST "${API_BASE_URL}/api/v1/supply/packages/batch-price" "${OWNER_BEARER_TOKEN}" "${BATCH_REQ}")"
echo "${BATCH_RESP}" > "${ART_DIR}/05_batch_price.json"

CLONE_RESP="$(curl_json POST "${API_BASE_URL}/api/v1/supply/packages/${PACKAGE_ID}/clone" "${OWNER_BEARER_TOKEN}")"
echo "${CLONE_RESP}" > "${ART_DIR}/06_clone.json"
CLONE_ID="$(echo "${CLONE_RESP}" | json_get '.data.package_id')"

cat > "${ART_DIR}/summary.txt" <<EOF
SUP-005 package flow executed.
account_id=${ACCOUNT_ID}
package_id=${PACKAGE_ID}
clone_package_id=${CLONE_ID}
artifacts:
  ${ART_DIR}/01_draft.json
  ${ART_DIR}/02_publish.json
  ${ART_DIR}/03_pause.json
  ${ART_DIR}/04_unlist.json
  ${ART_DIR}/05_batch_price.json
  ${ART_DIR}/06_clone.json
EOF

echo "done: ${ART_DIR}"
