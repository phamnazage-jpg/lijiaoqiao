//go:build slow
// +build slow

package benchmark

import (
	"context"
	"fmt"
	"testing"

	"lijiaoqiao/supply-api/internal/audit"
	"lijiaoqiao/supply-api/internal/domain"
)

// BenchmarkAccountService_Create 基准测试：账号创建性能
func BenchmarkAccountService_Create(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	store := newMockAccountStoreForBenchmark()
	auditStore := &mockAuditStoreForBenchmark{}
	svc := domain.NewAccountService(store, auditStore)
	ctx := context.Background()

	req := &domain.CreateAccountRequest{
		SupplierID:  1001,
		Provider:    domain.ProviderOpenAI,
		AccountType: domain.AccountTypeAPIKey,
		Credential:  "sk-test-key-benchmark",
		Alias:       "bench-account",
		RiskAck:     true,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req.Alias = fmt.Sprintf("bench-account-%d", i)
		_, _ = svc.Create(ctx, req)
	}
}

// BenchmarkAccountService_Verify 基准测试：账号验证性能
func BenchmarkAccountService_Verify(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	store := newMockAccountStoreForBenchmark()
	auditStore := &mockAuditStoreForBenchmark{}
	svc := domain.NewAccountService(store, auditStore)
	ctx := context.Background()

	// 先创建一个账号
	req := &domain.CreateAccountRequest{
		SupplierID:  1001,
		Provider:    domain.ProviderOpenAI,
		AccountType: domain.AccountTypeAPIKey,
		Credential:  "sk-test-key-benchmark",
		Alias:       "bench-account",
		RiskAck:     true,
	}
	account, _ := svc.Create(ctx, req)
	_ = account

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = svc.Verify(ctx, 1001, domain.ProviderOpenAI, domain.AccountTypeAPIKey, "sk-test-key-benchmark")
	}
}

// BenchmarkPackageService_CreateDraft 基准测试：套餐创建性能
func BenchmarkPackageService_CreateDraft(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	store := newMockPackageStoreForBenchmark()
	accountStore := newMockAccountStoreForBenchmark()
	auditStore := &mockAuditStoreForBenchmark{}
	svc := domain.NewPackageService(store, accountStore, auditStore)
	ctx := context.Background()

	req := &domain.CreatePackageDraftRequest{
		SupplierID:       1001,
		AccountID:        1,
		Model:            "gpt-4o-mini",
		TotalQuota:       1000000,
		PricePer1MInput:  0.5,
		PricePer1MOutput: 1.5,
		ValidDays:        30,
		MaxConcurrent:    10,
		RateLimitRPM:     1000,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = svc.CreateDraft(ctx, 1001, req)
	}
}

// BenchmarkPackageService_BatchUpdatePrice 基准测试：批量调价性能
func BenchmarkPackageService_BatchUpdatePrice(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	store := newMockPackageStoreForBenchmark()
	accountStore := newMockAccountStoreForBenchmark()
	auditStore := &mockAuditStoreForBenchmark{}
	svc := domain.NewPackageService(store, accountStore, auditStore)
	ctx := context.Background()

	// 创建多个套餐
	for i := 0; i < 100; i++ {
		req := &domain.CreatePackageDraftRequest{
			SupplierID:       1001,
			AccountID:        1,
			Model:            fmt.Sprintf("gpt-4o-mini-%d", i),
			TotalQuota:       1000000,
			PricePer1MInput:  0.5,
			PricePer1MOutput: 1.5,
			ValidDays:        30,
		}
		pkg, _ := svc.CreateDraft(ctx, 1001, req)
		_, _ = svc.Publish(ctx, 1001, pkg.ID)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := &domain.BatchUpdatePriceRequest{
			Items: make([]domain.BatchPriceItem, 50),
		}
		for j := 0; j < 50; j++ {
			req.Items[j] = domain.BatchPriceItem{
				PackageID:        int64(j + 1),
				PricePer1MInput:  float64(i) * 0.1,
				PricePer1MOutput: float64(i) * 0.2,
			}
		}
		_, _ = svc.BatchUpdatePrice(ctx, 1001, req)
	}
}

