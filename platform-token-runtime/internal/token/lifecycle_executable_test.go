package token_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lijiaoqiao/platform-token-runtime/internal/auth/middleware"
	"lijiaoqiao/platform-token-runtime/internal/auth/model"
	"lijiaoqiao/platform-token-runtime/internal/auth/service"
)

func TestTOKLife001IssueSuccess(t *testing.T) {
	t.Parallel()

	rt := service.NewInMemoryTokenRuntime(nil)
	ctx := context.Background()

	first, err := rt.Issue(ctx, service.IssueTokenInput{
		SubjectID: "2001",
		Role:      model.RoleOwner,
		Scope:     []string{"supply:*"},
		TTL:       30 * time.Minute,
	})
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}
	second, err := rt.Issue(ctx, service.IssueTokenInput{
		SubjectID: "2001",
		Role:      model.RoleOwner,
		Scope:     []string{"supply:*"},
		TTL:       30 * time.Minute,
	})
	if err != nil {
		t.Fatalf("issue second token failed: %v", err)
	}

	if first.Status != service.TokenStatusActive {
		t.Fatalf("unexpected status: got=%s want=%s", first.Status, service.TokenStatusActive)
	}
	if !first.ExpiresAt.After(first.IssuedAt) {
		t.Fatalf("expires_at must be greater than issued_at")
	}
	if first.TokenID == second.TokenID {
		t.Fatalf("token_id should be unique")
	}
}

func TestTOKLife002IssueInvalidInput(t *testing.T) {
	t.Parallel()

	rt := service.NewInMemoryTokenRuntime(nil)
	ctx := context.Background()
	_, err := rt.Issue(ctx, service.IssueTokenInput{
		SubjectID: "2001",
		Role:      model.RoleOwner,
		Scope:     []string{"supply:*"},
		TTL:       0,
	})
	if err == nil {
		t.Fatalf("expected error for invalid ttl_seconds")
	}
	if got := rt.TokenCount(); got != 0 {
		t.Fatalf("unexpected token count after invalid issue: got=%d want=0", got)
	}
}

func TestTOKLife003IssueIdempotencyReplay(t *testing.T) {
	t.Parallel()

	rt := service.NewInMemoryTokenRuntime(nil)
	ctx := context.Background()

	first, err := rt.Issue(ctx, service.IssueTokenInput{
		SubjectID:      "2001",
		Role:           model.RoleOwner,
		Scope:          []string{"supply:*"},
		TTL:            30 * time.Minute,
		IdempotencyKey: "idem-life-003",
	})
	if err != nil {
		t.Fatalf("first issue failed: %v", err)
	}
	second, err := rt.Issue(ctx, service.IssueTokenInput{
		SubjectID:      "2001",
		Role:           model.RoleOwner,
		Scope:          []string{"supply:*"},
		TTL:            30 * time.Minute,
		IdempotencyKey: "idem-life-003",
	})
	if err != nil {
		t.Fatalf("replay issue failed: %v", err)
	}

	if first.TokenID != second.TokenID {
		t.Fatalf("replayed issue must return same token_id: first=%s second=%s", first.TokenID, second.TokenID)
	}
	if got := rt.TokenCount(); got != 1 {
		t.Fatalf("idempotent replay must not create duplicate token: got=%d want=1", got)
	}

	_, err = rt.Issue(ctx, service.IssueTokenInput{
		SubjectID:      "2001",
		Role:           model.RoleOwner,
		Scope:          []string{"supply:read"},
		TTL:            30 * time.Minute,
		IdempotencyKey: "idem-life-003",
	})
	if err == nil {
		t.Fatalf("expected payload mismatch conflict for same idempotency key")
	}
}

func TestTOKLife004RefreshSuccess(t *testing.T) {
	t.Parallel()

	rt := service.NewInMemoryTokenRuntime(nil)
	ctx := context.Background()

	issued, err := rt.Issue(ctx, service.IssueTokenInput{
		SubjectID: "2001",
		Role:      model.RoleOwner,
		Scope:     []string{"supply:*"},
		TTL:       1 * time.Minute,
	})
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}
	previousExpiresAt := issued.ExpiresAt

	refreshed, err := rt.Refresh(ctx, issued.TokenID, 15*time.Minute)
	if err != nil {
		t.Fatalf("refresh token failed: %v", err)
	}

	if refreshed.Status != service.TokenStatusActive {
		t.Fatalf("unexpected status after refresh: got=%s want=%s", refreshed.Status, service.TokenStatusActive)
	}
	if !refreshed.ExpiresAt.After(previousExpiresAt) {
		t.Fatalf("expires_at should be delayed after refresh")
	}
}

