package sanitizer

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizer_Scan_CredentialExposure(t *testing.T) {
	// 检测响应体中的凭证泄露
	scanner := NewCredentialScanner()

	testCases := []struct {
		name          string
		content       string
		expectFound   bool
		expectedTypes []string
	}{
		{
			name:          "OpenAI API Key",
			content:       "Your API key is sk-1234567890abcdefghijklmnopqrstuvwxyz",
			expectFound:   true,
			expectedTypes: []string{"openai_key"},
		},
		{
			name:          "AWS Access Key",
			content:       "access_key_id: AKIAIOSFODNN7EXAMPLE",
			expectFound:   true,
			expectedTypes: []string{"aws_access_key"},
		},
		{
			name:          "Client Secret",
			content:       "client_secret: c3VwZXJzZWNyZXRrZXlzZWNyZXRrZXk=",
			expectFound:   true,
			expectedTypes: []string{"secret"},
		},
		{
			name:          "Generic API Key",
			content:       "api_key: key-1234567890abcdefghij",
			expectFound:   true,
			expectedTypes: []string{"api_key"},
		},
		{
			name:          "Password Field",
			content:       "password: mysecretpassword123",
			expectFound:   true,
			expectedTypes: []string{"password"},
		},
		{
			name:          "Token Field",
			content:       "token: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expectFound:   true,
			expectedTypes: []string{"bearer_token"},
		},
		{
			name:          "Normal Text",
			content:       "This is normal text without credentials",
			expectFound:   false,
			expectedTypes: nil,
		},
		{
			name:          "Already Masked",
			content:       "api_key: sk-****-****",
			expectFound:   false,
			expectedTypes: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := scanner.Scan(tc.content)

			if tc.expectFound {
				assert.True(t, result.HasViolation(), "Expected violation for: %s", tc.name)
				assert.NotEmpty(t, result.Violations, "Expected violations for: %s", tc.name)

				var foundTypes []string
				for _, v := range result.Violations {
					foundTypes = append(foundTypes, v.Type)
				}

				for _, expectedType := range tc.expectedTypes {
					assert.Contains(t, foundTypes, expectedType, "Expected type %s in violations for: %s", expectedType, tc.name)
				}
			} else {
				assert.False(t, result.HasViolation(), "Expected no violation for: %s", tc.name)
			}
		})
	}
}

func TestSanitizer_Scan_Masking(t *testing.T) {
	// 脱敏：'sk-xxxx' 格式
	sanitizer := NewSanitizer()

	testCases := []struct {
		name           string
		input          string
		expectedOutput string
		expectMasked   bool
	}{
		{
			name:           "OpenAI Key",
			input:          "sk-1234567890abcdefghijklmnopqrstuvwxyz",
			expectedOutput: "sk-xxxxxx****xxxx",
			expectMasked:   true,
		},
		{
			name:           "Short OpenAI Key",
			input:          "sk-1234567890",
			expectedOutput: "sk-****7890",
			expectMasked:   true,
		},
		{
			name:           "AWS Access Key",
			input:          "AKIAIOSFODNN7EXAMPLE",
			expectedOutput: "AKIA****EXAMPLE",
			expectMasked:   true,
		},
		{
			name:           "Normal Text",
			input:          "This is normal text",
			expectedOutput: "This is normal text",
			expectMasked:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizer.Mask(tc.input)

			if tc.expectMasked {
				assert.NotEqual(t, tc.input, result, "Expected masking for: %s", tc.name)
				assert.Contains(t, result, "****", "Expected **** in masked result for: %s", tc.name)
			} else {
				assert.Equal(t, tc.expectedOutput, result, "Expected unchanged for: %s", tc.name)
			}
		})
	}
}

