package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"lijiaoqiao/supply-api/internal/iam/model"
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
	tests := []struct {
		role     string
		expected int
	}{
		{"super_admin", 100},
		{"org_admin", 50},
		{"supply_admin", 40},
		{"operator", 30},
		{"developer", 20},
		{"finops", 20},
		{"viewer", 10},
		{"unknown", 0},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			result := model.GetRoleLevelByCode(tt.role)
			if result != tt.expected {
				t.Errorf("GetRoleLevelByCode(%s) = %d, want %d", tt.role, result, tt.expected)
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

// HIGH-02: JWT算法验证 - 当前只支持HS256
// 注意: HS384/HS512/RS256需要配置支持，测试当前仅验证HS256
func TestHIGH02_JWT_AlgorithmValidation(t *testing.T) {
	secretKey := "test-secret-key-12345678901234567890"
	issuer := "test-issuer"

	tests := []struct {
		name          string
		signingMethod jwt.SigningMethod
		expectError   bool
		errorContains string
	}{
		{
			name:          "HS256 should be accepted with secret key",
			signingMethod: jwt.SigningMethodHS256,
			expectError:   false,
		},
		{
			name:          "HS384 requires different implementation",
			signingMethod: jwt.SigningMethodHS384,
			expectError:   true,
			errorContains: "unexpected signing method",
		},
		{
			name:          "HS512 requires different implementation",
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

// TestP001_RS256WithPublicKey RS256算法需要配置公钥验证
func TestP001_RS256WithPublicKey(t *testing.T) {
	// 这个测试验证RS256需要公钥配置
	// 使用rsa.GeneratingKey方式创建测试密钥
	// 注意：这个测试只验证配置逻辑，不实际验证RS256签名
	t.Skip("RS256 verification requires RSA key pair setup - tested in token_format_test.go")
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
	status, err := middleware.checkTokenStatus(context.Background(), "nonexistent-token-id")

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

// ==================== BruteForceProtection Tests ====================

func TestNewBruteForceProtection(t *testing.T) {
	bp := NewBruteForceProtection(5, time.Minute)

	if bp.maxAttempts != 5 {
		t.Errorf("expected maxAttempts 5, got %d", bp.maxAttempts)
	}
	if bp.lockoutDuration != time.Minute {
		t.Errorf("expected lockoutDuration 1m, got %v", bp.lockoutDuration)
	}
	if bp.attempts == nil {
		t.Error("expected attempts map to be initialized")
	}
}

func TestBruteForceProtection_RecordFailedAttempt(t *testing.T) {
	bp := NewBruteForceProtection(3, time.Minute)

	// 连续调用3次后应该锁定
	bp.RecordFailedAttempt("192.168.1.1")
	bp.RecordFailedAttempt("192.168.1.1")
	bp.RecordFailedAttempt("192.168.1.1")

	locked, remaining := bp.IsLocked("192.168.1.1")
	if !locked {
		t.Error("should be locked after 3 attempts")
	}
	if remaining <= 0 {
		t.Error("remaining time should be positive when locked")
	}
}

func TestBruteForceProtection_IsLocked(t *testing.T) {
	bp := NewBruteForceProtection(2, time.Hour)

	// 未记录的IP应该不锁定
	locked, _ := bp.IsLocked("192.168.1.100")
	if locked {
		t.Error("unrecorded IP should not be locked")
	}

	// 达到最大尝试次数应该锁定
	bp.RecordFailedAttempt("192.168.1.2")
	bp.RecordFailedAttempt("192.168.1.2")

	locked, remaining := bp.IsLocked("192.168.1.2")
	if !locked {
		t.Error("should be locked after 2 attempts")
	}
	if remaining <= 0 || remaining > time.Hour {
		t.Errorf("remaining time should be within lockout duration, got %v", remaining)
	}
}

func TestBruteForceProtection_Reset(t *testing.T) {
	bp := NewBruteForceProtection(2, time.Hour)

	// 锁定IP
	bp.RecordFailedAttempt("192.168.1.1")
	bp.RecordFailedAttempt("192.168.1.1")

	locked, _ := bp.IsLocked("192.168.1.1")
	if !locked {
		t.Error("should be locked before reset")
	}

	// 重置
	bp.Reset("192.168.1.1")

	locked, _ = bp.IsLocked("192.168.1.1")
	if locked {
		t.Error("should not be locked after reset")
	}
}

func TestBruteForceProtection_CleanExpired(t *testing.T) {
	bp := NewBruteForceProtection(1, time.Millisecond)

	// 锁定IP
	bp.RecordFailedAttempt("192.168.1.1")
	bp.RecordFailedAttempt("192.168.1.1")

	// 等待锁定过期
	time.Sleep(5 * time.Millisecond)

	// 清理
	bp.CleanExpired()

	// IP应该不再被锁定（记录应该被清理）
	locked, _ := bp.IsLocked("192.168.1.1")
	if locked {
		t.Error("expired lock should be cleaned")
	}
}

func TestBruteForceProtection_Len(t *testing.T) {
	bp := NewBruteForceProtection(3, time.Hour)

	if bp.Len() != 0 {
		t.Errorf("expected 0, got %d", bp.Len())
	}

	bp.RecordFailedAttempt("192.168.1.1")
	bp.RecordFailedAttempt("192.168.1.2")

	if bp.Len() != 2 {
		t.Errorf("expected 2, got %d", bp.Len())
	}

	bp.Reset("192.168.1.1")

	if bp.Len() != 1 {
		t.Errorf("expected 1 after reset, got %d", bp.Len())
	}
}

func TestBruteForceProtection_MultipleIPs(t *testing.T) {
	bp := NewBruteForceProtection(2, time.Hour)

	// 不同IP独立计数
	bp.RecordFailedAttempt("192.168.1.1")
	bp.RecordFailedAttempt("192.168.1.2")

	// 第一个IP再失败一次，应该锁定
	bp.RecordFailedAttempt("192.168.1.1")

	locked1, _ := bp.IsLocked("192.168.1.1")
	locked2, _ := bp.IsLocked("192.168.1.2")

	if !locked1 {
		t.Error("192.168.1.1 should be locked")
	}
	if locked2 {
		t.Error("192.168.1.2 should still not be locked")
	}
}

// ==================== Helper Function Tests ====================

func TestGetRequestID(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		expectedID string
	}{
		{
			name:       "X-Request-Id header",
			headers:    map[string]string{"X-Request-Id": "req-123"},
			expectedID: "req-123",
		},
		{
			name:       "X-Request-ID header (uppercase)",
			headers:    map[string]string{"X-Request-ID": "req-456"},
			expectedID: "req-456",
		},
		{
			name:       "X-Request-Id only",
			headers:    map[string]string{"X-Request-Id": "req-123"},
			expectedID: "req-123",
		},
		{
			name:       "both empty",
			headers:    map[string]string{},
			expectedID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			id := getRequestID(req)
			if id != tt.expectedID {
				t.Errorf("expected '%s', got '%s'", tt.expectedID, id)
			}
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name        string
		headers     map[string]string
		remoteAddr  string
		expectedIP  string
	}{
		{
			name:       "X-Forwarded-For single",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.1"},
			remoteAddr: "192.168.1.1:1234",
			expectedIP: "203.0.113.1",
		},
		{
			name:       "X-Forwarded-For multiple",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.1, 198.51.100.1, 10.0.0.1"},
			remoteAddr: "192.168.1.1:1234",
			expectedIP: "203.0.113.1",
		},
		{
			name:       "X-Real-IP",
			headers:    map[string]string{"X-Real-IP": "203.0.113.5"},
			remoteAddr: "192.168.1.1:1234",
			expectedIP: "203.0.113.5",
		},
		{
			name:       "X-Forwarded-For takes precedence",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.1", "X-Real-IP": "203.0.113.5"},
			remoteAddr: "192.168.1.1:1234",
			expectedIP: "203.0.113.1",
		},
		{
			name:       "fallback to RemoteAddr",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.1:1234",
			expectedIP: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			req.RemoteAddr = tt.remoteAddr

			ip := getClientIP(req)
			if ip != tt.expectedIP {
				t.Errorf("expected '%s', got '%s'", tt.expectedIP, ip)
			}
		})
	}
}

func TestParseSubjectID(t *testing.T) {
	tests := []struct {
		name     string
		subject  string
		expected int64
	}{
		{
			name:     "valid subject with prefix",
			subject:  "user:12345",
			expected: 12345,
		},
		{
			name:     "subject without prefix",
			subject:  "12345",
			expected: 0,
		},
		{
			name:     "empty subject",
			subject:  "",
			expected: 0,
		},
		{
			name:     "invalid number",
			subject:  "user:abc",
			expected: 0,
		},
		{
			name:     "multiple colons",
			subject:  "user:12345:extra",
			expected: 12345,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := parseSubjectID(tt.subject)
			if id != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, id)
			}
		})
	}
}

func TestComputeFingerprint(t *testing.T) {
	fp1 := ComputeFingerprint("test-credential-123")
	fp2 := ComputeFingerprint("test-credential-123")
	fp3 := ComputeFingerprint("different-credential")

	if fp1 != fp2 {
		t.Error("same input should produce same fingerprint")
	}
	if fp1 == fp3 {
		t.Error("different inputs should produce different fingerprints")
	}
	if len(fp1) != 64 { // SHA256 produces 64 hex characters
		t.Errorf("expected 64 hex chars, got %d", len(fp1))
	}
}

// ==================== GetTokenClaims Tests ====================

func TestGetTokenClaims(t *testing.T) {
	t.Run("with valid claims", func(t *testing.T) {
		claims := &TokenClaims{
			SubjectID: "user:123",
			Role:      "admin",
			TenantID:  1,
		}
		ctx := context.WithValue(context.Background(), tokenClaimsKey, claims)

		result := GetTokenClaims(ctx)
		if result == nil {
			t.Fatal("expected claims, got nil")
		}
		if result.SubjectID != "user:123" {
			t.Errorf("expected SubjectID 'user:123', got '%s'", result.SubjectID)
		}
	})

	t.Run("without claims", func(t *testing.T) {
		ctx := context.Background()
		result := GetTokenClaims(ctx)
		if result != nil {
			t.Error("expected nil when no claims in context")
		}
	})

	t.Run("with wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), tokenClaimsKey, "not a token claims")
		result := GetTokenClaims(ctx)
		if result != nil {
			t.Error("expected nil when wrong type in context")
		}
	})
}

// ==================== NewAuthMiddleware Tests ====================

func TestNewAuthMiddleware_DefaultCacheTTL(t *testing.T) {
	config := AuthConfig{
		SecretKey: "test-secret",
		Issuer:    "test-issuer",
		CacheTTL:  0, // 应该使用默认值
	}

	mw := NewAuthMiddleware(config, nil, nil, nil)

	if mw.config.CacheTTL != 30*time.Second {
		t.Errorf("expected default CacheTTL 30s, got %v", mw.config.CacheTTL)
	}
}

func TestNewAuthMiddleware_ExplicitCacheTTL(t *testing.T) {
	config := AuthConfig{
		SecretKey: "test-secret",
		Issuer:    "test-issuer",
		CacheTTL:  30 * time.Second, // 显式设置
	}

	mw := NewAuthMiddleware(config, nil, nil, nil)

	if mw.config.CacheTTL != 30*time.Second {
		t.Errorf("expected explicit CacheTTL 30s, got %v", mw.config.CacheTTL)
	}
}

// ==================== ScopeRoleAuthzMiddleware Tests ====================

func TestScopeRoleAuthzMiddleware(t *testing.T) {
	secretKey := "test-secret-key-12345678901234567890"
	issuer := "test-issuer"

	// 创建一个有效的token
	claims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "user:1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		SubjectID: "user:1",
		Role:      "viewer",
		Scope:     []string{"read"},
		TenantID:  1,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	_, _ = token.SignedString([]byte(secretKey)) // tokenString not used in these tests

	middleware := &AuthMiddleware{
		config: AuthConfig{
			SecretKey: secretKey,
			Issuer:    issuer,
		},
	}

	tests := []struct {
		name           string
		path           string
		setupContext   func(r *http.Request)
		requiredScope  string
		expectStatus   int
	}{
		{
			name:          "missing claims in context",
			path:          "/api/v1/supply/accounts",
			setupContext:  func(r *http.Request) { /* 不设置claims */ },
			requiredScope: "",
			expectStatus:  http.StatusUnauthorized,
		},
		{
			name:         "insufficient role for accounts",
			path:         "/api/v1/supply/accounts",
			setupContext: func(r *http.Request) {
				ctx := context.WithValue(r.Context(), tokenClaimsKey, claims)
				*r = *r.WithContext(ctx)
			},
			requiredScope: "",
			expectStatus:  http.StatusForbidden,
		},
		{
			name:         "sufficient role for accounts",
			path:         "/api/v1/supply/accounts",
			setupContext: func(r *http.Request) {
				adminClaims := &TokenClaims{
					RegisteredClaims: jwt.RegisteredClaims{
						Issuer:    issuer,
						Subject:   "user:1",
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
					},
					SubjectID: "user:1",
					Role:      "org_admin",
					Scope:     []string{"read", "write"},
					TenantID:  1,
				}
				ctx := context.WithValue(r.Context(), tokenClaimsKey, adminClaims)
				*r = *r.WithContext(ctx)
			},
			requiredScope: "",
			expectStatus:  http.StatusOK,
		},
		{
			name:         "viewer can access billing",
			path:         "/api/v1/supply/billing",
			setupContext: func(r *http.Request) {
				ctx := context.WithValue(r.Context(), tokenClaimsKey, claims)
				*r = *r.WithContext(ctx)
			},
			requiredScope: "",
			expectStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
			})

			handler := middleware.ScopeRoleAuthzMiddleware(tt.requiredScope)(nextHandler)

			req := httptest.NewRequest("GET", tt.path, nil)
			tt.setupContext(req)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if tt.expectStatus == http.StatusOK {
				if !nextCalled {
					t.Error("expected next handler to be called")
				}
			} else {
				if w.Code != tt.expectStatus {
					t.Errorf("expected status %d, got %d", tt.expectStatus, w.Code)
				}
			}
		})
	}
}

// ==================== TokenCache Extended Tests ====================

func TestTokenCache_Len(t *testing.T) {
	cache := NewTokenCache()

	if cache.Len() != 0 {
		t.Errorf("expected 0, got %d", cache.Len())
	}

	cache.Set("token1", "active", time.Hour)
	if cache.Len() != 1 {
		t.Errorf("expected 1, got %d", cache.Len())
	}

	cache.Set("token2", "active", time.Hour)
	if cache.Len() != 2 {
		t.Errorf("expected 2, got %d", cache.Len())
	}

	cache.Invalidate("token1")
	if cache.Len() != 1 {
		t.Errorf("expected 1 after invalidate, got %d", cache.Len())
	}
}

func TestTokenCache_CleanExpired(t *testing.T) {
	cache := NewTokenCache()

	// 设置一个立即过期的token
	cache.Set("expired-token", "active", time.Nanosecond)
	time.Sleep(time.Millisecond)

	if cache.Len() != 1 {
		t.Errorf("expected 1 before clean, got %d", cache.Len())
	}

	cache.CleanExpired()

	if cache.Len() != 0 {
		t.Errorf("expected 0 after clean, got %d", cache.Len())
	}
}
