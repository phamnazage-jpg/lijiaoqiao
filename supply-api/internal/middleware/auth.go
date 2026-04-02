package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenClaims JWT token claims
type TokenClaims struct {
	jwt.RegisteredClaims
	SubjectID string   `json:"subject_id"`
	Role      string   `json:"role"`
	Scope     []string `json:"scope"`
	TenantID  int64    `json:"tenant_id"`
}

// AuthConfig 鉴权中间件配置
type AuthConfig struct {
	SecretKey string
	Issuer    string
	CacheTTL  time.Duration // token状态缓存TTL
	Enabled   bool          // 是否启用鉴权
}

// AuthMiddleware 鉴权中间件
type AuthMiddleware struct {
	config          AuthConfig
	tokenCache      *TokenCache
	tokenBackend    TokenStatusBackend
	auditEmitter    AuditEmitter
}

// TokenStatusBackend Token状态后端查询接口
type TokenStatusBackend interface {
	CheckTokenStatus(ctx context.Context, tokenID string) (string, error)
}

// AuditEmitter 审计事件发射器
type AuditEmitter interface {
	Emit(ctx context.Context, event AuditEvent) error
}

// AuditEvent 审计事件
type AuditEvent struct {
	EventName  string
	RequestID  string
	TokenID    string
	SubjectID  string
	Route      string
	ResultCode string
	ClientIP   string
	CreatedAt  time.Time
}

// NewAuthMiddleware 创建鉴权中间件
func NewAuthMiddleware(config AuthConfig, tokenCache *TokenCache, tokenBackend TokenStatusBackend, auditEmitter AuditEmitter) *AuthMiddleware {
	if config.CacheTTL == 0 {
		config.CacheTTL = 30 * time.Second
	}
	return &AuthMiddleware{
		config:       config,
		tokenCache:   tokenCache,
		tokenBackend: tokenBackend,
		auditEmitter: auditEmitter,
	}
}

