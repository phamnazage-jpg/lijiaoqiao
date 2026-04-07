package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ==================== P0-01 Token格式规范测试 ====================
// 验证Token格式规范：JWT + RS256 + 15min有效期
// 原问题：设计文档未定义Token格式，代码使用HS256
// 修复：明确JWT + RS256方案

// TestP001_JWTRS256Signing 验证RS256签名算法
func TestP001_JWTRS256Signing(t *testing.T) {
	// 生成RSA密钥对
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA private key: %v", err)
	}

	// 1. 测试RS256签名
	claims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "llm-gateway-platform",
			Subject:   "user:12345",
			Audience:  jwt.ClaimStrings{"llm-gateway-supply-api"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        "tok_abc123def456",
		},
		SubjectID: "user:12345",
		Role:      "owner",
		Scope:     []string{"supply:accounts:read", "supply:accounts:write"},
		TenantID:  10001,
	}

	// 使用RS256签名
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign token with RS256: %v", err)
	}

	// 验证签名
	parsedToken, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("failed to parse RS256 token: %v", err)
	}

	if !parsedToken.Valid {
		t.Error("RS256 token should be valid")
	}

	parsedClaims, ok := parsedToken.Claims.(*TokenClaims)
	if !ok {
		t.Fatal("failed to get token claims")
	}

	// 验证Claims
	if parsedClaims.Issuer != "llm-gateway-platform" {
		t.Errorf("issuer mismatch: got %s", parsedClaims.Issuer)
	}
	if parsedClaims.SubjectID != "user:12345" {
		t.Errorf("subject_id mismatch: got %s", parsedClaims.SubjectID)
	}
	if parsedClaims.Role != "owner" {
		t.Errorf("role mismatch: got %s", parsedClaims.Role)
	}
}

// TestP001_TokenExpiration 验证15分钟有效期
func TestP001_TokenExpiration(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA private key: %v", err)
	}

	// 生成15分钟有效期的token
	claims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "llm-gateway-platform",
			Subject:   "user:12345",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		SubjectID: "user:12345",
		Role:      "owner",
		TenantID:  10001,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	// 验证token有效
	parsedToken, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("valid token should parse: %v", err)
	}

	// 验证未过期
	parsedClaims := parsedToken.Claims.(*TokenClaims)
	if parsedClaims.ExpiresAt.Time.Before(time.Now()) {
		t.Error("token should not be expired")
	}
}

// TestP001_ExpiredTokenRejected 验证过期token被拒绝
func TestP001_ExpiredTokenRejected(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA private key: %v", err)
	}

	// 生成已过期的token（1小时前过期）
	claims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "llm-gateway-platform",
			Subject:   "user:12345",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), // 已过期
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
		SubjectID: "user:12345",
		Role:      "owner",
		TenantID:  10001,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	// 验证过期token被拒绝
	_, err = jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return &privateKey.PublicKey, nil
	})
	if err == nil {
		t.Error("expired token should be rejected")
	}
}

// TestP001_HS256RejectedInRS256Mode 验证RS256模式下拒绝HS256
func TestP001_HS256RejectedInRS256Mode(t *testing.T) {
	// 创建一个用HS256签名的token
	hs256Key := []byte("test-secret-key-12345678901234567890")
	claims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "llm-gateway-platform",
			Subject:   "user:12345",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
		SubjectID: "user:12345",
		Role:      "owner",
		TenantID:  10001,
	}

	hs256Token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	hs256TokenString, err := hs256Token.SignedString(hs256Key)
	if err != nil {
		t.Fatalf("failed to sign HS256 token: %v", err)
	}

	// 生成RSA密钥（用于RS256模式验证）
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA private key: %v", err)
	}

	// 尝试用RS256公钥验证HS256 token应该失败
	_, err = jwt.ParseWithClaims(hs256TokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, jwt.ErrSignatureInvalid
		}
		return &privateKey.PublicKey, nil
	})

	if err == nil {
		t.Error("HS256 token should be rejected in RS256 mode")
	}
}

