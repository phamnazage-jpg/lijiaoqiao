#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STATE_DIR="${ROOT_DIR}/.tmp/devtest"
LOG_DIR="${STATE_DIR}/logs"
PID_DIR="${STATE_DIR}/pids"
ENV_FILE="${STATE_DIR}/env.sh"

PG_CONTAINER_NAME="${LIJIAOQIAO_DEVTEST_PG_CONTAINER:-lijiaoqiao-devtest-postgres}"
PG_HOST="${LIJIAOQIAO_DEVTEST_PG_HOST:-127.0.0.1}"
PG_PORT="${LIJIAOQIAO_DEVTEST_PG_PORT:-15440}"
PG_USER="${LIJIAOQIAO_DEVTEST_PG_USER:-lijiaoqiao}"
PG_PASSWORD="${LIJIAOQIAO_DEVTEST_PG_PASSWORD:-secret}"
PG_IMAGE="${LIJIAOQIAO_DEVTEST_PG_IMAGE:-docker.io/library/postgres:15-alpine}"

SUPPLY_DB="${LIJIAOQIAO_DEVTEST_SUPPLY_DB:-supply_devtest}"
TOKEN_DB="${LIJIAOQIAO_DEVTEST_TOKEN_DB:-token_runtime_devtest}"

MOCK_OPENAI_ADDR="${LIJIAOQIAO_DEVTEST_MOCK_OPENAI_ADDR:-127.0.0.1:19090}"
TOKEN_RUNTIME_ADDR="${LIJIAOQIAO_DEVTEST_TOKEN_RUNTIME_ADDR:-127.0.0.1:18081}"
SUPPLY_API_ADDR="${LIJIAOQIAO_DEVTEST_SUPPLY_API_ADDR:-:18082}"
GATEWAY_HOST="${LIJIAOQIAO_DEVTEST_GATEWAY_HOST:-127.0.0.1}"
GATEWAY_PORT="${LIJIAOQIAO_DEVTEST_GATEWAY_PORT:-18080}"

SUPPLY_TOKEN_SECRET_KEY="${LIJIAOQIAO_DEVTEST_SUPPLY_TOKEN_SECRET_KEY:-devtest-secret-key-12345678901234567890}"
SUPPLY_TOKEN_ISSUER="${LIJIAOQIAO_DEVTEST_SUPPLY_TOKEN_ISSUER:-lijiaoqiao/supply-api}"

OPENAI_MODELS="${LIJIAOQIAO_DEVTEST_OPENAI_MODELS:-gpt-4o-mini,gpt-4o,gpt-4.1,o3-mini,claude-3-5-sonnet,claude-3-7-sonnet,gemini-2.0-flash,deepseek-chat}"

mkdir -p "${LOG_DIR}" "${PID_DIR}"

require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "missing required command: $1" >&2
        exit 1
    fi
}

require_cmd podman
require_cmd psql
require_cmd pg_isready
require_cmd curl
require_cmd go

psql_exec() {
    local database="$1"
    shift
    PGPASSWORD="${PG_PASSWORD}" psql \
        -v ON_ERROR_STOP=1 \
        -h "${PG_HOST}" \
        -p "${PG_PORT}" \
        -U "${PG_USER}" \
        -d "${database}" \
        "$@"
}

wait_for_pg() {
    local attempts=0
    until PGPASSWORD="${PG_PASSWORD}" pg_isready -h "${PG_HOST}" -p "${PG_PORT}" -U "${PG_USER}" >/dev/null 2>&1; do
        attempts=$((attempts + 1))
        if [[ "${attempts}" -ge 60 ]]; then
            echo "postgres did not become ready on ${PG_HOST}:${PG_PORT}" >&2
            exit 1
        fi
        sleep 1
    done
}

ensure_pg_container() {
    if podman container exists "${PG_CONTAINER_NAME}"; then
        if [[ "$(podman inspect -f '{{.State.Running}}' "${PG_CONTAINER_NAME}")" != "true" ]]; then
            podman start "${PG_CONTAINER_NAME}" >/dev/null
        fi
    else
        podman run -d \
            --name "${PG_CONTAINER_NAME}" \
            -p "${PG_HOST}:${PG_PORT}:5432" \
            -e POSTGRES_USER="${PG_USER}" \
            -e POSTGRES_PASSWORD="${PG_PASSWORD}" \
            -e POSTGRES_DB=postgres \
            "${PG_IMAGE}" >/dev/null
    fi

    wait_for_pg
}

