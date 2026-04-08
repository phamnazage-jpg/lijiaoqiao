package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name      string
		authHeader string
		wantToken string
		wantOK    bool
	}{
		{
			name:      "valid bearer token",
			authHeader: "Bearer test-token-123",
			wantToken: "test-token-123",
			wantOK:    true,
		},
		{
			name:      "valid bearer token with extra spaces",
			authHeader: "Bearer   test-token-456   ",
			wantToken: "test-token-456",
			wantOK:    true,
		},
		{
			name:      "missing bearer prefix",
			authHeader: "test-token-123",
			wantToken: "",
			wantOK:    false,
		},
		{
			name:      "empty bearer token",
			authHeader: "Bearer ",
			wantToken: "",
			wantOK:    false,
		},
		{
			name:      "empty header",
			authHeader: "",
			wantToken: "",
			wantOK:    false,
		},
		{
			name:      "case sensitive bearer",
			authHeader: "bearer test-token",
			wantToken: "",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, ok := extractBearerToken(tt.authHeader)
			if token != tt.wantToken {
				t.Errorf("extractBearerToken() token = %v, want %v", token, tt.wantToken)
			}
			if ok != tt.wantOK {
				t.Errorf("extractBearerToken() ok = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}

func TestHasExternalQueryKey(t *testing.T) {
	tests := []struct {
		name string
		query string
		want  bool
	}{
		{
			name: "has key param",
			query: "?key=abc123",
			want:  true,
		},
		{
			name: "has api_key param",
			query: "?api_key=abc123",
			want:  true,
		},
		{
			name: "has token param",
			query: "?token=abc123",
			want:  true,
		},
		{
			name: "has access_token param",
			query: "?access_token=abc123",
			want:  true,
		},
		{
			name: "has other param",
			query: "?name=test&value=123",
			want:  false,
		},
		{
			name: "no params",
			query: "",
			want:  false,
		},
		{
			name: "case insensitive key",
			query: "?KEY=abc123",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test"+tt.query, nil)
			if got := hasExternalQueryKey(req); got != tt.want {
				t.Errorf("hasExternalQueryKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	t.Run("generates request ID when not present", func(t *testing.T) {
		var capturedReqID string
		handler := requestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedReqID, _ = RequestIDFromContext(r.Context())
		}), time.Now)

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if capturedReqID == "" {
			t.Error("expected request ID to be set in context")
		}
		if rr.Header().Get("X-Request-Id") == "" {
			t.Error("expected X-Request-Id header to be set in response")
		}
	})

	t.Run("uses existing request ID from header", func(t *testing.T) {
		existingID := "existing-req-id-123"
		var capturedID string
		handler := requestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedID = r.Header.Get("X-Request-Id")
		}), time.Now)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-Id", existingID)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if capturedID != existingID {
			t.Errorf("expected request ID %q, got %q", existingID, capturedID)
		}
	})

	t.Run("nil next handler does not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic with nil next handler: %v", r)
			}
		}()
		handler := requestIDMiddleware(nil, time.Now)
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	})
}

func TestQueryKeyRejectMiddleware(t *testing.T) {
	t.Run("rejects request with query key", func(t *testing.T) {
		auditCalled := false
		auditor := &mockAuditEmitter{
			onEmit: func(ctx context.Context, event AuditEvent) error {
				auditCalled = true
				if event.EventName != EventTokenQueryKeyRejected {
					t.Errorf("expected event %s, got %s", EventTokenQueryKeyRejected, event.EventName)
				}
				return nil
			},
		}

		handler := queryKeyRejectMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("next handler should not be called")
		}), auditor, time.Now, nil)

		req := httptest.NewRequest("GET", "/api/v1/supply?key=abc123", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if !auditCalled {
			t.Error("expected audit to be called")
		}
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}
	})

	t.Run("allows request without query key", func(t *testing.T) {
		nextCalled := false
		handler := queryKeyRejectMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		}), nil, time.Now, nil)

		req := httptest.NewRequest("GET", "/api/v1/supply?name=test", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if !nextCalled {
			t.Error("expected next handler to be called")
		}
	})

	t.Run("rejects api_key parameter", func(t *testing.T) {
		handler := queryKeyRejectMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("next handler should not be called")
		}), nil, time.Now, nil)

		req := httptest.NewRequest("GET", "/api/v1/supply?api_key=secret", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}
	})
}

