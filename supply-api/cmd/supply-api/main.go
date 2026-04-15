package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lijiaoqiao/supply-api/internal/adapter"
	"lijiaoqiao/supply-api/internal/audit"
	auditrepo "lijiaoqiao/supply-api/internal/audit/repository"
	auditservice "lijiaoqiao/supply-api/internal/audit/service"
	"lijiaoqiao/supply-api/internal/cache"
	"lijiaoqiao/supply-api/internal/compensation"
	"lijiaoqiao/supply-api/internal/config"
	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/httpapi"
	"lijiaoqiao/supply-api/internal/messaging"
	"lijiaoqiao/supply-api/internal/middleware"
	"lijiaoqiao/supply-api/internal/outbox"
	"lijiaoqiao/supply-api/internal/pkg/logging"
	"lijiaoqiao/supply-api/internal/repository"
)

func main() {
	// 解析命令行参数
	env := flag.String("env", "dev", "environment: dev/staging/prod")
	configPath := flag.String("config", "", "config file path")
	flag.Parse()

	// 确定配置文件路径
	if *configPath == "" {
		*configPath = "./config/config." + *env + ".yaml"
	}

	// P1-010修复: 初始化结构化日志
	jsonLogger := logging.NewLogger("supply-api", logging.LogLevelInfo)

	// 加载配置
	cfg, err := config.LoadFromPath(*env, *configPath)
	if err != nil {
		jsonLogger.Fatalf("failed to load config: %v", err)
	}

	jsonLogger.Infof("starting supply-api in %s mode", *env)
	isProd := *env == "prod"
	rootCtx, stop := context.WithCancel(context.Background())
	defer stop()

	initCtx, initCancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer initCancel()

	// 初始化数据库连接
	db, err := repository.NewDB(initCtx, cfg.Database)
	if err != nil {
		if isProd {
			jsonLogger.Fatalf("production startup requirement failed: database unavailable: %v", err)
		}
		jsonLogger.Infof("warning: failed to connect to database: %v (using in-memory store)", err)
		db = nil
	} else {
		jsonLogger.Infof("connected to database at %s:%d", cfg.Database.Host, cfg.Database.Port)
		defer db.Close()
	}

	// 初始化Redis缓存
	redisCache, err := cache.NewRedisCache(cfg.Redis)
	if err != nil {
		if isProd {
			jsonLogger.Infof("warning: redis unavailable at startup: %v", err)
		} else {
			jsonLogger.Infof("warning: failed to connect to redis: %v (caching disabled)", err)
		}
		redisCache = nil
	} else {
		jsonLogger.Infof("connected to redis at %s:%d", cfg.Redis.Host, cfg.Redis.Port)
		defer redisCache.Close()
	}

	// 初始化存储层
	var accountStore domain.AccountStore
	var packageStore domain.PackageStore
	var settlementStore domain.SettlementStore
	var earningStore domain.EarningStore
	var auditRepo *auditrepo.PostgresAuditRepository
	var tokenStatusRepo *repository.TokenStatusRepository

	if db != nil {
		// 使用PostgreSQL存储
		accountRepo := repository.NewAccountRepository(db.Pool)
		packageRepo := repository.NewPackageRepository(db.Pool)
		settlementRepo := repository.NewSettlementRepository(db.Pool)
		usageRepo := repository.NewUsageRepository(db.Pool)
		idempotencyRepo := repository.NewIdempotencyRepository(db.Pool)
		auditRepo = auditrepo.NewPostgresAuditRepository(db.Pool)
		tokenStatusRepo = repository.NewTokenStatusRepository(db.Pool)

		// 创建DB-backed存储（使用adapter包中的类型）
		accountStore = adapter.NewDBAccountStore(accountRepo)
		packageStore = adapter.NewDBPackageStore(packageRepo)
		settlementStore = adapter.NewDBSettlementStore(settlementRepo, accountRepo, db.Pool)
		earningStore = adapter.NewDBEarningStore(usageRepo)

		_ = idempotencyRepo // 用于幂等中间件
	} else {
		// 回退到内存存储（开发模式）
		accountStore = adapter.NewInMemoryAccountStoreAdapter()
		packageStore = adapter.NewInMemoryPackageStoreAdapter()
		settlementStore = adapter.NewInMemorySettlementStoreAdapter()
		earningStore = adapter.NewInMemoryEarningStoreAdapter()
	}

	// P0-R08修复: 初始化审计存储 - 使用DB-backed实现
	var auditStore audit.AuditStore
	if auditRepo != nil {
		auditStore = audit.NewPostgresAuditStore(auditRepo)
		jsonLogger.Info("审计存储: 使用PostgreSQL (DB-backed)")
	} else {
		auditStore = audit.NewMemoryAuditStore()
		jsonLogger.Info("警告: 审计存储使用内存实现 (生产环境不应使用)")
	}

	var alertStore auditservice.AlertStoreInterface
	if db != nil {
		alertStore = auditrepo.NewPostgresAlertRepository(db.Pool)
		jsonLogger.Info("告警存储: 使用PostgreSQL (DB-backed)")
	} else {
		alertStore = auditservice.NewInMemoryAlertStore()
		jsonLogger.Info("警告: 告警存储使用内存实现 (仅开发环境允许)")
	}
	alertService := auditservice.NewAlertService(alertStore)

	// P0-09修复: 初始化外键校验器
	var fkValidator *repository.ForeignKeyValidator
	if db != nil {
		fkValidator = repository.NewForeignKeyValidator(db.Pool)
		jsonLogger.Info("外键校验器: 已初始化 (PostgreSQL-backed)")
	} else {
		jsonLogger.Info("警告: 外键校验器未启用 (db不可用)")
	}

	// 初始化不变量检查器
	invariantChecker := domain.NewInvariantChecker(accountStore, packageStore, settlementStore)
	_ = invariantChecker // 用于业务逻辑校验

	// 初始化领域服务
	accountService := domain.NewAccountService(accountStore, auditStore)
	packageService := domain.NewPackageService(packageStore, accountStore, auditStore)
	settlementService := domain.NewSettlementService(settlementStore, earningStore, auditStore)
	earningService := domain.NewEarningService(earningStore)

	// 初始化幂等仓储
	var idempotencyRepo *repository.IdempotencyRepository
	if db != nil {
		idempotencyRepo = repository.NewIdempotencyRepository(db.Pool)
		_ = idempotencyRepo // idempotencyRepo 会在下面初始化幂等中间件时使用
	}

	// 初始化Token缓存
	tokenCache := middleware.NewTokenCache()
	if redisCache != nil {
		// 可以使用Redis缓存
	}

	// 初始化token状态后端（P0-03修复: 使用DB-backed实现）
	var tokenBackend middleware.TokenStatusBackend
	if tokenStatusRepo != nil {
		tokenBackend = middleware.NewDBTokenStatusBackend(tokenStatusRepo, redisCache, cfg.Token.RevocationCacheTTL)
		jsonLogger.Info("Token状态后端: 使用PostgreSQL (DB-backed)")

		// 启动主动吊销订阅机制（仅在Redis可用时）
		if redisCache != nil {
			if dbTokenBackend, ok := tokenBackend.(*middleware.DBTokenStatusBackend); ok {
				if err := dbTokenBackend.StartRevocationSubscriber(rootCtx); err != nil {
					jsonLogger.Infof("警告: 启动主动吊销订阅失败: %v", err)
				} else {
					jsonLogger.Info("主动吊销机制: 已启动 (Redis Pub/Sub)")
				}
			}
		}
	} else {
		tokenBackend = adapter.NewMemoryTokenBackend()
		jsonLogger.Info("警告: Token状态后端使用内存实现 (生产环境不应使用)")
	}

	// 初始化审计事件适配器（NEW-P1-03修复）
	auditEmitter := adapter.NewAuditEmitterAdapter(auditStore)

	// 初始化鉴权中间件
	authConfig := middleware.AuthConfig{
		SecretKey: cfg.Token.SecretKey,
		PublicKey: cfg.Token.PublicKey,
		Algorithm: cfg.Token.Algorithm,
		Issuer:    cfg.Token.Issuer,
		CacheTTL:  cfg.Token.RevocationCacheTTL,
		Enabled:   *env != "dev", // 开发模式禁用鉴权
	}
	authMiddleware := middleware.NewAuthMiddleware(authConfig, tokenCache, tokenBackend, auditEmitter)

	// 初始化幂等中间件。
	// 当仓储不可用时，相关写接口会通过统一错误码返回 503，而不是切回其他实现路径。
	var idempotencyMiddleware *middleware.IdempotencyMiddleware
	if db != nil && idempotencyRepo != nil {
		idempotencyMiddleware = middleware.NewIdempotencyMiddleware(idempotencyRepo, middleware.IdempotencyConfig{
			TTL:     24 * time.Hour,
			Enabled: *env != "dev",
		})
		jsonLogger.Info("幂等中间件已启用（DB-backed）")
	} else {
		if isProd {
			jsonLogger.Fatalf("production startup requirement failed: idempotency repository unavailable")
		}
		jsonLogger.Info("警告：幂等中间件未启用（db或repo不可用）- 需要幂等的写接口将返回 503")
	}

	// P0-05修复: 初始化限流中间件
	rateLimitConfig := middleware.DefaultRateLimitConfig()
	rateLimitConfig.Enabled = *env != "dev" // 生产环境启用
	jsonLogger.Info("限流中间件已初始化")

	// 初始化HTTP API处理器
	// P0-P4修复: 使用DB-backed幂等中间件替代内联幂等存储
	api := httpapi.NewSupplyAPI(
		accountService,
		packageService,
		settlementService,
		earningService,
		idempotencyMiddleware, // 使用幂等中间件（DB-backed）
		auditStore,
		fkValidator, // P0-09修复: 外键校验器
		cfg.Server.DefaultSupplierID,
		cfg.Server.StatementBaseURL,
		time.Now,
	)
	api.SetWithdrawEnabled(cfg.Settlement.WithdrawEnabled)

	// 创建路由器
	mux := http.NewServeMux()

	// P1-007修复: 统一健康检查实现，使用HealthHandler代替重复的inline handlers
	var dbHealthCheck func(ctx context.Context) error
	var redisHealthCheck func(ctx context.Context) error
	if db != nil {
		dbHealthCheck = db.HealthCheck
	}
	if redisCache != nil {
		redisHealthCheck = redisCache.HealthCheck
	}
	healthHandler := httpapi.NewHealthHandlerWithDefaults(dbHealthCheck, redisHealthCheck)
	mux.HandleFunc("/actuator/health", healthHandler.ServeHealth)
	mux.HandleFunc("/actuator/health/live", healthHandler.ServeLiveness)
	mux.HandleFunc("/actuator/health/ready", healthHandler.ServeReadiness)

	// 注册API路由
	api.Register(mux)

	// 注册告警API路由
	alertAPI, err := httpapi.NewAlertAPI(alertService)
	if err != nil {
		jsonLogger.Fatalf("failed to initialize alert api: %v", err)
	}
	alertAPI.Register(mux)

	// 应用中间件链路
	// 1. RequestID - 请求追踪
	// 2. Recovery - Panic恢复
	// 3. Logging - 请求日志
	// 4. Tracing - W3C Trace Context (P1-006)
	// 5. QueryKeyReject - 拒绝外部query key (M-016)
	// 6. BearerExtract - Bearer Token提取
	// 7. TokenVerify - JWT校验
	// 8. RateLimit - 限流 (P0-05)
	// 注：幂等写路径由 SupplyAPI 内部统一经 IdempotencyMiddleware 包装，不额外挂到全局 mux。

	var handler http.Handler = mux
	handler = middleware.RequestID(handler)
	handler = middleware.Recovery(handler)
	handler = middleware.Logging(handler, jsonLogger) // P1-010: 使用结构化JSON日志
	handler = middleware.TracingMiddleware(handler)   // P1-006: W3C Trace Context中间件

	// 生产环境启用安全中间件
	if *env != "dev" {
		// 包装顺序与请求执行顺序相反，这里从内到外构建，保证实际执行顺序为：
		// QueryKeyReject -> BearerExtract -> TokenVerify -> RateLimit
		handler = middleware.NewRateLimitHandler(rateLimitConfig, handler)
		handler = authMiddleware.TokenVerifyMiddleware(handler)
		handler = authMiddleware.BearerExtractMiddleware(handler)
		handler = authMiddleware.QueryKeyRejectMiddleware(handler)
	}

	// 创建HTTP服务器
	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.Server.ReadTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	serverErrCh := make(chan error, 1)
	go func() {
		jsonLogger.Infof("starting HTTP server on %s", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	// P0-06修复: 启动OutboxProcessor（仅在DB可用时）
	var outboxProcessor *outbox.OutboxProcessorRunner
	if db != nil {
		outboxRepo := repository.NewOutboxRepository(db.Pool)
		var msgBroker messaging.MessageBroker
		if redisCache != nil {
			// 使用Redis Streams作为消息代理
			redisClient := redisCache.GetClient()
			msgBroker = messaging.NewOutboxMessageBroker(redisClient, "supply:outbox:stream", "outbox-processor")
		}
		if msgBroker == nil {
			if isProd {
				jsonLogger.Fatalf("production startup requirement failed: outbox message broker unavailable")
			}
			jsonLogger.Info("警告: OutboxProcessor未启动 (message broker不可用)")
		} else {
			stats := &messaging.NoOpOutboxStats{}
			outboxProcessor = outbox.NewOutboxProcessorRunner(outboxRepo, msgBroker, stats)
			go outboxProcessor.Start(rootCtx)
			jsonLogger.Info("OutboxProcessor已启动")
		}

		// 分区维护：确保未来分区已创建
		partitionManager := repository.NewPartitionManager(db.Pool)
		if err := partitionManager.EnsureFuturePartitions(initCtx); err != nil {
			jsonLogger.Infof("警告: 预创建未来分区失败: %v", err)
		} else {
			jsonLogger.Info("分区管理: 未来分区已确保存在")
		}

		// 启动后台分区维护goroutine（每小时检查一次）
		go func() {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-rootCtx.Done():
					return
				case <-ticker.C:
					if err := partitionManager.EnsureFuturePartitions(context.Background()); err != nil {
						jsonLogger.Infof("分区维护: 预创建未来分区失败: %v", err)
					}
					// 清理过期分区（仅在需要时）
					for _, tableName := range []string{"audit_events", "supply_usage_records", "supply_idempotency_records"} {
						if _, err := partitionManager.DropOldPartitions(context.Background(), tableName); err != nil {
							jsonLogger.Infof("分区维护: 清理过期分区失败 (%s): %v", tableName, err)
						}
					}
				}
			}
		}()

		// P0-07修复: 初始化批量补偿处理器
		compensationStore := domain.NewSQLCompensationStore(db.Pool)
		compensationStats := &domain.NoOpCompensationStats{}
		compensationExecutor := compensation.NewDefaultCompensationExecutor()
		compensationProcessor := domain.NewCompensationProcessor(compensationStore, compensationExecutor, compensationStats)
		jsonLogger.Info("批量补偿处理器: 已初始化")

		// 启动后台补偿处理goroutine
		compensationProcessor.StartBackgroundWorker(rootCtx, 5*time.Minute)
		jsonLogger.Info("批量补偿处理器: 后台worker已启动 (每5分钟检查一次)")
	}

	// 优雅关闭
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigCh:
		jsonLogger.Infof("received signal %s", sig)
	case err := <-serverErrCh:
		jsonLogger.Fatalf("server failed: %v", err)
	}

	stop()

	jsonLogger.Info("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		jsonLogger.Infof("graceful shutdown failed: %v", err)
	}

	jsonLogger.Info("shutdown complete")
}
