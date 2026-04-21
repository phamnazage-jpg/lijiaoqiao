package app

import (
	"net/http"
	"strings"
	"testing"

	"lijiaoqiao/gateway/internal/config"
	"lijiaoqiao/gateway/internal/router"
)

func TestBuildServer_FromConfigProviders(t *testing.T) {
	cfg := &config.Config{
		Providers: []config.ProviderConfig{{
			Name:    "openai",
			Type:    "openai",
			BaseURL: "https://api.openai.com",
			APIKey:  "secret",
			Models:  []string{"gpt-4o"},
		}},
	}

	bundle, err := BuildServer(cfg)
	if err != nil {
		t.Fatalf("BuildServer returned error: %v", err)
	}
	if bundle == nil {
		t.Fatal("expected server bundle")
	}
	if bundle.Server.Addr != "0.0.0.0:8080" {
		t.Fatalf("unexpected addr: %s", bundle.Server.Addr)
	}
}

func TestBuildServer_RejectsEmptyProviderList(t *testing.T) {
	_, err := BuildServer(&config.Config{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildMux_HealthRouteRemainsOpen(t *testing.T) {
	cfg := &config.Config{
		Providers: []config.ProviderConfig{{
			Name:    "openai",
			Type:    "openai",
			BaseURL: "https://api.openai.com",
			APIKey:  "secret",
			Models:  []string{"gpt-4o"},
		}},
	}

	bundle, err := BuildServer(cfg)
	if err != nil {
		t.Fatalf("BuildServer returned error: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	rr := newTestResponseRecorder()
	bundle.Server.Handler.ServeHTTP(rr, req)
	if rr.code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.code)
	}
}

func TestBuildServer_ProductionRejectsDefaultEncryptionKey(t *testing.T) {
	t.Setenv("PASSWORD_ENCRYPTION_KEY", "")

	_, err := buildServerWithoutPanic(t, newProductionServerConfig())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "PASSWORD_ENCRYPTION_KEY") {
		t.Fatalf("expected PASSWORD_ENCRYPTION_KEY error, got %v", err)
	}
}

func TestBuildServer_ProductionRejectsWildcardCORS(t *testing.T) {
	t.Setenv("PASSWORD_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")

	cfg := newProductionServerConfig()
	cfg.Auth.CORSAllowOrigins = []string{"*"}

	_, err := buildServerWithoutPanic(t, cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "CORS_ALLOW_ORIGINS") {
		t.Fatalf("expected CORS_ALLOW_ORIGINS error, got %v", err)
	}
}

func TestBuildServer_DevelopmentAllowsDefaultSecurityFallbacks(t *testing.T) {
	t.Setenv("PASSWORD_ENCRYPTION_KEY", "")

	cfg := &config.Config{
		Auth: config.AuthConfig{
			Env:              "dev",
			TokenRuntimeMode: "inmemory",
		},
		Providers: []config.ProviderConfig{{
			Name:    "openai",
			Type:    "openai",
			BaseURL: "https://api.openai.com",
			APIKey:  "secret",
			Models:  []string{"gpt-4o"},
		}},
	}

	bundle, err := buildServerWithoutPanic(t, cfg)
	if err != nil {
		t.Fatalf("expected dev config to succeed, got %v", err)
	}
	if bundle == nil {
		t.Fatal("expected server bundle")
	}
}

func TestResolveStrategy_ExperimentalStrategiesFallbackToLatency(t *testing.T) {
	tests := []string{"cost_based", "cost_aware", "fallback"}
	for _, strategy := range tests {
		if got := resolveStrategy(strategy); got != router.StrategyLatency {
			t.Fatalf("strategy %s should fallback to latency, got %s", strategy, got)
		}
	}
}

func buildServerWithoutPanic(t *testing.T, cfg *config.Config) (_ *ServerBundle, err error) {
	t.Helper()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("BuildServer panicked: %v", recovered)
		}
	}()

	return BuildServer(cfg)
}

func newProductionServerConfig() *config.Config {
	return &config.Config{
		Auth: config.AuthConfig{
			Env:              "production",
			TokenRuntimeMode: "remote_introspection",
			TokenRuntimeURL:  "http://127.0.0.1:18081",
		},
		Providers: []config.ProviderConfig{{
			Name:    "openai",
			Type:    "openai",
			BaseURL: "https://api.openai.com",
			APIKey:  "secret",
			Models:  []string{"gpt-4o"},
		}},
	}
}

type testResponseRecorder struct {
	header http.Header
	code   int
}

func newTestResponseRecorder() *testResponseRecorder {
	return &testResponseRecorder{header: make(http.Header)}
}

func (r *testResponseRecorder) Header() http.Header {
	return r.header
}

func (r *testResponseRecorder) WriteHeader(statusCode int) {
	r.code = statusCode
}

func (r *testResponseRecorder) Write(b []byte) (int, error) {
	if r.code == 0 {
		r.code = http.StatusOK
	}
	return len(b), nil
}
