#!/usr/bin/env bash

resolve_go_bin() {
  local root_dir="$1"
  local go_bin="${root_dir}/.tools/go-current/bin/go"

  if [[ ! -x "${go_bin}" ]]; then
    go_bin="$(command -v go || true)"
  fi

  if [[ -z "${go_bin}" ]]; then
    return 1
  fi

  echo "${go_bin}"
}

setup_go_env() {
  local go_bin="$1"
  local default_gocache="${2:-}"
  local default_gopath=""
  local default_gomodcache=""

  export PATH="$(dirname "${go_bin}"):${PATH}"

  if [[ -n "${default_gocache}" ]]; then
    export GOCACHE="${GOCACHE:-${default_gocache}}"
  fi

  default_gopath="$("${go_bin}" env GOPATH 2>/dev/null || true)"
  default_gomodcache="$("${go_bin}" env GOMODCACHE 2>/dev/null || true)"

  if [[ -n "${default_gopath}" ]]; then
    export GOPATH="${default_gopath}"
  fi
  if [[ -n "${default_gomodcache}" ]]; then
    export GOMODCACHE="${default_gomodcache}"
  fi
}

run_go_suite() {
  local root_dir="$1"
  local go_bin="$2"
  local label="$3"
  local workdir="$4"
  shift 4

  echo "[repo] ${label}"
  (
    cd "${root_dir}/${workdir}"
    "${go_bin}" "$@"
  )
}

check_fact_sources() {
  local root_dir="$1"
  local matches

  matches="$(
    rg -n "platform_core_schema_v1\\.sql|AUDIT_QUERY_NOT_READY|not implemented" \
      "${root_dir}/gateway/README.md" \
      "${root_dir}/platform-token-runtime/README.md" \
      "${root_dir}/platform-token-runtime/internal/httpapi/token_api.go" \
      "${root_dir}/supply-api/README.md" \
      "${root_dir}/supply-api/scripts/migrate.sh" || true
  )"

  if [[ -n "${matches}" ]]; then
    echo "${matches}" >&2
    return 1
  fi
}

write_step_result() {
  local -n result_ref="$1"
  local step_id="$2"
  local status="$3"
  local title="$4"
  local evidence="$5"

  result_ref+=("${step_id}|${status}|${title}|${evidence}")
}
