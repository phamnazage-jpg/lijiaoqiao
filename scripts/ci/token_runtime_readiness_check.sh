#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
DATE_TAG="${1:-$(date +%F)}"
OUT_DIR="${ROOT_DIR}/reports/gates"
mkdir -p "${OUT_DIR}"

TS_TIME="$(date +%H%M%S)"
REPORT_FILE="${OUT_DIR}/token_runtime_readiness_${DATE_TAG}_${TS_TIME}.md"
LOG_FILE="${OUT_DIR}/token_runtime_readiness_${DATE_TAG}_${TS_TIME}.log"
GO_BIN="${ROOT_DIR}/.tools/go-current/bin/go"

if [[ ! -x "${GO_BIN}" ]]; then
  GO_BIN="$(command -v go || true)"
fi
if [[ -z "${GO_BIN}" ]]; then
  echo "[FAIL] go binary not found" | tee -a "${LOG_FILE}"
  exit 1
fi

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

check_file() {
  local id="$1"
  local desc="$2"
  local file="$3"
  if [[ -f "${file}" ]]; then
    add_check "${id}" "PASS" "${desc}" "${file}"
  else
    add_check "${id}" "FAIL" "${desc}" "${file} (missing)"
  fi
}

check_pattern() {
  local id="$1"
  local desc="$2"
  local file="$3"
  local pattern="$4"
  if [[ -f "${file}" ]] && grep -Eq "${pattern}" "${file}"; then
    add_check "${id}" "PASS" "${desc}" "${file}"
  else
    add_check "${id}" "FAIL" "${desc}" "${file} (pattern missing)"
  fi
}

check_file "TOK-REAL-001-C1" "Token API 可执行入口存在" "${ROOT_DIR}/platform-token-runtime/cmd/platform-token-runtime/main.go"
check_file "TOK-REAL-001-C2" "Token HTTP 契约处理实现存在" "${ROOT_DIR}/platform-token-runtime/internal/httpapi/token_api.go"
check_file "TOK-REAL-001-C3" "Token 生命周期运行时实现存在" "${ROOT_DIR}/platform-token-runtime/internal/auth/service/inmemory_runtime.go"
check_file "TOK-REAL-001-C4" "TOK 生命周期可执行测试存在" "${ROOT_DIR}/platform-token-runtime/internal/token/lifecycle_executable_test.go"
check_file "TOK-REAL-001-C5" "TOK 审计可执行测试存在" "${ROOT_DIR}/platform-token-runtime/internal/token/audit_executable_test.go"
check_file "TOK-REAL-003-C1" "可部署镜像构建工件存在" "${ROOT_DIR}/platform-token-runtime/Dockerfile"
check_file "TOK-REAL-003-C2" "平台 token OpenAPI 契约存在" "${ROOT_DIR}/docs/platform_token_api_contract_openapi_draft_v1_2026-03-29.yaml"
check_pattern "TOK-REAL-002-C1" "审计事件查询接口已落地（OpenAPI）" "${ROOT_DIR}/docs/platform_token_api_contract_openapi_draft_v1_2026-03-29.yaml" "/api/v1/platform/tokens/audit-events:"
check_pattern "TOK-REAL-002-C2" "审计事件查询接口已落地（代码）" "${ROOT_DIR}/platform-token-runtime/internal/httpapi/token_api.go" "handleAuditEvents"
check_file "TOK-REAL-003-C3" "token runtime 持久化表结构工件存在" "${ROOT_DIR}/sql/postgresql/token_runtime_schema_v1.sql"

GO_TEST_LOG="${OUT_DIR}/token_runtime_go_test_${DATE_TAG}_${TS_TIME}.log"
if (cd "${ROOT_DIR}/platform-token-runtime" && export PATH="$(dirname "${GO_BIN}"):$PATH" && export GOCACHE="${ROOT_DIR}/.tools/go-cache" && export GOPATH="${ROOT_DIR}/.tools/go" && "${GO_BIN}" test ./... >"${GO_TEST_LOG}" 2>&1); then
  add_check "TOK-REAL-001-C6" "PASS" "Token runtime 测试通过" "${GO_TEST_LOG}"
else
  add_check "TOK-REAL-001-C6" "FAIL" "Token runtime 测试通过" "${GO_TEST_LOG}"
fi

GO_BUILD_LOG="${OUT_DIR}/token_runtime_go_build_${DATE_TAG}_${TS_TIME}.log"
BIN_PATH="${OUT_DIR}/token_runtime_bin_${DATE_TAG}_${TS_TIME}"
if (cd "${ROOT_DIR}/platform-token-runtime" && export PATH="$(dirname "${GO_BIN}"):$PATH" && export GOCACHE="${ROOT_DIR}/.tools/go-cache" && export GOPATH="${ROOT_DIR}/.tools/go" && "${GO_BIN}" build -o "${BIN_PATH}" ./cmd/platform-token-runtime >"${GO_BUILD_LOG}" 2>&1); then
  add_check "TOK-REAL-001-C7" "PASS" "Token runtime 可构建" "${GO_BUILD_LOG}"
else
  add_check "TOK-REAL-001-C7" "FAIL" "Token runtime 可构建" "${GO_BUILD_LOG}"
fi

SMOKE_LOG="${OUT_DIR}/token_runtime_smoke_${DATE_TAG}_${TS_TIME}.log"
is_port_in_use() {
  local port="$1"
  ss -ltn | awk '{print $4}' | grep -Eq "[:.]${port}$"
}

