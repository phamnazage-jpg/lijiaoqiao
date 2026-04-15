#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
LIB_FILE="${ROOT_DIR}/scripts/ci/lib/verification_common.sh"

if [[ ! -f "${LIB_FILE}" ]]; then
  echo "[FAIL] shared verification library missing: ${LIB_FILE}"
  exit 1
fi

for fn in resolve_go_bin setup_go_env run_go_suite check_fact_sources write_step_result; do
  if ! rg -n "^${fn}\\(\\)" "${LIB_FILE}" >/dev/null 2>&1; then
    echo "[FAIL] shared verification function missing: ${fn}"
    exit 1
  fi
done

for script in \
  "${ROOT_DIR}/scripts/ci/repo_integrity_check.sh" \
  "${ROOT_DIR}/scripts/ci/backend-verify.sh" \
  "${ROOT_DIR}/scripts/ci/superpowers_stage_validate.sh"; do
  if ! rg -n "verification_common\\.sh" "${script}" >/dev/null 2>&1; then
    echo "[FAIL] script does not source verification_common.sh: ${script}"
    exit 1
  fi
done

echo "[PASS] shared verification library is wired into all entry scripts"
