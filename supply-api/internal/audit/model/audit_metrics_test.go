package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==================== M-013 凭证暴露事件详情 ====================

func TestCredentialExposureDetail_New(t *testing.T) {
	// M-013: 凭证暴露事件专用
	detail := NewCredentialExposureDetail(
		"exposed_in_response",
		"response_body",
		"sk-[a-zA-Z0-9]{20,}",
		"sk-xxxxxx****xxxx",
		"SCAN-001",
	)

	assert.Equal(t, "exposed_in_response", detail.ExposureType)
	assert.Equal(t, "response_body", detail.ExposureLocation)
	assert.Equal(t, "sk-[a-zA-Z0-9]{20,}", detail.ExposurePattern)
	assert.Equal(t, "sk-xxxxxx****xxxx", detail.ExposedFragment)
	assert.Equal(t, "SCAN-001", detail.ScanRuleID)
	assert.False(t, detail.Resolved)
	assert.Nil(t, detail.ResolvedAt)
	assert.Nil(t, detail.ResolvedBy)
	assert.Empty(t, detail.ResolutionNotes)
}

func TestCredentialExposureDetail_Resolve(t *testing.T) {
	detail := NewCredentialExposureDetail(
		"exposed_in_response",
		"response_body",
		"sk-[a-zA-Z0-9]{20,}",
		"sk-xxxxxx****xxxx",
		"SCAN-001",
	)

	detail.Resolve(1001, "Fixed by adding masking")

	assert.True(t, detail.Resolved)
	assert.NotNil(t, detail.ResolvedAt)
	assert.Equal(t, int64(1001), *detail.ResolvedBy)
	assert.Equal(t, "Fixed by adding masking", detail.ResolutionNotes)
}

func TestCredentialExposureDetail_ExposureTypes(t *testing.T) {
	// 验证暴露类型常量
	validTypes := []string{
		"exposed_in_response",
		"exposed_in_log",
		"exposed_in_export",
	}

	for _, exposureType := range validTypes {
		detail := NewCredentialExposureDetail(
			exposureType,
			"response_body",
			"pattern",
			"fragment",
			"SCAN-001",
		)
		assert.Equal(t, exposureType, detail.ExposureType)
	}
}

func TestCredentialExposureDetail_ExposureLocations(t *testing.T) {
	// 验证暴露位置常量
	validLocations := []string{
		"response_body",
		"response_header",
		"log_file",
		"export_file",
	}

	for _, location := range validLocations {
		detail := NewCredentialExposureDetail(
			"exposed_in_response",
			location,
			"pattern",
			"fragment",
			"SCAN-001",
		)
		assert.Equal(t, location, detail.ExposureLocation)
	}
}

// ==================== M-014 凭证入站事件详情 ====================

func TestCredentialIngressDetail_New(t *testing.T) {
	// M-014: 凭证入站类型专用
	detail := NewCredentialIngressDetail(
		"platform_token",
		"platform_token",
		true,
		true,
		false,
	)

	assert.Equal(t, "platform_token", detail.RequestCredentialType)
	assert.Equal(t, "platform_token", detail.ExpectedCredentialType)
	assert.True(t, detail.CoverageCompliant)
	assert.True(t, detail.PlatformTokenPresent)
	assert.False(t, detail.UpstreamKeyPresent)
	assert.False(t, detail.Reviewed)
	assert.Nil(t, detail.ReviewedAt)
	assert.Nil(t, detail.ReviewedBy)
}

func TestCredentialIngressDetail_NonCompliant(t *testing.T) {
	// M-014 非合规场景：使用 query_key 而不是 platform_token
	detail := NewCredentialIngressDetail(
		"query_key",
		"platform_token",
		false,
		false,
		true,
	)

	assert.Equal(t, "query_key", detail.RequestCredentialType)
	assert.Equal(t, "platform_token", detail.ExpectedCredentialType)
	assert.False(t, detail.CoverageCompliant)
	assert.False(t, detail.PlatformTokenPresent)
	assert.True(t, detail.UpstreamKeyPresent)
}

func TestCredentialIngressDetail_Review(t *testing.T) {
	detail := NewCredentialIngressDetail(
		"platform_token",
		"platform_token",
		true,
		true,
		false,
	)

	detail.Review(1001)

	assert.True(t, detail.Reviewed)
	assert.NotNil(t, detail.ReviewedAt)
	assert.Equal(t, int64(1001), *detail.ReviewedBy)
}

