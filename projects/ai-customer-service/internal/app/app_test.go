package app

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/config"
	"github.com/bridge/ai-customer-service/internal/platform/health"
	"github.com/bridge/ai-customer-service/internal/platform/logging"
)

// minimalHTTPConfig returns a config minimal enough to let New() succeed with in-memory stores.
func minimalHTTPConfig() *config.Config {
	cfg := &config.Config{}
	cfg.HTTP.Addr = ":0"
	cfg.HTTP.ReadHeaderTimeout = 5
	cfg.HTTP.ReadTimeout = 10
	cfg.HTTP.WriteTimeout = 15
	cfg.HTTP.IdleTimeout = 60
	cfg.HTTP.MaxHeaderBytes = 1 << 20
	cfg.HTTP.MaxBodyBytes = 1 << 20
	cfg.Postgres.Enabled = false
	cfg.Runtime.Env = "test"
	return cfg
}

func TestNew_NilConfig(t *testing.T) {
	_, err := New(nil, logging.New())
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if err.Error() != "config is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestNew_DefaultLogger(t *testing.T) {
	cfg := minimalHTTPConfig()
	cfg.Webhook.Secret = "test-secret"

	app, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() with nil logger failed: %v", err)
	}
	if app == nil {
		t.Fatal("expected non-nil app")
	}
	if app.Logger == nil {
		t.Error("expected non-nil logger (should default)")
	}
}

func TestNew_WithPostgresDisabled(t *testing.T) {
	cfg := minimalHTTPConfig()
	cfg.Webhook.Secret = "test-secret"

	app, err := New(cfg, logging.New())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if app.Server == nil {
		t.Fatal("expected non-nil server")
	}
	if app.Probe == nil {
		t.Fatal("expected non-nil probe")
	}
	if app.ticketStore == nil {
		t.Fatal("expected non-nil ticketStore")
	}
}

func TestNew_RejectsMemoryModeWithoutExplicitNonProdEnv(t *testing.T) {
	cfg := &config.Config{}
	cfg.HTTP.Addr = ":0"
	cfg.HTTP.ReadHeaderTimeout = 5
	cfg.HTTP.ReadTimeout = 10
	cfg.HTTP.WriteTimeout = 15
	cfg.HTTP.IdleTimeout = 60
	cfg.HTTP.MaxHeaderBytes = 1 << 20
	cfg.HTTP.MaxBodyBytes = 1 << 20
	cfg.Postgres.Enabled = false
	cfg.Webhook.Secret = "test-secret"

	_, err := New(cfg, logging.New())
	if err == nil {
		t.Fatal("expected error when runtime env is not explicitly non-prod for memory mode")
	}
}

func TestNew_AllowsMemoryModeInTestEnv(t *testing.T) {
	cfg := minimalHTTPConfig()
	cfg.Webhook.Secret = "test-secret"

	app, err := New(cfg, logging.New())
	if err != nil {
		t.Fatalf("New() failed in test env: %v", err)
	}
	if app == nil {
		t.Fatal("expected non-nil app")
	}
}

func TestNew_RegistersPlatformWebhookRouteWhenSub2APIEnabled(t *testing.T) {
	cfg := minimalHTTPConfig()
	cfg.Webhook.Secret = "test-secret"
	cfg.PlatformAdapters.Enabled = true
	cfg.PlatformAdapters.Sub2API.Enabled = true
	cfg.PlatformAdapters.Sub2API.IngressSecret = "sub2api-secret"

	app, err := New(cfg, logging.New())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/platforms/sub2api/webhook", nil)
	rr := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(rr, req)
	if rr.Code == http.StatusNotFound {
		t.Fatal("platform webhook route returned 404; route should be registered")
	}
}

func TestApp_TicketStore(t *testing.T) {
	cfg := minimalHTTPConfig()
	cfg.Webhook.Secret = "test-secret"

	app, err := New(cfg, logging.New())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	store := app.TicketStore()
	if store == nil {
		t.Fatal("TicketStore() returned nil")
	}

	_ = store
}

func TestApp_Shutdown_NilApp(t *testing.T) {
	var app *App
	err := app.Shutdown(nil)
	if err != nil {
		t.Fatalf("Shutdown on nil app should return nil error, got: %v", err)
	}
}

