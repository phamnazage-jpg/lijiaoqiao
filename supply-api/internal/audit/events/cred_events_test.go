package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCREDEvents_Categories(t *testing.T) {
	// 测试 CRED 事件类别
	events := GetCREDEvents()

	// CRED-EXPOSE-RESPONSE: 响应中暴露凭证
	assert.Contains(t, events, "CRED-EXPOSE-RESPONSE", "Should contain CRED-EXPOSE-RESPONSE")

	// CRED-INGRESS-PLATFORM: 平台凭证入站
	assert.Contains(t, events, "CRED-INGRESS-PLATFORM", "Should contain CRED-INGRESS-PLATFORM")

	// CRED-DIRECT-SUPPLIER: 直连供应商
	assert.Contains(t, events, "CRED-DIRECT-SUPPLIER", "Should contain CRED-DIRECT-SUPPLIER")
}

func TestCREDEvents_ExposeEvents(t *testing.T) {
	// 测试 CRED-EXPOSE 事件
	events := GetCREDExposeEvents()

	assert.Contains(t, events, "CRED-EXPOSE-RESPONSE")
	assert.Contains(t, events, "CRED-EXPOSE-LOG")
	assert.Contains(t, events, "CRED-EXPOSE-EXPORT")
}

func TestCREDEvents_IngressEvents(t *testing.T) {
	// 测试 CRED-INGRESS 事件
	events := GetCREDFngressEvents()

	assert.Contains(t, events, "CRED-INGRESS-PLATFORM")
	assert.Contains(t, events, "CRED-INGRESS-SUPPLIER")
}

func TestCREDEvents_DirectEvents(t *testing.T) {
	// 测试 CRED-DIRECT 事件
	events := GetCREDDnirectEvents()

	assert.Contains(t, events, "CRED-DIRECT-SUPPLIER")
	assert.Contains(t, events, "CRED-DIRECT-BYPASS")
}

func TestCREDEvents_GetEventCategory(t *testing.T) {
	// 所有CRED事件的类别应该是CRED
	events := GetCREDEvents()
	for _, eventName := range events {
		category := GetCREDEventCategory(eventName)
		assert.Equal(t, "CRED", category, "Event %s should have category CRED", eventName)
	}
}

func TestCREDEvents_GetEventSubCategory(t *testing.T) {
	// 测试CRED事件的子类别
	testCases := []struct {
		eventName          string
		expectedSubCategory string
	}{
		{"CRED-EXPOSE-RESPONSE", "EXPOSE"},
		{"CRED-INGRESS-PLATFORM", "INGRESS"},
		{"CRED-DIRECT-SUPPLIER", "DIRECT"},
		{"CRED-ROTATE", "ROTATE"},
		{"CRED-REVOKE", "REVOKE"},
	}

	for _, tc := range testCases {
		t.Run(tc.eventName, func(t *testing.T) {
			subCategory := GetCREDEventSubCategory(tc.eventName)
			assert.Equal(t, tc.expectedSubCategory, subCategory)
		})
	}
}

func TestCREDEvents_IsValidEvent(t *testing.T) {
	// 测试有效事件验证
	assert.True(t, IsValidCREDEvent("CRED-EXPOSE-RESPONSE"))
	assert.True(t, IsValidCREDEvent("CRED-INGRESS-PLATFORM"))
	assert.True(t, IsValidCREDEvent("CRED-DIRECT-SUPPLIER"))
	assert.False(t, IsValidCREDEvent("INVALID-EVENT"))
	assert.False(t, IsValidCREDEvent("token.authn.success"))
}

func TestCREDEvents_IsM013Event(t *testing.T) {
	// 测试M-013相关事件
	assert.True(t, IsCREDExposeEvent("CRED-EXPOSE-RESPONSE"))
	assert.True(t, IsCREDExposeEvent("CRED-EXPOSE-LOG"))
	assert.False(t, IsCREDExposeEvent("CRED-INGRESS-PLATFORM"))
}

func TestCREDEvents_IsM014Event(t *testing.T) {
	// 测试M-014相关事件
	assert.True(t, IsCREDFngressEvent("CRED-INGRESS-PLATFORM"))
	assert.True(t, IsCREDFngressEvent("CRED-INGRESS-SUPPLIER"))
	assert.False(t, IsCREDFngressEvent("CRED-EXPOSE-RESPONSE"))
}

func TestCREDEvents_IsM015Event(t *testing.T) {
	// 测试M-015相关事件
	assert.True(t, IsCREDDnirectEvent("CRED-DIRECT-SUPPLIER"))
	assert.True(t, IsCREDDnirectEvent("CRED-DIRECT-BYPASS"))
	assert.False(t, IsCREDDnirectEvent("CRED-INGRESS-PLATFORM"))
}

func TestCREDEvents_GetMetricName(t *testing.T) {
	// 测试指标名称映射
	testCases := []struct {
		eventName       string
		expectedMetric  string
	}{
		{"CRED-EXPOSE-RESPONSE", "supplier_credential_exposure_events"},
		{"CRED-EXPOSE-LOG", "supplier_credential_exposure_events"},
		{"CRED-INGRESS-PLATFORM", "platform_credential_ingress_coverage_pct"},
		{"CRED-DIRECT-SUPPLIER", "direct_supplier_call_by_consumer_events"},
	}

	for _, tc := range testCases {
		t.Run(tc.eventName, func(t *testing.T) {
			metric := GetCREDMetricName(tc.eventName)
			assert.Equal(t, tc.expectedMetric, metric)
		})
	}
}

func TestCREDEvents_GetResultCode(t *testing.T) {
	// 测试CRED事件结果码
	testCases := []struct {
		eventName    string
		expectedCode string
	}{
		{"CRED-EXPOSE-RESPONSE", "SEC_CRED_EXPOSED"},
		{"CRED-INGRESS-PLATFORM", "CRED_INGRESS_OK"},
		{"CRED-DIRECT-SUPPLIER", "SEC_DIRECT_BYPASS"},
	}

	for _, tc := range testCases {
		t.Run(tc.eventName, func(t *testing.T) {
			code := GetCREDEventResultCode(tc.eventName)
			assert.Equal(t, tc.expectedCode, code)
		})
	}
}