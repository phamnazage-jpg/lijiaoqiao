#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/common.sh" "${1:-}"

require_bin curl
require_bin jq
require_var API_BASE_URL
require_var OWNER_BEARER_TOKEN

ART_DIR="$(init_artifact_dir "sup007")"

# 1) 平台凭证主路径访问（应可用）
MAIN_RESP="$(curl_json POST "${API_BASE_URL}/api/v1/chat/completions" "${OWNER_BEARER_TOKEN}" '{"model":"gpt-4o","messages":[{"role":"user","content":"ping"}]}')"
echo "${MAIN_RESP}" > "${ART_DIR}/01_main_path_with_platform_token.json"

# 2) 外部 query key 请求（应被拒绝）
set +e
QUERY_RESP="$(curl -sS -w "\nHTTP_STATUS:%{http_code}\n" \
  "${API_BASE_URL}/v1beta/models?key=test-query-key" 2>&1)"
set -e
echo "${QUERY_RESP}" > "${ART_DIR}/02_external_query_key_attempt.txt"

# 3) 可选：直连上游探测（应失败/阻断）
if [[ -n "${SUPPLIER_DIRECT_TEST_URL:-}" ]]; then
  set +e
  DIRECT_RESP="$(curl -sS -m 8 -w "\nHTTP_STATUS:%{http_code}\n" "${SUPPLIER_DIRECT_TEST_URL}" 2>&1)"
  set -e
  echo "${DIRECT_RESP}" > "${ART_DIR}/03_direct_supplier_probe.txt"
fi

# 4) 响应样本脱敏扫描（简单规则）
SCAN_TARGETS=("${ART_DIR}/01_main_path_with_platform_token.json" "${ART_DIR}/02_external_query_key_attempt.txt")
if [[ -n "${SUPPLIER_DIRECT_TEST_URL:-}" ]]; then
  SCAN_TARGETS+=("${ART_DIR}/03_direct_supplier_probe.txt")
fi

LEAK_COUNT=0
for f in "${SCAN_TARGETS[@]}"; do
  if grep -Eiq "(sk-[A-Za-z0-9]{10,}|api[_-]?key[\"'= :]+[A-Za-z0-9_-]{8,}|Bearer [A-Za-z0-9._-]{20,})" "${f}"; then
    echo "sensitive pattern found in ${f}" >> "${ART_DIR}/04_redaction_scan.txt"
    LEAK_COUNT=$((LEAK_COUNT + 1))
  fi
done

if [[ "${LEAK_COUNT}" -eq 0 ]]; then
  echo "redaction scan passed" > "${ART_DIR}/04_redaction_scan.txt"
fi

cat > "${ART_DIR}/summary.txt" <<EOF
SUP-007 boundary checks executed.
artifacts:
  ${ART_DIR}/01_main_path_with_platform_token.json
  ${ART_DIR}/02_external_query_key_attempt.txt
  ${ART_DIR}/04_redaction_scan.txt
optional:
  ${ART_DIR}/03_direct_supplier_probe.txt
leak_count=${LEAK_COUNT}
EOF

echo "done: ${ART_DIR}"
