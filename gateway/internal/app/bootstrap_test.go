package app

import (
	"net/http"
	"testing"

	"lijiaoqiao/gateway/internal/config"
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

	server, err := BuildServer(cfg)
	if err != nil {
		t.Fatalf("BuildServer returned error: %v", err)
	}
	if server == nil {
		t.Fatal("expected server")
	}
	if server.Addr != "0.0.0.0:8080" {
		t.Fatalf("unexpected addr: %s", server.Addr)
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

	server, err := BuildServer(cfg)
	if err != nil {
		t.Fatalf("BuildServer returned error: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	rr := newTestResponseRecorder()
	server.Handler.ServeHTTP(rr, req)
	if rr.code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.code)
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
