package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const requestIDHeader = "X-Request-Id"

var defaultNowFunc = time.Now

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	principalKey contextKey = "principal"
)

// Principal 认证成功后的主体信息
type Principal struct {
	RequestID string
	TokenID   string
	SubjectID string
	Role      string
	Scope     []string
}

// BuildTokenAuthChain 构建认证中间件链
func BuildTokenAuthChain(cfg AuthMiddlewareConfig, next http.Handler) http.Handler {
	handler := tokenAuthMiddleware(cfg)(next)
	handler = queryKeyRejectMiddleware(handler, cfg.Auditor, cfg.Now)
	handler = requestIDMiddleware(handler, cfg.Now)
	return handler
}

// RequestIDMiddleware 请求ID中间件
func requestIDMiddleware(next http.Handler, now func() time.Time) http.Handler {
	if next == nil {
		return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	}
	if now == nil {
		now = defaultNowFunc
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := ensureRequestID(r, now)
		w.Header().Set(requestIDHeader, requestID)
		next.ServeHTTP(w, r)
	})
}

// queryKeyRejectMiddleware 拒绝query key入站
func queryKeyRejectMiddleware(next http.Handler, auditor AuditEmitter, now func() time.Time) http.Handler {
	if next == nil {
		return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	}
	if now == nil {
		now = defaultNowFunc
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hasExternalQueryKey(r) {
			requestID, _ := RequestIDFromContext(r.Context())
			emitAudit(r.Context(), auditor, AuditEvent{
				EventName:  EventTokenQueryKeyRejected,
				RequestID:  requestID,
				Route:      r.URL.Path,
				ResultCode: CodeQueryKeyNotAllowed,
				ClientIP:   extractClientIP(r),
				CreatedAt:  now(),
			})
			writeError(w, http.StatusUnauthorized, requestID, CodeQueryKeyNotAllowed, "query key not allowed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// tokenAuthMiddleware Token认证中间件
func tokenAuthMiddleware(cfg AuthMiddlewareConfig) func(http.Handler) http.Handler {
	cfg = cfg.withDefaults()
	return func(next http.Handler) http.Handler {
		if next == nil {
			next = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.shouldProtect(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			requestID := ensureRequestID(r, cfg.Now)
			if cfg.Verifier == nil || cfg.StatusResolver == nil || cfg.Authorizer == nil {
				writeError(w, http.StatusServiceUnavailable, requestID, CodeAuthNotReady, "auth middleware dependencies are not ready")
				return
			}

			rawToken, ok := extractBearerToken(r.Header.Get("Authorization"))
			if !ok {
				emitAudit(r.Context(), cfg.Auditor, AuditEvent{
					EventName:  EventTokenAuthnFail,
					RequestID:  requestID,
					Route:      r.URL.Path,
					ResultCode: CodeAuthMissingBearer,
					ClientIP:   extractClientIP(r),
					CreatedAt:  cfg.Now(),
				})
				writeError(w, http.StatusUnauthorized, requestID, CodeAuthMissingBearer, "missing bearer token")
				return
			}

			claims, err := cfg.Verifier.Verify(r.Context(), rawToken)
			if err != nil {
				emitAudit(r.Context(), cfg.Auditor, AuditEvent{
					EventName:  EventTokenAuthnFail,
					RequestID:  requestID,
					Route:      r.URL.Path,
					ResultCode: CodeAuthInvalidToken,
					ClientIP:   extractClientIP(r),
					CreatedAt:  cfg.Now(),
				})
				writeError(w, http.StatusUnauthorized, requestID, CodeAuthInvalidToken, "invalid bearer token")
				return
			}

			tokenStatus, err := cfg.StatusResolver.Resolve(r.Context(), claims.TokenID)
			if err != nil || tokenStatus != TokenStatusActive {
				emitAudit(r.Context(), cfg.Auditor, AuditEvent{
					EventName:  EventTokenAuthnFail,
					RequestID:  requestID,
					TokenID:    claims.TokenID,
					SubjectID:  claims.SubjectID,
					Route:      r.URL.Path,
					ResultCode: CodeAuthTokenInactive,
					ClientIP:   extractClientIP(r),
					CreatedAt:  cfg.Now(),
				})
				writeError(w, http.StatusUnauthorized, requestID, CodeAuthTokenInactive, "token is inactive")
				return
			}

			if !cfg.Authorizer.Authorize(r.URL.Path, r.Method, claims.Scope, claims.Role) {
				emitAudit(r.Context(), cfg.Auditor, AuditEvent{
					EventName:  EventTokenAuthzDenied,
					RequestID:  requestID,
					TokenID:    claims.TokenID,
					SubjectID:  claims.SubjectID,
					Route:      r.URL.Path,
					ResultCode: CodeAuthScopeDenied,
					ClientIP:   extractClientIP(r),
					CreatedAt:  cfg.Now(),
				})
				writeError(w, http.StatusForbidden, requestID, CodeAuthScopeDenied, "scope denied")
				return
			}

			principal := Principal{
				RequestID: requestID,
				TokenID:   claims.TokenID,
				SubjectID: claims.SubjectID,
				Role:      claims.Role,
				Scope:     append([]string(nil), claims.Scope...),
			}
			ctx := context.WithValue(r.Context(), principalKey, principal)
			ctx = context.WithValue(ctx, requestIDKey, requestID)

			emitAudit(ctx, cfg.Auditor, AuditEvent{
				EventName:  EventTokenAuthnSuccess,
				RequestID:  requestID,
				TokenID:    claims.TokenID,
				SubjectID:  claims.SubjectID,
				Route:      r.URL.Path,
				ResultCode: "OK",
				ClientIP:   extractClientIP(r),
				CreatedAt:  cfg.Now(),
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestIDFromContext 从Context获取请求ID
func RequestIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	value, ok := ctx.Value(requestIDKey).(string)
	return value, ok
}

// PrincipalFromContext 从Context获取认证主体
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	if ctx == nil {
		return Principal{}, false
	}
	value, ok := ctx.Value(principalKey).(Principal)
	return value, ok
}

func (cfg AuthMiddlewareConfig) withDefaults() AuthMiddlewareConfig {
	if cfg.Now == nil {
		cfg.Now = defaultNowFunc
	}
	if len(cfg.ProtectedPrefixes) == 0 {
		cfg.ProtectedPrefixes = []string{"/api/v1/supply", "/api/v1/platform"}
	}
	if len(cfg.ExcludedPrefixes) == 0 {
		cfg.ExcludedPrefixes = []string{"/health", "/healthz", "/metrics", "/readyz"}
	}
	return cfg
}

func (cfg AuthMiddlewareConfig) shouldProtect(path string) bool {
	for _, prefix := range cfg.ExcludedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return false
		}
	}
	for _, prefix := range cfg.ProtectedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func ensureRequestID(r *http.Request, now func() time.Time) string {
	if now == nil {
		now = defaultNowFunc
	}
	if requestID, ok := RequestIDFromContext(r.Context()); ok && requestID != "" {
		return requestID
	}
	requestID := strings.TrimSpace(r.Header.Get(requestIDHeader))
	if requestID == "" {
		requestID = fmt.Sprintf("req-%d", now().UnixNano())
	}
	ctx := context.WithValue(r.Context(), requestIDKey, requestID)
	*r = *r.WithContext(ctx)
	return requestID
}

func extractBearerToken(authHeader string) (string, bool) {
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
	return token, token != ""
}

func hasExternalQueryKey(r *http.Request) bool {
	if r.URL == nil {
		return false
	}
	query := r.URL.Query()
	for key := range query {
		lowerKey := strings.ToLower(key)
		if lowerKey == "key" || lowerKey == "api_key" || lowerKey == "token" || lowerKey == "access_token" {
			return true
		}
	}
	return false
}

func emitAudit(ctx context.Context, auditor AuditEmitter, event AuditEvent) {
	if auditor == nil {
		return
	}
	_ = auditor.Emit(ctx, event)
}

type errorResponse struct {
	RequestID string       `json:"request_id"`
	Error     errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func writeError(w http.ResponseWriter, status int, requestID, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	payload := errorResponse{
		RequestID: requestID,
		Error: errorPayload{
			Code:    code,
			Message: message,
		},
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func extractClientIP(r *http.Request) string {
	xForwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xForwardedFor != "" {
		parts := strings.Split(xForwardedFor, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}