func TestCredentialIngressDetail_CredentialTypes(t *testing.T) {
	// 验证凭证类型
	testCases := []struct {
		credType       string
		platformToken  bool
		upstreamKey    bool
		compliant      bool
	}{
		{"platform_token", true, false, true},
		{"query_key", false, false, false},
		{"upstream_api_key", false, true, false},
		{"none", false, false, false},
	}

	for _, tc := range testCases {
		detail := NewCredentialIngressDetail(
			tc.credType,
			"platform_token",
			tc.compliant,
			tc.platformToken,
			tc.upstreamKey,
		)
		assert.Equal(t, tc.compliant, detail.CoverageCompliant, "Compliance mismatch for %s", tc.credType)
	}
}

// ==================== M-015 直连绕过事件详情 ====================

func TestDirectCallDetail_New(t *testing.T) {
	// M-015: 直连绕过专用
	detail := NewDirectCallDetail(
		1001, // consumerID
		2001, // supplierID
		"https://supplier.example.com/v1/chat/completions",
		false, // viaPlatform
		"ip_bypass",
		"upstream_api_pattern_match",
	)

	assert.Equal(t, int64(1001), detail.ConsumerID)
	assert.Equal(t, int64(2001), detail.SupplierID)
	assert.Equal(t, "https://supplier.example.com/v1/chat/completions", detail.DirectEndpoint)
	assert.False(t, detail.ViaPlatform)
	assert.Equal(t, "ip_bypass", detail.BypassType)
	assert.Equal(t, "upstream_api_pattern_match", detail.DetectionMethod)
	assert.False(t, detail.Blocked)
	assert.Nil(t, detail.BlockedAt)
	assert.Empty(t, detail.BlockReason)
}

func TestDirectCallDetail_Block(t *testing.T) {
	detail := NewDirectCallDetail(
		1001,
		2001,
		"https://supplier.example.com/v1/chat/completions",
		false,
		"ip_bypass",
		"upstream_api_pattern_match",
	)

	detail.Block("P0 event - immediate block")

	assert.True(t, detail.Blocked)
	assert.NotNil(t, detail.BlockedAt)
	assert.Equal(t, "P0 event - immediate block", detail.BlockReason)
}

func TestDirectCallDetail_BypassTypes(t *testing.T) {
	// 验证绕过类型常量
	validBypassTypes := []string{
		"ip_bypass",
		"proxy_bypass",
		"config_bypass",
		"dns_bypass",
	}

	for _, bypassType := range validBypassTypes {
		detail := NewDirectCallDetail(
			1001,
			2001,
			"https://example.com",
			false,
			bypassType,
			"detection_method",
		)
		assert.Equal(t, bypassType, detail.BypassType)
	}
}

func TestDirectCallDetail_DetectionMethods(t *testing.T) {
	// 验证检测方法常量
	validMethods := []string{
		"upstream_api_pattern_match",
		"dns_resolution_check",
		"connection_source_check",
		"ip_whitelist_check",
	}

	for _, method := range validMethods {
		detail := NewDirectCallDetail(
			1001,
			2001,
			"https://example.com",
			false,
			"ip_bypass",
			method,
		)
		assert.Equal(t, method, detail.DetectionMethod)
	}
}

func TestDirectCallDetail_ViaPlatform(t *testing.T) {
	// 通过平台的调用不应该标记为直连
	detail := NewDirectCallDetail(
		1001,
		2001,
		"https://platform.example.com/v1/chat/completions",
		true, // viaPlatform = true
		"",
		"platform_proxy",
	)

	assert.True(t, detail.ViaPlatform)
	assert.False(t, detail.Blocked)
}

// ==================== M-016 Query Key 拒绝事件详情 ====================

func TestQueryKeyRejectDetail_New(t *testing.T) {
	// M-016: query key 拒绝专用
	detail := NewQueryKeyRejectDetail(
		"qk-12345",
		"/v1/chat/completions",
		"not_allowed",
		"QUERY_KEY_NOT_ALLOWED",
	)

	assert.Equal(t, "qk-12345", detail.QueryKeyID)
	assert.Equal(t, "/v1/chat/completions", detail.RequestedEndpoint)
	assert.Equal(t, "not_allowed", detail.RejectReason)
	assert.Equal(t, "QUERY_KEY_NOT_ALLOWED", detail.RejectCode)
	assert.True(t, detail.FirstOccurrence)
	assert.Equal(t, 1, detail.OccurrenceCount)
}

func TestQueryKeyRejectDetail_RecordOccurrence(t *testing.T) {
	detail := NewQueryKeyRejectDetail(
		"qk-12345",
		"/v1/chat/completions",
		"not_allowed",
		"QUERY_KEY_NOT_ALLOWED",
	)

	// 第二次发生
	detail.RecordOccurrence(false)
	assert.Equal(t, 2, detail.OccurrenceCount)
	assert.False(t, detail.FirstOccurrence)

	// 第三次发生
	detail.RecordOccurrence(false)
	assert.Equal(t, 3, detail.OccurrenceCount)
}

