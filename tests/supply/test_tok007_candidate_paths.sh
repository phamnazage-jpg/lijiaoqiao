#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
CURRENT_POINTER="${ROOT_DIR}/review/outputs/current_machine_review_sources.md"

rg -n 'review/outputs/current_machine_review_sources\.md' \
  "${ROOT_DIR}/review/README.md" \
  "${ROOT_DIR}/docs/plans/2026-04-14-repo-integrity-refactor-plan.md" >/dev/null

rg -n 'mark_historical_snapshots\.sh' \
  "${ROOT_DIR}/scripts/ci/tok007_release_recheck.sh" \
  "${ROOT_DIR}/scripts/ci/tok007_generate_final_decision_candidate.sh" >/dev/null

if [[ ! -f "${CURRENT_POINTER}" ]]; then
  echo "[FAIL] current machine-review pointer missing: ${CURRENT_POINTER}"
  exit 1
fi

CURRENT_TOK007_REL="$(sed -n 's/^- 当前 TOK-007 复审稿：`\([^`]*\)`/\1/p' "${CURRENT_POINTER}" | head -n 1 || true)"
CURRENT_FINAL_REL="$(sed -n 's/^- 当前最终决议候选稿：`\([^`]*\)`/\1/p' "${CURRENT_POINTER}" | head -n 1 || true)"

if [[ -z "${CURRENT_TOK007_REL}" || -z "${CURRENT_FINAL_REL}" ]]; then
  echo "[FAIL] current machine-review pointer is incomplete: ${CURRENT_POINTER}"
  exit 1
fi

LATEST_TOK007_REL="$(find "${ROOT_DIR}/review/outputs" -maxdepth 1 -type f -name 'tok007_release_recheck_*.md' | sed "s#^${ROOT_DIR}/##" | sort | tail -n 1)"
LATEST_FINAL_REL="$(find "${ROOT_DIR}/review/outputs" -maxdepth 1 -type f -name 'final_decision_candidate_from_tok007_*.md' | sed "s#^${ROOT_DIR}/##" | sort | tail -n 1)"

if [[ "${CURRENT_TOK007_REL}" != "${LATEST_TOK007_REL}" ]]; then
  echo "[FAIL] current tok007 pointer mismatch: ${CURRENT_TOK007_REL} != ${LATEST_TOK007_REL}"
  exit 1
fi
if [[ "${CURRENT_FINAL_REL}" != "${LATEST_FINAL_REL}" ]]; then
  echo "[FAIL] current final pointer mismatch: ${CURRENT_FINAL_REL} != ${LATEST_FINAL_REL}"
  exit 1
fi

echo "[PASS] tok007 current-source pointer and automation wiring look correct"
