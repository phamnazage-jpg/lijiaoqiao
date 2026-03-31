package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"lijiaoqiao/platform-token-runtime/internal/auth/model"
	"lijiaoqiao/platform-token-runtime/internal/auth/service"
)

const requestIDHeader = "X-Request-Id"

var defaultNowFunc = time.Now

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	principalKey contextKey = "principal"
)

type AuthMiddlewareConfig struct {
	Verifier         service.TokenVerifier
	StatusResolver   service.TokenStatusResolver
	Authorizer       service.RouteAuthorizer
	Auditor          service.AuditEmitter
	ProtectedPrefixes []string
	ExcludedPrefixes []string
	Now              func() time.Time
}

func BuildTokenAuthChain(cfg AuthMiddlewareConfig, next http.Handler) http.Handler {
	handler := TokenAuthMiddleware(cfg)(next)
	handler = QueryKeyRejectMiddleware(handler, cfg.Auditor, cfg.Now)
	handler = RequestIDMiddleware(handler, cfg.Now)
	return handler
}

func RequestIDMiddleware(next http.Handler, now func() time.Time) http.Handler {
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

func TokenAuthMiddleware(cfg AuthMiddlewareConfig) func(http.Handler) http.Handler {
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
				writeError(w, http.StatusServiceUnavailable, requestID, service.CodeAuthNotReady, "auth middleware dependencies are not ready")
				return
			}

			rawToken, ok := extractBearerToken(r.Header.Get("Authorization"))
			if !ok {
				emitAuditEvent(r.Context(), cfg.Auditor, service.AuditEvent{
					EventName:  service.EventTokenAuthnFail,
					RequestID:  requestID,
					Route:      r.URL.Path,
					ResultCode: service.CodeAuthMissingBearer,
					ClientIP:   extractClientIP(r),
					CreatedAt:  cfg.Now(),
				})
				writeError(w, http.StatusUnauthorized, requestID, service.CodeAuthMissingBearer, "missing bearer token")
				return
			}

			claims, err := cfg.Verifier.Verify(r.Context(), rawToken)
			if err != nil {
				emitAuditEvent(r.Context(), cfg.Auditor, service.AuditEvent{
					EventName:  service.EventTokenAuthnFail,
					RequestID:  requestID,
					Route:      r.URL.Path,
					ResultCode: service.CodeAuthInvalidToken,
					ClientIP:   extractClientIP(r),
					CreatedAt:  cfg.Now(),
				})
				writeError(w, http.StatusUnauthorized, requestID, service.CodeAuthInvalidToken, "invalid bearer token")
				return
			}

			tokenStatus, err := cfg.StatusResolver.Resolve(r.Context(), claims.TokenID)
			if err != nil || tokenStatus != service.TokenStatusActive {
				emitAuditEvent(r.Context(), cfg.Auditor, service.AuditEvent{
					EventName:  service.EventTokenAuthnFail,
					RequestID:  requestID,
					TokenID:    claims.TokenID,
					SubjectID:  claims.SubjectID,
					Route:      r.URL.Path,
					ResultCode: service.CodeAuthTokenInactive,
					ClientIP:   extractClientIP(r),
					CreatedAt:  cfg.Now(),
				})
				writeError(w, http.StatusUnauthorized, requestID, service.CodeAuthTokenInactive, "token is inactive")
				return
			}

			if !cfg.Authorizer.Authorize(r.URL.Path, r.Method, claims.Scope, claims.Role) {
				emitAuditEvent(r.Context(), cfg.Auditor, service.AuditEvent{
					EventName:  service.EventTokenAuthzDenied,
					RequestID:  requestID,
					TokenID:    claims.TokenID,
					SubjectID:  claims.SubjectID,
					Route:      r.URL.Path,
					ResultCode: service.CodeAuthScopeDenied,
					ClientIP:   extractClientIP(r),
					CreatedAt:  cfg.Now(),
				})
				writeError(w, http.StatusForbidden, requestID, service.CodeAuthScopeDenied, "scope denied")
				return
			}

			principal := model.Principal{
				RequestID: requestID,
				TokenID:   claims.TokenID,
				SubjectID: claims.SubjectID,
				Role:      claims.Role,
				Scope:     append([]string(nil), claims.Scope...),
			}
			ctx := context.WithValue(r.Context(), principalKey, principal)
			ctx = context.WithValue(ctx, requestIDKey, requestID)

			emitAuditEvent(ctx, cfg.Auditor, service.AuditEvent{
				EventName:  service.EventTokenAuthnSuccess,
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

func RequestIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	value, ok := ctx.Value(requestIDKey).(string)
	return value, ok
}

func PrincipalFromContext(ctx context.Context) (model.Principal, bool) {
	if ctx == nil {
		return model.Principal{}, false
	}
	value, ok := ctx.Value(principalKey).(model.Principal)
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
		cfg.ExcludedPrefixes = []string{"/healthz", "/metrics", "/readyz"}
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

func emitAuditEvent(ctx context.Context, auditor service.AuditEmitter, event service.AuditEvent) {
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