ensure_database() {
    local database="$1"
    local exists
    exists="$(PGPASSWORD="${PG_PASSWORD}" psql -tA -h "${PG_HOST}" -p "${PG_PORT}" -U "${PG_USER}" -d postgres -c "SELECT 1 FROM pg_database WHERE datname='${database}'" | tr -d '[:space:]')"
    if [[ "${exists}" != "1" ]]; then
        PGPASSWORD="${PG_PASSWORD}" createdb -h "${PG_HOST}" -p "${PG_PORT}" -U "${PG_USER}" "${database}"
    fi
}

apply_supply_schema() {
    psql_exec "${SUPPLY_DB}" -c "CREATE EXTENSION IF NOT EXISTS pgcrypto;"
    psql_exec "${SUPPLY_DB}" -f "${ROOT_DIR}/scripts/devtest/sql/supply_iam_prereqs.sql"
    psql_exec "${SUPPLY_DB}" -f "${ROOT_DIR}/supply-api/sql/postgresql/supply_core_schema_v2.sql"
    psql_exec "${SUPPLY_DB}" -f "${ROOT_DIR}/supply-api/sql/postgresql/partition_strategy_v1.sql"
    psql_exec "${SUPPLY_DB}" -f "${ROOT_DIR}/supply-api/sql/postgresql/outbox_pattern_v1.sql"
    psql_exec "${SUPPLY_DB}" -f "${ROOT_DIR}/supply-api/sql/postgresql/token_status_registry_v1.sql"
    psql_exec "${SUPPLY_DB}" -f "${ROOT_DIR}/supply-api/sql/postgresql/audit_alerts_v1.sql"
    psql_exec "${SUPPLY_DB}" -f "${ROOT_DIR}/sql/postgresql/iam_schema_v1.sql"
}

apply_token_runtime_schema() {
    psql_exec "${TOKEN_DB}" -c "CREATE EXTENSION IF NOT EXISTS pgcrypto;"
    psql_exec "${TOKEN_DB}" -f "${ROOT_DIR}/sql/postgresql/token_runtime_schema_v1.sql"
}

write_env_file() {
    cat >"${ENV_FILE}" <<EOF
export LIJIAOQIAO_DEVTEST_STATE_DIR="${STATE_DIR}"
export LIJIAOQIAO_DEVTEST_LOG_DIR="${LOG_DIR}"
export LIJIAOQIAO_DEVTEST_PID_DIR="${PID_DIR}"
export LIJIAOQIAO_DEVTEST_PG_CONTAINER="${PG_CONTAINER_NAME}"
export LIJIAOQIAO_DEVTEST_PG_HOST="${PG_HOST}"
export LIJIAOQIAO_DEVTEST_PG_PORT="${PG_PORT}"
export LIJIAOQIAO_DEVTEST_PG_USER="${PG_USER}"
export LIJIAOQIAO_DEVTEST_PG_PASSWORD="${PG_PASSWORD}"
export LIJIAOQIAO_DEVTEST_SUPPLY_DB="${SUPPLY_DB}"
export LIJIAOQIAO_DEVTEST_TOKEN_DB="${TOKEN_DB}"
export LIJIAOQIAO_DEVTEST_SUPPLY_DSN="postgres://${PG_USER}:${PG_PASSWORD}@${PG_HOST}:${PG_PORT}/${SUPPLY_DB}?sslmode=disable"
export LIJIAOQIAO_DEVTEST_TOKEN_RUNTIME_DSN="postgres://${PG_USER}:${PG_PASSWORD}@${PG_HOST}:${PG_PORT}/${TOKEN_DB}?sslmode=disable"
export LIJIAOQIAO_DEVTEST_MOCK_OPENAI_ADDR="${MOCK_OPENAI_ADDR}"
export LIJIAOQIAO_DEVTEST_TOKEN_RUNTIME_ADDR="${TOKEN_RUNTIME_ADDR}"
export LIJIAOQIAO_DEVTEST_SUPPLY_API_ADDR="${SUPPLY_API_ADDR}"
export LIJIAOQIAO_DEVTEST_GATEWAY_HOST="${GATEWAY_HOST}"
export LIJIAOQIAO_DEVTEST_GATEWAY_PORT="${GATEWAY_PORT}"
export LIJIAOQIAO_DEVTEST_OPENAI_MODELS="${OPENAI_MODELS}"
export LIJIAOQIAO_DEVTEST_SUPPLY_TOKEN_SECRET_KEY="${SUPPLY_TOKEN_SECRET_KEY}"
export LIJIAOQIAO_DEVTEST_SUPPLY_TOKEN_ISSUER="${SUPPLY_TOKEN_ISSUER}"
EOF
}

