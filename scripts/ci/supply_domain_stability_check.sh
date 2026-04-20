#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SUPPLY_API_DIR="${ROOT_DIR}/supply-api"
ITERATIONS="${1:-20}"

if ! [[ "${ITERATIONS}" =~ ^[0-9]+$ ]] || [[ "${ITERATIONS}" -lt 1 ]]; then
  echo "usage: bash scripts/ci/supply_domain_stability_check.sh [iterations>=1]" >&2
  exit 1
fi

cd "${SUPPLY_API_DIR}"

for i in $(seq 1 "${ITERATIONS}"); do
  echo "[supply-domain-stability] iteration=${i}/${ITERATIONS}"
  go test -count=1 ./internal/domain
done

echo "[supply-domain-stability] PASS iterations=${ITERATIONS}"
