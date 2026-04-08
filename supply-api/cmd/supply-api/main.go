package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lijiaoqiao/supply-api/internal/audit"
	auditrepo "lijiaoqiao/supply-api/internal/audit/repository"
	"lijiaoqiao/supply-api/internal/cache"
	"lijiaoqiao/supply-api/internal/config"
	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/httpapi"
	"lijiaoqiao/supply-api/internal/messaging"
	"lijiaoqiao/supply-api/internal/middleware"
	"lijiaoqiao/supply-api/internal/pkg/logging"
	"lijiaoqiao/supply-api/internal/repository"
	"lijiaoqiao/supply-api/internal/storage"
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

	// 加载配置
	cfg, err := config.Load(*env)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Printf("starting supply-api in %s mode", *env)

	// P1-010修复: 初始化结构化日志
	jsonLogger := logging.NewLogger("supply-api", logging.LogLevelInfo)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 初始化数据库连接
	db, err := repository.NewDB(ctx, cfg.Database)
	if err != nil {
		log.Printf("warning: failed to connect to database: %v (using in-memory store)", err)
		db = nil
	} else {
		log.Printf("connected to database at %s:%d", cfg.Database.Host, cfg.Database.Port)
		defer db.Close()
	}

	// 初始化Redis缓存
	redisCache, err := cache.NewRedisCache(cfg.Redis)
	if err != nil {
		log.Printf("warning: failed to connect to redis: %v (caching disabled)", err)
		redisCache = nil
	} else {
		log.Printf("connected to redis at %s:%d", cfg.Redis.Host, cfg.Redis.Port)
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

		// 创建DB-backed存储（使用repository作为store接口）
		accountStore = &DBAccountStore{repo: accountRepo}
		packageStore = &DBPackageStore{repo: packageRepo}
		settlementStore = &DBSettlementStore{repo: settlementRepo, accountRepo: accountRepo}
		earningStore = &DBEarningStore{usageRepo: usageRepo}

		_ = idempotencyRepo // 用于幂等中间件
	} else {
		// 回退到内存存储（开发模式）
		accountStore = NewInMemoryAccountStoreAdapter()
		packageStore = NewInMemoryPackageStoreAdapter()
		settlementStore = NewInMemorySettlementStoreAdapter()
		earningStore = NewInMemoryEarningStoreAdapter()
	}

	// P0-R08修复: 初始化审计存储 - 使用DB-backed实现
	var auditStore audit.AuditStore
	if auditRepo != nil {
		auditStore = audit.NewPostgresAuditStore(auditRepo)
		log.Println("审计存储: 使用PostgreSQL (DB-backed)")
	} else {
		auditStore = audit.NewMemoryAuditStore()
		log.Println("警告: 审计存储使用内存实现 (生产环境不应使用)")
	}

	// P0-09修复: 初始化外键校验器
	var fkValidator *repository.ForeignKeyValidator
	if db != nil {
		fkValidator = repository.NewForeignKeyValidator(db.Pool)
		log.Println("外键校验器: 已初始化 (PostgreSQL-backed)")
	} else {
		log.Println("警告: 外键校验器未启用 (db不可用)")
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
	}
	_ = idempotencyRepo // TODO: 在生产环境中用于DB-backed幂等

	// 初始化Token缓存
	tokenCache := middleware.NewTokenCache()
	if redisCache != nil {
		// 可以使用Redis缓存
	}

	// 初始化token状态后端（P0-03修复: 使用DB-backed实现）
	var tokenBackend middleware.TokenStatusBackend
	if tokenStatusRepo != nil {
		tokenBackend = middleware.NewDBTokenStatusBackend(tokenStatusRepo, redisCache, cfg.Token.RevocationCacheTTL)
		log.Println("Token状态后端: 使用PostgreSQL (DB-backed)")

		// 启动主动吊销订阅机制（仅在Redis可用时）
		if redisCache != nil {
			if dbTokenBackend, ok := tokenBackend.(*middleware.DBTokenStatusBackend); ok {
				if err := dbTokenBackend.StartRevocationSubscriber(ctx); err != nil {
					log.Printf("警告: 启动主动吊销订阅失败: %v", err)
				} else {
					log.Println("主动吊销机制: 已启动 (Redis Pub/Sub)")
				}
			}
		}
	} else {
		tokenBackend = newMemoryTokenBackend()
		log.Println("警告: Token状态后端使用内存实现 (生产环境不应使用)")
	}

	// 初始化审计事件适配器（NEW-P1-03修复）
	auditEmitter := newAuditEmitterAdapter(auditStore)

	// 初始化鉴权中间件
	authConfig := middleware.AuthConfig{
		SecretKey: cfg.Token.SecretKey,
		Issuer:    cfg.Token.Issuer,
		CacheTTL:  cfg.Token.RevocationCacheTTL,
		Enabled:   *env != "dev", // 开发模式禁用鉴权
	}
	authMiddleware := middleware.NewAuthMiddleware(authConfig, tokenCache, tokenBackend, auditEmitter)

	// 初始化幂等中间件（NEW-P1-04修复 - 由于repo为nil，暂保持禁用状态）
	// 注意：幂等逻辑在supply_api.go中以内联方式实现
	var idempotencyMiddleware *middleware.IdempotencyMiddleware
	if db != nil && idempotencyRepo != nil {
		idempotencyMiddleware = middleware.NewIdempotencyMiddleware(idempotencyRepo, middleware.IdempotencyConfig{
			TTL:     24 * time.Hour,
			Enabled: *env != "dev",
		})
		log.Println("幂等中间件已启用（DB-backed）")
	} else {
		log.Println("警告：幂等中间件未启用（db或repo不可用）- 使用内联幂等逻辑作为替代")
	}

	// P0-05修复: 初始化限流中间件
	rateLimitConfig := middleware.DefaultRateLimitConfig()
	rateLimitConfig.Enabled = *env != "dev" // 生产环境启用
	log.Println("限流中间件已初始化")

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
	alertAPI := httpapi.NewAlertAPI()
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
	// 注：幂等处理在supply_api.go中以内联方式实现（NEW-P1-05已统一：中间件方案需要DB-backed repo）

	var handler http.Handler = mux
	handler = middleware.RequestID(handler)
	handler = middleware.Recovery(handler)
	handler = middleware.Logging(handler, jsonLogger) // P1-010: 使用结构化JSON日志
	handler = middleware.TracingMiddleware(handler)   // P1-006: W3C Trace Context中间件

	// 生产环境启用安全中间件
	if *env != "dev" {
		// 5. QueryKeyReject - 拒绝外部query key
		handler = authMiddleware.QueryKeyRejectMiddleware(handler)
		// 6. BearerExtract
		handler = authMiddleware.BearerExtractMiddleware(handler)
		// 7. TokenVerify
		handler = authMiddleware.TokenVerifyMiddleware(handler)
		// 8. RateLimit - 限流 (使用中间件包装器)
		handler = middleware.NewRateLimitHandler(rateLimitConfig, handler)
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

	// P0-06修复: 启动OutboxProcessor（仅在DB可用时）
	var outboxProcessor *OutboxProcessorRunner
	if db != nil {
		outboxRepo := repository.NewOutboxRepository(db.Pool)
		var msgBroker messaging.MessageBroker
		if redisCache != nil {
			// 使用Redis Streams作为消息代理
			redisClient := redisCache.GetClient()
			msgBroker = messaging.NewOutboxMessageBroker(redisClient, "supply:outbox:stream", "outbox-processor")
		}
		stats := &messaging.NoOpOutboxStats{}
		outboxProcessor = NewOutboxProcessorRunner(outboxRepo, msgBroker, stats)
		go outboxProcessor.Start(ctx)
		log.Println("OutboxProcessor已启动")

		// 分区维护：确保未来分区已创建
		partitionManager := repository.NewPartitionManager(db.Pool)
		if err := partitionManager.EnsureFuturePartitions(ctx); err != nil {
			log.Printf("警告: 预创建未来分区失败: %v", err)
		} else {
			log.Println("分区管理: 未来分区已确保存在")
		}

		// 启动后台分区维护goroutine（每小时检查一次）
		go func() {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := partitionManager.EnsureFuturePartitions(context.Background()); err != nil {
						log.Printf("分区维护: 预创建未来分区失败: %v", err)
					}
					// 清理过期分区（仅在需要时）
					for _, tableName := range []string{"audit_events", "supply_usage_records", "supply_idempotency_records"} {
						if _, err := partitionManager.DropOldPartitions(context.Background(), tableName); err != nil {
							log.Printf("分区维护: 清理过期分区失败 (%s): %v", tableName, err)
						}
					}
				}
			}
		}()

		// P0-07修复: 初始化批量补偿处理器
		compensationStore := domain.NewSQLCompensationStore(db.Pool)
		compensationStats := &domain.NoOpCompensationStats{}
		compensationExecutor := &defaultCompensationExecutor{} // 需要实现OperationExecutor接口
		compensationProcessor := domain.NewCompensationProcessor(compensationStore, compensationExecutor, compensationStats)
		log.Println("批量补偿处理器: 已初始化")
		_ = compensationProcessor // TODO: 启动后台补偿处理goroutine
	}

	// 优雅关闭
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}

	log.Println("shutdown complete")
}

