package middleware

import (
	"net/http"
	"strings"
	"time"

	"lijiaoqiao/platform-token-runtime/internal/auth/service"
)

var disallowedQueryKeys = []string{"key", "api_key", "token"}

func QueryKeyRejectMiddleware(next http.Handler, auditor service.AuditEmitter, now func() time.Time) http.Handler {
	if next == nil {
		next = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	}
	if now == nil {
		now = defaultNowFunc
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, exists := externalQueryKey(r)
		if !exists {
			next.ServeHTTP(w, r)
			return
		}

		requestID := ensureRequestID(r, now)
		emitAuditEvent(r.Context(), auditor, service.AuditEvent{
			EventName:  service.EventTokenQueryKeyRejected,
			RequestID:  requestID,
			Route:      r.URL.Path,
			ResultCode: service.CodeQueryKeyNotAllowed,
			ClientIP:   extractClientIP(r),
			CreatedAt:  now(),
		})
		writeError(w, http.StatusUnauthorized, requestID, service.CodeQueryKeyNotAllowed, "query key ingress is not allowed")
	})
}

func externalQueryKey(r *http.Request) (string, bool) {
	values := r.URL.Query()
	for key := range values {
		lowered := strings.ToLower(key)
		for _, disallowed := range disallowedQueryKeys {
			if lowered == disallowed {
				return key, true
			}
		}
	}
	return "", false
}
