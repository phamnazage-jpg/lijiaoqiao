#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
GO_BIN="${ROOT_DIR}/.tools/go-current/bin/go"

if [[ ! -x "${GO_BIN}" ]]; then
  GO_BIN="$(command -v go || true)"
fi

if [[ -z "${GO_BIN}" ]]; then
  echo "[repo] go binary not found" >&2
  exit 1
fi

export PATH="$(dirname "${GO_BIN}"):${PATH}"
export GOCACHE="${GOCACHE:-/tmp/lijiaoqiao-go-cache-repo-integrity}"

check_current_fact_sources() {
  echo "[repo] fact-sources"

  local matches
  matches="$(
    rg -n "platform_core_schema_v1\\.sql|AUDIT_QUERY_NOT_READY|not implemented" \
      "${ROOT_DIR}/gateway/README.md" \
      "${ROOT_DIR}/platform-token-runtime/README.md" \
      "${ROOT_DIR}/platform-token-runtime/internal/httpapi/token_api.go" \
      "${ROOT_DIR}/supply-api/README.md" \
      "${ROOT_DIR}/supply-api/scripts/migrate.sh" || true
  )"

  if [[ -n "${matches}" ]]; then
    echo "${matches}" >&2
    exit 1
  fi
}

check_shell_syntax() {
  echo "[repo] shell"
  bash -n "${ROOT_DIR}/supply-api/scripts/migrate.sh"
  bash -n "${ROOT_DIR}/supply-api/scripts/run_integration_tests.sh"
}

run_suite() {
  local label="$1"
  local workdir="$2"
  shift 2

  echo "[repo] ${label}"
  (
    cd "${ROOT_DIR}/${workdir}"
    "${GO_BIN}" "$@"
  )
}

check_current_fact_sources
check_shell_syntax
run_suite "gateway" "gateway" test ./...
run_suite "platform-token-runtime" "platform-token-runtime" test ./...
run_suite "supply-api unit" "supply-api" test ./...
run_suite "supply-api e2e" "supply-api" test -tags=e2e ./e2e
