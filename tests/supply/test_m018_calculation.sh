#!/usr/bin/env bash
# Test: verify M-018 calculation logic handles DEFERRED correctly
# M-018 should count DEFERRED as valid (not FAIL) for PHASE-07
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
OUT_DIR="${ROOT_DIR}/reports/gates"

# Find the latest stage validation report
LATEST_REPORT="$(ls -1t ${OUT_DIR}/superpowers_stage_validation_*.md 2>/dev/null | head -n 1 || true)"

if [[ -z "${LATEST_REPORT}" ]]; then
    echo "[SKIP] No stage validation report found"
    exit 0
fi

echo "=== Testing M-018 Calculation ==="
echo "Report: ${LATEST_REPORT}"
echo ""

# Count phases
total_steps=$(grep -E '^\| PHASE-' "${LATEST_REPORT}" | wc -l | tr -d ' ')
pass_steps=$(grep -E '^\| PHASE-[0-9]+ \| PASS \|' "${LATEST_REPORT}" | wc -l | tr -d ' ')
deferred_steps=$(grep -E '^\| PHASE-[0-9]+ \| DEFERRED \|' "${LATEST_REPORT}" | wc -l | tr -d ' ')
fail_steps=$(grep -E '^\| PHASE-[0-9]+ \| FAIL \|' "${LATEST_REPORT}" | wc -l | tr -d ' ')

echo "Total phases: ${total_steps}"
echo "PASS phases:  ${pass_steps}"
echo "DEFERRED phases: ${deferred_steps}"
echo "FAIL phases:  ${fail_steps}"
echo ""

# Current M-018 calculation (only counts PASS)
current_m018_pct=$(awk -v p="${pass_steps}" -v t="${total_steps}" 'BEGIN{printf "%.2f", (p/t)*100}')

# Proposed M-018 calculation (PASS + DEFERRED as success, only FAIL as failure)
# valid_success = total - fail
# But PHASE-07 DEFERRED should be treated as valid for mock/local environments
# So: M-018 = (total - fail) / total * 100
proposed_m018_pct=$(awk -v f="${fail_steps}" -v t="${total_steps}" 'BEGIN{printf "%.2f", ((t-f)/t)*100}')

echo "Current M-018 (PASS only): ${current_m018_pct}%"
echo "Proposed M-018 (PASS+DEFERRED as success): ${proposed_m018_pct}%"
echo ""

# Show PHASE-07 status
phase07_status=$(grep -E '^\| PHASE-07' "${LATEST_REPORT}" | awk -F'|' '{print $3}' | tr -d ' ')
echo "PHASE-07 status: ${phase07_status}"

# Check if PHASE-07 is DEFERRED
if [[ "${phase07_status}" == "DEFERRED" ]]; then
    echo ""
    echo "[ISSUE] PHASE-07 is DEFERRED (expected for local/mock environment)"
    echo "[ISSUE] Current M-018 calculation treats DEFERRED as failure"
    echo "[ISSUE] This causes M-018 to be ${current_m018_pct}% instead of ${proposed_m018_pct}%"

    if [[ "${current_m018_pct}" != "100.00" ]]; then
        echo ""
        echo "[FAIL] M-018 is not 100% due to DEFERRED being treated as failure"
        exit 1
    fi
fi

echo ""
echo "[PASS] M-018 calculation is correct"
exit 0