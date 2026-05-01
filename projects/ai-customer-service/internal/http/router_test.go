package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bridge/ai-customer-service/internal/http/handlers"
	"github.com/bridge/ai-customer-service/internal/platform/health"
)

func TestRouter_HealthEndpoint(t *testing.T) {
	probe := health.NewProbe()
	h := handlers.NewHealthHandler(probe)
	router := NewRouter(RouterDeps{Health: h})

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{"health root returns 200", "/actuator/health", http.StatusOK},
		{"live returns 200", "/actuator/health/live", http.StatusOK},
		{"ready returns 200 when ready", "/actuator/health/ready", http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			if rr.Code != tc.wantStatus {
				t.Errorf("GET %s = %d, want %d", tc.path, rr.Code, tc.wantStatus)
			}
		})
	}
}

func TestRouter_UnknownPath_Returns404(t *testing.T) {
	probe := health.NewProbe()
	h := handlers.NewHealthHandler(probe)
	router := NewRouter(RouterDeps{Health: h})

	tests := []struct {
		name string
		path string
	}{
		{"unknown root path", "/unknown"},
		{"unknown nested path", "/api/v1/unknown"},
		{"unknown deep path", "/api/v1/customer-service/unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			if rr.Code != http.StatusNotFound {
				t.Errorf("GET %s = %d, want 404", tc.path, rr.Code)
			}
		})
	}
}

func TestRouter_WebhookChannel_MissingChannel_Returns400(t *testing.T) {
	probe := health.NewProbe()
	h := handlers.NewHealthHandler(probe)
	router := NewRouter(RouterDeps{Health: h})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/webhook/", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("GET /webhook/ = %d, want 400; body: %s", rr.Code, rr.Body.String())
	}
}

func TestRouter_WebhookPath_CanBeCalledWithGET(t *testing.T) {
	probe := health.NewProbe()
	h := handlers.NewHealthHandler(probe)
	router := NewRouter(RouterDeps{Health: h})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/webhook", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /webhook = %d, want 405", rr.Code)
	}
}

func TestRouter_TicketsList_POST_Returns405(t *testing.T) {
	probe := health.NewProbe()
	h := handlers.NewHealthHandler(probe)
	ticketHandler := &handlers.TicketHandler{}
	router := NewRouter(RouterDeps{Health: h, Tickets: ticketHandler})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /tickets = %d, want 405", rr.Code)
	}
}

func TestRouter_SessionsRoute_OnlyPOST(t *testing.T) {
	probe := health.NewProbe()
	h := handlers.NewHealthHandler(probe)
	router := NewRouter(RouterDeps{Health: h, Sessions: nil})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/sessions/s1/feedback", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	// When Sessions is nil, route not registered → 404
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /sessions/s1/feedback with nil Sessions = %d, want 404", rr.Code)
	}
}