// BenchmarkSettlementService_Withdraw 基准测试：提现性能
func BenchmarkSettlementService_Withdraw(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	store := newMockSettlementStoreForBenchmark()
	earningStore := newMockEarningStoreForBenchmark()
	auditStore := &mockAuditStoreForBenchmark{}
	svc := domain.NewSettlementService(store, earningStore, auditStore)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := &domain.WithdrawRequest{
			Amount:         100.00,
			PaymentMethod:  domain.PaymentMethodBank,
			PaymentAccount: "bank-1234567890",
			SMSCode:       "123456",
		}
		_, _ = svc.Withdraw(ctx, 1001, req)
	}
}

// BenchmarkConcurrentAccountAccess 基准测试：并发账号访问
func BenchmarkConcurrentAccountAccess(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	store := newMockAccountStoreForBenchmark()
	auditStore := &mockAuditStoreForBenchmark{}
	svc := domain.NewAccountService(store, auditStore)
	ctx := context.Background()

	// 先创建一个账号
	req := &domain.CreateAccountRequest{
		SupplierID:  1001,
		Provider:    domain.ProviderOpenAI,
		AccountType: domain.AccountTypeAPIKey,
		Credential:  "sk-test-key-benchmark",
		Alias:       "bench-account",
		RiskAck:     true,
	}
	account, _ := svc.Create(ctx, req)
	_ = account

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = store.GetByID(ctx, 1001, 1)
	}
}

// BenchmarkSettlementConcurrency 基准测试：结算并发冲突
func BenchmarkSettlementConcurrency(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	store := newMockSettlementStoreForBenchmark()
	earningStore := newMockEarningStoreForBenchmark()
	auditStore := &mockAuditStoreForBenchmark{}
	svc := domain.NewSettlementService(store, earningStore, auditStore)
	ctx := context.Background()

	// 创建一个待处理的结算单
	settlement, _ := svc.Withdraw(ctx, 1001, &domain.WithdrawRequest{
		Amount:         100.00,
		PaymentMethod:  domain.PaymentMethodBank,
		PaymentAccount: "bank-1234567890",
		SMSCode:       "123456",
	})
	_ = settlement

	b.ResetTimer()
	b.ReportAllocs()

	// 模拟并发取消
	for i := 0; i < b.N; i++ {
		_, _ = svc.Cancel(context.Background(), 1001, 1)
	}
}

// 辅助类型

type mockAccountStoreForBenchmark struct {
	accounts map[int64]*domain.Account
	nextID   int64
}

func newMockAccountStoreForBenchmark() *mockAccountStoreForBenchmark {
	return &mockAccountStoreForBenchmark{
		accounts: make(map[int64]*domain.Account),
		nextID:   1,
	}
}

func (m *mockAccountStoreForBenchmark) Create(ctx context.Context, account *domain.Account) error {
	account.ID = m.nextID
	m.nextID++
	m.accounts[account.ID] = account
	return nil
}

func (m *mockAccountStoreForBenchmark) GetByID(ctx context.Context, supplierID, id int64) (*domain.Account, error) {
	if account, ok := m.accounts[id]; ok && account.SupplierID == supplierID {
		return account, nil
	}
	return nil, fmt.Errorf("account not found")
}

func (m *mockAccountStoreForBenchmark) Update(ctx context.Context, account *domain.Account) error {
	m.accounts[account.ID] = account
	return nil
}

func (m *mockAccountStoreForBenchmark) List(ctx context.Context, supplierID int64) ([]*domain.Account, error) {
	var result []*domain.Account
	for _, account := range m.accounts {
		if account.SupplierID == supplierID {
			result = append(result, account)
		}
	}
	return result, nil
}

type mockPackageStoreForBenchmark struct {
	packages map[int64]*domain.Package
	nextID   int64
}

func newMockPackageStoreForBenchmark() *mockPackageStoreForBenchmark {
	return &mockPackageStoreForBenchmark{
		packages: make(map[int64]*domain.Package),
		nextID:   1,
	}
}