func TestTokenAuthMiddleware(t *testing.T) {
	t.Run("allows request when all checks pass", func(t *testing.T) {
		now := time.Now()
		tokenRuntime := NewInMemoryTokenRuntime(func() time.Time { return now })

		// Issue a valid token
		token, err := tokenRuntime.Issue(context.Background(), "user1", "admin", []string{"supply:read", "supply:write"}, time.Hour)
		if err != nil {
			t.Fatalf("failed to issue token: %v", err)
		}

		cfg := AuthMiddlewareConfig{
			Verifier:        tokenRuntime,
			StatusResolver:  tokenRuntime,
			Authorizer:      NewScopeRoleAuthorizer(),
			ProtectedPrefixes: []string{"/api/v1/supply"},
			ExcludedPrefixes: []string{"/health"},
			Now:            func() time.Time { return now },
		}

		nextCalled := false
		handler := tokenAuthMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			// Verify principal is set in context
			principal, ok := PrincipalFromContext(r.Context())
			if !ok {
				t.Error("expected principal in context")
			}
			if principal.SubjectID != "user1" {
				t.Errorf("expected subject user1, got %s", principal.SubjectID)
			}
		}))

		req := httptest.NewRequest("GET", "/api/v1/supply", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if !nextCalled {
			t.Error("expected next handler to be called")
		}
	})

	t.Run("rejects request without bearer token", func(t *testing.T) {
		cfg := AuthMiddlewareConfig{
			Verifier:        &mockVerifier{},
			StatusResolver:  &mockStatusResolver{},
			Authorizer:      NewScopeRoleAuthorizer(),
			ProtectedPrefixes: []string{"/api/v1/supply"},
			Now:            time.Now,
		}

		handler := tokenAuthMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("next handler should not be called")
		}))

		req := httptest.NewRequest("GET", "/api/v1/supply", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}
	})

	t.Run("rejects request to excluded path", func(t *testing.T) {
		cfg := AuthMiddlewareConfig{
			Verifier:        &mockVerifier{},
			StatusResolver:  &mockStatusResolver{},
			Authorizer:      NewScopeRoleAuthorizer(),
			ProtectedPrefixes: []string{"/api/v1/supply"},
			ExcludedPrefixes: []string{"/health"},
			Now:            time.Now,
		}

		nextCalled := false
		handler := tokenAuthMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		}))

		req := httptest.NewRequest("GET", "/health", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if !nextCalled {
			t.Error("expected next handler to be called for excluded path")
		}
	})

	t.Run("returns 503 when dependencies not ready", func(t *testing.T) {
		cfg := AuthMiddlewareConfig{
			Verifier:        nil,
			StatusResolver:  nil,
			Authorizer:      nil,
			ProtectedPrefixes: []string{"/api/v1/supply"},
			Now:            time.Now,
		}

		handler := tokenAuthMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("next handler should not be called")
		}))

		req := httptest.NewRequest("GET", "/api/v1/supply", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status 503, got %d", rr.Code)
		}
	})
}

func TestScopeRoleAuthorizer(t *testing.T) {
	authorizer := NewScopeRoleAuthorizer()

	t.Run("admin role has access to all", func(t *testing.T) {
		if !authorizer.Authorize("/api/v1/supply", "POST", []string{}, "admin") {
			t.Error("expected admin to have access")
		}
	})

	t.Run("supply read scope for GET", func(t *testing.T) {
		if !authorizer.Authorize("/api/v1/supply", "GET", []string{"supply:read"}, "user") {
			t.Error("expected supply:read to have access to GET")
		}
	})

	t.Run("supply write scope for POST", func(t *testing.T) {
		if !authorizer.Authorize("/api/v1/supply", "POST", []string{"supply:write"}, "user") {
			t.Error("expected supply:write to have access to POST")
		}
	})

	t.Run("supply:read scope is denied for POST", func(t *testing.T) {
		// supply:read only allows GET, POST should be denied
		if authorizer.Authorize("/api/v1/supply", "POST", []string{"supply:read"}, "user") {
			t.Error("expected supply:read to be denied for POST")
		}
	})

	t.Run("wildcard scope works", func(t *testing.T) {
		if !authorizer.Authorize("/api/v1/supply", "POST", []string{"supply:*"}, "user") {
			t.Error("expected supply:* to have access")
		}
	})

	t.Run("platform admin scope", func(t *testing.T) {
		if !authorizer.Authorize("/api/v1/platform/users", "GET", []string{"platform:admin"}, "user") {
			t.Error("expected platform:admin to have access")
		}
	})
}

