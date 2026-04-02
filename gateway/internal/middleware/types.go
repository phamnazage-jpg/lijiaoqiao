package middleware

import (
	"context"
	"time"
)

// 认证常量
const (
	CodeAuthMissingBearer  = "AUTH_MISSING_BEARER"
	CodeQueryKeyNotAllowed = "QUERY_KEY_NOT_ALLOWED"
	CodeAuthInvalidToken   = "AUTH_INVALID_TOKEN"
	CodeAuthTokenInactive  = "AUTH_TOKEN_INACTIVE"
	CodeAuthScopeDenied    = "AUTH_SCOPE_DENIED"
	CodeAuthNotReady       = "AUTH_NOT_READY"
)

// 审计事件常量
const (
	EventTokenAuthnSuccess     = "token.authn.success"
	EventTokenAuthnFail        = "token.authn.fail"
	EventTokenAuthzDenied      = "token.authz.denied"
	EventTokenQueryKeyRejected = "token.query_key.rejected"
)

// TokenStatus Token状态
type TokenStatus string

const (
	TokenStatusActive  TokenStatus = "active"
	TokenStatusRevoked TokenStatus = "revoked"
	TokenStatusExpired TokenStatus = "expired"
)

// VerifiedToken 验证后的Token声明
type VerifiedToken struct {
	TokenID   string
	SubjectID string
	Role      string
	Scope     []string
	IssuedAt  time.Time
	ExpiresAt time.Time
	NotBefore time.Time
	Issuer    string
	Audience  string
}

// TokenVerifier Token验证器接口
type TokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (VerifiedToken, error)
}

// TokenStatusResolver Token状态解析器接口
type TokenStatusResolver interface {
	Resolve(ctx context.Context, tokenID string) (TokenStatus, error)
}

// RouteAuthorizer 路由授权器接口
type RouteAuthorizer interface {
	Authorize(path, method string, scopes []string, role string) bool
}

// AuditEvent 审计事件
type AuditEvent struct {
	EventID    string
	EventName  string
	RequestID  string
	TokenID    string
	SubjectID  string
	Route      string
	ResultCode string
	ClientIP   string
	CreatedAt  time.Time
}

// AuditEmitter 审计事件发射器接口
type AuditEmitter interface {
	Emit(ctx context.Context, event AuditEvent) error
}

// AuthMiddlewareConfig 认证中间件配置
type AuthMiddlewareConfig struct {
	Verifier          TokenVerifier
	StatusResolver    TokenStatusResolver
	Authorizer        RouteAuthorizer
	Auditor           AuditEmitter
	ProtectedPrefixes []string
	ExcludedPrefixes  []string
	Now               func() time.Time
	// TrustedProxies 可信的代理IP列表，用于IP伪造防护
	// 只有来自这些IP的请求才会使用X-Forwarded-For头
	TrustedProxies []string
}