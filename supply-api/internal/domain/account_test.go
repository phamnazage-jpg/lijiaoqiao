package domain

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"lijiaoqiao/supply-api/internal/audit"
)

// mockAccountStore Mock账号存储
type mockAccountStore struct {
	accounts map[int64]*Account
	nextID   int64
}

func newMockAccountStore() *mockAccountStore {
	return &mockAccountStore{
		accounts: make(map[int64]*Account),
		nextID:   1,
	}
}

func (m *mockAccountStore) Create(ctx context.Context, account *Account) error {
	account.ID = m.nextID
	m.nextID++
	m.accounts[account.ID] = account
	return nil
}

func (m *mockAccountStore) GetByID(ctx context.Context, supplierID, id int64) (*Account, error) {
	if account, ok := m.accounts[id]; ok && account.SupplierID == supplierID {
		return account, nil
	}
	return nil, errors.New("account not found")
}

func (m *mockAccountStore) Update(ctx context.Context, account *Account) error {
	if _, ok := m.accounts[account.ID]; ok {
		m.accounts[account.ID] = account
		return nil
	}
	return errors.New("account not found")
}

func (m *mockAccountStore) List(ctx context.Context, supplierID int64) ([]*Account, error) {
	var result []*Account
	for _, account := range m.accounts {
		if account.SupplierID == supplierID {
			result = append(result, account)
		}
	}
	return result, nil
}

// mockAuditStore Mock审计存储
type mockAuditStore struct{}

func (m *mockAuditStore) Emit(ctx context.Context, event audit.Event) error {
	return nil
}

func (m *mockAuditStore) Query(ctx context.Context, filter audit.EventFilter) ([]audit.Event, error) {
	return nil, nil
}

func (m *mockAuditStore) QueryWithTotal(ctx context.Context, filter audit.EventFilter) ([]audit.Event, int64, error) {
	return nil, 0, nil
}

func (m *mockAuditStore) GetByID(ctx context.Context, eventID string) (audit.Event, error) {
	return audit.Event{}, errors.New("not found")
}

func TestAccountService_Create(t *testing.T) {
	store := newMockAccountStore()
	auditStore := &mockAuditStore{}
	svc := NewAccountService(store, auditStore)

	tests := []struct {
		name    string
		req     *CreateAccountRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "create account success",
			req: &CreateAccountRequest{
				SupplierID:  1001,
				Provider:    ProviderOpenAI,
				AccountType: AccountTypeAPIKey,
				Credential:  "sk-test-key-12345",
				Alias:       "test-account",
				RiskAck:     true,
			},
			wantErr: false,
		},
		{
			name: "create account without risk ack",
			req: &CreateAccountRequest{
				SupplierID:  1001,
				Provider:    ProviderOpenAI,
				AccountType: AccountTypeAPIKey,
				Credential:  "sk-test-key-12345",
				RiskAck:     false,
			},
			wantErr: true,
			errMsg:  "risk_ack is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account, err := svc.Create(context.Background(), tt.req)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, account)
				assert.Equal(t, tt.req.SupplierID, account.SupplierID)
				assert.Equal(t, tt.req.Provider, account.Provider)
				assert.Equal(t, tt.req.AccountType, account.AccountType)
				assert.Equal(t, AccountStatusPending, account.Status)
				assert.NotEmpty(t, account.CredentialHash)
				assert.True(t, account.Version == 1)
			}
		})
	}
}

func TestAccountService_Activate(t *testing.T) {
	store := newMockAccountStore()
	auditStore := &mockAuditStore{}
	svc := NewAccountService(store, auditStore)

	tests := []struct {
		name       string
		setup      func() *Account
		supplierID int64
		accountID  int64
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "activate pending account success",
			supplierID: 1001,
			setup: func() *Account {
				account := &Account{
					SupplierID: 1001,
					Provider:   ProviderOpenAI,
					AccountType: AccountTypeAPIKey,
					Status:     AccountStatusPending,
					Version:   1,
				}
				store.Create(context.Background(), account)
				return account
			},
			wantErr: false,
		},
		{
			name:       "activate suspended account success",
			supplierID: 1001,
			setup: func() *Account {
				account := &Account{
					SupplierID: 1001,
					Provider:   ProviderOpenAI,
					AccountType: AccountTypeAPIKey,
					Status:     AccountStatusSuspended,
					Version:   1,
				}
				store.Create(context.Background(), account)
				return account
			},
			wantErr: false,
		},
		{
			name:       "activate active account fails",
			supplierID: 1001,
			setup: func() *Account {
				account := &Account{
					SupplierID: 1001,
					Provider:   ProviderOpenAI,
					AccountType: AccountTypeAPIKey,
					Status:     AccountStatusActive,
					Version:   1,
				}
				store.Create(context.Background(), account)
				return account
			},
			wantErr: true,
			errMsg:  "can only activate pending or suspended accounts",
		},
		{
			name:       "activate non-existent account fails",
			supplierID: 9999,
			accountID:  9999,
			setup:      func() *Account { return nil },
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var accountID int64
			if tt.setup != nil {
				account := tt.setup()
				if account != nil {
					accountID = account.ID
				}
			} else {
				accountID = tt.accountID
			}

			result, err := svc.Activate(context.Background(), tt.supplierID, accountID)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, AccountStatusActive, result.Status)
				assert.Equal(t, 2, result.Version)
			}
		})
	}
}