func (m *mockPackageStoreForBenchmark) Create(ctx context.Context, pkg *domain.Package) error {
	pkg.ID = m.nextID
	m.nextID++
	m.packages[pkg.ID] = pkg
	return nil
}

func (m *mockPackageStoreForBenchmark) GetByID(ctx context.Context, supplierID, id int64) (*domain.Package, error) {
	if pkg, ok := m.packages[id]; ok && pkg.SupplierID == supplierID {
		return pkg, nil
	}
	return nil, fmt.Errorf("package not found")
}

func (m *mockPackageStoreForBenchmark) Update(ctx context.Context, pkg *domain.Package) error {
	m.packages[pkg.ID] = pkg
	return nil
}

func (m *mockPackageStoreForBenchmark) List(ctx context.Context, supplierID int64) ([]*domain.Package, error) {
	var result []*domain.Package
	for _, pkg := range m.packages {
		if pkg.SupplierID == supplierID {
			result = append(result, pkg)
		}
	}
	return result, nil
}

type mockSettlementStoreForBenchmark struct {
	settlements map[int64]*domain.Settlement
	nextID      int64
	balance     float64
}

func newMockSettlementStoreForBenchmark() *mockSettlementStoreForBenchmark {
	return &mockSettlementStoreForBenchmark{
		settlements: make(map[int64]*domain.Settlement),
		nextID:      1,
		balance:     100000.00,
	}
}

func (m *mockSettlementStoreForBenchmark) Create(ctx context.Context, s *domain.Settlement) error {
	s.ID = m.nextID
	m.nextID++
	m.settlements[s.ID] = s
	return nil
}

func (m *mockSettlementStoreForBenchmark) GetByID(ctx context.Context, supplierID, id int64) (*domain.Settlement, error) {
	if s, ok := m.settlements[id]; ok && s.SupplierID == supplierID {
		return s, nil
	}
	return nil, fmt.Errorf("settlement not found")
}

func (m *mockSettlementStoreForBenchmark) Update(ctx context.Context, s *domain.Settlement, expectedVersion int) error {
	m.settlements[s.ID] = s
	return nil
}

func (m *mockSettlementStoreForBenchmark) List(ctx context.Context, supplierID int64) ([]*domain.Settlement, error) {
	var result []*domain.Settlement
	for _, s := range m.settlements {
		if s.SupplierID == supplierID {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSettlementStoreForBenchmark) GetWithdrawableBalance(ctx context.Context, supplierID int64) (float64, error) {
	return m.balance, nil
}

func (m *mockSettlementStoreForBenchmark) HasPendingOrProcessingWithdraw(ctx context.Context, supplierID int64) (bool, error) {
	return false, nil
}

type mockEarningStoreForBenchmark struct{}

func newMockEarningStoreForBenchmark() *mockEarningStoreForBenchmark {
	return &mockEarningStoreForBenchmark{}
}

func (m *mockEarningStoreForBenchmark) ListRecords(ctx context.Context, supplierID int64, startDate, endDate string, page, pageSize int) ([]*domain.EarningRecord, int, error) {
	return []*domain.EarningRecord{}, 0, nil
}

func (m *mockEarningStoreForBenchmark) GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*domain.BillingSummary, error) {
	return &domain.BillingSummary{}, nil
}

type mockAuditStoreForBenchmark struct{}

func (m *mockAuditStoreForBenchmark) Emit(ctx context.Context, event audit.Event) error {
	return nil
}

func (m *mockAuditStoreForBenchmark) Query(ctx context.Context, filter audit.EventFilter) ([]audit.Event, error) {
	return nil, nil
}

func (m *mockAuditStoreForBenchmark) QueryWithTotal(ctx context.Context, filter audit.EventFilter) ([]audit.Event, int64, error) {
	return nil, 0, nil
}

func (m *mockAuditStoreForBenchmark) GetByID(ctx context.Context, eventID string) (audit.Event, error) {
	return audit.Event{}, fmt.Errorf("not found")
}
