#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_ID="${RUN_ID:-gateb-$(date +%Y%m%d%H%M%S)}"
ARTIFACT_DIR="${ARTIFACT_DIR:-/tmp/ai-customer-service-preprod-gate-b/$RUN_ID}"
GO_HELPER_DIR="$ROOT_DIR/.tmp/verify_preprod_gate_b/$RUN_ID"
LOG_FILE="$ARTIFACT_DIR/service.log"
WEBHOOK_BODY_FILE="$ARTIFACT_DIR/webhook_body.json"
WEBHOOK_RESP_FILE="$ARTIFACT_DIR/webhook_response.json"
WEBHOOK_HEADERS_FILE="$ARTIFACT_DIR/webhook_headers.txt"
DEDUP_RESP_FILE="$ARTIFACT_DIR/dedup_response.json"
SUMMARY_FILE="$ARTIFACT_DIR/summary.txt"
DEFAULT_APP_BIN="$ARTIFACT_DIR/ai-customer-service"
APP_BIN="${APP_BIN:-$DEFAULT_APP_BIN}"

mkdir -p "$ARTIFACT_DIR"
mkdir -p "$GO_HELPER_DIR"

PASS_COUNT=0
FAIL_COUNT=0
APP_PID=""
TICKET_ID=""
SESSION_ID=""
MESSAGE_ID=""
OPEN_ID=""

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

cleanup() {
  if [[ -n "$APP_PID" ]] && kill -0 "$APP_PID" >/dev/null 2>&1; then
    kill "$APP_PID" >/dev/null 2>&1 || true
    wait "$APP_PID" >/dev/null 2>&1 || true
  fi
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

log "# verify_preprod_gate_b.sh"
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

assert_eq "$AI_CS_RUNTIME_ENV" "production" "runtime env is production"
assert_eq "$AI_CS_POSTGRES_ENABLED" "true" "postgres mode enabled for gate-b validation"

if [[ ! -d "$AI_CS_POSTGRES_MIGRATION_DIR" ]]; then
  fail "migration dir not found: $AI_CS_POSTGRES_MIGRATION_DIR"
fi
pass "migration dir exists: $AI_CS_POSTGRES_MIGRATION_DIR"

if [[ "$APP_BIN" == "$DEFAULT_APP_BIN" ]]; then
  (
    cd "$ROOT_DIR"
    go build -o "$APP_BIN" ./cmd/ai-customer-service
  )
  pass "built current source into gate-b app binary: $APP_BIN"
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

if [[ "$(db_value "SELECT CASE WHEN EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='cs_schema_migrations') THEN 'true' ELSE 'false' END")" != "true" ]]; then
  fail "cs_schema_migrations table missing"
fi
pass "migration bookkeeping table exists"

if [[ "$(db_value "SELECT CASE WHEN EXISTS (SELECT 1 FROM cs_schema_migrations WHERE version='0001_init') THEN 'true' ELSE 'false' END")" != "true" ]]; then
  fail "required migration version 0001_init not recorded"
fi
pass "migration version 0001_init is recorded"

(
  cd "$ROOT_DIR"
  "$APP_BIN"
) >"$LOG_FILE" 2>&1 &
APP_PID=$!
pass "service process started (pid=$APP_PID)"

ready_ok=""
for _ in $(seq 1 30); do
  if curl -fsS "$BASE_URL/actuator/health/live" >/dev/null 2>&1 && curl -fsS "$BASE_URL/actuator/health/ready" >/dev/null 2>&1; then
    ready_ok="yes"
    break
  fi
  sleep 1
done
if [[ "$ready_ok" != "yes" ]]; then
  tail -100 "$LOG_FILE" | tee -a "$SUMMARY_FILE" >/dev/null
  fail "service did not become live+ready"
fi
pass "service live and ready probes passed"

MESSAGE_ID="${RUN_ID}-message"
OPEN_ID="${RUN_ID}-open"
export MESSAGE_ID OPEN_ID
python3 >"$WEBHOOK_BODY_FILE" <<'PY'
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

TS="$(date +%s)"
SIG="$(python3 - "$TS" "$WEBHOOK_BODY_FILE" "$AI_CS_WEBHOOK_SECRET" <<'PY'
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
HTTP_CODE="$(curl -sS -o "$WEBHOOK_RESP_FILE" -D "$WEBHOOK_HEADERS_FILE" -w '%{http_code}' \
  -X POST "$BASE_URL/api/v1/customer-service/webhook" \
  -H "Content-Type: application/json" \
  -H "$AI_CS_WEBHOOK_TIMESTAMP_HEADER: $TS" \
  -H "$AI_CS_WEBHOOK_SIGNATURE_HEADER: $SIG" \
  --data-binary "@$WEBHOOK_BODY_FILE")"
assert_eq "$HTTP_CODE" "200" "signed webhook request returned HTTP 200"

readarray -t WEBHOOK_PARSED < <(python3 - "$WEBHOOK_RESP_FILE" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    data = json.load(fh)
print(str(data.get("received", False)).lower())
print(str(data.get("handoff", False)).lower())
print(data.get("ticket_id", ""))
print(data.get("session_id", ""))
print(data.get("reply", ""))
PY
)

