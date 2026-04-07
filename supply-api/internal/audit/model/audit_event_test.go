package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAuditEvent_NewEvent_ValidInput(t *testing.T) {
	// 测试创建审计事件
	event := NewAuditEvent(
		"CRED-EXPOSE-RESPONSE",
		"CRED",
		"EXPOSE",
		"supplier_credential_exposure_events",
		"test-request-id",
		"test-trace-id",
		1001,
		"user",
		"admin",
		2001,
		"supplier",
		"account",
		12345,
		"create",
		"platform_token",
		"api",
		"192.168.1.1",
		true,
		"SEC_CRED_EXPOSED",
		"Credential exposed in response",
	)

	// 验证字段
	assert.NotEmpty(t, event.EventID, "EventID should not be empty")
	assert.Equal(t, "CRED-EXPOSE-RESPONSE", event.EventName, "EventName should match")
	assert.Equal(t, "CRED", event.EventCategory, "EventCategory should match")
	assert.Equal(t, "EXPOSE", event.EventSubCategory, "EventSubCategory should match")
	assert.Equal(t, "test-request-id", event.RequestID, "RequestID should match")
	assert.Equal(t, "test-trace-id", event.TraceID, "TraceID should match")
	assert.Equal(t, int64(1001), event.OperatorID, "OperatorID should match")
	assert.Equal(t, "user", event.OperatorType, "OperatorType should match")
	assert.Equal(t, "admin", event.OperatorRole, "OperatorRole should match")
	assert.Equal(t, int64(2001), event.TenantID, "TenantID should match")
	assert.Equal(t, "supplier", event.TenantType, "TenantType should match")
	assert.Equal(t, "account", event.ObjectType, "ObjectType should match")
	assert.Equal(t, int64(12345), event.ObjectID, "ObjectID should match")
	assert.Equal(t, "create", event.Action, "Action should match")
	assert.Equal(t, "platform_token", event.CredentialType, "CredentialType should match")
	assert.Equal(t, "api", event.SourceType, "SourceType should match")
	assert.Equal(t, "192.168.1.1", event.SourceIP, "SourceIP should match")
	assert.True(t, event.Success, "Success should be true")
	assert.Equal(t, "SEC_CRED_EXPOSED", event.ResultCode, "ResultCode should match")
	assert.Equal(t, "Credential exposed in response", event.ResultMessage, "ResultMessage should match")

	// 验证时间戳
	assert.False(t, event.Timestamp.IsZero(), "Timestamp should not be zero")
	assert.True(t, event.TimestampMs > 0, "TimestampMs should be positive")
	assert.False(t, event.CreatedAt.IsZero(), "CreatedAt should not be zero")

	// 验证版本
	assert.Equal(t, 1, event.Version, "Version should be 1")
}

func TestAuditEvent_NewEvent_SecurityFlags(t *testing.T) {
	// 验证SecurityFlags字段
	event := NewAuditEvent(
		"CRED-EXPOSE-RESPONSE",
		"CRED",
		"EXPOSE",
		"supplier_credential_exposure_events",
		"test-request-id",
		"test-trace-id",
		1001,
		"user",
		"admin",
		2001,
		"supplier",
		"account",
		12345,
		"create",
		"platform_token",
		"api",
		"192.168.1.1",
		true,
		"SEC_CRED_EXPOSED",
		"Credential exposed in response",
	)

	// 验证安全标记
	assert.NotNil(t, event.SecurityFlags, "SecurityFlags should not be nil")
	assert.True(t, event.SecurityFlags.HasCredential, "HasCredential should be true")
	assert.True(t, event.SecurityFlags.CredentialExposed, "CredentialExposed should be true")
	assert.False(t, event.SecurityFlags.Desensitized, "Desensitized should be false by default")
	assert.False(t, event.SecurityFlags.Scanned, "Scanned should be false by default")
	assert.False(t, event.SecurityFlags.ScanPassed, "ScanPassed should be false by default")
	assert.Empty(t, event.SecurityFlags.ViolationTypes, "ViolationTypes should be empty by default")
}

func TestAuditEvent_NewEvent_WithSecurityFlags(t *testing.T) {
	// 测试带有完整安全标记的事件
	securityFlags := SecurityFlags{
		HasCredential:      true,
		CredentialExposed:  true,
		Desensitized:       false,
		Scanned:            true,
		ScanPassed:         false,
		ViolationTypes:     []string{"api_key", "secret"},
	}

	event := NewAuditEventWithSecurityFlags(
		"CRED-EXPOSE-RESPONSE",
		"CRED",
		"EXPOSE",
		"supplier_credential_exposure_events",
		"test-request-id",
		"test-trace-id",
		1001,
		"user",
		"admin",
		2001,
		"supplier",
		"account",
		12345,
		"create",
		"platform_token",
		"api",
		"192.168.1.1",
		true,
		"SEC_CRED_EXPOSED",
		"Credential exposed in response",
		securityFlags,
		80,
	)

	// 验证安全标记
	assert.Equal(t, true, event.SecurityFlags.HasCredential)
	assert.Equal(t, true, event.SecurityFlags.CredentialExposed)
	assert.Equal(t, false, event.SecurityFlags.Desensitized)
	assert.Equal(t, true, event.SecurityFlags.Scanned)
	assert.Equal(t, false, event.SecurityFlags.ScanPassed)
	assert.Equal(t, []string{"api_key", "secret"}, event.SecurityFlags.ViolationTypes)

	// 验证风险评分
	assert.Equal(t, 80, event.RiskScore, "RiskScore should be 80")
}

