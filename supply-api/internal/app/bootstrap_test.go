package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/adapter"
	"lijiaoqiao/supply-api/internal/audit"
	auditservice "lijiaoqiao/supply-api/internal/audit/service"
	"lijiaoqiao/supply-api/internal/config"
	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/httpapi"
	"lijiaoqiao/supply-api/internal/middleware"
)

type testLogger struct{}

func (testLogger) Debug(string, ...map[string]interface{}) {}
func (testLogger) Info(string, ...map[string]interface{})  {}
func (testLogger) Warn(string, ...map[string]interface{})  {}
func (testLogger) Error(string, ...map[string]interface{}) {}
func (testLogger) Fatal(string, ...map[string]interface{}) {}

func TestBuildServer_ReturnsErrorWhenSupplyAPIMissing(t *testing.T) {
	_, alertAPI := mustBuildTestAPIs(t)

	srv, err := BuildServer(BuildServerOptions{
		Env:      "dev",
		Logger:   testLogger{},
		AlertAPI: alertAPI,
	})
	if err == nil {
		t.Fatal("expected missing supply api to fail")
	}
	if srv != nil {
		t.Fatal("expected nil server when bootstrap options are invalid")
	}
	if !strings.Contains(err.Error(), "supply api") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildServer_ProdRequiresAuthMiddleware(t *testing.T) {
	supplyAPI, alertAPI := mustBuildTestAPIs(t)

	srv, err := BuildServer(BuildServerOptions{
		Env:       "prod",
		Logger:    testLogger{},
		SupplyAPI: supplyAPI,
		AlertAPI:  alertAPI,
	})
	if err == nil {
		t.Fatal("expected prod bootstrap to require auth middleware")
	}
	if srv != nil {
		t.Fatal("expected nil server when auth middleware is missing")
	}
	if !strings.Contains(err.Error(), "auth middleware") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildServer_RejectsUnsupportedEnv(t *testing.T) {
	supplyAPI, alertAPI := mustBuildTestAPIs(t)

	srv, err := BuildServer(BuildServerOptions{
		Env:       "qa",
		Logger:    testLogger{},
		SupplyAPI: supplyAPI,
		AlertAPI:  alertAPI,
	})
	if err == nil {
		t.Fatal("expected unsupported env to fail")
	}
	if srv != nil {
		t.Fatal("expected nil server when env is unsupported")
	}
	if !strings.Contains(err.Error(), "unsupported env") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildServer_RegistersHealthRoute(t *testing.T) {
	supplyAPI, alertAPI := mustBuildTestAPIs(t)

	srv, err := BuildServer(BuildServerOptions{
		Env:       "dev",
		Logger:    testLogger{},
		SupplyAPI: supplyAPI,
		AlertAPI:  alertAPI,
		ServerConfig: config.ServerConfig{
			Addr:            "127.0.0.1:18082",
			ReadTimeout:     2 * time.Second,
			WriteTimeout:    3 * time.Second,
			IdleTimeout:     4 * time.Second,
			ShutdownTimeout: 5 * time.Second,
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
	if !strings.Contains(rec.Body.String(), `"healthy"`) {
		t.Fatalf("unexpected health body: %s", rec.Body.String())
	}
}

func TestBuildServer_DefaultsTimeoutsWhenUnset(t *testing.T) {
	supplyAPI, alertAPI := mustBuildTestAPIs(t)

	srv, err := BuildServer(BuildServerOptions{
		Env:       "dev",
		Logger:    testLogger{},
		SupplyAPI: supplyAPI,
		AlertAPI:  alertAPI,
	})
	if err != nil {
		t.Fatalf("BuildServer returned error: %v", err)
	}
	if srv.Addr != ":18082" {
		t.Fatalf("unexpected addr: %s", srv.Addr)
	}
	if srv.ReadHeaderTimeout != 10*time.Second {
		t.Fatalf("unexpected read header timeout: %s", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != 10*time.Second {
		t.Fatalf("unexpected read timeout: %s", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 15*time.Second {
		t.Fatalf("unexpected write timeout: %s", srv.WriteTimeout)
	}
	if srv.IdleTimeout != 30*time.Second {
		t.Fatalf("unexpected idle timeout: %s", srv.IdleTimeout)
	}
}

func mustBuildTestAPIs(t *testing.T) (*httpapi.SupplyAPI, *httpapi.AlertAPI) {
	t.Helper()

	auditStore := audit.NewMemoryAuditStore()
	accountStore := adapter.NewInMemoryAccountStoreAdapter()
	packageStore := adapter.NewInMemoryPackageStoreAdapter()
	settlementStore := adapter.NewInMemorySettlementStoreAdapter()
	earningStore := adapter.NewInMemoryEarningStoreAdapter()

	supplyAPI, err := httpapi.NewSupplyAPI(
		domain.NewAccountService(accountStore, auditStore),
		domain.NewPackageService(packageStore, accountStore, auditStore),
		domain.NewSettlementService(settlementStore, earningStore, auditStore),
		domain.NewEarningService(earningStore),
		nil,
		auditStore,
		nil,
		1,
		"https://statements.example.com",
		func() time.Time { return time.Unix(1712800000, 0).UTC() },
	)
	if err != nil {
		t.Fatalf("expected supply api constructor to succeed: %v", err)
	}

	alertAPI, err := httpapi.NewAlertAPI(auditservice.NewAlertService(auditservice.NewInMemoryAlertStore()))
	if err != nil {
		t.Fatalf("expected alert api constructor to succeed: %v", err)
	}

	return supplyAPI, alertAPI
}

func TestBuildServer_ProdBuildsAuthenticatedHandler(t *testing.T) {
	supplyAPI, alertAPI := mustBuildTestAPIs(t)
	authMiddleware := middleware.NewAuthMiddleware(
		middleware.AuthConfig{
			SecretKey: "bootstrap-test-secret-key",
			Algorithm: "HS256",
			Issuer:    "bootstrap-test",
			Enabled:   true,
		},
		middleware.NewTokenCache(),
		nil,
		nil,
	)

	srv, err := BuildServer(BuildServerOptions{
		Env:            "prod",
		Logger:         testLogger{},
		SupplyAPI:      supplyAPI,
		AlertAPI:       alertAPI,
		AuthMiddleware: authMiddleware,
	})
	if err != nil {
		t.Fatalf("BuildServer returned error: %v", err)
	}
	if srv == nil || srv.Handler == nil {
		t.Fatal("expected non-nil server and handler")
	}
}
