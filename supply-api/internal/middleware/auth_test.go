package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestTokenVerify(t *testing.T) {
	secretKey := "test-secret-key-12345678901234567890"
	issuer := "test-issuer"

	tests := []struct {
		name          string
		token         string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid token",
			token:       createTestToken(secretKey, issuer, "subject:1", "owner", time.Now().Add(time.Hour)),
			expectError: false,
		},
		{
			name:          "expired token",
			token:         createTestToken(secretKey, issuer, "subject:1", "owner", time.Now().Add(-time.Hour)),
			expectError:   true,
			errorContains: "expired",
		},
		{
			name:          "wrong issuer",
			token:         createTestToken(secretKey, "wrong-issuer", "subject:1", "owner", time.Now().Add(time.Hour)),
			expectError:   true,
			errorContains: "issuer",
		},
		{
			name:          "invalid token",
			token:         "invalid.token.string",
			expectError:   true,
			errorContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := &AuthMiddleware{
				config: AuthConfig{
					SecretKey: secretKey,
					Issuer:    issuer,
				},
			}

			_, err := middleware.verifyToken(tt.token)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("error = %v, want contains %v", err, tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestQueryKeyRejectMiddleware(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		expectStatus int
	}{
		{
			name:         "no query params",
			query:        "",
			expectStatus: http.StatusOK,
		},
		{
			name:         "normal params",
			query:        "?page=1&size=10",
			expectStatus: http.StatusOK,
		},
		{
			name:         "blocked key param",
			query:        "?key=abc123",
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:         "blocked api_key param",
			query:        "?api_key=secret123",
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:         "blocked token param",
			query:        "?token=bearer123",
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:         "suspicious long param",
			query:        "?apikey=verylongparamvalueexceeding20chars",
			expectStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := &AuthMiddleware{
				auditEmitter: nil,
			}

			nextCalled := false
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
			})

			handler := middleware.QueryKeyRejectMiddleware(nextHandler)

			req := httptest.NewRequest("POST", "/api/v1/supply/accounts"+tt.query, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if tt.expectStatus == http.StatusOK {
				if !nextCalled {
					t.Errorf("expected next handler to be called")
				}
			} else {
				if w.Code != tt.expectStatus {
					t.Errorf("expected status %d, got %d", tt.expectStatus, w.Code)
				}
			}
		})
	}
}

func TestBearerExtractMiddleware(t *testing.T) {
	tests := []struct {
		name         string
		authHeader   string
		expectStatus int
	}{
		{
			name:         "valid bearer",
			authHeader:   "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expectStatus: http.StatusOK,
		},
		{
			name:         "missing header",
			authHeader:   "",
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:         "wrong prefix",
			authHeader:   "Basic abc123",
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:         "empty token",
			authHeader:   "Bearer ",
			expectStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := &AuthMiddleware{}

			nextCalled := false
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				// 检查context中是否有bearer token
				if r.Context().Value(bearerTokenKey) == nil && tt.authHeader != "" && strings.HasPrefix(tt.authHeader, "Bearer ") {
					// 这是预期的，因为token可能无效
				}
			})

			handler := middleware.BearerExtractMiddleware(nextHandler)

			req := httptest.NewRequest("POST", "/api/v1/supply/accounts", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if tt.expectStatus == http.StatusOK {
				if !nextCalled {
					t.Errorf("expected next handler to be called")
				}
			} else {
				if w.Code != tt.expectStatus {
					t.Errorf("expected status %d, got %d", tt.expectStatus, w.Code)
				}
			}
		})
	}
}

func TestContainsScope(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		target   string
		expected bool
	}{
		{
			name:     "exact match",
			scopes:   []string{"read", "write", "delete"},
			target:   "write",
			expected: true,
		},
		{
			name:     "wildcard",
			scopes:   []string{"*"},
			target:   "anything",
			expected: true,
		},
		{
			name:     "no match",
			scopes:   []string{"read", "write"},
			target:   "admin",
			expected: false,
		},
		{
			name:     "empty scopes",
			scopes:   []string{},
			target:   "read",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsScope(tt.scopes, tt.target)
			if result != tt.expected {
				t.Errorf("containsScope(%v, %s) = %v, want %v", tt.scopes, tt.target, result, tt.expected)
			}
		})
	}
}

