package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ==================== RateLimit Helper Function Tests ====================

func TestGetTenantIDFromRequest(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	tenantID := getTenantIDFromRequest(req)

	// Simplified implementation returns 0
	if tenantID != 0 {
		t.Errorf("expected tenantID 0, got %d", tenantID)
	}
}

func TestGetUserIDFromRequest(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	userID := getUserIDFromRequest(req)

	// Simplified implementation returns "unknown"
	if userID != "unknown" {
		t.Errorf("expected userID 'unknown', got '%s'", userID)
	}
}

func TestNewRateLimitHandler(t *testing.T) {
	config := &RateLimitConfig{
		Enabled:  true,
		Requests: 100,
		Window:   time.Minute,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	handler := NewRateLimitHandler(config, nextHandler)

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.config != config {
		t.Error("expected config to be set")
	}
	if handler.buckets == nil {
		t.Error("expected buckets to be initialized")
	}
}
