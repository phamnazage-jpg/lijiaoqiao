#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_ID="${RUN_ID:-gatec-rollback-$(date +%Y%m%d%H%M%S)}"
ARTIFACT_DIR="${ARTIFACT_DIR:-/tmp/ai-customer-service-gate-c-rollback/$RUN_ID}"
GO_HELPER_DIR="$ROOT_DIR/.tmp/verify_gate_c_rollback/$RUN_ID"
SUMMARY_FILE="$ARTIFACT_DIR/summary.txt"
BASELINE_LOG_FILE="$ARTIFACT_DIR/baseline-service.log"
BROKEN_LOG_FILE="$ARTIFACT_DIR/broken-service.log"
ROLLED_BACK_LOG_FILE="$ARTIFACT_DIR/rolled-back-service.log"
DEFAULT_APP_BIN="$ARTIFACT_DIR/ai-customer-service"
APP_BIN="${APP_BIN:-$DEFAULT_APP_BIN}"

mkdir -p "$ARTIFACT_DIR"
mkdir -p "$GO_HELPER_DIR"

PASS_COUNT=0
FAIL_COUNT=0
APP_PID=""
BASE_URL=""
BROKEN_AI_CS_POSTGRES_DSN="${BROKEN_AI_CS_POSTGRES_DSN:-}"

log() {
  printf '%s\n' "$*" | tee -a "$SUMMARY_FILE"
}

pass() {
  PASS_COUNT=$((PASS_COUNT + 1))
  log "[PASS] $*"
}

fail() {
  FAIL_COUNT=$((FAIL_COUNT + 1))
  log "[FAIL] $*"
  exit 1
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "missing command: $1"
  fi
}

require_env() {
  local key="$1"
  if [[ -z "${!key:-}" ]]; then
    fail "missing required env: $key"
  fi
}

stop_service() {
  if [[ -n "$APP_PID" ]] && kill -0 "$APP_PID" >/dev/null 2>&1; then
    kill "$APP_PID" >/dev/null 2>&1 || true
    wait "$APP_PID" >/dev/null 2>&1 || true
  fi
  APP_PID=""
}

cleanup() {
  stop_service
}

trap cleanup EXIT

extract_base_url() {
  local addr="$1"
  local host=""
  local port=""
  if [[ "$addr" == :* ]]; then
    host="127.0.0.1"
    port="${addr#:}"
  else
    host="${addr%:*}"
    port="${addr##*:}"
    if [[ -z "$host" || "$host" == "$addr" ]]; then
      fail "AI_CS_ADDR must be host:port or :port, got: $addr"
    fi
    if [[ "$host" == "0.0.0.0" ]]; then
      host="127.0.0.1"
    fi
  fi
  printf 'http://%s:%s' "$host" "$port"
}

derive_broken_dsn() {
  python3 - "$AI_CS_POSTGRES_DSN" <<'PY'
import re
import sys
dsn = sys.argv[1]
if dsn.startswith("postgres://") or dsn.startswith("postgresql://"):
    if re.search(r":\d+/", dsn):
        print(re.sub(r":\d+/", ":1/", dsn, count=1), end="")
    else:
        print(dsn, end="")
elif "port=" in dsn:
    print(re.sub(r"port=\d+", "port=1", dsn, count=1), end="")
else:
    print(f"{dsn} port=1", end="")
PY
}

DB_QUERY_HELPER="$GO_HELPER_DIR/db_query.go"

cat >"$DB_QUERY_HELPER" <<'EOF'
package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	dsn := os.Getenv("DB_DSN")
	query := os.Getenv("SQL_QUERY")
	if dsn == "" || query == "" {
		fmt.Fprintln(os.Stderr, "DB_DSN and SQL_QUERY are required")
		os.Exit(2)
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}

	var value string
	if err := db.QueryRow(query).Scan(&value); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}
	fmt.Print(value)
}
EOF

db_value() {
  local sql="$1"
  DB_DSN="$AI_CS_POSTGRES_DSN" SQL_QUERY="$sql" go run "$DB_QUERY_HELPER"
}

