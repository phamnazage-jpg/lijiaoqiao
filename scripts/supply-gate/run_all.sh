#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="${1:-${SCRIPT_DIR}/.env}"

bash "${SCRIPT_DIR}/sup004_accounts.sh" "${ENV_FILE}"
bash "${SCRIPT_DIR}/sup005_packages.sh" "${ENV_FILE}"
bash "${SCRIPT_DIR}/sup006_settlements.sh" "${ENV_FILE}"
bash "${SCRIPT_DIR}/sup007_boundary.sh" "${ENV_FILE}"

echo "SUP-004~SUP-007 scripts finished."
echo "next: fill reports in tests/supply/*.md and reports/supply_gate_review_2026-03-31.md"
