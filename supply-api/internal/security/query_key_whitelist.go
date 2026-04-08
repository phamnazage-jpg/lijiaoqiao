package security

import (
	"net/url"
	"strings"
)

// ==================== P0-04 Query Key白名单检测 ====================

// AllowedQueryParam 白名单参数
type AllowedQueryParam struct {
	Name        string
	Description string
}

// GetAllowedQueryParams 获取允许的query参数白名单
func GetAllowedQueryParams() []AllowedQueryParam {
	return []AllowedQueryParam{
		// 分页
		{Name: "page", Description: "页码"},
		{Name: "page_size", Description: "每页大小"},
		{Name: "limit", Description: "限制数量"},
		{Name: "offset", Description: "偏移量"},

		// 排序
		{Name: "sort", Description: "排序字段"},
		{Name: "order", Description: "排序方向"},

		// 过滤
		{Name: "filter", Description: "过滤条件"},
		{Name: "search", Description: "搜索关键词"},

		// 时间范围
		{Name: "start_date", Description: "开始日期"},
		{Name: "end_date", Description: "结束日期"},
		{Name: "from", Description: "起始时间"},
		{Name: "to", Description: "结束时间"},

		// 视图选项
		{Name: "format", Description: "响应格式"},
		{Name: "fields", Description: "字段选择"},

		// 调试选项
		{Name: "debug", Description: "调试模式"},
	}
}

// GetAllowedParamNames 获取白名单参数名集合
func GetAllowedParamNames() map[string]bool {
	params := GetAllowedQueryParams()
	allowed := make(map[string]bool)
	for _, p := range params {
		allowed[strings.ToLower(p.Name)] = true
	}
	return allowed
}

// isQueryParamAllowed 检查参数是否在白名单中（大小写不敏感）
func isQueryParamAllowed(param string, whitelist []AllowedQueryParam) bool {
	lowerParam := strings.ToLower(param)
	lowerWhitelist := make(map[string]bool)
	for _, p := range whitelist {
		lowerWhitelist[strings.ToLower(p.Name)] = true
	}
	return lowerWhitelist[lowerParam]
}

// isQueryParamBlocked 检查参数是否被禁止（大小写不敏感）
func isQueryParamBlocked(param string, whitelist []AllowedQueryParam) bool {
	return !isQueryParamAllowed(param, whitelist)
}

// blockedParamNames 禁止的参数名（包含各种变体）
var blockedParamNames = []string{
	"key", "api_key", "apikey", "api-key",
	"token", "access_token", "access-token",
	"refresh_token", "refresh-token",
	"secret", "secret_key", "secretkey",
	"password", "passwd", "pwd",
	"credential", "cred",
	"auth", "authorization",
	"session", "session_id", "sessionid",
	"jwt", "jti",
	"signature", "sig",
	"private", "private_key", "privatekey",
}

// detectBlockedParams 检测是否有被禁止的参数（支持URL解码和大小写不敏感）
func detectBlockedParams(query url.Values, whitelist []AllowedQueryParam) bool {
	whitelistMap := make(map[string]bool)
	for _, p := range whitelist {
		whitelistMap[strings.ToLower(p.Name)] = true
	}

	for param := range query {
		// 1. 检查白名单
		if whitelistMap[strings.ToLower(param)] {
			continue
		}

		// 2. 检查是否包含敏感关键词（即使参数名不同）
		lowerParam := strings.ToLower(param)
		if containsSensitiveKeyword(lowerParam) {
			return true
		}

		// 3. 检查参数值是否可疑
		value := query.Get(param)
		if isSuspiciousQueryValue(param, value) {
			return true
		}
	}

	return false
}

// containsSensitiveKeyword 检查是否包含敏感关键词
func containsSensitiveKeyword(param string) bool {
	sensitiveKeywords := []string{
		"key", "token", "secret", "password", "credential",
		"auth", "jwt", "signature", "private",
	}

	for _, kw := range sensitiveKeywords {
		if strings.Contains(param, kw) {
			return true
		}
	}
	return false
}

// isSuspiciousQueryValue 检查query参数值是否可疑
// 可疑模式：值看起来像API key、Bearer token等
func isSuspiciousQueryValue(param, value string) bool {
	if value == "" {
		return false
	}

	// 1. 检查JWT格式（即使参数名不像token）
	if strings.HasPrefix(value, "eyJ") && strings.Count(value, ".") == 2 {
		return true
	}

	// 2. 检查Bearer token格式
	if strings.HasPrefix(value, "Bearer ") || strings.HasPrefix(value, "bearer ") {
		return true
	}

	// 3. 检查长度 - 可疑的API key通常较长
	if len(value) > 20 && looksLikeAPIKey(value) {
		return true
	}

	// 4. 检查参数名是否包含敏感关键词，且值较长
	lowerParam := strings.ToLower(param)
	if len(value) > 20 {
		if containsSensitiveKeyword(lowerParam) {
			return true
		}
	}

	return false
}

// looksLikeAPIKey 检查值是否像API key
func looksLikeAPIKey(value string) bool {
	// 常见的API key前缀
	apiKeyPrefixes := []string{
		"sk-", "sk_", // OpenAI
		"ak-", "ak_", // AWS
		"pk-", "pk_", // Stripe
		"ghp_", "github_", // GitHub
		"xoxb-", // Slack
		"AIza", // Google API
	}

	lowerValue := strings.ToLower(value)
	for _, prefix := range apiKeyPrefixes {
		if strings.HasPrefix(lowerValue, prefix) {
			return true
		}
	}

	// 检查是否是长哈希值 (32+字符的十六进制)
	if len(value) >= 32 && isHexString(value) {
		return true
	}

	return false
}

// isHexString 检查字符串是否是十六进制
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return len(s) >= 32
}

// QueryKeyValidationResult Query Key验证结果
type QueryKeyValidationResult struct {
	Allowed   bool
	BlockedParam string
	Reason    string
}

// ValidateQueryParams 验证query参数
func ValidateQueryParams(rawQuery string) *QueryKeyValidationResult {
	parsed, err := url.ParseQuery(rawQuery)
	if err != nil {
		return &QueryKeyValidationResult{
			Allowed: false,
			Reason:  "invalid query string",
		}
	}

	whitelist := GetAllowedQueryParams()
	if detectBlockedParams(parsed, whitelist) {
		// 找出被阻止的参数
		for param := range parsed {
			if !isQueryParamAllowed(param, whitelist) {
				return &QueryKeyValidationResult{
					Allowed:      false,
					BlockedParam: param,
					Reason:       "query parameter not in whitelist or suspicious",
				}
			}
		}
	}

	return &QueryKeyValidationResult{
		Allowed: true,
	}
}
