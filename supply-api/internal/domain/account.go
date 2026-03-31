package domain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"lijiaoqiao/supply-api/internal/audit"
)

// 账号状态
type AccountStatus string

const (
	AccountStatusPending   AccountStatus = "pending"
	AccountStatusActive    AccountStatus = "active"
	AccountStatusSuspended AccountStatus = "suspended"
	AccountStatusDisabled  AccountStatus = "disabled"
)

// 账号类型
type AccountType string

const (
	AccountTypeAPIKey AccountType = "api_key"
	AccountTypeOAuth  AccountType = "oauth"
)

// 供应商
type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderGemini    Provider = "gemini"
	ProviderBaidu     Provider = "baidu"
	ProviderXfyun     Provider = "xfyun"
	ProviderTencent   Provider = "tencent"
)

// 账号
type Account struct {
	ID              int64        `json:"account_id"`
	SupplierID      int64        `json:"supplier_id"`
	Provider        Provider     `json:"provider"`
	AccountType     AccountType  `json:"account_type"`
	CredentialHash  string       `json:"-"` // 不暴露
	Alias           string       `json:"account_alias,omitempty"`
	Status          AccountStatus `json:"status"`
	AvailableQuota  float64      `json:"available_quota,omitempty"`
	RiskScore       int          `json:"risk_score,omitempty"`
	Version         int          `json:"version"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

// 验证结果
type VerifyResult struct {
	VerifyStatus   string         `json:"verify_status"` // pass, review_required, reject
	AvailableQuota float64        `json:"available_quota,omitempty"`
	RiskScore      int            `json:"risk_score"`
	CheckItems     []CheckItem    `json:"check_items,omitempty"`
}

type CheckItem struct {
	Item    string `json:"item"`
	Result  string `json:"result"` // pass, fail, warn
	Message string `json:"message,omitempty"`
}

// 账号服务接口
type AccountService interface {
	Verify(ctx context.Context, supplierID int64, provider Provider, accountType AccountType, credential string) (*VerifyResult, error)
	Create(ctx context.Context, req *CreateAccountRequest) (*Account, error)
	Activate(ctx context.Context, supplierID, accountID int64) (*Account, error)
	Suspend(ctx context.Context, supplierID, accountID int64) (*Account, error)
	Delete(ctx context.Context, supplierID, accountID int64) error
	GetByID(ctx context.Context, supplierID, accountID int64) (*Account, error)
}

// 创建账号请求
type CreateAccountRequest struct {
	SupplierID      int64
	Provider        Provider
	AccountType     AccountType
	Credential      string
	Alias           string
	RiskAck         bool
}

// 账号仓储接口
type AccountStore interface {
	Create(ctx context.Context, account *Account) error
	GetByID(ctx context.Context, supplierID, id int64) (*Account, error)
	Update(ctx context.Context, account *Account) error
	List(ctx context.Context, supplierID int64) ([]*Account, error)
}

// 账号服务实现
type accountService struct {
	store     AccountStore
	auditStore audit.AuditStore
}

func NewAccountService(store AccountStore, auditStore audit.AuditStore) AccountService {
	return &accountService{store: store, auditStore: auditStore}
}

func (s *accountService) Verify(ctx context.Context, supplierID int64, provider Provider, accountType AccountType, credential string) (*VerifyResult, error) {
	// 开发阶段：模拟验证逻辑
	result := &VerifyResult{
		VerifyStatus: "pass",
		RiskScore:    10,
		CheckItems: []CheckItem{
			{Item: "credential_format", Result: "pass", Message: "凭证格式正确"},
			{Item: "provider_connectivity", Result: "pass", Message: "供应商连接正常"},
			{Item: "quota_availability", Result: "pass", Message: "额度可用"},
		},
	}

	// 模拟获取额度
	result.AvailableQuota = 1000.0

	return result, nil
}

func (s *accountService) Create(ctx context.Context, req *CreateAccountRequest) (*Account, error) {
	if !req.RiskAck {
		return nil, errors.New("risk_ack is required")
	}

	account := &Account{
		SupplierID:     req.SupplierID,
		Provider:       req.Provider,
		AccountType:    req.AccountType,
		CredentialHash: hashCredential(req.Credential),
		Alias:          req.Alias,
		Status:         AccountStatusPending,
		Version:        1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.store.Create(ctx, account); err != nil {
		return nil, err
	}

	// 记录审计日志
	s.auditStore.Emit(ctx, audit.Event{
		TenantID:   req.SupplierID,
		ObjectType: "supply_account",
		ObjectID:   account.ID,
		Action:     "create",
		ResultCode: "OK",
	})

	return account, nil
}

func (s *accountService) Activate(ctx context.Context, supplierID, accountID int64) (*Account, error) {
	account, err := s.store.GetByID(ctx, supplierID, accountID)
	if err != nil {
		return nil, err
	}

	if account.Status != AccountStatusPending && account.Status != AccountStatusSuspended {
		return nil, errors.New("SUP_ACC_4091: can only activate pending or suspended accounts")
	}

	account.Status = AccountStatusActive
	account.UpdatedAt = time.Now()
	account.Version++

	if err := s.store.Update(ctx, account); err != nil {
		return nil, err
	}

	s.auditStore.Emit(ctx, audit.Event{
		TenantID:   supplierID,
		ObjectType: "supply_account",
		ObjectID:   accountID,
		Action:     "activate",
		ResultCode: "OK",
	})

	return account, nil
}

func (s *accountService) Suspend(ctx context.Context, supplierID, accountID int64) (*Account, error) {
	account, err := s.store.GetByID(ctx, supplierID, accountID)
	if err != nil {
		return nil, err
	}

	if account.Status != AccountStatusActive {
		return nil, errors.New("SUP_ACC_4091: can only suspend active accounts")
	}

	account.Status = AccountStatusSuspended
	account.UpdatedAt = time.Now()
	account.Version++

	if err := s.store.Update(ctx, account); err != nil {
		return nil, err
	}

	s.auditStore.Emit(ctx, audit.Event{
		TenantID:   supplierID,
		ObjectType: "supply_account",
		ObjectID:   accountID,
		Action:     "suspend",
		ResultCode: "OK",
	})

	return account, nil
}

func (s *accountService) Delete(ctx context.Context, supplierID, accountID int64) error {
	account, err := s.store.GetByID(ctx, supplierID, accountID)
	if err != nil {
		return err
	}

	if account.Status == AccountStatusActive {
		return errors.New("SUP_ACC_4092: cannot delete active accounts")
	}

	s.auditStore.Emit(ctx, audit.Event{
		TenantID:   supplierID,
		ObjectType: "supply_account",
		ObjectID:   accountID,
		Action:     "delete",
		ResultCode: "OK",
	})

	return nil
}

func (s *accountService) GetByID(ctx context.Context, supplierID, accountID int64) (*Account, error) {
	return s.store.GetByID(ctx, supplierID, accountID)
}

func hashCredential(cred string) string {
	// 开发阶段简单实现
	return fmt.Sprintf("hash_%s", cred[:min(8, len(cred))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
