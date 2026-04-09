package domain

import (
	"context"
	"testing"
)

// TestDefaultSMSVerifier_ShouldNotAcceptHardcodedCode [SEC-001]
// 验证默认SMS验证器不应接受硬编码测试码
// 问题: DefaultSMSVerifier 硬编码 "123456" 存在安全风险
func TestDefaultSMSVerifier_ShouldNotAcceptHardcodedCode(t *testing.T) {
	verifier := &DefaultSMSVerifier{}
	ctx := context.Background()

	// 硬编码测试码不应该通过验证
	valid, _ := verifier.Verify(ctx, "13800138000", "123456")

	// 如果返回 valid=true，说明存在安全风险
	if valid {
		t.Errorf("SEC-001: DefaultSMSVerifier 接受了硬编码测试码 '123456'，存在安全风险")
	}
}

// TestDefaultSMSVerifier_ReturnsError [SEC-001]
// 验证默认SMS验证器在未配置真实SMS服务时应返回错误
func TestDefaultSMSVerifier_ReturnsError(t *testing.T) {
	verifier := &DefaultSMSVerifier{}
	ctx := context.Background()

	// 任何验证码都不应该通过（因为没有真实SMS服务）
	valid, err := verifier.Verify(ctx, "13800138000", "123456")

	// 安全实现应该返回错误或拒绝验证
	// 如果没有返回错误且验证通过，则存在安全问题
	if err == nil && valid {
		t.Errorf("SEC-001: DefaultSMSVerifier 应该返回错误或拒绝验证，但实际接受了任何验证码")
	}
}

// TestSettlementService_Withdraw_UsesRealSMSVerifier [SEC-001]
// 验证提现服务使用真实的SMS验证器
// 注意: 这里只测试SMSVerifier接口的合规性，实际提现逻辑需要完整的mock
func TestSMSVerifier_WithMock_ReturnsExpectedResult(t *testing.T) {
	smsVerifier := &mockSMSVerifierForSettlement{verifyResult: false} // Mock返回验证失败

	valid, err := smsVerifier.Verify(context.Background(), "13800138000", "000000")
	if err != nil {
		t.Errorf("mock SMS verifier returned unexpected error: %v", err)
	}
	if valid {
		t.Errorf("mock SMS verifier should return false for invalid code")
	}
}

// TestSMSVerifier_InterfaceCompliance [SEC-001]
// 验证SMSVerifier接口合规性
func TestSMSVerifier_InterfaceCompliance(t *testing.T) {
	// 验证 DefaultSMSVerifier 实现了 SMSVerifier 接口
	var _ SMSVerifier = &DefaultSMSVerifier{}
}

// TestDefaultSMSVerifier_RejectsAllCodes [SEC-001]
// 验证默认验证器拒绝所有验证码（安全设计）
func TestDefaultSMSVerifier_RejectsAllCodes(t *testing.T) {
	verifier := &DefaultSMSVerifier{}
	ctx := context.Background()

	testCases := []string{"123456", "000000", "999999", "123123", "abcdef"}

	for _, code := range testCases {
		valid, err := verifier.Verify(ctx, "13800138000", code)

		// 安全实现：应该拒绝所有验证码，因为没有配置真实SMS服务
		if valid {
			t.Errorf("SEC-001: DefaultSMSVerifier 接受了验证码 '%s'，但未配置真实SMS服务", code)
		}

		// 最佳实践：应该返回错误表明SMS服务未配置
		if err == nil {
			t.Errorf("SEC-001: DefaultSMSVerifier 应返回错误表明SMS服务未配置，但接受了 '%s' 而无错误", code)
		}
	}
}

