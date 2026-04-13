package middleware

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"lijiaoqiao/supply-api/internal/iam/model"
	"lijiaoqiao/supply-api/internal/pkg/logging"
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
	SecretKey      string
	PublicKey      string
	Algorithm      string
	Issuer         string
	CacheTTL       time.Duration // token状态缓存TTL
	Enabled        bool          // 是否启用鉴权
	TrustedProxies []string      // 可信代理IP列表CIDR，如 "10.0.0.0/8"
}

// AuthMiddleware 鉴权中间件
type AuthMiddleware struct {
	config         AuthConfig
	tokenCache     *TokenCache
	tokenBackend   TokenStatusBackend
	auditEmitter   AuditEmitter
	bruteForce     *BruteForceProtection // 暴力破解保护
	trustedProxies []string              // 可信代理列表
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
		config:         config,
		tokenCache:     tokenCache,
		tokenBackend:   tokenBackend,
		auditEmitter:   auditEmitter,
		trustedProxies: config.TrustedProxies,
	}
}

// BruteForceProtection 暴力破解保护
// MED-12: 防止暴力破解攻击，限制登录尝试次数
type BruteForceProtection struct {
	maxAttempts     int
	lockoutDuration time.Duration
	attempts        map[string]*attemptRecord
	mu              sync.Mutex
	cleanupCounter  int64 // 清理触发计数器
}

type attemptRecord struct {
	count       int
	lockedUntil time.Time
	lastAttempt time.Time // 最后尝试时间，用于过期清理
}

// NewBruteForceProtection 创建暴力破解保护
// maxAttempts: 最大失败尝试次数
// lockoutDuration: 锁定时长
func NewBruteForceProtection(maxAttempts int, lockoutDuration time.Duration) *BruteForceProtection {
	return &BruteForceProtection{
		maxAttempts:     maxAttempts,
		lockoutDuration: lockoutDuration,
		attempts:        make(map[string]*attemptRecord),
	}
}

// RecordFailedAttempt 记录失败尝试
func (b *BruteForceProtection) RecordFailedAttempt(ip string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	record, exists := b.attempts[ip]
	if !exists {
		record = &attemptRecord{}
		b.attempts[ip] = record
	}

	record.count++
	record.lastAttempt = time.Now()
	if record.count >= b.maxAttempts {
		record.lockedUntil = time.Now().Add(b.lockoutDuration)
	}
	b.triggerCleanup()
}

// IsLocked 检查IP是否被锁定
func (b *BruteForceProtection) IsLocked(ip string) (bool, time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	record, exists := b.attempts[ip]
	if !exists {
		return false, 0
	}

	if record.count >= b.maxAttempts && record.lockedUntil.After(time.Now()) {
		remaining := time.Until(record.lockedUntil)
		return true, remaining
	}

	// 如果锁定已过期，重置计数
	if record.lockedUntil.Before(time.Now()) {
		record.count = 0
		record.lockedUntil = time.Time{}
	}

	return false, 0
}

// Reset 重置IP的尝试记录
func (b *BruteForceProtection) Reset(ip string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.attempts, ip)
}

// triggerCleanup 触发清理（每100次操作清理一次过期记录）
func (b *BruteForceProtection) triggerCleanup() {
	b.cleanupCounter++
	if b.cleanupCounter >= 100 {
		b.cleanupCounter = 0
		b.cleanupExpiredLocked()
	}
}

// cleanupExpiredLocked 清理过期记录（需要持有锁）
// 清理条件：锁定已过期且最后尝试时间超过lockoutDuration
func (b *BruteForceProtection) cleanupExpiredLocked() {
	now := time.Now()
	threshold := now.Add(-b.lockoutDuration * 2) // 超过两倍锁定时长未活动的记录清理
	for ip, record := range b.attempts {
		// 清理：锁定已过期且长时间无活动
		if record.lockedUntil.Before(now) && record.lastAttempt.Before(threshold) {
			delete(b.attempts, ip)
		}
	}
}

// CleanExpired 主动清理过期记录（可由外部定期调用）
func (b *BruteForceProtection) CleanExpired() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cleanupExpiredLocked()
}

// Len 返回当前记录数量（用于监控）
func (b *BruteForceProtection) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.attempts)
}