// ==================== 内存存储适配器（开发模式）====================

// InMemoryAccountStoreAdapter 内存账号存储适配器
type InMemoryAccountStoreAdapter struct {
	store *storage.InMemoryAccountStore
}

func NewInMemoryAccountStoreAdapter() *InMemoryAccountStoreAdapter {
	return &InMemoryAccountStoreAdapter{store: storage.NewInMemoryAccountStore()}
}

func (a *InMemoryAccountStoreAdapter) Create(ctx context.Context, account *domain.Account) error {
	return a.store.Create(ctx, account)
}

func (a *InMemoryAccountStoreAdapter) GetByID(ctx context.Context, supplierID, id int64) (*domain.Account, error) {
	return a.store.GetByID(ctx, supplierID, id)
}

func (a *InMemoryAccountStoreAdapter) Update(ctx context.Context, account *domain.Account) error {
	return a.store.Update(ctx, account)
}

func (a *InMemoryAccountStoreAdapter) List(ctx context.Context, supplierID int64) ([]*domain.Account, error) {
	return a.store.List(ctx, supplierID)
}

// InMemoryPackageStoreAdapter 内存套餐存储适配器
type InMemoryPackageStoreAdapter struct {
	store *storage.InMemoryPackageStore
}

func NewInMemoryPackageStoreAdapter() *InMemoryPackageStoreAdapter {
	return &InMemoryPackageStoreAdapter{store: storage.NewInMemoryPackageStore()}
}