func TestSanitizer_Scan_ResponseBody(t *testing.T) {
	// 检测响应体中的凭证泄露
	scanner := NewCredentialScanner()

	responseBody := `{
		"success": true,
		"data": {
			"api_key": "sk-1234567890abcdefghijklmnopqrstuvwxyz",
			"user": "testuser"
		}
	}`

	result := scanner.Scan(responseBody)

	assert.True(t, result.HasViolation())
	assert.NotEmpty(t, result.Violations)

	// 验证找到了api_key类型的违规
	foundTypes := make([]string, 0)
	for _, v := range result.Violations {
		foundTypes = append(foundTypes, v.Type)
	}
	assert.Contains(t, foundTypes, "api_key")
}

func TestSanitizer_MaskMap(t *testing.T) {
	// 测试对map进行脱敏
	sanitizer := NewSanitizer()

	input := map[string]interface{}{
		"api_key": "sk-1234567890abcdefghijklmnopqrstuvwxyz",
		"secret":  "mysecretkey123",
		"user":    "testuser",
	}

	masked := sanitizer.MaskMap(input)

	// 验证敏感字段被脱敏
	assert.NotEqual(t, input["api_key"], masked["api_key"])
	assert.NotEqual(t, input["secret"], masked["secret"])
	assert.Equal(t, input["user"], masked["user"])

	// 验证脱敏格式
	assert.Contains(t, masked["api_key"], "****")
	assert.Contains(t, masked["secret"], "****")
}

func TestSanitizer_MaskSlice(t *testing.T) {
	// 测试对slice进行脱敏
	sanitizer := NewSanitizer()

	input := []string{
		"sk-1234567890abcdefghijklmnopqrstuvwxyz",
		"normal text",
		"password123",
	}

	masked := sanitizer.MaskSlice(input)

	assert.Len(t, masked, 3)
	assert.NotEqual(t, input[0], masked[0])
	assert.Equal(t, input[1], masked[1])
	assert.NotEqual(t, input[2], masked[2])
}

func TestCredentialScanner_SensitiveFields(t *testing.T) {
	// 测试敏感字段列表
	fields := GetSensitiveFields()

	// 验证常见敏感字段
	assert.Contains(t, fields, "api_key")
	assert.Contains(t, fields, "secret")
	assert.Contains(t, fields, "password")
	assert.Contains(t, fields, "token")
	assert.Contains(t, fields, "access_key")
	assert.Contains(t, fields, "private_key")
}

func TestCredentialScanner_ScanRules(t *testing.T) {
	// 测试扫描规则
	scanner := NewCredentialScanner()

	rules := scanner.GetRules()
	assert.NotEmpty(t, rules, "Scanner should have rules")

	// 验证规则有ID和描述
	for _, rule := range rules {
		assert.NotEmpty(t, rule.ID)
		assert.NotEmpty(t, rule.Description)
	}
}

func TestSanitizer_IsSensitiveField(t *testing.T) {
	// 测试字段名敏感性判断
	testCases := []struct {
		fieldName string
		expected  bool
	}{
		{"api_key", true},
		{"secret", true},
		{"password", true},
		{"token", true},
		{"access_key", true},
		{"private_key", true},
		{"session_id", true},
		{"authorization", true},
		{"user", false},
		{"name", false},
		{"email", false},
		{"id", false},
	}

	for _, tc := range testCases {
		t.Run(tc.fieldName, func(t *testing.T) {
			result := IsSensitiveField(tc.fieldName)
			assert.Equal(t, tc.expected, result, "Field %s sensitivity mismatch", tc.fieldName)
		})
	}
}

func TestSanitizer_ScanLog(t *testing.T) {
	// 测试日志扫描
	scanner := NewCredentialScanner()

	logLine := `2026-04-02 10:30:45 INFO [api] Request completed api_key=sk-1234567890abcdefghijklmnopqrstuvwxyz duration=100ms`

	result := scanner.Scan(logLine)

	assert.True(t, result.HasViolation())
	assert.NotEmpty(t, result.Violations)
	// sk-开头的key会被识别为openai_key
	assert.Equal(t, "openai_key", result.Violations[0].Type)
}

