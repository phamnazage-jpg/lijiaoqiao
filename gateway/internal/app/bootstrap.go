package app

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"lijiaoqiao/gateway/internal/config"
	"lijiaoqiao/gateway/internal/handler"
	"lijiaoqiao/gateway/internal/middleware"
	"lijiaoqiao/gateway/internal/ratelimit"
	"lijiaoqiao/gateway/internal/router"
)

func BuildServer(cfg *config.Config) (*http.Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	normalized := normalizeConfig(*cfg)

	if err := config.ValidateAuthConfig(normalized.Auth); err != nil {
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

	return server, nil
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
		return middleware.NewRemoteTokenRuntime(cfg.TokenRuntimeURL, http.DefaultClient, time.Now), nil
	default:
		return nil, fmt.Errorf("unsupported token runtime mode: %s", cfg.TokenRuntimeMode)
	}
}

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
	// P0-1: Fail startup in production if encryption key is not explicitly set
	if strings.EqualFold(cfg.Auth.Env, "production") || strings.EqualFold(cfg.Auth.Env, "prod") || strings.EqualFold(cfg.Auth.Env, "online") {
		if _, isDefault := checkEncryptionKeyIsDefault(); isDefault {
			panic("FATAL: PASSWORD_ENCRYPTION_KEY environment variable must be explicitly set in production environment. Using the default key is not allowed.")
		}
	}
	return cfg
}

// buildCORSConfig builds CORS config from normalized config
// In production (Env=production/prod/online), rejects wildcard if CORSAllowOrigins not explicitly set
func buildCORSConfig(cfg config.Config) middleware.CORSConfig {
	corsOrigins := cfg.Auth.CORSAllowOrigins
	if len(corsOrigins) == 0 {
		corsOrigins = []string{"*"}
	}
	// P0-2: Warn in production if using wildcard
	if strings.EqualFold(cfg.Auth.Env, "production") || strings.EqualFold(cfg.Auth.Env, "prod") || strings.EqualFold(cfg.Auth.Env, "online") {
		if len(corsOrigins) == 1 && corsOrigins[0] == "*" {
			panic("FATAL: CORS_ALLOW_ORIGINS must be explicitly set in production environment. Using wildcard '*' is not allowed.")
		}
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

func checkEncryptionKeyIsDefault() (string, bool) {
	envKey := os.Getenv("PASSWORD_ENCRYPTION_KEY")
	defaultKey := "default-key-32-bytes-long!!!!!!!"
	return envKey, envKey == "" || envKey == defaultKey
}

func limitHandler(limiter *ratelimit.Middleware, next http.Handler) http.Handler {
	if limiter == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limiter.Limit(next.ServeHTTP)(w, r)
	})
}
