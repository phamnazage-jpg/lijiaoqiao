package domain

import (
	"context"
	"errors"
	"log"
	"net/netip"
	"time"

	"lijiaoqiao/supply-api/internal/audit"
)

// 套餐状态
type PackageStatus string

const (
	PackageStatusDraft   PackageStatus = "draft"
	PackageStatusActive  PackageStatus = "active"
	PackageStatusPaused  PackageStatus = "paused"
	PackageStatusSoldOut PackageStatus = "sold_out"
	PackageStatusExpired PackageStatus = "expired"
)

// 套餐
type Package struct {
	ID               int64         `json:"package_id"`
	SupplierID       int64         `json:"supply_account_id"`
	AccountID        int64         `json:"account_id,omitempty"`
	Platform         string        `json:"platform,omitempty"`
	Model            string        `json:"model"`
	TotalQuota       float64       `json:"total_quota"`
	AvailableQuota   float64       `json:"available_quota"`
	SoldQuota        float64       `json:"sold_quota"`
	ReservedQuota    float64       `json:"reserved_quota"`
	PricePer1MInput  float64       `json:"price_per_1m_input"`
	PricePer1MOutput float64       `json:"price_per_1m_output"`
	MinPurchase      float64       `json:"min_purchase,omitempty"`
	StartAt          time.Time     `json:"start_at,omitempty"`
	EndAt            time.Time     `json:"end_at,omitempty"`
	ValidDays        int           `json:"valid_days"`
	MaxConcurrent    int           `json:"max_concurrent,omitempty"`
	RateLimitRPM     int           `json:"rate_limit_rpm,omitempty"`
	Status           PackageStatus `json:"status"`
	TotalOrders      int           `json:"total_orders"`
	TotalRevenue     float64       `json:"total_revenue"`
	Rating           float64       `json:"rating"`
	RatingCount      int           `json:"rating_count"`

	// 单位与币种 (XR-001)
	QuotaUnit    string `json:"quota_unit"`
	PriceUnit    string `json:"price_unit"`
	CurrencyCode string `json:"currency_code"`

	// 审计字段 (XR-001)
	Version      int         `json:"version"`
	CreatedIP    *netip.Addr `json:"created_ip,omitempty"`
	UpdatedIP    *netip.Addr `json:"updated_ip,omitempty"`
	AuditTraceID string      `json:"audit_trace_id,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// 套餐服务接口
type PackageService interface {
	CreateDraft(ctx context.Context, supplierID int64, req *CreatePackageDraftRequest) (*Package, error)
	Publish(ctx context.Context, supplierID, packageID int64) (*Package, error)
	Pause(ctx context.Context, supplierID, packageID int64) (*Package, error)
	Unlist(ctx context.Context, supplierID, packageID int64) (*Package, error)
	Clone(ctx context.Context, supplierID, packageID int64) (*Package, error)
	BatchUpdatePrice(ctx context.Context, supplierID int64, req *BatchUpdatePriceRequest) (*BatchUpdatePriceResponse, error)
	GetByID(ctx context.Context, supplierID, packageID int64) (*Package, error)
}

// 创建套餐草稿请求
type CreatePackageDraftRequest struct {
	SupplierID       int64
	AccountID        int64
	Model            string
	TotalQuota       float64
	PricePer1MInput  float64
	PricePer1MOutput float64
	ValidDays        int
	MaxConcurrent    int
	RateLimitRPM     int
}

// 批量调价请求
type BatchUpdatePriceRequest struct {
	Items []BatchPriceItem `json:"items"`
}

type BatchPriceItem struct {
	PackageID        int64   `json:"package_id"`
	PricePer1MInput  float64 `json:"price_per_1m_input"`
	PricePer1MOutput float64 `json:"price_per_1m_output"`
}

// 批量调价响应
type BatchUpdatePriceResponse struct {
	Total        int                 `json:"total"`
	SuccessCount int                 `json:"success_count"`
	FailedCount  int                 `json:"failed_count"`
	Failures     []BatchPriceFailure `json:"failures,omitempty"`
}

type BatchPriceFailure struct {
	PackageID int64  `json:"package_id"`
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

// 套餐仓储接口
type PackageStore interface {
	Create(ctx context.Context, pkg *Package) error
	GetByID(ctx context.Context, supplierID, id int64) (*Package, error)
	Update(ctx context.Context, pkg *Package) error
	List(ctx context.Context, supplierID int64) ([]*Package, error)
}

// 套餐服务实现
type packageService struct {
	store        PackageStore
	accountStore AccountStore
	auditStore   audit.AuditStore
}

func NewPackageService(store PackageStore, accountStore AccountStore, auditStore audit.AuditStore) PackageService {
	return &packageService{
		store:        store,
		accountStore: accountStore,
		auditStore:   auditStore,
	}
}

// emitAudit 安全记录审计日志（失败只记录错误，不影响主流程）
func (s *packageService) emitAudit(ctx context.Context, event audit.Event) {
	if err := s.auditStore.Emit(ctx, event); err != nil {
		log.Printf("[AUDIT_ERROR] failed to emit audit event: %v, object_type=%s, object_id=%d, action=%s",
			err, event.ObjectType, event.ObjectID, event.Action)
	}
}

func (s *packageService) CreateDraft(ctx context.Context, supplierID int64, req *CreatePackageDraftRequest) (*Package, error) {
	pkg := &Package{
		SupplierID:       supplierID,
		AccountID:        req.AccountID,
		Model:            req.Model,
		TotalQuota:       req.TotalQuota,
		AvailableQuota:   req.TotalQuota,
		PricePer1MInput:  req.PricePer1MInput,
		PricePer1MOutput: req.PricePer1MOutput,
		ValidDays:        req.ValidDays,
		MaxConcurrent:    req.MaxConcurrent,
		RateLimitRPM:     req.RateLimitRPM,
		Status:           PackageStatusDraft,
		Version:          1,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := s.store.Create(ctx, pkg); err != nil {
		return nil, err
	}

	s.emitAudit(ctx, audit.Event{
		TenantID:   supplierID,
		ObjectType: "supply_package",
		ObjectID:   pkg.ID,
		Action:     "create_draft",
		ResultCode: "OK",
	})

	return pkg, nil
}

func (s *packageService) Publish(ctx context.Context, supplierID, packageID int64) (*Package, error) {
	pkg, err := s.store.GetByID(ctx, supplierID, packageID)
	if err != nil {
		return nil, err
	}

	if pkg.Status != PackageStatusDraft && pkg.Status != PackageStatusPaused {
		return nil, errors.New("SUP_PKG_4092: can only publish draft or paused packages")
	}

	pkg.Status = PackageStatusActive
	pkg.UpdatedAt = time.Now()
	pkg.Version++

	if err := s.store.Update(ctx, pkg); err != nil {
		return nil, err
	}

	s.emitAudit(ctx, audit.Event{
		TenantID:   supplierID,
		ObjectType: "supply_package",
		ObjectID:   packageID,
		Action:     "publish",
		ResultCode: "OK",
	})

	return pkg, nil
}

func (s *packageService) Pause(ctx context.Context, supplierID, packageID int64) (*Package, error) {
	pkg, err := s.store.GetByID(ctx, supplierID, packageID)
	if err != nil {
		return nil, err
	}

	if pkg.Status != PackageStatusActive {
		return nil, errors.New("SUP_PKG_4092: can only pause active packages")
	}

	pkg.Status = PackageStatusPaused
	pkg.UpdatedAt = time.Now()
	pkg.Version++

	if err := s.store.Update(ctx, pkg); err != nil {
		return nil, err
	}

	s.emitAudit(ctx, audit.Event{
		TenantID:   supplierID,
		ObjectType: "supply_package",
		ObjectID:   packageID,
		Action:     "pause",
		ResultCode: "OK",
	})

	return pkg, nil
}

func (s *packageService) Unlist(ctx context.Context, supplierID, packageID int64) (*Package, error) {
	pkg, err := s.store.GetByID(ctx, supplierID, packageID)
	if err != nil {
		return nil, err
	}

	pkg.Status = PackageStatusExpired
	pkg.UpdatedAt = time.Now()
	pkg.Version++

	if err := s.store.Update(ctx, pkg); err != nil {
		return nil, err
	}

	s.emitAudit(ctx, audit.Event{
		TenantID:   supplierID,
		ObjectType: "supply_package",
		ObjectID:   packageID,
		Action:     "unlist",
		ResultCode: "OK",
	})

	return pkg, nil
}

func (s *packageService) Clone(ctx context.Context, supplierID, packageID int64) (*Package, error) {
	original, err := s.store.GetByID(ctx, supplierID, packageID)
	if err != nil {
		return nil, err
	}

	clone := &Package{
		SupplierID:       supplierID,
		AccountID:        original.AccountID,
		Model:            original.Model,
		TotalQuota:       original.TotalQuota,
		AvailableQuota:   original.TotalQuota,
		PricePer1MInput:  original.PricePer1MInput,
		PricePer1MOutput: original.PricePer1MOutput,
		ValidDays:        original.ValidDays,
		MaxConcurrent:    original.MaxConcurrent,
		RateLimitRPM:     original.RateLimitRPM,
		Status:           PackageStatusDraft,
		Version:          1,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := s.store.Create(ctx, clone); err != nil {
		return nil, err
	}

	s.emitAudit(ctx, audit.Event{
		TenantID:   supplierID,
		ObjectType: "supply_package",
		ObjectID:   clone.ID,
		Action:     "clone",
		ResultCode: "OK",
	})

	return clone, nil
}

func (s *packageService) BatchUpdatePrice(ctx context.Context, supplierID int64, req *BatchUpdatePriceRequest) (*BatchUpdatePriceResponse, error) {
	resp := &BatchUpdatePriceResponse{
		Total: len(req.Items),
	}

	for _, item := range req.Items {
		// 验证价格不能为负数
		if item.PricePer1MInput < 0 || item.PricePer1MOutput < 0 {
			resp.FailedCount++
			resp.Failures = append(resp.Failures, BatchPriceFailure{
				PackageID: item.PackageID,
				ErrorCode: "SUP_PKG_4004",
				Message:   "price cannot be negative",
			})
			continue
		}

		pkg, err := s.store.GetByID(ctx, supplierID, item.PackageID)
		if err != nil {
			resp.FailedCount++
			resp.Failures = append(resp.Failures, BatchPriceFailure{
				PackageID: item.PackageID,
				ErrorCode: "NOT_FOUND",
				Message:   err.Error(),
			})
			continue
		}

		if pkg.Status == PackageStatusSoldOut || pkg.Status == PackageStatusExpired {
			resp.FailedCount++
			resp.Failures = append(resp.Failures, BatchPriceFailure{
				PackageID: item.PackageID,
				ErrorCode: "SUP_PKG_4093",
				Message:   "cannot update price for sold_out or expired packages",
			})
			continue
		}

		pkg.PricePer1MInput = item.PricePer1MInput
		pkg.PricePer1MOutput = item.PricePer1MOutput
		pkg.UpdatedAt = time.Now()
		pkg.Version++

		if err := s.store.Update(ctx, pkg); err != nil {
			resp.FailedCount++
			resp.Failures = append(resp.Failures, BatchPriceFailure{
				PackageID: item.PackageID,
				ErrorCode: "UPDATE_FAILED",
				Message:   err.Error(),
			})
			continue
		}

		resp.SuccessCount++
	}

	return resp, nil
}

func (s *packageService) GetByID(ctx context.Context, supplierID, packageID int64) (*Package, error) {
	return s.store.GetByID(ctx, supplierID, packageID)
}
