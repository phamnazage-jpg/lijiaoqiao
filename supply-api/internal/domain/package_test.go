package domain

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"lijiaoqiao/supply-api/internal/audit"
)

// mockPackageStoreForPackageTest Mock套餐存储
type mockPackageStoreForPackageTest struct {
	packages map[int64]*Package
	nextID   int64
}

func newMockPackageStoreForPackageTest() *mockPackageStoreForPackageTest {
	return &mockPackageStoreForPackageTest{
		packages: make(map[int64]*Package),
		nextID:   1,
	}
}

func (m *mockPackageStoreForPackageTest) Create(ctx context.Context, pkg *Package) error {
	pkg.ID = m.nextID
	m.nextID++
	m.packages[pkg.ID] = pkg
	return nil
}

func (m *mockPackageStoreForPackageTest) GetByID(ctx context.Context, supplierID, id int64) (*Package, error) {
	if pkg, ok := m.packages[id]; ok && pkg.SupplierID == supplierID {
		return pkg, nil
	}
	return nil, errors.New("package not found")
}

func (m *mockPackageStoreForPackageTest) Update(ctx context.Context, pkg *Package) error {
	m.packages[pkg.ID] = pkg
	return nil
}

func (m *mockPackageStoreForPackageTest) List(ctx context.Context, supplierID int64) ([]*Package, error) {
	var result []*Package
	for _, pkg := range m.packages {
		if pkg.SupplierID == supplierID {
			result = append(result, pkg)
		}
	}
	return result, nil
}

// mockAccountStoreForPackageTest Mock账号存储
type mockAccountStoreForPackageTest struct {
	accounts map[int64]*Account
}

func newMockAccountStoreForPackageTest() *mockAccountStoreForPackageTest {
	return &mockAccountStoreForPackageTest{
		accounts: make(map[int64]*Account),
	}
}

func (m *mockAccountStoreForPackageTest) Create(ctx context.Context, account *Account) error {
	m.accounts[account.ID] = account
	return nil
}

func (m *mockAccountStoreForPackageTest) GetByID(ctx context.Context, supplierID, id int64) (*Account, error) {
	if account, ok := m.accounts[id]; ok && account.SupplierID == supplierID {
		return account, nil
	}
	return nil, errors.New("account not found")
}

func (m *mockAccountStoreForPackageTest) Update(ctx context.Context, account *Account) error {
	m.accounts[account.ID] = account
	return nil
}

func (m *mockAccountStoreForPackageTest) List(ctx context.Context, supplierID int64) ([]*Account, error) {
	var result []*Account
	for _, account := range m.accounts {
		if account.SupplierID == supplierID {
			result = append(result, account)
		}
	}
	return result, nil
}

// mockAuditStoreForPackageTest Mock审计存储
type mockAuditStoreForPackageTest struct{}

func (m *mockAuditStoreForPackageTest) Emit(ctx context.Context, event audit.Event) error {
	return nil
}

func (m *mockAuditStoreForPackageTest) Query(ctx context.Context, filter audit.EventFilter) ([]audit.Event, error) {
	return nil, nil
}

func (m *mockAuditStoreForPackageTest) QueryWithTotal(ctx context.Context, filter audit.EventFilter) ([]audit.Event, int64, error) {
	return nil, 0, nil
}

func (m *mockAuditStoreForPackageTest) GetByID(ctx context.Context, eventID string) (audit.Event, error) {
	return audit.Event{}, errors.New("not found")
}

// TestPackageStatusConstants 测试套餐状态常量
func TestPackageStatusConstants(t *testing.T) {
	assert.Equal(t, PackageStatus("draft"), PackageStatusDraft)
	assert.Equal(t, PackageStatus("active"), PackageStatusActive)
	assert.Equal(t, PackageStatus("paused"), PackageStatusPaused)
	assert.Equal(t, PackageStatus("sold_out"), PackageStatusSoldOut)
	assert.Equal(t, PackageStatus("expired"), PackageStatusExpired)
}