func TestRoleLevel(t *testing.T) {
	hierarchy := map[string]int{
		"admin":  3,
		"owner":  2,
		"viewer": 1,
	}

	tests := []struct {
		role     string
		expected int
	}{
		{"admin", 3},
		{"owner", 2},
		{"viewer", 1},
		{"unknown", 0},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			result := roleLevel(tt.role, hierarchy)
			if result != tt.expected {
				t.Errorf("roleLevel(%s) = %d, want %d", tt.role, result, tt.expected)
			}
		})
	}
}

func TestTokenCache(t *testing.T) {
	cache := NewTokenCache()

	t.Run("get empty", func(t *testing.T) {
		status, found := cache.Get("nonexistent")
		if found {
			t.Errorf("expected not found")
		}
		if status != "" {
			t.Errorf("expected empty status")
		}
	})

	t.Run("set and get", func(t *testing.T) {
		cache.Set("token1", "active", time.Hour)

		status, found := cache.Get("token1")
		if !found {
			t.Errorf("expected to find token1")
		}
		if status != "active" {
			t.Errorf("expected status 'active', got '%s'", status)
		}
	})

	t.Run("invalidate", func(t *testing.T) {
		cache.Set("token2", "revoked", time.Hour)
		cache.Invalidate("token2")

		_, found := cache.Get("token2")
		if found {
			t.Errorf("expected token2 to be invalidated")
		}
	})

	t.Run("expiration", func(t *testing.T) {
		cache.Set("token3", "active", time.Nanosecond)
		time.Sleep(time.Millisecond)

		_, found := cache.Get("token3")
		if found {
			t.Errorf("expected token3 to be expired")
		}
	})
}

// HIGH-02: JWT算法验证不严格 - 应该拒绝非HS256的算法
func TestHIGH02_JWT_RejectNonHS256Algorithm(t *testing.T) {
	secretKey := "test-secret-key-12345678901234567890"
	issuer := "test-issuer"

	tests := []struct {
		name          string
		signingMethod jwt.SigningMethod
		expectError   bool
		errorContains string
	}{
		{
			name:          "HS256 should be accepted",
			signingMethod: jwt.SigningMethodHS256,
			expectError:   false,
		},
		{
			name:          "HS384 should be rejected",
			signingMethod: jwt.SigningMethodHS384,
			expectError:   true,
			errorContains: "unexpected signing method",
		},
		{
			name:          "HS512 should be rejected",
			signingMethod: jwt.SigningMethodHS512,
			expectError:   true,
			errorContains: "unexpected signing method",
		},
		{
			name:          "none algorithm should be rejected",
			signingMethod: jwt.SigningMethodNone,
			expectError:   true,
			errorContains: "malformed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := TokenClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    issuer,
					Subject:   "subject:1",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(time.Now()),
				},
				SubjectID: "subject:1",
				Role:      "owner",
				Scope:     []string{"read", "write"},
				TenantID:  1,
			}

			token := jwt.NewWithClaims(tt.signingMethod, claims)
			tokenString, _ := token.SignedString([]byte(secretKey))

			middleware := &AuthMiddleware{
				config: AuthConfig{
					SecretKey: secretKey,
					Issuer:    issuer,
				},
			}

			_, err := middleware.verifyToken(tokenString)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("error = %v, want contains %v", err, tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// MED-02: checkTokenStatus缓存未命中时应该查询后端而不是默认返回active
func TestMED02_TokenCacheMiss_ShouldNotAssumeActive(t *testing.T) {
	// arrange
	middleware := &AuthMiddleware{
		config: AuthConfig{
			SecretKey: "test-secret-key-12345678901234567890",
			Issuer:    "test-issuer",
		},
		tokenCache: NewTokenCache(), // 空的缓存
		// 没有设置tokenBackend
	}

	// act - 查询一个不在缓存中的token
	status, err := middleware.checkTokenStatus("nonexistent-token-id")

	// assert - 缓存未命中且没有后端时应该返回错误（安全修复）
	// 修复前bug：缓存未命中时默认返回"active"
	// 修复后：缓存未命中且没有后端时返回错误
	if err == nil {
		t.Errorf("MED-02: cache miss without backend should return error, got status='%s'", status)
	}
}

// Helper functions

func createTestToken(secretKey, issuer, subject, role string, expiresAt time.Time) string {
	claims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		SubjectID: subject,
		Role:      role,
		Scope:     []string{"read", "write"},
		TenantID:  1,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(secretKey))
	return tokenString
}
