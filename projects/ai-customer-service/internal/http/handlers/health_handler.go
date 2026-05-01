package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/bridge/ai-customer-service/internal/platform/health"
)

type HealthHandler struct {
	probe    *health.Probe
	checkers []health.Checker
	now      func() time.Time
}

func NewHealthHandler(probe *health.Probe, checkers ...health.Checker) *HealthHandler {
	return &HealthHandler{probe: probe, checkers: checkers, now: time.Now}
}

func (h *HealthHandler) Live(w http.ResponseWriter, _ *http.Request) {
	status := http.StatusOK
	payload := map[string]any{"status": "UP"}
	if h.probe != nil && !h.probe.IsLive() {
		status = http.StatusServiceUnavailable
		payload["status"] = "DOWN"
	}
	writeJSON(w, status, payload)
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ok, checks := h.evaluate(r.Context())
	if h.probe != nil {
		h.probe.SetReady(ok)
	}
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "DOWN", "checks": checks})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "UP", "checks": checks})
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ok, checks := h.evaluate(r.Context())
	status := "UP"
	if !ok {
		status = "DEGRADED"
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": status, "checks": checks, "time": h.now().UTC().Format(time.RFC3339)})
}

func (h *HealthHandler) evaluate(ctx context.Context) (bool, []health.CheckResult) {
	if h.probe != nil && !h.probe.IsLive() {
		return false, []health.CheckResult{{Name: "liveness", Status: "DOWN", Error: "server stopping"}}
	}
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return health.Evaluate(checkCtx, h.checkers)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
