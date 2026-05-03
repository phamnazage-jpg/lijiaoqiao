package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
	"github.com/bridge/ai-customer-service/internal/domain/ticketstats"
	"github.com/bridge/ai-customer-service/internal/platform/health"
)

func TestHealthHandler_Live_ReturnsUPWhenLive(t *testing.T) {
	probe := health.NewProbe()
	probe.SetLive(true)
	h := NewHealthHandler(probe)

	req := httptest.NewRequest(http.MethodGet, "/actuator/health/live", nil)
	rr := httptest.NewRecorder()
	h.Live(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Live() status = %d, want 200", rr.Code)
	}
}

func TestHealthHandler_Live_ReturnsDOWNWhenNotLive(t *testing.T) {
	probe := health.NewProbe()
	probe.SetLive(false)
	h := NewHealthHandler(probe)

	req := httptest.NewRequest(http.MethodGet, "/actuator/health/live", nil)
	rr := httptest.NewRecorder()
	h.Live(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Live() status = %d, want 503", rr.Code)
	}
}

func TestHealthHandler_Live_WithNilProbe(t *testing.T) {
	h := NewHealthHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/actuator/health/live", nil)
	rr := httptest.NewRecorder()
	h.Live(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Live() with nil probe status = %d, want 200", rr.Code)
	}
}

func TestHealthHandler_Ready_WithFailingChecker(t *testing.T) {
	probe := health.NewProbe()
	probe.SetLive(true)
	h := NewHealthHandler(probe, &failingHealthChecker{})

	req := httptest.NewRequest(http.MethodGet, "/actuator/health/ready", nil)
	rr := httptest.NewRecorder()
	h.Ready(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Ready() with failing checker status = %d, want 503", rr.Code)
	}
}

func TestHealthHandler_Ready_WithPassingChecker(t *testing.T) {
	probe := health.NewProbe()
	probe.SetLive(true)
	probe.SetReady(true)
	h := NewHealthHandler(probe, &passingHealthChecker{})

	req := httptest.NewRequest(http.MethodGet, "/actuator/health/ready", nil)
	rr := httptest.NewRecorder()
	h.Ready(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Ready() with passing checker status = %d, want 200", rr.Code)
	}
}

func TestHealthHandler_Ready_ReturnsDownWhenProbeNotReady(t *testing.T) {
	probe := health.NewProbe()
	probe.SetLive(true)
	probe.SetReady(false)
	h := NewHealthHandler(probe, &passingHealthChecker{})

	req := httptest.NewRequest(http.MethodGet, "/actuator/health/ready", nil)
	rr := httptest.NewRecorder()
	h.Ready(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Ready() with probe not ready status = %d, want 503", rr.Code)
	}
}

func TestHealthHandler_Health_ReturnsOK(t *testing.T) {
	probe := health.NewProbe()
	probe.SetLive(true)
	h := NewHealthHandler(probe)

	req := httptest.NewRequest(http.MethodGet, "/actuator/health", nil)
	rr := httptest.NewRecorder()
	h.Health(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Health() status = %d, want 200", rr.Code)
	}
}

// --- TicketStatsHandler tests ---

func TestTicketStatsHandler_Get_Success(t *testing.T) {
	mock := &mockTicketStatsServiceForStats{
		stats: ticketstats.Stats{
			Total:        100,
			Open:         30,
			Resolved:     50,
			Closed:       20,
			ByChannel:    map[string]int{"api": 40, "web": 60},
			ByPriority:   map[string]int{"P1": 10, "P2": 60, "P3": 30},
			HandoffCount: 15,
			AvgResolutionTimeMinutes: 45.5,
		},
		err: nil,
	}
	recorder := &stubAuditRecorderForStats{}
	h := NewTicketStatsHandler(mock, recorder)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/tickets/stats", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Get() status = %d, want 200", rr.Code)
	}
}

func TestTicketStatsHandler_Get_Error(t *testing.T) {
	mock := &mockTicketStatsServiceForStats{
		stats: ticketstats.Stats{},
		err:   errStub{"stats error"},
	}
	recorder := &stubAuditRecorderForStats{}
	h := NewTicketStatsHandler(mock, recorder)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/tickets/stats", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Get() with error status = %d, want 500", rr.Code)
	}
}

func TestTicketStatsHandler_Get_NilAudit(t *testing.T) {
	mock := &mockTicketStatsServiceForStats{
		stats: ticketstats.Stats{},
		err:   nil,
	}
	h := NewTicketStatsHandler(mock, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/tickets/stats", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Get() with nil audit status = %d, want 200", rr.Code)
	}
}

// --- Test doubles ---

type passingHealthChecker struct{}

func (c *passingHealthChecker) Name() string { return "passing" }

func (c *passingHealthChecker) Check(ctx context.Context) error { return nil }

type failingHealthChecker struct{}

func (c *failingHealthChecker) Name() string { return "failing" }

func (c *failingHealthChecker) Check(ctx context.Context) error {
	return errStub{"checker failed"}
}

type errStub struct{ msg string }

func (e errStub) Error() string { return e.msg }

type mockTicketStatsServiceForStats struct {
	stats ticketstats.Stats
	err   error
}

func (m *mockTicketStatsServiceForStats) GetStats(ctx context.Context) (ticketstats.Stats, error) {
	return m.stats, m.err
}

type stubAuditRecorderForStats struct{}

func (s *stubAuditRecorderForStats) Add(ctx context.Context, event audit.Event) error {
	return nil
}
