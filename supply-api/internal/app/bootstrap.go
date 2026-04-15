package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"lijiaoqiao/supply-api/internal/config"
	"lijiaoqiao/supply-api/internal/httpapi"
	"lijiaoqiao/supply-api/internal/middleware"
	"lijiaoqiao/supply-api/internal/pkg/logging"
)

// BuildServerOptions 定义 HTTP 服务装配所需的最小输入。
type BuildServerOptions struct {
	Env              string
	ServerConfig     config.ServerConfig
	Logger           logging.Logger
	SupplyAPI        *httpapi.SupplyAPI
	AlertAPI         *httpapi.AlertAPI
	AuthMiddleware   *middleware.AuthMiddleware
	RateLimitConfig  *middleware.RateLimitConfig
	DBHealthCheck    func(context.Context) error
	RedisHealthCheck func(context.Context) error
}

// BuildServer 构建可复用的 HTTP server 与 handler 装配。
func BuildServer(opts BuildServerOptions) (*http.Server, error) {
	if opts.SupplyAPI == nil {
		return nil, errors.New("supply api is required")
	}
	if opts.AlertAPI == nil {
		return nil, errors.New("alert api is required")
	}
	if opts.Logger == nil {
		return nil, errors.New("logger is required")
	}

	env, err := ResolveEnv(opts.Env)
	if err != nil {
		return nil, err
	}
	if env != "dev" && opts.AuthMiddleware == nil {
		return nil, errors.New("auth middleware is required outside dev")
	}

	rateLimitConfig := opts.RateLimitConfig
	if rateLimitConfig == nil {
		rateLimitConfig = middleware.DefaultRateLimitConfig()
		rateLimitConfig.Enabled = env != "dev"
	}

	mux := http.NewServeMux()
	healthHandler := httpapi.NewHealthHandlerWithDefaults(opts.DBHealthCheck, opts.RedisHealthCheck)
	healthHandler.RegisterRoutes(mux)
	opts.SupplyAPI.Register(mux)
	opts.AlertAPI.Register(mux)

	handler := buildHandler(env, mux, opts.Logger, opts.AuthMiddleware, rateLimitConfig)

	serverConfig := normalizeServerConfig(opts.ServerConfig)

	return &http.Server{
		Addr:              serverConfig.Addr,
		Handler:           handler,
		ReadHeaderTimeout: serverConfig.ReadTimeout,
		ReadTimeout:       serverConfig.ReadTimeout,
		WriteTimeout:      serverConfig.WriteTimeout,
		IdleTimeout:       serverConfig.IdleTimeout,
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

func buildHandler(
	env string,
	mux *http.ServeMux,
	logger logging.Logger,
	authMiddleware *middleware.AuthMiddleware,
	rateLimitConfig *middleware.RateLimitConfig,
) http.Handler {
	var handler http.Handler = mux
	handler = middleware.RequestID(handler)
	handler = middleware.Recovery(handler)
	handler = middleware.Logging(handler, logger)
	handler = middleware.TracingMiddleware(handler)

	if env != "dev" {
		handler = middleware.NewRateLimitHandler(rateLimitConfig, handler)
		handler = authMiddleware.TokenVerifyMiddleware(handler)
		handler = authMiddleware.BearerExtractMiddleware(handler)
		handler = authMiddleware.QueryKeyRejectMiddleware(handler)
	}

	return handler
}
