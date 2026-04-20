package adapter

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"lijiaoqiao/supply-api/internal/audit"
	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/middleware"
	"lijiaoqiao/supply-api/internal/repository"
	"lijiaoqiao/supply-api/internal/storage"
)

// ==================== 内存存储适配器（开发模式）====================

// InMemoryAccountStoreAdapter 内存账号存储适配器
type InMemoryAccountStoreAdapter struct {
	store *storage.InMemoryAccountStore
}

// NewInMemoryAccountStoreAdapter 创建内存账号存储适配器
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

// NewInMemoryPackageStoreAdapter 创建内存套餐存储适配器
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

// NewInMemorySettlementStoreAdapter 创建内存结算存储适配器
func NewInMemorySettlementStoreAdapter() *InMemorySettlementStoreAdapter {
	return &InMemorySettlementStoreAdapter{store: storage.NewInMemorySettlementStore()}
}

func (a *InMemorySettlementStoreAdapter) Create(ctx context.Context, s *domain.Settlement) error {
	return a.store.Create(ctx, s)
}

// CreateWithdrawTx 内存存储的原子提现创建（简化实现，假设无并发）
func (a *InMemorySettlementStoreAdapter) CreateWithdrawTx(ctx context.Context, s *domain.Settlement) error {
	return a.store.CreateInTx(ctx, s)
}

func (a *InMemorySettlementStoreAdapter) CreateInTx(ctx context.Context, s *domain.Settlement) error {
	return a.store.CreateInTx(ctx, s)
}

func (a *InMemorySettlementStoreAdapter) GetByID(ctx context.Context, supplierID, id int64) (*domain.Settlement, error) {
	return a.store.GetByID(ctx, supplierID, id)
}

func (a *InMemorySettlementStoreAdapter) Update(ctx context.Context, s *domain.Settlement, expectedVersion int) error {
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

// NewInMemoryEarningStoreAdapter 创建内存收益存储适配器
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

// NewDBAccountStore 创建DB-backed账号存储
func NewDBAccountStore(repo *repository.AccountRepository) *DBAccountStore {
	return &DBAccountStore{repo: repo}
}

func (s *DBAccountStore) Create(ctx context.Context, account *domain.Account) error {
	return s.repo.Create(ctx, account, "", "", "")
}

func (s *DBAccountStore) GetByID(ctx context.Context, supplierID, id int64) (*domain.Account, error) {
	return s.repo.GetByID(ctx, supplierID, id)
}

func (s *DBAccountStore) Update(ctx context.Context, account *domain.Account) error {
	expectedVersion := 0
	if account.Version > 0 {
		expectedVersion = account.Version - 1
	}
	return s.repo.Update(ctx, account, expectedVersion)
}

func (s *DBAccountStore) List(ctx context.Context, supplierID int64) ([]*domain.Account, error) {
	return s.repo.List(ctx, supplierID)
}

// DBPackageStore DB-backed套餐存储
type DBPackageStore struct {
	repo *repository.PackageRepository
}

// NewDBPackageStore 创建DB-backed套餐存储
func NewDBPackageStore(repo *repository.PackageRepository) *DBPackageStore {
	return &DBPackageStore{repo: repo}
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
	pool        *pgxpool.Pool                 // 用于创建事务
}

// NewDBSettlementStore 创建DB-backed结算存储
func NewDBSettlementStore(repo *repository.SettlementRepository, accountRepo *repository.AccountRepository, pool *pgxpool.Pool) *DBSettlementStore {
	return &DBSettlementStore{
		repo:        repo,
		accountRepo: accountRepo,
		pool:        pool,
	}
}

func (s *DBSettlementStore) Create(ctx context.Context, settlement *domain.Settlement) error {
	return s.repo.Create(ctx, settlement, "", "", "")
}

// CreateWithdrawTx 原子化创建提现（带锁）
// 使用 SELECT ... FOR UPDATE SKIP LOCKED 防止并发提现
func (s *DBSettlementStore) CreateWithdrawTx(ctx context.Context, settlement *domain.Settlement) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.CreateWithdrawTx(ctx, tx, settlement, "", "", ""); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// CreateInTx 在事务中创建结算单（非提现）
func (s *DBSettlementStore) CreateInTx(ctx context.Context, settlement *domain.Settlement) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.CreateTx(ctx, tx, settlement, "", "", ""); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (s *DBSettlementStore) GetByID(ctx context.Context, supplierID, id int64) (*domain.Settlement, error) {
	return s.repo.GetByID(ctx, supplierID, id)
}

func (s *DBSettlementStore) Update(ctx context.Context, settlement *domain.Settlement, expectedVersion int) error {
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

// NewDBEarningStore 创建DB-backed收益存储
func NewDBEarningStore(usageRepo *repository.UsageRepository) *DBEarningStore {
	return &DBEarningStore{usageRepo: usageRepo}
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

// MemoryTokenBackend 内存token状态后端（临时实现，生产应使用DB-backed）
type MemoryTokenBackend struct {
	revokedTokens map[string]string // tokenID -> status
}

// NewMemoryTokenBackend 创建内存token后端
func NewMemoryTokenBackend() *MemoryTokenBackend {
	return &MemoryTokenBackend{
		revokedTokens: make(map[string]string),
	}
}

// CheckTokenStatus 检查token状态
func (b *MemoryTokenBackend) CheckTokenStatus(ctx context.Context, tokenID string) (string, error) {
	// 默认所有token都是active的
	if status, found := b.revokedTokens[tokenID]; found {
		return status, nil
	}
	return "active", nil
}

// RevokeToken 吊销token
func (b *MemoryTokenBackend) RevokeToken(tokenID string) {
	b.revokedTokens[tokenID] = "revoked"
}

// ==================== 审计事件适配器 ====================

// AuditEmitterAdapter 将auditStore适配为middleware.AuditEmitter
type AuditEmitterAdapter struct {
	store audit.AuditStore
}

// NewAuditEmitterAdapter 创建审计事件适配器
func NewAuditEmitterAdapter(store audit.AuditStore) *AuditEmitterAdapter {
	return &AuditEmitterAdapter{store: store}
}

// Emit 发送审计事件
func (a *AuditEmitterAdapter) Emit(ctx context.Context, event middleware.AuditEvent) error {
	if a.store == nil {
		return nil
	}
	// 转换middleware.AuditEvent为audit.Event
	auditEvent := audit.Event{
		EventID:    event.RequestID,
		ObjectType: "auth",
		Action:    event.EventName,
		RequestID:  event.RequestID,
		ResultCode: event.ResultCode,
		SourceIP:   event.ClientIP,
	}
	a.store.Emit(ctx, auditEvent)
	return nil
}