// QueryKeyRejectMiddleware 拒绝外部query key入站
// 对应M-016指标
func (m *AuthMiddleware) QueryKeyRejectMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查query string中的可疑参数
		queryParams := r.URL.Query()

		// 禁止的query参数名
		blockedParams := []string{"key", "api_key", "token", "secret", "password", "credential"}

		for _, param := range blockedParams {
			if _, exists := queryParams[param]; exists {
				// 触发M-016指标事件
				if m.auditEmitter != nil {
					m.auditEmitter.Emit(r.Context(), AuditEvent{
						EventName:  "token.query_key.rejected",
						RequestID:  getRequestID(r),
						Route:      r.URL.Path,
						ResultCode: "QUERY_KEY_NOT_ALLOWED",
						ClientIP:   getClientIP(r),
						CreatedAt:  time.Now(),
					})
				}

				writeAuthError(w, http.StatusUnauthorized, "QUERY_KEY_NOT_ALLOWED",
					"external query key is not allowed, use Authorization header")
				return
			}
		}

		// 检查是否有API Key在query中（即使参数名不同）
		for param := range queryParams {
			lowerParam := strings.ToLower(param)
			if strings.Contains(lowerParam, "key") || strings.Contains(lowerParam, "token") || strings.Contains(lowerParam, "secret") {
				// 可能是编码的API Key
				if len(queryParams.Get(param)) > 20 {
					if m.auditEmitter != nil {
						m.auditEmitter.Emit(r.Context(), AuditEvent{
							EventName:  "token.query_key.rejected",
							RequestID:  getRequestID(r),
							Route:      r.URL.Path,
							ResultCode: "QUERY_KEY_NOT_ALLOWED",
							ClientIP:   getClientIP(r),
							CreatedAt:  time.Now(),
						})
					}

					writeAuthError(w, http.StatusUnauthorized, "QUERY_KEY_NOT_ALLOWED",
						"suspicious query parameter detected")
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

// BearerExtractMiddleware 提取Bearer Token
func (m *AuthMiddleware) BearerExtractMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		if authHeader == "" {
			if m.auditEmitter != nil {
				m.auditEmitter.Emit(r.Context(), AuditEvent{
					EventName:  "token.authn.fail",
					RequestID:  getRequestID(r),
					Route:      r.URL.Path,
					ResultCode: "AUTH_MISSING_BEARER",
					ClientIP:   getClientIP(r),
					CreatedAt:  time.Now(),
				})
			}

			writeAuthError(w, http.StatusUnauthorized, "AUTH_MISSING_BEARER",
				"Authorization header with Bearer token is required")
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeAuthError(w, http.StatusUnauthorized, "AUTH_INVALID_FORMAT",
				"Authorization header must be in format: Bearer <token>")
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == "" {
			writeAuthError(w, http.StatusUnauthorized, "AUTH_MISSING_BEARER",
				"Bearer token is empty")
			return
		}

		// 将token存入context供后续使用
		ctx := context.WithValue(r.Context(), bearerTokenKey, tokenString)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TokenVerifyMiddleware 校验JWT Token
func (m *AuthMiddleware) TokenVerifyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := r.Context().Value(bearerTokenKey).(string)

		claims, err := m.verifyToken(tokenString)
		if err != nil {
			if m.auditEmitter != nil {
				m.auditEmitter.Emit(r.Context(), AuditEvent{
					EventName:  "token.authn.fail",
					RequestID:  getRequestID(r),
					Route:      r.URL.Path,
					ResultCode: "AUTH_INVALID_TOKEN",
					ClientIP:   getClientIP(r),
					CreatedAt:  time.Now(),
				})
			}

			writeAuthError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN",
				"token verification failed: "+err.Error())
			return
		}

		// 检查token状态（是否被吊销）
		status, err := m.checkTokenStatus(claims.ID)
		if err == nil && status != "active" {
			if m.auditEmitter != nil {
				m.auditEmitter.Emit(r.Context(), AuditEvent{
					EventName:  "token.authn.fail",
					RequestID:  getRequestID(r),
					TokenID:    claims.ID,
					SubjectID:  claims.SubjectID,
					Route:      r.URL.Path,
					ResultCode: "AUTH_TOKEN_INACTIVE",
					ClientIP:   getClientIP(r),
					CreatedAt:  time.Now(),
				})
			}

			writeAuthError(w, http.StatusUnauthorized, "AUTH_TOKEN_INACTIVE",
				"token is revoked or expired")
			return
		}

		// 将claims存入context
		ctx := context.WithValue(r.Context(), tokenClaimsKey, claims)
		ctx = WithTenantID(ctx, claims.TenantID)
		ctx = WithOperatorID(ctx, parseSubjectID(claims.SubjectID))

		if m.auditEmitter != nil {
			m.auditEmitter.Emit(r.Context(), AuditEvent{
				EventName:  "token.authn.success",
				RequestID:  getRequestID(r),
				TokenID:    claims.ID,
				SubjectID:  claims.SubjectID,
				Route:      r.URL.Path,
				ResultCode: "OK",
				ClientIP:   getClientIP(r),
				CreatedAt:  time.Now(),
			})
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ScopeRoleAuthzMiddleware 权限校验中间件
func (m *AuthMiddleware) ScopeRoleAuthzMiddleware(requiredScope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(tokenClaimsKey).(*TokenClaims)
			if !ok {
				writeAuthError(w, http.StatusUnauthorized, "AUTH_CONTEXT_MISSING",
					"authentication context is missing")
				return
			}

			// 检查scope
			if requiredScope != "" && !containsScope(claims.Scope, requiredScope) {
				if m.auditEmitter != nil {
					m.auditEmitter.Emit(r.Context(), AuditEvent{
						EventName:  "token.authz.denied",
						RequestID:  getRequestID(r),
						TokenID:    claims.ID,
						SubjectID:  claims.SubjectID,
						Route:      r.URL.Path,
						ResultCode: "AUTH_SCOPE_DENIED",
						ClientIP:   getClientIP(r),
						CreatedAt:  time.Now(),
					})
				}

				writeAuthError(w, http.StatusForbidden, "AUTH_SCOPE_DENIED",
					fmt.Sprintf("required scope '%s' is not granted", requiredScope))
				return
			}

			// 检查role权限
			roleHierarchy := map[string]int{
				"admin":  3,
				"owner":  2,
				"viewer": 1,
			}

			// 路由权限要求
			routeRoles := map[string]string{
				"/api/v1/supply/accounts":    "owner",
				"/api/v1/supply/packages":    "owner",
				"/api/v1/supply/settlements": "owner",
				"/api/v1/supply/billing":     "viewer",
				"/api/v1/supplier/billing":   "viewer",
			}

			for path, requiredRole := range routeRoles {
				if strings.HasPrefix(r.URL.Path, path) {
					if roleLevel(claims.Role, roleHierarchy) < roleLevel(requiredRole, roleHierarchy) {
						writeAuthError(w, http.StatusForbidden, "AUTH_ROLE_DENIED",
							fmt.Sprintf("required role '%s' is not granted, current role: '%s'", requiredRole, claims.Role))
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// verifyToken 校验JWT token
func (m *AuthMiddleware) verifyToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 严格验证算法：只接受HS256
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.config.SecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		// 验证issuer
		if claims.Issuer != m.config.Issuer {
			return nil, errors.New("invalid token issuer")
		}

		// 验证expiration
		if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
			return nil, errors.New("token has expired")
		}

		// 验证not before
		if claims.NotBefore != nil && claims.NotBefore.Time.After(time.Now()) {
			return nil, errors.New("token is not yet valid")
		}

		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// checkTokenStatus 检查token状态（从缓存或数据库）
func (m *AuthMiddleware) checkTokenStatus(tokenID string) (string, error) {
	if m.tokenCache != nil {
		// 先从缓存检查
		if status, found := m.tokenCache.Get(tokenID); found {
			return status, nil
		}
	}

	// 缓存未命中，查询后端验证token状态
	if m.tokenBackend != nil {
		return m.tokenBackend.CheckTokenStatus(context.Background(), tokenID)
	}

	// 没有后端实现时，应该拒绝访问而不是默认active
	return "", errors.New("token status unknown: backend not configured")
}

// GetTokenClaims 从context获取token claims
func GetTokenClaims(ctx context.Context) *TokenClaims {
	if claims, ok := ctx.Value(tokenClaimsKey).(*TokenClaims); ok {
		return claims
	}
	return nil
}

// context keys
const (
	bearerTokenKey contextKey = "bearer_token"
	tokenClaimsKey contextKey = "token_claims"
)

// writeAuthError 写入鉴权错误
func writeAuthError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]interface{}{
		"request_id": "",
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	}
	json.NewEncoder(w).Encode(resp)
}

// getRequestID 获取请求ID
func getRequestID(r *http.Request) string {
	if id := r.Header.Get("X-Request-Id"); id != "" {
		return id
	}
	return r.Header.Get("X-Request-ID")
}

// getClientIP 获取客户端IP
func getClientIP(r *http.Request) string {
	// 优先从X-Forwarded-For获取
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// containsScope 检查scope列表是否包含目标scope
func containsScope(scopes []string, target string) bool {
	for _, scope := range scopes {
		if scope == target || scope == "*" {
			return true
		}
	}
	return false
}

// roleLevel 获取角色等级
func roleLevel(role string, hierarchy map[string]int) int {
	if level, ok := hierarchy[role]; ok {
		return level
	}
	return 0
}

// parseSubjectID 解析subject ID
func parseSubjectID(subject string) int64 {
	parts := strings.Split(subject, ":")
	if len(parts) >= 2 {
		id, _ := strconv.ParseInt(parts[1], 10, 64)
		return id
	}
	return 0
}

// TokenCache Token状态缓存
type TokenCache struct {
	data map[string]cacheEntry
}

type cacheEntry struct {
	status  string
	expires time.Time
}

// NewTokenCache 创建token缓存
func NewTokenCache() *TokenCache {
	return &TokenCache{
		data: make(map[string]cacheEntry),
	}
}

// Get 获取token状态
func (c *TokenCache) Get(tokenID string) (string, bool) {
	if entry, ok := c.data[tokenID]; ok {
		if time.Now().Before(entry.expires) {
			return entry.status, true
		}
		delete(c.data, tokenID)
	}
	return "", false
}

// Set 设置token状态
func (c *TokenCache) Set(tokenID, status string, ttl time.Duration) {
	c.data[tokenID] = cacheEntry{
		status:  status,
		expires: time.Now().Add(ttl),
	}
}

// Invalidate 使token失效
func (c *TokenCache) Invalidate(tokenID string) {
	delete(c.data, tokenID)
}

// ComputeFingerprint 计算凭证指纹（用于审计）
func ComputeFingerprint(credential string) string {
	hash := sha256.Sum256([]byte(credential))
	return hex.EncodeToString(hash[:])
}
