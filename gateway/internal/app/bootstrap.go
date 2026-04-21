package app

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"lijiaoqiao/gateway/internal/config"
	"lijiaoqiao/gateway/internal/handler"
	gwmetrics "lijiaoqiao/gateway/internal/metrics"
	"lijiaoqiao/gateway/internal/middleware"
	"lijiaoqiao/gateway/internal/ratelimit"
	"lijiaoqiao/gateway/internal/router"
)

// ServerBundle 服务器启动Bundle
// P3-B-06: 包含 router 引用以便管理健康检查器生命周期
type ServerBundle struct {
	Server       *http.Server
	Router       *router.Router
	ShutdownFunc func()
}

// BuildServer 创建服务器Bundle
func BuildServer(cfg *config.Config) (*ServerBundle, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	normalized := normalizeConfig(*cfg)

	if err := config.ValidateAuthConfig(normalized.Auth); err != nil {
		return nil, err
	}
	if err := validateStartupSecurity(normalized); err != nil {
		return nil, err
	}

	r, err := buildRouter(&normalized)
	if err != nil {
		return nil, err
	}

	limiter := buildLimiter(normalized.RateLimit)
	auditor, err := buildAuditor(normalized)
	if err != nil {
		return nil, err
	}

	tokenRuntime, err := buildTokenRuntime(normalized.Auth)
	if err != nil {
		return nil, err
	}

	authConfig := middleware.AuthMiddlewareConfig{
		Verifier:       tokenRuntime,
		StatusResolver: tokenRuntime,
		Authorizer:     middleware.NewScopeRoleAuthorizer(),
		Auditor:        auditor,
		ProtectedPrefixes: []string{
			"/v1/chat/completions",
			"/v1/completions",
			"/api/v1/chat/completions",
			"/api/v1/completions",
			"/api/v1/supply",
			"/api/v1/platform",
		},
		ExcludedPrefixes: []string{"/health", "/healthz", "/metrics", "/readyz"},
		Now:              time.Now,
		TrustedProxies:   normalized.Auth.TrustedProxies,
	}

	handler := handler.NewHandler(r)
	corsConfig := buildCORSConfig(normalized)
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", normalized.Server.Host, normalized.Server.Port),
		Handler:      BuildMux(handler, limiter, authConfig, corsConfig),
		ReadTimeout:  normalized.Server.ReadTimeout,
		WriteTimeout: normalized.Server.WriteTimeout,
		IdleTimeout:  normalized.Server.IdleTimeout,
	}

	// P3-B-06: 启动后台健康检查循环
	r.StartHealthChecker(normalized.Router.HealthCheckInterval)

	bundle := &ServerBundle{
		Server:       server,
		Router:       r,
		ShutdownFunc: func() { r.StopHealthChecker() },
	}

	return bundle, nil
}

func BuildMux(h *handler.Handler, limiter *ratelimit.Middleware, authConfig middleware.AuthMiddlewareConfig, corsConfig middleware.CORSConfig) http.Handler {
	mux := http.NewServeMux()

	chatHandler := middleware.BuildTokenAuthChain(authConfig, http.HandlerFunc(h.ChatCompletionsHandle))
	completionsHandler := middleware.BuildTokenAuthChain(authConfig, http.HandlerFunc(h.CompletionsHandle))

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		limitHandler(limiter, chatHandler).ServeHTTP(w, r)
	})

	mux.HandleFunc("/v1/completions", func(w http.ResponseWriter, r *http.Request) {
		limitHandler(limiter, completionsHandler).ServeHTTP(w, r)
	})

	mux.HandleFunc("/v1/models", h.ModelsHandle)

	mux.HandleFunc("/api/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		limitHandler(limiter, chatHandler).ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/v1/completions", func(w http.ResponseWriter, r *http.Request) {
		limitHandler(limiter, completionsHandler).ServeHTTP(w, r)
	})

	mux.HandleFunc("/health", h.HealthHandle)
	mux.HandleFunc("/healthz", h.HealthHandle)
	mux.HandleFunc("/readyz", h.HealthHandle)
	// P3-C: /metrics 端点（Prometheus-text 格式）
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(gwmetrics.Export()))
	})

	return middleware.CORSMiddleware(corsConfig)(mux)
}