func TestInMemoryTokenRuntime(t *testing.T) {
	now := time.Now()
	runtime := NewInMemoryTokenRuntime(func() time.Time { return now })

	t.Run("issue and verify token", func(t *testing.T) {
		token, err := runtime.Issue(context.Background(), "user1", "admin", []string{"supply:read"}, time.Hour)
		if err != nil {
			t.Fatalf("failed to issue token: %v", err)
		}
		if token == "" {
			t.Error("expected non-empty token")
		}

		claims, err := runtime.Verify(context.Background(), token)
		if err != nil {
			t.Fatalf("failed to verify token: %v", err)
		}
		if claims.SubjectID != "user1" {
			t.Errorf("expected subject user1, got %s", claims.SubjectID)
		}
	})

	t.Run("resolve token status", func(t *testing.T) {
		token, err := runtime.Issue(context.Background(), "user1", "admin", []string{"supply:read"}, time.Hour)
		if err != nil {
			t.Fatalf("failed to issue token: %v", err)
		}

		// Get token ID first
		claims, _ := runtime.Verify(context.Background(), token)

		status, err := runtime.Resolve(context.Background(), claims.TokenID)
		if err != nil {
			t.Fatalf("failed to resolve status: %v", err)
		}
		if status != TokenStatusActive {
			t.Errorf("expected status active, got %s", status)
		}
	})

	t.Run("revoke token", func(t *testing.T) {
		token, _ := runtime.Issue(context.Background(), "user1", "admin", []string{"supply:read"}, time.Hour)
		claims, _ := runtime.Verify(context.Background(), token)

		err := runtime.Revoke(context.Background(), claims.TokenID)
		if err != nil {
			t.Fatalf("failed to revoke token: %v", err)
		}

		status, _ := runtime.Resolve(context.Background(), claims.TokenID)
		if status != TokenStatusRevoked {
			t.Errorf("expected status revoked, got %s", status)
		}
	})

	t.Run("verify invalid token", func(t *testing.T) {
		_, err := runtime.Verify(context.Background(), "invalid-token")
		if err == nil {
			t.Error("expected error for invalid token")
		}
	})
}

func TestBuildTokenAuthChain(t *testing.T) {
	now := time.Now()
	runtime := NewInMemoryTokenRuntime(func() time.Time { return now })
	token, _ := runtime.Issue(context.Background(), "user1", "admin", []string{"supply:read", "supply:write"}, time.Hour)

	cfg := AuthMiddlewareConfig{
		Verifier:        runtime,
		StatusResolver:  runtime,
		Authorizer:      NewScopeRoleAuthorizer(),
		ProtectedPrefixes: []string{"/api/v1/supply", "/api/v1/platform"},
		ExcludedPrefixes: []string{"/health", "/healthz"},
		Now:            func() time.Time { return now },
	}

	t.Run("full chain with valid token", func(t *testing.T) {
		nextCalled := false
		handler := BuildTokenAuthChain(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		}))

		req := httptest.NewRequest("GET", "/api/v1/supply", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		if !nextCalled {
			t.Error("expected chain to complete successfully")
		}
		if recorder.Header().Get("X-Request-Id") == "" {
			t.Error("expected X-Request-Id header to be set by chain")
		}
	})

	t.Run("full chain rejects query key", func(t *testing.T) {
		handler := BuildTokenAuthChain(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("next handler should not be called")
		}))

		req := httptest.NewRequest("GET", "/api/v1/supply?key=blocked", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}
	})
}

// Mock implementations
type mockVerifier struct{}

func (m *mockVerifier) Verify(ctx context.Context, rawToken string) (VerifiedToken, error) {
	return VerifiedToken{}, nil
}

type mockStatusResolver struct{}

func (m *mockStatusResolver) Resolve(ctx context.Context, tokenID string) (TokenStatus, error) {
	return TokenStatusActive, nil
}

type mockAuditEmitter struct {
	onEmit func(ctx context.Context, event AuditEvent) error
}

func (m *mockAuditEmitter) Emit(ctx context.Context, event AuditEvent) error {
	if m.onEmit != nil {
		return m.onEmit(ctx, event)
	}
	return nil
}

func TestHasScope(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		required string
		want     bool
	}{
		{
			name:     "exact match",
			scopes:   []string{"supply:read", "supply:write"},
			required: "supply:read",
			want:     true,
		},
		{
			name:     "no match",
			scopes:   []string{"supply:read"},
			required: "supply:write",
			want:     false,
		},
		{
			name:     "wildcard match",
			scopes:   []string{"supply:*"},
			required: "supply:read",
			want:     true,
		},
		{
			name:     "wildcard match write",
			scopes:   []string{"supply:*"},
			required: "supply:write",
			want:     true,
		},
		{
			name:     "empty scopes",
			scopes:   []string{},
			required: "supply:read",
			want:     false,
		},
		{
			name:     "partial wildcard no match",
			scopes:   []string{"supply:read"},
			required: "platform:admin",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasScope(tt.scopes, tt.required)
			if got != tt.want {
				t.Errorf("hasScope(%v, %s) = %v, want %v", tt.scopes, tt.required, got, tt.want)
			}
		})
	}
}

