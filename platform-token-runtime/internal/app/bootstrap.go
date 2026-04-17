package app

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"lijiaoqiao/platform-token-runtime/internal/auth/service"
	"lijiaoqiao/platform-token-runtime/internal/httpapi"
)

type Config struct {
	Addr         string
	Env          string
	RuntimeStore service.RuntimeStore
	AuditStore   service.AuditStore
	Now          func() time.Time
}

func BuildRuntime(cfg Config) (*service.InMemoryTokenRuntime, service.AuditStore, error) {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	env := strings.ToLower(strings.TrimSpace(cfg.Env))
	runtimeStore := cfg.RuntimeStore
	auditStore := cfg.AuditStore

	switch env {
	case "", "dev":
		if runtimeStore == nil {
			runtimeStore = service.NewInMemoryRuntimeStore()
		}
		if auditStore == nil {
			auditStore = service.NewMemoryAuditStore()
		}
	case "prod", "staging":
		if runtimeStore == nil {
			return nil, nil, fmt.Errorf("runtime store is required in %s", env)
		}
		if auditStore == nil {
			return nil, nil, fmt.Errorf("audit store is required in %s", env)
		}
	default:
		return nil, nil, fmt.Errorf("unsupported TOKEN_RUNTIME_ENV %q", cfg.Env)
	}

	return service.NewInMemoryTokenRuntimeWithStore(now, runtimeStore), auditStore, nil
}

func BuildServer(cfg Config) (*http.Server, error) {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		addr = ":18081"
	}

	runtime, auditor, err := BuildRuntime(cfg)
	if err != nil {
		return nil, err
	}

	api := httpapi.NewTokenAPI(runtime, auditor, now)
	mux := http.NewServeMux()
	mux.HandleFunc("/actuator/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"UP"}`))
	})
	api.Register(mux)

	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       30 * time.Second,
	}, nil
}
