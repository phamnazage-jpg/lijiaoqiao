package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lijiaoqiao/platform-token-runtime/internal/auth/service"
)

func TestBuildRuntime_ProdRequiresConcreteStore(t *testing.T) {
	_, _, err := BuildRuntime(Config{Env: "prod"})
	if err == nil {
		t.Fatal("expected missing store error")
	}
	if !strings.Contains(err.Error(), "runtime store") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRuntime_DevUsesInMemoryDefaults(t *testing.T) {
	runtime, auditStore, err := BuildRuntime(Config{Env: "dev"})
	if err != nil {
		t.Fatalf("BuildRuntime returned error: %v", err)
	}
	if runtime == nil {
		t.Fatal("expected runtime")
	}
	if auditStore == nil {
		t.Fatal("expected audit store")
	}
}

func TestBuildRuntime_ProdAcceptsStoreContracts(t *testing.T) {
	runtimeStore := service.RuntimeStore(service.NewInMemoryRuntimeStore())
	auditStore := service.AuditStore(service.NewMemoryAuditStore())

	runtime, auditor, err := BuildRuntime(Config{
		Env:          "prod",
		RuntimeStore: runtimeStore,
		AuditStore:   auditStore,
	})
	if err != nil {
		t.Fatalf("BuildRuntime returned error: %v", err)
	}
	if runtime == nil {
		t.Fatal("expected runtime")
	}
	if auditor == nil {
		t.Fatal("expected audit store")
	}
}

func TestBuildServer_HealthEndpoint(t *testing.T) {
	srv, err := BuildServer(Config{
		Env:  "dev",
		Addr: "127.0.0.1:18081",
		Now: func() time.Time {
			return time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("BuildServer returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/actuator/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"status":"UP"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}
