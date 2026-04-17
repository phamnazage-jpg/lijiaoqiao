package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"lijiaoqiao/supply-api/internal/adapter"
	"lijiaoqiao/supply-api/internal/audit"
	auditrepo "lijiaoqiao/supply-api/internal/audit/repository"
	auditservice "lijiaoqiao/supply-api/internal/audit/service"
	"lijiaoqiao/supply-api/internal/cache"
	"lijiaoqiao/supply-api/internal/config"
	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/httpapi"
	iamhandler "lijiaoqiao/supply-api/internal/iam/handler"
	iamrepo "lijiaoqiao/supply-api/internal/iam/repository"
	iamservice "lijiaoqiao/supply-api/internal/iam/service"
	"lijiaoqiao/supply-api/internal/middleware"
	"lijiaoqiao/supply-api/internal/pkg/logging"
	"lijiaoqiao/supply-api/internal/repository"
)

// RuntimeOptions 定义构建 supply-api 运行时所需的输入。
type RuntimeOptions struct {
	Env         string
	Config      *config.Config
	Logger      logging.Logger
	InitContext context.Context
	Now         func() time.Time
}

type runtimeTuning struct {
	outboxStreamName             string
	outboxConsumerGroup          string
	idempotencyTTL               time.Duration
	partitionMaintenanceInterval time.Duration
	compensationCheckInterval    time.Duration
	partitionedTables            []string
}

// Runtime 聚合 HTTP 启动和后台任务启动所需的运行时依赖。
type Runtime struct {
	resources    runtimeExternalResources
	startupViews runtimeStartupViews
}

type revocationSubscriber interface {
	StartRevocationSubscriber(ctx context.Context) error
}

type runtimeExternalResources struct {
	db         *repository.DB
	redisCache *cache.RedisCache
}

type runtimeHTTPStartupView struct {
	env             string
	logger          logging.Logger
	serverConfig    config.ServerConfig
	supplyAPI       *httpapi.SupplyAPI
	alertAPI        *httpapi.AlertAPI
	iamHandler      routeRegistrar
	authMiddleware  *middleware.AuthMiddleware
	rateLimitConfig *middleware.RateLimitConfig
}

type runtimeBackgroundStartupView struct {
	env                  string
	logger               logging.Logger
	tuning               runtimeTuning
	revocationSubscriber revocationSubscriber
}

type runtimeStartupViews struct {
	http       runtimeHTTPStartupView
	background runtimeBackgroundStartupView
}

type runtimeFactory struct {
	newDB         func(ctx context.Context, cfg config.DatabaseConfig) (*repository.DB, error)
	newRedisCache func(cfg config.RedisConfig) (*cache.RedisCache, error)
	newSMSVerifier func(cfg config.SMSConfig) (domain.SMSVerifier, error)
}

type runtimeBuildInputs struct {
	env     string
	cfg     *config.Config
	logger  logging.Logger
	initCtx context.Context
	now     func() time.Time
	isProd  bool
	tuning  runtimeTuning
}

type runtimeStoreBundle struct {
	accountStore    domain.AccountStore
	packageStore    domain.PackageStore
	settlementStore domain.SettlementStore
	earningStore    domain.EarningStore
	auditStore      audit.AuditStore
	alertService    *auditservice.AlertService
	fkValidator     *repository.ForeignKeyValidator
	tokenStatusRepo *repository.TokenStatusRepository
	idempotencyRepo *repository.IdempotencyRepository
}

type runtimeSecurityBundle struct {
	authMiddleware       *middleware.AuthMiddleware
	revocationSubscriber revocationSubscriber
}

type runtimeAPIBundle struct {
	supplyAPI       *httpapi.SupplyAPI
	alertAPI        *httpapi.AlertAPI
	iamHandler      routeRegistrar
	rateLimitConfig *middleware.RateLimitConfig
}

type runtimeHealthChecks struct {
	DBHealthCheck    func(context.Context) error
	RedisHealthCheck func(context.Context) error
}

type runtimeHTTPView struct {
	env             string
	logger          logging.Logger
	serverConfig    config.ServerConfig
	supplyAPI       *httpapi.SupplyAPI
	alertAPI        *httpapi.AlertAPI
	iamHandler      routeRegistrar
	authMiddleware  *middleware.AuthMiddleware
	rateLimitConfig *middleware.RateLimitConfig
	healthChecks    runtimeHealthChecks
}

