package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCredIngressPlatform 测试平台凭证入站检测
func TestCredIngressPlatform(t *testing.T) {
	loader := NewRuleLoader()
	engine := NewRuleEngine(loader)

	rule := Rule{
		ID:       "CRED-INGRESS-PLATFORM",
		Name:     "平台凭证入站检测",
		Severity: "P0",
		Matchers: []Matcher{
			{
				Type:    "regex_match",
				Pattern: "Authorization:\\s*Bearer\\s*ptk_[A-Za-z0-9]{20,}",
				Target:  "request_header",
				Scope:   "all",
			},
		},
		Action: Action{
			Primary:   "block",
			Secondary: "alert",
		},
	}

	testCases := []struct {
		name        string
		input       string
		shouldMatch bool
	}{
		{
			name:        "包含有效平台凭证",
			input:       "Authorization: Bearer ptk_1234567890abcdefghijklmnopqrst",
			shouldMatch: true,
		},
		{
			name:        "不包含Authorization头",
			input:       "Content-Type: application/json",
			shouldMatch: false,
		},
		{
			name:        "包含无效凭证格式",
			input:       "Authorization: Bearer invalid",
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matchResult := engine.Match(rule, tc.input)
			assert.Equal(t, tc.shouldMatch, matchResult.Matched, "Test case: %s", tc.name)
		})
	}
}

// TestCredIngressSupplier 测试供应商凭证入站检测
func TestCredIngressSupplier(t *testing.T) {
	loader := NewRuleLoader()
	engine := NewRuleEngine(loader)

	rule := Rule{
		ID:       "CRED-INGRESS-SUPPLIER",
		Name:     "供应商凭证入站检测",
		Severity: "P0",
		Matchers: []Matcher{
			{
				Type:    "regex_match",
				Pattern: "(sk-|ak-|api_key).*[a-zA-Z0-9]{20,}",
				Target:  "request_header",
				Scope:   "all",
			},
		},
		Action: Action{
			Primary:   "block",
			Secondary: "alert",
		},
	}

	testCases := []struct {
		name        string
		input       string
		shouldMatch bool
	}{
		{
			name:        "请求头包含供应商凭证",
			input:       "X-API-Key: sk-1234567890abcdefghijklmnopqrstuvwxyz",
			shouldMatch: true,
		},
		{
			name:        "请求头不包含供应商凭证",
			input:       "X-Request-ID: abc123",
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matchResult := engine.Match(rule, tc.input)
			assert.Equal(t, tc.shouldMatch, matchResult.Matched, "Test case: %s", tc.name)
		})
	}
}

// TestCredIngressFormat 测试凭证格式验证
func TestCredIngressFormat(t *testing.T) {
	loader := NewRuleLoader()
	engine := NewRuleEngine(loader)

	rule := Rule{
		ID:       "CRED-INGRESS-FORMAT",
		Name:     "凭证格式验证",
		Severity: "P1",
		Matchers: []Matcher{
			{
				Type:    "regex_match",
				Pattern: "^ptk_[A-Za-z0-9]{32,}$",
				Target:  "credential_format",
				Scope:   "all",
			},
		},
		Action: Action{
			Primary:   "block",
			Secondary: "alert",
		},
	}

	testCases := []struct {
		name        string
		input       string
		shouldMatch bool
	}{
		{
			name:        "有效平台凭证格式",
			input:       "ptk_1234567890abcdefghijklmnopqrstuvwx",
			shouldMatch: true,
		},
		{
			name:        "无效格式-缺少ptk_前缀",
			input:       "1234567890abcdefghijklmnopqrstuvwx",
			shouldMatch: false,
		},
		{
			name:        "无效格式-太短",
			input:       "ptk_short",
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matchResult := engine.Match(rule, tc.input)
			assert.Equal(t, tc.shouldMatch, matchResult.Matched, "Test case: %s", tc.name)
		})
	}
}

// TestCredIngressExpired 测试凭证过期检测
func TestCredIngressExpired(t *testing.T) {
	loader := NewRuleLoader()
	engine := NewRuleEngine(loader)

	rule := Rule{
		ID:       "CRED-INGRESS-EXPIRED",
		Name:     "凭证过期检测",
		Severity: "P0",
		Matchers: []Matcher{
			{
				Type:    "regex_match",
				Pattern: "token_expired|token_invalid|TOKEN_EXPIRED|CredentialExpired",
				Target:  "error_response",
				Scope:   "all",
			},
		},
		Action: Action{
			Primary: "block",
		},
	}

	testCases := []struct {
		name        string
		input       string
		shouldMatch bool
	}{
		{
			name:        "包含token过期错误",
			input:       `{"error": "token_expired", "message": "Your token has expired"}`,
			shouldMatch: true,
		},
		{
			name:        "包含CredentialExpired错误",
			input:       `{"error": "CredentialExpired", "message": "Credential has been revoked"}`,
			shouldMatch: true,
		},
		{
			name:        "正常响应",
			input:       `{"status": "success", "data": "valid"}`,
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matchResult := engine.Match(rule, tc.input)
			assert.Equal(t, tc.shouldMatch, matchResult.Matched, "Test case: %s", tc.name)
		})
	}
}

// TestCredIngressRuleIDFormat 测试规则ID格式
func TestCredIngressRuleIDFormat(t *testing.T) {
	loader := NewRuleLoader()

	validIDs := []string{
		"CRED-INGRESS-PLATFORM",
		"CRED-INGRESS-SUPPLIER",
		"CRED-INGRESS-FORMAT",
		"CRED-INGRESS-EXPIRED",
	}

	for _, id := range validIDs {
		t.Run(id, func(t *testing.T) {
			assert.True(t, loader.ValidateRuleID(id), "Rule ID %s should be valid", id)
		})
	}
}
