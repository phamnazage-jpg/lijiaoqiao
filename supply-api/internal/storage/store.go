package storage

import (
	"context"
	"errors"
	"sync"
	"time"

	"lijiaoqiao/supply-api/internal/domain"
)

// 错误定义
var ErrNotFound = errors.New("resource not found")

// 内存账号存储
type InMemoryAccountStore struct {
	mu       sync.RWMutex
	accounts map[int64]*domain.Account
	nextID   int64
}

func NewInMemoryAccountStore() *InMemoryAccountStore {
	return &InMemoryAccountStore{
		accounts: make(map[int64]*domain.Account),
		nextID:   1,
	}
}

func (s *InMemoryAccountStore) Create(ctx context.Context, account *domain.Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	account.ID = s.nextID
	s.nextID++
	account.CreatedAt = time.Now()
	account.UpdatedAt = time.Now()
	s.accounts[account.ID] = account
	return nil
}

func (s *InMemoryAccountStore) GetByID(ctx context.Context, supplierID, id int64) (*domain.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	account, ok := s.accounts[id]
	if !ok || account.SupplierID != supplierID {
		return nil, ErrNotFound
	}
	return account, nil
}

func (s *InMemoryAccountStore) Update(ctx context.Context, account *domain.Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.accounts[account.ID]
	if !ok || existing.SupplierID != account.SupplierID {
		return ErrNotFound
	}
	account.UpdatedAt = time.Now()
	s.accounts[account.ID] = account
	return nil
}

func (s *InMemoryAccountStore) List(ctx context.Context, supplierID int64) ([]*domain.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.Account
	for _, account := range s.accounts {
		if account.SupplierID == supplierID {
			result = append(result, account)
		}
	}
	return result, nil
}

// 内存套餐存储
type InMemoryPackageStore struct {
	mu       sync.RWMutex
	packages map[int64]*domain.Package
	nextID   int64
}

func NewInMemoryPackageStore() *InMemoryPackageStore {
	return &InMemoryPackageStore{
		packages: make(map[int64]*domain.Package),
		nextID:   1,
	}
}

func (s *InMemoryPackageStore) Create(ctx context.Context, pkg *domain.Package) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pkg.ID = s.nextID
	s.nextID++
	pkg.CreatedAt = time.Now()
	pkg.UpdatedAt = time.Now()
	s.packages[pkg.ID] = pkg
	return nil
}

func (s *InMemoryPackageStore) GetByID(ctx context.Context, supplierID, id int64) (*domain.Package, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pkg, ok := s.packages[id]
	if !ok || pkg.SupplierID != supplierID {
		return nil, ErrNotFound
	}
	return pkg, nil
}

func (s *InMemoryPackageStore) Update(ctx context.Context, pkg *domain.Package) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.packages[pkg.ID]
	if !ok || existing.SupplierID != pkg.SupplierID {
		return ErrNotFound
	}
	pkg.UpdatedAt = time.Now()
	s.packages[pkg.ID] = pkg
	return nil
}

func (s *InMemoryPackageStore) List(ctx context.Context, supplierID int64) ([]*domain.Package, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.Package
	for _, pkg := range s.packages {
		if pkg.SupplierID == supplierID {
			result = append(result, pkg)
		}
	}
	return result, nil
}

// 内存结算存储
type InMemorySettlementStore struct {
	mu          sync.RWMutex
	settlements map[int64]*domain.Settlement
	nextID      int64
}

func NewInMemorySettlementStore() *InMemorySettlementStore {
	return &InMemorySettlementStore{
		settlements: make(map[int64]*domain.Settlement),
		nextID:      1,
	}
}

func (s *InMemorySettlementStore) Create(ctx context.Context, settlement *domain.Settlement) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	settlement.ID = s.nextID
	s.nextID++
	settlement.CreatedAt = time.Now()
	settlement.UpdatedAt = time.Now()
	s.settlements[settlement.ID] = settlement
	return nil
}

func (s *InMemorySettlementStore) GetByID(ctx context.Context, supplierID, id int64) (*domain.Settlement, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	settlement, ok := s.settlements[id]
	if !ok || settlement.SupplierID != supplierID {
		return nil, ErrNotFound
	}
	return settlement, nil
}

func (s *InMemorySettlementStore) Update(ctx context.Context, settlement *domain.Settlement) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.settlements[settlement.ID]
	if !ok || existing.SupplierID != settlement.SupplierID {
		return ErrNotFound
	}
	settlement.UpdatedAt = time.Now()
	s.settlements[settlement.ID] = settlement
	return nil
}

func (s *InMemorySettlementStore) List(ctx context.Context, supplierID int64) ([]*domain.Settlement, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.Settlement
	for _, settlement := range s.settlements {
		if settlement.SupplierID == supplierID {
			result = append(result, settlement)
		}
	}
	return result, nil
}

func (s *InMemorySettlementStore) GetWithdrawableBalance(ctx context.Context, supplierID int64) (float64, error) {
	return 10000.0, nil
}

// 内存收益存储
type InMemoryEarningStore struct {
	mu      sync.RWMutex
	records map[int64]*domain.EarningRecord
	nextID  int64
}

func NewInMemoryEarningStore() *InMemoryEarningStore {
	return &InMemoryEarningStore{
		records: make(map[int64]*domain.EarningRecord),
		nextID:  1,
	}
}

func (s *InMemoryEarningStore) ListRecords(ctx context.Context, supplierID int64, startDate, endDate string, page, pageSize int) ([]*domain.EarningRecord, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.EarningRecord
	for _, record := range s.records {
		if record.SupplierID == supplierID {
			result = append(result, record)
		}
	}

	total := len(result)
	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= total {
		return []*domain.EarningRecord{}, total, nil
	}
	if end > total {
		end = total
	}

	return result[start:end], total, nil
}

func (s *InMemoryEarningStore) GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*domain.BillingSummary, error) {
	return &domain.BillingSummary{
		Period: domain.BillingPeriod{
			Start: startDate,
			End:   endDate,
		},
		Summary: domain.BillingTotal{
			TotalRevenue:   10000.0,
			TotalOrders:    100,
			TotalUsage:     1000000,
			TotalRequests:  50000,
			AvgSuccessRate: 99.5,
			PlatformFee:    100.0,
			NetEarnings:    9900.0,
		},
	}, nil
}

// 内存幂等存储
type InMemoryIdempotencyStore struct {
	mu      sync.RWMutex
	records map[string]*IdempotencyRecord
}

type IdempotencyRecord struct {
	Key       string
	Status    string // processing, succeeded, failed
	Response  interface{}
	CreatedAt time.Time
	ExpiresAt time.Time
}

func NewInMemoryIdempotencyStore() *InMemoryIdempotencyStore {
	return &InMemoryIdempotencyStore{
		records: make(map[string]*IdempotencyRecord),
	}
}

func (s *InMemoryIdempotencyStore) Get(key string) (*IdempotencyRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.records[key]
	if ok && record.ExpiresAt.After(time.Now()) {
		return record, true
	}
	return nil, false
}

func (s *InMemoryIdempotencyStore) SetProcessing(key string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records[key] = &IdempotencyRecord{
		Key:       key,
		Status:    "processing",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (s *InMemoryIdempotencyStore) SetSuccess(key string, response interface{}, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records[key] = &IdempotencyRecord{
		Key:       key,
		Status:    "succeeded",
		Response:  response,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}
}
