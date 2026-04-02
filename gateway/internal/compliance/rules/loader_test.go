package rules

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRuleLoader_ValidYaml 测试加载有效YAML
func TestRuleLoader_ValidYaml(t *testing.T) {
	// 创建临时有效YAML文件
	tmpfile, err := os.CreateTemp("", "valid_rule_*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	validYAML := `
rules:
  - id: "CRED-EXPOSE-RESPONSE"
    name: "响应体凭证泄露检测"
    description: "检测 API 响应中是否包含可复用的供应商凭证片段"
    severity: "P0"
    matchers:
      - type: "regex_match"
        pattern: "(sk-|ak-|api_key|secret|token).*[a-zA-Z0-9]{20,}"
        target: "response_body"
        scope: "all"
    action:
      primary: "block"
      secondary: "alert"
    audit:
      event_name: "CRED-EXPOSE-RESPONSE"
      event_category: "CRED"
      event_sub_category: "EXPOSE"
`
	_, err = tmpfile.WriteString(validYAML)
	require.NoError(t, err)
	tmpfile.Close()

	// 测试加载
	loader := NewRuleLoader()
	rules, err := loader.LoadFromFile(tmpfile.Name())

	assert.NoError(t, err)
	assert.NotNil(t, rules)
	assert.Len(t, rules, 1)

	rule := rules[0]
	assert.Equal(t, "CRED-EXPOSE-RESPONSE", rule.ID)
	assert.Equal(t, "P0", rule.Severity)
	assert.Equal(t, "block", rule.Action.Primary)
}

// TestRuleLoader_InvalidYaml 测试加载无效YAML
func TestRuleLoader_InvalidYaml(t *testing.T) {
	// 创建临时无效YAML文件
	tmpfile, err := os.CreateTemp("", "invalid_rule_*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	invalidYAML := `
rules:
  - id: "CRED-EXPOSE-RESPONSE"
    name: "响应体凭证泄露检测"
    severity: "P0"
    # 缺少必需的matchers字段
    action:
      primary: "block"
`
	_, err = tmpfile.WriteString(invalidYAML)
	require.NoError(t, err)
	tmpfile.Close()

	// 测试加载
	loader := NewRuleLoader()
	rules, err := loader.LoadFromFile(tmpfile.Name())

	assert.Error(t, err)
	assert.Nil(t, rules)
}

// TestRuleLoader_MissingFields 测试缺少必需字段
func TestRuleLoader_MissingFields(t *testing.T) {
	// 创建缺少必需字段的YAML
	tmpfile, err := os.CreateTemp("", "missing_fields_*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	// 缺少 id 字段
	missingIDYAML := `
rules:
  - name: "响应体凭证泄露检测"
    severity: "P0"
    matchers:
      - type: "regex_match"
    action:
      primary: "block"
`
	_, err = tmpfile.WriteString(missingIDYAML)
	require.NoError(t, err)
	tmpfile.Close()

	loader := NewRuleLoader()
	rules, err := loader.LoadFromFile(tmpfile.Name())

	assert.Error(t, err)
	assert.Nil(t, rules)
	assert.Contains(t, err.Error(), "missing required field: id")
}

// TestRuleLoader_FileNotFound 测试文件不存在
func TestRuleLoader_FileNotFound(t *testing.T) {
	loader := NewRuleLoader()
	rules, err := loader.LoadFromFile("/nonexistent/path/rules.yaml")

	assert.Error(t, err)
	assert.Nil(t, rules)
}

// TestRuleLoader_ValidateRuleFormat 测试规则格式验证
func TestRuleLoader_ValidateRuleFormat(t *testing.T) {
	tests := []struct {
		name     string
		ruleID   string
		valid    bool
	}{
		{"标准格式", "CRED-EXPOSE-RESPONSE", true},
		{"带Detail格式", "CRED-EXPOSE-RESPONSE-DETAIL", true},
		{"双连字符", "CRED--EXPOSE-RESPONSE", false},
		{"小写字母", "cred-expose-response", false},
		{"单字符Category", "C-EXPOSE-RESPONSE", false},
	}

	loader := NewRuleLoader()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := loader.ValidateRuleID(tt.ruleID)
			assert.Equal(t, tt.valid, valid)
		})
	}
}

// TestRuleLoader_EmptyRules 测试空规则列表
func TestRuleLoader_EmptyRules(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "empty_rules_*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	emptyYAML := `
rules: []
`
	_, err = tmpfile.WriteString(emptyYAML)
	require.NoError(t, err)
	tmpfile.Close()

	loader := NewRuleLoader()
	rules, err := loader.LoadFromFile(tmpfile.Name())

	assert.NoError(t, err)
	assert.NotNil(t, rules)
	assert.Len(t, rules, 0)
}