func TestQueryKeyRejectDetail_RejectReasons(t *testing.T) {
	// 验证拒绝原因常量
	validReasons := []string{
		"not_allowed",
		"expired",
		"malformed",
		"revoked",
		"rate_limited",
	}

	for _, reason := range validReasons {
		detail := NewQueryKeyRejectDetail(
			"qk-12345",
			"/v1/chat/completions",
			reason,
			"QUERY_KEY_REJECT",
		)
		assert.Equal(t, reason, detail.RejectReason)
	}
}

func TestQueryKeyRejectDetail_RejectCodes(t *testing.T) {
	// 验证拒绝码常量
	validCodes := []string{
		"QUERY_KEY_NOT_ALLOWED",
		"QUERY_KEY_EXPIRED",
		"QUERY_KEY_MALFORMED",
		"QUERY_KEY_REVOKED",
		"QUERY_KEY_RATE_LIMITED",
	}

	for _, code := range validCodes {
		detail := NewQueryKeyRejectDetail(
			"qk-12345",
			"/v1/chat/completions",
			"not_allowed",
			code,
		)
		assert.Equal(t, code, detail.RejectCode)
	}
}

// ==================== 指标计算辅助函数 ====================

func TestCalculateM013(t *testing.T) {
	// M-013: 凭证泄露事件数 = 0
	events := []struct {
		eventName string
		resolved  bool
	}{
		{"CRED-EXPOSE-RESPONSE", true},
		{"CRED-EXPOSE-RESPONSE", true},
		{"CRED-EXPOSE-LOG", false},
		{"token.authn.success", true},
	}

	var unresolvedCount int
	for _, e := range events {
		if IsM013Event(e.eventName) && !e.resolved {
			unresolvedCount++
		}
	}

	assert.Equal(t, 1, unresolvedCount, "M-013 should have 1 unresolved event")
}

func TestCalculateM014(t *testing.T) {
	// M-014: 平台凭证入站覆盖率 = 100%
	events := []struct {
		credentialType string
		compliant      bool
	}{
		{"platform_token", true},
		{"platform_token", true},
		{"query_key", false},
		{"upstream_api_key", false},
		{"platform_token", true},
	}

	var platformCount, totalCount int
	for _, e := range events {
		if IsM014Compliant(e.credentialType) {
			platformCount++
		}
		totalCount++
	}

	coverage := float64(platformCount) / float64(totalCount) * 100
	assert.Equal(t, 60.0, coverage, "M-014 coverage should be 60%%")
	assert.Equal(t, 3, platformCount)
	assert.Equal(t, 5, totalCount)
}

func TestCalculateM015(t *testing.T) {
	// M-015: 直连事件数 = 0
	events := []struct {
		targetDirect bool
		blocked      bool
	}{
		{targetDirect: true, blocked: false},
		{targetDirect: true, blocked: true},
		{targetDirect: false, blocked: false},
		{targetDirect: true, blocked: false},
	}

	var directCallCount, blockedCount int
	for _, e := range events {
		if e.targetDirect {
			directCallCount++
			if e.blocked {
				blockedCount++
			}
		}
	}

	assert.Equal(t, 3, directCallCount, "M-015 should have 3 direct call events")
	assert.Equal(t, 1, blockedCount, "M-015 should have 1 blocked event")
}

func TestCalculateM016(t *testing.T) {
	// M-016: query key 拒绝率 = 50%
	// 分母：所有query key请求（含rejected）
	// 分子：被拒绝的query key请求
	events := []struct {
		eventName string
	}{
		{"token.query_key.rejected"}, // 拒绝
		{"token.query_key.rejected"}, // 拒绝
		{"token.query_key"},          // 有效
		{"token.query_key"},          // 有效
		{"token.authn.success"},      // 非query key
	}

	var totalQueryKey, rejectedCount int
	for _, e := range events {
		if IsM016Event(e.eventName) {
			totalQueryKey++
			if IsM016QueryKeyRejectEvent(e.eventName) {
				rejectedCount++
			}
		}
	}

	rejectRate := float64(rejectedCount) / float64(totalQueryKey) * 100
	assert.Equal(t, 4, totalQueryKey, "M-016 should have 4 query key events")
	assert.Equal(t, 2, rejectedCount, "M-016 should have 2 rejected events")
	assert.Equal(t, 50.0, rejectRate, "M-016 reject rate should be 50%%")
}

// IsM014Compliant 检查凭证类型是否为M-014合规
func IsM014Compliant(credentialType string) bool {
	return credentialType == CredentialTypePlatformToken
}