package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"lijiaoqiao/supply-api/internal/config"
	"lijiaoqiao/supply-api/internal/httpapi"
	"lijiaoqiao/supply-api/internal/metrics"
	"lijiaoqiao/supply-api/internal/middleware"
	"lijiaoqiao/supply-api/internal/pkg/logging"
)

type routeRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
}

// BuildServerOptions 定义 HTTP 服务装配所需的最小输入。
type BuildServerOptions struct {
	Env              string
	ServerConfig     config.ServerConfig
	Logger           logging.Logger
	SupplyAPI        *httpapi.SupplyAPI
	AlertAPI         *httpapi.AlertAPI
	IAMHandler       routeRegistrar
	AuthMiddleware   *middleware.AuthMiddleware
	RateLimitConfig  *middleware.RateLimitConfig
	DBHealthCheck    func(context.Context) error
	RedisHealthCheck func(context.Context) error
}

type buildRouteMuxOptions struct {
	SupplyAPI        *httpapi.SupplyAPI
	AlertAPI         *httpapi.AlertAPI
	IAMHandler       routeRegistrar
	DBHealthCheck    func(context.Context) error
	RedisHealthCheck func(context.Context) error
}

type middlewareChainOptions struct {
	Env             string
	Logger          logging.Logger
	AuthMiddleware  *middleware.AuthMiddleware
	RateLimitConfig *middleware.RateLimitConfig
}

type resolvedBuildServerOptions struct {
	Env              string
	ServerConfig     config.ServerConfig
	Logger           logging.Logger
	SupplyAPI        *httpapi.SupplyAPI
	AlertAPI         *httpapi.AlertAPI
	IAMHandler       routeRegistrar
	AuthMiddleware   *middleware.AuthMiddleware
	RateLimitConfig  *middleware.RateLimitConfig
	DBHealthCheck    func(context.Context) error
	RedisHealthCheck func(context.Context) error
}

// BuildServer 构建可复用的 HTTP server 与 handler 装配。
func BuildServer(opts BuildServerOptions) (*http.Server, error) {
	resolved, err := resolveBuildServerOptions(opts)
	if err != nil {
		return nil, err
	}

	mux := buildRouteMux(buildRouteMuxOptions{
		SupplyAPI:        resolved.SupplyAPI,
		AlertAPI:         resolved.AlertAPI,
		IAMHandler:       resolved.IAMHandler,
		DBHealthCheck:    resolved.DBHealthCheck,
		RedisHealthCheck: resolved.RedisHealthCheck,
	})
	handler := buildMiddlewareChain(middlewareChainOptions{
		Env:             resolved.Env,
		Logger:          resolved.Logger,
		AuthMiddleware:  resolved.AuthMiddleware,
		RateLimitConfig: resolved.RateLimitConfig,
	}, mux)

	return &http.Server{
		Addr:              resolved.ServerConfig.Addr,
		Handler:           handler,
		ReadHeaderTimeout: resolved.ServerConfig.ReadTimeout,
		ReadTimeout:       resolved.ServerConfig.ReadTimeout,
		WriteTimeout:      resolved.ServerConfig.WriteTimeout,
		IdleTimeout:       resolved.ServerConfig.IdleTimeout,
	}, nil
}

func resolveBuildServerOptions(opts BuildServerOptions) (resolvedBuildServerOptions, error) {
	if opts.SupplyAPI == nil {
		return resolvedBuildServerOptions{}, errors.New("supply api is required")
	}
	if opts.AlertAPI == nil {
		return resolvedBuildServerOptions{}, errors.New("alert api is required")
	}
	if opts.Logger == nil {
		return resolvedBuildServerOptions{}, errors.New("logger is required")
	}

	env, err := ResolveEnv(opts.Env)
	if err != nil {
		return resolvedBuildServerOptions{}, err
	}
	if env != "dev" && opts.AuthMiddleware == nil {
		return resolvedBuildServerOptions{}, errors.New("auth middleware is required outside dev")
	}

	return resolvedBuildServerOptions{
		Env:              env,
		ServerConfig:     normalizeServerConfig(opts.ServerConfig),
		Logger:           opts.Logger,
		SupplyAPI:        opts.SupplyAPI,
		AlertAPI:         opts.AlertAPI,
		IAMHandler:       opts.IAMHandler,
		AuthMiddleware:   opts.AuthMiddleware,
		RateLimitConfig:  resolveRateLimitConfig(env, opts.RateLimitConfig),
		DBHealthCheck:    opts.DBHealthCheck,
		RedisHealthCheck: opts.RedisHealthCheck,
	}, nil
}

func normalizeServerConfig(serverConfig config.ServerConfig) config.ServerConfig {
	if strings.TrimSpace(serverConfig.Addr) == "" {
		serverConfig.Addr = ":18082"
	}
	if serverConfig.ReadTimeout == 0 {
		serverConfig.ReadTimeout = 10 * time.Second
	}
	if serverConfig.WriteTimeout == 0 {
		serverConfig.WriteTimeout = 15 * time.Second
	}
	if serverConfig.IdleTimeout == 0 {
		serverConfig.IdleTimeout = 30 * time.Second
	}
	if serverConfig.ShutdownTimeout == 0 {
		serverConfig.ShutdownTimeout = 5 * time.Second
	}
	return serverConfig
}

func resolveRateLimitConfig(env string, rateLimitConfig *middleware.RateLimitConfig) *middleware.RateLimitConfig {
	if rateLimitConfig != nil {
		return rateLimitConfig
	}

	rateLimitConfig = middleware.DefaultRateLimitConfig()
	rateLimitConfig.Enabled = env != "dev"
	return rateLimitConfig
}

func buildRouteMux(opts buildRouteMuxOptions) *http.ServeMux {
	mux := http.NewServeMux()
	healthHandler := httpapi.NewHealthHandlerWithDefaults(opts.DBHealthCheck, opts.RedisHealthCheck)
	healthHandler.RegisterRoutes(mux)
	// P3-C: /metrics 端点（Prometheus-text 格式）
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(metrics.Export()))
	})
	// P3-C-05: /health 别名（统一路径，对齐 gateway/platform-token-runtime）
	mux.HandleFunc("/health", healthHandler.ServeHealth)
	mux.HandleFunc("/healthz", healthHandler.ServeHealth)
	opts.SupplyAPI.Register(mux)
	opts.AlertAPI.Register(mux)
	if opts.IAMHandler != nil {
		opts.IAMHandler.RegisterRoutes(mux)
	}
	return mux
}

func buildMiddlewareChain(opts middlewareChainOptions, next http.Handler) http.Handler {
	var handler http.Handler = next
	handler = middleware.RequestID(handler)
	handler = middleware.Recovery(handler)
	handler = middleware.Logging(handler, opts.Logger)
	handler = middleware.TracingMiddleware(handler)

	if opts.Env != "dev" {
		handler = middleware.NewRateLimitHandler(opts.RateLimitConfig, handler)
		handler = opts.AuthMiddleware.TokenVerifyMiddleware(handler)
		handler = opts.AuthMiddleware.BearerExtractMiddleware(handler)
		handler = opts.AuthMiddleware.QueryKeyRejectMiddleware(handler)
	}

	return handler
}
