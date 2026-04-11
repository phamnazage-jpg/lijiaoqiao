package sms

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMockSMSService_SendWhenDisabled_ShouldReturnError
// TDD: 当SMS服务禁用时，SendVerificationCode应返回错误而非成功
func TestMockSMSService_SendWhenDisabled_ShouldReturnError(t *testing.T) {
	config := &Config{Enabled: false}
	svc := NewMockSMSService(config)

	codeID, err := svc.SendVerificationCode(context.Background(), "13800138000")

	// 安全要求：禁用时应返回错误
	assert.Error(t, err)
	assert.Equal(t, ErrSMSServiceDisabled, err)
	assert.Empty(t, codeID)
}

// TestMockSMSService_VerifyWhenDisabled_ShouldReturnError
// TDD: 当SMS服务禁用时，VerifyCode应返回错误而非验证
func TestMockSMSService_VerifyWhenDisabled_ShouldReturnError(t *testing.T) {
	config := &Config{Enabled: false}
	svc := NewMockSMSService(config)

	// 不应接受任何验证码
	valid, err := svc.VerifyCode(context.Background(), "any-id", "13800138000", "123456")

	// 安全要求：禁用时应返回错误
	assert.Error(t, err)
	assert.Equal(t, ErrSMSServiceDisabled, err)
	assert.False(t, valid)
}

// TestTencentSMSService_SendWhenDisabled_ShouldReturnError
// TDD: 腾讯云SMS服务禁用时应返回错误
func TestTencentSMSService_SendWhenDisabled_ShouldReturnError(t *testing.T) {
	config := &Config{Enabled: false}
	svc := NewTencentSMSService(config)

	codeID, err := svc.SendVerificationCode(context.Background(), "13800138000")

	assert.Error(t, err)
	assert.Equal(t, ErrSMSServiceDisabled, err)
	assert.Empty(t, codeID)
}

// TestAliyunSMSService_SendWhenDisabled_ShouldReturnError
// TDD: 阿里云SMS服务禁用时应返回错误
func TestAliyunSMSService_SendWhenDisabled_ShouldReturnError(t *testing.T) {
	config := &Config{Enabled: false}
	svc, err := NewAliyunSMSService(config)
	assert.NoError(t, err)

	codeID, err := svc.SendVerificationCode(context.Background(), "13800138000")

	assert.Error(t, err)
	assert.Equal(t, ErrSMSServiceDisabled, err)
	assert.Empty(t, codeID)
}

// TestDisabledServiceAcceptsNoCodes
// TDD: 禁用服务时不应接受任何验证码
func TestDisabledServiceAcceptsNoCodes(t *testing.T) {
	config := &Config{Enabled: false}
	svc := NewMockSMSService(config)
	ctx := context.Background()

	testCases := []string{"123456", "000000", "999999", "abcdef"}

	for _, code := range testCases {
		valid, err := svc.VerifyCode(ctx, "test-id", "13800138000", code)
		assert.Error(t, err, "code %s should be rejected when service is disabled", code)
		assert.False(t, valid)
	}
}