// TestPackageStruct 测试套餐结构体
func TestPackageStruct(t *testing.T) {
	now := time.Now()
	pkg := &Package{
		ID:               1,
		SupplierID:       1001,
		AccountID:        2001,
		Platform:         "openai",
		Model:            "gpt-4",
		TotalQuota:       10000.0,
		AvailableQuota:   8000.0,
		SoldQuota:        2000.0,
		ReservedQuota:    500.0,
		PricePer1MInput:  0.5,
		PricePer1MOutput: 1.5,
		MinPurchase:      100.0,
		StartAt:          now,
		EndAt:            now.Add(30 * 24 * time.Hour),
		ValidDays:        30,
		MaxConcurrent:    10,
		RateLimitRPM:     100,
		Status:           PackageStatusActive,
		TotalOrders:      100,
		TotalRevenue:    5000.0,
		Rating:          4.5,
		RatingCount:     50,
		QuotaUnit:       "tokens",
		PriceUnit:       "yuan",
		CurrencyCode:    "CNY",
		Version:         1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	assert.Equal(t, int64(1), pkg.ID)
	assert.Equal(t, int64(1001), pkg.SupplierID)
	assert.Equal(t, int64(2001), pkg.AccountID)
	assert.Equal(t, "openai", pkg.Platform)
	assert.Equal(t, "gpt-4", pkg.Model)
	assert.Equal(t, 10000.0, pkg.TotalQuota)
	assert.Equal(t, 8000.0, pkg.AvailableQuota)
	assert.Equal(t, 2000.0, pkg.SoldQuota)
	assert.Equal(t, 500.0, pkg.ReservedQuota)
	assert.Equal(t, 0.5, pkg.PricePer1MInput)
	assert.Equal(t, 1.5, pkg.PricePer1MOutput)
	assert.Equal(t, PackageStatusActive, pkg.Status)
	assert.Equal(t, 100, pkg.TotalOrders)
	assert.Equal(t, 5000.0, pkg.TotalRevenue)
	assert.Equal(t, 4.5, pkg.Rating)
	assert.Equal(t, 50, pkg.RatingCount)
	assert.Equal(t, "CNY", pkg.CurrencyCode)
	assert.Equal(t, 1, pkg.Version)
}

// TestCreatePackageDraftRequest 测试创建套餐草稿请求
func TestCreatePackageDraftRequest(t *testing.T) {
	req := &CreatePackageDraftRequest{
		SupplierID:       1001,
		AccountID:        2001,
		Model:            "gpt-4",
		TotalQuota:       10000.0,
		PricePer1MInput:  0.5,
		PricePer1MOutput: 1.5,
		ValidDays:        30,
		MaxConcurrent:    10,
		RateLimitRPM:     100,
	}

	assert.Equal(t, int64(1001), req.SupplierID)
	assert.Equal(t, int64(2001), req.AccountID)
	assert.Equal(t, "gpt-4", req.Model)
	assert.Equal(t, 10000.0, req.TotalQuota)
	assert.Equal(t, 0.5, req.PricePer1MInput)
	assert.Equal(t, 1.5, req.PricePer1MOutput)
	assert.Equal(t, 30, req.ValidDays)
	assert.Equal(t, 10, req.MaxConcurrent)
	assert.Equal(t, 100, req.RateLimitRPM)
}

// TestBatchUpdatePriceRequest 测试批量更新价格请求
func TestBatchUpdatePriceRequest(t *testing.T) {
	req := &BatchUpdatePriceRequest{
		Items: []BatchPriceItem{
			{PackageID: 1, PricePer1MInput: 0.6},
			{PackageID: 2, PricePer1MOutput: 1.6},
		},
	}

	assert.Len(t, req.Items, 2)
	assert.Equal(t, int64(1), req.Items[0].PackageID)
	assert.Equal(t, 0.6, req.Items[0].PricePer1MInput)
}

// TestBatchUpdatePriceResponse 测试批量更新价格响应
func TestBatchUpdatePriceResponse(t *testing.T) {
	resp := &BatchUpdatePriceResponse{
		Total:        10,
		SuccessCount: 8,
		FailedCount:  2,
		Failures: []BatchPriceFailure{
			{PackageID: 1, ErrorCode: "ERR_001", Message: "invalid price"},
		},
	}

	assert.Equal(t, 10, resp.Total)
	assert.Equal(t, 8, resp.SuccessCount)
	assert.Equal(t, 2, resp.FailedCount)
	assert.Len(t, resp.Failures, 1)
	assert.Equal(t, int64(1), resp.Failures[0].PackageID)
}

// TestInvariantPackageErrors 测试套餐相关不变量错误
func TestInvariantPackageErrors(t *testing.T) {
	assert.Contains(t, ErrPackageSoldOutSystemOnly.Error(), "sold_out")
	assert.Contains(t, ErrPackageExpiredCannotRestore.Error(), "expired package")
	assert.Contains(t, ErrPriceBelowProtection.Error(), "price cannot be below")
}

// TestNewPackageService 测试创建套餐服务
func TestNewPackageService(t *testing.T) {
	pkgStore := newMockPackageStoreForPackageTest()
	acctStore := newMockAccountStoreForPackageTest()
	auditStore := &mockAuditStoreForPackageTest{}

	svc := NewPackageService(pkgStore, acctStore, auditStore)
	assert.NotNil(t, svc)
}

// TestPackageService_CreateDraft 测试创建套餐草稿
func TestPackageService_CreateDraft(t *testing.T) {
	pkgStore := newMockPackageStoreForPackageTest()
	acctStore := newMockAccountStoreForPackageTest()
	auditStore := &mockAuditStoreForPackageTest{}

	svc := NewPackageService(pkgStore, acctStore, auditStore)

	req := &CreatePackageDraftRequest{
		SupplierID:       1001,
		AccountID:        2001,
		Model:            "gpt-4",
		TotalQuota:       10000.0,
		PricePer1MInput:  0.5,
		PricePer1MOutput: 1.5,
		ValidDays:        30,
		MaxConcurrent:    10,
		RateLimitRPM:     100,
	}

	pkg, err := svc.CreateDraft(context.Background(), 1001, req)
	assert.NoError(t, err)
	assert.NotNil(t, pkg)
	assert.Equal(t, int64(1001), pkg.SupplierID)
	assert.Equal(t, "gpt-4", pkg.Model)
	assert.Equal(t, PackageStatusDraft, pkg.Status)
	assert.Equal(t, 10000.0, pkg.AvailableQuota)
	assert.Equal(t, 1, pkg.Version)
}

// TestPackageService_Publish 测试发布套餐
func TestPackageService_Publish(t *testing.T) {
	pkgStore := newMockPackageStoreForPackageTest()
	acctStore := newMockAccountStoreForPackageTest()
	auditStore := &mockAuditStoreForPackageTest{}

	svc := NewPackageService(pkgStore, acctStore, auditStore)

	// 先创建草稿
	req := &CreatePackageDraftRequest{
		SupplierID:       1001,
		AccountID:        2001,
		Model:            "gpt-4",
		TotalQuota:       10000.0,
		PricePer1MInput:  0.5,
		PricePer1MOutput: 1.5,
		ValidDays:        30,
	}
	pkg, _ := svc.CreateDraft(context.Background(), 1001, req)

	// 发布
	published, err := svc.Publish(context.Background(), 1001, pkg.ID)
	assert.NoError(t, err)
	assert.NotNil(t, published)
	assert.Equal(t, PackageStatusActive, published.Status)
}

// TestPackageService_Publish_ExpiredPackage 测试发布过期套餐
func TestPackageService_Publish_ExpiredPackage(t *testing.T) {
	pkgStore := newMockPackageStoreForPackageTest()
	acctStore := newMockAccountStoreForPackageTest()
	auditStore := &mockAuditStoreForPackageTest{}

	svc := NewPackageService(pkgStore, acctStore, auditStore)

	// 创建并直接标记为 expired（通过手动设置 store）
	req := &CreatePackageDraftRequest{
		SupplierID:       1001,
		AccountID:        2001,
		Model:            "gpt-4",
		TotalQuota:       10000.0,
		PricePer1MInput:  0.5,
		PricePer1MOutput: 1.5,
		ValidDays:        30,
	}
	pkg, _ := svc.CreateDraft(context.Background(), 1001, req)
	pkgStore.packages[pkg.ID].Status = PackageStatusExpired

	// 尝试发布过期套餐应该失败
	_, err := svc.Publish(context.Background(), 1001, pkg.ID)
	assert.Error(t, err)
}

// TestPackageService_Pause 测试暂停套餐
func TestPackageService_Pause(t *testing.T) {
	pkgStore := newMockPackageStoreForPackageTest()
	acctStore := newMockAccountStoreForPackageTest()
	auditStore := &mockAuditStoreForPackageTest{}

	svc := NewPackageService(pkgStore, acctStore, auditStore)

	// 创建并发布
	req := &CreatePackageDraftRequest{
		SupplierID:       1001,
		AccountID:        2001,
		Model:            "gpt-4",
		TotalQuota:       10000.0,
		PricePer1MInput:  0.5,
		PricePer1MOutput: 1.5,
		ValidDays:        30,
	}
	pkg, _ := svc.CreateDraft(context.Background(), 1001, req)
	svc.Publish(context.Background(), 1001, pkg.ID)

	// 暂停
	paused, err := svc.Pause(context.Background(), 1001, pkg.ID)
	assert.NoError(t, err)
	assert.Equal(t, PackageStatusPaused, paused.Status)
}

// TestPackageService_Unlist 测试下架套餐
func TestPackageService_Unlist(t *testing.T) {
	pkgStore := newMockPackageStoreForPackageTest()
	acctStore := newMockAccountStoreForPackageTest()
	auditStore := &mockAuditStoreForPackageTest{}

	svc := NewPackageService(pkgStore, acctStore, auditStore)

	// 创建并发布
	req := &CreatePackageDraftRequest{
		SupplierID:       1001,
		AccountID:        2001,
		Model:            "gpt-4",
		TotalQuota:       10000.0,
		PricePer1MInput:  0.5,
		PricePer1MOutput: 1.5,
		ValidDays:        30,
	}
	pkg, _ := svc.CreateDraft(context.Background(), 1001, req)
	svc.Publish(context.Background(), 1001, pkg.ID)

	// 下架
	unlisted, err := svc.Unlist(context.Background(), 1001, pkg.ID)
	assert.NoError(t, err)
	assert.Equal(t, PackageStatusExpired, unlisted.Status)
}

// TestPackageService_GetByID 测试获取套餐
func TestPackageService_GetByID(t *testing.T) {
	pkgStore := newMockPackageStoreForPackageTest()
	acctStore := newMockAccountStoreForPackageTest()
	auditStore := &mockAuditStoreForPackageTest{}

	svc := NewPackageService(pkgStore, acctStore, auditStore)

	// 创建套餐
	req := &CreatePackageDraftRequest{
		SupplierID:       1001,
		AccountID:        2001,
		Model:            "gpt-4",
		TotalQuota:       10000.0,
		PricePer1MInput:  0.5,
		PricePer1MOutput: 1.5,
		ValidDays:        30,
	}
	pkg, _ := svc.CreateDraft(context.Background(), 1001, req)

	// 获取
	found, err := svc.GetByID(context.Background(), 1001, pkg.ID)
	assert.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, pkg.ID, found.ID)
}

