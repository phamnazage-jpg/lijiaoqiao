package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCredExposeResponse 测试响应体凭证泄露检测
func TestCredExposeResponse(t *testing.T) {
	loader := NewRuleLoader()
	engine := NewRuleEngine(loader)

	// 创建CRED-EXPOSE-RESPONSE规则
	rule := Rule{
		ID:       "CRED-EXPOSE-RESPONSE",
		Name:     "响应体凭证泄露检测",
		Severity: "P0",
		Matchers: []Matcher{
			{
				Type:    "regex_match",
				Pattern: "(sk-|ak-|api_key|secret|token).*[a-zA-Z0-9]{20,}",
				Target:  "response_body",
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
			name:        "包含sk-凭证",
			input:       `{"api_key": "sk-1234567890abcdefghijklmnopqrstuvwxyz"}`,
			shouldMatch: true,
		},
		{
			name:        "包含ak-凭证",
			input:       `{"access_key": "ak-1234567890abcdefghijklmnopqrstuvwxyz"}`,
			shouldMatch: true,
		},
		{
			name:        "包含api_key",
			input:       `{"result": "api_key_1234567890abcdefghijklmnopqr"}`,
			shouldMatch: true,
		},
		{
			name:        "不包含凭证的正常响应",
			input:       `{"status": "success", "data": "hello world"}`,
			shouldMatch: false,
		},
		{
			name:        "短token不匹配",
			input:       `{"token": "sk-short"}`,
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

// TestCredExposeLog 测试日志凭证泄露检测
func TestCredExposeLog(t *testing.T) {
	loader := NewRuleLoader()
	engine := NewRuleEngine(loader)

	rule := Rule{
		ID:       "CRED-EXPOSE-LOG",
		Name:     "日志凭证泄露检测",
		Severity: "P0",
		Matchers: []Matcher{
			{
				Type:    "regex_match",
				Pattern: "(sk-|ak-|api_key|secret|token).*[a-zA-Z0-9]{20,}",
				Target:  "log",
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
			name:        "日志包含凭证",
			input:       "[INFO] Using API key: sk-1234567890abcdefghijklmnopqrstuvwxyz",
			shouldMatch: true,
		},
		{
			name:        "日志不包含凭证",
			input:       "[INFO] Processing request from 192.168.1.1",
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

// TestCredExposeExport 测试导出凭证泄露检测
func TestCredExposeExport(t *testing.T) {
	loader := NewRuleLoader()
	engine := NewRuleEngine(loader)

	rule := Rule{
		ID:       "CRED-EXPOSE-EXPORT",
		Name:     "导出凭证泄露检测",
		Severity: "P0",
		Matchers: []Matcher{
			{
				Type:    "regex_match",
				Pattern: "(sk-|ak-|api_key|secret|token).*[a-zA-Z0-9]{20,}",
				Target:  "export",
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
			name:        "导出CSV包含凭证",
			input:       "api_key,secret\nsk-1234567890abcdefghijklmnopqrstuvwxyz,mysupersecret",
			shouldMatch: true,
		},
		{
			name:        "导出CSV不包含凭证",
			input:       "id,name\n1,John Doe",
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

// TestCredExposeWebhook 测试Webhook凭证泄露检测
func TestCredExposeWebhook(t *testing.T) {
	loader := NewRuleLoader()
	engine := NewRuleEngine(loader)

	rule := Rule{
		ID:       "CRED-EXPOSE-WEBHOOK",
		Name:     "Webhook凭证泄露检测",
		Severity: "P0",
		Matchers: []Matcher{
			{
				Type:    "regex_match",
				Pattern: "(sk-|ak-|api_key|secret|token).*[a-zA-Z0-9]{20,}",
				Target:  "webhook",
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
			name:        "Webhook请求包含凭证",
			input:       `{"url": "https://example.com/callback", "token": "sk-1234567890abcdefghijklmnopqrstuvwxyz"}`,
			shouldMatch: true,
		},
		{
			name:        "Webhook请求不包含凭证",
			input:       `{"url": "https://example.com/callback", "status": "ok"}`,
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

// TestCredExposeRuleIDFormat 测试规则ID格式
func TestCredExposeRuleIDFormat(t *testing.T) {
	loader := NewRuleLoader()

	validIDs := []string{
		"CRED-EXPOSE-RESPONSE",
		"CRED-EXPOSE-LOG",
		"CRED-EXPOSE-EXPORT",
		"CRED-EXPOSE-WEBHOOK",
	}

	for _, id := range validIDs {
		t.Run(id, func(t *testing.T) {
			assert.True(t, loader.ValidateRuleID(id), "Rule ID %s should be valid", id)
		})
	}
}
