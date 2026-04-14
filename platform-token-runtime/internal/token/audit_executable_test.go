package token_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lijiaoqiao/platform-token-runtime/internal/auth/middleware"
	"lijiaoqiao/platform-token-runtime/internal/auth/model"
	"lijiaoqiao/platform-token-runtime/internal/auth/service"
)

func TestTOKAud001IssueSuccessEvent(t *testing.T) {
	t.Parallel()

	auditor := service.NewMemoryAuditStore()
	rt := service.NewInMemoryTokenRuntime(nil)

	record, err := rt.IssueAndAudit(context.Background(), service.IssueTokenInput{
		SubjectID: "2001",
		Role:      model.RoleOwner,
		Scope:     []string{"supply:*"},
		TTL:       10 * time.Minute,
		RequestID: "req-aud-001",
	}, auditor)
	if err != nil {
		t.Fatalf("issue with audit failed: %v", err)
	}

	event, ok := auditor.LastEvent()
	if !ok {
		t.Fatalf("expected issue success event")
	}
	if event.EventName != service.EventTokenIssueSuccess {
		t.Fatalf("unexpected event name: got=%s want=%s", event.EventName, service.EventTokenIssueSuccess)
	}
	assertAuditRequiredFields(t, event)
	if event.TokenID != record.TokenID {
		t.Fatalf("unexpected token_id in event: got=%s want=%s", event.TokenID, record.TokenID)
	}
}

func TestTOKAud002IssueFailEvent(t *testing.T) {
	t.Parallel()

	auditor := service.NewMemoryAuditStore()
	rt := service.NewInMemoryTokenRuntime(nil)

	_, err := rt.IssueAndAudit(context.Background(), service.IssueTokenInput{
		SubjectID: "2001",
		Role:      model.RoleOwner,
		Scope:     []string{"supply:*"},
		TTL:       0,
		RequestID: "req-aud-002",
	}, auditor)
	if err == nil {
		t.Fatalf("expected issue failure")
	}

	event, ok := auditor.LastEvent()
	if !ok {
		t.Fatalf("expected issue fail event")
	}
	if event.EventName != service.EventTokenIssueFail {
		t.Fatalf("unexpected event name: got=%s want=%s", event.EventName, service.EventTokenIssueFail)
	}
	assertAuditRequiredFields(t, event)
	if event.ResultCode != "ISSUE_FAILED" {
		t.Fatalf("unexpected result_code: got=%s want=ISSUE_FAILED", event.ResultCode)
	}
}

func TestTOKAud003AuthnFailEvent(t *testing.T) {
	t.Parallel()

	auditor := service.NewMemoryAuditStore()
	rt := service.NewInMemoryTokenRuntime(nil)
	authorizer := service.NewScopeRoleAuthorizer()

	handler := middleware.BuildTokenAuthChain(middleware.AuthMiddlewareConfig{
		Verifier:       rt,
		StatusResolver: rt,
		Authorizer:     authorizer,
		Auditor:        auditor,
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/supply/accounts", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status code: got=%d want=%d", rec.Code, http.StatusUnauthorized)
	}
	event, ok := auditor.LastEvent()
	if !ok {
		t.Fatalf("expected audit event for authn failure")
	}
	if event.EventName != service.EventTokenAuthnFail {
		t.Fatalf("unexpected event name: got=%s want=%s", event.EventName, service.EventTokenAuthnFail)
	}
	if event.RequestID == "" {
		t.Fatalf("request_id must not be empty")
	}
}