// BuildRuntime 构建 supply-api 运行时依赖。
func BuildRuntime(opts RuntimeOptions) (*Runtime, error) {
	return buildRuntimeWithFactory(opts, runtimeFactory{
		newDB:         repository.NewDB,
		newRedisCache: cache.NewRedisCache,
	})
}

func buildRuntimeWithFactory(opts RuntimeOptions, factory runtimeFactory) (*Runtime, error) {
	inputs, err := resolveRuntimeBuildInputs(opts)
	if err != nil {
		return nil, err
	}
	factory = withDefaultRuntimeFactory(factory)

	resources, err := initializeRuntimeExternalResources(inputs, factory)
	if err != nil {
		return nil, err
	}

	storeBundle := buildStoreBundle(resources.db, inputs.logger)
	securityBundle := buildSecurityBundle(inputs.env, inputs.cfg, inputs.logger, storeBundle.auditStore, resources.redisCache, storeBundle.tokenStatusRepo)
	apiBundle, err := buildAPIBundle(inputs.env, inputs.cfg, inputs.now, inputs.tuning, inputs.logger, inputs.isProd, factory, resources.db, storeBundle)
	if err != nil {
		return nil, err
	}

	startupViews := buildRuntimeStartupViews(inputs.env, inputs.logger, inputs.cfg.Server, inputs.tuning, securityBundle, apiBundle)

	return &Runtime{
		resources:    resources,
		startupViews: startupViews,
	}, nil
}

