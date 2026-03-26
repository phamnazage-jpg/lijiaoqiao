#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE_DEFAULT="${ROOT_DIR}/scripts/supply-gate/.env"
ENV_FILE="${1:-${ENV_FILE_DEFAULT}}"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "missing env file: ${ENV_FILE}"
  echo "copy scripts/supply-gate/.env.example to scripts/supply-gate/.env and edit it."
  exit 1
fi

# shellcheck disable=SC1090
source "${ENV_FILE}"

require_bin() {
  local b="$1"
  if ! command -v "${b}" >/dev/null 2>&1; then
    echo "missing required binary: ${b}"
    exit 1
  fi
}

require_var() {
  local n="$1"
  if [[ -z "${!n:-}" ]]; then
    echo "missing required env var: ${n}"
    exit 1
  fi
}

json_get() {
  local expr="$1"
  jq -r "${expr} // empty"
}

init_artifact_dir() {
  local case_id="$1"
  local dir="${ROOT_DIR}/tests/supply/artifacts/${case_id}"
  mkdir -p "${dir}"
  echo "${dir}"
}

curl_json() {
  local method="$1"
  local url="$2"
  local token="$3"
  local data="${4:-}"
  if [[ -n "${data}" ]]; then
    curl -sS -X "${method}" \
      -H "Authorization: Bearer ${token}" \
      -H "Content-Type: application/json" \
      -d "${data}" \
      "${url}"
  else
    curl -sS -X "${method}" \
      -H "Authorization: Bearer ${token}" \
      "${url}"
  fi
}
