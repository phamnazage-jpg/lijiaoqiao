package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lijiaoqiao/platform-token-runtime/internal/auth/model"
	"lijiaoqiao/platform-token-runtime/internal/auth/service"
)

var fixedNow = func() time.Time {
	return time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
}

type fakeVerifier struct {
	token service.VerifiedToken
	err   error
}

func (f *fakeVerifier) Verify(context.Context, string) (service.VerifiedToken, error) {
	return f.token, f.err
}

type fakeStatusResolver struct {
	status service.TokenStatus
	err    error
}

func (f *fakeStatusResolver) Resolve(context.Context, string) (service.TokenStatus, error) {
	return f.status, f.err
}

type fakeAuthorizer struct {
	allowed bool
}

func (f *fakeAuthorizer) Authorize(string, string, []string, string) bool {
	return f.allowed
}

type fakeAuditor struct {
	events []service.AuditEvent
}

func (f *fakeAuditor) Emit(_ context.Context, event service.AuditEvent) error {
	f.events = append(f.events, event)
	return nil
}

func TestQueryKeyRejectMiddleware(t *testing.T) {
	auditor := &fakeAuditor{}
	nextCalled := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	})
	handler := QueryKeyRejectMiddleware(next, auditor, fixedNow)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/supply/accounts?api_key=secret", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if nextCalled {
		t.Fatalf("next handler should not be called when query key exists")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status code: got=%d want=%d", rec.Code, http.StatusUnauthorized)
	}
	if got := decodeErrorCode(t, rec); got != service.CodeQueryKeyNotAllowed {
		t.Fatalf("unexpected error code: got=%s want=%s", got, service.CodeQueryKeyNotAllowed)
	}
	if len(auditor.events) != 1 {
		t.Fatalf("unexpected audit event count: got=%d want=1", len(auditor.events))
	}
	if auditor.events[0].EventName != service.EventTokenQueryKeyRejected {
		t.Fatalf("unexpected event name: got=%s want=%s", auditor.events[0].EventName, service.EventTokenQueryKeyRejected)
	}
}

func TestTokenAuthMiddleware(t *testing.T) {
	baseToken := service.VerifiedToken{
		TokenID:   "tok-001",
		SubjectID: "subject-001",
		Role:      model.RoleOwner,
		Scope:     []string{"supply:*"},
		IssuedAt:  fixedNow(),
		ExpiresAt: fixedNow().Add(time.Hour),
	}

	cases := []struct {
		name          string
		path          string
		authHeader    string
		verifierErr   error
		status        service.TokenStatus
		statusErr     error
		allowed       bool
		wantStatus    int
		wantErrorCode string
		wantEvent     string
		wantNext      bool
	}{
		{
			name:          "missing bearer",
			path:          "/api/v1/supply/packages",
			wantStatus:    http.StatusUnauthorized,
			wantErrorCode: service.CodeAuthMissingBearer,
			wantEvent:     service.EventTokenAuthnFail,
		},
		{
			name:          "invalid token",
			path:          "/api/v1/supply/packages",
			authHeader:    "Bearer invalid-token",
			verifierErr:   errors.New("invalid signature"),
			wantStatus:    http.StatusUnauthorized,
			wantErrorCode: service.CodeAuthInvalidToken,
			wantEvent:     service.EventTokenAuthnFail,
		},
		{
			name:          "inactive token",
			path:          "/api/v1/supply/packages",
			authHeader:    "Bearer active-token",
			status:        service.TokenStatusRevoked,
			wantStatus:    http.StatusUnauthorized,
			wantErrorCode: service.CodeAuthTokenInactive,
			wantEvent:     service.EventTokenAuthnFail,
		},
		{
			name:          "scope denied",
			path:          "/api/v1/supply/packages",
			authHeader:    "Bearer active-token",
			status:        service.TokenStatusActive,
			allowed:       false,
			wantStatus:    http.StatusForbidden,
			wantErrorCode: service.CodeAuthScopeDenied,
			wantEvent:     service.EventTokenAuthzDenied,
		},
		{
			name:       "authn success",
			path:       "/api/v1/supply/packages",
			authHeader: "Bearer active-token",
			status:     service.TokenStatusActive,
			allowed:    true,
			wantStatus: http.StatusNoContent,
			wantEvent:  service.EventTokenAuthnSuccess,
			wantNext:   true,
		},
		{
			name:       "excluded path bypasses auth",
			path:       "/healthz",
			wantStatus: http.StatusNoContent,
			wantNext:   true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			auditor := &fakeAuditor{}
			verifier := &fakeVerifier{
				token: baseToken,
				err:   tc.verifierErr,
			}
			resolver := &fakeStatusResolver{
				status: tc.status,
				err:    tc.statusErr,
			}
			authorizer := &fakeAuthorizer{allowed: tc.allowed}
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				if tc.wantNext && strings.HasPrefix(tc.path, "/api/v1/") {
					principal, ok := PrincipalFromContext(r.Context())
					if !ok {
						t.Fatalf("principal should be attached when auth succeeded")
					}
					if principal.TokenID != baseToken.TokenID {
						t.Fatalf("unexpected principal token id: got=%s want=%s", principal.TokenID, baseToken.TokenID)
					}
				}
				w.WriteHeader(http.StatusNoContent)
			})

			handler := TokenAuthMiddleware(AuthMiddlewareConfig{
				Verifier:         verifier,
				StatusResolver:   resolver,
				Authorizer:       authorizer,
				Auditor:          auditor,
				ProtectedPrefixes: []string{"/api/v1/supply/", "/api/v1/platform/"},
				ExcludedPrefixes: []string{"/healthz"},
				Now:              fixedNow,
			})(next)

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("unexpected status code: got=%d want=%d", rec.Code, tc.wantStatus)
			}
			if tc.wantErrorCode != "" {
				if got := decodeErrorCode(t, rec); got != tc.wantErrorCode {
					t.Fatalf("unexpected error code: got=%s want=%s", got, tc.wantErrorCode)
				}
			}
			if nextCalled != tc.wantNext {
				t.Fatalf("unexpected next call state: got=%v want=%v", nextCalled, tc.wantNext)
			}
			if tc.wantEvent == "" {
				return
			}
			if len(auditor.events) == 0 {
				t.Fatalf("audit event should be emitted")
			}
			lastEvent := auditor.events[len(auditor.events)-1]
			if lastEvent.EventName != tc.wantEvent {
				t.Fatalf("unexpected event name: got=%s want=%s", lastEvent.EventName, tc.wantEvent)
			}
		})
	}
}

type errorEnvelope struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func decodeErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var envelope errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return envelope.Error.Code
}