// TestPackageService_GetByID_NotFound 测试获取不存在的套餐
func TestPackageService_GetByID_NotFound(t *testing.T) {
	pkgStore := newMockPackageStoreForPackageTest()
	acctStore := newMockAccountStoreForPackageTest()
	auditStore := &mockAuditStoreForPackageTest{}

	svc := NewPackageService(pkgStore, acctStore, auditStore)

	_, err := svc.GetByID(context.Background(), 1001, 9999)
	assert.Error(t, err)
}

// TestPackageService_Clone 测试克隆套餐
func TestPackageService_Clone(t *testing.T) {
	pkgStore := newMockPackageStoreForPackageTest()
	acctStore := newMockAccountStoreForPackageTest()
	auditStore := &mockAuditStoreForPackageTest{}

	svc := NewPackageService(pkgStore, acctStore, auditStore)

	// 创建并发布原套餐
	req := &CreatePackageDraftRequest{
		SupplierID:       1001,
		AccountID:        2001,
		Model:            "gpt-4",
		TotalQuota:       10000.0,
		PricePer1MInput:  0.5,
		PricePer1MOutput: 1.5,
		ValidDays:        30,
		MaxConcurrent:    10,
		RateLimitRPM:     100,
	}
	original, _ := svc.CreateDraft(context.Background(), 1001, req)
	svc.Publish(context.Background(), 1001, original.ID)

	// 克隆
	clone, err := svc.Clone(context.Background(), 1001, original.ID)
	assert.NoError(t, err)
	assert.NotNil(t, clone)
	assert.NotEqual(t, original.ID, clone.ID)
	assert.Equal(t, original.SupplierID, clone.SupplierID)
	assert.Equal(t, original.AccountID, clone.AccountID)
	assert.Equal(t, original.Model, clone.Model)
	assert.Equal(t, original.TotalQuota, clone.TotalQuota)
	assert.Equal(t, original.TotalQuota, clone.AvailableQuota) // 可用配额重置为总量
	assert.Equal(t, 0.0, clone.SoldQuota)                      // 售出配额重置为0
	assert.Equal(t, PackageStatusDraft, clone.Status)          // 克隆后为草稿状态
}

