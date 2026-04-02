package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCredDirectSupplier 测试直连供应商检测
func TestCredDirectSupplier(t *testing.T) {
	loader := NewRuleLoader()
	engine := NewRuleEngine(loader)

	rule := Rule{
		ID:       "CRED-DIRECT-SUPPLIER",
		Name:     "直连供应商检测",
		Severity: "P0",
		Matchers: []Matcher{
			{
				Type:    "regex_match",
				Pattern: "(api\\.openai\\.com|api\\.anthropic\\.com|api\\.minimax\\.chat)",
				Target:  "request_host",
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
			name:        "直连OpenAI API",
			input:       "api.openai.com",
			shouldMatch: true,
		},
		{
			name:        "直连Anthropic API",
			input:       "api.anthropic.com",
			shouldMatch: true,
		},
		{
			name:        "通过平台代理",
			input:       "gateway.platform.com",
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

// TestCredDirectAPI 测试直连API端点检测
func TestCredDirectAPI(t *testing.T) {
	loader := NewRuleLoader()
	engine := NewRuleEngine(loader)

	rule := Rule{
		ID:       "CRED-DIRECT-API",
		Name:     "直连API端点检测",
		Severity: "P0",
		Matchers: []Matcher{
			{
				Type:    "regex_match",
				Pattern: "^/v1/(chat/completions|completions|embeddings)$",
				Target:  "request_path",
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
			name:        "直接访问chat completions",
			input:       "/v1/chat/completions",
			shouldMatch: true,
		},
		{
			name:        "直接访问completions",
			input:       "/v1/completions",
			shouldMatch: true,
		},
		{
			name:        "平台代理路径",
			input:       "/api/platform/v1/chat/completions",
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

// TestCredDirectUnauth 测试未授权直连检测
func TestCredDirectUnauth(t *testing.T) {
	loader := NewRuleLoader()
	engine := NewRuleEngine(loader)

	rule := Rule{
		ID:       "CRED-DIRECT-UNAUTH",
		Name:     "未授权直连检测",
		Severity: "P0",
		Matchers: []Matcher{
			{
				Type:    "regex_match",
				Pattern: "(direct_ip| bypass_proxy| no_platform_auth)",
				Target:  "connection_metadata",
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
			name:        "检测到直连标记",
			input:       "direct_ip: 203.0.113.50, bypass_proxy: true",
			shouldMatch: true,
		},
		{
			name:        "正常代理请求",
			input:       "via: platform_proxy, auth: platform_token",
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

// TestCredDirectRuleIDFormat 测试规则ID格式
func TestCredDirectRuleIDFormat(t *testing.T) {
	loader := NewRuleLoader()

	validIDs := []string{
		"CRED-DIRECT-SUPPLIER",
		"CRED-DIRECT-API",
		"CRED-DIRECT-UNAUTH",
	}

	for _, id := range validIDs {
		t.Run(id, func(t *testing.T) {
			assert.True(t, loader.ValidateRuleID(id), "Rule ID %s should be valid", id)
		})
	}
}
