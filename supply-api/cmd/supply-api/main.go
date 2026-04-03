package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lijiaoqiao/supply-api/internal/audit"
	"lijiaoqiao/supply-api/internal/cache"
	"lijiaoqiao/supply-api/internal/config"
	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/httpapi"
	"lijiaoqiao/supply-api/internal/middleware"
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

	// 初始化审计存储
	// R-08: DatabaseAuditService 已创建 (audit/service/audit_service_db.go)
	// 注意：由于domain层使用audit.AuditStore接口(旧)，而DatabaseAuditService实现的是AuditStoreInterface(新)
	// 需要接口适配。暂保持内存存储，后续统一架构时处理。
	auditStore := audit.NewMemoryAuditStore()

	// 初始化存储层
	var accountStore domain.AccountStore
	var packageStore domain.PackageStore
	var settlementStore domain.SettlementStore
	var earningStore domain.EarningStore

	if db != nil {
		// 使用PostgreSQL存储
		accountRepo := repository.NewAccountRepository(db.Pool)
		packageRepo := repository.NewPackageRepository(db.Pool)
		settlementRepo := repository.NewSettlementRepository(db.Pool)
		idempotencyRepo := repository.NewIdempotencyRepository(db.Pool)

		// 创建DB-backed存储（使用repository作为store接口）
		accountStore = &DBAccountStore{repo: accountRepo}
		packageStore = &DBPackageStore{repo: packageRepo}
		settlementStore = &DBSettlementStore{repo: settlementRepo}
		earningStore = &DBEarningStore{repo: settlementRepo} // 复用

		_ = idempotencyRepo // 用于幂等中间件
	} else {
		// 回退到内存存储（开发模式）
		accountStore = NewInMemoryAccountStoreAdapter()
		packageStore = NewInMemoryPackageStoreAdapter()
		settlementStore = NewInMemorySettlementStoreAdapter()
		earningStore = NewInMemoryEarningStoreAdapter()
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

	// 初始化token状态后端（NEW-P1-03修复）
	tokenBackend := newMemoryTokenBackend()

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
		log.Println("幂等中间件已启用")
	} else {
		log.Println("警告：幂等中间件未启用（db或repo不可用）- 使用内联幂等逻辑作为替代")
	}
	_ = idempotencyMiddleware // 暂不使用，幂等逻辑在supply_api.go中实现

	// 初始化幂等存储
	idempotencyStore := storage.NewInMemoryIdempotencyStore()

	// 初始化HTTP API处理器
	api := httpapi.NewSupplyAPI(
		accountService,
		packageService,
		settlementService,
		earningService,
		idempotencyStore,
		auditStore,
		1, // 默认供应商ID
		time.Now,
	)

	// 创建路由器
	mux := http.NewServeMux()

	// 健康检查端点
	mux.HandleFunc("/actuator/health", handleHealthCheck(db, redisCache))
	mux.HandleFunc("/actuator/health/live", handleLiveness)
	mux.HandleFunc("/actuator/health/ready", handleReadiness(db, redisCache))

	// 注册API路由
	api.Register(mux)

	// 应用中间件链路
	// 1. RequestID - 请求追踪
	// 2. Recovery - Panic恢复
	// 3. Logging - 请求日志
	// 4. QueryKeyReject - 拒绝外部query key (M-016)
	// 5. BearerExtract - Bearer Token提取
	// 6. TokenVerify - JWT校验
	// 注：幂等处理在supply_api.go中以内联方式实现（NEW-P1-05已统一：中间件方案需要DB-backed repo）

	var handler http.Handler = mux
	handler = middleware.RequestID(handler)
	handler = middleware.Recovery(handler)
	handler = middleware.Logging(handler)

	// 生产环境启用安全中间件
	if *env != "dev" {
		// 4. QueryKeyReject - 拒绝外部query key
		handler = authMiddleware.QueryKeyRejectMiddleware(handler)
		// 5. BearerExtract
		handler = authMiddleware.BearerExtractMiddleware(handler)
		// 6. TokenVerify
		handler = authMiddleware.TokenVerifyMiddleware(handler)
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

	// 启动服务器
	go func() {
		log.Printf("supply-api listening on %s", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen failed: %v", err)
		}
	}()

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

// handleHealthCheck 健康检查
func handleHealthCheck(db *repository.DB, redisCache *cache.RedisCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		checks := map[string]string{
			"database": "UP",
			"redis":    "UP",
		}

		if db != nil {
			if err := db.HealthCheck(ctx); err != nil {
				checks["database"] = "DOWN"
			}
		} else {
			checks["database"] = "MISSING"
		}

		if redisCache != nil {
			if err := redisCache.HealthCheck(ctx); err != nil {
				checks["redis"] = "DOWN"
			}
		} else {
			checks["redis"] = "MISSING"
		}

		status := http.StatusOK
		for _, v := range checks {
			if v == "DOWN" {
				status = http.StatusServiceUnavailable
				break
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": map[bool]string{true: "UP", false: "DOWN"}[status == http.StatusOK],
			"checks": checks,
			"time":   time.Now().Format(time.RFC3339),
		})
	}
}

// handleLiveness 存活探针
func handleLiveness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"LIVE"}`))
}

// handleReadiness 就绪探针
func handleReadiness(db *repository.DB, redisCache *cache.RedisCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		ready := true
		if db == nil {
			ready = false
		} else if err := db.HealthCheck(ctx); err != nil {
			ready = false
		}

		w.Header().Set("Content-Type", "application/json")
		if ready {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"READY"}`))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"NOT_READY"}`))
		}
	}
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

func (a *InMemorySettlementStoreAdapter) Update(ctx context.Context, s *domain.Settlement) error {
	return a.store.Update(ctx, s)
}

func (a *InMemorySettlementStoreAdapter) List(ctx context.Context, supplierID int64) ([]*domain.Settlement, error) {
	return a.store.List(ctx, supplierID)
}

func (a *InMemorySettlementStoreAdapter) GetWithdrawableBalance(ctx context.Context, supplierID int64) (float64, error) {
	return a.store.GetWithdrawableBalance(ctx, supplierID)
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
	repo *repository.SettlementRepository
}

func (s *DBSettlementStore) Create(ctx context.Context, settlement *domain.Settlement) error {
	return s.repo.Create(ctx, settlement, "", "", "")
}

func (s *DBSettlementStore) GetByID(ctx context.Context, supplierID, id int64) (*domain.Settlement, error) {
	return s.repo.GetByID(ctx, supplierID, id)
}

func (s *DBSettlementStore) Update(ctx context.Context, settlement *domain.Settlement) error {
	return s.repo.Update(ctx, settlement, settlement.Version)
}

func (s *DBSettlementStore) List(ctx context.Context, supplierID int64) ([]*domain.Settlement, error) {
	return s.repo.List(ctx, supplierID)
}

func (s *DBSettlementStore) GetWithdrawableBalance(ctx context.Context, supplierID int64) (float64, error) {
	// TODO: 实现真实查询 - 通过 account service 获取
	return 0.0, nil
}

// DBEarningStore DB-backed收益存储
type DBEarningStore struct {
	repo *repository.SettlementRepository
}

func (s *DBEarningStore) ListRecords(ctx context.Context, supplierID int64, startDate, endDate string, page, pageSize int) ([]*domain.EarningRecord, int, error) {
	// TODO: 实现真实查询
	return nil, 0, nil
}

func (s *DBEarningStore) GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*domain.BillingSummary, error) {
	// TODO: 实现真实查询
	return nil, nil
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
		ClientIP:   event.ClientIP,
	}
	a.store.Emit(ctx, auditEvent)
	return nil
}
