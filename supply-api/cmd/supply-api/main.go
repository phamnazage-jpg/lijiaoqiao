package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lijiaoqiao/supply-api/internal/audit"
	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/httpapi"
	"lijiaoqiao/supply-api/internal/middleware"
	"lijiaoqiao/supply-api/internal/storage"
)

func main() {
	addr := envOrDefault("SUPPLY_API_ADDR", ":18082")
	supplierID := int64(1) // 默认供应商ID（开发阶段）

	// 初始化内存存储
	accountStore := storage.NewInMemoryAccountStore()
	packageStore := storage.NewInMemoryPackageStore()
	settlementStore := storage.NewInMemorySettlementStore()
	earningStore := storage.NewInMemoryEarningStore()
	idempotencyStore := storage.NewInMemoryIdempotencyStore()
	auditStore := audit.NewMemoryAuditStore()

	// 初始化领域服务
	accountService := domain.NewAccountService(accountStore, auditStore)
	packageService := domain.NewPackageService(packageStore, accountStore, auditStore)
	settlementService := domain.NewSettlementService(settlementStore, earningStore, auditStore)
	earningService := domain.NewEarningService(earningStore)

	// 初始化 HTTP API 处理器
	api := httpapi.NewSupplyAPI(
		accountService,
		packageService,
		settlementService,
		earningService,
		idempotencyStore,
		auditStore,
		supplierID,
		time.Now,
	)

	// 创建路由器
	mux := http.NewServeMux()

	// 健康检查
	mux.HandleFunc("/actuator/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"UP"}`))
	})

	// 注册 API 路由
	api.Register(mux)

	// 应用中间件
	handler := middleware.Logging(middleware.Recovery(middleware.RequestID(mux)))

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	go func() {
		log.Printf("supply-api listening on %s", addr)
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
