package domain

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"lijiaoqiao/supply-api/internal/audit"
	"lijiaoqiao/supply-api/internal/pkg/logging"
)

// 结算状态
type SettlementStatus string

const (
	SettlementStatusPending    SettlementStatus = "pending"
	SettlementStatusProcessing SettlementStatus = "processing"
	SettlementStatusCompleted  SettlementStatus = "completed"
	SettlementStatusFailed     SettlementStatus = "failed"
)

// 支付方式
type PaymentMethod string

const (
	PaymentMethodBank   PaymentMethod = "bank"
	PaymentMethodAlipay PaymentMethod = "alipay"
	PaymentMethodWechat PaymentMethod = "wechat"
)

// 结算单
type Settlement struct {
	ID                   int64            `json:"settlement_id"`
	SupplierID           int64            `json:"supplier_id"`
	SettlementNo         string           `json:"settlement_no"`
	Status               SettlementStatus `json:"status"`
	TotalAmount          float64          `json:"total_amount"`
	FeeAmount            float64          `json:"fee_amount"`
	NetAmount            float64          `json:"net_amount"`
	PaymentMethod        PaymentMethod    `json:"payment_method"`
	PaymentAccount       string           `json:"payment_account,omitempty"`
	PaymentTransactionID string           `json:"payment_transaction_id,omitempty"`
	PaidAt               *time.Time       `json:"paid_at,omitempty"`

	// 账期 (XR-001)
	PeriodStart       time.Time `json:"period_start"`
	PeriodEnd         time.Time `json:"period_end"`
	TotalOrders       int       `json:"total_orders"`
	TotalUsageRecords int       `json:"total_usage_records"`

	// 单位与币种 (XR-001)
	CurrencyCode string `json:"currency_code"`
	AmountUnit   string `json:"amount_unit"`

	// 幂等字段 (XR-001)
	RequestID      string `json:"request_id,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`

	// 审计字段 (XR-001)
	AuditTraceID string      `json:"audit_trace_id,omitempty"`
	Version      int         `json:"version"`
	CreatedIP    *netip.Addr `json:"created_ip,omitempty"`
	UpdatedIP    *netip.Addr `json:"updated_ip,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// 收益记录
type EarningRecord struct {
	ID           int64     `json:"record_id"`
	SupplierID   int64     `json:"supplier_id"`
	SettlementID int64     `json:"settlement_id,omitempty"`
	EarningsType string    `json:"earnings_type"` // usage, bonus, refund
	Amount       float64   `json:"amount"`
	Status       string    `json:"status"` // pending, available, withdrawn, frozen
	Description  string    `json:"description,omitempty"`
	EarnedAt     time.Time `json:"earned_at"`
}

// 结算服务接口
type SettlementService interface {
	Withdraw(ctx context.Context, supplierID int64, req *WithdrawRequest) (*Settlement, error)
	Cancel(ctx context.Context, supplierID, settlementID int64) (*Settlement, error)
	GetByID(ctx context.Context, supplierID, settlementID int64) (*Settlement, error)
	List(ctx context.Context, supplierID int64) ([]*Settlement, error)
	GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*BillingSummary, error)
}

// 收益服务接口
type EarningService interface {
	ListRecords(ctx context.Context, supplierID int64, startDate, endDate string, page, pageSize int) ([]*EarningRecord, int, error)
	GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*BillingSummary, error)
}

// 提现请求
type WithdrawRequest struct {
	Amount         float64
	PaymentMethod  PaymentMethod
	PaymentAccount string
	SMSCode        string
}

// 账单汇总
type BillingSummary struct {
	Period     BillingPeriod  `json:"period"`
	Summary    BillingTotal   `json:"summary"`
	ByPlatform []PlatformStat `json:"by_platform,omitempty"`
}

type BillingPeriod struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type BillingTotal struct {
	TotalRevenue   float64 `json:"total_revenue"`
	TotalOrders    int     `json:"total_orders"`
	TotalUsage     int64   `json:"total_usage"`
	TotalRequests  int64   `json:"total_requests"`
	AvgSuccessRate float64 `json:"avg_success_rate"`
	PlatformFee    float64 `json:"platform_fee"`
	NetEarnings    float64 `json:"net_earnings"`
}

type PlatformStat struct {
	Platform    string  `json:"platform"`
	Revenue     float64 `json:"revenue"`
	Orders      int     `json:"orders"`
	Tokens      int64   `json:"tokens"`
	SuccessRate float64 `json:"success_rate"`
}

// 结算仓储接口
// P1-005: 乐观锁支持 - Update需要expectedVersion参数防止并发更新
type SettlementStore interface {
	Create(ctx context.Context, s *Settlement) error
	// CreateWithdrawTx 原子化创建提现（带锁防止并发）
	CreateWithdrawTx(ctx context.Context, s *Settlement) error
	// CreateInTx 在事务中创建结算单（非提现场景）
	CreateInTx(ctx context.Context, s *Settlement) error
	GetByID(ctx context.Context, supplierID, id int64) (*Settlement, error)
	// Update 使用乐观锁，expectedVersion是更新前的版本号，如果版本不匹配返回ErrConcurrencyConflict
	Update(ctx context.Context, s *Settlement, expectedVersion int) error
	List(ctx context.Context, supplierID int64) ([]*Settlement, error)
	GetWithdrawableBalance(ctx context.Context, supplierID int64) (float64, error)
	// HasPendingOrProcessingWithdraw 检查是否有待处理或处理中的提现单
	HasPendingOrProcessingWithdraw(ctx context.Context, supplierID int64) (bool, error)
}

// 收益仓储接口
type EarningStore interface {
	ListRecords(ctx context.Context, supplierID int64, startDate, endDate string, page, pageSize int) ([]*EarningRecord, int, error)
	GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*BillingSummary, error)
}

// SMSVerifier 定义短信验证码验证接口
type SMSVerifier interface {
	// Verify 验证短信验证码是否正确
	// phone 手机号, code 验证码
	// 返回: 是否验证通过, 错误信息
	Verify(ctx context.Context, phone string, code string) (bool, error)
}

// DefaultSMSVerifier 默认的短信验证码验证器（安全实现）
// 注意: 默认实现拒绝所有验证码，要求配置真实的SMS服务
type DefaultSMSVerifier struct{}

// ErrSMSServiceNotConfigured SMS服务未配置错误
var ErrSMSServiceNotConfigured = errors.New("SMS service not configured: default verifier cannot validate codes")

// Verify 验证短信验证码 - 默认实现拒绝所有验证码
// 安全设计：默认实现返回错误，强制要求配置真实SMS服务
func (v *DefaultSMSVerifier) Verify(ctx context.Context, phone string, code string) (bool, error) {
	// 默认实现拒绝所有验证码，返回错误表明SMS服务未配置
	// 这防止了硬编码测试码的安全风险
	return false, ErrSMSServiceNotConfigured
}

// 结算服务实现
type settlementService struct {
	store        SettlementStore
	earningStore EarningStore
	auditStore   audit.AuditStore
	smsVerifier  SMSVerifier
}

func NewSettlementService(store SettlementStore, earningStore EarningStore, auditStore audit.AuditStore) SettlementService {
	return &settlementService{
		store:        store,
		earningStore: earningStore,
		auditStore:   auditStore,
		smsVerifier:  resolveSMSVerifier(nil),
	}
}

// NewSettlementServiceWithSMS 创建支持真实SMS服务的结算服务
func NewSettlementServiceWithSMS(store SettlementStore, earningStore EarningStore, auditStore audit.AuditStore, smsVerifier SMSVerifier) SettlementService {
	return &settlementService{
		store:        store,
		earningStore: earningStore,
		auditStore:   auditStore,
		smsVerifier:  resolveSMSVerifier(smsVerifier),
	}
}

func resolveSMSVerifier(verifier SMSVerifier) SMSVerifier {
	if verifier == nil {
		return &DefaultSMSVerifier{}
	}
	return verifier
}

// emitAudit 安全记录审计日志（失败只记录错误，不影响主流程）
func (s *settlementService) emitAudit(ctx context.Context, event audit.Event) {
	if err := s.auditStore.Emit(ctx, event); err != nil {
		logger := logging.NewLogger("supply-api", logging.LogLevelError)
		logger.Error("failed to emit audit event", map[string]interface{}{
			"error":       err.Error(),
			"object_type": event.ObjectType,
			"object_id":   event.ObjectID,
			"action":      event.Action,
		})
	}
}

func (s *settlementService) Withdraw(ctx context.Context, supplierID int64, req *WithdrawRequest) (*Settlement, error) {
	// 使用SMS验证码验证器验证
	valid, err := s.smsVerifier.Verify(ctx, req.PaymentAccount, req.SMSCode)
	if err != nil {
		return nil, fmt.Errorf("failed to verify SMS code: %w", err)
	}
	if !valid {
		return nil, errors.New("invalid sms code")
	}

	// INV-SET-004: 检查是否已有待处理或处理中的提现
	// 注意: CreateWithdrawTx 内部已使用 SELECT ... FOR UPDATE SKIP LOCKED 做原子化检查，
	// 这里不再单独检查，避免检查和插入之间的竞态窗口
	// 双重检查会导致：请求A检查通过 -> 请求B检查通过 -> 请求A插入 -> 请求B插入失败
	// 正确做法：CreateWithdrawTx 内部原子检查并插入

	// 验证金额：必须为正数
	if req.Amount <= 0 {
		return nil, errors.New("SUP_SET_4003: withdraw amount must be positive")
	}

	balance, err := s.store.GetWithdrawableBalance(ctx, supplierID)
	if err != nil {
		return nil, err
	}

	if req.Amount > balance {
		return nil, errors.New("SUP_SET_4001: withdraw amount exceeds available balance")
	}

	settlement := &Settlement{
		SupplierID:     supplierID,
		SettlementNo:   generateSettlementNo(),
		Status:         SettlementStatusPending,
		TotalAmount:    req.Amount,
		FeeAmount:      req.Amount * 0.01, // 1% fee
		NetAmount:      req.Amount * 0.99,
		PaymentMethod:  req.PaymentMethod,
		PaymentAccount: req.PaymentAccount,
		Version:        1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// P0-02修复: 使用原子化提现创建（带锁）
	// CreateWithdrawTx 使用 SELECT ... FOR UPDATE SKIP LOCKED 防止并发提现
	if err := s.store.CreateWithdrawTx(ctx, settlement); err != nil {
		return nil, err
	}

	s.emitAudit(ctx, audit.Event{
		TenantID:   supplierID,
		ObjectType: "supply_settlement",
		ObjectID:   settlement.ID,
		Action:     "withdraw",
		ResultCode: "OK",
	})

	return settlement, nil
}

func (s *settlementService) Cancel(ctx context.Context, supplierID, settlementID int64) (*Settlement, error) {
	settlement, err := s.store.GetByID(ctx, supplierID, settlementID)
	if err != nil {
		return nil, err
	}

	if settlement.Status == SettlementStatusProcessing || settlement.Status == SettlementStatusCompleted {
		return nil, errors.New("SUP_SET_4092: cannot cancel processing or completed settlements")
	}

	// 保存更新前的版本号用于乐观锁
	expectedVersion := settlement.Version

	settlement.Status = SettlementStatusFailed
	settlement.UpdatedAt = time.Now()
	// 注意：Version++由Repository的Update方法自动处理

	if err := s.store.Update(ctx, settlement, expectedVersion); err != nil {
		return nil, err
	}

	s.emitAudit(ctx, audit.Event{
		TenantID:   supplierID,
		ObjectType: "supply_settlement",
		ObjectID:   settlementID,
		Action:     "cancel",
		ResultCode: "OK",
	})

	// 重新获取更新后的settlement
	return s.store.GetByID(ctx, supplierID, settlementID)
}

func (s *settlementService) GetByID(ctx context.Context, supplierID, settlementID int64) (*Settlement, error) {
	return s.store.GetByID(ctx, supplierID, settlementID)
}

func (s *settlementService) List(ctx context.Context, supplierID int64) ([]*Settlement, error) {
	return s.store.List(ctx, supplierID)
}

func (s *settlementService) GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*BillingSummary, error) {
	return s.earningStore.GetBillingSummary(ctx, supplierID, startDate, endDate)
}

// 收益服务实现
type earningService struct {
	store EarningStore
}

func NewEarningService(store EarningStore) EarningService {
	return &earningService{store: store}
}

func (s *earningService) ListRecords(ctx context.Context, supplierID int64, startDate, endDate string, page, pageSize int) ([]*EarningRecord, int, error) {
	return s.store.ListRecords(ctx, supplierID, startDate, endDate, page, pageSize)
}

func (s *earningService) GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*BillingSummary, error) {
	return s.store.GetBillingSummary(ctx, supplierID, startDate, endDate)
}

func generateSettlementNo() string {
	return time.Now().Format("20060102150405")
}
