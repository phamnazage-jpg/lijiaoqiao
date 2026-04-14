package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lijiaoqiao/platform-token-runtime/internal/auth/service"
)

func TestTokenAPIIssueAndIntrospect(t *testing.T) {
	t.Parallel()

	runtime := service.NewInMemoryTokenRuntime(nil)
	auditor := service.NewMemoryAuditStore()
	api := NewTokenAPI(runtime, auditor, func() time.Time {
		return time.Date(2026, 3, 30, 15, 50, 0, 0, time.UTC)
	})
	mux := http.NewServeMux()
	api.Register(mux)

	issueBody := map[string]any{
		"subject_id":  "2001",
		"role":        "owner",
		"ttl_seconds": 600,
		"scope":       []string{"supply:*"},
	}
	issueReq := httptest.NewRequest(http.MethodPost, "/api/v1/platform/tokens/issue", mustJSON(t, issueBody))
	issueReq.Header.Set("X-Request-Id", "req-api-001")
	issueReq.Header.Set("Idempotency-Key", "idem-api-001")
	issueRec := httptest.NewRecorder()
	mux.ServeHTTP(issueRec, issueReq)

	if issueRec.Code != http.StatusCreated {
		t.Fatalf("unexpected issue status: got=%d want=%d body=%s", issueRec.Code, http.StatusCreated, issueRec.Body.String())
	}
	issueResp := decodeMap(t, issueRec.Body.Bytes())
	data := issueResp["data"].(map[string]any)
	accessToken := data["access_token"].(string)
	if accessToken == "" {
		t.Fatalf("access_token should not be empty")
	}

	introspectBody := map[string]any{"token": accessToken}
	introReq := httptest.NewRequest(http.MethodPost, "/api/v1/platform/tokens/introspect", mustJSON(t, introspectBody))
	introReq.Header.Set("X-Request-Id", "req-api-002")
	introRec := httptest.NewRecorder()
	mux.ServeHTTP(introRec, introReq)

	if introRec.Code != http.StatusOK {
		t.Fatalf("unexpected introspect status: got=%d want=%d body=%s", introRec.Code, http.StatusOK, introRec.Body.String())
	}
	introResp := decodeMap(t, introRec.Body.Bytes())
	introData := introResp["data"].(map[string]any)
	if introData["role"].(string) != "owner" {
		t.Fatalf("unexpected role: got=%s want=owner", introData["role"].(string))
	}
}

func TestTokenAPIIssueIdempotencyConflict(t *testing.T) {
	t.Parallel()

	runtime := service.NewInMemoryTokenRuntime(nil)
	api := NewTokenAPI(runtime, service.NewMemoryAuditStore(), time.Now)
	mux := http.NewServeMux()
	api.Register(mux)

	firstBody := map[string]any{
		"subject_id":  "2001",
		"role":        "owner",
		"ttl_seconds": 600,
		"scope":       []string{"supply:*"},
	}
	secondBody := map[string]any{
		"subject_id":  "2001",
		"role":        "owner",
		"ttl_seconds": 600,
		"scope":       []string{"supply:read"},
	}

	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/platform/tokens/issue", mustJSON(t, firstBody))
	firstReq.Header.Set("X-Request-Id", "req-api-003-1")
	firstReq.Header.Set("Idempotency-Key", "idem-api-003")
	firstRec := httptest.NewRecorder()
	mux.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("first issue should succeed: code=%d body=%s", firstRec.Code, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/platform/tokens/issue", mustJSON(t, secondBody))
	secondReq.Header.Set("X-Request-Id", "req-api-003-2")
	secondReq.Header.Set("Idempotency-Key", "idem-api-003")
	secondRec := httptest.NewRecorder()
	mux.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusConflict {
		t.Fatalf("expected idempotency conflict: code=%d body=%s", secondRec.Code, secondRec.Body.String())
	}
}

func TestTokenAPIRefreshAndRevoke(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 30, 16, 0, 0, 0, time.UTC)
	runtime := service.NewInMemoryTokenRuntime(func() time.Time { return now })
	api := NewTokenAPI(runtime, service.NewMemoryAuditStore(), func() time.Time { return now })
	mux := http.NewServeMux()
	api.Register(mux)

	issueReq := httptest.NewRequest(http.MethodPost, "/api/v1/platform/tokens/issue", mustJSON(t, map[string]any{
		"subject_id":  "2008",
		"role":        "owner",
		"ttl_seconds": 120,
		"scope":       []string{"supply:*"},
	}))
	issueReq.Header.Set("X-Request-Id", "req-api-004-1")
	issueReq.Header.Set("Idempotency-Key", "idem-api-004")
	issueRec := httptest.NewRecorder()
	mux.ServeHTTP(issueRec, issueReq)
	if issueRec.Code != http.StatusCreated {
		t.Fatalf("issue failed: code=%d body=%s", issueRec.Code, issueRec.Body.String())
	}
	issued := decodeMap(t, issueRec.Body.Bytes())
	issuedData := issued["data"].(map[string]any)
	tokenID := issuedData["token_id"].(string)

	now = now.Add(10 * time.Second)
	refreshReq := httptest.NewRequest(http.MethodPost, "/api/v1/platform/tokens/"+tokenID+"/refresh", mustJSON(t, map[string]any{"ttl_seconds": 300}))
	refreshReq.Header.Set("X-Request-Id", "req-api-004-2")
	refreshReq.Header.Set("Idempotency-Key", "idem-api-004-r")
	refreshRec := httptest.NewRecorder()
	mux.ServeHTTP(refreshRec, refreshReq)
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("refresh failed: code=%d body=%s", refreshRec.Code, refreshRec.Body.String())
	}
	refreshResp := decodeMap(t, refreshRec.Body.Bytes())
	refreshData := refreshResp["data"].(map[string]any)
	if refreshData["previous_expires_at"] == nil {
		t.Fatalf("previous_expires_at must not be nil")
	}

	revokeReq := httptest.NewRequest(http.MethodPost, "/api/v1/platform/tokens/"+tokenID+"/revoke", mustJSON(t, map[string]any{"reason": "operator_request"}))
	revokeReq.Header.Set("X-Request-Id", "req-api-004-3")
	revokeReq.Header.Set("Idempotency-Key", "idem-api-004-v")
	revokeRec := httptest.NewRecorder()
	mux.ServeHTTP(revokeRec, revokeReq)
	if revokeRec.Code != http.StatusOK {
		t.Fatalf("revoke failed: code=%d body=%s", revokeRec.Code, revokeRec.Body.String())
	}
	revokeResp := decodeMap(t, revokeRec.Body.Bytes())
	revokeData := revokeResp["data"].(map[string]any)
	if revokeData["status"].(string) != "revoked" {
		t.Fatalf("unexpected status after revoke: got=%s", revokeData["status"].(string))
	}
}

