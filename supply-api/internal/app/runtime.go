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
	env                  string
	logger               logging.Logger
	now                  func() time.Time
	tuning               runtimeTuning
	serverConfig         config.ServerConfig
	db                   *repository.DB
	redisCache           *cache.RedisCache
	supplyAPI            *httpapi.SupplyAPI
	alertAPI             *httpapi.AlertAPI
	authMiddleware       *middleware.AuthMiddleware
	rateLimitConfig      *middleware.RateLimitConfig
	revocationSubscriber revocationSubscriber
}

type revocationSubscriber interface {
	StartRevocationSubscriber(ctx context.Context) error
}

type runtimeFactory struct {
	newDB         func(ctx context.Context, cfg config.DatabaseConfig) (*repository.DB, error)
	newRedisCache func(cfg config.RedisConfig) (*cache.RedisCache, error)
}

// BuildRuntime 构建 supply-api 运行时依赖。
func BuildRuntime(opts RuntimeOptions) (*Runtime, error) {
	return buildRuntimeWithFactory(opts, runtimeFactory{
		newDB:         repository.NewDB,
		newRedisCache: cache.NewRedisCache,
	})
}

func buildRuntimeWithFactory(opts RuntimeOptions, factory runtimeFactory) (*Runtime, error) {
	if opts.Config == nil {
		return nil, errors.New("config is required")
	}
	if opts.Logger == nil {
		return nil, errors.New("logger is required")
	}

	if factory.newDB == nil {
		factory.newDB = repository.NewDB
	}
	if factory.newRedisCache == nil {
		factory.newRedisCache = cache.NewRedisCache
	}

	env, err := ResolveEnv(opts.Env)
	if err != nil {
		return nil, err
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	initCtx := opts.InitContext
	if initCtx == nil {
		initCtx = context.Background()
	}

	isProd := env == "prod"
	tuning := defaultRuntimeTuning()

	db, err := factory.newDB(initCtx, opts.Config.Database)
	if err != nil {
		if isProd {
			return nil, fmt.Errorf("database unavailable: %w", err)
		}
		warnf(opts.Logger, "failed to connect to database: %v (using in-memory store)", err)
		db = nil
	} else if db != nil {
		infof(opts.Logger, "connected to database at %s:%d", opts.Config.Database.Host, opts.Config.Database.Port)
	}

	redisCache, err := factory.newRedisCache(opts.Config.Redis)
	if err != nil {
		if isProd {
			warnf(opts.Logger, "redis unavailable at startup: %v", err)
		} else {
			warnf(opts.Logger, "failed to connect to redis: %v (caching disabled)", err)
		}
		redisCache = nil
	} else if redisCache != nil {
		infof(opts.Logger, "connected to redis at %s:%d", opts.Config.Redis.Host, opts.Config.Redis.Port)
	}

	var accountStore domain.AccountStore
	var packageStore domain.PackageStore
	var settlementStore domain.SettlementStore
	var earningStore domain.EarningStore
	var auditRepository *auditrepo.PostgresAuditRepository
	var tokenStatusRepo *repository.TokenStatusRepository
	var idempotencyRepo *repository.IdempotencyRepository

	if db != nil {
		accountRepo := repository.NewAccountRepository(db.Pool)
		packageRepo := repository.NewPackageRepository(db.Pool)
		settlementRepo := repository.NewSettlementRepository(db.Pool)
		usageRepo := repository.NewUsageRepository(db.Pool)
		idempotencyRepo = repository.NewIdempotencyRepository(db.Pool)
		auditRepository = auditrepo.NewPostgresAuditRepository(db.Pool)
		tokenStatusRepo = repository.NewTokenStatusRepository(db.Pool)

		accountStore = adapter.NewDBAccountStore(accountRepo)
		packageStore = adapter.NewDBPackageStore(packageRepo)
		settlementStore = adapter.NewDBSettlementStore(settlementRepo, accountRepo, db.Pool)
		earningStore = adapter.NewDBEarningStore(usageRepo)
	} else {
		accountStore = adapter.NewInMemoryAccountStoreAdapter()
		packageStore = adapter.NewInMemoryPackageStoreAdapter()
		settlementStore = adapter.NewInMemorySettlementStoreAdapter()
		earningStore = adapter.NewInMemoryEarningStoreAdapter()
	}

	var auditStore audit.AuditStore
	if auditRepository != nil {
		auditStore = audit.NewPostgresAuditStore(auditRepository)
		opts.Logger.Info("审计存储: 使用PostgreSQL (DB-backed)", nil)
	} else {
		auditStore = audit.NewMemoryAuditStore()
		opts.Logger.Warn("审计存储使用内存实现 (生产环境不应使用)", nil)
	}

	var alertStore auditservice.AlertStoreInterface
	if db != nil {
		alertStore = auditrepo.NewPostgresAlertRepository(db.Pool)
		opts.Logger.Info("告警存储: 使用PostgreSQL (DB-backed)", nil)
	} else {
		alertStore = auditservice.NewInMemoryAlertStore()
		opts.Logger.Warn("告警存储使用内存实现 (仅开发环境允许)", nil)
	}
	alertService := auditservice.NewAlertService(alertStore)

	var fkValidator *repository.ForeignKeyValidator
	if db != nil {
		fkValidator = repository.NewForeignKeyValidator(db.Pool)
		opts.Logger.Info("外键校验器: 已初始化 (PostgreSQL-backed)", nil)
	} else {
		opts.Logger.Warn("外键校验器未启用 (db不可用)", nil)
	}

	_ = domain.NewInvariantChecker(accountStore, packageStore, settlementStore)

	accountService := domain.NewAccountService(accountStore, auditStore)
	packageService := domain.NewPackageService(packageStore, accountStore, auditStore)
	settlementService := domain.NewSettlementService(settlementStore, earningStore, auditStore)
	earningService := domain.NewEarningService(earningStore)

	tokenCache := middleware.NewTokenCache()
	var tokenBackend middleware.TokenStatusBackend
	var revocationSubscriber revocationSubscriber
	if tokenStatusRepo != nil {
		dbTokenBackend := middleware.NewDBTokenStatusBackend(tokenStatusRepo, redisCache, opts.Config.Token.RevocationCacheTTL)
		tokenBackend = dbTokenBackend
		revocationSubscriber = dbTokenBackend
		opts.Logger.Info("Token状态后端: 使用PostgreSQL (DB-backed)", nil)
	} else {
		tokenBackend = adapter.NewMemoryTokenBackend()
		opts.Logger.Warn("Token状态后端使用内存实现 (生产环境不应使用)", nil)
	}

	auditEmitter := adapter.NewAuditEmitterAdapter(auditStore)
	authMiddleware := middleware.NewAuthMiddleware(middleware.AuthConfig{
		SecretKey: opts.Config.Token.SecretKey,
		PublicKey: opts.Config.Token.PublicKey,
		Algorithm: opts.Config.Token.Algorithm,
		Issuer:    opts.Config.Token.Issuer,
		CacheTTL:  opts.Config.Token.RevocationCacheTTL,
		Enabled:   env != "dev",
	}, tokenCache, tokenBackend, auditEmitter)

	var idempotencyMiddleware *middleware.IdempotencyMiddleware
	if db != nil && idempotencyRepo != nil {
		idempotencyMiddleware = middleware.NewIdempotencyMiddleware(idempotencyRepo, middleware.IdempotencyConfig{
			TTL:     tuning.idempotencyTTL,
			Enabled: env != "dev",
		})
		opts.Logger.Info("幂等中间件已启用（DB-backed）", nil)
	} else {
		if isProd {
			return nil, errors.New("idempotency repository unavailable")
		}
		opts.Logger.Warn("幂等中间件未启用（db或repo不可用）- 需要幂等的写接口将返回 503", nil)
	}

	rateLimitConfig := middleware.DefaultRateLimitConfig()
	rateLimitConfig.Enabled = env != "dev"
	opts.Logger.Info("限流中间件已初始化", nil)

	supplyAPI, err := httpapi.NewSupplyAPI(
		accountService,
		packageService,
		settlementService,
		earningService,
		idempotencyMiddleware,
		auditStore,
		fkValidator,
		opts.Config.Server.DefaultSupplierID,
		opts.Config.Server.StatementBaseURL,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize supply api: %w", err)
	}
	supplyAPI.SetWithdrawEnabled(opts.Config.Settlement.WithdrawEnabled)

	alertAPI, err := httpapi.NewAlertAPI(alertService)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize alert api: %w", err)
	}

	return &Runtime{
		env:                  env,
		logger:               opts.Logger,
		now:                  now,
		tuning:               tuning,
		serverConfig:         normalizeServerConfig(opts.Config.Server),
		db:                   db,
		redisCache:           redisCache,
		supplyAPI:            supplyAPI,
		alertAPI:             alertAPI,
		authMiddleware:       authMiddleware,
		rateLimitConfig:      rateLimitConfig,
		revocationSubscriber: revocationSubscriber,
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
	if r == nil {
		return nil, errors.New("runtime is required")
	}

	var dbHealthCheck func(context.Context) error
	var redisHealthCheck func(context.Context) error
	if r.db != nil {
		dbHealthCheck = r.db.HealthCheck
	}
	if r.redisCache != nil {
		redisHealthCheck = r.redisCache.HealthCheck
	}

	return BuildServer(BuildServerOptions{
		Env:              r.env,
		ServerConfig:     r.serverConfig,
		Logger:           r.logger,
		SupplyAPI:        r.supplyAPI,
		AlertAPI:         r.alertAPI,
		AuthMiddleware:   r.authMiddleware,
		RateLimitConfig:  r.rateLimitConfig,
		DBHealthCheck:    dbHealthCheck,
		RedisHealthCheck: redisHealthCheck,
	})
}

// Close 关闭运行时持有的外部资源。
func (r *Runtime) Close() {
	if r == nil {
		return
	}
	if r.redisCache != nil {
		_ = r.redisCache.Close()
	}
	if r.db != nil {
		r.db.Close()
	}
}

// ShutdownTimeout 返回服务优雅关闭超时时间。
func (r *Runtime) ShutdownTimeout() time.Duration {
	if r == nil {
		return 0
	}
	return r.serverConfig.ShutdownTimeout
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