// TestP001_RefreshTokenFlow 验证Refresh Token流程
func TestP001_RefreshTokenFlow(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA private key: %v", err)
	}

	// 1. 签发Access Token（15分钟有效期）
	accessClaims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "llm-gateway-platform",
			Subject:   "user:12345",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        "tok_access_123",
		},
		SubjectID: "user:12345",
		Role:      "owner",
		Scope:     []string{"supply:accounts:read"},
		TenantID:  10001,
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodRS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign access token: %v", err)
	}

	// 2. 签发Refresh Token（7天有效期）
	refreshClaims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "llm-gateway-platform",
			Subject:   "user:12345",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)), // 7天
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        "tok_refresh_456",
		},
		SubjectID: "user:12345",
		Role:      "owner",
		Scope:     []string{"supply:accounts:read"}, // Refresh token scope
		TenantID:  10001,
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodRS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign refresh token: %v", err)
	}

	// 3. 验证Access Token
	parsedAccess, err := jwt.ParseWithClaims(accessTokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("access token should be valid: %v", err)
	}

	accessClaimsParsed := parsedAccess.Claims.(*TokenClaims)
	if accessClaimsParsed.ExpiresAt.Time.Sub(time.Now()) > 15*time.Minute {
		t.Error("access token should have max 15min lifetime")
	}

	// 4. 验证Refresh Token
	parsedRefresh, err := jwt.ParseWithClaims(refreshTokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("refresh token should be valid: %v", err)
	}

	refreshClaimsParsed := parsedRefresh.Claims.(*TokenClaims)
	refreshLifetime := refreshClaimsParsed.ExpiresAt.Time.Sub(time.Now())
	expectedMinLifetime := 7*24*time.Hour - time.Minute // 留1分钟容差
	if refreshLifetime < expectedMinLifetime {
		t.Errorf("refresh token should have at least 7 day lifetime, got %v", refreshLifetime)
	}
}

// TestP001_TokenClaimsComplete 验证完整Token Claims
func TestP001_TokenClaimsComplete(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA private key: %v", err)
	}

	// 完整的Claims
	claims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "llm-gateway-platform",
			Subject:   "user:12345",
			Audience:  jwt.ClaimStrings{"llm-gateway-supply-api"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        "tok_abc123def456",
		},
		SubjectID: "user:12345",
		Role:      "owner",
		Scope:     []string{"supply:accounts:read", "supply:accounts:write"},
		TenantID:  10001,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	// 解析并验证所有字段
	parsedToken, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("token should parse: %v", err)
	}

	parsedClaims := parsedToken.Claims.(*TokenClaims)

	// 验证所有字段
	if parsedClaims.Issuer != "llm-gateway-platform" {
		t.Errorf("issuer mismatch")
	}
	if parsedClaims.Subject != "user:12345" {
		t.Errorf("subject mismatch")
	}
	if len(parsedClaims.Audience) != 1 || parsedClaims.Audience[0] != "llm-gateway-supply-api" {
		t.Errorf("audience mismatch")
	}
	if parsedClaims.ID != "tok_abc123def456" {
		t.Errorf("jti/id mismatch")
	}
	if parsedClaims.SubjectID != "user:12345" {
		t.Errorf("subject_id mismatch")
	}
	if parsedClaims.Role != "owner" {
		t.Errorf("role mismatch")
	}
	if len(parsedClaims.Scope) != 2 {
		t.Errorf("scope mismatch: got %v", parsedClaims.Scope)
	}
	if parsedClaims.TenantID != 10001 {
		t.Errorf("tenant_id mismatch")
	}
}

// ==================== 基准测试 ====================

// BenchmarkP001_RS256Signing 基准测试：RS256签名性能
func BenchmarkP001_RS256Signing(b *testing.B) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	claims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "llm-gateway-platform",
			Subject:   "user:12345",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
		SubjectID: "user:12345",
		Role:      "owner",
		TenantID:  10001,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.SignedString(privateKey)
	}
}

// BenchmarkP001_RS256Verification 基准测试：RS256验证性能
func BenchmarkP001_RS256Verification(b *testing.B) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	claims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "llm-gateway-platform",
			Subject:   "user:12345",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
		SubjectID: "user:12345",
		Role:      "owner",
		TenantID:  10001,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, _ := token.SignedString(privateKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
			return &privateKey.PublicKey, nil
		})
	}
}

// ==================== 辅助函数 ====================

// CreateTestRS256Token 创建用于测试的RS256 Token
func CreateTestRS256Token(t *testing.T, claims *TokenClaims) (string, *rsa.PrivateKey) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA private key: %v", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	return tokenString, privateKey
}
