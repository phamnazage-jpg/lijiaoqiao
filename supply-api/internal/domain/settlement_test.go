package domain

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"lijiaoqiao/supply-api/internal/audit"
)

// mockSettlementStore Mock结算存储
type mockSettlementStore struct {
	settlements map[int64]*Settlement
	balances    map[int64]float64
	nextID      int64
	// 控制 HasPendingOrProcessingWithdraw 的返回值
	hasPendingWithdraw      bool
	hasPendingWithdrawError error
}

func newMockSettlementStore() *mockSettlementStore {
	return &mockSettlementStore{
		settlements: make(map[int64]*Settlement),
		balances:    make(map[int64]float64),
		nextID:      1,
	}
}

func (m *mockSettlementStore) Create(ctx context.Context, s *Settlement) error {
	s.ID = m.nextID
	m.nextID++
	m.settlements[s.ID] = s
	return nil
}

// CreateWithdrawTx 原子化提现创建（mock实现）
func (m *mockSettlementStore) CreateWithdrawTx(ctx context.Context, s *Settlement) error {
	// 检查是否有pending的提现
	if m.hasPendingWithdraw {
		return errors.New("already has pending or processing withdrawal")
	}
	return m.Create(ctx, s)
}

func (m *mockSettlementStore) CreateInTx(ctx context.Context, s *Settlement) error {
	return m.Create(ctx, s)
}

func (m *mockSettlementStore) GetByID(ctx context.Context, supplierID, id int64) (*Settlement, error) {
	if s, ok := m.settlements[id]; ok && s.SupplierID == supplierID {
		return s, nil
	}
	return nil, errors.New("settlement not found")
}

func (m *mockSettlementStore) Update(ctx context.Context, s *Settlement, expectedVersion int) error {
	if s.Version != expectedVersion {
		return errors.New("concurrency conflict")
	}
	m.settlements[s.ID] = s
	return nil
}

