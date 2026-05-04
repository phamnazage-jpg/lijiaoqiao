package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bridge/ai-customer-service/internal/domain/error/cserrors"
)

const (
	HeaderActorID   = "X-CS-Actor-ID"
	HeaderActorRole = "X-CS-Actor-Role"
)

type Actor struct {
	ID   string
	Role string
}

type actorContextKey struct{}

func WithActor(ctx context.Context, id, role string) context.Context {
	return context.WithValue(ctx, actorContextKey{}, Actor{
		ID:   strings.TrimSpace(id),
		Role: normalizeRole(role),
	})
}

func ActorFromContext(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(actorContextKey{}).(Actor)
	if !ok {
		return Actor{}, false
	}
	if strings.TrimSpace(actor.ID) == "" || strings.TrimSpace(actor.Role) == "" {
		return Actor{}, false
	}
	return actor, true
}

func RequireRoles(next http.Handler, allowedRoles ...string) http.Handler {
	allowed := make(map[string]struct{}, len(allowedRoles))
	for _, role := range allowedRoles {
		if normalized := normalizeRole(role); normalized != "" {
			allowed[normalized] = struct{}{}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actorID := strings.TrimSpace(r.Header.Get(HeaderActorID))
		role := normalizeRole(r.Header.Get(HeaderActorRole))
		if actorID == "" || role == "" {
			writeAccessDenied(w)
			return
		}
		if _, ok := allowed[role]; !ok {
			writeAccessDenied(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithActor(r.Context(), actorID, role)))
	})
}

func normalizeRole(role string) string {
	return strings.ToLower(strings.TrimSpace(role))
}

func writeAccessDenied(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":    cserrors.CS_AUTH_4001,
			"message": cserrors.ErrorMsg(cserrors.CS_AUTH_4001),
		},
	})
}
