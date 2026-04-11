package assert

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"lijiaoqiao/supply-api/internal/domain"
)

// Account 相关的自定义断言

// AssertAccountEqual 断言两个账号相等（忽略指针差异）
func AssertAccountEqual(t *testing.T, expected, actual *domain.Account, msgAndArgs ...interface{}) {
	assert.Equal(t, expected, actual, msgAndArgs...)
}

// AssertAccountStatus 断言账号状态
func AssertAccountStatus(t *testing.T, account *domain.Account, expectedStatus domain.AccountStatus, msgAndArgs ...interface{}) {
	assert.Equal(t, expectedStatus, account.Status, msgAndArgs...)
}

// AssertAccountFields 断言账号字段
func AssertAccountFields(t *testing.T, account *domain.Account, supplierID int64, provider domain.Provider, accountType domain.AccountType, msgAndArgs ...interface{}) {
	assert.Equal(t, supplierID, account.SupplierID, msgAndArgs...)
	assert.Equal(t, provider, account.Provider, msgAndArgs...)
	assert.Equal(t, accountType, account.AccountType, msgAndArgs...)
}

// Package 相关的自定义断言

// AssertPackageEqual 断言两个套餐相等
func AssertPackageEqual(t *testing.T, expected, actual *domain.Package, msgAndArgs ...interface{}) {
	assert.Equal(t, expected, actual, msgAndArgs...)
}

// AssertPackageStatus 断言套餐状态
func AssertPackageStatus(t *testing.T, pkg *domain.Package, expectedStatus domain.PackageStatus, msgAndArgs ...interface{}) {
	assert.Equal(t, expectedStatus, pkg.Status, msgAndArgs...)
}

// AssertPackageQuota 断言套餐配额
func AssertPackageQuota(t *testing.T, pkg *domain.Package, total, available, sold float64, msgAndArgs ...interface{}) {
	assert.InDelta(t, total, pkg.TotalQuota, 0.001, msgAndArgs...)
	assert.InDelta(t, available, pkg.AvailableQuota, 0.001, msgAndArgs...)
	assert.InDelta(t, sold, pkg.SoldQuota, 0.001, msgAndArgs...)
}

// Settlement 相关的自定义断言

// AssertSettlementEqual 断言两个结算单相等
func AssertSettlementEqual(t *testing.T, expected, actual *domain.Settlement, msgAndArgs ...interface{}) {
	assert.Equal(t, expected, actual, msgAndArgs...)
}

// AssertSettlementStatus 断言结算单状态
func AssertSettlementStatus(t *testing.T, s *domain.Settlement, expectedStatus domain.SettlementStatus, msgAndArgs ...interface{}) {
	assert.Equal(t, expectedStatus, s.Status, msgAndArgs...)
}

// AssertSettlementAmount 断言结算单金额
func AssertSettlementAmount(t *testing.T, s *domain.Settlement, total, fee, net float64, msgAndArgs ...interface{}) {
	assert.InDelta(t, total, s.TotalAmount, 0.001, msgAndArgs...)
	assert.InDelta(t, fee, s.FeeAmount, 0.001, msgAndArgs...)
	assert.InDelta(t, net, s.NetAmount, 0.001, msgAndArgs...)
}

// EarningRecord 相关的自定义断言

// AssertEarningRecordEqual 断言两条收益记录相等
func AssertEarningRecordEqual(t *testing.T, expected, actual *domain.EarningRecord, msgAndArgs ...interface{}) {
	assert.Equal(t, expected, actual, msgAndArgs...)
}

// Error 相关的自定义断言

// AssertErrorCode 断言错误码包含特定字符串
func AssertErrorCode(t *testing.T, err error, code string, msgAndArgs ...interface{}) {
	if err == nil {
		assert.Fail(t, "Expected error but got nil", msgAndArgs...)
		return
	}
	assert.Contains(t, err.Error(), code, msgAndArgs...)
}

// AssertErrorMessage 断言错误信息
func AssertErrorMessage(t *testing.T, err error, msg string, msgAndArgs ...interface{}) {
	if err == nil {
		assert.Fail(t, "Expected error but got nil", msgAndArgs...)
		return
	}
	assert.Contains(t, err.Error(), msg, msgAndArgs...)
}