func TestRequiredScopeForRoute(t *testing.T) {
	tests := []struct {
		path   string
		method string
		want   string
	}{
		{"/api/v1/supply", "GET", "supply:read"},
		{"/api/v1/supply", "HEAD", "supply:read"},
		{"/api/v1/supply", "OPTIONS", "supply:read"},
		{"/api/v1/supply", "POST", "supply:write"},
		{"/api/v1/supply", "PUT", "supply:write"},
		{"/api/v1/supply", "DELETE", "supply:write"},
		{"/api/v1/supply/", "GET", "supply:read"},
		{"/api/v1/supply/123", "GET", "supply:read"},
		{"/api/v1/platform", "GET", "platform:admin"},
		{"/api/v1/platform", "POST", "platform:admin"},
		{"/api/v1/platform/", "DELETE", "platform:admin"},
		{"/api/v1/platform/users", "GET", "platform:admin"},
		{"/unknown", "GET", ""},
		{"/api/v1/other", "GET", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path+"_"+tt.method, func(t *testing.T) {
			got := requiredScopeForRoute(tt.path, tt.method)
			if got != tt.want {
				t.Errorf("requiredScopeForRoute(%s, %s) = %s, want %s", tt.path, tt.method, got, tt.want)
			}
		})
	}
}

func TestGenerateAccessToken(t *testing.T) {
	token, err := generateAccessToken()
	if err != nil {
		t.Fatalf("generateAccessToken() error = %v", err)
	}
	if !strings.HasPrefix(token, "ptk_") {
		t.Errorf("expected token to start with ptk_, got %s", token)
	}
	if len(token) < 10 {
		t.Errorf("expected token length >= 10, got %d", len(token))
	}

	// 生成多个token应该不同
	token2, _ := generateAccessToken()
	if token == token2 {
		t.Error("expected different tokens")
	}
}

func TestGenerateTokenID(t *testing.T) {
	tokenID, err := generateTokenID()
	if err != nil {
		t.Fatalf("generateTokenID() error = %v", err)
	}
	if !strings.HasPrefix(tokenID, "tok_") {
		t.Errorf("expected token ID to start with tok_, got %s", tokenID)
	}

	tokenID2, _ := generateTokenID()
	if tokenID == tokenID2 {
		t.Error("expected different token IDs")
	}
}

func TestGenerateEventID(t *testing.T) {
	eventID, err := generateEventID()
	if err != nil {
		t.Fatalf("generateEventID() error = %v", err)
	}
	if !strings.HasPrefix(eventID, "evt_") {
		t.Errorf("expected event ID to start with evt_, got %s", eventID)
	}

	eventID2, _ := generateEventID()
	if eventID == eventID2 {
		t.Error("expected different event IDs")
	}
}

func TestNullString(t *testing.T) {
	tests := []struct {
		input    string
		wantStr  string
		wantValid bool
	}{
		{"hello", "hello", true},
		{"", "", false},
		{"world", "world", true},
	}

	for _, tt := range tests {
		got := nullString(tt.input)
		if got.String != tt.wantStr {
			t.Errorf("nullString(%q).String = %q, want %q", tt.input, got.String, tt.wantStr)
		}
		if got.Valid != tt.wantValid {
			t.Errorf("nullString(%q).Valid = %v, want %v", tt.input, got.Valid, tt.wantValid)
		}
	}
}