func TestAuditEvent_NewAuditEventWithIdempotencyKey(t *testing.T) {
	// 测试带幂等键的事件
	event := NewAuditEvent(
		"token.query_key.rejected",
		"AUTH",
		"QUERY",
		"query_key_external_reject_rate_pct",
		"test-request-id",
		"test-trace-id",
		1001,
		"user",
		"admin",
		2001,
		"supplier",
		"account",
		12345,
		"query",
		"query_key",
		"api",
		"192.168.1.1",
		true,
		"AUTH_QUERY_KEY",
		"Query key request",
	)

	// 设置幂等键
	event.SetIdempotencyKey("idem-key-12345")

	assert.Equal(t, "idem-key-12345", event.IdempotencyKey, "IdempotencyKey should be set")
}

func TestAuditEvent_NewAuditEventWithTarget(t *testing.T) {
	// 测试带目标信息的事件（用于M-015直连检测）
	event := NewAuditEvent(
		"CRED-DIRECT-SUPPLIER",
		"CRED",
		"DIRECT",
		"direct_supplier_call_by_consumer_events",
		"test-request-id",
		"test-trace-id",
		1001,
		"user",
		"admin",
		2001,
		"supplier",
		"api",
		12345,
		"call",
		"none",
		"api",
		"192.168.1.1",
		false,
		"SEC_DIRECT_BYPASS",
		"Direct call detected",
	)

	// 设置直连目标
	event.SetTarget("upstream_api", "https://supplier.example.com/v1/chat/completions", true)

	assert.Equal(t, "upstream_api", event.TargetType, "TargetType should be set")
	assert.Equal(t, "https://supplier.example.com/v1/chat/completions", event.TargetEndpoint, "TargetEndpoint should be set")
	assert.True(t, event.TargetDirect, "TargetDirect should be true")
}

func TestAuditEvent_NewAuditEventWithInvariantRule(t *testing.T) {
	// 测试不变量规则（用于SECURITY事件）
	event := NewAuditEvent(
		"INVARIANT-VIOLATION",
		"SECURITY",
		"VIOLATION",
		"invariant_violation",
		"test-request-id",
		"test-trace-id",
		1001,
		"system",
		"admin",
		2001,
		"supplier",
		"settlement",
		12345,
		"withdraw",
		"platform_token",
		"api",
		"192.168.1.1",
		false,
		"SEC_INV_SET_001",
		"Settlement cannot be revoked",
	)

	// 设置不变量规则
	event.SetInvariantRule("INV-SET-001")

	assert.Equal(t, "INV-SET-001", event.InvariantRule, "InvariantRule should be set")
	assert.Contains(t, event.ComplianceTags, "XR-001", "ComplianceTags should contain XR-001")
}

func TestSecurityFlags_HasViolation(t *testing.T) {
	// 测试安全标记的违规检测
	sf := NewSecurityFlags()

	// 初始状态无违规
	assert.False(t, sf.HasViolation(), "Should have no violation initially")

	// 添加违规类型
	sf.AddViolationType("api_key")
	assert.True(t, sf.HasViolation(), "Should have violation after adding type")
	assert.True(t, sf.HasViolationOfType("api_key"), "Should have api_key violation")
	assert.False(t, sf.HasViolationOfType("password"), "Should not have password violation")
}

func TestSecurityFlags_AddViolationType(t *testing.T) {
	sf := NewSecurityFlags()

	sf.AddViolationType("api_key")
	sf.AddViolationType("secret")
	sf.AddViolationType("password")

	assert.Len(t, sf.ViolationTypes, 3, "Should have 3 violation types")
	assert.Contains(t, sf.ViolationTypes, "api_key")
	assert.Contains(t, sf.ViolationTypes, "secret")
	assert.Contains(t, sf.ViolationTypes, "password")
}

func TestAuditEvent_MetricName(t *testing.T) {
	// 测试事件与指标的映射
	testCases := []struct {
		eventName       string
		expectedMetric  string
	}{
		{"CRED-EXPOSE-RESPONSE", "supplier_credential_exposure_events"},
		{"CRED-EXPOSE-LOG", "supplier_credential_exposure_events"},
		{"CRED-INGRESS-PLATFORM", "platform_credential_ingress_coverage_pct"},
		{"CRED-DIRECT-SUPPLIER", "direct_supplier_call_by_consumer_events"},
		{"token.query_key.rejected", "query_key_external_reject_rate_pct"},
		{"token.query_key.rejected", "query_key_external_reject_rate_pct"},
	}

	for _, tc := range testCases {
		t.Run(tc.eventName, func(t *testing.T) {
			event := &AuditEvent{
				EventName: tc.eventName,
			}
			assert.Equal(t, tc.expectedMetric, event.GetMetricName(), "MetricName should match for %s", tc.eventName)
		})
	}
}

