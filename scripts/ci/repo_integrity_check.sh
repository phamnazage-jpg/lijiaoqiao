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
run_go_suite "${ROOT_DIR}" "${GO_BIN}" "gateway" "gateway" test -count=1 ./...
run_go_suite "${ROOT_DIR}" "${GO_BIN}" "platform-token-runtime" "platform-token-runtime" test -count=1 ./...
run_go_suite "${ROOT_DIR}" "${GO_BIN}" "supply-api unit" "supply-api" test -count=1 ./...
echo "[repo] supply-api repository integration"
(
  cd "${ROOT_DIR}/supply-api"
  bash scripts/run_integration_tests.sh ./internal/repository
)
run_go_suite "${ROOT_DIR}" "${GO_BIN}" "supply-api service-http" "supply-api" test -count=1 -tags=e2e ./e2e

# Phase 1 contract gate entry:
# - execute after service-local suites and repository integration pass
# - command entry: bash "${ROOT_DIR}/scripts/ci/backend-verify.sh" --phase1-contract-gate
# - primary artifacts:
#     reports/archive/gate_verification/contract_gate_<timestamp>.log
#     reports/archive/gate_verification/contract_gate_<timestamp>.md
# - failure semantics: if the contract gate exits non-zero or any required scenario is missing,
#   repo_integrity_check must fail and Phase 1 cannot be marked complete.
echo "[repo] Phase 1 contract gate (SCENARIO-1~4)"
if ! bash "${ROOT_DIR}/scripts/ci/backend-verify.sh" --phase1-contract-gate >> "${ROOT_DIR}/reports/archive/gate_verification/repo_integrity_contract_gate_${TS}.log" 2>&1; then
  echo "[repo] contract gate FAILED — see contract_gate_*.log in reports/archive/gate_verification/"
  exit 1
fi

# Phase 2 boundary note:
# - repo_integrity_check only proves code completeness, syntax, unit/integration and service-local HTTP coverage.
# - cross_service_smoke belongs to release/path validation and must not be redefined as repository integrity.
# - even after smoke is introduced, this script remains a code integrity gate rather than a release gate.
