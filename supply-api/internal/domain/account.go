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
	ID             int64         `json:"account_id"`
	SupplierID     int64         `json:"supplier_id"`
	Provider       Provider      `json:"provider"`
	AccountType    AccountType   `json:"account_type"`
	CredentialHash string        `json:"-"`                // 不暴露
	KeyID          string        `json:"key_id,omitempty"` // 不暴露
	Alias          string        `json:"account_alias,omitempty"`
	Status         AccountStatus `json:"status"`
	RiskLevel      string        `json:"risk_level"`
	TotalQuota     float64       `json:"total_quota,omitempty"`
	AvailableQuota float64       `json:"available_quota,omitempty"`
	FrozenQuota    float64       `json:"frozen_quota,omitempty"`
	IsVerified     bool          `json:"is_verified"`
	VerifiedAt     *time.Time    `json:"verified_at,omitempty"`
	LastCheckAt    *time.Time    `json:"last_check_at,omitempty"`
	TosCompliant   bool          `json:"tos_compliant"`
	TosCheckResult string        `json:"tos_check_result,omitempty"`
	TotalRequests  int64         `json:"total_requests"`
	TotalTokens    int64         `json:"total_tokens"`
	TotalCost      float64       `json:"total_cost"`
	SuccessRate    float64       `json:"success_rate"`
	RiskScore      int           `json:"risk_score"`
	RiskReason     string        `json:"risk_reason,omitempty"`
	IsFrozen       bool          `json:"is_frozen"`
	FrozenReason   string        `json:"frozen_reason,omitempty"`

	// 加密元数据字段 (XR-001)
	CredentialCipherAlgo  string     `json:"credential_cipher_algo,omitempty"`
	CredentialKMSKeyAlias string     `json:"credential_kms_key_alias,omitempty"`
	CredentialKeyVersion  int        `json:"credential_key_version,omitempty"`
	CredentialFingerprint string     `json:"credential_fingerprint,omitempty"`
	LastRotationAt        *time.Time `json:"last_rotation_at,omitempty"`

	// 单位与币种 (XR-001)
	QuotaUnit    string `json:"quota_unit"`
	CurrencyCode string `json:"currency_code"`

	// 审计字段 (XR-001)
	Version      int         `json:"version"`
	CreatedIP    *netip.Addr `json:"created_ip,omitempty"`
	UpdatedIP    *netip.Addr `json:"updated_ip,omitempty"`
	AuditTraceID string      `json:"audit_trace_id,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// 验证结果
type VerifyResult struct {
	VerifyStatus   string      `json:"verify_status"` // pass, review_required, reject
	AvailableQuota float64     `json:"available_quota,omitempty"`
	RiskScore      int         `json:"risk_score"`
	CheckItems     []CheckItem `json:"check_items,omitempty"`
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
	SupplierID  int64
	Provider    Provider
	AccountType AccountType
	Credential  string
	Alias       string
	RiskAck     bool
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
	store      AccountStore
	auditStore audit.AuditStore
}

func NewAccountService(store AccountStore, auditStore audit.AuditStore) AccountService {
	return &accountService{store: store, auditStore: auditStore}
}

// emitAudit 安全记录审计日志（失败只记录错误，不影响主流程）
func (s *accountService) emitAudit(ctx context.Context, event audit.Event) {
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
	s.emitAudit(ctx, audit.Event{
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
		return nil, ErrAccountCannotActivateState
	}

	account.Status = AccountStatusActive
	account.UpdatedAt = time.Now()
	account.Version++

	if err := s.store.Update(ctx, account); err != nil {
		return nil, err
	}

	s.emitAudit(ctx, audit.Event{
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
		return nil, ErrAccountCannotSuspendState
	}

	account.Status = AccountStatusSuspended
	account.UpdatedAt = time.Now()
	account.Version++

	if err := s.store.Update(ctx, account); err != nil {
		return nil, err
	}

	s.emitAudit(ctx, audit.Event{
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
		return ErrAccountCannotDeleteActive
	}

	s.emitAudit(ctx, audit.Event{
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