func TestTokenAPIMissingHeaders(t *testing.T) {
	t.Parallel()

	runtime := service.NewInMemoryTokenRuntime(nil)
	api := NewTokenAPI(runtime, service.NewMemoryAuditStore(), time.Now)
	mux := http.NewServeMux()
	api.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/platform/tokens/issue", mustJSON(t, map[string]any{
		"subject_id":  "2001",
		"role":        "owner",
		"ttl_seconds": 120,
		"scope":       []string{"supply:*"},
	}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing headers must be rejected: code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTokenAPIAuditEventsQuery(t *testing.T) {
	t.Parallel()

	runtime := service.NewInMemoryTokenRuntime(nil)
	auditor := service.NewMemoryAuditStore()
	api := NewTokenAPI(runtime, auditor, time.Now)
	mux := http.NewServeMux()
	api.Register(mux)

	issueReq := httptest.NewRequest(http.MethodPost, "/api/v1/platform/tokens/issue", mustJSON(t, map[string]any{
		"subject_id":  "2010",
		"role":        "owner",
		"ttl_seconds": 300,
		"scope":       []string{"supply:*"},
	}))
	issueReq.Header.Set("X-Request-Id", "req-audit-query-1")
	issueReq.Header.Set("Idempotency-Key", "idem-audit-query-1")
	issueRec := httptest.NewRecorder()
	mux.ServeHTTP(issueRec, issueReq)
	if issueRec.Code != http.StatusCreated {
		t.Fatalf("issue failed: code=%d body=%s", issueRec.Code, issueRec.Body.String())
	}
	issueResp := decodeMap(t, issueRec.Body.Bytes())
	tokenID := issueResp["data"].(map[string]any)["token_id"].(string)

	queryReq := httptest.NewRequest(http.MethodGet, "/api/v1/platform/tokens/audit-events?token_id="+tokenID+"&limit=5", nil)
	queryReq.Header.Set("X-Request-Id", "req-audit-query-2")
	queryRec := httptest.NewRecorder()
	mux.ServeHTTP(queryRec, queryReq)
	if queryRec.Code != http.StatusOK {
		t.Fatalf("audit query failed: code=%d body=%s", queryRec.Code, queryRec.Body.String())
	}
	resp := decodeMap(t, queryRec.Body.Bytes())
	data := resp["data"].(map[string]any)
	items := data["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("audit query should return at least one event")
	}
	first := items[0].(map[string]any)
	if first["token_id"].(string) != tokenID {
		t.Fatalf("unexpected token_id in first item: got=%s want=%s", first["token_id"].(string), tokenID)
	}
	if strings.Contains(queryRec.Body.String(), "access_token") {
		t.Fatalf("audit query response must not contain access_token")
	}
}

func TestTokenAPIAuditEventsReady(t *testing.T) {
	t.Parallel()

	runtime := service.NewInMemoryTokenRuntime(nil)
	auditor := service.NewMemoryAuditStore()
	api := NewTokenAPI(runtime, auditor, time.Now)
	mux := http.NewServeMux()
	api.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/tokens/audit-events?limit=3", nil)
	req.Header.Set("X-Request-Id", "req-audit-ready")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected ready audit query endpoint: code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTokenAPIAuditEventsNotReady(t *testing.T) {
	t.Parallel()

	runtime := service.NewInMemoryTokenRuntime(nil)
	api := NewTokenAPI(runtime, noopAuditEmitter{}, time.Now)
	mux := http.NewServeMux()
	api.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/tokens/audit-events?limit=3", nil)
	req.Header.Set("X-Request-Id", "req-audit-query-3")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected not implemented: code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func mustJSON(t *testing.T, payload any) *bytes.Reader {
	t.Helper()
	buf, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal json failed: %v", err)
	}
	return bytes.NewReader(buf)
}

func decodeMap(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode json failed: %v, raw=%s", err, string(raw))
	}
	return out
}

type noopAuditEmitter struct{}

func (noopAuditEmitter) Emit(context.Context, service.AuditEvent) error {
	return nil
}
