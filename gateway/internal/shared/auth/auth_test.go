package sharedauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ==================== 契约测试 ====================
// P4-A-08: 共享 auth 行为契约测试方案
// 完成标准：至少覆盖 missing bearer、invalid token、inactive token
// ====================

// TestExtractBearerToken_MissingBearer 契约测试：missing bearer
func TestExtractBearerToken_MissingBearer(t *testing.T) {
	testCases := []struct {
		name       string
		authHeader string
		wantToken  string
		wantOK     bool
	}{
		{"empty header", "", "", false},
		{"no bearer prefix", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", "", false},
		{"basic auth", "Basic dXNlcjpwYXNz", "", false},
		{"bearer only whitespace", "Bearer   ", "", false},
		{"bearer lowercase", "bearer token123", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token, ok := ExtractBearerToken(tc.authHeader)
			if ok != tc.wantOK {
				t.Errorf("ExtractBearerToken(%q) ok = %v, want %v", tc.authHeader, ok, tc.wantOK)
			}
			if token != tc.wantToken {
				t.Errorf("ExtractBearerToken(%q) token = %q, want %q", tc.authHeader, token, tc.wantToken)
			}
		})
	}
}

// TestExtractBearerToken_ValidBearer 契约测试：valid bearer
func TestExtractBearerToken_ValidBearer(t *testing.T) {
	testCases := []struct {
		name       string
		authHeader string
		wantToken  string
	}{
		{"standard bearer", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
		{"with trimming", "Bearer   tok_abc123   ", "tok_abc123"},
		{"short token", "Bearer a", "a"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token, ok := ExtractBearerToken(tc.authHeader)
			if !ok {
				t.Fatalf("ExtractBearerToken(%q) ok = false, want true", tc.authHeader)
			}
			if token != tc.wantToken {
				t.Errorf("token = %q, want %q", token, tc.wantToken)
			}
		})
	}
}

// TestHasExternalQueryKey_MissingBearer 契约测试：query key 检测
func TestHasExternalQueryKey(t *testing.T) {
	testCases := []struct {
		name      string
		queryVals map[string][]string
		want      bool
	}{
		{"no sensitive keys", map[string][]string{"page": {"1"}, "limit": {"10"}}, false},
		{"key parameter", map[string][]string{"key": {"tok_xxxxx"}}, true},
		{"api_key parameter", map[string][]string{"api_key": {"sk_live_xxxxx"}}, true},
		{"token parameter", map[string][]string{"token": {"tok_xxxxx"}}, true},
		{"access_token parameter", map[string][]string{"access_token": {"tok_xxxxx"}}, true},
		{"KEY uppercase", map[string][]string{"KEY": {"secret"}}, true},
		{"API_KEY mixed case", map[string][]string{"Api_Key": {"key"}}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := HasExternalQueryKey(tc.queryVals)
			if got != tc.want {
				t.Errorf("HasExternalQueryKey(%v) = %v, want %v", tc.queryVals, got, tc.want)
			}
		})
	}
}

// TestWriteAuthError_MissingBearer 契约测试：missing bearer 错误响应
func TestWriteAuthError_MissingBearer(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteAuthError(rr, http.StatusUnauthorized, "req-123", CodeMissingBearer,
		"Authorization header with Bearer token is required")

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}

	var resp errorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	if resp.RequestID != "req-123" {
		t.Errorf("request_id = %q, want req-123", resp.RequestID)
	}
	if resp.Error.Code != string(CodeMissingBearer) {
		t.Errorf("error.code = %q, want %q", resp.Error.Code, CodeMissingBearer)
	}
}

// TestWriteAuthError_InvalidToken 契约测试：invalid token 错误响应
func TestWriteAuthError_InvalidToken(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteAuthError(rr, http.StatusUnauthorized, "req-456", CodeInvalidToken,
		"token verification failed")

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}

	var resp errorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	if resp.Error.Code != string(CodeInvalidToken) {
		t.Errorf("error.code = %q, want %q", resp.Error.Code, CodeInvalidToken)
	}
}

// TestWriteAuthError_TokenInactive 契约测试：inactive token 错误响应
func TestWriteAuthError_TokenInactive(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteAuthError(rr, http.StatusUnauthorized, "req-789", CodeTokenInactive,
		"token is revoked or expired")

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}

	var resp errorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	if resp.Error.Code != string(CodeTokenInactive) {
		t.Errorf("error.code = %q, want %q", resp.Error.Code, CodeTokenInactive)
	}
}

// TestWriteAuthError_AuthzDenied 契约测试：scope denied 错误响应
func TestWriteAuthError_AuthzDenied(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteAuthError(rr, http.StatusForbidden, "req-abc", CodeAuthzDenied,
		"required scope 'admin' is not granted")

	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}

	var resp errorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	if resp.Error.Code != string(CodeAuthzDenied) {
		t.Errorf("error.code = %q, want %q", resp.Error.Code, CodeAuthzDenied)
	}
}

// TestWriteAuthError_ContentType 契约测试：所有错误响应 Content-Type 为 application/json
func TestWriteAuthError_ContentType(t *testing.T) {
	codes := []AuthErrorCode{
		CodeMissingBearer,
		CodeInvalidToken,
		CodeTokenInactive,
		CodeQueryKeyNotAllowed,
		CodeAuthzDenied,
		CodeAuthNotReady,
	}

	for _, code := range codes {
		t.Run(string(code), func(t *testing.T) {
			rr := httptest.NewRecorder()
			WriteAuthError(rr, http.StatusUnauthorized, "req-test", code, "test message")

			contentType := rr.Header().Get("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				t.Errorf("Content-Type = %q, want 'application/json'", contentType)
			}
		})
	}
}

// TestAuditEvent_JSONSerialization 契约测试：AuditEvent JSON 序列化
func TestAuditEvent_JSONSerialization(t *testing.T) {
	event := AuditEvent{
		EventName:  "token.authn.fail",
		RequestID:  "req-123",
		TokenID:    "tok_abc",
		SubjectID:  "user_456",
		Route:      "/api/v1/supply/accounts",
		ResultCode: string(CodeInvalidToken),
		ClientIP:   "10.0.0.1",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded AuditEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.EventName != event.EventName {
		t.Errorf("event_name = %q, want %q", decoded.EventName, event.EventName)
	}
	if decoded.RequestID != event.RequestID {
		t.Errorf("request_id = %q, want %q", decoded.RequestID, event.RequestID)
	}
	if decoded.ResultCode != event.ResultCode {
		t.Errorf("result_code = %q, want %q", decoded.ResultCode, event.ResultCode)
	}
}
