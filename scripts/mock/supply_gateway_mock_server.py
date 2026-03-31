#!/usr/bin/env python3
import json
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import parse_qs, urlparse


STATE = {
    "next_account_id": 1000,
    "next_package_id": 2000,
    "next_settlement_id": 3000,
    "accounts": {},
    "packages": {},
    "settlements": {},
}


def json_bytes(payload):
    return json.dumps(payload, ensure_ascii=True).encode("utf-8")


class Handler(BaseHTTPRequestHandler):
    server_version = "supply-mock/1.0"

    def _read_json(self):
        length = int(self.headers.get("Content-Length", "0"))
        if length <= 0:
            return {}
        body = self.rfile.read(length)
        try:
            return json.loads(body.decode("utf-8"))
        except Exception:
            return {}

    def _write(self, status, payload):
        data = json_bytes(payload)
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def _ok(self, payload):
        self._write(200, {"code": 0, "message": "ok", "data": payload})

    def do_GET(self):
        parsed = urlparse(self.path)
        path = parsed.path
        query = parse_qs(parsed.query)

        if path.startswith("/api/v1/supply/accounts/") and path.endswith("/audit-logs"):
            account_id = path.split("/")[5]
            self._ok(
                {
                    "items": [
                        {
                            "request_id": f"req-audit-{account_id}",
                            "action": "state_change",
                            "result": "success",
                        }
                    ],
                    "page": 1,
                    "page_size": 20,
                    "total": 1,
                }
            )
            return

        if path == "/api/v1/supplier/billing":
            self._ok(
                {
                    "summary": {"total_amount": 123.45, "currency": "USD"},
                    "items": [],
                    "page": int(query.get("page", ["1"])[0]),
                    "page_size": int(query.get("page_size", ["20"])[0]),
                    "total": 0,
                }
            )
            return

        if path.startswith("/api/v1/supply/settlements/") and path.endswith("/statement"):
            settlement_id = path.split("/")[5]
            self._ok(
                {
                    "settlement_id": int(settlement_id),
                    "download_url": f"http://127.0.0.1:18080/mock/statement/{settlement_id}.csv",
                }
            )
            return

        if path == "/api/v1/supply/earnings/records":
            self._ok(
                {
                    "items": [
                        {
                            "record_id": 1,
                            "amount": 10,
                            "currency_code": "USD",
                            "status": "available",
                        }
                    ],
                    "page": int(query.get("page", ["1"])[0]),
                    "page_size": int(query.get("page_size", ["20"])[0]),
                    "total": 1,
                }
            )
            return

        if path == "/v1beta/models":
            # External query key should be rejected.
            self._write(
                403,
                {
                    "code": 403,
                    "message": "query key rejected",
                    "data": {"reason": "external_query_key_forbidden"},
                },
            )
            return

        if path == "/actuator/health":
            self._write(200, {"status": "UP"})
            return

        self._write(404, {"code": 404, "message": "not found", "data": None})

    def do_POST(self):
        path = urlparse(self.path).path
        payload = self._read_json()

        if path == "/api/v1/supply/accounts/verify":
            self._ok(
                {
                    "verify_status": "pass",
                    "risk_level": "normal",
                    "provider": payload.get("provider", "openai"),
                }
            )
            return

        if path == "/api/v1/supply/accounts":
            account_id = STATE["next_account_id"]
            STATE["next_account_id"] += 1
            STATE["accounts"][str(account_id)] = {"status": "pending"}
            self._ok({"account_id": account_id, "status": "pending"})
            return

        if path.startswith("/api/v1/supply/accounts/") and path.endswith("/activate"):
            account_id = path.split("/")[5]
            STATE["accounts"].setdefault(account_id, {})["status"] = "active"
            self._ok({"account_id": int(account_id), "status": "active"})
            return

        if path.startswith("/api/v1/supply/accounts/") and path.endswith("/suspend"):
            account_id = path.split("/")[5]
            STATE["accounts"].setdefault(account_id, {})["status"] = "suspended"
            self._ok({"account_id": int(account_id), "status": "suspended"})
            return

        if path == "/api/v1/supply/packages/draft":
            package_id = STATE["next_package_id"]
            STATE["next_package_id"] += 1
            STATE["packages"][str(package_id)] = {"status": "draft"}
            self._ok({"package_id": package_id, "status": "draft"})
            return

        if path.startswith("/api/v1/supply/packages/") and path.endswith("/publish"):
            package_id = path.split("/")[5]
            STATE["packages"].setdefault(package_id, {})["status"] = "active"
            self._ok({"package_id": int(package_id), "status": "active"})
            return

        if path.startswith("/api/v1/supply/packages/") and path.endswith("/pause"):
            package_id = path.split("/")[5]
            STATE["packages"].setdefault(package_id, {})["status"] = "paused"
            self._ok({"package_id": int(package_id), "status": "paused"})
            return

        if path.startswith("/api/v1/supply/packages/") and path.endswith("/unlist"):
            package_id = path.split("/")[5]
            STATE["packages"].setdefault(package_id, {})["status"] = "expired"
            self._ok({"package_id": int(package_id), "status": "expired"})
            return

        if path == "/api/v1/supply/packages/batch-price":
            items = payload.get("items", [])
            self._ok(
                {
                    "total": len(items),
                    "success_count": len(items),
                    "failed_count": 0,
                    "failed_items": [],
                }
            )
            return

        if path.startswith("/api/v1/supply/packages/") and path.endswith("/clone"):
            package_id = STATE["next_package_id"]
            STATE["next_package_id"] += 1
            STATE["packages"][str(package_id)] = {"status": "draft"}
            self._ok({"package_id": package_id, "status": "draft"})
            return

        if path == "/api/v1/supply/settlements/withdraw":
            settlement_id = STATE["next_settlement_id"]
            STATE["next_settlement_id"] += 1
            STATE["settlements"][str(settlement_id)] = {"status": "pending"}
            self._ok({"settlement_id": settlement_id, "status": "pending"})
            return

        if path.startswith("/api/v1/supply/settlements/") and path.endswith("/cancel"):
            settlement_id = path.split("/")[5]
            STATE["settlements"].setdefault(settlement_id, {})["status"] = "cancelled"
            self._ok({"settlement_id": int(settlement_id), "status": "cancelled"})
            return

        if path == "/api/v1/chat/completions":
            self._ok(
                {
                    "id": "chatcmpl-mock-001",
                    "object": "chat.completion",
                    "choices": [
                        {
                            "index": 0,
                            "message": {"role": "assistant", "content": "pong"},
                            "finish_reason": "stop",
                        }
                    ],
                }
            )
            return

        self._write(404, {"code": 404, "message": "not found", "data": None})

    def log_message(self, format, *args):
        return


if __name__ == "__main__":
    server = HTTPServer(("127.0.0.1", 18080), Handler)
    server.serve_forever()
