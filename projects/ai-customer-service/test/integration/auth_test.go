package integration

import (
	"net/http"

	"github.com/bridge/ai-customer-service/internal/http/middleware"
)

func withActor(req *http.Request, actorID, role string) *http.Request {
	return req.WithContext(middleware.WithActor(req.Context(), actorID, role))
}

func setActorHeaders(req *http.Request, actorID, role string) {
	req.Header.Set(middleware.HeaderActorID, actorID)
	req.Header.Set(middleware.HeaderActorRole, role)
}