assert_eq "${WEBHOOK_PARSED[0]}" "true" "webhook response received=true"
assert_eq "${WEBHOOK_PARSED[1]}" "true" "webhook response handoff=true"
TICKET_ID="${WEBHOOK_PARSED[2]}"
SESSION_ID="${WEBHOOK_PARSED[3]}"
if [[ -z "$TICKET_ID" || -z "$SESSION_ID" ]]; then
  fail "webhook response missing ticket_id or session_id"
fi
pass "webhook response returned ticket_id and session_id"

assert_eq "$(db_value "SELECT status FROM cs_tickets WHERE id = '$TICKET_ID'::uuid")" "open" "ticket inserted in postgres with open status"
assert_eq "$(db_value "SELECT COUNT(*)::text FROM cs_audit_logs WHERE object_type = 'message_processed' AND object_id = '$SESSION_ID' AND action = 'process' AND after_state->>'ticket_id' = '$TICKET_ID'")" "1" "message_processed audit row persisted with ticket_id"
assert_eq "$(db_value "SELECT COUNT(*)::text FROM cs_message_dedup WHERE channel = 'widget' AND message_id = '$MESSAGE_ID'")" "1" "dedup row created for first webhook message"

HTTP_CODE="$(curl -sS -o "$DEDUP_RESP_FILE" -w '%{http_code}' \
  -X POST "$BASE_URL/api/v1/customer-service/webhook" \
  -H "Content-Type: application/json" \
  -H "$AI_CS_WEBHOOK_TIMESTAMP_HEADER: $TS" \
  -H "$AI_CS_WEBHOOK_SIGNATURE_HEADER: $SIG" \
  --data-binary "@$WEBHOOK_BODY_FILE")"
assert_eq "$HTTP_CODE" "200" "duplicate webhook request returned HTTP 200"

DEDUP_REPLY="$(python3 - "$DEDUP_RESP_FILE" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    data = json.load(fh)
print(data.get("reply", ""))
PY
)"
assert_eq "$DEDUP_REPLY" "duplicate message ignored" "duplicate webhook request is deduplicated"
assert_eq "$(db_value "SELECT COUNT(*)::text FROM cs_message_dedup WHERE channel = 'widget' AND message_id = '$MESSAGE_ID'")" "1" "dedup table still contains exactly one row for duplicate message"

HTTP_CODE="$(curl -sS -o /dev/null -w '%{http_code}' \
  -X POST "$BASE_URL/api/v1/customer-service/tickets/$TICKET_ID/assign?agent_id=gate-b-agent" \
  -H "X-CS-Actor-ID: gate-b-supervisor" \
  -H "X-CS-Actor-Role: supervisor")"
assert_eq "$HTTP_CODE" "200" "ticket assign API returned HTTP 200"
assert_eq "$(db_value "SELECT status FROM cs_tickets WHERE id = '$TICKET_ID'::uuid")" "assigned" "ticket status becomes assigned after assign"

HTTP_CODE="$(curl -sS -o /dev/null -w '%{http_code}' \
  -X POST "$BASE_URL/api/v1/customer-service/tickets/$TICKET_ID/resolve?resolution=handled-by-gate-b" \
  -H "X-CS-Actor-ID: gate-b-agent" \
  -H "X-CS-Actor-Role: agent")"
assert_eq "$HTTP_CODE" "200" "ticket resolve API returned HTTP 200"
assert_eq "$(db_value "SELECT status FROM cs_tickets WHERE id = '$TICKET_ID'::uuid")" "resolved" "ticket status becomes resolved after resolve"

HTTP_CODE="$(curl -sS -o /dev/null -w '%{http_code}' \
  -X POST "$BASE_URL/api/v1/customer-service/tickets/$TICKET_ID/close?resolution=confirmed-by-gate-b" \
  -H "X-CS-Actor-ID: gate-b-supervisor" \
  -H "X-CS-Actor-Role: supervisor")"
assert_eq "$HTTP_CODE" "200" "ticket close API returned HTTP 200"
assert_eq "$(db_value "SELECT status FROM cs_tickets WHERE id = '$TICKET_ID'::uuid")" "closed" "ticket status becomes closed after close"

assert_eq "$(db_value "SELECT COUNT(*)::text FROM cs_audit_logs WHERE object_type = 'ticket' AND object_id = '$TICKET_ID' AND action = 'assign'")" "2" "assign audit persisted in both workflow store and handler layers"
assert_eq "$(db_value "SELECT COUNT(*)::text FROM cs_audit_logs WHERE object_type = 'ticket' AND object_id = '$TICKET_ID' AND action = 'resolve'")" "2" "resolve audit persisted in both workflow store and handler layers"
assert_eq "$(db_value "SELECT COUNT(*)::text FROM cs_audit_logs WHERE object_type = 'ticket' AND object_id = '$TICKET_ID' AND action = 'close'")" "2" "close audit persisted in both workflow store and handler layers"

pass "gate-b verification completed successfully"
log "ticket_id=$TICKET_ID"
log "session_id=$SESSION_ID"
log "message_id=$MESSAGE_ID"
log "log_file=$LOG_FILE"
log "summary: pass=$PASS_COUNT fail=$FAIL_COUNT"
