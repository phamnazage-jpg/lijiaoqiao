package domain

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Mock implementations for testing InvariantChecker

type mockAccountStoreForInvariant struct {
	accounts map[int64]*Account
}

func newMockAccountStoreForInvariant() *mockAccountStoreForInvariant {
	return &mockAccountStoreForInvariant{
		accounts: make(map[int64]*Account),
	}
}

func (m *mockAccountStoreForInvariant) Create(ctx context.Context, account *Account) error {
	m.accounts[account.ID] = account
	return nil
}

func (m *mockAccountStoreForInvariant) GetByID(ctx context.Context, supplierID, id int64) (*Account, error) {
	if account, ok := m.accounts[id]; ok && account.SupplierID == supplierID {
		return account, nil
	}
	return nil, errors.New("account not found")
}

func (m *mockAccountStoreForInvariant) Update(ctx context.Context, account *Account) error {
	m.accounts[account.ID] = account
	return nil
}

func (m *mockAccountStoreForInvariant) List(ctx context.Context, supplierID int64) ([]*Account, error) {
	var result []*Account
	for _, account := range m.accounts {
		if account.SupplierID == supplierID {
			result = append(result, account)
		}
	}
	return result, nil
}

type mockPackageStoreForInvariant struct {
	packages map[int64]*Package
}

func newMockPackageStoreForInvariant() *mockPackageStoreForInvariant {
	return &mockPackageStoreForInvariant{
		packages: make(map[int64]*Package),
	}
}

func (m *mockPackageStoreForInvariant) Create(ctx context.Context, pkg *Package) error {
	m.packages[pkg.ID] = pkg
	return nil
}

func (m *mockPackageStoreForInvariant) GetByID(ctx context.Context, supplierID, id int64) (*Package, error) {
	if pkg, ok := m.packages[id]; ok && pkg.SupplierID == supplierID {
		return pkg, nil
	}
	return nil, errors.New("package not found")
}

func (m *mockPackageStoreForInvariant) Update(ctx context.Context, pkg *Package) error {
	m.packages[pkg.ID] = pkg
	return nil
}

func (m *mockPackageStoreForInvariant) List(ctx context.Context, supplierID int64) ([]*Package, error) {
	var result []*Package
	for _, pkg := range m.packages {
		if pkg.SupplierID == supplierID {
			result = append(result, pkg)
		}
	}
	return result, nil
}

type mockSettlementStoreForInvariant struct {
	settlements map[int64]*Settlement
	balances    map[int64]float64
}

func newMockSettlementStoreForInvariant() *mockSettlementStoreForInvariant {
	return &mockSettlementStoreForInvariant{
		settlements: make(map[int64]*Settlement),
		balances:    make(map[int64]float64),
	}
}

func (m *mockSettlementStoreForInvariant) Create(ctx context.Context, s *Settlement) error {
	m.settlements[s.ID] = s
	return nil
}

func (m *mockSettlementStoreForInvariant) GetByID(ctx context.Context, supplierID, id int64) (*Settlement, error) {
	if s, ok := m.settlements[id]; ok && s.SupplierID == supplierID {
		return s, nil
	}
	return nil, errors.New("settlement not found")
}

func (m *mockSettlementStoreForInvariant) Update(ctx context.Context, s *Settlement, expectedVersion int) error {
	m.settlements[s.ID] = s
	return nil
}

