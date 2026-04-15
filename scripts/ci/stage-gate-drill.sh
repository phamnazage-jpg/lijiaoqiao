#!/usr/bin/env bash
set -euo pipefail

FAIL_STAGE="${1:-G3}"
DATE_TAG="${2:-$(date +%F)}"
PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT_DIR="$PROJECT_ROOT/reports/archive/gate_verification"
mkdir -p "$OUT_DIR"
LOG_FILE="$OUT_DIR/stage_gate_drill_${DATE_TAG}.log"

stages=(G0 G1 G2 G3 G4 G5)

: > "$LOG_FILE"

log() {
  echo "$1" | tee -a "$LOG_FILE"
}

log "[INFO] stage gate drill start, fail_stage=$FAIL_STAGE, date=$DATE_TAG"
pass_count=0
failed=0
failed_stage=""
rollback_to=""

for s in "${stages[@]}"; do
  if [[ "$s" == "$FAIL_STAGE" ]]; then
    log "[FAIL] $s quality gate check failed: simulated contract drift"
    failed=1
    failed_stage="$s"
    break
  fi
  log "[PASS] $s quality gate check passed"
  pass_count=$((pass_count+1))
done

if [[ $failed -eq 0 ]]; then
  log "[INFO] no failure injected; drill considered invalid"
  exit 2
fi

case "$failed_stage" in
  G0) rollback_to="G0" ;;
  G1) rollback_to="G0" ;;
  G2) rollback_to="G1" ;;
  G3) rollback_to="G2" ;;
  G4) rollback_to="G3" ;;
  G5) rollback_to="G4" ;;
  *) rollback_to="G0" ;;
esac

log "[ACTION] rollback triggered: from $failed_stage to $rollback_to"
log "[ACTION] freeze subsequent promotion stages"
log "[ACTION] open corrective task with 24h SLA"
log "[PASS] rollback drill complete"

echo "LOG_FILE=$LOG_FILE"
echo "PASS_COUNT=$pass_count"
echo "FAILED_STAGE=$failed_stage"
echo "ROLLBACK_TO=$rollback_to"