assert_eq() {
  local actual="$1"
  local expected="$2"
  local message="$3"
  if [[ "$actual" != "$expected" ]]; then
    fail "$message (got=$actual want=$expected)"
  fi
  pass "$message"
}

assert_nonzero_count() {
  local actual="$1"
  local message="$2"
  if [[ "$actual" =~ ^[1-9][0-9]*$ ]]; then
    pass "$message"
    return
  fi
  fail "$message (got=$actual want>=1)"
}

start_service_with_env() {
  local dsn="$1"
  local log_file="$2"
  stop_service
  (
    cd "$ROOT_DIR"
    AI_CS_RUNTIME_ENV="$AI_CS_RUNTIME_ENV" \
    AI_CS_ADDR="$AI_CS_ADDR" \
    AI_CS_POSTGRES_ENABLED="$AI_CS_POSTGRES_ENABLED" \
    AI_CS_POSTGRES_DSN="$dsn" \
    AI_CS_POSTGRES_MIGRATION_DIR="$AI_CS_POSTGRES_MIGRATION_DIR" \
    AI_CS_WEBHOOK_SECRET="$AI_CS_WEBHOOK_SECRET" \
    AI_CS_WEBHOOK_TIMESTAMP_HEADER="$AI_CS_WEBHOOK_TIMESTAMP_HEADER" \
    AI_CS_WEBHOOK_SIGNATURE_HEADER="$AI_CS_WEBHOOK_SIGNATURE_HEADER" \
    AI_CS_WEBHOOK_MAX_SKEW_SECONDS="$AI_CS_WEBHOOK_MAX_SKEW_SECONDS" \
    "$APP_BIN"
  ) >"$log_file" 2>&1 &
  APP_PID=$!
}

wait_ready() {
  local log_file="$1"
  local ready_ok=""
  for _ in $(seq 1 30); do
    if curl -fsS "$BASE_URL/actuator/health/live" >/dev/null 2>&1 && curl -fsS "$BASE_URL/actuator/health/ready" >/dev/null 2>&1; then
      ready_ok="yes"
      break
    fi
    sleep 1
  done
  if [[ "$ready_ok" != "yes" ]]; then
    tail -100 "$log_file" | tee -a "$SUMMARY_FILE" >/dev/null || true
    fail "service did not become live+ready"
  fi
}

wait_broken_startup() {
  local log_file="$1"
  for _ in $(seq 1 12); do
    if [[ -n "$APP_PID" ]] && ! kill -0 "$APP_PID" >/dev/null 2>&1; then
      pass "broken release process exited as expected"
      return
    fi
    if curl -fsS "$BASE_URL/actuator/health/ready" >/dev/null 2>&1; then
      tail -100 "$log_file" | tee -a "$SUMMARY_FILE" >/dev/null || true
      fail "broken release unexpectedly became ready"
    fi
    sleep 1
  done
  if curl -fsS "$BASE_URL/actuator/health/ready" >/dev/null 2>&1; then
    tail -100 "$log_file" | tee -a "$SUMMARY_FILE" >/dev/null || true
    fail "broken release unexpectedly became ready after timeout"
  fi
  pass "broken release never became ready"
}

send_signed_webhook() {
  local message_id="$1"
  local open_id="$2"
  local response_file="$3"
  local body_file="$ARTIFACT_DIR/${message_id}.json"
  MESSAGE_ID="$message_id" OPEN_ID="$open_id" python3 >"$body_file" <<'PY'
import json
import os
import sys

payload = {
    "message_id": os.environ["MESSAGE_ID"],
    "channel": "widget",
    "open_id": os.environ["OPEN_ID"],
    "content": "我要退款",
}
sys.stdout.write(json.dumps(payload, ensure_ascii=False, separators=(",", ":")))
PY

  local ts
  ts="$(date +%s)"
  local sig
  sig="$(python3 - "$ts" "$body_file" "$AI_CS_WEBHOOK_SECRET" <<'PY'
import hashlib
import hmac
import sys

timestamp, body_path, secret = sys.argv[1], sys.argv[2], sys.argv[3]
with open(body_path, "rb") as fh:
    body = fh.read()
payload = timestamp.encode("utf-8") + b"." + body
print(hmac.new(secret.encode("utf-8"), payload, hashlib.sha256).hexdigest(), end="")
PY
)"

  curl -sS -o "$response_file" -w '%{http_code}' \
    -X POST "$BASE_URL/api/v1/customer-service/webhook" \
    -H "Content-Type: application/json" \
    -H "$AI_CS_WEBHOOK_TIMESTAMP_HEADER: $ts" \
    -H "$AI_CS_WEBHOOK_SIGNATURE_HEADER: $sig" \
    --data-binary "@$body_file"
}

