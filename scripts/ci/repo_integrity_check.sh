#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
GO_BIN="${ROOT_DIR}/.tools/go-current/bin/go"

if [[ ! -x "${GO_BIN}" ]]; then
  GO_BIN="$(command -v go || true)"
fi

if [[ -z "${GO_BIN}" ]]; then
  echo "[repo] go binary not found" >&2
  exit 1
fi

export PATH="$(dirname "${GO_BIN}"):${PATH}"
export GOCACHE="${GOCACHE:-/tmp/lijiaoqiao-go-cache-repo-integrity}"

run_suite() {
  local label="$1"
  local workdir="$2"
  shift 2

  echo "[repo] ${label}"
  (
    cd "${ROOT_DIR}/${workdir}"
    "${GO_BIN}" "$@"
  )
}

run_suite "gateway" "gateway" test ./...
run_suite "platform-token-runtime" "platform-token-runtime" test ./...
run_suite "supply-api unit" "supply-api" test ./...
run_suite "supply-api e2e" "supply-api" test -tags=e2e ./e2e