func (a *InMemoryPackageStoreAdapter) Create(ctx context.Context, pkg *domain.Package) error {
	return a.store.Create(ctx, pkg)
}

func (a *InMemoryPackageStoreAdapter) GetByID(ctx context.Context, supplierID, id int64) (*domain.Package, error) {
	return a.store.GetByID(ctx, supplierID, id)
}

func (a *InMemoryPackageStoreAdapter) Update(ctx context.Context, pkg *domain.Package) error {
	return a.store.Update(ctx, pkg)
}

func (a *InMemoryPackageStoreAdapter) List(ctx context.Context, supplierID int64) ([]*domain.Package, error) {
	return a.store.List(ctx, supplierID)
}

// InMemorySettlementStoreAdapter 内存结算存储适配器
type InMemorySettlementStoreAdapter struct {
	store *storage.InMemorySettlementStore
}

func NewInMemorySettlementStoreAdapter() *InMemorySettlementStoreAdapter {
	return &InMemorySettlementStoreAdapter{store: storage.NewInMemorySettlementStore()}
}

func (a *InMemorySettlementStoreAdapter) Create(ctx context.Context, s *domain.Settlement) error {
	return a.store.Create(ctx, s)
}

func (a *InMemorySettlementStoreAdapter) GetByID(ctx context.Context, supplierID, id int64) (*domain.Settlement, error) {
	return a.store.GetByID(ctx, supplierID, id)
}

func (a *InMemorySettlementStoreAdapter) Update(ctx context.Context, s *domain.Settlement, expectedVersion int) error {
	// P1-005: 乐观锁更新
	return a.store.Update(ctx, s, expectedVersion)
}

