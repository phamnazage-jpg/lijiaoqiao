package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestMED09_ErrorMessageShouldNotLeakInternalDetails verifies that internal error details
// are not exposed to clients
func TestMED09_ErrorMessageShouldNotLeakInternalDetails(t *testing.T) {
	secretKey := "test-secret-key-12345678901234567890"
	issuer := "test-issuer"

	// Create middleware with a token that will cause an error
	middleware := &AuthMiddleware{
		config: AuthConfig{
			SecretKey: secretKey,
			Issuer:    issuer,
		},
		tokenCache: NewTokenCache(),
		// Intentionally no tokenBackend - to simulate error scenario
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Next handler should not be called for auth failures
	})

	handler := middleware.TokenVerifyMiddleware(nextHandler)

	// Create a token that will fail verification
	// Using wrong signing key to simulate internal error
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

	// Sign with wrong key to cause error
	wrongKeyToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	wrongKeyTokenString, _ := wrongKeyToken.SignedString([]byte("wrong-secret-key-that-will-cause-error"))

	// Create request with Bearer token
	req := httptest.NewRequest("POST", "/api/v1/test", nil)
	ctx := context.WithValue(req.Context(), bearerTokenKey, wrongKeyTokenString)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should return 401
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}

	// Parse response
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Check error map
	errorMap, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("response should contain error object")
	}

	message, ok := errorMap["message"].(string)
	if !ok {
		t.Fatal("error should contain message")
	}

	// The error message should NOT contain internal details like:
	// - "crypto" or "cipher" related terms (implementation details)
	// - "secret", "key", "password" (credential info)
	// - "SQL", "database", "connection" (database details)
	// - File paths or line numbers

	internalKeywords := []string{
		"crypto/",
		"/go/src/",
		".go:",
		"sql",
		"database",
		"connection",
		"pq",
		"pgx",
	}

	for _, keyword := range internalKeywords {
		if strings.Contains(strings.ToLower(message), keyword) {
			t.Errorf("MED-09: error message should NOT contain internal details like '%s'. Got: %s", keyword, message)
		}
	}

	// The message should be a generic user-safe message
	if message == "" {
		t.Error("error message should not be empty")
	}
}

// TestMED09_TokenVerifyErrorShouldBeSanitized tests that token verification errors
// don't leak sensitive information
func TestMED09_TokenVerifyErrorShouldBeSanitized(t *testing.T) {
	secretKey := "test-secret-key-12345678901234567890"
	issuer := "test-issuer"

	// Create middleware
	m := &AuthMiddleware{
		config: AuthConfig{
			SecretKey: secretKey,
			Issuer:    issuer,
		},
	}

	// Test with various invalid tokens
	invalidTokens := []struct {
		name        string
		token       string
		expectError bool
	}{
		{
			name:        "completely invalid token",
			token:       "not.a.valid.token.at.all",
			expectError: true,
		},
		{
			name:        "expired token",
			token:       createExpiredTestToken(secretKey, issuer),
			expectError: true,
		},
		{
			name:        "wrong issuer token",
			token:       createWrongIssuerTestToken(secretKey, issuer),
			expectError: true,
		},
	}

	for _, tt := range invalidTokens {
		t.Run(tt.name, func(t *testing.T) {
			_, err := m.verifyToken(tt.token)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}

			if err != nil {
				errMsg := err.Error()

				// Internal error messages should be sanitized
				// They should NOT contain sensitive keywords
				sensitiveKeywords := []string{
					"secret",
					"password",
					"credential",
					"/",
					".go:",
				}

				for _, keyword := range sensitiveKeywords {
					if strings.Contains(strings.ToLower(errMsg), keyword) {
						t.Errorf("MED-09: internal error should NOT contain '%s'. Got: %s", keyword, errMsg)
					}
				}
			}
		})
	}
}

// Helper function to create expired token
func createExpiredTestToken(secretKey, issuer string) string {
	claims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "subject:1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // Expired
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
		SubjectID: "subject:1",
		Role:      "owner",
		Scope:     []string{"read", "write"},
		TenantID:  1,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(secretKey))
	return tokenString
}

// Helper function to create wrong issuer token
func createWrongIssuerTestToken(secretKey, issuer string) string {
	claims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "wrong-issuer",
			Subject:   "subject:1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		SubjectID: "subject:1",
		Role:      "owner",
		Scope:     []string{"read", "write"},
		TenantID:  1,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(secretKey))
	return tokenString
}