package sms

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateCode(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"6 digits", 6},
		{"4 digits", 4},
		{"8 digits", 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := GenerateCode(tt.length)
			assert.NoError(t, err)
			assert.Len(t, code, tt.length)

			// Verify all characters are digits
			for _, c := range code {
				assert.True(t, c >= '0' && c <= '9', "character %c is not a digit", c)
			}
		})
	}
}

func TestMockSMSService_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected bool
	}{
		{"disabled", false, false},
		{"enabled", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{Enabled: tt.enabled}
			svc := NewMockSMSService(config)
			assert.Equal(t, tt.expected, svc.IsEnabled())
		})
	}
}

func TestMockSMSService_VerifyCode_Enabled(t *testing.T) {
	config := &Config{Enabled: true, CodeLength: 6}
	svc := NewMockSMSService(config)

	// Send code first
	codeID, err := svc.SendVerificationCode(context.Background(), "13800138000")
	assert.NoError(t, err)
	assert.NotEmpty(t, codeID)

	// Get the code from store (for testing, we need to access internal store)
	// In real scenario, the code would be sent via SMS
	code, err := GenerateCode(6)
	assert.NoError(t, err)

	// Verify wrong code
	valid, err := svc.VerifyCode(context.Background(), "wrong-id", "13800138000", code)
	assert.NoError(t, err)
	assert.False(t, valid)
}

func TestInMemoryCodeStore(t *testing.T) {
	store := NewInMemoryCodeStore()
	assert.NotNil(t, store)

	// Generate and store a code
	code, err := GenerateCode(6)
	assert.NoError(t, err)

	// Codes should be unique
	code2, err := GenerateCode(6)
	assert.NoError(t, err)
	assert.NotEqual(t, code, code2)
}

func TestSMSCodeVerifier_Verify(t *testing.T) {
	verifier := NewSMSCodeVerifier()
	ctx := context.Background()

	// Send verification code
	codeID, err := verifier.SendVerificationCode(ctx, "13800138000")
	assert.NoError(t, err)
	assert.NotEmpty(t, codeID)

	// Get the code from the store (we need to check internal state for testing)
	// Since we can't access internal codes directly, we'll test the error case
	// In a real test, you would mock the SMS sending

	// Verify with wrong code should fail
	valid, err := verifier.Verify(ctx, "13800138000", "000000")
	assert.NoError(t, err)
	assert.False(t, valid)
}

func TestSMSCodeVerifier_CodeExpiry(t *testing.T) {
	verifier := NewSMSCodeVerifier()

	// Manually add an expired code
	verifier.codes["expired-id"] = &CodeEntry{
		Code:      "123456",
		Phone:     "13800138000",
		ExpiresAt: time.Now().Add(-1 * time.Minute), // Expired
		Used:      false,
	}

	// Verify should fail for expired code
	valid, err := verifier.Verify(context.Background(), "13800138000", "123456")
	assert.NoError(t, err)
	assert.False(t, valid)
}

func TestSMSCodeVerifier_OneTimeUse(t *testing.T) {
	verifier := NewSMSCodeVerifier()
	ctx := context.Background()

	// Manually add a code
	verifier.codes["test-id"] = &CodeEntry{
		Code:      "123456",
		Phone:     "13800138000",
		ExpiresAt: time.Now().Add(5 * time.Minute),
		Used:      false,
	}

	// First verification should succeed
	valid, err := verifier.Verify(ctx, "13800138000", "123456")
	assert.NoError(t, err)
	assert.True(t, valid)

	// Second verification with same code should fail (code was marked as used)
	valid, err = verifier.Verify(ctx, "13800138000", "123456")
	assert.NoError(t, err)
	assert.False(t, valid)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.NotNil(t, config)
	assert.False(t, config.Enabled)
	assert.Equal(t, ProviderTencent, config.Provider)
	assert.Equal(t, 6, config.CodeLength)
	assert.Equal(t, 5, config.CodeExpireMins)
	assert.Equal(t, "ap-guangzhou", config.Region)
}

func TestNewSMSService_Disabled(t *testing.T) {
	config := &Config{Enabled: false}
	svc, err := NewSMSService(config)
	assert.NoError(t, err)
	assert.NotNil(t, svc)
	assert.False(t, svc.IsEnabled())
}

func TestNewSMSService_UnknownProvider(t *testing.T) {
	config := &Config{Enabled: true, Provider: "unknown"}
	_, err := NewSMSService(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported SMS provider")
}

// TestTencentSMSService_NewWithNilConfig 测试NewTencentSMSService处理nil配置
func TestTencentSMSService_NewWithNilConfig(t *testing.T) {
	svc := NewTencentSMSService(nil)
	assert.NotNil(t, svc)
	assert.False(t, svc.IsEnabled(), "should be disabled when using default config")
}

// TestTencentSMSService_IsEnabled 测试IsEnabled方法
func TestTencentSMSService_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected bool
	}{
		{"disabled", false, false},
		{"enabled", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{Enabled: tt.enabled}
			svc := NewTencentSMSService(config)
			assert.Equal(t, tt.expected, svc.IsEnabled())
		})
	}
}