// TestPackageService_Clone_NotFound 测试克隆不存在的套餐
func TestPackageService_Clone_NotFound(t *testing.T) {
	pkgStore := newMockPackageStoreForPackageTest()
	acctStore := newMockAccountStoreForPackageTest()
	auditStore := &mockAuditStoreForPackageTest{}

	svc := NewPackageService(pkgStore, acctStore, auditStore)

	_, err := svc.Clone(context.Background(), 1001, 9999)
	assert.Error(t, err)
}

// TestPackageService_BatchUpdatePrice 测试批量更新价格
func TestPackageService_BatchUpdatePrice(t *testing.T) {
	pkgStore := newMockPackageStoreForPackageTest()
	acctStore := newMockAccountStoreForPackageTest()
	auditStore := &mockAuditStoreForPackageTest{}

	svc := NewPackageService(pkgStore, acctStore, auditStore)

	// 创建套餐
	req := &CreatePackageDraftRequest{
		SupplierID:       1001,
		AccountID:        2001,
		Model:            "gpt-4",
		TotalQuota:       10000.0,
		PricePer1MInput:  0.5,
		PricePer1MOutput: 1.5,
		ValidDays:        30,
	}
	pkg, _ := svc.CreateDraft(context.Background(), 1001, req)
	svc.Publish(context.Background(), 1001, pkg.ID)

	// 批量更新价格
	batchReq := &BatchUpdatePriceRequest{
		Items: []BatchPriceItem{
			{PackageID: pkg.ID, PricePer1MInput: 0.6, PricePer1MOutput: 1.6},
		},
	}

	resp, err := svc.BatchUpdatePrice(context.Background(), 1001, batchReq)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Total)
	assert.Equal(t, 1, resp.SuccessCount)
	assert.Equal(t, 0, resp.FailedCount)
}