func TestAccountService_Suspend(t *testing.T) {
	store := newMockAccountStore()
	auditStore := &mockAuditStore{}
	svc := NewAccountService(store, auditStore)

	tests := []struct {
		name       string
		setup      func() *Account
		supplierID int64
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "suspend active account success",
			supplierID: 1001,
			setup: func() *Account {
				account := &Account{
					SupplierID: 1001,
					Provider:   ProviderOpenAI,
					AccountType: AccountTypeAPIKey,
					Status:     AccountStatusActive,
					Version:   1,
				}
				store.Create(context.Background(), account)
				return account
			},
			wantErr: false,
		},
		{
			name:       "suspend pending account fails",
			supplierID: 1001,
			setup: func() *Account {
				account := &Account{
					SupplierID: 1001,
					Provider:   ProviderOpenAI,
					AccountType: AccountTypeAPIKey,
					Status:     AccountStatusPending,
					Version:   1,
				}
				store.Create(context.Background(), account)
				return account
			},
			wantErr: true,
			errMsg:  "can only suspend active accounts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := tt.setup()
			result, err := svc.Suspend(context.Background(), tt.supplierID, account.ID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, AccountStatusSuspended, result.Status)
			}
		})
	}
}

func TestAccountService_Delete(t *testing.T) {
	store := newMockAccountStore()
	auditStore := &mockAuditStore{}
	svc := NewAccountService(store, auditStore)

	tests := []struct {
		name       string
		setup      func() *Account
		supplierID int64
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "delete pending account success",
			supplierID: 1001,
			setup: func() *Account {
				account := &Account{
					SupplierID: 1001,
					Provider:   ProviderOpenAI,
					AccountType: AccountTypeAPIKey,
					Status:     AccountStatusPending,
					Version:   1,
				}
				store.Create(context.Background(), account)
				return account
			},
			wantErr: false,
		},
		{
			name:       "delete active account fails",
			supplierID: 1001,
			setup: func() *Account {
				account := &Account{
					SupplierID: 1001,
					Provider:   ProviderOpenAI,
					AccountType: AccountTypeAPIKey,
					Status:     AccountStatusActive,
					Version:   1,
				}
				store.Create(context.Background(), account)
				return account
			},
			wantErr: true,
			errMsg:  "cannot delete active accounts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := tt.setup()
			err := svc.Delete(context.Background(), tt.supplierID, account.ID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAccountService_GetByID(t *testing.T) {
	store := newMockAccountStore()
	auditStore := &mockAuditStore{}
	svc := NewAccountService(store, auditStore)

	// Setup: create an account
	account := &Account{
		SupplierID:  1001,
		Provider:    ProviderOpenAI,
		AccountType: AccountTypeAPIKey,
		Status:     AccountStatusActive,
		Version:   1,
	}
	store.Create(context.Background(), account)

	tests := []struct {
		name       string
		supplierID int64
		accountID  int64
		wantErr    bool
	}{
		{
			name:       "get existing account",
			supplierID: 1001,
			accountID:  account.ID,
			wantErr:    false,
		},
		{
			name:       "get non-existent account",
			supplierID: 9999,
			accountID:  9999,
			wantErr:    true,
		},
		{
			name:       "get account wrong supplier",
			supplierID: 2002,
			accountID:  account.ID,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.GetByID(context.Background(), tt.supplierID, tt.accountID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, account.ID, result.ID)
			}
		})
	}
}

func TestAccountService_Verify(t *testing.T) {
	store := newMockAccountStore()
	auditStore := &mockAuditStore{}
	svc := NewAccountService(store, auditStore)

	result, err := svc.Verify(context.Background(), 1001, ProviderOpenAI, AccountTypeAPIKey, "sk-test-key")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "pass", result.VerifyStatus)
	assert.Equal(t, 10, result.RiskScore)
	assert.NotEmpty(t, result.CheckItems)
	assert.Equal(t, float64(1000), result.AvailableQuota)
}

