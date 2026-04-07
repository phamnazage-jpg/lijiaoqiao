package sanitizer

import (
	"regexp"
	"strings"
)

// ScanRule 扫描规则
type ScanRule struct {
	ID          string
	Pattern     *regexp.Regexp
	Description string
	Severity    string
}

// Violation 违规项
type Violation struct {
	Type        string // 违规类型
	Pattern     string // 匹配的正则模式
	Value       string // 匹配的值（已脱敏）
	Description string
}

// ScanResult 扫描结果
type ScanResult struct {
	Violations []Violation
	Passed     bool
}

// NewScanResult 创建扫描结果
func NewScanResult() *ScanResult {
	return &ScanResult{
		Violations: []Violation{},
		Passed:     true,
	}
}

// HasViolation 检查是否有违规
func (r *ScanResult) HasViolation() bool {
	return len(r.Violations) > 0
}

// AddViolation 添加违规项
func (r *ScanResult) AddViolation(v Violation) {
	r.Violations = append(r.Violations, v)
	r.Passed = false
}

// CredentialScanner 凭证扫描器
type CredentialScanner struct {
	rules []ScanRule
}

// compileRegex 安全编译正则表达式，避免panic
func compileRegex(pattern string) *regexp.Regexp {
	re, err := regexp.Compile(pattern)
	if err != nil {
		// 如果编译失败，使用一个永远不会匹配的pattern
		// 这样可以避免panic，同时让扫描器继续工作
		return regexp.MustCompile("(?!)")
	}
	return re
}

// NewCredentialScanner 创建凭证扫描器
func NewCredentialScanner() *CredentialScanner {
	scanner := &CredentialScanner{
		rules: []ScanRule{
			{
				ID:          "openai_key",
				Pattern:     compileRegex(`sk-[a-zA-Z0-9]{20,}`),
				Description: "OpenAI API Key",
				Severity:    "HIGH",
			},
			{
				ID:          "api_key",
				Pattern:     compileRegex(`(?i)(api[_-]?key|apikey)["\s:=]+['"]?([a-zA-Z0-9_\-]{16,})['"]?`),
				Description: "Generic API Key",
				Severity:    "MEDIUM",
			},
			{
				ID:          "aws_access_key",
				Pattern:     compileRegex(`(?i)(access[_-]?key[_-]?id|aws[_-]?access[_-]?key)["\s:=]+['"]?(AKIA[0-9A-Z]{16})['"]?`),
				Description: "AWS Access Key ID",
				Severity:    "HIGH",
			},
			{
				ID:          "aws_secret_key",
				Pattern:     compileRegex(`(?i)(secret[_-]?key|aws[_-]?.*secret[_-]?key)["\s:=]+['"]?([a-zA-Z0-9/+=]{40})['"]?`),
				Description: "AWS Secret Access Key",
				Severity:    "HIGH",
			},
			{
				ID:          "password",
				Pattern:     compileRegex(`(?i)(password|passwd|pwd)["\s:=]+['"]?([a-zA-Z0-9@#$%^&*!]{8,})['"]?`),
				Description: "Password",
				Severity:    "HIGH",
			},
			{
				ID:          "bearer_token",
				Pattern:     compileRegex(`(?i)(token|bearer|authorization)["\s:=]+['"]?([Bb]earer\s+)?([a-zA-Z0-9_\-\.]+)['"]?`),
				Description: "Bearer Token",
				Severity:    "MEDIUM",
			},
			{
				ID:          "private_key",
				Pattern:     compileRegex(`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`),
				Description: "Private Key",
				Severity:    "CRITICAL",
			},
			{
				ID:          "secret",
				Pattern:     compileRegex(`(?i)(secret|client[_-]?secret)["\s:=]+['"]?([a-zA-Z0-9_\-]{16,})['"]?`),
				Description: "Secret",
				Severity:    "HIGH",
			},
		},
	}
	return scanner
}

// Scan 扫描内容
func (s *CredentialScanner) Scan(content string) *ScanResult {
	result := NewScanResult()

	for _, rule := range s.rules {
		matches := rule.Pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			// 构建违规项
			violation := Violation{
				Type:        rule.ID,
				Pattern:     rule.Pattern.String(),
				Description: rule.Description,
			}

			// 提取匹配的值（取最后一个匹配组）
			if len(match) > 1 {
				violation.Value = maskString(match[len(match)-1])
			} else {
				violation.Value = maskString(match[0])
			}

			result.AddViolation(violation)
		}
	}

	return result
}

