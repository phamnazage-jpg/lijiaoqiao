package domain

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// TestSettlementService_Withdraw_ConcurrentRequests_RaceCondition
// TDD: 验证提现操作在并发请求下的竞态条件
// 问题: HasPendingOrProcessingWithdraw 和 CreateInTx 不在同一事务中
func TestSettlementService_Withdraw_ConcurrentRequests_RaceCondition(t *testing.T) {
	store := newMockConcurrentSettlementStore()
	earningStore := newMockEarningStore()
	auditStore := &mockAuditStoreForSettlement{}
	smsVerifier := &mockSMSVerifierForSettlement{verifyResult: true}

	// 设置余额充足
	store.balances[1001] = 10000.0

	svc := NewSettlementServiceWithSMS(store, earningStore, auditStore, smsVerifier)

	req := &WithdrawRequest{
		Amount:         1000,
		SMSCode:        "123456",
		PaymentMethod:  PaymentMethodBank,
		PaymentAccount: "1234567890",
	}

	// 并发发起10个提现请求
	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	successCount := 0
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			result, err := svc.Withdraw(context.Background(), 1001, req)
			if err == nil && result != nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// 验证：应该只有0或1个成功（因为有pending check）
	// 但由于竞态条件，可能有多个成功
	t.Logf("成功数量: %d (预期: 1)", successCount)

	// 这个测试在有竞态条件时会 flaky
	// 修复后，成功数量应该是1
	if successCount > 1 {
		t.Errorf("发现竞态条件: %d个请求同时成功 (预期最多1个)", successCount)
	}
}

// mockConcurrentSettlementStore 支持并发的Mock存储
// 模拟真实数据库行为：检查和插入分开时会有竞态条件
type mockConcurrentSettlementStore struct {
	settlements map[int64]*Settlement
	balances    map[int64]float64
	nextID      int64
	mu          sync.Mutex
}

func newMockConcurrentSettlementStore() *mockConcurrentSettlementStore {
	return &mockConcurrentSettlementStore{
		settlements: make(map[int64]*Settlement),
		balances:    make(map[int64]float64),
		nextID:      1,
	}
}

func (m *mockConcurrentSettlementStore) Create(ctx context.Context, s *Settlement) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s.ID = m.nextID
	m.nextID++
	m.settlements[s.ID] = s
	return nil
}

// CreateWithdrawTx 原子化提现创建（带锁）
// 这个实现会先锁定检查，然后原子性插入，模拟真实数据库的 FOR UPDATE SKIP LOCKED
func (m *mockConcurrentSettlementStore) CreateWithdrawTx(ctx context.Context, s *Settlement) error {
	// Step 1: 锁定检查 - 模拟 SELECT ... FOR UPDATE SKIP LOCKED
	m.mu.Lock()
	var hasPending bool
	for _, existing := range m.settlements {
		if existing.SupplierID == s.SupplierID &&
			(existing.Status == SettlementStatusPending || existing.Status == SettlementStatusProcessing) {
			hasPending = true
			break
		}
	}

	if hasPending {
		m.mu.Unlock()
		return errors.New("already has pending or processing withdrawal")
	}

	// Step 2: 原子插入（在锁内完成）
	s.ID = m.nextID
	m.nextID++
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	m.settlements[s.ID] = s
	m.mu.Unlock()

	return nil
}

// CreateInTx 模拟真实事务行为
// 注意：这个实现与旧版DBSettlementStore有相同的竞态问题（检查和插入分开）
func (m *mockConcurrentSettlementStore) CreateInTx(ctx context.Context, s *Settlement) error {
	// 首先检查（模拟 HasPendingOrProcessingWithdraw）
	m.mu.Lock()
	var hasPending bool
	for _, existing := range m.settlements {
		if existing.SupplierID == s.SupplierID && (existing.Status == SettlementStatusPending || existing.Status == SettlementStatusProcessing) {
			hasPending = true
			break
		}
	}
	m.mu.Unlock()

	if hasPending {
		return errors.New("already has pending withdraw")
	}

	// 模拟其他事务在这里插入的时间窗口
	// 提交检查和插入之间有延迟

	// 现在执行插入
	m.mu.Lock()
	defer m.mu.Unlock()

	s.ID = m.nextID
	m.nextID++
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	m.settlements[s.ID] = s
	return nil
}

func (m *mockConcurrentSettlementStore) GetByID(ctx context.Context, supplierID, id int64) (*Settlement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.settlements[id]; ok && s.SupplierID == supplierID {
		return s, nil
	}
	return nil, errors.New("settlement not found")
}

func (m *mockConcurrentSettlementStore) Update(ctx context.Context, s *Settlement, expectedVersion int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s.Version != expectedVersion {
		return errors.New("concurrency conflict")
	}
	m.settlements[s.ID] = s
	return nil
}

func (m *mockConcurrentSettlementStore) List(ctx context.Context, supplierID int64) ([]*Settlement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []*Settlement
	for _, s := range m.settlements {
		if s.SupplierID == supplierID {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockConcurrentSettlementStore) GetWithdrawableBalance(ctx context.Context, supplierID int64) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if balance, ok := m.balances[supplierID]; ok {
		return balance, nil
	}
	return 0, nil
}

func (m *mockConcurrentSettlementStore) HasPendingOrProcessingWithdraw(ctx context.Context, supplierID int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, s := range m.settlements {
		if s.SupplierID == supplierID && (s.Status == SettlementStatusPending || s.Status == SettlementStatusProcessing) {
			return true, nil
		}
	}
	return false, nil
}