func buildRouter(cfg *config.Config) (*router.Router, error) {
	if len(cfg.Providers) == 0 {
		return nil, fmt.Errorf("at least one provider must be configured")
	}

	r := router.NewRouter(resolveStrategy(cfg.Router.Strategy))
	for _, providerCfg := range cfg.Providers {
		provider, err := buildProvider(providerCfg)
		if err != nil {
			return nil, err
		}
		r.RegisterProvider(providerCfg.Name, provider)
	}

	return r, nil
}

func buildLimiter(cfg config.RateLimitConfig) *ratelimit.Middleware {
	if strings.EqualFold(cfg.Algorithm, "sliding_window") {
		limiter := ratelimit.NewSlidingWindowLimiter(time.Minute, cfg.DefaultRPM)
		return ratelimit.NewMiddleware(limiter)
	}

	limiter := ratelimit.NewTokenBucketLimiter(cfg.DefaultRPM, cfg.DefaultTPM, cfg.BurstMultiplier)
	return ratelimit.NewMiddleware(limiter)
}

func buildAuditor(cfg config.Config) (middleware.AuditEmitter, error) {
	if strings.TrimSpace(cfg.Database.Host) == "" {
		return middleware.NewMemoryAuditEmitter(), nil
	}

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.Database.User,
		cfg.Database.GetPassword(),
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Database,
	)

	auditor, err := middleware.NewDatabaseAuditEmitter(dsn, time.Now)
	if err != nil {
		return nil, fmt.Errorf("create database audit emitter: %w", err)
	}

	return auditor, nil
}

func buildTokenRuntime(cfg config.AuthConfig) (interface {
	middleware.TokenVerifier
	middleware.TokenStatusResolver
}, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.TokenRuntimeMode)) {
	case "", "inmemory":
		return middleware.NewInMemoryTokenRuntime(time.Now), nil
	case "remote_introspection":
		// P3-A: 使用硬化 HTTP client 替代 http.DefaultClient
		httpClient := buildTimeoutClient(cfg.TokenRuntime)
		return middleware.NewRemoteTokenRuntime(cfg.TokenRuntimeURL, httpClient, time.Now), nil
	default:
		return nil, fmt.Errorf("unsupported token runtime mode: %s", cfg.TokenRuntimeMode)
	}
}

// buildTimeoutClient 根据硬化配置构建专属 HTTP client
// P3-A: 确保 remote introspection 调用有上限，避免 gateway 被 token-runtime 拖挂
func buildTimeoutClient(cfg config.HTTPTimeoutConfig) *http.Client {
	dialer := &net.Dialer{
		Timeout: cfg.DialTimeout,
	}

	transport := &http.Transport{
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext:         dialer.DialContext,
		IdleConnTimeout:     cfg.IdleConnTimeout,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		MaxIdleConns:        cfg.MaxIdleConnsPerHost * 2,
		ForceAttemptHTTP2:   true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   cfg.TotalTimeout,
	}
}

// resolveStrategy 只暴露当前主启动链路已验证的策略。
// cost_based、cost_aware 与 fallback 仍停留在实验模块，未接入 BuildServer。
func resolveStrategy(strategy string) router.LoadBalancerStrategy {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case string(router.StrategyRoundRobin):
		return router.StrategyRoundRobin
	case string(router.StrategyWeighted):
		return router.StrategyWeighted
	case string(router.StrategyAvailability):
		return router.StrategyAvailability
	default:
		return router.StrategyLatency
	}
}