func (m *mockSettlementStoreForInvariant) List(ctx context.Context, supplierID int64) ([]*Settlement, error) {
	var result []*Settlement
	for _, s := range m.settlements {
		if s.SupplierID == supplierID {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSettlementStoreForInvariant) GetWithdrawableBalance(ctx context.Context, supplierID int64) (float64, error) {
	if balance, ok := m.balances[supplierID]; ok {
		return balance, nil
	}
	return 0, nil
}

func TestValidateAccountStateTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     AccountStatus
		to       AccountStatus
		expected bool
	}{
		{"pending to active", AccountStatusPending, AccountStatusActive, true},
		{"pending to disabled", AccountStatusPending, AccountStatusDisabled, true},
		{"active to suspended", AccountStatusActive, AccountStatusSuspended, true},
		{"active to disabled", AccountStatusActive, AccountStatusDisabled, true},
		{"suspended to active", AccountStatusSuspended, AccountStatusActive, true},
		{"suspended to disabled", AccountStatusSuspended, AccountStatusDisabled, true},
		{"disabled to active", AccountStatusDisabled, AccountStatusActive, true},
		{"active to pending", AccountStatusActive, AccountStatusPending, false},
		{"suspended to pending", AccountStatusSuspended, AccountStatusPending, false},
		{"disabled to suspended", AccountStatusDisabled, AccountStatusSuspended, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateStateTransition(tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("ValidateStateTransition(%s, %s) = %v, want %v", tt.from, tt.to, result, tt.expected)
			}
		})
	}
}

func TestValidatePackageStateTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     PackageStatus
		to       PackageStatus
		expected bool
	}{
		{"draft to active", PackageStatusDraft, PackageStatusActive, true},
		{"active to paused", PackageStatusActive, PackageStatusPaused, true},
		{"active to sold_out", PackageStatusActive, PackageStatusSoldOut, true},
		{"active to expired", PackageStatusActive, PackageStatusExpired, true},
		{"paused to active", PackageStatusPaused, PackageStatusActive, true},
		{"paused to expired", PackageStatusPaused, PackageStatusExpired, true},
		{"draft to paused", PackageStatusDraft, PackageStatusPaused, false},
		{"sold_out to active", PackageStatusSoldOut, PackageStatusActive, false},
		{"expired to active", PackageStatusExpired, PackageStatusActive, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidatePackageStateTransition(tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("ValidatePackageStateTransition(%s, %s) = %v, want %v", tt.from, tt.to, result, tt.expected)
			}
		})
	}
}

func TestInvariantErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{"account cannot delete active", ErrAccountCannotDeleteActive, "cannot delete active"},
		{"account disabled requires admin", ErrAccountDisabledRequiresAdmin, "disabled account requires admin"},
		{"package sold out system only", ErrPackageSoldOutSystemOnly, "sold_out status"},
		{"package expired cannot restore", ErrPackageExpiredCannotRestore, "expired package cannot"},
		{"settlement cannot cancel", ErrSettlementCannotCancel, "cannot cancel"},
		{"withdraw exceeds balance", ErrWithdrawExceedsBalance, "exceeds available balance"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("expected error but got nil")
			}
			if tt.contains != "" && !containsString(tt.err.Error(), tt.contains) {
				t.Errorf("error = %v, want contains %v", tt.err, tt.contains)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestInvariantViolationStruct 测试不变量违反结构体
func TestInvariantViolationStruct(t *testing.T) {
	violation := &InvariantViolation{
		RuleCode:   "INV-PKG-001",
		ObjectType: "supply_package",
		ObjectID:   123,
		Message:    "test violation",
		OccurredAt: "2024-01-01T00:00:00Z",
	}

	assert.Equal(t, "INV-PKG-001", violation.RuleCode)
	assert.Equal(t, "supply_package", violation.ObjectType)
	assert.Equal(t, int64(123), violation.ObjectID)
	assert.Equal(t, "test violation", violation.Message)
	assert.Equal(t, "2024-01-01T00:00:00Z", violation.OccurredAt)
}

// TestEmitInvariantViolation 测试发射不变量违反事件
func TestEmitInvariantViolation(t *testing.T) {
	err := errors.New("test error")
	violation := EmitInvariantViolation("INV-ACC-001", "supply_account", 456, err)

	assert.Equal(t, "INV-ACC-001", violation.RuleCode)
	assert.Equal(t, "supply_account", violation.ObjectType)
	assert.Equal(t, int64(456), violation.ObjectID)
	assert.Equal(t, "test error", violation.Message)
	assert.Equal(t, "now", violation.OccurredAt)
}

// TestNewInvariantChecker 测试创建不变量检查器
func TestNewInvariantChecker(t *testing.T) {
	// Create a mock invariant checker
	checker := NewInvariantChecker(nil, nil, nil)
	assert.NotNil(t, checker)
}

// TestCheckPackagePrice 测试套餐价格检查
func TestCheckPackagePrice(t *testing.T) {
	checker := &InvariantChecker{}

	tests := []struct {
		name              string
		newPricePer1MInput float64
		newPricePer1MOutput float64
		wantErr           bool
		errContains       string
	}{
		{
			name:               "valid prices",
			newPricePer1MInput: 0.5,
			newPricePer1MOutput: 1.5,
			wantErr:           false,
		},
		{
			name:               "zero input price is allowed",
			newPricePer1MInput: 0.0,
			newPricePer1MOutput: 1.5,
			wantErr:           false,
		},
		{
			name:               "input price below minimum",
			newPricePer1MInput: 0.001,
			newPricePer1MOutput: 1.5,
			wantErr:           true,
			errContains:       "below minimum",
		},
		{
			name:               "output price below minimum",
			newPricePer1MInput: 0.5,
			newPricePer1MOutput: 0.001,
			wantErr:           true,
			errContains:       "below minimum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checker.CheckPackagePrice(nil, nil, tt.newPricePer1MInput, tt.newPricePer1MOutput)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateAccountStateTransition_Invalid 测试无效状态转换
func TestValidateAccountStateTransition_Invalid(t *testing.T) {
	// Test invalid from status
	assert.False(t, ValidateStateTransition(AccountStatus("invalid"), AccountStatusActive))

	// Test to status not in allowed list
	assert.False(t, ValidateStateTransition(AccountStatusPending, AccountStatusSuspended))
	assert.False(t, ValidateStateTransition(AccountStatusActive, AccountStatusPending))
}

// TestValidatePackageStateTransition_Invalid 测试无效套餐状态转换
func TestValidatePackageStateTransition_Invalid(t *testing.T) {
	// Test invalid from status
	assert.False(t, ValidatePackageStateTransition(PackageStatus("invalid"), PackageStatusActive))

	// Test to status not in allowed list
	assert.False(t, ValidatePackageStateTransition(PackageStatusDraft, PackageStatusPaused))
	assert.False(t, ValidatePackageStateTransition(PackageStatusSoldOut, PackageStatusActive))
}

// TestInvariantErrorsAll 测试所有不变量错误
func TestInvariantErrorsAll(t *testing.T) {
	errors := []error{
		ErrAccountCannotDeleteActive,
		ErrAccountDisabledRequiresAdmin,
		ErrPackageSoldOutSystemOnly,
		ErrPackageExpiredCannotRestore,
		ErrPriceBelowProtection,
		ErrSettlementCannotCancel,
		ErrWithdrawExceedsBalance,
		ErrSettlementBalanceMismatch,
	}

	for _, err := range errors {
		assert.NotNil(t, err)
		assert.NotEmpty(t, err.Error())
	}
}

// TestInvariantChecker_CheckAccountDelete 测试账号删除检查
func TestInvariantChecker_CheckAccountDelete(t *testing.T) {
	accountStore := newMockAccountStoreForInvariant()
	checker := NewInvariantChecker(accountStore, nil, nil)

	// Setup: create an active account
	accountStore.accounts[1] = &Account{
		ID:        1,
		SupplierID: 1001,
		Status:    AccountStatusActive,
	}

	// Test: active account cannot be deleted
	err := checker.CheckAccountDelete(context.Background(), 1, 1001)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete active")

	// Setup: change to pending account
	accountStore.accounts[1].Status = AccountStatusPending

	// Test: pending account can be deleted
	err = checker.CheckAccountDelete(context.Background(), 1, 1001)
	assert.NoError(t, err)
}

// TestInvariantChecker_CheckAccountActivate 测试账号激活检查
func TestInvariantChecker_CheckAccountActivate(t *testing.T) {
	accountStore := newMockAccountStoreForInvariant()
	checker := NewInvariantChecker(accountStore, nil, nil)

	// Setup: create a disabled account
	accountStore.accounts[1] = &Account{
		ID:        1,
		SupplierID: 1001,
		Status:    AccountStatusDisabled,
	}

	// Test: disabled account requires admin to activate
	err := checker.CheckAccountActivate(context.Background(), 1, 1001)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled account requires admin")

	// Setup: change to pending account
	accountStore.accounts[1].Status = AccountStatusPending

	// Test: pending account can be activated
	err = checker.CheckAccountActivate(context.Background(), 1, 1001)
	assert.NoError(t, err)
}

// TestInvariantChecker_CheckPackagePublish 测试套餐发布检查
func TestInvariantChecker_CheckPackagePublish(t *testing.T) {
	packageStore := newMockPackageStoreForInvariant()
	checker := NewInvariantChecker(nil, packageStore, nil)

	// Setup: create an expired package
	packageStore.packages[1] = &Package{
		ID:         1,
		SupplierID: 1001,
		Status:    PackageStatusExpired,
	}

	// Test: expired package cannot be directly restored
	err := checker.CheckPackagePublish(context.Background(), 1, 1001)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired package")

	// Setup: change to draft package
	packageStore.packages[1].Status = PackageStatusDraft

	// Test: draft package can be published
	err = checker.CheckPackagePublish(context.Background(), 1, 1001)
	assert.NoError(t, err)
}

// TestInvariantChecker_CheckSettlementCancel 测试结算撤销检查
func TestInvariantChecker_CheckSettlementCancel(t *testing.T) {
	settlementStore := newMockSettlementStoreForInvariant()
	checker := NewInvariantChecker(nil, nil, settlementStore)

	// Setup: create a processing settlement
	settlementStore.settlements[1] = &Settlement{
		ID:         1,
		SupplierID: 1001,
		Status:    SettlementStatusProcessing,
	}

	// Test: processing settlement cannot be cancelled
	err := checker.CheckSettlementCancel(context.Background(), 1, 1001)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot cancel")

	// Setup: change to pending settlement
	settlementStore.settlements[1].Status = SettlementStatusPending

	// Test: pending settlement can be cancelled
	err = checker.CheckSettlementCancel(context.Background(), 1, 1001)
	assert.NoError(t, err)
}

// TestInvariantChecker_CheckWithdrawBalance 测试提现余额检查
func TestInvariantChecker_CheckWithdrawBalance(t *testing.T) {
	settlementStore := newMockSettlementStoreForInvariant()
	checker := NewInvariantChecker(nil, nil, settlementStore)

	// Setup: set balance to 1000
	settlementStore.balances[1001] = 1000.0

	// Test: amount less than balance should pass
	err := checker.CheckWithdrawBalance(context.Background(), 1001, 500.0)
	assert.NoError(t, err)

	// Test: amount equal to balance should pass
	err = checker.CheckWithdrawBalance(context.Background(), 1001, 1000.0)
	assert.NoError(t, err)

	// Test: amount greater than balance should fail
	err = checker.CheckWithdrawBalance(context.Background(), 1001, 1500.0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds available balance")
}

// TestInvariantChecker_NonExistent 测试不存在的实体
func TestInvariantChecker_NonExistent(t *testing.T) {
	accountStore := newMockAccountStoreForInvariant()
	packageStore := newMockPackageStoreForInvariant()
	settlementStore := newMockSettlementStoreForInvariant()
	checker := NewInvariantChecker(accountStore, packageStore, settlementStore)

	// Test non-existent account
	err := checker.CheckAccountDelete(context.Background(), 999, 1001)
	assert.Error(t, err)

	// Test non-existent package
	err = checker.CheckPackagePublish(context.Background(), 999, 1001)
	assert.Error(t, err)

	// Test non-existent settlement
	err = checker.CheckSettlementCancel(context.Background(), 999, 1001)
	assert.Error(t, err)
}
