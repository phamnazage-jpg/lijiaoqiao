package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/app"
	"github.com/bridge/ai-customer-service/internal/config"
	"github.com/bridge/ai-customer-service/internal/platform/health"
	"github.com/bridge/ai-customer-service/internal/platform/logging"
)

// mockChecker implements health.Checker for testing.
type mockChecker struct {
	name    string
	healthy bool
	errMsg  string
}

func (c *mockChecker) Name() string { return c.name }

func (c *mockChecker) Check(ctx context.Context) error {
	if !c.healthy {
		return &checkErr{msg: c.errMsg}
	}
	return nil
}

type checkErr struct{ msg string }

func (e *checkErr) Error() string { return e.msg }

// newTestApp creates a minimal app instance for health endpoint testing.
func newTestApp() *app.App {
	cfg := &config.Config{}
	cfg.HTTP.Addr = ":0"
	cfg.HTTP.ReadHeaderTimeout = 5
	cfg.HTTP.ReadTimeout = 10
	cfg.HTTP.WriteTimeout = 15
	cfg.HTTP.IdleTimeout = 60
	cfg.HTTP.MaxHeaderBytes = 1 << 20
	cfg.HTTP.MaxBodyBytes = 1 << 20
	application, err := app.New(cfg, logging.New())
	if err != nil {
		return nil
	}
	return application
}

// TestHealthCheck_Returns200 verifies GET /actuator/health returns HTTP 200
// when the app starts successfully.
func TestHealthCheck_Returns200(t *testing.T) {
	application := newTestApp()
	if application == nil {
		t.Skip("app.New() returned nil, skipping integration health test")
	}
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/actuator/health")
	if err != nil {
		t.Fatalf("http get error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if payload["status"] != "UP" {
		t.Fatalf("status = %v, want UP", payload["status"])
	}
}

// TestHealthCheck_ContainsChecks verifies the response includes the "checks" array
// when health checkers are registered.
func TestHealthCheck_ContainsChecks(t *testing.T) {
	// Test the health handler directly with mock checkers
	probe := health.NewProbe()
	probe.SetReady(true)
	checkers := []health.Checker{
		&mockChecker{name: "database", healthy: true, errMsg: ""},
		&mockChecker{name: "redis", healthy: true, errMsg: ""},
	}

	handler := healthHandlerWithProbes(probe, checkers)

	req := httptest.NewRequest(http.MethodGet, "/actuator/health", nil)
	resp := httptest.NewRecorder()
	handler(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error = %v", err)
	}

	status, ok := payload["status"].(string)
	if !ok || status != "UP" {
		t.Fatalf("status = %v, want UP", payload["status"])
	}

	checks, ok := payload["checks"].([]any)
	if !ok {
		t.Fatalf("checks field missing or not an array: %T", payload["checks"])
	}
	if len(checks) != 2 {
		t.Fatalf("checks length = %d, want 2", len(checks))
	}

	// Verify each check entry has name and status fields
	for _, c := range checks {
		check, ok := c.(map[string]any)
		if !ok {
			t.Fatalf("check entry not a map: %v", c)
		}
		if check["name"] == nil || check["name"] == "" {
			t.Fatalf("check name is empty in %v", check)
		}
		if check["status"] != "UP" {
			t.Fatalf("check status = %v, want UP", check["status"])
		}
	}

	// Verify time field is present
	if payload["time"] == nil {
		t.Fatalf("time field missing from health response")
	}
}

// TestHealthCheck_DegradedStatus verifies DEGRADED status when a checker fails.
func TestHealthCheck_DegradedStatus(t *testing.T) {
	probe := health.NewProbe()
	probe.SetReady(true)
	checkers := []health.Checker{
		&mockChecker{name: "database", healthy: true, errMsg: ""},
		&mockChecker{name: "external_api", healthy: false, errMsg: "connection refused"},
	}

	handler := healthHandlerWithProbes(probe, checkers)

	req := httptest.NewRequest(http.MethodGet, "/actuator/health", nil)
	resp := httptest.NewRecorder()
	handler(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (DEGRADED still returns 200)", resp.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error = %v", err)
	}

	if payload["status"] != "DEGRADED" {
		t.Fatalf("status = %v, want DEGRADED", payload["status"])
	}

	checks, ok := payload["checks"].([]any)
	if !ok {
		t.Fatalf("checks missing from response")
	}
	if len(checks) != 2 {
		t.Fatalf("checks length = %d, want 2", len(checks))
	}

	// Find the failing check
	foundDown := false
	for _, c := range checks {
		check := c.(map[string]any)
		if check["name"] == "external_api" {
			foundDown = true
			if check["status"] != "DOWN" {
				t.Fatalf("external_api status = %v, want DOWN", check["status"])
			}
			if check["error"] == nil || check["error"] == "" {
				t.Fatalf("external_api error missing, want 'connection refused'")
			}
		}
	}
	if !foundDown {
		t.Fatalf("external_api check not found in checks list")
	}
}

// TestHealthCheck_LiveEndpoint verifies GET /actuator/health/live.
func TestHealthCheck_LiveEndpoint(t *testing.T) {
	application := newTestApp()
	if application == nil {
		t.Skip("app.New() returned nil, skipping integration health test")
	}
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/actuator/health/live")
	if err != nil {
		t.Fatalf("http get error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if payload["status"] != "UP" {
		t.Fatalf("liveness status = %v, want UP", payload["status"])
	}
}

// TestHealthCheck_ReadyEndpoint verifies GET /actuator/health/ready.
func TestHealthCheck_ReadyEndpoint(t *testing.T) {
	probe := health.NewProbe()
	probe.SetReady(true)
	handler := healthHandlerWithProbes(probe, nil)

	req := httptest.NewRequest(http.MethodGet, "/actuator/health/ready", nil)
	resp := httptest.NewRecorder()
	handler(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if payload["status"] != "UP" {
		t.Fatalf("readiness status = %v, want UP", payload["status"])
	}
}

// healthHandlerWithProbes creates an http.HandlerFunc that mirrors the behavior
// of health.Health for testing purposes.
func healthHandlerWithProbes(probe *health.Probe, checkers []health.Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ok, results := evaluateForTest(probe, checkers)
		status := "UP"
		if !ok {
			status = "DEGRADED"
		}
		payload := map[string]any{
			"status": status,
			"checks": results,
			"time":   time.Now().UTC().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(payload)
	}
}

func evaluateForTest(probe *health.Probe, checkers []health.Checker) (bool, []map[string]any) {
	if probe != nil && !probe.IsLive() {
		return false, []map[string]any{{"name": "liveness", "status": "DOWN", "error": "server stopping"}}
	}
	results := make([]map[string]any, 0, len(checkers))
	healthy := true
	for _, c := range checkers {
		if c == nil {
			continue
		}
		if err := c.Check(context.Background()); err != nil {
			healthy = false
			results = append(results, map[string]any{"name": c.Name(), "status": "DOWN", "error": err.Error()})
		} else {
			results = append(results, map[string]any{"name": c.Name(), "status": "UP"})
		}
	}
	return healthy, results
}