func TestHashCredential(t *testing.T) {
	tests := []struct {
		name     string
		cred     string
		expected string
	}{
		{"short credential", "abc", "hash_abc"},
		{"long credential", "abcdefghijklmnop", "hash_abcdefgh"},
		{"exact 8 chars", "abcdefgh", "hash_abcdefgh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hashCredential(tt.cred)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMin(t *testing.T) {
	assert.Equal(t, 1, min(1, 2))   // 1 < 2, returns 1
	assert.Equal(t, 1, min(2, 1))   // 1 < 2, returns 1
	assert.Equal(t, 0, min(0, 5))    // 0 < 5, returns 0
	assert.Equal(t, -1, min(-1, 1))  // -1 < 1, returns -1
	assert.Equal(t, 5, min(5, 5))    // equal, returns 5
}

// TestAccountConstants 测试账号常量
func TestAccountConstants(t *testing.T) {
	// AccountStatus
	assert.Equal(t, AccountStatus("pending"), AccountStatusPending)
	assert.Equal(t, AccountStatus("active"), AccountStatusActive)
	assert.Equal(t, AccountStatus("suspended"), AccountStatusSuspended)
	assert.Equal(t, AccountStatus("disabled"), AccountStatusDisabled)

	// AccountType
	assert.Equal(t, AccountType("api_key"), AccountTypeAPIKey)
	assert.Equal(t, AccountType("oauth"), AccountTypeOAuth)

	// Provider
	assert.Equal(t, Provider("openai"), ProviderOpenAI)
	assert.Equal(t, Provider("anthropic"), ProviderAnthropic)
	assert.Equal(t, Provider("gemini"), ProviderGemini)
	assert.Equal(t, Provider("baidu"), ProviderBaidu)
	assert.Equal(t, Provider("xfyun"), ProviderXfyun)
	assert.Equal(t, Provider("tencent"), ProviderTencent)
}

// mockFailingAuditStore Mock审计存储（总是失败）
type mockFailingAuditStore struct{}

func (m *mockFailingAuditStore) Emit(ctx context.Context, event audit.Event) error {
	return errors.New("audit emit failed")
}

func (m *mockFailingAuditStore) Query(ctx context.Context, filter audit.EventFilter) ([]audit.Event, error) {
	return nil, nil
}

func (m *mockFailingAuditStore) QueryWithTotal(ctx context.Context, filter audit.EventFilter) ([]audit.Event, int64, error) {
	return nil, 0, nil
}

func (m *mockFailingAuditStore) GetByID(ctx context.Context, eventID string) (audit.Event, error) {
	return audit.Event{}, errors.New("not found")
}

// TestAccountService_Create_WithFailingAudit 测试创建账号时审计失败（不应影响主流程）
func TestAccountService_Create_WithFailingAudit(t *testing.T) {
	store := newMockAccountStore()
	failingAuditStore := &mockFailingAuditStore{}
	svc := NewAccountService(store, failingAuditStore)

	// 即使审计失败，账号创建也应该成功
	req := &CreateAccountRequest{
		SupplierID:  1001,
		Provider:    ProviderOpenAI,
		AccountType: AccountTypeAPIKey,
		Credential:  "sk-test-key",
		Alias:       "test-account",
		RiskAck:     true,
	}

	account, err := svc.Create(context.Background(), req)
	assert.NoError(t, err) // 主流程应该成功
	assert.NotNil(t, account)
	assert.Equal(t, AccountStatusPending, account.Status)
}

// TestAccountService_Activate_WithFailingAudit 测试激活账号时审计失败
func TestAccountService_Activate_WithFailingAudit(t *testing.T) {
	store := newMockAccountStore()
	failingAuditStore := &mockFailingAuditStore{}
	svc := NewAccountService(store, failingAuditStore)

	// 创建pending账号
	account := &Account{
		SupplierID:  1001,
		Provider:    ProviderOpenAI,
		AccountType: AccountTypeAPIKey,
		Status:      AccountStatusPending,
		Version:     1,
	}
	store.Create(context.Background(), account)

	// 激活（审计会失败但主流程应成功）
	result, err := svc.Activate(context.Background(), 1001, account.ID)
	assert.NoError(t, err)
	assert.Equal(t, AccountStatusActive, result.Status)
}

// TestVerifyResultStruct 测试验证结果结构体
func TestVerifyResultStruct(t *testing.T) {
	result := &VerifyResult{
		VerifyStatus:   "pass",
		AvailableQuota: 1000.0,
		RiskScore:     10,
		CheckItems: []CheckItem{
			{Item: "credential_format", Result: "pass", Message: "ok"},
		},
	}

	assert.Equal(t, "pass", result.VerifyStatus)
	assert.Equal(t, float64(1000), result.AvailableQuota)
	assert.Equal(t, 10, result.RiskScore)
	assert.Len(t, result.CheckItems, 1)
	assert.Equal(t, "credential_format", result.CheckItems[0].Item)
}

// TestAccountService_Create_DuplicateAlias 测试创建账号（已有别名）
func TestAccountService_Create_WithAlias(t *testing.T) {
	store := newMockAccountStore()
	auditStore := &mockAuditStore{}
	svc := NewAccountService(store, auditStore)

	req := &CreateAccountRequest{
		SupplierID:  1001,
		Provider:    ProviderOpenAI,
		AccountType: AccountTypeAPIKey,
		Credential:  "sk-test-key-12345",
		Alias:       "my-openai-account",
		RiskAck:     true,
	}

	account, err := svc.Create(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, account)
	assert.Equal(t, "my-openai-account", account.Alias)
}
