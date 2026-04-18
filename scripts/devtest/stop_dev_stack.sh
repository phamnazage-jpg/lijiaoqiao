#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STATE_DIR="${ROOT_DIR}/.tmp/devtest"
PID_DIR="${STATE_DIR}/pids"
WITH_DB="${1:-}"

stop_pid_file() {
    local pid_file="$1"
    if [[ ! -f "${pid_file}" ]]; then
        return 0
    fi

    local pid
    pid="$(cat "${pid_file}")"
    if kill -0 "${pid}" >/dev/null 2>&1; then
        kill "${pid}" >/dev/null 2>&1 || true
        for _ in {1..20}; do
            if ! kill -0 "${pid}" >/dev/null 2>&1; then
                break
            fi
            sleep 1
        done
        if kill -0 "${pid}" >/dev/null 2>&1; then
            kill -9 "${pid}" >/dev/null 2>&1 || true
        fi
    fi

    rm -f "${pid_file}"
}

if [[ -d "${PID_DIR}" ]]; then
    stop_pid_file "${PID_DIR}/gateway.pid"
    stop_pid_file "${PID_DIR}/supply-api.pid"
    stop_pid_file "${PID_DIR}/platform-token-runtime.pid"
    stop_pid_file "${PID_DIR}/mock-openai.pid"
fi

if [[ "${WITH_DB}" == "--with-db" ]]; then
    if [[ -f "${STATE_DIR}/env.sh" ]]; then
        # shellcheck disable=SC1090
        source "${STATE_DIR}/env.sh"
        if command -v podman >/dev/null 2>&1 && podman container exists "${LIJIAOQIAO_DEVTEST_PG_CONTAINER}" >/dev/null 2>&1; then
            podman stop "${LIJIAOQIAO_DEVTEST_PG_CONTAINER}" >/dev/null 2>&1 || true
        fi
    fi
fi

echo "[devtest] stack stopped"