// GetRules 获取扫描规则
func (s *CredentialScanner) GetRules() []ScanRule {
	return s.rules
}

// Sanitizer 脱敏器
type Sanitizer struct {
	patterns []*regexp.Regexp
}

// NewSanitizer 创建脱敏器
func NewSanitizer() *Sanitizer {
	return &Sanitizer{
		patterns: []*regexp.Regexp{
			// OpenAI API Key
			compileRegex(`(sk-[a-zA-Z0-9]{4})[a-zA-Z0-9]+([a-zA-Z0-9]{4})`),
			// AWS Access Key
			compileRegex(`(AKIA[0-9A-Z]{4})[0-9A-Z]+([0-9A-Z]{4})`),
			// Generic API Key
			compileRegex(`([a-zA-Z0-9_\-]{4})[a-zA-Z0-9_\-]{8,}([a-zA-Z0-9_\-]{4})`),
			// Password
			compileRegex(`([a-zA-Z0-9@#$%^&*!]{4})[a-zA-Z0-9@#$%^&*!]+([a-zA-Z0-9@#$%^&*!]{4})`),
		},
	}
}

// Mask 对字符串进行脱敏
func (s *Sanitizer) Mask(content string) string {
	result := content

	for _, pattern := range s.patterns {
		// 替换为格式：前4字符 + **** + 后4字符
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			// 尝试分组替换
			re := compileRegex(`^(.{4}).+(.{4})$`)
			submatch := re.FindStringSubmatch(match)
			if len(submatch) == 3 {
				return submatch[1] + "****" + submatch[2]
			}
			// 如果无法分组，直接掩码
			if len(match) > 8 {
				return match[:4] + "****" + match[len(match)-4:]
			}
			return "****"
		})
	}

	return result
}

// MaskMap 对map进行脱敏
func (s *Sanitizer) MaskMap(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range data {
		if IsSensitiveField(key) {
			switch v := value.(type) {
			case string:
				result[key] = s.Mask(v)
			case []string:
				result[key] = s.MaskSlice(v)
			case []interface{}:
				result[key] = s.maskSliceInterface(v)
			default:
				result[key] = value
			}
		} else {
			result[key] = s.maskValue(value)
		}
	}

	return result
}

// maskSliceInterface 处理 []interface{} 类型的切片
func (s *Sanitizer) maskSliceInterface(data []interface{}) []interface{} {
	result := make([]interface{}, len(data))
	for i, item := range data {
		result[i] = s.maskValue(item)
	}
	return result
}

// MaskSlice 对slice进行脱敏
func (s *Sanitizer) MaskSlice(data []string) []string {
	result := make([]string, len(data))
	for i, item := range data {
		result[i] = s.Mask(item)
	}
	return result
}

// maskValue 递归掩码
func (s *Sanitizer) maskValue(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return s.Mask(v)
	case map[string]interface{}:
		return s.MaskMap(v)
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = s.maskValue(item)
		}
		return result
	case []string:
		return s.MaskSlice(v)
	default:
		return v
	}
}

// maskString 掩码字符串
func maskString(s string) string {
	if len(s) > 8 {
		return s[:4] + "****" + s[len(s)-4:]
	}
	return "****"
}

// GetSensitiveFields 获取敏感字段列表
func GetSensitiveFields() []string {
	return []string{
		"api_key",
		"apikey",
		"secret",
		"secret_key",
		"password",
		"passwd",
		"pwd",
		"token",
		"access_key",
		"access_key_id",
		"private_key",
		"session_id",
		"authorization",
		"bearer",
		"client_secret",
		"credentials",
	}
}

// IsSensitiveField 判断字段名是否为敏感字段
func IsSensitiveField(fieldName string) bool {
	lowerName := strings.ToLower(fieldName)
	sensitiveFields := GetSensitiveFields()

	for _, sf := range sensitiveFields {
		if strings.Contains(lowerName, sf) {
			return true
		}
	}

	return false
}