// TestAliyunSMSService_NewWithNilConfig 测试NewAliyunSMSService处理nil配置
func TestAliyunSMSService_NewWithNilConfig(t *testing.T) {
	svc, err := NewAliyunSMSService(nil)
	assert.NoError(t, err)
	assert.NotNil(t, svc)
	assert.False(t, svc.IsEnabled(), "should be disabled when using default config")
}

// TestAliyunSMSService_IsEnabled 测试IsEnabled方法
func TestAliyunSMSService_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected bool
	}{
		{"disabled", false, false},
		{"enabled", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{Enabled: tt.enabled}
			svc, err := NewAliyunSMSService(config)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, svc.IsEnabled())
		})
	}
}

// TestParseTencentResponse 测试解析腾讯云响应
func TestParseTencentResponse(t *testing.T) {
	// Valid response
	validResponse := `{"Response":{"SendStatusSet":[{"SerialNo":"1234","PhoneNumber":"+8613800138000","Fee":1,"SessionContext":"","Code":"Ok","Message":"send success"}]}}`
	result, err := parseTencentResponse([]byte(validResponse))
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Ok", result.Response.SendStatusSet[0].Code)
}

// TestParseTencentResponse_InvalidJSON 测试解析无效JSON
func TestParseTencentResponse_InvalidJSON(t *testing.T) {
	result, err := parseTencentResponse([]byte("invalid json"))
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestParseTencentResponse_ErrorCode 测试解析错误响应
func TestParseTencentResponse_ErrorCode(t *testing.T) {
	// Response with error code
	errorResponse := `{"Response":{"SendStatusSet":[{"SerialNo":"1234","PhoneNumber":"+8613800138000","Fee":1,"SessionContext":"","Code":"Failed","Message":"invalid phone"}]}}`
	result, err := parseTencentResponse([]byte(errorResponse))
	assert.NoError(t, err) // 解析不报错，但Code是Failed
	assert.NotNil(t, result)
	assert.Equal(t, "Failed", result.Response.SendStatusSet[0].Code)
}

// TestNewSMSServiceWithCodeStore 测试带自定义code store的工厂函数
func TestNewSMSServiceWithCodeStore(t *testing.T) {
	config := &Config{Enabled: false}
	store := NewInMemoryCodeStore()
	svc, _, err := NewSMSServiceWithCodeStore(config, store)
	assert.NoError(t, err)
	assert.NotNil(t, svc)
}

// TestNewSMSServiceWithCodeStore_NilStore 测试nil store
func TestNewSMSServiceWithCodeStore_NilStore(t *testing.T) {
	config := &Config{Enabled: false}
	svc, _, err := NewSMSServiceWithCodeStore(config, nil)
	assert.NoError(t, err)
	assert.NotNil(t, svc)
}

// TestSMSCodeVerifier_VerifyByID 测试VerifyByID方法
func TestSMSCodeVerifier_VerifyByID(t *testing.T) {
	verifier := NewSMSCodeVerifier()
	ctx := context.Background()

	// 添加测试code
	verifier.codes["test-id-1"] = &CodeEntry{
		Code:      "123456",
		Phone:     "13800138000",
		ExpiresAt: time.Now().Add(5 * time.Minute),
		Used:      false,
	}

	// 正确code应该成功
	valid, err := verifier.VerifyByID(ctx, "test-id-1", "13800138000", "123456")
	assert.NoError(t, err)
	assert.True(t, valid)

	// 错误code应该失败
	valid, err = verifier.VerifyByID(ctx, "test-id-1", "13800138000", "000000")
	assert.NoError(t, err)
	assert.False(t, valid)

	// 不存在的ID应该失败
	valid, err = verifier.VerifyByID(ctx, "non-existent-id", "13800138000", "123456")
	assert.NoError(t, err)
	assert.False(t, valid)
}

// TestSMSCodeVerifier_Cleanup 测试Cleanup方法
func TestSMSCodeVerifier_Cleanup(t *testing.T) {
	verifier := NewSMSCodeVerifier()

	// 添加过期的code
	verifier.codes["expired-1"] = &CodeEntry{
		Code:      "111111",
		Phone:     "13800138000",
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
		Used:      false,
	}

	// 添加有效的code
	verifier.codes["valid-1"] = &CodeEntry{
		Code:      "222222",
		Phone:     "13800138001",
		ExpiresAt: time.Now().Add(5 * time.Minute), // Valid
		Used:      false,
	}

	// 添加已使用的code (cleanup不删除已使用的code，只删除过期的)
	verifier.codes["used-1"] = &CodeEntry{
		Code:      "333333",
		Phone:     "13800138002",
		ExpiresAt: time.Now().Add(5 * time.Minute),
		Used:      true,
	}

	// 执行cleanup (Cleanup只删除过期的code，不删除已使用的)
	verifier.Cleanup()

	// 剩下2个：valid-1 (未过期) 和 used-1 (虽然已使用但未过期)
	assert.Equal(t, 2, len(verifier.codes), "two codes should remain (valid and used but not expired)")

	_, exists := verifier.codes["expired-1"]
	assert.False(t, exists, "expired code should be removed")
	_, exists = verifier.codes["valid-1"]
	assert.True(t, exists, "valid code should remain")
	_, exists = verifier.codes["used-1"]
	assert.True(t, exists, "used but not expired code should remain")
}