func (m *mockSettlementStore) List(ctx context.Context, supplierID int64) ([]*Settlement, error) {
	var result []*Settlement
	for _, s := range m.settlements {
		if s.SupplierID == supplierID {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSettlementStore) GetWithdrawableBalance(ctx context.Context, supplierID int64) (float64, error) {
	if balance, ok := m.balances[supplierID]; ok {
		return balance, nil
	}
	return 0, nil
}

func (m *mockSettlementStore) HasPendingOrProcessingWithdraw(ctx context.Context, supplierID int64) (bool, error) {
	if m.hasPendingWithdrawError != nil {
		return false, m.hasPendingWithdrawError
	}
	return m.hasPendingWithdraw, nil
}

// mockEarningStore Mock收益存储
type mockEarningStore struct {
	records []*EarningRecord
}

func newMockEarningStore() *mockEarningStore {
	return &mockEarningStore{
		records: make([]*EarningRecord, 0),
	}
}

func (m *mockEarningStore) ListRecords(ctx context.Context, supplierID int64, startDate, endDate string, page, pageSize int) ([]*EarningRecord, int, error) {
	var result []*EarningRecord
	for _, r := range m.records {
		if r.SupplierID == supplierID {
			result = append(result, r)
		}
	}
	return result, len(result), nil
}

func (m *mockEarningStore) GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*BillingSummary, error) {
	return &BillingSummary{
		Period: BillingPeriod{
			Start: startDate,
			End:   endDate,
		},
		Summary: BillingTotal{
			TotalRevenue:   1000.00,
			TotalOrders:    100,
			TotalUsage:     5000,
			TotalRequests:  10000,
			AvgSuccessRate: 99.5,
			PlatformFee:    10.00,
			NetEarnings:    990.00,
		},
	}, nil
}

// mockAuditStoreForSettlement Mock审计存储
type mockAuditStoreForSettlement struct{}

func (m *mockAuditStoreForSettlement) Emit(ctx context.Context, event audit.Event) error {
	return nil
}

func (m *mockAuditStoreForSettlement) Query(ctx context.Context, filter audit.EventFilter) ([]audit.Event, error) {
	return nil, nil
}

func (m *mockAuditStoreForSettlement) QueryWithTotal(ctx context.Context, filter audit.EventFilter) ([]audit.Event, int64, error) {
	return nil, 0, nil
}

func (m *mockAuditStoreForSettlement) GetByID(ctx context.Context, eventID string) (audit.Event, error) {
	return audit.Event{}, errors.New("not found")
}

// mockSMSVerifierForSettlement Mock短信验证码验证器
type mockSMSVerifierForSettlement struct {
	verifyResult bool
	verifyError  error
}

func (m *mockSMSVerifierForSettlement) Verify(ctx context.Context, phone string, code string) (bool, error) {
	if m.verifyError != nil {
		return false, m.verifyError
	}
	// Mock 验证器忽略 phone 和 code 参数，只根据 verifyResult 返回
	// 真实测试中应根据具体场景设置 verifyResult
	return m.verifyResult, nil
}

// TestSettlementConstants 测试结算状态常量
func TestSettlementConstants(t *testing.T) {
	assert.Equal(t, SettlementStatus("pending"), SettlementStatusPending)
	assert.Equal(t, SettlementStatus("processing"), SettlementStatusProcessing)
	assert.Equal(t, SettlementStatus("completed"), SettlementStatusCompleted)
	assert.Equal(t, SettlementStatus("failed"), SettlementStatusFailed)
}

// TestPaymentMethodConstants 测试支付方式常量
func TestPaymentMethodConstants(t *testing.T) {
	assert.Equal(t, PaymentMethod("bank"), PaymentMethodBank)
	assert.Equal(t, PaymentMethod("alipay"), PaymentMethodAlipay)
	assert.Equal(t, PaymentMethod("wechat"), PaymentMethodWechat)
}

// TestSettlementStruct 测试结算单结构体
func TestSettlementStruct(t *testing.T) {
	now := time.Now()
	s := &Settlement{
		ID:              1,
		SupplierID:      1001,
		SettlementNo:   "SET-2024-001",
		Status:         SettlementStatusPending,
		TotalAmount:    1000.00,
		FeeAmount:      10.00,
		NetAmount:      990.00,
		PaymentMethod:  PaymentMethodBank,
		PaymentAccount: "1234567890",
		PeriodStart:    now,
		PeriodEnd:      now.Add(24 * time.Hour),
		TotalOrders:   100,
		CurrencyCode:  "CNY",
		AmountUnit:    "yuan",
		Version:       1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	assert.Equal(t, int64(1), s.ID)
	assert.Equal(t, int64(1001), s.SupplierID)
	assert.Equal(t, "SET-2024-001", s.SettlementNo)
	assert.Equal(t, SettlementStatusPending, s.Status)
	assert.Equal(t, 1000.00, s.TotalAmount)
	assert.Equal(t, 10.00, s.FeeAmount)
	assert.Equal(t, 990.00, s.NetAmount)
	assert.Equal(t, PaymentMethodBank, s.PaymentMethod)
	assert.Equal(t, "1234567890", s.PaymentAccount)
	assert.Equal(t, 100, s.TotalOrders)
	assert.Equal(t, "CNY", s.CurrencyCode)
	assert.Equal(t, "yuan", s.AmountUnit)
	assert.Equal(t, 1, s.Version)
}

// TestEarningRecordStruct 测试收益记录结构体
func TestEarningRecordStruct(t *testing.T) {
	now := time.Now()
	e := &EarningRecord{
		ID:           1,
		SupplierID:   1001,
		SettlementID: 10,
		EarningsType: "usage",
		Amount:       500.00,
		Status:       "available",
		Description:  "usage earnings",
		EarnedAt:     now,
	}

	assert.Equal(t, int64(1), e.ID)
	assert.Equal(t, int64(1001), e.SupplierID)
	assert.Equal(t, int64(10), e.SettlementID)
	assert.Equal(t, "usage", e.EarningsType)
	assert.Equal(t, 500.00, e.Amount)
	assert.Equal(t, "available", e.Status)
}

// TestSettlementStatusTransitions 测试结算状态转换
func TestSettlementStatusTransitions(t *testing.T) {
	// 测试有效状态
	s := &Settlement{Status: SettlementStatusPending}
	assert.Equal(t, SettlementStatusPending, s.Status)

	s.Status = SettlementStatusProcessing
	assert.Equal(t, SettlementStatusProcessing, s.Status)

	s.Status = SettlementStatusCompleted
	assert.Equal(t, SettlementStatusCompleted, s.Status)

	s.Status = SettlementStatusFailed
	assert.Equal(t, SettlementStatusFailed, s.Status)
}

// TestInvariantErrors 测试结算相关不变量错误
func TestSettlementInvariantErrors(t *testing.T) {
	// ERRORS from invariants.go related to settlements
	assert.Contains(t, ErrSettlementCannotCancel.Error(), "cannot cancel")
	assert.Contains(t, ErrWithdrawExceedsBalance.Error(), "exceeds available balance")
	assert.Contains(t, ErrSettlementBalanceMismatch.Error(), "does not match balance")
}

// TestNewSettlementService 测试创建结算服务
func TestNewSettlementService(t *testing.T) {
	store := newMockSettlementStore()
	earningStore := newMockEarningStore()
	auditStore := &mockAuditStoreForSettlement{}

	svc := NewSettlementService(store, earningStore, auditStore)
	assert.NotNil(t, svc)
}

// TestSettlementService_Withdraw 测试提现
func TestSettlementService_Withdraw(t *testing.T) {
	store := newMockSettlementStore()
	earningStore := newMockEarningStore()
	auditStore := &mockAuditStoreForSettlement{}

	// 设置余额
	store.balances[1001] = 5000.0

	// 测试无效验证码
	t.Run("invalid sms code", func(t *testing.T) {
		smsVerifier := &mockSMSVerifierForSettlement{verifyResult: false}
		svc := NewSettlementServiceWithSMS(store, earningStore, auditStore, smsVerifier)

		req := &WithdrawRequest{
			Amount:         1000,
			SMSCode:        "000000",
			PaymentMethod:  PaymentMethodBank,
			PaymentAccount: "1234567890",
		}
		result, err := svc.Withdraw(context.Background(), 1001, req)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid sms code")
	})

	// 测试验证码错误
	t.Run("sms verify error", func(t *testing.T) {
		smsVerifier := &mockSMSVerifierForSettlement{verifyError: errors.New("SMS service unavailable")}
		svc := NewSettlementServiceWithSMS(store, earningStore, auditStore, smsVerifier)

		req := &WithdrawRequest{
			Amount:         1000,
			SMSCode:        "123456",
			PaymentMethod:  PaymentMethodBank,
			PaymentAccount: "1234567890",
		}
		result, err := svc.Withdraw(context.Background(), 1001, req)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to verify SMS code")
	})

	// 测试负数金额
	t.Run("negative amount", func(t *testing.T) {
		smsVerifier := &mockSMSVerifierForSettlement{verifyResult: true}
		svc := NewSettlementServiceWithSMS(store, earningStore, auditStore, smsVerifier)

		req := &WithdrawRequest{
			Amount:         -100,
			SMSCode:        "123456",
			PaymentMethod:  PaymentMethodBank,
			PaymentAccount: "1234567890",
		}
		result, err := svc.Withdraw(context.Background(), 1001, req)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "must be positive")
	})

	// 测试零金额
	t.Run("zero amount", func(t *testing.T) {
		smsVerifier := &mockSMSVerifierForSettlement{verifyResult: true}
		svc := NewSettlementServiceWithSMS(store, earningStore, auditStore, smsVerifier)

		req := &WithdrawRequest{
			Amount:         0,
			SMSCode:        "123456",
			PaymentMethod:  PaymentMethodBank,
			PaymentAccount: "1234567890",
		}
		result, err := svc.Withdraw(context.Background(), 1001, req)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "must be positive")
	})

	// 测试超过余额
	t.Run("exceeds balance", func(t *testing.T) {
		smsVerifier := &mockSMSVerifierForSettlement{verifyResult: true}
		svc := NewSettlementServiceWithSMS(store, earningStore, auditStore, smsVerifier)

		req := &WithdrawRequest{
			Amount:         10000,
			SMSCode:        "123456",
			PaymentMethod:  PaymentMethodBank,
			PaymentAccount: "1234567890",
		}
		result, err := svc.Withdraw(context.Background(), 1001, req)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "exceeds available balance")
	})

	// 测试成功提现
	t.Run("success", func(t *testing.T) {
		smsVerifier := &mockSMSVerifierForSettlement{verifyResult: true}
		svc := NewSettlementServiceWithSMS(store, earningStore, auditStore, smsVerifier)

		req := &WithdrawRequest{
			Amount:         1000,
			SMSCode:        "123456",
			PaymentMethod:  PaymentMethodBank,
			PaymentAccount: "1234567890",
		}
		result, err := svc.Withdraw(context.Background(), 1001, req)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, int64(1001), result.SupplierID)
		assert.Equal(t, SettlementStatusPending, result.Status)
		assert.Equal(t, 1000.0, result.TotalAmount)
		assert.Equal(t, 10.0, result.FeeAmount)    // 1% fee
		assert.Equal(t, 990.0, result.NetAmount)   // 99%
	})
}

