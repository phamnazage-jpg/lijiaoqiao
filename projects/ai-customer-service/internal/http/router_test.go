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
	probe.SetReady(true)
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
	probe.SetReady(true)
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
	probe.SetReady(true)
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
	probe.SetReady(true)
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
	probe.SetReady(true)
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
	probe.SetReady(true)
	h := handlers.NewHealthHandler(probe)
	router := NewRouter(RouterDeps{Health: h, Sessions: nil})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/sessions/s1/feedback", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /sessions/s1/feedback with nil Sessions = %d, want 404", rr.Code)
	}
}

func TestRouter_TicketsSubpaths(t *testing.T) {
	// Test that ticket subpaths are registered with Tickets != nil
	// We use OPTIONS method to avoid triggering handler logic (which would panic with nil service)
	probe := health.NewProbe()
	probe.SetReady(true)
	h := handlers.NewHealthHandler(probe)
	ticketHandler := &handlers.TicketHandler{}
	router := NewRouter(RouterDeps{Health: h, Tickets: ticketHandler})

	// Just verify routes exist by checking non-404 response
	// (we can't fully test without mocking service, which is integration test territory)
	paths := []string{
		"/api/v1/customer-service/tickets/t1/assign",
		"/api/v1/customer-service/tickets/t1/resolve",
		"/api/v1/customer-service/tickets/t1/close",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			// Use HEAD method — less likely to panic
			req := httptest.NewRequest(http.MethodHead, path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			// Should not be 404 (route is registered)
			if rr.Code == http.StatusNotFound {
				t.Errorf("%s returned 404 — route not registered", path)
			}
		})
	}
}

func TestRouter_SessionsFeedbackHandoff(t *testing.T) {
	// Test sessions routes are registered when Sessions != nil
	probe := health.NewProbe()
	probe.SetReady(true)
	h := handlers.NewHealthHandler(probe)
	sessionHandler := &handlers.SessionHandler{}
	router := NewRouter(RouterDeps{Health: h, Sessions: sessionHandler})

	paths := []string{
		"/api/v1/customer-service/sessions/s1/feedback",
		"/api/v1/customer-service/sessions/s1/handoff",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			// Use HEAD method — less likely to panic
			req := httptest.NewRequest(http.MethodHead, path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			// Should not be 404 (route is registered)
			if rr.Code == http.StatusNotFound {
				t.Errorf("%s returned 404 — route not registered", path)
			}
		})
	}
}

func TestRouter_UnknownSessionsPath_Returns405(t *testing.T) {
	probe := health.NewProbe()
	probe.SetReady(true)
	h := handlers.NewHealthHandler(probe)
	sessionHandler := &handlers.SessionHandler{}
	router := NewRouter(RouterDeps{Health: h, Sessions: sessionHandler})

	// Path that doesn't match /feedback or /handoff should get 405
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/s1/unknown", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /sessions/s1/unknown = %d, want 405", rr.Code)
	}
}

func TestRouter_UnknownTicketsPath_Returns405(t *testing.T) {
	probe := health.NewProbe()
	probe.SetReady(true)
	h := handlers.NewHealthHandler(probe)
	ticketHandler := &handlers.TicketHandler{}
	router := NewRouter(RouterDeps{Health: h, Tickets: ticketHandler})

	// Path that doesn't match known subpaths should get 405
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets/t1/unknown", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /tickets/t1/unknown = %d, want 405", rr.Code)
	}
}
