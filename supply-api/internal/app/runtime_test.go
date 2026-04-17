package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/audit"
	"lijiaoqiao/supply-api/internal/cache"
	"lijiaoqiao/supply-api/internal/config"
	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/messaging"
	"lijiaoqiao/supply-api/internal/middleware"
	"lijiaoqiao/supply-api/internal/repository"
)

type captureLogger struct {
	infoMessages  []string
	warnMessages  []string
	errorMessages []string
}

func (l *captureLogger) Debug(string, ...map[string]interface{}) {}

func (l *captureLogger) Info(msg string, _ ...map[string]interface{}) {
	l.infoMessages = append(l.infoMessages, msg)
}

func (l *captureLogger) Warn(msg string, _ ...map[string]interface{}) {
	l.warnMessages = append(l.warnMessages, msg)
}

func (l *captureLogger) Error(msg string, _ ...map[string]interface{}) {
	l.errorMessages = append(l.errorMessages, msg)
}

func (l *captureLogger) Fatal(msg string, _ ...map[string]interface{}) {
	l.errorMessages = append(l.errorMessages, msg)
}

func TestBuildRuntime_ProdRequiresDatabase(t *testing.T) {
	_, err := buildRuntimeWithFactory(RuntimeOptions{
		Env:         "prod",
		Config:      testRuntimeConfig(),
		Logger:      testLogger{},
		InitContext: context.Background(),
		Now: func() time.Time {
			return time.Unix(1712800000, 0).UTC()
		},
	}, runtimeFactory{
		newDB: func(context.Context, config.DatabaseConfig) (*repository.DB, error) {
			return nil, errors.New("db down")
		},
		newRedisCache: func(config.RedisConfig) (*cache.RedisCache, error) {
			return nil, nil
		},
	})
	if err == nil {
		t.Fatal("expected prod runtime build to reject database outage")
	}
	if !strings.Contains(err.Error(), "database unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildStoreBundle_UsesInMemoryStoresWithoutDatabase(t *testing.T) {
	bundle := buildStoreBundle(nil, testLogger{})
	if bundle.accountStore == nil {
		t.Fatal("expected account store")
	}
	if bundle.packageStore == nil {
		t.Fatal("expected package store")
	}
	if bundle.settlementStore == nil {
		t.Fatal("expected settlement store")
	}
	if bundle.earningStore == nil {
		t.Fatal("expected earning store")
	}
	if bundle.auditStore == nil {
		t.Fatal("expected audit store")
	}
	if bundle.alertService == nil {
		t.Fatal("expected alert service")
	}
	if bundle.tokenStatusRepo != nil {
		t.Fatal("expected nil token status repo without database")
	}
	if bundle.idempotencyRepo != nil {
		t.Fatal("expected nil idempotency repo without database")
	}
}

func TestBuildMemoryStoreBundle_DisablesDatabaseOnlyDependencies(t *testing.T) {
	bundle := buildMemoryStoreBundle()
	if bundle.accountStore == nil {
		t.Fatal("expected account store")
	}
	if bundle.packageStore == nil {
		t.Fatal("expected package store")
	}
	if bundle.settlementStore == nil {
		t.Fatal("expected settlement store")
	}
	if bundle.earningStore == nil {
		t.Fatal("expected earning store")
	}
	if bundle.auditStore == nil {
		t.Fatal("expected audit store")
	}
	if bundle.alertService == nil {
		t.Fatal("expected alert service")
	}
	if bundle.tokenStatusRepo != nil {
		t.Fatal("expected nil token status repo")
	}
	if bundle.idempotencyRepo != nil {
		t.Fatal("expected nil idempotency repo")
	}
}

func TestBuildStoreBundle_UsesDatabaseBackedStoresWithDatabase(t *testing.T) {
	bundle := buildStoreBundle(&repository.DB{}, testLogger{})
	if bundle.accountStore == nil {
		t.Fatal("expected account store")
	}
	if bundle.packageStore == nil {
		t.Fatal("expected package store")
	}
	if bundle.settlementStore == nil {
		t.Fatal("expected settlement store")
	}
	if bundle.earningStore == nil {
		t.Fatal("expected earning store")
	}
	if bundle.auditStore == nil {
		t.Fatal("expected audit store")
	}
	if bundle.alertService == nil {
		t.Fatal("expected alert service")
	}
	if bundle.tokenStatusRepo == nil {
		t.Fatal("expected token status repo with database")
	}
	if bundle.idempotencyRepo == nil {
		t.Fatal("expected idempotency repo with database")
	}
}

func TestBuildDBStoreBundle_InitializesDatabaseOnlyDependencies(t *testing.T) {
	bundle := buildDBStoreBundle(&repository.DB{})
	if bundle.accountStore == nil {
		t.Fatal("expected account store")
	}
	if bundle.packageStore == nil {
		t.Fatal("expected package store")
	}
	if bundle.settlementStore == nil {
		t.Fatal("expected settlement store")
	}
	if bundle.earningStore == nil {
		t.Fatal("expected earning store")
	}
	if bundle.auditStore == nil {
		t.Fatal("expected audit store")
	}
	if bundle.alertService == nil {
		t.Fatal("expected alert service")
	}
	if bundle.tokenStatusRepo == nil {
		t.Fatal("expected token status repo")
	}
	if bundle.idempotencyRepo == nil {
		t.Fatal("expected idempotency repo")
	}
}

func TestBuildSecurityBundle_UsesMemoryTokenBackendWithoutRepository(t *testing.T) {
	security := buildSecurityBundle("dev", testRuntimeConfig(), testLogger{}, audit.NewMemoryAuditStore(), nil, nil)
	if security.authMiddleware == nil {
		t.Fatal("expected auth middleware")
	}
	if security.revocationSubscriber != nil {
		t.Fatal("expected nil revocation subscriber without token repository")
	}
}

func TestResolveEnv_RejectsUnsupportedValue(t *testing.T) {
	_, err := ResolveEnv("qa")
	if err == nil {
		t.Fatal("expected unsupported env to fail")
	}
	if !strings.Contains(err.Error(), "unsupported env") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveRuntimeBuildInputs_NormalizesDefaults(t *testing.T) {
	inputs, err := resolveRuntimeBuildInputs(RuntimeOptions{
		Config: testRuntimeConfig(),
		Logger: testLogger{},
	})
	if err != nil {
		t.Fatalf("expected input normalization to succeed, got %v", err)
	}
	if inputs.cfg == nil {
		t.Fatal("expected config to be preserved")
	}
	if inputs.logger == nil {
		t.Fatal("expected logger to be preserved")
	}
	if inputs.env != "dev" {
		t.Fatalf("unexpected env: %s", inputs.env)
	}
	if inputs.isProd {
		t.Fatal("expected default env to be non-prod")
	}
	if inputs.now == nil {
		t.Fatal("expected now func to be defaulted")
	}
	if inputs.initCtx == nil {
		t.Fatal("expected init context to be defaulted")
	}
	if inputs.tuning.outboxStreamName != "supply:outbox:stream" {
		t.Fatalf("unexpected outbox stream: %s", inputs.tuning.outboxStreamName)
	}
}

func TestInitializeRuntimeExternalResources_ProdRejectsDatabaseFailure(t *testing.T) {
	_, err := initializeRuntimeExternalResources(runtimeBuildInputs{
		env:     "prod",
		cfg:     testRuntimeConfig(),
		logger:  testLogger{},
		initCtx: context.Background(),
		isProd:  true,
		tuning:  defaultRuntimeTuning(),
		now:     time.Now,
	}, runtimeFactory{
		newDB: func(context.Context, config.DatabaseConfig) (*repository.DB, error) {
			return nil, errors.New("db down")
		},
		newRedisCache: func(config.RedisConfig) (*cache.RedisCache, error) {
			return nil, nil
		},
	})
	if err == nil {
		t.Fatal("expected prod runtime resources to reject database outage")
	}
	if !strings.Contains(err.Error(), "database unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInitializeRuntimeExternalResources_DevFallsBackToNilResources(t *testing.T) {
	logger := &captureLogger{}

	resources, err := initializeRuntimeExternalResources(runtimeBuildInputs{
		env:     "dev",
		cfg:     testRuntimeConfig(),
		logger:  logger,
		initCtx: context.Background(),
		isProd:  false,
		tuning:  defaultRuntimeTuning(),
		now:     time.Now,
	}, runtimeFactory{
		newDB: func(context.Context, config.DatabaseConfig) (*repository.DB, error) {
			return nil, errors.New("db down")
		},
		newRedisCache: func(config.RedisConfig) (*cache.RedisCache, error) {
			return nil, errors.New("redis down")
		},
	})
	if err != nil {
		t.Fatalf("expected dev runtime resources to fall back, got %v", err)
	}
	if resources.db != nil {
		t.Fatal("expected nil db after dev fallback")
	}
	if resources.redisCache != nil {
		t.Fatal("expected nil redis cache after dev fallback")
	}
	if len(logger.warnMessages) == 0 {
		t.Fatal("expected warning logs during dev resource fallback")
	}
}

func TestBuildRuntime_RejectsUnsupportedEnv(t *testing.T) {
	_, err := buildRuntimeWithFactory(RuntimeOptions{
		Env:         "qa",
		Config:      testRuntimeConfig(),
		Logger:      testLogger{},
		InitContext: context.Background(),
	}, runtimeFactory{
		newDB: func(context.Context, config.DatabaseConfig) (*repository.DB, error) {
			return nil, errors.New("db down")
		},
		newRedisCache: func(config.RedisConfig) (*cache.RedisCache, error) {
			return nil, nil
		},
	})
	if err == nil {
		t.Fatal("expected unsupported env to fail")
	}
	if !strings.Contains(err.Error(), "unsupported env") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRuntime_DevFallsBackToInMemoryDependencies(t *testing.T) {
	runtime, err := buildRuntimeWithFactory(RuntimeOptions{
		Env:         "dev",
		Config:      testRuntimeConfig(),
		Logger:      testLogger{},
		InitContext: context.Background(),
		Now: func() time.Time {
			return time.Unix(1712800000, 0).UTC()
		},
	}, runtimeFactory{
		newDB: func(context.Context, config.DatabaseConfig) (*repository.DB, error) {
			return nil, errors.New("db down")
		},
		newRedisCache: func(config.RedisConfig) (*cache.RedisCache, error) {
			return nil, errors.New("redis down")
		},
	})
	if err != nil {
		t.Fatalf("expected dev runtime to fall back to in-memory dependencies, got %v", err)
	}
	if runtime == nil {
		t.Fatal("expected runtime")
	}
	if runtime.resources.db != nil {
		t.Fatal("expected nil db after dev fallback")
	}
	if runtime.resources.redisCache != nil {
		t.Fatal("expected nil redis cache after dev fallback")
	}
	if runtime.startupViews.http.supplyAPI == nil || runtime.startupViews.http.alertAPI == nil {
		t.Fatal("expected apis to be initialized")
	}
	if runtime.startupViews.http.authMiddleware == nil {
		t.Fatal("expected auth middleware to be initialized")
	}
	if runtime.startupViews.http.rateLimitConfig == nil {
		t.Fatal("expected rate limit config to be initialized")
	}
	if runtime.startupViews.http.iamHandler != nil {
		t.Fatal("expected IAM routes to stay disabled by default")
	}
}

func TestBuildRuntime_EnableIAMRequiresDatabaseBackedRuntime(t *testing.T) {
	cfg := testRuntimeConfig()
	cfg.Server.IAMEnabled = true

	_, err := buildRuntimeWithFactory(RuntimeOptions{
		Env:         "dev",
		Config:      cfg,
		Logger:      testLogger{},
		InitContext: context.Background(),
		Now: func() time.Time {
			return time.Unix(1712800000, 0).UTC()
		},
	}, runtimeFactory{
		newDB: func(context.Context, config.DatabaseConfig) (*repository.DB, error) {
			return nil, errors.New("db down")
		},
		newRedisCache: func(config.RedisConfig) (*cache.RedisCache, error) {
			return nil, nil
		},
	})
	if err == nil {
		t.Fatal("expected IAM-enabled runtime build to reject missing database")
	}
	if !strings.Contains(err.Error(), "iam requires database-backed runtime") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRuntime_EnablesIAMRoutesWhenConfigured(t *testing.T) {
	cfg := testRuntimeConfig()
	cfg.Server.IAMEnabled = true

	runtime, err := buildRuntimeWithFactory(RuntimeOptions{
		Env:         "dev",
		Config:      cfg,
		Logger:      testLogger{},
		InitContext: context.Background(),
		Now: func() time.Time {
			return time.Unix(1712800000, 0).UTC()
		},
	}, runtimeFactory{
		newDB: func(context.Context, config.DatabaseConfig) (*repository.DB, error) {
			return &repository.DB{}, nil
		},
		newRedisCache: func(config.RedisConfig) (*cache.RedisCache, error) {
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("expected IAM-enabled runtime build to succeed, got %v", err)
	}
	if runtime.startupViews.http.iamHandler == nil {
		t.Fatal("expected IAM handler to be wired when enabled")
	}
}

func TestBuildRuntime_NormalizesServerConfigDefaults(t *testing.T) {
	cfg := testRuntimeConfig()
	cfg.Server = config.ServerConfig{}

	runtime, err := buildRuntimeWithFactory(RuntimeOptions{
		Env:         "dev",
		Config:      cfg,
		Logger:      testLogger{},
		InitContext: context.Background(),
		Now: func() time.Time {
			return time.Unix(1712800000, 0).UTC()
		},
	}, runtimeFactory{
		newDB: func(context.Context, config.DatabaseConfig) (*repository.DB, error) {
			return nil, errors.New("db down")
		},
		newRedisCache: func(config.RedisConfig) (*cache.RedisCache, error) {
			return nil, errors.New("redis down")
		},
	})
	if err != nil {
		t.Fatalf("expected runtime build to succeed, got %v", err)
	}
	if runtime.startupViews.http.serverConfig.Addr != ":18082" {
		t.Fatalf("unexpected addr: %s", runtime.startupViews.http.serverConfig.Addr)
	}
	if runtime.ShutdownTimeout() != 5*time.Second {
		t.Fatalf("unexpected shutdown timeout: %s", runtime.ShutdownTimeout())
	}
}

func TestBuildRuntime_SeedsDefaultTuning(t *testing.T) {
	runtime, err := buildRuntimeWithFactory(RuntimeOptions{
		Env:         "dev",
		Config:      testRuntimeConfig(),
		Logger:      testLogger{},
		InitContext: context.Background(),
		Now: func() time.Time {
			return time.Unix(1712800000, 0).UTC()
		},
	}, runtimeFactory{
		newDB: func(context.Context, config.DatabaseConfig) (*repository.DB, error) {
			return nil, errors.New("db down")
		},
		newRedisCache: func(config.RedisConfig) (*cache.RedisCache, error) {
			return nil, errors.New("redis down")
		},
	})
	if err != nil {
		t.Fatalf("expected runtime build to succeed, got %v", err)
	}
	if runtime.startupViews.background.tuning.outboxStreamName != "supply:outbox:stream" {
		t.Fatalf("unexpected outbox stream: %s", runtime.startupViews.background.tuning.outboxStreamName)
	}
	if runtime.startupViews.background.tuning.outboxConsumerGroup != "outbox-processor" {
		t.Fatalf("unexpected outbox group: %s", runtime.startupViews.background.tuning.outboxConsumerGroup)
	}
	if runtime.startupViews.background.tuning.idempotencyTTL != 24*time.Hour {
		t.Fatalf("unexpected idempotency ttl: %s", runtime.startupViews.background.tuning.idempotencyTTL)
	}
	if runtime.startupViews.background.tuning.partitionMaintenanceInterval != time.Hour {
		t.Fatalf("unexpected partition maintenance interval: %s", runtime.startupViews.background.tuning.partitionMaintenanceInterval)
	}
	if runtime.startupViews.background.tuning.compensationCheckInterval != 5*time.Minute {
		t.Fatalf("unexpected compensation interval: %s", runtime.startupViews.background.tuning.compensationCheckInterval)
	}
}

func TestBuildRuntime_DevFallbackLogsWarnings(t *testing.T) {
	logger := &captureLogger{}

	_, err := buildRuntimeWithFactory(RuntimeOptions{
		Env:         "dev",
		Config:      testRuntimeConfig(),
		Logger:      logger,
		InitContext: context.Background(),
	}, runtimeFactory{
		newDB: func(context.Context, config.DatabaseConfig) (*repository.DB, error) {
			return nil, errors.New("db down")
		},
		newRedisCache: func(config.RedisConfig) (*cache.RedisCache, error) {
			return nil, errors.New("redis down")
		},
	})
	if err != nil {
		t.Fatalf("expected runtime build to succeed, got %v", err)
	}
	if len(logger.warnMessages) == 0 {
		t.Fatal("expected warning logs during dev fallback")
	}
	if len(logger.infoMessages) == 0 {
		t.Fatal("expected info logs during successful in-memory runtime initialization")
	}
}

func TestBuildRuntimeResources_GroupsExternalDependencies(t *testing.T) {
	db := &repository.DB{}
	redisCache := &cache.RedisCache{}

	resources := buildRuntimeResources(db, redisCache)
	if resources.db != db {
		t.Fatal("expected db resource to be preserved")
	}
	if resources.redisCache != redisCache {
		t.Fatal("expected redis cache resource to be preserved")
	}
}

func TestBuildRuntimeStartupViews_GroupsHTTPAndBackgroundDependencies(t *testing.T) {
	supplyAPI, alertAPI := mustBuildTestAPIs(t)
	logger := testLogger{}
	authMiddleware := &middleware.AuthMiddleware{}
	rateLimitConfig := &middleware.RateLimitConfig{Enabled: true}
	tuning := defaultRuntimeTuning()
	subscriber := stubRevocationSubscriber{}

	views := buildRuntimeStartupViews(
		"staging",
		logger,
		config.ServerConfig{},
		tuning,
		runtimeSecurityBundle{
			authMiddleware:       authMiddleware,
			revocationSubscriber: subscriber,
		},
		runtimeAPIBundle{
			supplyAPI:       supplyAPI,
			alertAPI:        alertAPI,
			rateLimitConfig: rateLimitConfig,
		},
	)
	if views.http.env != "staging" {
		t.Fatalf("unexpected http env: %s", views.http.env)
	}
	if views.background.env != "staging" {
		t.Fatalf("unexpected background env: %s", views.background.env)
	}
	if views.http.serverConfig.Addr != ":18082" {
		t.Fatalf("unexpected default addr: %s", views.http.serverConfig.Addr)
	}
	if views.http.serverConfig.ShutdownTimeout != 5*time.Second {
		t.Fatalf("unexpected default shutdown timeout: %s", views.http.serverConfig.ShutdownTimeout)
	}
	if views.http.supplyAPI != supplyAPI {
		t.Fatal("expected supply api to be preserved")
	}
	if views.http.alertAPI != alertAPI {
		t.Fatal("expected alert api to be preserved")
	}
	if views.http.authMiddleware != authMiddleware {
		t.Fatal("expected auth middleware to be preserved")
	}
	if views.http.rateLimitConfig != rateLimitConfig {
		t.Fatal("expected rate limit config to be preserved")
	}
	if views.background.revocationSubscriber != subscriber {
		t.Fatal("expected revocation subscriber to be preserved")
	}
	if views.background.tuning.outboxStreamName != tuning.outboxStreamName {
		t.Fatalf("unexpected background outbox stream: %s", views.background.tuning.outboxStreamName)
	}
}

func TestBuildRuntime_GroupsResourcesAndStartupViews(t *testing.T) {
	runtime, err := buildRuntimeWithFactory(RuntimeOptions{
		Env:         "dev",
		Config:      testRuntimeConfig(),
		Logger:      testLogger{},
		InitContext: context.Background(),
		Now: func() time.Time {
			return time.Unix(1712800000, 0).UTC()
		},
	}, runtimeFactory{
		newDB: func(context.Context, config.DatabaseConfig) (*repository.DB, error) {
			return nil, errors.New("db down")
		},
		newRedisCache: func(config.RedisConfig) (*cache.RedisCache, error) {
			return nil, errors.New("redis down")
		},
	})
	if err != nil {
		t.Fatalf("expected runtime build to succeed, got %v", err)
	}
	if runtime.startupViews.http.env != "dev" {
		t.Fatalf("unexpected http env: %s", runtime.startupViews.http.env)
	}
	if runtime.startupViews.background.env != "dev" {
		t.Fatalf("unexpected background env: %s", runtime.startupViews.background.env)
	}
	if runtime.resources.db != nil {
		t.Fatal("expected nil db resource after dev fallback")
	}
	if runtime.resources.redisCache != nil {
		t.Fatal("expected nil redis resource after dev fallback")
	}
	if runtime.startupViews.http.supplyAPI == nil || runtime.startupViews.http.alertAPI == nil {
		t.Fatal("expected http startup view apis to be initialized")
	}
	if runtime.startupViews.http.authMiddleware == nil {
		t.Fatal("expected http startup view auth middleware")
	}
	if runtime.startupViews.background.tuning.outboxStreamName != "supply:outbox:stream" {
		t.Fatalf("unexpected background outbox stream: %s", runtime.startupViews.background.tuning.outboxStreamName)
	}
}

func TestResolveRuntimeHealthChecks_OmitsUnavailableDependencies(t *testing.T) {
	checks := resolveRuntimeHealthChecks(&Runtime{})
	if checks.DBHealthCheck != nil {
		t.Fatal("expected nil db health check without database")
	}
	if checks.RedisHealthCheck != nil {
		t.Fatal("expected nil redis health check without redis")
	}
}

func TestResolveRuntimeHealthChecks_ExposesAvailableDependencies(t *testing.T) {
	checks := resolveRuntimeHealthChecks(&Runtime{
		resources: runtimeExternalResources{
			db:         &repository.DB{},
			redisCache: &cache.RedisCache{},
		},
	})
	if checks.DBHealthCheck == nil {
		t.Fatal("expected db health check")
	}
	if checks.RedisHealthCheck == nil {
		t.Fatal("expected redis health check")
	}
}

func TestBuildRuntimeHTTPView_RequiresRuntime(t *testing.T) {
	_, err := buildRuntimeHTTPView(nil)
	if err == nil {
		t.Fatal("expected nil runtime to fail")
	}
	if !strings.Contains(err.Error(), "runtime is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRuntimeHTTPView_MapsHTTPFields(t *testing.T) {
	supplyAPI, alertAPI := mustBuildTestAPIs(t)
	authMiddleware := &middleware.AuthMiddleware{}
	rateLimitConfig := &middleware.RateLimitConfig{Enabled: true}

	view, err := buildRuntimeHTTPView(&Runtime{
		resources: runtimeExternalResources{
			db:         &repository.DB{},
			redisCache: &cache.RedisCache{},
		},
		startupViews: runtimeStartupViews{
			http: runtimeHTTPStartupView{
				env:             "staging",
				logger:          testLogger{},
				serverConfig:    config.ServerConfig{Addr: ":19090"},
				supplyAPI:       supplyAPI,
				alertAPI:        alertAPI,
				authMiddleware:  authMiddleware,
				rateLimitConfig: rateLimitConfig,
			},
		},
	})
	if err != nil {
		t.Fatalf("expected view build to succeed, got %v", err)
	}
	if view.env != "staging" {
		t.Fatalf("unexpected env: %s", view.env)
	}
	if view.serverConfig.Addr != ":19090" {
		t.Fatalf("unexpected server addr: %s", view.serverConfig.Addr)
	}
	if view.supplyAPI != supplyAPI {
		t.Fatal("expected supply api to be preserved")
	}
	if view.alertAPI != alertAPI {
		t.Fatal("expected alert api to be preserved")
	}
	if view.authMiddleware != authMiddleware {
		t.Fatal("expected auth middleware to be preserved")
	}
	if view.rateLimitConfig != rateLimitConfig {
		t.Fatal("expected rate limit config to be preserved")
	}
	if view.healthChecks.DBHealthCheck == nil {
		t.Fatal("expected db health check")
	}
	if view.healthChecks.RedisHealthCheck == nil {
		t.Fatal("expected redis health check")
	}
}

func TestAdaptRuntimeToBuildServerOptions_RequiresRuntime(t *testing.T) {
	_, err := adaptRuntimeToBuildServerOptions(nil)
	if err == nil {
		t.Fatal("expected nil runtime to fail")
	}
	if !strings.Contains(err.Error(), "runtime is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdaptRuntimeToBuildServerOptions_MapsRuntimeFields(t *testing.T) {
	supplyAPI, alertAPI := mustBuildTestAPIs(t)
	authMiddleware := &middleware.AuthMiddleware{}
	rateLimitConfig := &middleware.RateLimitConfig{Enabled: true}

	opts, err := adaptRuntimeToBuildServerOptions(&Runtime{
		resources: runtimeExternalResources{
			db:         &repository.DB{},
			redisCache: &cache.RedisCache{},
		},
		startupViews: runtimeStartupViews{
			http: runtimeHTTPStartupView{
				env:             "staging",
				logger:          testLogger{},
				serverConfig:    config.ServerConfig{Addr: ":19090"},
				supplyAPI:       supplyAPI,
				alertAPI:        alertAPI,
				authMiddleware:  authMiddleware,
				rateLimitConfig: rateLimitConfig,
			},
		},
	})
	if err != nil {
		t.Fatalf("expected adapter to succeed, got %v", err)
	}
	if opts.Env != "staging" {
		t.Fatalf("unexpected env: %s", opts.Env)
	}
	if opts.ServerConfig.Addr != ":19090" {
		t.Fatalf("unexpected server addr: %s", opts.ServerConfig.Addr)
	}
	if opts.SupplyAPI != supplyAPI {
		t.Fatal("expected supply api to be preserved")
	}
	if opts.AlertAPI != alertAPI {
		t.Fatal("expected alert api to be preserved")
	}
	if opts.AuthMiddleware != authMiddleware {
		t.Fatal("expected auth middleware to be preserved")
	}
	if opts.RateLimitConfig != rateLimitConfig {
		t.Fatal("expected rate limit config to be preserved")
	}
	if opts.DBHealthCheck == nil {
		t.Fatal("expected db health check")
	}
	if opts.RedisHealthCheck == nil {
		t.Fatal("expected redis health check")
	}
}

func TestAdaptRuntimeHTTPViewToBuildServerOptions_MapsHealthChecks(t *testing.T) {
	supplyAPI, alertAPI := mustBuildTestAPIs(t)
	authMiddleware := &middleware.AuthMiddleware{}
	rateLimitConfig := &middleware.RateLimitConfig{Enabled: true}
	view := runtimeHTTPView{
		env:             "staging",
		logger:          testLogger{},
		serverConfig:    config.ServerConfig{Addr: ":19090"},
		supplyAPI:       supplyAPI,
		alertAPI:        alertAPI,
		authMiddleware:  authMiddleware,
		rateLimitConfig: rateLimitConfig,
		healthChecks: runtimeHealthChecks{
			DBHealthCheck:    func(context.Context) error { return nil },
			RedisHealthCheck: func(context.Context) error { return nil },
		},
	}

	opts := adaptRuntimeHTTPViewToBuildServerOptions(view)
	if opts.Env != "staging" {
		t.Fatalf("unexpected env: %s", opts.Env)
	}
	if opts.SupplyAPI != supplyAPI {
		t.Fatal("expected supply api to be preserved")
	}
	if opts.AlertAPI != alertAPI {
		t.Fatal("expected alert api to be preserved")
	}
	if opts.AuthMiddleware != authMiddleware {
		t.Fatal("expected auth middleware to be preserved")
	}
	if opts.RateLimitConfig != rateLimitConfig {
		t.Fatal("expected rate limit config to be preserved")
	}
	if opts.DBHealthCheck == nil {
		t.Fatal("expected db health check")
	}
	if opts.RedisHealthCheck == nil {
		t.Fatal("expected redis health check")
	}
}

func TestBuildRuntimeBackgroundView_RequiresRuntime(t *testing.T) {
	_, err := buildRuntimeBackgroundView(nil)
	if err == nil {
		t.Fatal("expected nil runtime to fail")
	}
	if !strings.Contains(err.Error(), "runtime is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRuntimeBackgroundView_MapsBackgroundFields(t *testing.T) {
	subscriber := stubRevocationSubscriber{}

	view, err := buildRuntimeBackgroundView(&Runtime{
		resources: runtimeExternalResources{
			db:         &repository.DB{},
			redisCache: &cache.RedisCache{},
		},
		startupViews: runtimeStartupViews{
			background: runtimeBackgroundStartupView{
				env:                  "prod",
				logger:               testLogger{},
				tuning:               defaultRuntimeTuning(),
				revocationSubscriber: subscriber,
			},
		},
	})
	if err != nil {
		t.Fatalf("expected background view build to succeed, got %v", err)
	}
	if view.env != "prod" {
		t.Fatalf("unexpected env: %s", view.env)
	}
	if view.logger == nil {
		t.Fatal("expected logger to be preserved")
	}
	if view.db == nil {
		t.Fatal("expected db to be preserved")
	}
	if view.redisCache == nil {
		t.Fatal("expected redis cache to be preserved")
	}
	if view.revocationSubscriber == nil {
		t.Fatal("expected revocation subscriber to be preserved")
	}
	if view.tuning.outboxStreamName != "supply:outbox:stream" {
		t.Fatalf("unexpected outbox stream: %s", view.tuning.outboxStreamName)
	}
}

func TestRuntime_StartBackgroundWorkers_WithoutDatabaseIsNoop(t *testing.T) {
	var outboxRepoCalled bool

	err := startBackgroundWorkersWithFactory(context.Background(), context.Background(), &Runtime{
		startupViews: runtimeStartupViews{
			background: runtimeBackgroundStartupView{
				env:    "dev",
				logger: testLogger{},
			},
		},
	}, backgroundFactory{
		newOutboxRepository: func(*repository.DB) outboxRepository {
			outboxRepoCalled = true
			return stubOutboxRepository{}
		},
	})
	if err != nil {
		t.Fatalf("expected nil error when database is unavailable, got %v", err)
	}
	if outboxRepoCalled {
		t.Fatal("expected background workers to skip db-backed startup when db is nil")
	}
}

func TestRuntime_StartBackgroundWorkers_ProdRequiresOutboxBroker(t *testing.T) {
	err := startBackgroundWorkersWithFactory(context.Background(), context.Background(), &Runtime{
		resources: runtimeExternalResources{
			db: &repository.DB{},
		},
		startupViews: runtimeStartupViews{
			background: runtimeBackgroundStartupView{
				env:    "prod",
				logger: testLogger{},
			},
		},
	}, backgroundFactory{
		newOutboxRepository: func(*repository.DB) outboxRepository {
			return stubOutboxRepository{}
		},
		newMessageBroker: func(*cache.RedisCache) messaging.MessageBroker {
			return nil
		},
	})
	if err == nil {
		t.Fatal("expected missing outbox broker to fail in prod")
	}
	if !strings.Contains(err.Error(), "outbox message broker unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartOutboxProcessor_ProdRequiresBroker(t *testing.T) {
	err := startOutboxProcessor(context.Background(), runtimeBackgroundView{
		env:    "prod",
		logger: testLogger{},
		db:     &repository.DB{},
		tuning: defaultRuntimeTuning(),
	}, backgroundFactory{
		newOutboxRepository: func(*repository.DB) outboxRepository {
			return stubOutboxRepository{}
		},
		newMessageBroker: func(*cache.RedisCache) messaging.MessageBroker {
			return nil
		},
	})
	if err == nil {
		t.Fatal("expected missing outbox broker to fail in prod")
	}
	if !strings.Contains(err.Error(), "outbox message broker unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRuntime_StartBackgroundWorkers_UsesDefaultCompensationInterval(t *testing.T) {
	var gotInterval time.Duration

	err := startBackgroundWorkersWithFactory(context.Background(), context.Background(), &Runtime{
		resources: runtimeExternalResources{
			db: &repository.DB{},
		},
		startupViews: runtimeStartupViews{
			background: runtimeBackgroundStartupView{
				env:    "dev",
				logger: testLogger{},
				tuning: defaultRuntimeTuning(),
			},
		},
	}, backgroundFactory{
		newOutboxRepository: func(*repository.DB) outboxRepository {
			return stubOutboxRepository{}
		},
		newMessageBroker: func(*cache.RedisCache) messaging.MessageBroker {
			return stubMessageBroker{}
		},
		newOutboxRunner: func(outboxRepository, messaging.MessageBroker, messaging.OutboxStats) outboxRunner {
			return stubOutboxRunner{}
		},
		newPartitionManager: func(*repository.DB) partitionManager {
			return stubPartitionManager{}
		},
		newCompensationStore: func(*repository.DB) domain.CompensationStore {
			return stubCompensationStore{}
		},
		newCompensationExecutor: func() domain.OperationExecutor {
			return stubOperationExecutor{}
		},
		newCompensationProcessor: func(
			domain.CompensationStore,
			domain.OperationExecutor,
			domain.CompensationStats,
		) compensationWorker {
			return stubCompensationWorker{
				start: func(_ context.Context, interval time.Duration) {
					gotInterval = interval
				},
			}
		},
	})
	if err != nil {
		t.Fatalf("expected background startup to succeed, got %v", err)
	}
	if gotInterval != 5*time.Minute {
		t.Fatalf("unexpected compensation interval: %s", gotInterval)
	}
}

func TestCompensationNewDefaultExecutor_FailsClosedWithoutRollbackDependencies(t *testing.T) {
	executor := compensationNewDefaultExecutor()

	err := executor.Execute(context.Background(), "account.create", json.RawMessage(`{"account_id":101,"supplier_id":201}`))
	if err == nil {
		t.Fatal("expected default background compensation executor to fail closed")
	}
	if !strings.Contains(err.Error(), "not implemented for production rollback") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartCompensationWorker_UsesConfiguredInterval(t *testing.T) {
	var gotInterval time.Duration

	startCompensationWorker(context.Background(), runtimeBackgroundView{
		env:    "dev",
		logger: testLogger{},
		db:     &repository.DB{},
		tuning: defaultRuntimeTuning(),
	}, backgroundFactory{
		newCompensationStore: func(*repository.DB) domain.CompensationStore {
			return stubCompensationStore{}
		},
		newCompensationExecutor: func() domain.OperationExecutor {
			return stubOperationExecutor{}
		},
		newCompensationProcessor: func(
			domain.CompensationStore,
			domain.OperationExecutor,
			domain.CompensationStats,
		) compensationWorker {
			return stubCompensationWorker{
				start: func(_ context.Context, interval time.Duration) {
					gotInterval = interval
				},
			}
		},
	})
	if gotInterval != 5*time.Minute {
		t.Fatalf("unexpected compensation interval: %s", gotInterval)
	}
}

func TestRuntime_StartBackgroundWorkers_DevMissingOutboxBrokerLogsWarning(t *testing.T) {
	logger := &captureLogger{}

	err := startBackgroundWorkersWithFactory(context.Background(), context.Background(), &Runtime{
		resources: runtimeExternalResources{
			db: &repository.DB{},
		},
		startupViews: runtimeStartupViews{
			background: runtimeBackgroundStartupView{
				env:    "dev",
				logger: logger,
				tuning: defaultRuntimeTuning(),
			},
		},
	}, backgroundFactory{
		newOutboxRepository: func(*repository.DB) outboxRepository {
			return stubOutboxRepository{}
		},
		newMessageBroker: func(*cache.RedisCache) messaging.MessageBroker {
			return nil
		},
		newPartitionManager: func(*repository.DB) partitionManager {
			return stubPartitionManager{}
		},
		newCompensationStore: func(*repository.DB) domain.CompensationStore {
			return stubCompensationStore{}
		},
		newCompensationExecutor: func() domain.OperationExecutor {
			return stubOperationExecutor{}
		},
		newCompensationProcessor: func(
			domain.CompensationStore,
			domain.OperationExecutor,
			domain.CompensationStats,
		) compensationWorker {
			return stubCompensationWorker{}
		},
	})
	if err != nil {
		t.Fatalf("expected background startup to succeed, got %v", err)
	}
	if len(logger.warnMessages) == 0 {
		t.Fatal("expected warning log when outbox broker is unavailable in dev")
	}
}

func testRuntimeConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Addr:              ":18082",
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       30 * time.Second,
			ShutdownTimeout:   5 * time.Second,
			DefaultSupplierID: 1,
			StatementBaseURL:  "https://statements.example.com",
		},
		Database: config.DatabaseConfig{
			Host:            "127.0.0.1",
			Port:            5432,
			User:            "test",
			Password:        "test",
			Database:        "supply",
			MaxOpenConns:    4,
			MaxIdleConns:    2,
			ConnMaxLifetime: time.Minute,
			ConnMaxIdleTime: time.Minute,
		},
		Redis: config.RedisConfig{
			Host:     "127.0.0.1",
			Port:     6379,
			Password: "",
			DB:       0,
			PoolSize: 2,
		},
		Token: config.TokenConfig{
			SecretKey:          "runtime-test-secret",
			Algorithm:          "HS256",
			Issuer:             "runtime-test",
			RevocationCacheTTL: 10 * time.Second,
		},
		Settlement: config.SettlementConfig{
			WithdrawEnabled: true,
		},
	}
}

type stubOutboxRepository struct{}

type stubRevocationSubscriber struct{}

func (stubRevocationSubscriber) StartRevocationSubscriber(context.Context) error {
	return nil
}

func (stubOutboxRepository) FetchAndLock(context.Context, int) ([]*repository.OutboxEvent, error) {
	return nil, nil
}

func (stubOutboxRepository) MarkCompleted(context.Context, string) error {
	return nil
}

func (stubOutboxRepository) MarkFailed(context.Context, string, string, *time.Time) error {
	return nil
}

func (stubOutboxRepository) MoveToDeadLetter(context.Context, *repository.OutboxEvent, string) error {
	return nil
}

type stubMessageBroker struct{}

func (stubMessageBroker) Publish(context.Context, *repository.OutboxEvent) error {
	return nil
}

type stubOutboxRunner struct{}

func (stubOutboxRunner) Start(context.Context) {}

type stubPartitionManager struct{}

func (stubPartitionManager) EnsureFuturePartitions(context.Context) error {
	return nil
}

func (stubPartitionManager) DropOldPartitions(context.Context, string) (int, error) {
	return 0, nil
}

type stubCompensationStore struct{}

func (stubCompensationStore) Create(context.Context, *domain.BatchCompensation) (int64, error) {
	return 0, nil
}

func (stubCompensationStore) GetByBatchID(context.Context, string) ([]*domain.BatchCompensation, error) {
	return nil, nil
}

func (stubCompensationStore) GetPending(context.Context) ([]*domain.BatchCompensation, error) {
	return nil, nil
}

func (stubCompensationStore) UpdateStatus(context.Context, int64, string) error {
	return nil
}

func (stubCompensationStore) Resolve(context.Context, int64, int64, string) error {
	return nil
}

func (stubCompensationStore) MarkManualRequired(context.Context, int64, string) error {
	return nil
}

type stubOperationExecutor struct{}

func (stubOperationExecutor) Execute(context.Context, string, json.RawMessage) error {
	return nil
}

type stubCompensationWorker struct {
	start func(context.Context, time.Duration)
}

func (w stubCompensationWorker) StartBackgroundWorker(ctx context.Context, interval time.Duration) context.Context {
	if w.start != nil {
		w.start(ctx, interval)
	}
	return ctx
}