// TestSettlementService_Withdraw_AlreadyHasPending 测试已有待处理提现时的行为
// P0-03 修复后: 冗余的外部 HasPendingOrProcessingWithdraw 检查已移除
// 原子检查由 CreateWithdrawTx 内部通过 SELECT ... FOR UPDATE SKIP LOCKED 完成
// 本测试验证: 当数据库层因锁冲突拒绝时，错误正确传播
func TestSettlementService_Withdraw_AlreadyHasPending(t *testing.T) {
	store := newMockSettlementStore()
	earningStore := newMockEarningStore()
	auditStore := &mockAuditStoreForSettlement{}
	smsVerifier := &mockSMSVerifierForSettlement{verifyResult: true}

	// 设置已有待处理提现 - 但由于检查已移除，不再阻塞提现
	// 改为测试正常提现流程仍然工作
	store.hasPendingWithdraw = true

	svc := NewSettlementServiceWithSMS(store, earningStore, auditStore, smsVerifier)

	// 设置余额
	store.balances[1001] = 5000.0

	req := &WithdrawRequest{
		Amount:         1000,
		SMSCode:        "123456",
		PaymentMethod:  PaymentMethodBank,
		PaymentAccount: "1234567890",
	}

	result, err := svc.Withdraw(context.Background(), 1001, req)
	// CreateWithdrawTx 在 hasPendingWithdraw=true 时返回错误
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "already has pending or processing withdrawal")
}