extract_response_field() {
  local response_file="$1"
  local field="$2"
  python3 - "$response_file" "$field" <<'PY'
import json
import sys
with open(sys.argv[1], "r", encoding="utf-8") as fh:
    data = json.load(fh)
value = data.get(sys.argv[2], "")
if isinstance(value, bool):
    print(str(value).lower(), end="")
else:
    print(value, end="")
PY
}

log "# verify_gate_c_rollback.sh"
log "run_id=$RUN_ID"
log "artifact_dir=$ARTIFACT_DIR"
log "root_dir=$ROOT_DIR"

require_cmd curl
require_cmd go
require_cmd openssl
require_cmd python3
pass "required commands available"

require_env AI_CS_RUNTIME_ENV
require_env AI_CS_ADDR
require_env AI_CS_POSTGRES_ENABLED
require_env AI_CS_POSTGRES_DSN
require_env AI_CS_POSTGRES_MIGRATION_DIR
require_env AI_CS_WEBHOOK_SECRET

AI_CS_WEBHOOK_TIMESTAMP_HEADER="${AI_CS_WEBHOOK_TIMESTAMP_HEADER:-X-CS-Timestamp}"
AI_CS_WEBHOOK_SIGNATURE_HEADER="${AI_CS_WEBHOOK_SIGNATURE_HEADER:-X-CS-Signature}"
AI_CS_WEBHOOK_MAX_SKEW_SECONDS="${AI_CS_WEBHOOK_MAX_SKEW_SECONDS:-300}"
BASE_URL="$(extract_base_url "$AI_CS_ADDR")"

if [[ -z "$BROKEN_AI_CS_POSTGRES_DSN" ]]; then
  BROKEN_AI_CS_POSTGRES_DSN="$(derive_broken_dsn)"
fi

assert_eq "$AI_CS_RUNTIME_ENV" "production" "runtime env is production"
assert_eq "$AI_CS_POSTGRES_ENABLED" "true" "postgres mode enabled for rollback drill"

if [[ ! -d "$AI_CS_POSTGRES_MIGRATION_DIR" ]]; then
  fail "migration dir not found: $AI_CS_POSTGRES_MIGRATION_DIR"
fi
pass "migration dir exists: $AI_CS_POSTGRES_MIGRATION_DIR"

if [[ "$APP_BIN" == "$DEFAULT_APP_BIN" ]]; then
  (
    cd "$ROOT_DIR"
    go build -o "$APP_BIN" ./cmd/ai-customer-service
  )
  pass "built current source into rollback drill app binary: $APP_BIN"
elif [[ ! -x "$APP_BIN" ]]; then
  fail "app binary is not executable: $APP_BIN"
else
  pass "using provided executable app binary: $APP_BIN"
fi

if [[ -n "$(db_value "SELECT '1'")" ]]; then
  pass "postgres connectivity check passed"
else
  fail "postgres connectivity check returned empty result"
fi

BASELINE_MESSAGE_ID="${RUN_ID}-baseline-message"
BASELINE_OPEN_ID="${RUN_ID}-baseline-open"
BASELINE_RESP_FILE="$ARTIFACT_DIR/baseline_webhook_response.json"

start_service_with_env "$AI_CS_POSTGRES_DSN" "$BASELINE_LOG_FILE"
pass "baseline service process started (pid=$APP_PID)"
wait_ready "$BASELINE_LOG_FILE"
pass "baseline service live and ready probes passed"