func (a *InMemorySettlementStoreAdapter) List(ctx context.Context, supplierID int64) ([]*domain.Settlement, error) {
	return a.store.List(ctx, supplierID)
}

func (a *InMemorySettlementStoreAdapter) GetWithdrawableBalance(ctx context.Context, supplierID int64) (float64, error) {
	return a.store.GetWithdrawableBalance(ctx, supplierID)
}

func (a *InMemorySettlementStoreAdapter) HasPendingOrProcessingWithdraw(ctx context.Context, supplierID int64) (bool, error) {
	return a.store.HasPendingOrProcessingWithdraw(ctx, supplierID)
}

// InMemoryEarningStoreAdapter 内存收益存储适配器
type InMemoryEarningStoreAdapter struct {
	store *storage.InMemoryEarningStore
}

func NewInMemoryEarningStoreAdapter() *InMemoryEarningStoreAdapter {
	return &InMemoryEarningStoreAdapter{store: storage.NewInMemoryEarningStore()}
}

func (a *InMemoryEarningStoreAdapter) ListRecords(ctx context.Context, supplierID int64, startDate, endDate string, page, pageSize int) ([]*domain.EarningRecord, int, error) {
	return a.store.ListRecords(ctx, supplierID, startDate, endDate, page, pageSize)
}

func (a *InMemoryEarningStoreAdapter) GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*domain.BillingSummary, error) {
	return a.store.GetBillingSummary(ctx, supplierID, startDate, endDate)
}

// ==================== DB-backed存储适配器 ====================

// DBAccountStore DB-backed账号存储
type DBAccountStore struct {
	repo *repository.AccountRepository
}

func (s *DBAccountStore) Create(ctx context.Context, account *domain.Account) error {
	return s.repo.Create(ctx, account, "", "", "")
}

func (s *DBAccountStore) GetByID(ctx context.Context, supplierID, id int64) (*domain.Account, error) {
	return s.repo.GetByID(ctx, supplierID, id)
}

func (s *DBAccountStore) Update(ctx context.Context, account *domain.Account) error {
	return s.repo.Update(ctx, account, account.Version)
}

func (s *DBAccountStore) List(ctx context.Context, supplierID int64) ([]*domain.Account, error) {
	return s.repo.List(ctx, supplierID)
}

// DBPackageStore DB-backed套餐存储
type DBPackageStore struct {
	repo *repository.PackageRepository
}

func (s *DBPackageStore) Create(ctx context.Context, pkg *domain.Package) error {
	return s.repo.Create(ctx, pkg, "", "")
}

func (s *DBPackageStore) GetByID(ctx context.Context, supplierID, id int64) (*domain.Package, error) {
	return s.repo.GetByID(ctx, supplierID, id)
}

func (s *DBPackageStore) Update(ctx context.Context, pkg *domain.Package) error {
	return s.repo.Update(ctx, pkg, pkg.Version)
}

func (s *DBPackageStore) List(ctx context.Context, supplierID int64) ([]*domain.Package, error) {
	return s.repo.List(ctx, supplierID)
}

// DBSettlementStore DB-backed结算存储
type DBSettlementStore struct {
	repo        *repository.SettlementRepository
	accountRepo *repository.AccountRepository // 用于GetWithdrawableBalance查询账户余额
}

func (s *DBSettlementStore) Create(ctx context.Context, settlement *domain.Settlement) error {
	return s.repo.Create(ctx, settlement, "", "", "")
}

func (s *DBSettlementStore) GetByID(ctx context.Context, supplierID, id int64) (*domain.Settlement, error) {
	return s.repo.GetByID(ctx, supplierID, id)
}

func (s *DBSettlementStore) Update(ctx context.Context, settlement *domain.Settlement, expectedVersion int) error {
	// P1-005: 乐观锁更新，expectedVersion由调用方传入更新前的版本号
	return s.repo.Update(ctx, settlement, expectedVersion)
}

func (s *DBSettlementStore) List(ctx context.Context, supplierID int64) ([]*domain.Settlement, error) {
	return s.repo.List(ctx, supplierID)
}

