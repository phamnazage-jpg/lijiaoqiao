#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
ENV_FILE="${1:-${SCRIPT_DIR}/.env}"
OUT_DIR="${ROOT_DIR}/reports/archive/gate_verification"
ART_ROOT="${ROOT_DIR}/tests/supply/artifacts"
TS="$(date +%F_%H%M%S)"
CASE_ID="tok005_dryrun_${TS}"
ART_DIR="${ART_ROOT}/${CASE_ID}"
REPORT_FILE="${OUT_DIR}/${CASE_ID}.md"
LOG_FILE="${OUT_DIR}/${CASE_ID}.log"

mkdir -p "${OUT_DIR}" "${ART_DIR}"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "[FAIL] env file not found: ${ENV_FILE}" | tee -a "${LOG_FILE}"
  exit 1
fi

# shellcheck disable=SC1090
source "${ENV_FILE}"

GO_BIN="${ROOT_DIR}/.tools/go-current/bin/go"
if [[ ! -x "${GO_BIN}" ]]; then
  if command -v go >/dev/null 2>&1; then
    GO_BIN="$(command -v go)"
  else
    echo "[FAIL] go binary not found. expected: ${ROOT_DIR}/.tools/go-current/bin/go" | tee -a "${LOG_FILE}"
    exit 1
  fi
fi

PLATFORM_RT_DIR="${ROOT_DIR}/platform-token-runtime"
if [[ ! -d "${PLATFORM_RT_DIR}" ]]; then
  echo "[FAIL] missing runtime dir: ${PLATFORM_RT_DIR}" | tee -a "${LOG_FILE}"
  exit 1
fi

{
  echo "[INFO] TOK-005 dry-run started at ${TS}"
  echo "[INFO] go bin: ${GO_BIN}"
  "${GO_BIN}" version
} | tee "${LOG_FILE}"

GO_TEST_STATUS="PASS"
set +e
(
  cd "${PLATFORM_RT_DIR}"
  export PATH="$(dirname "${GO_BIN}"):${PATH}"
  export GOCACHE="${ROOT_DIR}/.tools/go-cache"
  export GOPATH="${ROOT_DIR}/.tools/go"
  "${GO_BIN}" test ./...
) > "${ART_DIR}/go_test_output.txt" 2>&1
GO_TEST_RC=$?
set -e
if [[ "${GO_TEST_RC}" -ne 0 ]]; then
  GO_TEST_STATUS="FAIL"
fi
cat "${ART_DIR}/go_test_output.txt" >> "${LOG_FILE}"

# M-016: query key 外拒能力静态检查
QUERY_KEY_STATUS="PASS"
if ! grep -Eq 'disallowedQueryKeys = \[\]string\{"key", "api_key", "token"\}' \
  "${PLATFORM_RT_DIR}/internal/auth/middleware/query_key_reject_middleware.go"; then
  QUERY_KEY_STATUS="FAIL"
fi

# M-013: 敏感值不落审计（用例断言存在性）
REDACTION_STATUS="PASS"
if ! grep -q 'TestTOKAud006QueryKeyRejectedEvent' "${PLATFORM_RT_DIR}/internal/token/audit_executable_test.go"; then
  REDACTION_STATUS="FAIL"
fi
if ! grep -q 'must not contain raw query key value' "${PLATFORM_RT_DIR}/internal/token/audit_executable_test.go"; then
  REDACTION_STATUS="FAIL"
fi

# TOK-LIFE/TOK-AUD 全量可执行用例覆盖检查
CASE_COVERAGE_STATUS="PASS"
for case_id in TOKLife001 TOKLife002 TOKLife003 TOKLife004 TOKLife005 TOKLife006 TOKLife007 TOKLife008; do
  if ! grep -q "Test${case_id}" "${PLATFORM_RT_DIR}/internal/token/lifecycle_executable_test.go"; then
    CASE_COVERAGE_STATUS="FAIL"
  fi
done
for case_id in TOKAud001 TOKAud002 TOKAud003 TOKAud004 TOKAud005 TOKAud006 TOKAud007; do
  if ! grep -q "Test${case_id}" "${PLATFORM_RT_DIR}/internal/token/audit_executable_test.go"; then
    CASE_COVERAGE_STATUS="FAIL"
  fi
done

# 真实 staging 准备度（当前阶段预期为 BLOCKED）
LIVE_READY="YES"
LIVE_BLOCK_REASON=""
required=(API_BASE_URL OWNER_BEARER_TOKEN VIEWER_BEARER_TOKEN ADMIN_BEARER_TOKEN)
for v in "${required[@]}"; do
  if [[ -z "${!v:-}" ]]; then
    LIVE_READY="NO"
    LIVE_BLOCK_REASON="missing ${v}"
    break
  fi
done
if [[ "${LIVE_READY}" == "YES" ]]; then
  for t in "${OWNER_BEARER_TOKEN}" "${VIEWER_BEARER_TOKEN}" "${ADMIN_BEARER_TOKEN}"; do
    if [[ "${t}" == replace-me-* ]]; then
      LIVE_READY="NO"
      LIVE_BLOCK_REASON="placeholder token detected"
      break
    fi
  done
fi
if [[ "${LIVE_READY}" == "YES" && "${API_BASE_URL}" == *"example.com"* ]]; then
  LIVE_READY="NO"
  LIVE_BLOCK_REASON="placeholder API_BASE_URL detected"
fi

cat > "${REPORT_FILE}" <<EOF
# TOK-005 凭证边界 Dry-Run 报告

- 时间戳：${TS}
- 环境文件：${ENV_FILE}
- 用途：开发阶段预联调（不替代真实 staging 结论）

## 1. 结果总览

| 检查项 | 结果 | 说明 |
|---|---|---|
| Go 测试执行 | ${GO_TEST_STATUS} | \`go test ./...\` 输出见 artifacts |
| Query Key 外拒检查（M-016） | ${QUERY_KEY_STATUS} | 中间件规则静态校验 |
| 审计脱敏检查（M-013） | ${REDACTION_STATUS} | 审计测试中存在敏感值禁止断言 |
| TOK 用例全量可执行覆盖 | ${CASE_COVERAGE_STATUS} | TOK-LIFE-001~008 / TOK-AUD-001~007 |
| staging 实测就绪性 | ${LIVE_READY} | ${LIVE_BLOCK_REASON:-ready} |

## 2. 证据路径

1. \`${ART_DIR}/go_test_output.txt\`
2. \`${LOG_FILE}\`

## 3. 判定

1. Dry-run 通过条件：
   1. Go 测试执行=PASS
   2. Query Key 外拒检查=PASS
   3. 审计脱敏检查=PASS
   4. TOK 用例全量可执行覆盖=PASS
2. staging 就绪性为 NO 时，仅表示“真实联调暂不可启动”，不影响开发阶段 dry-run 结论。
EOF

RESULT="PASS"
if [[ "${GO_TEST_STATUS}" != "PASS" || "${QUERY_KEY_STATUS}" != "PASS" || "${REDACTION_STATUS}" != "PASS" || "${CASE_COVERAGE_STATUS}" != "PASS" ]]; then
  RESULT="FAIL"
fi

{
  echo "[INFO] report: ${REPORT_FILE}"
  echo "[INFO] artifact: ${ART_DIR}"
  echo "[RESULT] ${RESULT}"
} | tee -a "${LOG_FILE}"

if [[ "${RESULT}" != "PASS" ]]; then
  exit 1
fi
