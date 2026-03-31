package service

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	CodeAuthMissingBearer  = "AUTH_MISSING_BEARER"
	CodeQueryKeyNotAllowed = "QUERY_KEY_NOT_ALLOWED"
	CodeAuthInvalidToken   = "AUTH_INVALID_TOKEN"
	CodeAuthTokenInactive  = "AUTH_TOKEN_INACTIVE"
	CodeAuthScopeDenied    = "AUTH_SCOPE_DENIED"
	CodeAuthNotReady       = "AUTH_NOT_READY"
)

const (
	EventTokenAuthnSuccess      = "token.authn.success"
	EventTokenAuthnFail         = "token.authn.fail"
	EventTokenAuthzDenied       = "token.authz.denied"
	EventTokenQueryKeyRejected  = "token.query_key.rejected"
	EventTokenIssueSuccess      = "token.issue.success"
	EventTokenIssueFail         = "token.issue.fail"
	EventTokenIntrospectSuccess = "token.introspect.success"
	EventTokenIntrospectFail    = "token.introspect.fail"
	EventTokenRefreshSuccess    = "token.refresh.success"
	EventTokenRefreshFail       = "token.refresh.fail"
	EventTokenRevokeSuccess     = "token.revoke.success"
	EventTokenRevokeFail        = "token.revoke.fail"
)

type TokenStatus string

const (
	TokenStatusActive  TokenStatus = "active"
	TokenStatusRevoked TokenStatus = "revoked"
	TokenStatusExpired TokenStatus = "expired"
)

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

type TokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (VerifiedToken, error)
}

type TokenStatusResolver interface {
	Resolve(ctx context.Context, tokenID string) (TokenStatus, error)
}

type RouteAuthorizer interface {
	Authorize(path, method string, scopes []string, role string) bool
}

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

type AuditEmitter interface {
	Emit(ctx context.Context, event AuditEvent) error
}

type AuditEventFilter struct {
	RequestID  string
	TokenID    string
	SubjectID  string
	EventName  string
	ResultCode string
	Limit      int
}

type AuditEventQuerier interface {
	QueryEvents(ctx context.Context, filter AuditEventFilter) ([]AuditEvent, error)
}

type AuthError struct {
	Code  string
	Cause error
}

func (e *AuthError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return e.Code
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Cause)
}

func (e *AuthError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewAuthError(code string, cause error) *AuthError {
	return &AuthError{Code: code, Cause: cause}
}

func IsAuthCode(err error, code string) bool {
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		return false
	}
	return authErr.Code == code
}
