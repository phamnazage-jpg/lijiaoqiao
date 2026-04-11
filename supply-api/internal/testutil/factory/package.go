package factory

import (
	"time"

	"lijiaoqiao/supply-api/internal/domain"
)

// PackageFactory 套餐测试数据工厂
type PackageFactory struct {
	supplierID       int64
	accountID        int64
	model            string
	totalQuota       float64
	availableQuota   float64
	soldQuota        float64
	reservedQuota    float64
	pricePer1MInput  float64
	pricePer1MOutput float64
	validDays        int
	maxConcurrent    int
	rateLimitRPM     int
	status           domain.PackageStatus
	quotaUnit        string
	priceUnit        string
	currencyCode     string
}

// NewPackageFactory 创建默认套餐工厂
func NewPackageFactory() *PackageFactory {
	return &PackageFactory{
		supplierID:       1001,
		accountID:        1,
		model:            "gpt-4o-mini",
		totalQuota:       1000000,
		availableQuota:   1000000,
		soldQuota:        0,
		reservedQuota:    0,
		pricePer1MInput:  0.5,
		pricePer1MOutput: 1.5,
		validDays:        30,
		maxConcurrent:    10,
		rateLimitRPM:     1000,
		status:           domain.PackageStatusDraft,
		quotaUnit:        "tokens",
		priceUnit:        "USD",
		currencyCode:     "USDT",
	}
}

// WithSupplierID 设置供应商ID
func (f *PackageFactory) WithSupplierID(id int64) *PackageFactory {
	f.supplierID = id
	return f
}

// WithAccountID 设置账户ID
func (f *PackageFactory) WithAccountID(id int64) *PackageFactory {
	f.accountID = id
	return f
}

// WithModel 设置模型
func (f *PackageFactory) WithModel(model string) *PackageFactory {
	f.model = model
	return f
}

// WithQuota 设置配额
func (f *PackageFactory) WithQuota(total, available float64) *PackageFactory {
	f.totalQuota = total
	f.availableQuota = available
	return f
}

// WithPrice 设置价格
func (f *PackageFactory) WithPrice(input, output float64) *PackageFactory {
	f.pricePer1MInput = input
	f.pricePer1MOutput = output
	return f
}

// WithValidDays 设置有效期
func (f *PackageFactory) WithValidDays(days int) *PackageFactory {
	f.validDays = days
	return f
}

// WithStatus 设置状态
func (f *PackageFactory) WithStatus(status domain.PackageStatus) *PackageFactory {
	f.status = status
	return f
}

// BuildRequest 构建创建套餐草稿请求
func (f *PackageFactory) BuildRequest() *domain.CreatePackageDraftRequest {
	return &domain.CreatePackageDraftRequest{
		SupplierID:       f.supplierID,
		AccountID:        f.accountID,
		Model:            f.model,
		TotalQuota:       f.totalQuota,
		PricePer1MInput:  f.pricePer1MInput,
		PricePer1MOutput: f.pricePer1MOutput,
		ValidDays:        f.validDays,
		MaxConcurrent:    f.maxConcurrent,
		RateLimitRPM:     f.rateLimitRPM,
	}
}

// Build 构建套餐对象
func (f *PackageFactory) Build() *domain.Package {
	now := time.Now()
	return &domain.Package{
		ID:                1,
		SupplierID:        f.supplierID,
		AccountID:         f.accountID,
		Model:             f.model,
		TotalQuota:        f.totalQuota,
		AvailableQuota:    f.availableQuota,
		SoldQuota:         f.soldQuota,
		ReservedQuota:     f.reservedQuota,
		PricePer1MInput:   f.pricePer1MInput,
		PricePer1MOutput:  f.pricePer1MOutput,
		ValidDays:         f.validDays,
		MaxConcurrent:     f.maxConcurrent,
		RateLimitRPM:      f.rateLimitRPM,
		Status:            f.status,
		TotalOrders:       0,
		TotalRevenue:      0,
		Rating:            0,
		RatingCount:       0,
		QuotaUnit:         f.quotaUnit,
		PriceUnit:         f.priceUnit,
		CurrencyCode:      f.currencyCode,
		Version:           1,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// BuildActive 构建活跃状态的套餐
func (f *PackageFactory) BuildActive() *domain.Package {
	pkg := f.Build()
	pkg.Status = domain.PackageStatusActive
	return pkg
}

// BuildPaused 构建暂停状态的套餐
func (f *PackageFactory) BuildPaused() *domain.Package {
	pkg := f.Build()
	pkg.Status = domain.PackageStatusPaused
	return pkg
}

// BuildSoldOut 构建售罄状态的套餐
func (f *PackageFactory) BuildSoldOut() *domain.Package {
	pkg := f.Build()
	pkg.Status = domain.PackageStatusSoldOut
	pkg.AvailableQuota = 0
	return pkg
}

// BuildExpired 构建过期状态的套餐
func (f *PackageFactory) BuildExpired() *domain.Package {
	pkg := f.Build()
	pkg.Status = domain.PackageStatusExpired
	return pkg
}

// BuildWithID 构建带指定ID的套餐
func (f *PackageFactory) BuildWithID(id int64) *domain.Package {
	pkg := f.Build()
	pkg.ID = id
	return pkg
}