func TestAuditEvent_IsM013Event(t *testing.T) {
	// M-013: 凭证暴露事件
	assert.True(t, IsM013Event("CRED-EXPOSE-RESPONSE"), "CRED-EXPOSE-RESPONSE is M-013 event")
	assert.True(t, IsM013Event("CRED-EXPOSE-LOG"), "CRED-EXPOSE-LOG is M-013 event")
	assert.True(t, IsM013Event("CRED-EXPOSE"), "CRED-EXPOSE is M-013 event")
	assert.False(t, IsM013Event("CRED-INGRESS-PLATFORM"), "CRED-INGRESS-PLATFORM is not M-013 event")
	assert.False(t, IsM013Event("token.query_key.rejected"), "token.query_key.rejected is not M-013 event")
}

func TestAuditEvent_IsM014Event(t *testing.T) {
	// M-014: 凭证入站事件
	assert.True(t, IsM014Event("CRED-INGRESS-PLATFORM"), "CRED-INGRESS-PLATFORM is M-014 event")
	assert.True(t, IsM014Event("CRED-INGRESS"), "CRED-INGRESS is M-014 event")
	assert.False(t, IsM014Event("CRED-EXPOSE-RESPONSE"), "CRED-EXPOSE-RESPONSE is not M-014 event")
}

func TestAuditEvent_IsM015Event(t *testing.T) {
	// M-015: 直连绕过事件
	assert.True(t, IsM015Event("CRED-DIRECT-SUPPLIER"), "CRED-DIRECT-SUPPLIER is M-015 event")
	assert.True(t, IsM015Event("CRED-DIRECT"), "CRED-DIRECT is M-015 event")
	assert.False(t, IsM015Event("CRED-INGRESS-PLATFORM"), "CRED-INGRESS-PLATFORM is not M-015 event")
}

func TestAuditEvent_IsM016Event(t *testing.T) {
	// M-016: query key拒绝事件
	assert.True(t, IsM016Event("token.query_key.rejected"), "token.query_key.rejected is M-016 event")
	assert.True(t, IsM016Event("token.query_key.rejected"), "token.query_key.rejected is M-016 event")
	assert.True(t, IsM016Event("token.query_key"), "token.query_key is M-016 event")
	assert.False(t, IsM016Event("CRED-EXPOSE-RESPONSE"), "CRED-EXPOSE-RESPONSE is not M-016 event")
}

func TestAuditEvent_CredentialType(t *testing.T) {
	// 测试凭证类型常量
	assert.Equal(t, "platform_token", CredentialTypePlatformToken)
	assert.Equal(t, "query_key", CredentialTypeQueryKey)
	assert.Equal(t, "upstream_api_key", CredentialTypeUpstreamAPIKey)
	assert.Equal(t, "none", CredentialTypeNone)
}

func TestAuditEvent_OperatorType(t *testing.T) {
	// 测试操作者类型常量
	assert.Equal(t, "user", OperatorTypeUser)
	assert.Equal(t, "system", OperatorTypeSystem)
	assert.Equal(t, "admin", OperatorTypeAdmin)
}

func TestAuditEvent_TenantType(t *testing.T) {
	// 测试租户类型常量
	assert.Equal(t, "supplier", TenantTypeSupplier)
	assert.Equal(t, "consumer", TenantTypeConsumer)
	assert.Equal(t, "platform", TenantTypePlatform)
}

func TestAuditEvent_Category(t *testing.T) {
	// 测试事件类别常量
	assert.Equal(t, "CRED", CategoryCRED)
	assert.Equal(t, "AUTH", CategoryAUTH)
	assert.Equal(t, "DATA", CategoryDATA)
	assert.Equal(t, "CONFIG", CategoryCONFIG)
	assert.Equal(t, "SECURITY", CategorySECURITY)
}

func TestAuditEvent_NewAuditEventTimestamp(t *testing.T) {
	// 测试时间戳自动生成
	before := time.Now()
	event := NewAuditEvent(
		"CRED-EXPOSE-RESPONSE",
		"CRED",
		"EXPOSE",
		"supplier_credential_exposure_events",
		"test-request-id",
		"test-trace-id",
		1001,
		"user",
		"admin",
		2001,
		"supplier",
		"account",
		12345,
		"create",
		"platform_token",
		"api",
		"192.168.1.1",
		true,
		"SEC_CRED_EXPOSED",
		"Credential exposed in response",
	)
	after := time.Now()

	// 验证时间戳在合理范围内
	assert.True(t, event.Timestamp.After(before) || event.Timestamp.Equal(before), "Timestamp should be after or equal to before")
	assert.True(t, event.Timestamp.Before(after) || event.Timestamp.Equal(after), "Timestamp should be before or equal to after")
	assert.Equal(t, event.Timestamp.UnixMilli(), event.TimestampMs, "TimestampMs should match Timestamp")
}