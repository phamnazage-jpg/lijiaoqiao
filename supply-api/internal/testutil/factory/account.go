package factory

import (
	"lijiaoqiao/supply-api/internal/domain"
)

// AccountFactory 账号测试数据工厂
type AccountFactory struct {
	supplierID  int64
	provider    domain.Provider
	accountType domain.AccountType
	credential  string
	alias       string
	riskAck     bool
}

// NewAccountFactory 创建默认账号工厂
func NewAccountFactory() *AccountFactory {
	return &AccountFactory{
		supplierID:  1001,
		provider:    domain.ProviderOpenAI,
		accountType: domain.AccountTypeAPIKey,
		credential:  "sk-test-key-" + randomString(8),
		alias:       "test-account",
		riskAck:     true,
	}
}

// WithSupplierID 设置供应商ID
func (f *AccountFactory) WithSupplierID(id int64) *AccountFactory {
	f.supplierID = id
	return f
}

// WithProvider 设置提供商
func (f *AccountFactory) WithProvider(p domain.Provider) *AccountFactory {
	f.provider = p
	return f
}

// WithAccountType 设置账号类型
func (f *AccountFactory) WithAccountType(t domain.AccountType) *AccountFactory {
	f.accountType = t
	return f
}

// WithCredential 设置凭证
func (f *AccountFactory) WithCredential(cred string) *AccountFactory {
	f.credential = cred
	return f
}

// WithAlias 设置别名
func (f *AccountFactory) WithAlias(alias string) *AccountFactory {
	f.alias = alias
	return f
}

// WithRiskAck 设置风险确认
func (f *AccountFactory) WithRiskAck(ack bool) *AccountFactory {
	f.riskAck = ack
	return f
}

// BuildRequest 构建创建账号请求
func (f *AccountFactory) BuildRequest() *domain.CreateAccountRequest {
	return &domain.CreateAccountRequest{
		SupplierID:  f.supplierID,
		Provider:    f.provider,
		AccountType: f.accountType,
		Credential:  f.credential,
		Alias:       f.alias,
		RiskAck:     f.riskAck,
	}
}

// Build 构建账号对象
func (f *AccountFactory) Build() *domain.Account {
	return &domain.Account{
		ID:             1,
		SupplierID:     f.supplierID,
		Provider:       f.provider,
		AccountType:    f.accountType,
		CredentialHash: f.credential,
		KeyID:          "key-" + randomString(8),
		Alias:          f.alias,
		Status:         domain.AccountStatusPending,
		RiskLevel:      "low",
		TosCompliant:   f.riskAck,
		Version:        1,
	}
}

// BuildActive 构建活跃状态的账号
func (f *AccountFactory) BuildActive() *domain.Account {
	account := f.Build()
	account.Status = domain.AccountStatusActive
	return account
}

// BuildSuspended 构建暂停状态的账号
func (f *AccountFactory) BuildSuspended() *domain.Account {
	account := f.Build()
	account.Status = domain.AccountStatusSuspended
	return account
}

// BuildInvalidCredential 构建无效凭证的工厂
func (f *AccountFactory) BuildInvalidCredential() *AccountFactory {
	return f.WithCredential("")
}

// BuildWithoutRiskAck 构建未确认风险的工厂
func (f *AccountFactory) BuildWithoutRiskAck() *AccountFactory {
	return f.WithRiskAck(false)
}

// randomString 生成随机字符串（简化版）
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[i%len(letters)]
	}
	return string(b)
}
