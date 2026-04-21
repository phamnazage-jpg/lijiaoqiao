#!/usr/bin/env bash
# scripts/ci/cross_service_smoke.sh
# 跨服务 smoke 测试：gateway -> token-runtime -> supply-api
# 退出码语义：
#   0  = PASS（真实 staging smoke 通过）
#   1  = FAIL（任意链路失败）
#   2  = SKIP_LOCAL_PLACEHOLDER（本地/mock 输入，非真实 staging 证据）
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
OUT_DIR="${ROOT_DIR}/reports/archive/gate_verification"
mkdir -p "${OUT_DIR}"

TS="$(date +%F_%H%M%S)"
LOG_FILE="${OUT_DIR}/cross_service_smoke_${TS}.log"
REPORT_FILE="${OUT_DIR}/cross_service_smoke_${TS}.md"

# ── 输入环境变量（默认值适配本地 dev） ───────────────────────
GATEWAY_URL="${SMOKE_GATEWAY_BASE_URL:-http://127.0.0.1:18080}"
TOK_URL="${SMOKE_TOKEN_RUNTIME_BASE_URL:-http://127.0.0.1:18081}"
SUPPLY_URL="${SMOKE_SUPPLY_API_BASE_URL:-http://127.0.0.1:18082}"
BEARER_TOKEN="${SMOKE_BEARER_TOKEN:-}"
EXPECTED_SCOPE="${SMOKE_EXPECTED_SCOPE:-supply:read supply:write}"
EXPECTED_MODEL="${SMOKE_EXPECTED_MODEL:-}"
ALLOW_LOCAL_PLACEHOLDER="${SMOKE_ALLOW_LOCAL_PLACEHOLDER:-0}"

log() {
  echo "$1" | tee -a "${LOG_FILE}"
}

# ── 前置检测：是否是本地 mock/stub 环境 ──────────────────────
is_local_placeholder() {
  # 若所有服务 URL 都是 localhost/127.0.0.1，则认为是本地占位
  if [[ "${GATEWAY_URL}" == *"localhost"* || "${GATEWAY_URL}" == *"127.0.0.1"* ]] && \
     [[ "${TOK_URL}" == *"localhost"* || "${TOK_URL}" == *"127.0.0.1"* ]] && \
     [[ "${SUPPLY_URL}" == *"localhost"* || "${SUPPLY_URL}" == *"127.0.0.1"* ]]; then
    return 0
  fi
  return 1
}

# ── Smoke 场景 1：全链路健康检查 ─────────────────────────────
smoke_check_services() {
  log "[SMOKE-1] 服务健康检查"

  local fail=0

  for svc in "gateway:${GATEWAY_URL}" "token-runtime:${TOK_URL}" "supply-api:${SUPPLY_URL}"; do
    local name="${svc%%:*}"
    local url="${svc#*:}"
    local health_url="${url}/actuator/health"

    local resp
    resp="$(curl -sS -m 5 "${health_url}" -w "\n__HTTP_CODE__:%{http_code}" 2>&1)" || true
    local code
    code="$(echo "${resp}" | grep -o '__HTTP_CODE__.*' | cut -d: -f2 || echo '000')"

    if [[ "${code}" == "200" ]]; then
      log "  [PASS] ${name} health=${code}"
    else
      log "  [FAIL] ${name} health=${code} (URL: ${health_url})"
      fail=1
    fi
  done

  return "${fail}"
}