func (s *DBSettlementStore) GetWithdrawableBalance(ctx context.Context, supplierID int64) (float64, error) {
	if s.accountRepo == nil {
		return 0.0, fmt.Errorf("account repository not initialized")
	}
	return s.accountRepo.GetWithdrawableBalance(ctx, supplierID)
}

func (s *DBSettlementStore) HasPendingOrProcessingWithdraw(ctx context.Context, supplierID int64) (bool, error) {
	return s.repo.HasPendingOrProcessingWithdraw(ctx, supplierID)
}

// DBEarningStore DB-backed收益存储
type DBEarningStore struct {
	usageRepo *repository.UsageRepository
}

func (s *DBEarningStore) ListRecords(ctx context.Context, supplierID int64, startDate, endDate string, page, pageSize int) ([]*domain.EarningRecord, int, error) {
	if s.usageRepo == nil {
		return nil, 0, fmt.Errorf("usage repository not initialized")
	}
	return s.usageRepo.ListRecords(ctx, supplierID, startDate, endDate, page, pageSize)
}

func (s *DBEarningStore) GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*domain.BillingSummary, error) {
	if s.usageRepo == nil {
		return nil, fmt.Errorf("usage repository not initialized")
	}
	return s.usageRepo.GetBillingSummary(ctx, supplierID, startDate, endDate)
}

// ==================== 内存Backend适配器 ====================

// memoryTokenBackend 内存token状态后端（临时实现，生产应使用DB-backed）
type memoryTokenBackend struct {
	revokedTokens map[string]string // tokenID -> status
}

func newMemoryTokenBackend() *memoryTokenBackend {
	return &memoryTokenBackend{
		revokedTokens: make(map[string]string),
	}
}

func (b *memoryTokenBackend) CheckTokenStatus(ctx context.Context, tokenID string) (string, error) {
	// 默认所有token都是active的
	if status, found := b.revokedTokens[tokenID]; found {
		return status, nil
	}
	return "active", nil
}

func (b *memoryTokenBackend) RevokeToken(tokenID string) {
	b.revokedTokens[tokenID] = "revoked"
}

// ==================== 审计事件适配器 ====================

// auditEmitterAdapter 将auditStore适配为middleware.AuditEmitter
type auditEmitterAdapter struct {
	store audit.AuditStore
}

func newAuditEmitterAdapter(store audit.AuditStore) *auditEmitterAdapter {
	return &auditEmitterAdapter{store: store}
}

func (a *auditEmitterAdapter) Emit(ctx context.Context, event middleware.AuditEvent) error {
	if a.store == nil {
		return nil
	}
	// 转换middleware.AuditEvent为audit.Event
	auditEvent := audit.Event{
		EventID:    event.RequestID,
		ObjectType: "auth",
		Action:     event.EventName,
		RequestID:  event.RequestID,
		ResultCode: event.ResultCode,
		SourceIP:   event.ClientIP, // C-002修复: 使用ClientIP替代SourceIP
	}
	a.store.Emit(ctx, auditEvent)
	return nil
}

// ==================== Outbox处理器 ====================

// OutboxProcessorRunner Outbox处理器运行器
type OutboxProcessorRunner struct {
	repo        *repository.OutboxRepository
	msgBroker   messaging.MessageBroker
	stats       messaging.OutboxStats
	stopCh      chan struct{}
	batchSize   int
	interval    time.Duration
}

// NewOutboxProcessorRunner 创建Outbox处理器运行器
func NewOutboxProcessorRunner(
	repo *repository.OutboxRepository,
	msgBroker messaging.MessageBroker,
	stats messaging.OutboxStats,
) *OutboxProcessorRunner {
	return &OutboxProcessorRunner{
		repo:      repo,
		msgBroker: msgBroker,
		stats:     stats,
		stopCh:    make(chan struct{}),
		batchSize: 100,
		interval:  1 * time.Second,
	}
}

// Start 启动Outbox处理器
func (r *OutboxProcessorRunner) Start(ctx context.Context) {
	log.Println("OutboxProcessor started")
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("OutboxProcessor stopping due to context cancellation")
			return
		case <-r.stopCh:
			log.Println("OutboxProcessor stopping")
			return
		case <-ticker.C:
			if err := r.process(ctx); err != nil {
				log.Printf("OutboxProcessor error: %v", err)
			}
		}
	}
}