func TestTOKLife005RevokeSuccess(t *testing.T) {
	t.Parallel()

	start := time.Now()
	rt := service.NewInMemoryTokenRuntime(nil)
	ctx := context.Background()

	issued, err := rt.Issue(ctx, service.IssueTokenInput{
		SubjectID: "2001",
		Role:      model.RoleOwner,
		Scope:     []string{"supply:*"},
		TTL:       10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}
	if _, err := rt.Revoke(ctx, issued.TokenID, "security_event"); err != nil {
		t.Fatalf("revoke token failed: %v", err)
	}

	introspected, err := rt.Introspect(ctx, issued.AccessToken)
	if err != nil {
		t.Fatalf("introspect failed: %v", err)
	}
	if introspected.Status != service.TokenStatusRevoked {
		t.Fatalf("unexpected status after revoke: got=%s want=%s", introspected.Status, service.TokenStatusRevoked)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("revoke propagation exceeded 5 seconds in in-memory runtime")
	}
}

func TestTOKLife006RevokedTokenAccessDenied(t *testing.T) {
	t.Parallel()

	auditor := service.NewMemoryAuditEmitter()
	rt := service.NewInMemoryTokenRuntime(nil)
	authorizer := service.NewScopeRoleAuthorizer()
	ctx := context.Background()

	issued, err := rt.Issue(ctx, service.IssueTokenInput{
		SubjectID: "2001",
		Role:      model.RoleOwner,
		Scope:     []string{"supply:*"},
		TTL:       5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}
	if _, err := rt.Revoke(ctx, issued.TokenID, "test_revoke"); err != nil {
		t.Fatalf("revoke failed: %v", err)
	}

	handler := middleware.BuildTokenAuthChain(middleware.AuthMiddlewareConfig{
		Verifier:       rt,
		StatusResolver: rt,
		Authorizer:     authorizer,
		Auditor:        auditor,
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/supply/accounts", nil)
	req.Header.Set("Authorization", "Bearer "+issued.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status code: got=%d want=%d", rec.Code, http.StatusUnauthorized)
	}
	if code := decodeMiddlewareErrorCode(t, rec); code != service.CodeAuthTokenInactive {
		t.Fatalf("unexpected error code: got=%s want=%s", code, service.CodeAuthTokenInactive)
	}
}

func TestTOKLife007ExpiredTokenInactive(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 3, 29, 15, 0, 0, 0, time.UTC)
	rt := service.NewInMemoryTokenRuntime(func() time.Time { return current })
	ctx := context.Background()

	issued, err := rt.Issue(ctx, service.IssueTokenInput{
		SubjectID: "2001",
		Role:      model.RoleOwner,
		Scope:     []string{"supply:*"},
		TTL:       2 * time.Second,
	})
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}
	current = current.Add(3 * time.Second)

	introspected, err := rt.Introspect(ctx, issued.AccessToken)
	if err != nil {
		t.Fatalf("introspect failed: %v", err)
	}
	if introspected.Status != service.TokenStatusExpired {
		t.Fatalf("unexpected token status: got=%s want=%s", introspected.Status, service.TokenStatusExpired)
	}

	auditor := service.NewMemoryAuditEmitter()
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
	req.Header.Set("Authorization", "Bearer "+issued.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status code: got=%d want=%d", rec.Code, http.StatusUnauthorized)
	}
	if code := decodeMiddlewareErrorCode(t, rec); code != service.CodeAuthTokenInactive {
		t.Fatalf("unexpected error code: got=%s want=%s", code, service.CodeAuthTokenInactive)
	}
}

func TestTOKLife008ViewerWriteDenied(t *testing.T) {
	t.Parallel()

	auditor := service.NewMemoryAuditEmitter()
	rt := service.NewInMemoryTokenRuntime(nil)
	authorizer := service.NewScopeRoleAuthorizer()

	ctx := context.Background()
	viewer, err := rt.Issue(ctx, service.IssueTokenInput{
		SubjectID: "2002",
		Role:      model.RoleViewer,
		Scope:     []string{"supply:read"},
		TTL:       10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("issue viewer token failed: %v", err)
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	handler := middleware.BuildTokenAuthChain(middleware.AuthMiddlewareConfig{
		Verifier:       rt,
		StatusResolver: rt,
		Authorizer:     authorizer,
		Auditor:        auditor,
	}, next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/supply/packages", nil)
	req.Header.Set("Authorization", "Bearer "+viewer.AccessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("unexpected status code: got=%d want=%d", rec.Code, http.StatusForbidden)
	}
	if code := decodeMiddlewareErrorCode(t, rec); code != service.CodeAuthScopeDenied {
		t.Fatalf("unexpected error code: got=%s want=%s", code, service.CodeAuthScopeDenied)
	}
	if nextCalled {
		t.Fatalf("write handler should be blocked for viewer token")
	}
}

type middlewareErrorEnvelope struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func decodeMiddlewareErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var envelope middlewareErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to decode middleware error response: %v", err)
	}
	return envelope.Error.Code
}