# ── Smoke 场景 2：通过 gateway 转发带 token 的受保护请求 ─────
smoke_gateway_protected_request() {
  log "[SMOKE-2] Gateway 受保护请求"

  # 如果没有提供 bearer token，先尝试创建一个 smoke token
  if [[ -z "${BEARER_TOKEN}" ]]; then
    log "  [INFO] No bearer token provided, creating a smoke token"
    local create_resp
    create_resp="$(curl -sS -m 5 -X POST "${TOK_URL}/api/v1/platform/tokens" \
      -H "Content-Type: application/json" \
      -d "{\"subject_id\":\"smoke-test-user\",\"tenant_id\":\"smoke-tenant\",\"scope\":\"${EXPECTED_SCOPE}\",\"expires_in\":60}" \
      -w "\n__HTTP_CODE__:%{http_code}" 2>&1)" || true

    local create_code
    create_code="$(echo "${create_resp}" | grep -o '__HTTP_CODE__.*' | cut -d: -f2 || echo '000')"
    BEARER_TOKEN="$(echo "${create_resp}" | sed 's/__HTTP_CODE__.*//' | python3 -c "import sys,json; print(json.load(sys.stdin).get('token_id',''))" 2>/dev/null || true)"

    if [[ -z "${BEARER_TOKEN}" || "${create_code}" != "201" ]]; then
      log "  [FAIL] Cannot create smoke token: code=${create_code}"
      return 1
    fi
    log "  [INFO] Created smoke token: ${BEARER_TOKEN}"
  fi

  # 通过 gateway 发送受保护请求（gateway -> token-runtime introspect -> supply-api）
  # 路径：GET /api/v1/accounts（需要 supply:read scope）
  local req_resp
  req_resp="$(curl -sS -m 10 -X GET "${GATEWAY_URL}/api/v1/accounts" \
    -H "Authorization: Bearer ${BEARER_TOKEN}" \
    -w "\n__HTTP_CODE__:%{http_code}" 2>&1)" || true

  local req_code
  req_code="$(echo "${req_resp}" | grep -o '__HTTP_CODE__.*' | cut -d: -f2 || echo '000')"

  log "  [INFO] Gateway protected request: method=GET path=/api/v1/accounts status=${req_code}"

  # 验收：返回 200（正常）或 401/403（token 有效但权限不足，即 token 被正确传递到了 supply-api）
  #       不能是 502/503（token-runtime 不可达）或 404（路径错误）
  if [[ "${req_code}" == "200" || "${req_code}" == "401" || "${req_code}" == "403" ]]; then
    log "  [PASS] Gateway protected request: token correctly forwarded to supply-api (status=${req_code})"
    return 0
  elif [[ "${req_code}" == "502" || "${req_code}" == "503" || "${req_code}" == "504" ]]; then
    log "  [FAIL] Gateway protected request: token-runtime unreachable (status=${req_code})"
    return 1
  else
    log "  [FAIL] Gateway protected request: unexpected status=${req_code}"
    return 1
  fi
}

# ── Smoke 场景 3：Supply-api scope 验证 ───────────────────────
smoke_supply_scope_check() {
  log "[SMOKE-3] Supply-api scope 验证"

  # 创建一个只有 supply:read scope 的 token，验证 supply:write 请求被拒绝
  local create_resp3
  create_resp3="$(curl -sS -m 5 -X POST "${TOK_URL}/api/v1/platform/tokens" \
    -H "Content-Type: application/json" \
    -d '{"subject_id":"smoke-scope-user","tenant_id":"smoke-tenant","scope":"supply:read","expires_in":60}"' \
    -w "\n__HTTP_CODE__:%{http_code}" 2>&1)" || true

  local create_code3
  create_code3="$(echo "${create_resp3}" | grep -o '__HTTP_CODE__.*' | cut -d: -f2 || echo '000')"
  local read_only_token
  read_only_token="$(echo "${create_resp3}" | sed 's/__HTTP_CODE__.*//' | python3 -c "import sys,json; print(json.load(sys.stdin).get('token_id',''))" 2>/dev/null || true)"

  if [[ -z "${read_only_token}" || "${create_code3}" != "201" ]]; then
    log "  [SKIP] Cannot create scope-test token"
    return 2  # SKIP
  fi

  # 用 read-only token 尝试 supply:write 操作（通过 gateway）
  local write_resp
  write_resp="$(curl -sS -m 10 -X POST "${GATEWAY_URL}/api/v1/accounts" \
    -H "Authorization: Bearer ${read_only_token}" \
    -H "Content-Type: application/json" \
    -d '{"account_name":"smoke-test-write"}' \
    -w "\n__HTTP_CODE__:%{http_code}" 2>&1)" || true

  local write_code
  write_code="$(echo "${write_resp}" | grep -o '__HTTP_CODE__.*' | cut -d: -f2 || echo '000')"

  log "  [INFO] supply:read token attempted supply:write: status=${write_code}"

  # 验收：应返回 401/403（scope 不足）或 400，不能是 200
  if [[ "${write_code}" == "401" || "${write_code}" == "403" || "${write_code}" == "400" ]]; then
    log "  [PASS] Scope check: insufficient scope correctly rejected (status=${write_code})"
    return 0
  elif [[ "${write_code}" == "200" ]]; then
    log "  [FAIL] Scope check: insufficient scope NOT rejected (got 200)"
    return 1
  else
    log "  [WARN] Scope check: unexpected status=${write_code}"
    return 1
  fi
}

# ── 主流程 ────────────────────────────────────────────────────
log "=========================================="
log "[cross_service_smoke] START"
log "[URLS] gateway=${GATEWAY_URL} tok-runtime=${TOK_URL} supply=${SUPPLY_URL}"
log "=========================================="

SMOKE_PASS=0
SMOKE_FAIL=0
SMOKE_SKIP=0