// TestSettlementService_Withdraw_AtomicCheck 测试原子化检查（无冗余外部检查）
// P0-03 修复后: HasPendingOrProcessingWithdraw 不再在 Withdraw 中单独调用，
// 检查合并到 CreateWithdrawTx 内部（SELECT ... FOR UPDATE SKIP LOCKED）
func TestSettlementService_Withdraw_AtomicCheck(t *testing.T) {
	store := newMockSettlementStore()
	earningStore := newMockEarningStore()
	auditStore := &mockAuditStoreForSettlement{}
	smsVerifier := &mockSMSVerifierForSettlement{verifyResult: true}

	// 设置 HasPendingOrProcessingWithdraw 错误 - 但现在不会调用它
	store.hasPendingWithdrawError = errors.New("database error")

	svc := NewSettlementServiceWithSMS(store, earningStore, auditStore, smsVerifier)

	// 设置余额
	store.balances[1001] = 5000.0

	req := &WithdrawRequest{
		Amount:         1000,
		SMSCode:        "123456",
		PaymentMethod:  PaymentMethodBank,
		PaymentAccount: "1234567890",
	}

	result, err := svc.Withdraw(context.Background(), 1001, req)
	// 验证正常提现流程：Amount=1000, Fee=1%, NetAmount=990
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1000.0, result.TotalAmount)
	assert.Equal(t, 10.0, result.FeeAmount)    // 1% fee
	assert.Equal(t, 990.0, result.NetAmount)    // 99% of amount
}

// TestSettlementService_Cancel 测试取消结算
func TestSettlementService_Cancel(t *testing.T) {
	store := newMockSettlementStore()
	earningStore := newMockEarningStore()
	auditStore := &mockAuditStoreForSettlement{}

	svc := NewSettlementService(store, earningStore, auditStore)

	// 创建待处理结算
	settlement := &Settlement{
		ID:             1,
		SupplierID:     1001,
		SettlementNo:   "SET-001",
		Status:         SettlementStatusPending,
		TotalAmount:    1000,
		PaymentMethod:  PaymentMethodBank,
		PaymentAccount: "1234567890",
		Version:        1,
	}
	store.Create(context.Background(), settlement)

	// 取消待处理结算应该成功
	canceled, err := svc.Cancel(context.Background(), 1001, 1)
	assert.NoError(t, err)
	assert.NotNil(t, canceled)
	assert.Equal(t, SettlementStatusFailed, canceled.Status)
}