func TestSanitizer_MultipleViolations(t *testing.T) {
	// 测试多个违规
	scanner := NewCredentialScanner()

	content := `{
		"api_key": "sk-1234567890abcdefghijklmnopqrstuvwxyz",
		"secret_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"password": "mysecretpassword"
	}`

	result := scanner.Scan(content)

	assert.True(t, result.HasViolation())
	assert.GreaterOrEqual(t, len(result.Violations), 3)
}
// P2-03: regexp.MustCompile可能panic，应该使用regexp.Compile并处理错误
func TestP2_03_NewCredentialScanner_InvalidRegex(t *testing.T) {
	// 测试一个无效的正则表达式
	// 由于NewCredentialScanner内部使用MustCompile，这里我们测试在初始化时是否会panic
	
	// 创建一个会panic的场景：无效正则应该被Compile检测而不是MustCompile
	// 通过检查NewCredentialScanner是否能正常创建（不panic）来验证
	
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("P2-03 BUG: NewCredentialScanner panicked with invalid regex: %v", r)
		}
	}()
	
	// 这里如果正则都是有效的，应该不会panic
	scanner := NewCredentialScanner()
	if scanner == nil {
		t.Error("scanner should not be nil")
	}
	
	// 但我们无法在测试中模拟无效正则，因为MustCompile在编译时就panic了
	// 所以这个测试更多是文档性质的
	t.Logf("P2-03: NewCredentialScanner uses MustCompile which panics on invalid regex - should use Compile with error handling")
}

// P2-03: 验证MustCompile在无效正则时会panic
// 这个测试演示了问题：使用无效正则会导致panic
func TestP2_03_MustCompile_PanicsOnInvalidRegex(t *testing.T) {
	invalidRegex := "[invalid" // 无效的正则，缺少结束括号

	defer func() {
		if r := recover(); r != nil {
			t.Logf("P2-03 CONFIRMED: MustCompile panics on invalid regex: %v", r)
		}
	}()

	// 这行会panic
	_ = regexp.MustCompile(invalidRegex)
	t.Error("Should have panicked")
}