// Stop 停止Outbox处理器
func (r *OutboxProcessorRunner) Stop() {
	close(r.stopCh)
}

// process 处理一批Outbox事件
func (r *OutboxProcessorRunner) process(ctx context.Context) error {
	// 获取待处理事件
	events, err := r.repo.FetchAndLock(ctx, r.batchSize)
	if err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	for _, event := range events {
		// 转换为domain.OutboxEvent
		domainEvent := &domain.OutboxEvent{
			ID:            event.ID,
			AggregateType: event.AggregateType,
			AggregateID:   event.AggregateID,
			EventType:     event.EventType,
			EventID:       event.EventID,
			Payload:       event.Payload,
			Status:       string(event.Status),
			RetryCount:    event.RetryCount,
			MaxRetries:    event.MaxRetries,
			ErrorMessage:  event.ErrorMessage,
			Version:       event.Version,
		}

		// 发布消息
		if err := r.msgBroker.Publish(ctx, event); err != nil {
			r.handleFailure(ctx, domainEvent, err)
			continue
		}

		// 标记完成
		if err := r.repo.MarkCompleted(ctx, event.EventID); err != nil {
			r.stats.RecordOutboxFailure("mark_completed_failed")
			continue
		}

		r.stats.RecordOutboxSuccess(event.EventType)
	}

	return nil
}

// handleFailure 处理失败事件
func (r *OutboxProcessorRunner) handleFailure(ctx context.Context, event *domain.OutboxEvent, publishErr error) {
	event.RetryCount++

	if event.RetryCount >= event.MaxRetries {
		// 移入死信队列
		domainEvent := &repository.OutboxEvent{
			ID:         event.ID,
			EventID:    event.EventID,
			Payload:    event.Payload,
			RetryCount: event.RetryCount,
		}
		if err := r.repo.MoveToDeadLetter(ctx, domainEvent, publishErr.Error()); err != nil {
			r.stats.RecordOutboxFailure("move_to_dlq_failed")
		} else {
			r.stats.RecordOutboxDLQ(event.EventType)
		}
	} else {
		// 计算下次重试时间（指数退避）
		backoffSeconds := calculateOutboxBackoff(event.RetryCount, event.MaxRetries)
		nextRetry := time.Now().Add(time.Duration(backoffSeconds) * time.Second)

		if err := r.repo.MarkFailed(ctx, event.EventID, publishErr.Error(), &nextRetry); err != nil {
			r.stats.RecordOutboxFailure("mark_failed_failed")
		} else {
			r.stats.RecordOutboxRetry(event.EventType)
		}
	}
}

// calculateOutboxBackoff 计算指数退避时间
func calculateOutboxBackoff(retryCount, maxRetries int) int {
	initialBackoff := 1.0
	maxBackoff := 60.0
	backoff := initialBackoff * math.Pow(2, float64(retryCount-1))
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	return int(backoff)
}

// Ensure domain.OutboxEvent is compatible with our conversion
var _ = domain.OutboxEvent{}

// ==================== 补偿执行器 ====================

// defaultCompensationExecutor 默认补偿执行器
type defaultCompensationExecutor struct{}

func (e *defaultCompensationExecutor) Execute(ctx context.Context, operationType string, payload json.RawMessage) error {
	// TODO: 根据operationType执行相应的补偿操作
	// 目前为placeholder实现，实际生产需要根据业务类型实现具体逻辑
	log.Printf("补偿执行器: operation_type=%s, payload=%s", operationType, string(payload))
	return nil
}

// parseRSAPublicKey 解析PEM格式的RSA公钥
func parseRSAPublicKey(pemKey string) interface{} {
	if pemKey == "" {
		return nil
	}
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return nil
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// 尝试解析PKCS1公钥
		rsaPub, err2 := x509.ParsePKCS1PublicKey(block.Bytes)
		if err2 != nil {
			log.Printf("警告: 解析RSA公钥失败: %v", err2)
			return nil
		}
		return rsaPub
	}
	return pub
}
