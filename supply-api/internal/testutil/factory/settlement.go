package factory

import (
	"time"

	"lijiaoqiao/supply-api/internal/domain"
)

// SettlementFactory 结算测试数据工厂
type SettlementFactory struct {
	supplierID           int64
	settlementNo         string
	status               domain.SettlementStatus
	totalAmount          float64
	feeAmount            float64
	netAmount            float64
	paymentMethod        domain.PaymentMethod
	paymentAccount       string
	paymentTransactionID string
	periodStart          time.Time
	periodEnd            time.Time
	totalOrders          int
	totalUsageRecords    int
	currencyCode         string
	amountUnit           string
	requestID            string
	idempotencyKey      string
}

// NewSettlementFactory 创建默认结算工厂
func NewSettlementFactory() *SettlementFactory {
	now := time.Now()
	return &SettlementFactory{
		supplierID:     1001,
		settlementNo:   "SET" + now.Format("20060102150405"),
		status:         domain.SettlementStatusPending,
		totalAmount:    1000.00,
		feeAmount:      10.00,
		netAmount:      990.00,
		paymentMethod:  domain.PaymentMethodBank,
		paymentAccount: "bank-1234567890",
		periodStart:    now.AddDate(0, 0, -30),
		periodEnd:      now,
		totalOrders:    100,
		totalUsageRecords: 1000,
		currencyCode:   "USDT",
		amountUnit:     "USD",
	}
}

// WithSupplierID 设置供应商ID
func (f *SettlementFactory) WithSupplierID(id int64) *SettlementFactory {
	f.supplierID = id
	return f
}

// WithStatus 设置状态
func (f *SettlementFactory) WithStatus(status domain.SettlementStatus) *SettlementFactory {
	f.status = status
	return f
}

// WithAmount 设置金额
func (f *SettlementFactory) WithAmount(total, fee, net float64) *SettlementFactory {
	f.totalAmount = total
	f.feeAmount = fee
	f.netAmount = net
	return f
}

// WithPaymentMethod 设置支付方式
func (f *SettlementFactory) WithPaymentMethod(method domain.PaymentMethod, account string) *SettlementFactory {
	f.paymentMethod = method
	f.paymentAccount = account
	return f
}

// WithPeriod 设置账期
func (f *SettlementFactory) WithPeriod(start, end time.Time) *SettlementFactory {
	f.periodStart = start
	f.periodEnd = end
	return f
}

// WithIdempotencyKey 设置幂等键
func (f *SettlementFactory) WithIdempotencyKey(key string) *SettlementFactory {
	f.idempotencyKey = key
	return f
}

// Build 构建结算单对象
func (f *SettlementFactory) Build() *domain.Settlement {
	now := time.Now()
	return &domain.Settlement{
		ID:                   1,
		SupplierID:           f.supplierID,
		SettlementNo:         f.settlementNo,
		Status:               f.status,
		TotalAmount:          f.totalAmount,
		FeeAmount:            f.feeAmount,
		NetAmount:            f.netAmount,
		PaymentMethod:        f.paymentMethod,
		PaymentAccount:       f.paymentAccount,
		PaymentTransactionID: f.paymentTransactionID,
		PeriodStart:          f.periodStart,
		PeriodEnd:            f.periodEnd,
		TotalOrders:          f.totalOrders,
		TotalUsageRecords:    f.totalUsageRecords,
		CurrencyCode:         f.currencyCode,
		AmountUnit:           f.amountUnit,
		RequestID:            f.requestID,
		IdempotencyKey:       f.idempotencyKey,
		Version:              1,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

// BuildPending 构建待处理状态的结算单
func (f *SettlementFactory) BuildPending() *domain.Settlement {
	s := f.Build()
	s.Status = domain.SettlementStatusPending
	return s
}

// BuildProcessing 构建处理中的结算单
func (f *SettlementFactory) BuildProcessing() *domain.Settlement {
	s := f.Build()
	s.Status = domain.SettlementStatusProcessing
	return s
}

// BuildCompleted 构建已完成状态的结算单
func (f *SettlementFactory) BuildCompleted() *domain.Settlement {
	s := f.Build()
	s.Status = domain.SettlementStatusCompleted
	now := time.Now()
	s.PaidAt = &now
	return s
}

// BuildFailed 构建失败状态的结算单
func (f *SettlementFactory) BuildFailed() *domain.Settlement {
	s := f.Build()
	s.Status = domain.SettlementStatusFailed
	return s
}

// BuildWithID 构建带指定ID的结算单
func (f *SettlementFactory) BuildWithID(id int64) *domain.Settlement {
	s := f.Build()
	s.ID = id
	return s
}

// WithdrawRequest 构建提现请求
func (f *SettlementFactory) WithdrawRequest() *domain.WithdrawRequest {
	return &domain.WithdrawRequest{
		Amount:         f.totalAmount,
		PaymentMethod:  f.paymentMethod,
		PaymentAccount: f.paymentAccount,
		SMSCode:       "123456",
	}
}

// WithdrawRequestWithCode 构建带指定短信验证码的提现请求
func (f *SettlementFactory) WithdrawRequestWithCode(smsCode string) *domain.WithdrawRequest {
	return &domain.WithdrawRequest{
		Amount:         f.totalAmount,
		PaymentMethod:  f.paymentMethod,
		PaymentAccount: f.paymentAccount,
		SMSCode:       smsCode,
	}
}