// TestPackageService_BatchUpdatePrice_NegativePrice 测试批量更新价格-负数价格
func TestPackageService_BatchUpdatePrice_NegativePrice(t *testing.T) {
	pkgStore := newMockPackageStoreForPackageTest()
	acctStore := newMockAccountStoreForPackageTest()
	auditStore := &mockAuditStoreForPackageTest{}

	svc := NewPackageService(pkgStore, acctStore, auditStore)

	// 创建套餐
	req := &CreatePackageDraftRequest{
		SupplierID:       1001,
		AccountID:        2001,
		Model:            "gpt-4",
		TotalQuota:       10000.0,
		PricePer1MInput:  0.5,
		PricePer1MOutput: 1.5,
		ValidDays:        30,
	}
	pkg, _ := svc.CreateDraft(context.Background(), 1001, req)
	svc.Publish(context.Background(), 1001, pkg.ID)

	// 批量更新价格为负数
	batchReq := &BatchUpdatePriceRequest{
		Items: []BatchPriceItem{
			{PackageID: pkg.ID, PricePer1MInput: -0.1, PricePer1MOutput: 1.6},
		},
	}

	resp, err := svc.BatchUpdatePrice(context.Background(), 1001, batchReq)
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
	assert.Equal(t, 0, resp.SuccessCount)
	assert.Equal(t, 1, resp.FailedCount)
	assert.Contains(t, resp.Failures[0].Message, "price cannot be negative")
}

// TestPackageService_BatchUpdatePrice_NotFound 测试批量更新价格-套餐不存在
func TestPackageService_BatchUpdatePrice_NotFound(t *testing.T) {
	pkgStore := newMockPackageStoreForPackageTest()
	acctStore := newMockAccountStoreForPackageTest()
	auditStore := &mockAuditStoreForPackageTest{}

	svc := NewPackageService(pkgStore, acctStore, auditStore)

	batchReq := &BatchUpdatePriceRequest{
		Items: []BatchPriceItem{
			{PackageID: 9999, PricePer1MInput: 0.6, PricePer1MOutput: 1.6},
		},
	}

	resp, err := svc.BatchUpdatePrice(context.Background(), 1001, batchReq)
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
	assert.Equal(t, 0, resp.SuccessCount)
	assert.Equal(t, 1, resp.FailedCount)
	assert.Equal(t, "NOT_FOUND", resp.Failures[0].ErrorCode)
}
