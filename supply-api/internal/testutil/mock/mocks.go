package mock

import (
	"context"
	"errors"

	"lijiaoqiao/supply-api/internal/audit"
	"lijiaoqiao/supply-api/internal/domain"
)

// MockAccountStore 账号存储 mock
type MockAccountStore struct {
	Accounts map[int64]*domain.Account
	NextID   int64
}

// NewMockAccountStore 创建账号存储 mock
func NewMockAccountStore() *MockAccountStore {
	return &MockAccountStore{
		Accounts: make(map[int64]*domain.Account),
		NextID:   1,
	}
}

func (m *MockAccountStore) Create(ctx context.Context, account *domain.Account) error {
	account.ID = m.NextID
	m.NextID++
	m.Accounts[account.ID] = account
	return nil
}

func (m *MockAccountStore) GetByID(ctx context.Context, supplierID, id int64) (*domain.Account, error) {
	if account, ok := m.Accounts[id]; ok && account.SupplierID == supplierID {
		return account, nil
	}
	return nil, errors.New("account not found")
}

func (m *MockAccountStore) Update(ctx context.Context, account *domain.Account) error {
	if _, ok := m.Accounts[account.ID]; ok {
		m.Accounts[account.ID] = account
		return nil
	}
	return errors.New("account not found")
}

func (m *MockAccountStore) List(ctx context.Context, supplierID int64) ([]*domain.Account, error) {
	var result []*domain.Account
	for _, account := range m.Accounts {
		if account.SupplierID == supplierID {
			result = append(result, account)
		}
	}
	return result, nil
}

// MockPackageStore 套餐存储 mock
type MockPackageStore struct {
	Packages map[int64]*domain.Package
}

// NewMockPackageStore 创建套餐存储 mock
func NewMockPackageStore() *MockPackageStore {
	return &MockPackageStore{
		Packages: make(map[int64]*domain.Package),
	}
}

func (m *MockPackageStore) Create(ctx context.Context, pkg *domain.Package) error {
	if pkg.ID == 0 {
		pkg.ID = int64(len(m.Packages) + 1)
	}
	m.Packages[pkg.ID] = pkg
	return nil
}

func (m *MockPackageStore) GetByID(ctx context.Context, supplierID, id int64) (*domain.Package, error) {
	if pkg, ok := m.Packages[id]; ok && pkg.SupplierID == supplierID {
		return pkg, nil
	}
	return nil, errors.New("package not found")
}

func (m *MockPackageStore) Update(ctx context.Context, pkg *domain.Package) error {
	if _, ok := m.Packages[pkg.ID]; ok {
		m.Packages[pkg.ID] = pkg
		return nil
	}
	return errors.New("package not found")
}

func (m *MockPackageStore) List(ctx context.Context, supplierID int64) ([]*domain.Package, error) {
	var result []*domain.Package
	for _, pkg := range m.Packages {
		if pkg.SupplierID == supplierID {
			result = append(result, pkg)
		}
	}
	return result, nil
}

// MockSettlementStore 结算存储 mock
type MockSettlementStore struct {
	Settlements map[int64]*domain.Settlement
	NextID      int64
	Balance     float64
}

// NewMockSettlementStore 创建结算存储 mock
func NewMockSettlementStore() *MockSettlementStore {
	return &MockSettlementStore{
		Settlements: make(map[int64]*domain.Settlement),
		NextID:      1,
		Balance:     10000.00, // 默认余额
	}
}

func (m *MockSettlementStore) Create(ctx context.Context, s *domain.Settlement) error {
	s.ID = m.NextID
	m.NextID++
	m.Settlements[s.ID] = s
	return nil
}

func (m *MockSettlementStore) GetByID(ctx context.Context, supplierID, id int64) (*domain.Settlement, error) {
	if s, ok := m.Settlements[id]; ok && s.SupplierID == supplierID {
		return s, nil
	}
	return nil, errors.New("settlement not found")
}

func (m *MockSettlementStore) Update(ctx context.Context, s *domain.Settlement, expectedVersion int) error {
	if existing, ok := m.Settlements[s.ID]; ok && existing.Version != expectedVersion {
		return errors.New("concurrency conflict")
	}
	m.Settlements[s.ID] = s
	return nil
}

func (m *MockSettlementStore) List(ctx context.Context, supplierID int64) ([]*domain.Settlement, error) {
	var result []*domain.Settlement
	for _, s := range m.Settlements {
		if s.SupplierID == supplierID {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *MockSettlementStore) GetWithdrawableBalance(ctx context.Context, supplierID int64) (float64, error) {
	return m.Balance, nil
}

func (m *MockSettlementStore) HasPendingOrProcessingWithdraw(ctx context.Context, supplierID int64) (bool, error) {
	return false, nil // 默认允许提现
}

// MockEarningStore 收益存储 mock
type MockEarningStore struct {
	Records []*domain.EarningRecord
}

// NewMockEarningStore 创建收益存储 mock
func NewMockEarningStore() *MockEarningStore {
	return &MockEarningStore{
		Records: make([]*domain.EarningRecord, 0),
	}
}

func (m *MockEarningStore) ListRecords(ctx context.Context, supplierID int64, startDate, endDate string, page, pageSize int) ([]*domain.EarningRecord, int, error) {
	return m.Records, len(m.Records), nil
}

func (m *MockEarningStore) GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*domain.BillingSummary, error) {
	return &domain.BillingSummary{
		Period: domain.BillingPeriod{
			Start: startDate,
			End:   endDate,
		},
		Summary: domain.BillingTotal{
			TotalRevenue: 1000.00,
			TotalOrders:  100,
		},
	}, nil
}

// MockAuditStore 审计存储 mock
type MockAuditStore struct {
	Events []audit.Event
	EmitFn func(ctx context.Context, event audit.Event) error
}

// NewMockAuditStore 创建审计存储 mock
func NewMockAuditStore() *MockAuditStore {
	return &MockAuditStore{
		Events: make([]audit.Event, 0),
		EmitFn: func(ctx context.Context, event audit.Event) error {
			return nil
		},
	}
}

func (m *MockAuditStore) Emit(ctx context.Context, event audit.Event) error {
	m.Events = append(m.Events, event)
	return m.EmitFn(ctx, event)
}

func (m *MockAuditStore) Query(ctx context.Context, filter audit.EventFilter) ([]audit.Event, error) {
	return m.Events, nil
}

func (m *MockAuditStore) QueryWithTotal(ctx context.Context, filter audit.EventFilter) ([]audit.Event, int64, error) {
	return m.Events, int64(len(m.Events)), nil
}

func (m *MockAuditStore) GetByID(ctx context.Context, eventID string) (audit.Event, error) {
	return audit.Event{}, errors.New("not found")
}

// MockFailingAuditStore 总是失败的审计存储 mock（用于测试错误处理）
type MockFailingAuditStore struct{}

func (m *MockFailingAuditStore) Emit(ctx context.Context, event audit.Event) error {
	return errors.New("audit emit failed")
}

func (m *MockFailingAuditStore) Query(ctx context.Context, filter audit.EventFilter) ([]audit.Event, error) {
	return nil, nil
}

func (m *MockFailingAuditStore) QueryWithTotal(ctx context.Context, filter audit.EventFilter) ([]audit.Event, int64, error) {
	return nil, 0, nil
}

func (m *MockFailingAuditStore) GetByID(ctx context.Context, eventID string) (audit.Event, error) {
	return audit.Event{}, errors.New("not found")
}