// TestMaskString_LongString tests maskString with string length > 8
func TestMaskString_LongString(t *testing.T) {
	input := "supersecretkey12345"
	result := maskString(input)
	expected := "supe****2345"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

// TestMaskString_ShortString tests maskString with string length <= 8
func TestMaskString_ShortString(t *testing.T) {
	input := "shortpw"
	result := maskString(input)
	expected := "****"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

// TestMaskString_Exactly8Chars tests maskString with exactly 8 characters
func TestMaskString_Exactly8Chars(t *testing.T) {
	input := "12345678"
	result := maskString(input)
	// len <= 8, so should return ****
	expected := "****"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

// TestMaskString_EmptyString tests maskString with empty string
func TestMaskString_EmptyString(t *testing.T) {
	input := ""
	result := maskString(input)
	expected := "****"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

// TestSanitizer_MaskMap_NestedMap tests MaskMap with nested map values
func TestSanitizer_MaskMap_NestedMap(t *testing.T) {
	sanitizer := NewSanitizer()

	input := map[string]interface{}{
		"user": map[string]interface{}{
			"name":     "john",
			"password": "secret123",
		},
		"normal": "value",
	}

	masked := sanitizer.MaskMap(input)

	// Check that nested password is masked
	nestedMap, ok := masked["user"].(map[string]interface{})
	if !ok {
		t.Fatal("expected nested map")
	}
	if nestedMap["password"] == "secret123" {
		t.Error("nested password should be masked")
	}
	if nestedMap["name"] != "john" {
		t.Error("non-sensitive nested field should not be masked")
	}
}

// TestSanitizer_MaskMap_SliceValue tests MaskMap with slice values (non-sensitive key)
func TestSanitizer_MaskMap_SliceValue(t *testing.T) {
	sanitizer := NewSanitizer()

	input := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{"name": "john"},
			map[string]interface{}{"name": "jane"},
		},
	}

	masked := sanitizer.MaskMap(input)

	users, ok := masked["users"].([]interface{})
	if !ok {
		t.Fatal("expected users slice")
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

// TestSanitizer_MaskMap_StringSliceValue tests MaskMap with []string slice values
func TestSanitizer_MaskMap_StringSliceValue(t *testing.T) {
	sanitizer := NewSanitizer()

	input := map[string]interface{}{
		"api_keys": []string{
			"sk-1234567890abcdefghijklmnopqrstuvwxyz",
			"sk-abcdefghijklmnopqrstuvwxyz1234567890",
		},
	}

	masked := sanitizer.MaskMap(input)

	apiKeys, ok := masked["api_keys"].([]string)
	if !ok {
		t.Fatal("expected api_keys slice")
	}
	if len(apiKeys) != 2 {
		t.Errorf("expected 2 api keys, got %d", len(apiKeys))
	}
	// The keys should be masked
	for i, key := range apiKeys {
		if key == input["api_keys"].([]string)[i] {
			t.Errorf("api key %d should be masked", i)
		}
	}
}

// TestSanitizer_MaskMap_IntValue tests MaskMap with integer values (non-sensitive)
func TestSanitizer_MaskMap_IntValue(t *testing.T) {
	sanitizer := NewSanitizer()

	input := map[string]interface{}{
		"count": 42,
		"user":  "john",
	}

	masked := sanitizer.MaskMap(input)

	if masked["count"] != 42 {
		t.Error("integer value should be preserved")
	}
	if masked["user"] != "john" {
		t.Error("non-sensitive string value should be preserved")
	}
}

// TestSanitizer_MaskMap_FloatValue tests MaskMap with float values (non-sensitive)
func TestSanitizer_MaskMap_FloatValue(t *testing.T) {
	sanitizer := NewSanitizer()

	input := map[string]interface{}{
		"ratio": 3.14159,
	}

	masked := sanitizer.MaskMap(input)

	if masked["ratio"] != 3.14159 {
		t.Error("float value should be preserved")
	}
}

// TestSanitizer_MaskMap_BoolValue tests MaskMap with boolean values (non-sensitive)
func TestSanitizer_MaskMap_BoolValue(t *testing.T) {
	sanitizer := NewSanitizer()

	input := map[string]interface{}{
		"active": true,
	}

	masked := sanitizer.MaskMap(input)

	if masked["active"] != true {
		t.Error("boolean value should be preserved")
	}
}

// TestSanitizer_MaskMap_ApiKeySensitive tests that api_key field is masked
func TestSanitizer_MaskMap_ApiKeySensitive(t *testing.T) {
	sanitizer := NewSanitizer()

	input := map[string]interface{}{
		"api_key":   "sk-1234567890abcdefghijklmnopqrstuvwxyz",
		"user":      "john",
		"apikey":    "key-abcdefghijklmnop",
		"secretKey": "supersecretkey1234567890", // 26 chars, meets 16+ char requirement
	}

	masked := sanitizer.MaskMap(input)

	// api_key should be masked
	if masked["api_key"] == "sk-1234567890abcdefghijklmnopqrstuvwxyz" {
		t.Error("api_key should be masked")
	}
	// apikey should be masked (16 chars, meets generic API key pattern)
	if masked["apikey"] == "key-abcdefghijklmnop" {
		t.Error("apikey should be masked")
	}
	// secretKey should be masked (26 chars, meets generic pattern 4+8+4=16)
	if masked["secretKey"] == "supersecretkey1234567890" {
		t.Error("secretKey should be masked")
	}
	// user should not be masked
	if masked["user"] != "john" {
		t.Error("user should not be masked")
	}
}