func resolveRuntimeBuildInputs(opts RuntimeOptions) (runtimeBuildInputs, error) {
	if opts.Config == nil {
		return runtimeBuildInputs{}, errors.New("config is required")
	}
	if opts.Logger == nil {
		return runtimeBuildInputs{}, errors.New("logger is required")
	}

	env, err := ResolveEnv(opts.Env)
	if err != nil {
		return runtimeBuildInputs{}, err
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	initCtx := opts.InitContext
	if initCtx == nil {
		initCtx = context.Background()
	}

	return runtimeBuildInputs{
		env:     env,
		cfg:     opts.Config,
		logger:  opts.Logger,
		initCtx: initCtx,
		now:     now,
		isProd:  env == "prod",
		tuning:  defaultRuntimeTuning(),
	}, nil
}

func withDefaultRuntimeFactory(factory runtimeFactory) runtimeFactory {
	if factory.newDB == nil {
		factory.newDB = repository.NewDB
	}
	if factory.newRedisCache == nil {
		factory.newRedisCache = cache.NewRedisCache
	}
	if factory.newSMSVerifier == nil {
		factory.newSMSVerifier = func(config.SMSConfig) (domain.SMSVerifier, error) {
			return nil, nil
		}
	}
	return factory
}

func initializeRuntimeExternalResources(inputs runtimeBuildInputs, factory runtimeFactory) (runtimeExternalResources, error) {
	factory = withDefaultRuntimeFactory(factory)

	db, err := factory.newDB(inputs.initCtx, inputs.cfg.Database)
	if err != nil {
		if inputs.isProd {
			return runtimeExternalResources{}, fmt.Errorf("database unavailable: %w", err)
		}
		warnf(inputs.logger, "failed to connect to database: %v (using in-memory store)", err)
		db = nil
	} else if db != nil {
		infof(inputs.logger, "connected to database at %s:%d", inputs.cfg.Database.Host, inputs.cfg.Database.Port)
	}

	redisCache, err := factory.newRedisCache(inputs.cfg.Redis)
	if err != nil {
		if inputs.isProd {
			warnf(inputs.logger, "redis unavailable at startup: %v", err)
		} else {
			warnf(inputs.logger, "failed to connect to redis: %v (caching disabled)", err)
		}
		redisCache = nil
	} else if redisCache != nil {
		infof(inputs.logger, "connected to redis at %s:%d", inputs.cfg.Redis.Host, inputs.cfg.Redis.Port)
	}

	return buildRuntimeResources(db, redisCache), nil
}

func buildRuntimeResources(db *repository.DB, redisCache *cache.RedisCache) runtimeExternalResources {
	return runtimeExternalResources{
		db:         db,
		redisCache: redisCache,
	}
}

func buildRuntimeStartupViews(
	env string,
	logger logging.Logger,
	serverConfig config.ServerConfig,
	tuning runtimeTuning,
	securityBundle runtimeSecurityBundle,
	apiBundle runtimeAPIBundle,
) runtimeStartupViews {
	return runtimeStartupViews{
		http: runtimeHTTPStartupView{
			env:             env,
			logger:          logger,
			serverConfig:    normalizeServerConfig(serverConfig),
			supplyAPI:       apiBundle.supplyAPI,
			alertAPI:        apiBundle.alertAPI,
			iamHandler:      apiBundle.iamHandler,
			authMiddleware:  securityBundle.authMiddleware,
			rateLimitConfig: apiBundle.rateLimitConfig,
		},
		background: runtimeBackgroundStartupView{
			env:                  env,
			logger:               logger,
			tuning:               tuning,
			revocationSubscriber: securityBundle.revocationSubscriber,
		},
	}
}

func buildStoreBundle(db *repository.DB, logger logging.Logger) runtimeStoreBundle {
	if db != nil {
		bundle := buildDBStoreBundle(db)
		logger.Info("审计存储: 使用PostgreSQL (DB-backed)", nil)
		logger.Info("告警存储: 使用PostgreSQL (DB-backed)", nil)
		logger.Info("外键校验器: 已初始化 (PostgreSQL-backed)", nil)
		return bundle
	}

	bundle := buildMemoryStoreBundle()
	logger.Warn("审计存储使用内存实现 (生产环境不应使用)", nil)
	logger.Warn("告警存储使用内存实现 (仅开发环境允许)", nil)
	logger.Warn("外键校验器未启用 (db不可用)", nil)
	return bundle
}

func buildDBStoreBundle(db *repository.DB) runtimeStoreBundle {
	accountRepo := repository.NewAccountRepository(db.Pool)
	packageRepo := repository.NewPackageRepository(db.Pool)
	settlementRepo := repository.NewSettlementRepository(db.Pool)
	usageRepo := repository.NewUsageRepository(db.Pool)

	return runtimeStoreBundle{
		accountStore:    adapter.NewDBAccountStore(accountRepo),
		packageStore:    adapter.NewDBPackageStore(packageRepo),
		settlementStore: adapter.NewDBSettlementStore(settlementRepo, accountRepo, db.Pool),
		earningStore:    adapter.NewDBEarningStore(usageRepo),
		auditStore:      audit.NewPostgresAuditStore(auditrepo.NewPostgresAuditRepository(db.Pool)),
		alertService:    auditservice.NewAlertService(auditrepo.NewPostgresAlertRepository(db.Pool)),
		fkValidator:     repository.NewForeignKeyValidator(db.Pool),
		tokenStatusRepo: repository.NewTokenStatusRepository(db.Pool),
		idempotencyRepo: repository.NewIdempotencyRepository(db.Pool),
	}
}

func buildMemoryStoreBundle() runtimeStoreBundle {
	return runtimeStoreBundle{
		accountStore:    adapter.NewInMemoryAccountStoreAdapter(),
		packageStore:    adapter.NewInMemoryPackageStoreAdapter(),
		settlementStore: adapter.NewInMemorySettlementStoreAdapter(),
		earningStore:    adapter.NewInMemoryEarningStoreAdapter(),
		auditStore:      audit.NewMemoryAuditStore(),
		alertService:    auditservice.NewAlertService(auditservice.NewInMemoryAlertStore()),
	}
}

func buildSecurityBundle(
	env string,
	cfg *config.Config,
	logger logging.Logger,
	auditStore audit.AuditStore,
	redisCache *cache.RedisCache,
	tokenStatusRepo *repository.TokenStatusRepository,
) runtimeSecurityBundle {
	tokenCache := middleware.NewTokenCache()
	var tokenBackend middleware.TokenStatusBackend
	var revocationSubscriber revocationSubscriber

	if tokenStatusRepo != nil {
		dbTokenBackend := middleware.NewDBTokenStatusBackend(tokenStatusRepo, redisCache, cfg.Token.RevocationCacheTTL)
		tokenBackend = dbTokenBackend
		revocationSubscriber = dbTokenBackend
		logger.Info("Token状态后端: 使用PostgreSQL (DB-backed)", nil)
	} else {
		tokenBackend = adapter.NewMemoryTokenBackend()
		logger.Warn("Token状态后端使用内存实现 (生产环境不应使用)", nil)
	}

	return runtimeSecurityBundle{
		authMiddleware: middleware.NewAuthMiddleware(middleware.AuthConfig{
			SecretKey:                 cfg.Token.SecretKey,
			PublicKey:                 cfg.Token.PublicKey,
			Algorithm:                 cfg.Token.Algorithm,
			Issuer:                    cfg.Token.Issuer,
			CacheTTL:                  cfg.Token.RevocationCacheTTL,
			Enabled:                   env != "dev",
			BruteForceMaxAttempts:     5,             // MED-12: 暴力破解保护，默认5次失败后锁定
			BruteForceLockoutDuration: 15 * time.Minute, // MED-12: 默认锁定15分钟
		}, tokenCache, tokenBackend, adapter.NewAuditEmitterAdapter(auditStore)),
		revocationSubscriber: revocationSubscriber,
	}
}

func buildAPIBundle(
	env string,
	cfg *config.Config,
	now func() time.Time,
	tuning runtimeTuning,
	logger logging.Logger,
	isProd bool,
	factory runtimeFactory,
	db *repository.DB,
	storeBundle runtimeStoreBundle,
) (runtimeAPIBundle, error) {
	_ = domain.NewInvariantChecker(storeBundle.accountStore, storeBundle.packageStore, storeBundle.settlementStore)

	accountService := domain.NewAccountService(storeBundle.accountStore, storeBundle.auditStore)
	packageService := domain.NewPackageService(storeBundle.packageStore, storeBundle.accountStore, storeBundle.auditStore)
	settlementService := domain.NewSettlementService(storeBundle.settlementStore, storeBundle.earningStore, storeBundle.auditStore)
	withdrawEnabled := false
	earningService := domain.NewEarningService(storeBundle.earningStore)
	var iamAPI routeRegistrar

	var idempotencyMiddleware *middleware.IdempotencyMiddleware
	if storeBundle.idempotencyRepo != nil {
		idempotencyMiddleware = middleware.NewIdempotencyMiddleware(storeBundle.idempotencyRepo, middleware.IdempotencyConfig{
			TTL:     tuning.idempotencyTTL,
			Enabled: env != "dev",
		})
		logger.Info("幂等中间件已启用（DB-backed）", nil)
	} else {
		if isProd {
			return runtimeAPIBundle{}, errors.New("idempotency repository unavailable")
		}
		logger.Warn("幂等中间件未启用（db或repo不可用）- 需要幂等的写接口将返回 503", nil)
	}

	rateLimitConfig := middleware.DefaultRateLimitConfig()
	rateLimitConfig.Enabled = env != "dev"
	logger.Info("限流中间件已初始化", nil)

	if cfg.Settlement.WithdrawEnabled {
		if !cfg.SMS.IsReadyForWithdraw() {
			logger.Warn("提现能力未启用：SMS is not ready", nil)
		} else {
			smsVerifier, err := factory.newSMSVerifier(cfg.SMS)
			if err != nil {
				return runtimeAPIBundle{}, fmt.Errorf("failed to initialize SMS verifier: %w", err)
			}
			if smsVerifier == nil {
				logger.Warn("提现能力未启用：SMS verifier is not wired", nil)
			} else {
				settlementService = domain.NewSettlementServiceWithSMS(storeBundle.settlementStore, storeBundle.earningStore, storeBundle.auditStore, smsVerifier)
				withdrawEnabled = true
				logger.Info("提现能力已启用（SMS verifier wired）", nil)
			}
		}
	} else {
		logger.Info("提现能力未启用（settlement.withdraw_enabled=false）", nil)
	}

	if cfg.Server.IAMEnabled {
		if db == nil {
			return runtimeAPIBundle{}, errors.New("iam requires database-backed runtime")
		}
		iamAPI = iamhandler.NewIAMHandler(
			iamservice.NewDatabaseIAMService(
				iamrepo.NewPostgresIAMRepository(db.Pool),
			),
		)
		logger.Info("IAM 路由已启用（DB-backed）", nil)
	} else {
		logger.Info("IAM 路由未启用（server.iam_enabled=false）", nil)
	}

	supplyAPI, err := httpapi.NewSupplyAPI(
		accountService,
		packageService,
		settlementService,
		earningService,
		idempotencyMiddleware,
		storeBundle.auditStore,
		storeBundle.fkValidator,
		cfg.Server.DefaultSupplierID,
		cfg.Server.StatementBaseURL,
		now,
	)
	if err != nil {
		return runtimeAPIBundle{}, fmt.Errorf("failed to initialize supply api: %w", err)
	}
	supplyAPI.SetWithdrawEnabled(withdrawEnabled)

	alertAPI, err := httpapi.NewAlertAPI(storeBundle.alertService)
	if err != nil {
		return runtimeAPIBundle{}, fmt.Errorf("failed to initialize alert api: %w", err)
	}

	return runtimeAPIBundle{
		supplyAPI:       supplyAPI,
		alertAPI:        alertAPI,
		iamHandler:      iamAPI,
		rateLimitConfig: rateLimitConfig,
	}, nil
}

func defaultRuntimeTuning() runtimeTuning {
	return runtimeTuning{
		outboxStreamName:             "supply:outbox:stream",
		outboxConsumerGroup:          "outbox-processor",
		idempotencyTTL:               24 * time.Hour,
		partitionMaintenanceInterval: time.Hour,
		compensationCheckInterval:    5 * time.Minute,
		partitionedTables: []string{
			"audit_events",
			"supply_usage_records",
			"supply_idempotency_records",
		},
	}
}

// BuildServer 使用运行时依赖构建 HTTP server。
func (r *Runtime) BuildServer() (*http.Server, error) {
	view, err := buildRuntimeHTTPView(r)
	if err != nil {
		return nil, err
	}
	return BuildServer(adaptRuntimeHTTPViewToBuildServerOptions(view))
}

func resolveRuntimeHealthChecks(runtime *Runtime) runtimeHealthChecks {
	var checks runtimeHealthChecks
	if runtime == nil {
		return checks
	}
	if runtime.resources.db != nil {
		checks.DBHealthCheck = runtime.resources.db.HealthCheck
	}
	if runtime.resources.redisCache != nil {
		checks.RedisHealthCheck = runtime.resources.redisCache.HealthCheck
	}
	return checks
}

func buildRuntimeHTTPView(runtime *Runtime) (runtimeHTTPView, error) {
	if runtime == nil {
		return runtimeHTTPView{}, errors.New("runtime is required")
	}

	return runtimeHTTPView{
		env:             runtime.startupViews.http.env,
		logger:          runtime.startupViews.http.logger,
		serverConfig:    runtime.startupViews.http.serverConfig,
		supplyAPI:       runtime.startupViews.http.supplyAPI,
		alertAPI:        runtime.startupViews.http.alertAPI,
		iamHandler:      runtime.startupViews.http.iamHandler,
		authMiddleware:  runtime.startupViews.http.authMiddleware,
		rateLimitConfig: runtime.startupViews.http.rateLimitConfig,
		healthChecks:    resolveRuntimeHealthChecks(runtime),
	}, nil
}

func adaptRuntimeHTTPViewToBuildServerOptions(view runtimeHTTPView) BuildServerOptions {
	return BuildServerOptions{
		Env:              view.env,
		ServerConfig:     view.serverConfig,
		Logger:           view.logger,
		SupplyAPI:        view.supplyAPI,
		AlertAPI:         view.alertAPI,
		IAMHandler:       view.iamHandler,
		AuthMiddleware:   view.authMiddleware,
		RateLimitConfig:  view.rateLimitConfig,
		DBHealthCheck:    view.healthChecks.DBHealthCheck,
		RedisHealthCheck: view.healthChecks.RedisHealthCheck,
	}
}

func adaptRuntimeToBuildServerOptions(runtime *Runtime) (BuildServerOptions, error) {
	view, err := buildRuntimeHTTPView(runtime)
	if err != nil {
		return BuildServerOptions{}, err
	}
	return adaptRuntimeHTTPViewToBuildServerOptions(view), nil
}

// Close 关闭运行时持有的外部资源。
func (r *Runtime) Close() {
	if r == nil {
		return
	}
	if r.resources.redisCache != nil {
		_ = r.resources.redisCache.Close()
	}
	if r.resources.db != nil {
		r.resources.db.Close()
	}
}

// ShutdownTimeout 返回服务优雅关闭超时时间。
func (r *Runtime) ShutdownTimeout() time.Duration {
	if r == nil {
		return 0
	}
	return r.startupViews.http.serverConfig.ShutdownTimeout
}

func ResolveEnv(env string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(env))
	if normalized == "" {
		return "dev", nil
	}
	switch normalized {
	case "dev", "staging", "prod":
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported env %q", env)
	}
}

func infof(logger logging.Logger, format string, args ...any) {
	logger.Info(fmt.Sprintf(format, args...), nil)
}

func warnf(logger logging.Logger, format string, args ...any) {
	logger.Warn(fmt.Sprintf(format, args...), nil)
}
