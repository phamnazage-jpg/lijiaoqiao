#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DATE_TAG="${1:-$(date +%F)}"
REPORT_DIR="$PROJECT_ROOT/reports/dependency"

SBOM_FILE="$REPORT_DIR/sbom_${DATE_TAG}.spdx.json"
LOCK_DIFF_FILE="$REPORT_DIR/lockfile_diff_${DATE_TAG}.md"
COMPAT_FILE="$REPORT_DIR/compat_matrix_${DATE_TAG}.md"
RISK_FILE="$REPORT_DIR/risk_register_${DATE_TAG}.md"
OUT_FILE="$REPORT_DIR/dependency_audit_result_${DATE_TAG}.md"

missing=0
for f in "$SBOM_FILE" "$LOCK_DIFF_FILE" "$COMPAT_FILE" "$RISK_FILE"; do
  if [[ ! -s "$f" ]]; then
    echo "[FAIL] missing or empty: $f"
    missing=1
  else
    echo "[OK] found: $f"
  fi
done

if [[ $missing -ne 0 ]]; then
  exit 1
fi

if ! grep -q '"spdxVersion"' "$SBOM_FILE"; then
  echo "[FAIL] sbom missing spdxVersion"
  exit 1
fi

if ! grep -q '"packages"' "$SBOM_FILE"; then
  echo "[FAIL] sbom missing packages"
  exit 1
fi

for f in "$LOCK_DIFF_FILE" "$COMPAT_FILE" "$RISK_FILE"; do
  if ! grep -q '^- Audit-Status: PASS' "$f"; then
    echo "[FAIL] audit status not PASS in: $f"
    exit 1
  fi
done

cat > "$OUT_FILE" <<REPORT
# Dependency Audit Check Result (${DATE_TAG})

- Result: PASS
- M-017 (\`dependency_compat_audit_pass_pct\`): 100%
- Checked files:
  1. ${SBOM_FILE##$PROJECT_ROOT/}
  2. ${LOCK_DIFF_FILE##$PROJECT_ROOT/}
  3. ${COMPAT_FILE##$PROJECT_ROOT/}
  4. ${RISK_FILE##$PROJECT_ROOT/}

REPORT

echo "[PASS] dependency audit check complete"
echo "result report: $OUT_FILE"