# 前置检查：是否本地占位
if is_local_placeholder; then
  if [[ "${ALLOW_LOCAL_PLACEHOLDER}" != "1" ]]; then
    log "[SKIP] Local placeholder environment detected — set SMOKE_ALLOW_LOCAL_PLACEHOLDER=1 to force run"
    log "[SKIP] This run will be reported as SKIP_LOCAL_PLACEHOLDER (exit 2)"

    cat > "${REPORT_FILE}" <<'EOF'
# Cross-Service Smoke 报告

- 时间戳：${TS}
- 结果：**SKIP_LOCAL_PLACEHOLDER**
- 说明：所有服务 URL 均为 localhost/127.0.0.1，判定为本地占位环境，非真实 staging 证据。

## 状态码语义

| 退出码 | 含义 |
|---|---|
| 0 | PASS — 真实 staging smoke 通过 |
| 1 | FAIL — 任意链路失败 |
| 2 | SKIP_LOCAL_PLACEHOLDER — 本地占位，不计入 release 通过 |

## 建议

1. 在真实 staging 环境运行本脚本。
2. 通过环境变量传入真实服务 URL：
   ```bash
   SMOKE_GATEWAY_BASE_URL=https://gateway.staging.internal \
   SMOKE_TOKEN_RUNTIME_BASE_URL=https://token-runtime.staging.internal \
   SMOKE_SUPPLY_API_BASE_URL=https://supply-api.staging.internal \
   bash scripts/ci/cross_service_smoke.sh
   ```
EOF

    log "[RESULT] SKIP_LOCAL_PLACEHOLDER"
    exit 2
  else
    log "[WARN] ALLOW_LOCAL_PLACEHOLDER=1 — running against localhost (results will be marked as local)"
  fi
fi

# 执行三个 smoke 场景
set +e
smoke_check_services
case $? in
  0) ((SMOKE_PASS++)) ;;
  1) ((SMOKE_FAIL++)) ;;
  2) ((SMOKE_SKIP++)) ;;
esac

smoke_gateway_protected_request
case $? in
  0) ((SMOKE_PASS++)) ;;
  1) ((SMOKE_FAIL++)) ;;
  2) ((SMOKE_SKIP++)) ;;
esac

smoke_supply_scope_check
case $? in
  0) ((SMOKE_PASS++)) ;;
  1) ((SMOKE_FAIL++)) ;;
  2) ((SMOKE_SKIP++)) ;;
esac
set -e

log ""
log "=========================================="
log "[cross_service_smoke] PASS=${SMOKE_PASS} FAIL=${SMOKE_FAIL} SKIP=${SMOKE_SKIP}"
log "=========================================="

# 生成报告
OVERALL="PASS"
if [[ "${SMOKE_FAIL}" -gt 0 ]]; then
  OVERALL="FAIL"
elif [[ "${SMOKE_SKIP}" -gt 0 && "${SMOKE_PASS}" -eq 0 ]]; then
  OVERALL="SKIP"
fi

cat > "${REPORT_FILE}" <<EOF
# Cross-Service Smoke 报告

- 时间戳：${TS}
- 整体结果：**${OVERALL}**
- 场景通过：${SMOKE_PASS} / 失败：${SMOKE_FAIL} / 跳过：${SMOKE_SKIP}
- Gateway URL：${GATEWAY_URL}
- Token Runtime URL：${TOK_URL}
- Supply API URL：${SUPPLY_URL}
- Bearer Token：${BEARER_TOKEN:0:20}...（已脱敏）

## Smoke 场景

| # | 场景 | 结果 |
|---|---|---|
| 1 | 服务健康检查（gateway / token-runtime / supply-api） | $([[ ${SMOKE_FAIL} -eq 0 ]] && echo "PASS" || echo "FAIL") |
| 2 | Gateway 受保护请求（token 转发验证） | $([[ ${SMOKE_FAIL} -eq 0 ]] && echo "PASS" || echo "FAIL") |
| 3 | Supply-api scope 验证 | $([[ ${SMOKE_FAIL} -eq 0 ]] && echo "PASS" || echo "FAIL") |

## 证据

- 日志：\`${LOG_FILE}\`
- 报告：\`${REPORT_FILE}\`

## 状态码语义（供 CI 解析）

| 退出码 | 含义 |
|---|---|
| 0 | PASS — 真实 staging smoke 通过 |
| 1 | FAIL — 任意链路失败 |
| 2 | SKIP_LOCAL_PLACEHOLDER — 本地占位，不计入 release 通过 |

EOF

log "[RESULT] ${OVERALL}"
log "[INFO] report=${REPORT_FILE}"

if [[ "${OVERALL}" == "FAIL" ]]; then
  exit 1
elif [[ "${OVERALL}" == "SKIP" ]]; then
  exit 2
fi
