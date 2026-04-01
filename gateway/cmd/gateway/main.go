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
	"lijiaoqiao/gateway/internal/alert"
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

	// 初始化限流器
	var limiter ratelimit.Limiter
	if cfg.RateLimit.Algorithm == "token_bucket" {
		limiter = ratelimit.NewTokenBucketLimiter(
			cfg.RateLimit.DefaultRPM,
			cfg.RateLimit.DefaultTPM,
			cfg.RateLimit.BurstMultiplier,
		)
	} else {
		limiter = ratelimit.NewSlidingWindowLimiter(
			time.Minute,
			cfg.RateLimit.DefaultRPM,
		)
	}

	// 初始化告警管理器
	alertManager, err := alert.NewManager(&cfg.Alert)
	if err != nil {
		log.Printf("Warning: Failed to create alert manager: %v", err)
	}

	// 初始化Handler
	h := handler.NewHandler(r)

	// 创建Server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      createMux(h, limiter, alertManager),
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

func createMux(h *handler.Handler, limiter *ratelimit.Middleware, alertMgr *alert.Manager) *http.ServeMux {
	mux := http.NewServeMux()

	// V1 API
	v1 := mux.PathPrefix("/v1").Subrouter()

	// Chat Completions (需要限流和认证)
	v1.HandleFunc("/chat/completions", withMiddleware(h.ChatCompletionsHandle,
		limiter.Limit,
		authMiddleware(),
	))

	// Completions
	v1.HandleFunc("/completions", withMiddleware(h.CompletionsHandle,
		limiter.Limit,
		authMiddleware(),
	))

	// Models
	v1.HandleFunc("/models", h.ModelsHandle)

	// Health
	mux.HandleFunc("/health", h.HealthHandle)

	return mux
}

// MiddlewareFunc 中间件函数类型
type MiddlewareFunc func(http.HandlerFunc) http.HandlerFunc

// withMiddleware 应用中间件
func withMiddleware(h http.HandlerFunc, limiters ...func(http.HandlerFunc) http.HandlerFunc) http.HandlerFunc {
	for _, m := range limiters {
		h = m(h)
	}
	return h
}

// authMiddleware 认证中间件(简化实现)
func authMiddleware() MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// 简化: 检查Authorization头
			if r.Header.Get("Authorization") == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":{"message":"Missing Authorization header","code":"AUTH_001"}}`))
				return
			}
			next.ServeHTTP(w, r)
		}
	}
}