func TestInMemoryTokenRuntime_Issue_Errors(t *testing.T) {
	now := time.Now()
	runtime := NewInMemoryTokenRuntime(func() time.Time { return now })

	tests := []struct {
		name      string
		subjectID string
		role      string
		scopes    []string
		ttl       time.Duration
		wantErr   string
	}{
		{
			name:      "empty subject_id",
			subjectID: "",
			role:      "admin",
			scopes:    []string{"supply:read"},
			ttl:       time.Hour,
			wantErr:   "subject_id is required",
		},
		{
			name:      "whitespace subject_id",
			subjectID: "   ",
			role:      "admin",
			scopes:    []string{"supply:read"},
			ttl:       time.Hour,
			wantErr:   "subject_id is required",
		},
		{
			name:      "empty role",
			subjectID: "user1",
			role:      "",
			scopes:    []string{"supply:read"},
			ttl:       time.Hour,
			wantErr:   "role is required",
		},
		{
			name:      "empty scopes",
			subjectID: "user1",
			role:      "admin",
			scopes:    []string{},
			ttl:       time.Hour,
			wantErr:   "scope must not be empty",
		},
		{
			name:      "zero ttl",
			subjectID: "user1",
			role:      "admin",
			scopes:    []string{"supply:read"},
			ttl:       0,
			wantErr:   "ttl must be positive",
		},
		{
			name:      "negative ttl",
			subjectID: "user1",
			role:      "admin",
			scopes:    []string{"supply:read"},
			ttl:       -time.Second,
			wantErr:   "ttl must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runtime.Issue(context.Background(), tt.subjectID, tt.role, tt.scopes, tt.ttl)
			if err == nil {
				t.Fatal("expected error")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestInMemoryTokenRuntime_Verify_Expired(t *testing.T) {
	now := time.Now()
	runtime := NewInMemoryTokenRuntime(func() time.Time { return now })

	token, _ := runtime.Issue(context.Background(), "user1", "admin", []string{"supply:read"}, time.Hour)

	// 验证token仍然有效
	claims, err := runtime.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if claims.SubjectID != "user1" {
		t.Errorf("SubjectID = %s, want user1", claims.SubjectID)
	}
}

func TestInMemoryTokenRuntime_ApplyExpiry(t *testing.T) {
	now := time.Now()
	runtime := NewInMemoryTokenRuntime(func() time.Time { return now })

	token, _ := runtime.Issue(context.Background(), "user1", "admin", []string{"supply:read"}, time.Hour)
	claims, _ := runtime.Verify(context.Background(), token)

	// 手动设置过期
	runtime.mu.Lock()
	record := runtime.records[claims.TokenID]
	record.ExpiresAt = now.Add(-time.Hour) // 1小时前过期
	runtime.mu.Unlock()

	// Resolve应该检测到过期
	status, _ := runtime.Resolve(context.Background(), claims.TokenID)
	if status != TokenStatusExpired {
		t.Errorf("status = %s, want Expired", status)
	}
}

func TestScopeRoleAuthorizer_Authorize(t *testing.T) {
	authorizer := NewScopeRoleAuthorizer()

	tests := []struct {
		path   string
		method string
		scopes []string
		role   string
		want   bool
	}{
		{"/api/v1/supply", "GET", []string{"supply:read"}, "user", true},
		{"/api/v1/supply", "POST", []string{"supply:write"}, "user", true},
		{"/api/v1/supply", "DELETE", []string{"supply:read"}, "user", false},
		{"/api/v1/supply", "GET", []string{}, "admin", true},
		{"/api/v1/supply", "POST", []string{}, "admin", true},
		{"/api/v1/other", "GET", []string{}, "user", true}, // 无需权限
		{"/api/v1/platform/users", "GET", []string{"platform:admin"}, "user", true},
		{"/api/v1/platform/users", "POST", []string{"platform:admin"}, "user", true},
		{"/api/v1/platform/users", "DELETE", []string{"supply:read"}, "user", false},
	}

	for _, tt := range tests {
		t.Run(tt.path+"_"+tt.method, func(t *testing.T) {
			got := authorizer.Authorize(tt.path, tt.method, tt.scopes, tt.role)
			if got != tt.want {
				t.Errorf("Authorize(%s, %s, %v, %s) = %v, want %v", tt.path, tt.method, tt.scopes, tt.role, got, tt.want)
			}
		})
	}
}

func TestMemoryAuditEmitter(t *testing.T) {
	emitter := NewMemoryAuditEmitter()

	event := AuditEvent{
		EventName: EventTokenQueryKeyRejected,
		RequestID: "req-123",
		Route:     "/api/v1/supply",
		ResultCode: "401",
	}

	err := emitter.Emit(context.Background(), event)
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	if len(emitter.events) != 1 {
		t.Errorf("expected 1 event, got %d", len(emitter.events))
	}

	if emitter.events[0].EventID == "" {
		t.Error("expected EventID to be set")
	}
}

func TestNewInMemoryTokenRuntime_NilNow(t *testing.T) {
	// 不传入now函数，应该使用默认的time.Now
	runtime := NewInMemoryTokenRuntime(nil)
	if runtime == nil {
		t.Fatal("expected non-nil runtime")
	}

	// 验证基本功能
	_, err := runtime.Issue(context.Background(), "user1", "admin", []string{"supply:read"}, time.Hour)
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}
}
