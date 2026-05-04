package handlers

import (
	"net/http"

	"github.com/bridge/ai-customer-service/internal/http/middleware"
)

func withActor(req *http.Request, actorID, role string) *http.Request {
	return req.WithContext(middleware.WithActor(req.Context(), actorID, role))
}