HTTP_CODE="$(send_signed_webhook "$BASELINE_MESSAGE_ID" "$BASELINE_OPEN_ID" "$BASELINE_RESP_FILE")"
assert_eq "$HTTP_CODE" "200" "baseline signed webhook request returned HTTP 200"
assert_eq "$(extract_response_field "$BASELINE_RESP_FILE" "received")" "true" "baseline webhook response received=true"
assert_eq "$(extract_response_field "$BASELINE_RESP_FILE" "handoff")" "true" "baseline webhook response handoff=true"

stop_service
pass "baseline service stopped before broken release"

start_service_with_env "$BROKEN_AI_CS_POSTGRES_DSN" "$BROKEN_LOG_FILE"
pass "broken release process started (pid=$APP_PID)"
wait_broken_startup "$BROKEN_LOG_FILE"

start_service_with_env "$AI_CS_POSTGRES_DSN" "$ROLLED_BACK_LOG_FILE"
pass "rollback restart process started (pid=$APP_PID)"
wait_ready "$ROLLED_BACK_LOG_FILE"
pass "rolled-back service live and ready probes passed"

ROLLED_BACK_MESSAGE_ID="${RUN_ID}-rollback-message"
ROLLED_BACK_OPEN_ID="${RUN_ID}-rollback-open"
ROLLED_BACK_RESP_FILE="$ARTIFACT_DIR/rolled_back_webhook_response.json"
HTTP_CODE="$(send_signed_webhook "$ROLLED_BACK_MESSAGE_ID" "$ROLLED_BACK_OPEN_ID" "$ROLLED_BACK_RESP_FILE")"
assert_eq "$HTTP_CODE" "200" "rolled-back signed webhook request returned HTTP 200"
assert_eq "$(extract_response_field "$ROLLED_BACK_RESP_FILE" "received")" "true" "rolled-back webhook response received=true"
assert_eq "$(extract_response_field "$ROLLED_BACK_RESP_FILE" "handoff")" "true" "rolled-back webhook response handoff=true"

ROLLED_BACK_TICKET_ID="$(extract_response_field "$ROLLED_BACK_RESP_FILE" "ticket_id")"
ROLLED_BACK_SESSION_ID="$(extract_response_field "$ROLLED_BACK_RESP_FILE" "session_id")"
if [[ -z "$ROLLED_BACK_TICKET_ID" || -z "$ROLLED_BACK_SESSION_ID" ]]; then
  fail "rolled-back webhook response missing ticket_id or session_id"
fi
pass "rolled-back webhook response returned ticket_id and session_id"

assert_eq "$(db_value "SELECT status FROM cs_tickets WHERE id = '$ROLLED_BACK_TICKET_ID'::uuid")" "open" "rolled-back webhook created open ticket"
assert_nonzero_count "$(db_value "SELECT COUNT(*)::text FROM cs_message_dedup WHERE channel = 'widget' AND message_id = '$ROLLED_BACK_MESSAGE_ID'")" "rolled-back webhook persisted dedup row"
assert_nonzero_count "$(db_value "SELECT COUNT(*)::text FROM cs_audit_logs WHERE object_type = 'message_processed' AND action = 'process' AND actor_id = '$ROLLED_BACK_OPEN_ID'")" "rolled-back webhook persisted message_processed audit"
assert_nonzero_count "$(db_value "SELECT COUNT(*)::text FROM cs_tickets t JOIN cs_sessions s ON s.id = t.session_id WHERE s.channel = 'widget' AND s.open_id = '$ROLLED_BACK_OPEN_ID'")" "rolled-back webhook persisted ticket linked to session"

pass "gate-c rollback drill completed successfully"
log "baseline_message_id=$BASELINE_MESSAGE_ID"
log "rolled_back_message_id=$ROLLED_BACK_MESSAGE_ID"
log "rolled_back_ticket_id=$ROLLED_BACK_TICKET_ID"
log "rolled_back_session_id=$ROLLED_BACK_SESSION_ID"
log "baseline_log_file=$BASELINE_LOG_FILE"
log "broken_log_file=$BROKEN_LOG_FILE"
log "rolled_back_log_file=$ROLLED_BACK_LOG_FILE"
log "summary: pass=$PASS_COUNT fail=$FAIL_COUNT"