func normalizeConfig(cfg config.Config) config.Config {
	if strings.TrimSpace(cfg.Server.Host) == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 30 * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 30 * time.Second
	}
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = 120 * time.Second
	}
	if cfg.Router.Strategy == "" {
		cfg.Router.Strategy = string(router.StrategyLatency)
	}
	if cfg.RateLimit.DefaultRPM == 0 {
		cfg.RateLimit.DefaultRPM = 60
	}
	if cfg.RateLimit.DefaultTPM == 0 {
		cfg.RateLimit.DefaultTPM = 60000
	}
	if cfg.RateLimit.BurstMultiplier == 0 {
		cfg.RateLimit.BurstMultiplier = 1.5
	}
	if cfg.RateLimit.Algorithm == "" {
		cfg.RateLimit.Algorithm = "token_bucket"
	}
	if cfg.Auth.TokenRuntimeMode == "" {
		cfg.Auth.TokenRuntimeMode = "inmemory"
	}
	// TrustedProxies from env: comma-separated list of trusted proxy IPs
	if len(cfg.Auth.TrustedProxies) == 0 {
		trustedProxiesEnv := strings.TrimSpace(os.Getenv("GATEWAY_TRUSTED_PROXIES"))
		if trustedProxiesEnv != "" {
			for _, ip := range strings.Split(trustedProxiesEnv, ",") {
				ip = strings.TrimSpace(ip)
				if ip != "" {
					cfg.Auth.TrustedProxies = append(cfg.Auth.TrustedProxies, ip)
				}
			}
		}
	}
	// CORSAllowOrigins from env: comma-separated list of allowed origins
	if len(cfg.Auth.CORSAllowOrigins) == 0 {
		corsEnv := strings.TrimSpace(os.Getenv("GATEWAY_CORS_ALLOW_ORIGINS"))
		if corsEnv != "" {
			for _, origin := range strings.Split(corsEnv, ",") {
				origin = strings.TrimSpace(origin)
				if origin != "" {
					cfg.Auth.CORSAllowOrigins = append(cfg.Auth.CORSAllowOrigins, origin)
				}
			}
		}
	}
	return cfg
}

func buildCORSConfig(cfg config.Config) middleware.CORSConfig {
	corsOrigins := cfg.Auth.CORSAllowOrigins
	if len(corsOrigins) == 0 {
		corsOrigins = []string{"*"}
	}
	return middleware.CORSConfig{
		AllowOrigins:     corsOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-Request-ID", "X-Request-Key"},
		ExposeHeaders:    []string{"X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           86400,
	}
}

func validateStartupSecurity(cfg config.Config) error {
	if !isProductionEnv(cfg.Auth.Env) {
		return nil
	}
	if isDefaultEncryptionKey() {
		return fmt.Errorf("PASSWORD_ENCRYPTION_KEY must be explicitly set in production environment")
	}
	if usesWildcardCORS(cfg.Auth.CORSAllowOrigins) {
		return fmt.Errorf("CORS_ALLOW_ORIGINS must be explicitly set in production environment")
	}
	return nil
}

func isProductionEnv(env string) bool {
	// 共享环境别名归一化只允许在 config.NormalizeEnv 一处定义，
	// 启动安全校验只消费归一化后的 prod 结果，避免多处规则漂移。
	return config.NormalizeEnv(env) == "prod"
}

func isDefaultEncryptionKey() bool {
	envKey := strings.TrimSpace(os.Getenv("PASSWORD_ENCRYPTION_KEY"))
	return envKey == "" || envKey == configDefaultEncryptionKey()
}

func configDefaultEncryptionKey() string {
	return "default-key-32-bytes-long!!!!!!!"
}

func usesWildcardCORS(origins []string) bool {
	if len(origins) == 0 {
		return true
	}
	return len(origins) == 1 && strings.TrimSpace(origins[0]) == "*"
}

func limitHandler(limiter *ratelimit.Middleware, next http.Handler) http.Handler {
	if limiter == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limiter.Limit(next.ServeHTTP)(w, r)
	})
}