func TestTOKAud004AuthzDeniedEvent(t *testing.T) {
	t.Parallel()

	auditor := service.NewMemoryAuditStore()
	rt := service.NewInMemoryTokenRuntime(nil)
	authorizer := service.NewScopeRoleAuthorizer()

	ctx := context.Background()
	viewer, err := rt.Issue(ctx, service.IssueTokenInput{
		SubjectID: "2002",
		Role:      model.RoleViewer,
		Scope:     []string{"supply:read"},
		TTL:       5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("issue viewer token failed: %v", err)
	}

	handler := middleware.BuildTokenAuthChain(middleware.AuthMiddlewareConfig{
		Verifier:       rt,
		StatusResolver: rt,
		Authorizer:     authorizer,
		Auditor:        auditor,
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/supply/packages", nil)
	req.Header.Set("Authorization", "Bearer "+viewer.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("unexpected status code: got=%d want=%d", rec.Code, http.StatusForbidden)
	}
	event, ok := auditor.LastEvent()
	if !ok {
		t.Fatalf("expected audit event for authz denial")
	}
	if event.EventName != service.EventTokenAuthzDenied {
		t.Fatalf("unexpected event name: got=%s want=%s", event.EventName, service.EventTokenAuthzDenied)
	}
	if event.SubjectID != viewer.SubjectID {
		t.Fatalf("unexpected subject_id: got=%s want=%s", event.SubjectID, viewer.SubjectID)
	}
}

func TestTOKAud005RevokeSuccessEvent(t *testing.T) {
	t.Parallel()

	auditor := service.NewMemoryAuditStore()
	rt := service.NewInMemoryTokenRuntime(nil)

	record, err := rt.Issue(context.Background(), service.IssueTokenInput{
		SubjectID: "2001",
		Role:      model.RoleOwner,
		Scope:     []string{"supply:*"},
		TTL:       8 * time.Minute,
	})
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}
	_, err = rt.RevokeAndAudit(context.Background(), record.TokenID, "operator_request", "req-aud-005", record.SubjectID, auditor)
	if err != nil {
		t.Fatalf("revoke with audit failed: %v", err)
	}

	event, ok := auditor.LastEvent()
	if !ok {
		t.Fatalf("expected revoke success event")
	}
	if event.EventName != service.EventTokenRevokeSuccess {
		t.Fatalf("unexpected event name: got=%s want=%s", event.EventName, service.EventTokenRevokeSuccess)
	}
	assertAuditRequiredFields(t, event)
	if event.TokenID != record.TokenID {
		t.Fatalf("unexpected token_id in event: got=%s want=%s", event.TokenID, record.TokenID)
	}
}

func TestTOKAud006QueryKeyRejectedEvent(t *testing.T) {
	t.Parallel()

	auditor := service.NewMemoryAuditStore()
	rt := service.NewInMemoryTokenRuntime(nil)
	authorizer := service.NewScopeRoleAuthorizer()

	handler := middleware.BuildTokenAuthChain(middleware.AuthMiddlewareConfig{
		Verifier:       rt,
		StatusResolver: rt,
		Authorizer:     authorizer,
		Auditor:        auditor,
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/supply/accounts?api_key=raw-secret-value", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status code: got=%d want=%d", rec.Code, http.StatusUnauthorized)
	}
	event, ok := auditor.LastEvent()
	if !ok {
		t.Fatalf("expected query key rejection audit event")
	}
	if event.EventName != service.EventTokenQueryKeyRejected {
		t.Fatalf("unexpected event name: got=%s want=%s", event.EventName, service.EventTokenQueryKeyRejected)
	}

	serialized := strings.Join([]string{
		event.EventID,
		event.EventName,
		event.RequestID,
		event.TokenID,
		event.SubjectID,
		event.Route,
		event.ResultCode,
		event.ClientIP,
	}, "|")
	if strings.Contains(serialized, "raw-secret-value") {
		t.Fatalf("audit event must not contain raw query key value")
	}
}

func TestTOKAud007EventImmutability(t *testing.T) {
	t.Parallel()

	auditor := service.NewMemoryAuditStore()
	rt := service.NewInMemoryTokenRuntime(nil)

	issued, err := rt.IssueAndAudit(context.Background(), service.IssueTokenInput{
		SubjectID: "2001",
		Role:      model.RoleOwner,
		Scope:     []string{"supply:*"},
		TTL:       20 * time.Minute,
		RequestID: "req-aud-007-1",
	}, auditor)
	if err != nil {
		t.Fatalf("issue with audit failed: %v", err)
	}
	_, err = rt.RevokeAndAudit(context.Background(), issued.TokenID, "test", "req-aud-007-2", issued.SubjectID, auditor)
	if err != nil {
		t.Fatalf("revoke with audit failed: %v", err)
	}

	firstRead := auditor.Events()
	secondRead := auditor.Events()
	if len(firstRead) < 2 || len(secondRead) < 2 {
		t.Fatalf("expected at least two audit events")
	}
	for idx := range firstRead {
		if firstRead[idx].EventID != secondRead[idx].EventID ||
			firstRead[idx].EventName != secondRead[idx].EventName ||
			!firstRead[idx].CreatedAt.Equal(secondRead[idx].CreatedAt) {
			t.Fatalf("event should be immutable across reads at index=%d", idx)
		}
	}
	for idx := 1; idx < len(firstRead); idx++ {
		if firstRead[idx].CreatedAt.Before(firstRead[idx-1].CreatedAt) {
			t.Fatalf("event timeline should be ordered by created_at")
		}
	}
}

func assertAuditRequiredFields(t *testing.T, event service.AuditEvent) {
	t.Helper()
	if event.EventID == "" {
		t.Fatalf("event_id must not be empty")
	}
	if event.RequestID == "" {
		t.Fatalf("request_id must not be empty")
	}
	if event.ResultCode == "" {
		t.Fatalf("result_code must not be empty")
	}
	if event.Route == "" {
		t.Fatalf("route must not be empty")
	}
	if event.CreatedAt.IsZero() {
		t.Fatalf("created_at must not be zero")
	}
}
