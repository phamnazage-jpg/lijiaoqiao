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

	"lijiaoqiao/platform-token-runtime/internal/app"
)

func main() {
	cfg := app.Config{
		Addr: envOrDefault("TOKEN_RUNTIME_ADDR", ":18081"),
		Env:  strings.ToLower(envOrDefault("TOKEN_RUNTIME_ENV", "dev")),
		Now:  time.Now,
	}

	if databaseURL := strings.TrimSpace(os.Getenv("TOKEN_RUNTIME_DATABASE_URL")); databaseURL != "" {
		runtimeStore, auditStore, closeFn, err := app.BuildPostgresStores(context.Background(), databaseURL)
		if err != nil {
			log.Fatalf("platform-token-runtime postgres bootstrap failed: %v", err)
		}
		cfg.RuntimeStore = runtimeStore
		cfg.AuditStore = auditStore
		defer closeFn()
	}

	srv, err := app.BuildServer(cfg)
	if err != nil {
		log.Fatalf("platform-token-runtime bootstrap failed: %v", err)
	}

	go func() {
		log.Printf("platform-token-runtime listening on %s", srv.Addr)
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