start_process() {
    local name="$1"
    local command="$2"
    local pid_file="${PID_DIR}/${name}.pid"
    local log_file="${LOG_DIR}/${name}.log"

    if [[ -f "${pid_file}" ]]; then
        local pid
        pid="$(cat "${pid_file}")"
        if kill -0 "${pid}" >/dev/null 2>&1; then
            echo "[devtest] ${name} already running (pid=${pid})"
            return 0
        fi
        rm -f "${pid_file}"
    fi

    nohup bash -lc "${command}" >"${log_file}" 2>&1 &
    local pid=$!
    echo "${pid}" >"${pid_file}"
    echo "[devtest] started ${name} (pid=${pid})"
}

wait_http() {
    local name="$1"
    local url="$2"
    local attempts=0
    until curl -fsS "${url}" >/dev/null 2>&1; do
        attempts=$((attempts + 1))
        if [[ "${attempts}" -ge 60 ]]; then
            echo "[devtest] ${name} did not become ready: ${url}" >&2
            exit 1
        fi
        sleep 1
    done
}

ensure_pg_container
ensure_database "${SUPPLY_DB}"
ensure_database "${TOKEN_DB}"
apply_supply_schema
apply_token_runtime_schema
write_env_file

start_process "mock-openai" \
    "cd \"${ROOT_DIR}/supply-api\" && GOCACHE=\"${STATE_DIR}/go-cache/mock-openai\" go run ./cmd/devtestctl mock-openai --addr \"${MOCK_OPENAI_ADDR}\" --models \"${OPENAI_MODELS}\""
wait_http "mock-openai" "http://${MOCK_OPENAI_ADDR}/healthz"

start_process "platform-token-runtime" \
    "cd \"${ROOT_DIR}/platform-token-runtime\" && TOKEN_RUNTIME_ADDR=\"${TOKEN_RUNTIME_ADDR}\" TOKEN_RUNTIME_ENV=\"prod\" TOKEN_RUNTIME_DATABASE_URL=\"postgres://${PG_USER}:${PG_PASSWORD}@${PG_HOST}:${PG_PORT}/${TOKEN_DB}?sslmode=disable\" GOCACHE=\"${STATE_DIR}/go-cache/token-runtime\" go run ./cmd/platform-token-runtime"
wait_http "platform-token-runtime" "http://${TOKEN_RUNTIME_ADDR}/actuator/health"

start_process "supply-api" \
    "cd \"${ROOT_DIR}/supply-api\" && SUPPLY_API_ADDR=\"${SUPPLY_API_ADDR}\" SUPPLY_DB_HOST=\"${PG_HOST}\" SUPPLY_DB_PORT=\"${PG_PORT}\" SUPPLY_DB_USER=\"${PG_USER}\" SUPPLY_DB_PASSWORD=\"${PG_PASSWORD}\" SUPPLY_DB_NAME=\"${SUPPLY_DB}\" SUPPLY_API_IAM_ENABLED=\"true\" SUPPLY_TOKEN_SECRET_KEY=\"${SUPPLY_TOKEN_SECRET_KEY}\" SUPPLY_TOKEN_ISSUER=\"${SUPPLY_TOKEN_ISSUER}\" GOCACHE=\"${STATE_DIR}/go-cache/supply-api\" go run ./cmd/supply-api -env=staging -config ./config/config.dev.yaml"
wait_http "supply-api" "http://127.0.0.1${SUPPLY_API_ADDR#:}/actuator/health"

start_process "gateway" \
    "cd \"${ROOT_DIR}/gateway\" && GATEWAY_HOST=\"${GATEWAY_HOST}\" GATEWAY_PORT=\"${GATEWAY_PORT}\" GATEWAY_ENV=\"staging\" GATEWAY_TOKEN_RUNTIME_MODE=\"remote_introspection\" GATEWAY_TOKEN_RUNTIME_URL=\"http://${TOKEN_RUNTIME_ADDR}\" OPENAI_BASE_URL=\"http://${MOCK_OPENAI_ADDR}\" OPENAI_API_KEY=\"mock-devtest-key\" OPENAI_MODELS=\"${OPENAI_MODELS}\" GOCACHE=\"${STATE_DIR}/go-cache/gateway\" go run ./cmd/gateway"
wait_http "gateway" "http://${GATEWAY_HOST}:${GATEWAY_PORT}/health"

echo "[devtest] stack is ready"
echo "[devtest] env file: ${ENV_FILE}"