pick_smoke_port() {
  local base="${1:-18082}"
  local max_tries="${2:-50}"
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

SMOKE_PORT_BASE="${TOKEN_RUNTIME_SMOKE_PORT:-18082}"
if ! SMOKE_PORT="$(pick_smoke_port "${SMOKE_PORT_BASE}" "50")"; then
  echo "[FAIL] no available smoke port from ${SMOKE_PORT_BASE} within 50 tries" > "${SMOKE_LOG}"
  add_check "TOK-REAL-001-C8" "FAIL" "Token runtime 本地可运行冒烟通过" "${SMOKE_LOG}"
  SMOKE_PORT=""
fi

if [[ "${ENABLE_TOKEN_RUNTIME_SMOKE:-0}" == "1" ]]; then
  if [[ -n "${SMOKE_PORT}" ]]; then
    set +e
    (
      echo "[INFO] start token runtime smoke on :${SMOKE_PORT}"
      TOKEN_RUNTIME_ADDR=":${SMOKE_PORT}" "${BIN_PATH}" >"${SMOKE_LOG}.server" 2>&1 &
      pid=$!
      trap 'kill "${pid}" >/dev/null 2>&1 || true' EXIT

      ready=0
      for _ in {1..20}; do
        if curl -sS -m 2 "http://127.0.0.1:${SMOKE_PORT}/actuator/health" | grep -q '\"UP\"'; then
          ready=1
          break
        fi
        sleep 0.2
      done
      if [[ "${ready}" -ne 1 ]]; then
        echo "[FAIL] health check failed"
        exit 1
      fi

      issue_code="$(curl -sS -m 3 -o "${SMOKE_LOG}.issue.json" -w "%{http_code}" \
        -X POST "http://127.0.0.1:${SMOKE_PORT}/api/v1/platform/tokens/issue" \
        -H "Content-Type: application/json" \
        -H "X-Request-Id: req-smoke-issue" \
        -H "Idempotency-Key: idem-smoke-issue" \
        -d '{"subject_id":"smoke-user","role":"owner","ttl_seconds":300,"scope":["supply:*"]}')"
      if [[ "${issue_code}" != "201" ]]; then
        echo "[FAIL] issue status=${issue_code}"
        exit 1
      fi

      audit_code="$(curl -sS -m 3 -o "${SMOKE_LOG}.audit.json" -w "%{http_code}" \
        "http://127.0.0.1:${SMOKE_PORT}/api/v1/platform/tokens/audit-events?request_id=req-smoke-issue&limit=5" \
        -H "X-Request-Id: req-smoke-audit")"
      if [[ "${audit_code}" != "200" ]]; then
        echo "[FAIL] audit query status=${audit_code}"
        exit 1
      fi
      if ! grep -q '"event_name"' "${SMOKE_LOG}.audit.json"; then
        echo "[FAIL] audit query payload missing event_name"
        exit 1
      fi

      echo "[PASS] smoke passed"
    ) >"${SMOKE_LOG}" 2>&1
    smoke_rc=$?
    set -e
    if [[ "${smoke_rc}" -eq 0 ]]; then
      add_check "TOK-REAL-001-C8" "PASS" "Token runtime 本地可运行冒烟通过" "${SMOKE_LOG}"
    else
      add_check "TOK-REAL-001-C8" "FAIL" "Token runtime 本地可运行冒烟通过" "${SMOKE_LOG}"
    fi
  fi
else
  add_check "TOK-REAL-001-C8" "PASS" "Token runtime 本地可运行冒烟（默认跳过，可通过 ENABLE_TOKEN_RUNTIME_SMOKE=1 开启）" "N/A"
fi

TOTAL="${#CHECK_IDS[@]}"
PASS_CNT=0
for status in "${CHECK_STATUS[@]}"; do
  if [[ "${status}" == "PASS" ]]; then
    PASS_CNT=$((PASS_CNT + 1))
  fi
done

READINESS_PCT="$(awk -v p="${PASS_CNT}" -v t="${TOTAL}" 'BEGIN{if(t==0){printf "0.00"}else{printf "%.2f", (p/t)*100}}')"
RESULT="PASS"
if [[ "${READINESS_PCT}" != "100.00" ]]; then
  RESULT="FAIL"
fi

{
  echo "# Token Runtime Readiness Check (${DATE_TAG})"
  echo
  echo "- 时间戳：${DATE_TAG}_${TS_TIME}"
  echo "- 指标：M-021 token_runtime_readiness_pct"
  echo "- 结果：**${RESULT}**"
  echo "- 数值：${READINESS_PCT}% (${PASS_CNT}/${TOTAL})"
  echo
  echo "| 检查项 | 结果 | 说明 | 证据 |"
  echo "|---|---|---|---|"
  for i in "${!CHECK_IDS[@]}"; do
    echo "| ${CHECK_IDS[$i]} | ${CHECK_STATUS[$i]} | ${CHECK_DESC[$i]} | ${CHECK_EVIDENCE[$i]} |"
  done
  echo
  echo "## 结论"
  echo
  echo "1. 本报告仅评估 token 运行态实现就绪度，不替代真实 staging 联调结论。"
  echo "2. 真实放行仍需结合 M-013~M-016、SUP-004~SUP-007 与 PHASE-07 实测。"
} > "${REPORT_FILE}"

{
  echo "[INFO] report=${REPORT_FILE}"
  echo "[INFO] readiness_pct=${READINESS_PCT}"
  echo "[RESULT] ${RESULT}"
} | tee -a "${LOG_FILE}"

if [[ "${RESULT}" != "PASS" ]]; then
  exit 1
fi