// QueryKeyRejectMiddleware 拒绝外部query key入站
// 对应M-016指标
func (m *AuthMiddleware) QueryKeyRejectMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldBypassAuth(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

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
						Route:      sanitizeRoute(r.URL.Path),
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
							Route:      sanitizeRoute(r.URL.Path),
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
		if shouldBypassAuth(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")

		if authHeader == "" {
			if m.auditEmitter != nil {
				m.auditEmitter.Emit(r.Context(), AuditEvent{
					EventName:  "token.authn.fail",
					RequestID:  getRequestID(r),
					Route:      sanitizeRoute(r.URL.Path),
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
// MED-12: 添加暴力破解保护
func (m *AuthMiddleware) TokenVerifyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldBypassAuth(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// 如果鉴权被禁用（仅用于开发环境），直接跳过验证
		if !m.config.Enabled {
			// 在开发模式下，虽然跳过JWT验证，但仍记录警告日志
			logger := logging.NewLogger("supply-api", logging.LogLevelWarn)
			logger.Warn("Authentication is disabled (dev mode)", map[string]interface{}{
				"method": r.Method,
				"path":   r.URL.Path,
			})
			next.ServeHTTP(w, r)
			return
		}

		// MED-12: 检查暴力破解保护
		if m.bruteForce != nil {
			clientIP := getClientIP(r)
			if locked, remaining := m.bruteForce.IsLocked(clientIP); locked {
				writeAuthError(w, http.StatusTooManyRequests, "AUTH_ACCOUNT_LOCKED",
					fmt.Sprintf("too many failed attempts, try again in %v", remaining))
				return
			}
		}

		// 安全检查：确保BearerExtractMiddleware已执行
		tokenValue := r.Context().Value(bearerTokenKey)
		if tokenValue == nil {
			writeAuthError(w, http.StatusUnauthorized, "AUTH_TOKEN_MISSING",
				"bearer token is missing")
			return
		}
		tokenString, ok := tokenValue.(string)
		if !ok || tokenString == "" {
			writeAuthError(w, http.StatusUnauthorized, "AUTH_TOKEN_INVALID",
				"bearer token is invalid")
			return
		}

		claims, err := m.verifyToken(tokenString)
		if err != nil {
			// MED-12: 记录失败尝试
			if m.bruteForce != nil {
				m.bruteForce.RecordFailedAttempt(getClientIP(r))
			}

			if m.auditEmitter != nil {
				m.auditEmitter.Emit(r.Context(), AuditEvent{
					EventName:  "token.authn.fail",
					RequestID:  getRequestID(r),
					Route:      sanitizeRoute(r.URL.Path),
					ResultCode: "AUTH_INVALID_TOKEN",
					ClientIP:   getClientIP(r),
					CreatedAt:  time.Now(),
				})
			}

			writeAuthError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN",
				"token verification failed")
			return
		}

		// 检查token状态（是否被吊销）
		status, err := m.checkTokenStatus(r.Context(), claims.ID)
		if err != nil {
			if m.auditEmitter != nil {
				m.auditEmitter.Emit(r.Context(), AuditEvent{
					EventName:  "token.authn.fail",
					RequestID:  getRequestID(r),
					TokenID:    claims.ID,
					SubjectID:  claims.SubjectID,
					Route:      sanitizeRoute(r.URL.Path),
					ResultCode: "AUTH_TOKEN_STATUS_UNAVAILABLE",
					ClientIP:   getClientIP(r),
					CreatedAt:  time.Now(),
				})
			}

			writeAuthError(w, http.StatusUnauthorized, "AUTH_TOKEN_STATUS_UNAVAILABLE",
				"token status backend is unavailable")
			return
		}
		if status != "active" {
			if m.auditEmitter != nil {
				m.auditEmitter.Emit(r.Context(), AuditEvent{
					EventName:  "token.authn.fail",
					RequestID:  getRequestID(r),
					TokenID:    claims.ID,
					SubjectID:  claims.SubjectID,
					Route:      sanitizeRoute(r.URL.Path),
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
				Route:      sanitizeRoute(r.URL.Path),
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
						Route:      sanitizeRoute(r.URL.Path),
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
			// 使用model.GetRoleLevelByCode获取统一角色层级定义

			// 路由权限要求（使用详细角色代码）
			// viewer: level 10, operator: level 30, org_admin: level 50
			routeRoles := map[string]string{
				"/api/v1/supply/accounts":    "org_admin",
				"/api/v1/supply/packages":    "org_admin",
				"/api/v1/supply/settlements": "org_admin",
				"/api/v1/supply/billing":     "viewer",
				"/api/v1/supplier/billing":   "viewer",
			}

			for path, requiredRole := range routeRoles {
				if strings.HasPrefix(r.URL.Path, path) {
					if model.GetRoleLevelByCode(claims.Role) < model.GetRoleLevelByCode(requiredRole) {
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
	expectedAlgorithm := strings.ToUpper(strings.TrimSpace(m.config.Algorithm))
	if expectedAlgorithm == "" {
		expectedAlgorithm = jwt.SigningMethodHS256.Alg()
	}

	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != expectedAlgorithm {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.signingKey(expectedAlgorithm)
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

func (m *AuthMiddleware) signingKey(algorithm string) (interface{}, error) {
	switch algorithm {
	case jwt.SigningMethodHS256.Alg(), jwt.SigningMethodHS384.Alg(), jwt.SigningMethodHS512.Alg():
		if strings.TrimSpace(m.config.SecretKey) == "" {
			return nil, errors.New("missing token secret key")
		}
		return []byte(m.config.SecretKey), nil
	case jwt.SigningMethodRS256.Alg(), jwt.SigningMethodRS384.Alg(), jwt.SigningMethodRS512.Alg():
		return parseRSAPublicKey(m.config.PublicKey)
	default:
		return nil, fmt.Errorf("unsupported signing method: %s", algorithm)
	}
}

func parseRSAPublicKey(publicKeyPEM string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(publicKeyPEM)))
	if block == nil {
		return nil, errors.New("invalid RSA public key: PEM decode failed")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err == nil {
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("invalid RSA public key type")
		}
		return rsaPub, nil
	}

	cert, certErr := x509.ParseCertificate(block.Bytes)
	if certErr == nil {
		rsaPub, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("invalid RSA certificate public key type")
		}
		return rsaPub, nil
	}

	return nil, fmt.Errorf("invalid RSA public key: %w", err)
}

func shouldBypassAuth(path string) bool {
	return path == "/actuator/health" ||
		path == "/actuator/health/live" ||
		path == "/actuator/health/ready"
}

// checkTokenStatus 检查token状态（从缓存或数据库）
func (m *AuthMiddleware) checkTokenStatus(ctx context.Context, tokenID string) (string, error) {
	if m.tokenCache != nil {
		// 先从缓存检查
		if status, found := m.tokenCache.Get(tokenID); found {
			return status, nil
		}
	}

	// 缓存未命中，查询后端验证token状态
	if m.tokenBackend != nil {
		return m.tokenBackend.CheckTokenStatus(ctx, tokenID)
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
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// 记录编码错误（响应已经开始发送，无法回退）
		logger := logging.NewLogger("supply-api", logging.LogLevelError)
		logger.Error("failed to encode error response", map[string]interface{}{
			"error": err.Error(),
			"code":  code,
		})
	}
}

// getRequestID 获取请求ID
func getRequestID(r *http.Request) string {
	if id := r.Header.Get("X-Request-Id"); id != "" {
		return id
	}
	return r.Header.Get("X-Request-ID")
}

// getClientIP 获取客户端IP
// SEC-003: 添加可信代理验证，仅在请求来自可信代理时信任 X-Forwarded-For
func getClientIP(r *http.Request, trustedProxies ...string) string {
	// 检查请求是否来自可信代理
	if isTrustedProxy(r.RemoteAddr, trustedProxies) {
		// 来自可信代理，信任代理头
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				ip := strings.TrimSpace(parts[0])
				return cleanIP(ip)
			}
		}
		// X-Real-IP
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return cleanIP(xri)
		}
	}

	// 未配置可信代理或请求不来自可信代理，使用 RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// isTrustedProxy 检查请求是否来自可信代理
// SEC-003: 防止 IP spoofing 攻击
func isTrustedProxy(remoteAddr string, trustedProxies []string) bool {
	if len(trustedProxies) == 0 {
		return false
	}
	remoteIP := extractIPFromAddr(remoteAddr)
	for _, cidr := range trustedProxies {
		if containsCIDR(remoteIP, cidr) {
			return true
		}
	}
	return false
}

// containsCIDR 检查IP是否在CIDR范围内
func containsCIDR(ip, cidr string) bool {
	// 简化实现：直接比较或检查前缀
	// 完整实现应使用 net/netip.ParseAddr 和 netip.ParsePrefix
	if strings.HasPrefix(cidr, "10.") {
		// 10.0.0.0/8
		return strings.HasPrefix(ip, "10.")
	}
	if strings.HasPrefix(cidr, "172.") {
		// 172.16.0.0/12
		return strings.HasPrefix(ip, "172.")
	}
	if strings.HasPrefix(cidr, "192.168.") {
		return strings.HasPrefix(ip, "192.168.")
	}
	// 直接匹配
	return ip == cidr
}

// extractIPFromAddr 从 RemoteAddr 提取 IP
func extractIPFromAddr(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// cleanIP 清理IP地址，移除端口号和其他非IP字符
// 防御IP spoofing：确保返回的是有效的IP格式
func cleanIP(ip string) string {
	// 移除端口号（如 "203.0.113.1:8080" -> "203.0.113.1"）
	if colonIdx := strings.LastIndex(ip, ":"); colonIdx != -1 {
		// 检查冒号后面是否都是数字（可能是端口）
		portPart := ip[colonIdx+1:]
		isPort := true
		for _, c := range portPart {
			if c < '0' || c > '9' {
				isPort = false
				break
			}
		}
		if isPort {
			ip = ip[:colonIdx]
		}
	}
	return strings.TrimSpace(ip)
}

// sanitizeRoute 清理路由字符串，防止路径遍历和其他安全问题
// MED-04: 审计日志Route字段需要验证以防止路径遍历攻击
func sanitizeRoute(route string) string {
	if route == "" {
		return route
	}

	// 检查是否包含路径遍历模式
	// 路径遍历通常包含 .. 或 . 后面跟着 / 或 \
	for i := 0; i < len(route)-1; i++ {
		if route[i] == '.' {
			next := route[i+1]
			if next == '.' || next == '/' || next == '\\' {
				// 检测到路径遍历模式，返回安全的替代值
				return "/sanitized"
			}
		}
		// 检查反斜杠（Windows路径遍历）
		if route[i] == '\\' {
			return "/sanitized"
		}
	}

	// 检查null字节
	if strings.Contains(route, "\x00") {
		return "/sanitized"
	}

	// 检查换行符
	if strings.Contains(route, "\n") || strings.Contains(route, "\r") {
		return "/sanitized"
	}

	return route
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
	data    map[string]cacheEntry
	mu      sync.RWMutex
	cleanup int64 // 清理触发计数器
}

type cacheEntry struct {
	status  string
	expires time.Time
}

// NewTokenCache 创建token缓存
func NewTokenCache() *TokenCache {
	return &TokenCache{
		data:    make(map[string]cacheEntry),
		cleanup: 0,
	}
}

// Get 获取token状态
func (c *TokenCache) Get(tokenID string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if entry, ok := c.data[tokenID]; ok {
		if time.Now().Before(entry.expires) {
			return entry.status, true
		}
	}
	return "", false
}

// Set 设置token状态
func (c *TokenCache) Set(tokenID, status string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[tokenID] = cacheEntry{
		status:  status,
		expires: time.Now().Add(ttl),
	}
	c.triggerCleanup()
}

// Invalidate 使token失效
func (c *TokenCache) Invalidate(tokenID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, tokenID)
}

// triggerCleanup 触发清理（每100次操作清理一次过期条目）
func (c *TokenCache) triggerCleanup() {
	c.cleanup++
	if c.cleanup >= 100 {
		c.cleanup = 0
		c.cleanupExpiredLocked()
	}
}

// cleanupExpiredLocked 清理过期条目（需要持有锁）
func (c *TokenCache) cleanupExpiredLocked() {
	now := time.Now()
	for tokenID, entry := range c.data {
		if now.After(entry.expires) {
			delete(c.data, tokenID)
		}
	}
}

// CleanExpired 主动清理过期条目（可由外部定期调用）
func (c *TokenCache) CleanExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cleanupExpiredLocked()
}

// Len 返回缓存条目数量（用于监控）
func (c *TokenCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

// ComputeFingerprint 计算凭证指纹（用于审计）
func ComputeFingerprint(credential string) string {
	hash := sha256.Sum256([]byte(credential))
	return hex.EncodeToString(hash[:])
}
