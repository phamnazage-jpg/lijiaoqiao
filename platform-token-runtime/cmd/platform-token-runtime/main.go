package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"lijiaoqiao/platform-token-runtime/internal/app"
	"lijiaoqiao/platform-token-runtime/internal/pkg/logging"
)

func main() {
	logger := logging.NewLogger("platform-token-runtime", logging.LogLevelInfo)

	cfg := app.Config{
		Addr: envOrDefault("TOKEN_RUNTIME_ADDR", ":18081"),
		Env:  strings.ToLower(envOrDefault("TOKEN_RUNTIME_ENV", "dev")),
		Now:  time.Now,
	}

	if databaseURL := strings.TrimSpace(os.Getenv("TOKEN_RUNTIME_DATABASE_URL")); databaseURL != "" {
		runtimeStore, auditStore, closeFn, err := app.BuildPostgresStores(context.Background(), databaseURL)
		if err != nil {
			logger.Fatalf("platform-token-runtime postgres bootstrap failed: %v", err)
		}
		cfg.RuntimeStore = runtimeStore
		cfg.AuditStore = auditStore
		defer closeFn()
	}

	srv, err := app.BuildServer(cfg)
	if err != nil {
		logger.Fatalf("platform-token-runtime bootstrap failed: %v", err)
	}

	serverErrCh := make(chan error, 1)

	go func() {
		logger.Infof("platform-token-runtime listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigCh:
		logger.Infof("received signal %s", sig)
	case err := <-serverErrCh:
		logger.Fatalf("listen failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	logger.Info("shutting down...")
	if err := srv.Shutdown(ctx); err != nil {
		logger.Warnf("graceful shutdown failed: %v", err)
	}
	logger.Info("shutdown complete")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
