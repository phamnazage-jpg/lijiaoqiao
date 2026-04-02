package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAuthQueryKey 测试query key请求检测
func TestAuthQueryKey(t *testing.T) {
	loader := NewRuleLoader()
	engine := NewRuleEngine(loader)

	rule := Rule{
		ID:       "AUTH-QUERY-KEY",
		Name:     "Query Key请求检测",
		Severity: "P0",
		Matchers: []Matcher{
			{
				Type:    "regex_match",
				Pattern: "(key=|api_key=|token=|bearer=|authorization=)",
				Target:  "query_string",
				Scope:   "all",
			},
		},
		Action: Action{
			Primary:   "reject",
			Secondary: "alert",
		},
	}

	testCases := []struct {
		name        string
		input       string
		shouldMatch bool
	}{
		{
			name:        "包含key参数",
			input:       "?key=sk-1234567890abcdefghijklmnopqrstuvwxyz",
			shouldMatch: true,
		},
		{
			name:        "包含api_key参数",
			input:       "?api_key=sk-1234567890abcdefghijklmnopqrstuvwxyz",
			shouldMatch: true,
		},
		{
			name:        "包含token参数",
			input:       "?token=bearer_1234567890abcdefghijklmnop",
			shouldMatch: true,
		},
		{
			name:        "不包含认证参数",
			input:       "?query=hello&limit=10",
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

// TestAuthQueryInject 测试query key注入检测
func TestAuthQueryInject(t *testing.T) {
	loader := NewRuleLoader()
	engine := NewRuleEngine(loader)

	rule := Rule{
		ID:       "AUTH-QUERY-INJECT",
		Name:     "Query Key注入检测",
		Severity: "P0",
		Matchers: []Matcher{
			{
				Type:    "regex_match",
				Pattern: "(key=|api_key=|token=|bearer=|authorization=).*[a-zA-Z0-9]{20,}",
				Target:  "query_string",
				Scope:   "all",
			},
		},
		Action: Action{
			Primary:   "reject",
			Secondary: "alert",
		},
	}

	testCases := []struct {
		name        string
		input       string
		shouldMatch bool
	}{
		{
			name:        "包含注入的key",
			input:       "?key=sk-1234567890abcdefghijklmnopqrstuvwxyz",
			shouldMatch: true,
		},
		{
			name:        "包含空key值",
			input:       "?key=",
			shouldMatch: false,
		},
		{
			name:        "包含短key值",
			input:       "?key=short",
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

// TestAuthQueryAudit 测试query key审计检测
func TestAuthQueryAudit(t *testing.T) {
	loader := NewRuleLoader()
	engine := NewRuleEngine(loader)

	rule := Rule{
		ID:       "AUTH-QUERY-AUDIT",
		Name:     "Query Key审计检测",
		Severity: "P1",
		Matchers: []Matcher{
			{
				Type:    "regex_match",
				Pattern: "(query_key|qkey|query_token)",
				Target:  "internal_context",
				Scope:   "all",
			},
		},
		Action: Action{
			Primary:   "alert",
			Secondary: "log",
		},
	}

	testCases := []struct {
		name        string
		input       string
		shouldMatch bool
	}{
		{
			name:        "包含query_key标记",
			input:       "internal: query_key=abc123",
			shouldMatch: true,
		},
		{
			name:        "不包含query_key标记",
			input:       "internal: platform_token=xyz789",
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

// TestAuthQueryRuleIDFormat 测试规则ID格式
func TestAuthQueryRuleIDFormat(t *testing.T) {
	loader := NewRuleLoader()

	validIDs := []string{
		"AUTH-QUERY-KEY",
		"AUTH-QUERY-INJECT",
		"AUTH-QUERY-AUDIT",
	}

	for _, id := range validIDs {
		t.Run(id, func(t *testing.T) {
			assert.True(t, loader.ValidateRuleID(id), "Rule ID %s should be valid", id)
		})
	}
}