func TestApp_Shutdown_NilServer(t *testing.T) {
	app := &App{Server: nil, Probe: nil}
	if err := app.Shutdown(nil); err != nil {
		t.Fatalf("Shutdown on nil server should return nil error, got: %v", err)
	}
}

func TestApp_Shutdown_ServerShutdownCalled(t *testing.T) {
	t.Run("server is shut down and stops accepting connections", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		listener := ts.Listener
		ts.Close()

		app := &App{
			Server: &http.Server{
				Addr:    listener.Addr().String(),
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
			},
			Logger: logging.New(),
		}

		go func() { _ = app.Server.Serve(listener) }()
		time.Sleep(10 * time.Millisecond)

		err := app.Shutdown(context.Background())
		if err != nil {
			t.Fatalf("Shutdown returned unexpected error: %v", err)
		}

		conn, err := net.Dial("tcp", listener.Addr().String())
		if err == nil {
			conn.Close()
			t.Error("server should not be accepting connections after Shutdown")
		}
	})
}

func TestApp_Shutdown_CallsAllClosersInOrder(t *testing.T) {
	callOrder := []string{}

	firstCloser := func() error {
		callOrder = append(callOrder, "first")
		return nil
	}
	secondCloser := func() error {
		callOrder = append(callOrder, "second")
		return nil
	}
	thirdCloser := func() error {
		callOrder = append(callOrder, "third")
		return nil
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	listener := ts.Listener
	ts.Close()

	app := &App{
		Server: &http.Server{
			Addr:    listener.Addr().String(),
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		},
		Logger:  logging.New(),
		closers: []func() error{firstCloser, secondCloser, thirdCloser},
	}

	go func() { _ = app.Server.Serve(listener) }()
	time.Sleep(10 * time.Millisecond)

	_ = app.Shutdown(context.Background())

	if len(callOrder) != 3 {
		t.Fatalf("expected 3 closer calls, got %d: %v", len(callOrder), callOrder)
	}
	if callOrder[0] != "first" || callOrder[1] != "second" || callOrder[2] != "third" {
		t.Errorf("closers called in wrong order: %v", callOrder)
	}
}

func TestApp_Shutdown_ProbeSetNotReady(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	listener := ts.Listener
	ts.Close()

	probe := health.NewProbe()
	probe.SetReady(true)
	probe.SetLive(true)

	app := &App{
		Server: &http.Server{
			Addr:    listener.Addr().String(),
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		},
		Probe:  probe,
		Logger: logging.New(),
	}

	go func() { _ = app.Server.Serve(listener) }()
	time.Sleep(10 * time.Millisecond)

	_ = app.Shutdown(context.Background())

	if probe.IsReady() {
		t.Error("Probe should not be ready after Shutdown")
	}
	if probe.IsLive() {
		t.Error("Probe should not be live after Shutdown")
	}
}

func TestNew_WithPostgresEnabled_InvalidDSN(t *testing.T) {
	cfg := minimalHTTPConfig()
	cfg.Runtime.Env = "production"
	cfg.Webhook.Secret = "test-secret"
	cfg.Postgres.Enabled = true
	cfg.Postgres.DSN = "invalid_dsn_format"
	cfg.Postgres.MaxOpenConns = 5
	cfg.Postgres.MaxIdleConns = 2
	cfg.Postgres.ConnMaxLifetime = 300

	_, err := New(cfg, logging.New())
	if err == nil {
		t.Fatal("expected error when postgres enabled with invalid DSN")
	}
}

func TestNew_WithPostgresEnabled_MigrationFails(t *testing.T) {
	cfg := minimalHTTPConfig()
	cfg.Runtime.Env = "production"
	cfg.Webhook.Secret = "test-secret"
	cfg.Postgres.Enabled = true
	cfg.Postgres.DSN = "host=127.0.0.1 port=9999 user=postgres dbname=nonexistent password=nonexistent sslmode=disable"
	cfg.Postgres.MigrationDir = "/nonexistent/migration/dir"
	cfg.Postgres.MaxOpenConns = 5
	cfg.Postgres.MaxIdleConns = 2
	cfg.Postgres.ConnMaxLifetime = 300

	_, err := New(cfg, logging.New())
	if err == nil {
		t.Fatal("expected error when postgres migration directory does not exist")
	}
}
