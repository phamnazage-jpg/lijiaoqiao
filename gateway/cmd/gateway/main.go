package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lijiaoqiao/gateway/internal/adapter"
	"lijiaoqiao/gateway/internal/config"
	"lijiaoqiao/gateway/internal/handler"
	"lijiaoqiao/gateway/internal/middleware"
	"lijiaoqiao/gateway/internal/ratelimit"
	"lijiaoqiao/gateway/internal/router"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化Router
	r := router.NewRouter(router.StrategyLatency)

	// 注册Provider (示例: OpenAI)
	openaiAdapter := adapter.NewOpenAIAdapter(
		"https://api.openai.com",
		os.Getenv("OPENAI_API_KEY"),
		[]string{"gpt-4", "gpt-3.5-turbo"},
	)
	r.RegisterProvider("openai", openaiAdapter)

	// 初始化限流中间件
	var limiterMiddleware *ratelimit.Middleware
	if cfg.RateLimit.Algorithm == "token_bucket" {
		limiter := ratelimit.NewTokenBucketLimiter(
			cfg.RateLimit.DefaultRPM,
			cfg.RateLimit.DefaultTPM,
			cfg.RateLimit.BurstMultiplier,
		)
		limiterMiddleware = ratelimit.NewMiddleware(limiter)
	} else {
		limiter := ratelimit.NewSlidingWindowLimiter(
			time.Minute,
			cfg.RateLimit.DefaultRPM,
		)
		limiterMiddleware = ratelimit.NewMiddleware(limiter)
	}

	// 初始化审计发射器
	var auditor middleware.AuditEmitter
	if cfg.Database.Host != "" {
		// MED-10: 使用 GetPassword() 获取解密后的密码，避免在日志中暴露明文密码
		dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
			cfg.Database.User,
			cfg.Database.GetPassword(),
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.Database,
		)
		auditEmitter, err := middleware.NewDatabaseAuditEmitter(dsn, time.Now)
		if err != nil {
			log.Printf("Warning: Failed to create database audit emitter: %v, using memory emitter", err)
			auditor = middleware.NewMemoryAuditEmitter()
		} else {
			auditor = auditEmitter
			defer auditEmitter.Close()
		}
	} else {
		log.Printf("Warning: Database not configured, using memory audit emitter")
		auditor = middleware.NewMemoryAuditEmitter()
	}

	// 初始化 token 运行时（内存实现）
	tokenRuntime := middleware.NewInMemoryTokenRuntime(time.Now)

	// 构建认证中间件配置
	authMiddlewareConfig := middleware.AuthMiddlewareConfig{
		Verifier:          tokenRuntime,
		StatusResolver:    tokenRuntime,
		Authorizer:        middleware.NewScopeRoleAuthorizer(),
		Auditor:           auditor,
		ProtectedPrefixes: []string{"/api/v1/supply", "/api/v1/platform"},
		ExcludedPrefixes:  []string{"/health", "/healthz", "/metrics", "/readyz"},
		Now:               time.Now,
	}

	// 初始化Handler
	h := handler.NewHandler(r)

	// 创建Server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      createMux(h, limiterMiddleware, authMiddlewareConfig),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// 启动Server
	go func() {
		log.Printf("Starting gateway server on %s:%d", cfg.Server.Host, cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func createMux(h *handler.Handler, limiter *ratelimit.Middleware, authConfig middleware.AuthMiddlewareConfig) http.Handler {
	mux := http.NewServeMux()

	// 创建认证处理链
	authHandler := middleware.BuildTokenAuthChain(authConfig, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ChatCompletionsHandle(w, r)
	}))

	// Chat Completions - 应用限流和认证
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		limiter.Limit(authHandler.ServeHTTP)(w, r)
	})

	// Completions - 应用限流和认证
	mux.HandleFunc("/v1/completions", func(w http.ResponseWriter, r *http.Request) {
		limiter.Limit(authHandler.ServeHTTP)(w, r)
	})

	// Models - 公开接口
	mux.HandleFunc("/v1/models", h.ModelsHandle)

	// 旧版路径兼容
	mux.HandleFunc("/api/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		h.ChatCompletionsHandle(w, r)
	})

	// Health - 排除认证
	mux.HandleFunc("/health", h.HealthHandle)
	mux.HandleFunc("/healthz", h.HealthHandle)
	mux.HandleFunc("/readyz", h.HealthHandle)

	return mux
}
