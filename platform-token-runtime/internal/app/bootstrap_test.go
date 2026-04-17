package app

import (
	"context"
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

type stubRuntimeStore struct{}

func (stubRuntimeStore) Save(context.Context, service.TokenRecord, string, string) error {
	return nil
}

func (stubRuntimeStore) GetByTokenID(context.Context, string) (*service.TokenRecord, bool, error) {
	return nil, false, nil
}

func (stubRuntimeStore) GetByAccessToken(context.Context, string) (*service.TokenRecord, bool, error) {
	return nil, false, nil
}

func (stubRuntimeStore) LookupIdempotency(context.Context, string) (service.IdempotencyEntry, bool, error) {
	return service.IdempotencyEntry{}, false, nil
}

type stubAuditStore struct{}

func (stubAuditStore) Emit(context.Context, service.AuditEvent) error { return nil }

func (stubAuditStore) QueryEvents(context.Context, service.AuditEventFilter) ([]service.AuditEvent, error) {
	return nil, nil
}

func TestBuildPostgresStores_RequiresDatabaseURL(t *testing.T) {
	_, _, closeFn, err := BuildPostgresStores(context.Background(), "")
	if err == nil {
		t.Fatal("expected empty database url error")
	}
	if closeFn != nil {
		t.Fatal("expected nil close function on error")
	}
}

func TestBuildPostgresStores_UsesFactory(t *testing.T) {
	oldFactory := newPostgresStoreBundle
	defer func() { newPostgresStoreBundle = oldFactory }()

	closed := false
	newPostgresStoreBundle = func(ctx context.Context, databaseURL string) (service.RuntimeStore, service.AuditStore, func(), error) {
		if databaseURL != "postgres://token-runtime" {
			t.Fatalf("unexpected database url: %s", databaseURL)
		}
		return stubRuntimeStore{}, stubAuditStore{}, func() { closed = true }, nil
	}

	runtimeStore, auditStore, closeFn, err := BuildPostgresStores(context.Background(), "postgres://token-runtime")
	if err != nil {
		t.Fatalf("BuildPostgresStores returned error: %v", err)
	}
	if runtimeStore == nil {
		t.Fatal("expected runtime store")
	}
	if auditStore == nil {
		t.Fatal("expected audit store")
	}
	if closeFn == nil {
		t.Fatal("expected close function")
	}
	closeFn()
	if !closed {
		t.Fatal("expected close function to run")
	}
}