// TestSettlementService_Cancel_ProcessingFails 测试取消处理中结算失败
func TestSettlementService_Cancel_ProcessingFails(t *testing.T) {
	store := newMockSettlementStore()
	earningStore := newMockEarningStore()
	auditStore := &mockAuditStoreForSettlement{}

	svc := NewSettlementService(store, earningStore, auditStore)

	// 创建处理中结算
	settlement := &Settlement{
		ID:             1,
		SupplierID:     1001,
		SettlementNo:   "SET-001",
		Status:         SettlementStatusProcessing,
		TotalAmount:    1000,
		PaymentMethod:  PaymentMethodBank,
		PaymentAccount: "1234567890",
		Version:        1,
	}
	store.Create(context.Background(), settlement)

	// 取消处理中结算应该失败
	_, err := svc.Cancel(context.Background(), 1001, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot cancel")
}

// TestSettlementService_GetByID 测试获取结算单
func TestSettlementService_GetByID(t *testing.T) {
	store := newMockSettlementStore()
	earningStore := newMockEarningStore()
	auditStore := &mockAuditStoreForSettlement{}

	svc := NewSettlementService(store, earningStore, auditStore)

	// 创建结算单
	settlement := &Settlement{
		SupplierID:     1001,
		SettlementNo:   "SET-001",
		Status:         SettlementStatusPending,
		TotalAmount:    1000,
		PaymentMethod:  PaymentMethodBank,
		PaymentAccount: "1234567890",
		Version:        1,
	}
	store.Create(context.Background(), settlement)

	// 获取
	found, err := svc.GetByID(context.Background(), 1001, settlement.ID)
	assert.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, settlement.ID, found.ID)
}

// TestSettlementService_GetByID_NotFound 测试获取不存在的结算单
func TestSettlementService_GetByID_NotFound(t *testing.T) {
	store := newMockSettlementStore()
	earningStore := newMockEarningStore()
	auditStore := &mockAuditStoreForSettlement{}

	svc := NewSettlementService(store, earningStore, auditStore)

	_, err := svc.GetByID(context.Background(), 1001, 9999)
	assert.Error(t, err)
}

// TestSettlementService_List 测试列出结算单
func TestSettlementService_List(t *testing.T) {
	store := newMockSettlementStore()
	earningStore := newMockEarningStore()
	auditStore := &mockAuditStoreForSettlement{}

	svc := NewSettlementService(store, earningStore, auditStore)

	// 创建结算单
	for i := 0; i < 3; i++ {
		settlement := &Settlement{
			SupplierID:     1001,
			SettlementNo:   "SET-00" + string(rune('1'+i)),
			Status:         SettlementStatusPending,
			TotalAmount:    1000 + float64(i)*100,
			PaymentMethod:  PaymentMethodBank,
			PaymentAccount: "1234567890",
			Version:        1,
		}
		store.Create(context.Background(), settlement)
	}

	list, err := svc.List(context.Background(), 1001)
	assert.NoError(t, err)
	assert.Len(t, list, 3)
}

// TestNewEarningService 测试创建收益服务
func TestNewEarningService(t *testing.T) {
	earningStore := newMockEarningStore()

	svc := NewEarningService(earningStore)
	assert.NotNil(t, svc)
}

// TestEarningService_ListRecords 测试列出收益记录
func TestEarningService_ListRecords(t *testing.T) {
	earningStore := newMockEarningStore()

	svc := NewEarningService(earningStore)

	records, total, err := svc.ListRecords(context.Background(), 1001, "2024-01-01", "2024-01-31", 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Len(t, records, 0)
}

// TestEarningService_GetBillingSummary 测试获取账单摘要
func TestEarningService_GetBillingSummary(t *testing.T) {
	earningStore := newMockEarningStore()

	svc := NewEarningService(earningStore)

	summary, err := svc.GetBillingSummary(context.Background(), 1001, "2024-01-01", "2024-01-31")
	assert.NoError(t, err)
	assert.NotNil(t, summary)
	assert.Equal(t, "2024-01-01", summary.Period.Start)
	assert.Equal(t, "2024-01-31", summary.Period.End)
	assert.Equal(t, float64(1000), summary.Summary.TotalRevenue)
}

// TestGenerateSettlementNo 测试生成结算单号
func TestGenerateSettlementNo(t *testing.T) {
	no := generateSettlementNo()

	assert.NotEmpty(t, no)
	// 格式为时间戳 20060102150405
	assert.Equal(t, 14, len(no))
}

// TestSettlementService_GetBillingSummary 测试通过结算服务获取账单摘要
func TestSettlementService_GetBillingSummary(t *testing.T) {
	store := newMockSettlementStore()
	earningStore := newMockEarningStore()
	auditStore := &mockAuditStoreForSettlement{}

	svc := NewSettlementService(store, earningStore, auditStore)

	summary, err := svc.GetBillingSummary(context.Background(), 1001, "2024-01-01", "2024-01-31")
	assert.NoError(t, err)
	assert.NotNil(t, summary)
	assert.Equal(t, "2024-01-01", summary.Period.Start)
	assert.Equal(t, "2024-01-31", summary.Period.End)
	assert.Equal(t, float64(1000), summary.Summary.TotalRevenue)
}
