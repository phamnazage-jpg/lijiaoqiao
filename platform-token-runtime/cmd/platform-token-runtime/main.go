package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"lijiaoqiao/platform-token-runtime/internal/auth/service"
	"lijiaoqiao/platform-token-runtime/internal/httpapi"
)

func main() {
	addr := envOrDefault("TOKEN_RUNTIME_ADDR", ":18081")
	env := strings.ToLower(envOrDefault("TOKEN_RUNTIME_ENV", "dev"))
	if env == "prod" || env == "staging" {
		log.Fatalf("in-memory token runtime is not allowed in %s", env)
	}

	runtime := service.NewInMemoryTokenRuntime(nil)
	auditor := service.NewMemoryAuditEmitter()
	api := httpapi.NewTokenAPI(runtime, auditor, time.Now)

	mux := http.NewServeMux()
	mux.HandleFunc("/actuator/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"UP"}`))
	})
	api.Register(mux)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	go func() {
		log.Printf("platform-token-runtime listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen failed: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
