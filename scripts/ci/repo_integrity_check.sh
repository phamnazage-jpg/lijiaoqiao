#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
LIB_FILE="${ROOT_DIR}/scripts/ci/lib/verification_common.sh"
# shellcheck disable=SC1091
source "${LIB_FILE}"

GO_BIN="$(resolve_go_bin "${ROOT_DIR}" || true)"
if [[ -z "${GO_BIN}" ]]; then
  echo "[repo] go binary not found" >&2
  exit 1
fi

setup_go_env "${GO_BIN}" "/tmp/lijiaoqiao-go-cache-repo-integrity"

check_shell_syntax() {
  echo "[repo] shell"
  bash -n "${ROOT_DIR}/supply-api/scripts/migrate.sh"
  bash -n "${ROOT_DIR}/supply-api/scripts/run_integration_tests.sh"
}

echo "[repo] fact-sources"
check_fact_sources "${ROOT_DIR}"
check_shell_syntax
run_go_suite "${ROOT_DIR}" "${GO_BIN}" "gateway" "gateway" test ./...
run_go_suite "${ROOT_DIR}" "${GO_BIN}" "platform-token-runtime" "platform-token-runtime" test ./...
run_go_suite "${ROOT_DIR}" "${GO_BIN}" "supply-api unit" "supply-api" test ./...
run_go_suite "${ROOT_DIR}" "${GO_BIN}" "supply-api e2e" "supply-api" test -tags=e2e ./e2e
