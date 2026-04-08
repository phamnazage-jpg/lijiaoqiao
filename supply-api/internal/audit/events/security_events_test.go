package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSECURITYEvents_InvariantViolation(t *testing.T) {
	// 测试 invariant_violation 事件
	events := GetSECURITYEvents()

	// INV-PKG-001: 供应方资质过期
	assert.Contains(t, events, "INV-PKG-001", "Should contain INV-PKG-001")

	// INV-SET-001: processing/completed 不可撤销
	assert.Contains(t, events, "INV-SET-001", "Should contain INV-SET-001")
}

func TestSECURITYEvents_AllEvents(t *testing.T) {
	// 测试所有SECURITY事件
	events := GetSECURITYEvents()

	// 验证不变量违反事件
	invariantEvents := GetInvariantViolationEvents()
	for _, event := range invariantEvents {
		assert.Contains(t, events, event, "SECURITY events should contain %s", event)
	}
}

func TestSECURITYEvents_GetInvariantViolationEvents(t *testing.T) {
	events := GetInvariantViolationEvents()

	// INV-PKG-001: 供应方资质过期
	assert.Contains(t, events, "INV-PKG-001")

	// INV-PKG-002: 供应方余额为负
	assert.Contains(t, events, "INV-PKG-002")

	// INV-PKG-003: 售价不得低于保护价
	assert.Contains(t, events, "INV-PKG-003")

	// INV-SET-001: processing/completed 不可撤销
	assert.Contains(t, events, "INV-SET-001")

	// INV-SET-002: 提现金额不得超过可提现余额
	assert.Contains(t, events, "INV-SET-002")

	// INV-SET-003: 结算单金额与余额流水必须平衡
	assert.Contains(t, events, "INV-SET-003")
}

func TestSECURITYEvents_GetSecurityAlertEvents(t *testing.T) {
	events := GetSecurityAlertEvents()

	// 安全告警事件应该存在
	assert.NotEmpty(t, events)
}

func TestSECURITYEvents_GetSecurityBreachEvents(t *testing.T) {
	events := GetSecurityBreachEvents()

	// 安全突破事件应该存在
	assert.NotEmpty(t, events)
}

func TestSECURITYEvents_GetEventCategory(t *testing.T) {
	// 所有SECURITY事件的类别应该是SECURITY
	events := GetSECURITYEvents()
	for _, eventName := range events {
		category := GetEventCategory(eventName)
		assert.Equal(t, "SECURITY", category, "Event %s should have category SECURITY", eventName)
	}
}

func TestSECURITYEvents_GetResultCode(t *testing.T) {
	// 测试不变量违反事件的结果码映射
	testCases := []struct {
		eventName    string
		expectedCode string
	}{
		{"INV-PKG-001", "SEC_INV_PKG_001"},
		{"INV-PKG-002", "SEC_INV_PKG_002"},
		{"INV-PKG-003", "SEC_INV_PKG_003"},
		{"INV-SET-001", "SEC_INV_SET_001"},
		{"INV-SET-002", "SEC_INV_SET_002"},
		{"INV-SET-003", "SEC_INV_SET_003"},
	}

	for _, tc := range testCases {
		t.Run(tc.eventName, func(t *testing.T) {
			code := GetResultCode(tc.eventName)
			assert.Equal(t, tc.expectedCode, code, "Result code mismatch for %s", tc.eventName)
		})
	}
}

func TestSECURITYEvents_GetEventDescription(t *testing.T) {
	// 测试事件描述
	desc := GetEventDescription("INV-PKG-001")
	assert.NotEmpty(t, desc)
	assert.Contains(t, desc, "供应方资质", "Description should contain 供应方资质")
}

func TestSECURITYEvents_IsValidEvent(t *testing.T) {
	// 测试有效事件验证
	assert.True(t, IsValidEvent("INV-PKG-001"))
	assert.True(t, IsValidEvent("INV-SET-001"))
	assert.False(t, IsValidEvent("INVALID-EVENT"))
	assert.False(t, IsValidEvent(""))
}

func TestSECURITYEvents_GetEventSubCategory(t *testing.T) {
	// SECURITY事件的子类别应该是VIOLATION/ALERT/BREACH
	testCases := []struct {
		eventName          string
		expectedSubCategory string
	}{
		{"INV-PKG-001", "VIOLATION"},
		{"INV-PKG-002", "VIOLATION"},
		{"INV-PKG-003", "VIOLATION"},
		{"INV-SET-001", "VIOLATION"},
		{"INV-SET-002", "VIOLATION"},
		{"INV-SET-003", "VIOLATION"},
		{"SEC-BREACH-001", "BREACH"},
		{"SEC-BREACH-002", "BREACH"},
		{"SEC-ALERT-001", "ALERT"},
		{"SEC-ALERT-002", "ALERT"},
		{"UNKNOWN", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.eventName, func(t *testing.T) {
			subCategory := GetEventSubCategory(tc.eventName)
			assert.Equal(t, tc.expectedSubCategory, subCategory)
		})
	}
}

// TestSECURITYEvents_GetEventCategory_Unknown 测试未知事件的类别
func TestSECURITYEvents_GetEventCategory_Unknown(t *testing.T) {
	assert.Equal(t, "", GetEventCategory("UNKNOWN-EVENT"))
	assert.Equal(t, "", GetEventCategory(""))
}

// TestSECURITYEvents_GetResultCode_Unknown 测试未知事件的结果码
func TestSECURITYEvents_GetResultCode_Unknown(t *testing.T) {
	code := GetResultCode("UNKNOWN-EVENT")
	assert.Equal(t, "", code)
}

// TestSECURITYEvents_GetEventDescription_Unknown 测试未知事件的描述
func TestSECURITYEvents_GetEventDescription_Unknown(t *testing.T) {
	desc := GetEventDescription("UNKNOWN-EVENT")
	assert.Equal(t, "", desc)
}

// TestSECURITYEvents_FormatSECURITYEvent 测试格式化SECURITY事件
func TestSECURITYEvents_FormatSECURITYEvent(t *testing.T) {
	// 测试有描述的事件
	desc := FormatSECURITYEvent("INV-PKG-001", nil)
	assert.Contains(t, desc, "供应方资质过期")

	// 测试带参数的事件
	descWithParams := FormatSECURITYEvent("INV-PKG-001", map[string]string{"key": "value"})
	assert.Contains(t, descWithParams, "供应方资质过期")

	// 测试未知事件
	descUnknown := FormatSECURITYEvent("UNKNOWN-EVENT", nil)
	assert.Contains(t, descUnknown, "SECURITY event")

	// 测试带参数但无描述的事件
	descUnknownWithParams := FormatSECURITYEvent("UNKNOWN-EVENT", map[string]string{"key": "value"})
	assert.Contains(t, descUnknownWithParams, "SECURITY event")
}

// TestSECURITYEvents_isSecurityAlert 测试安全告警检测
func TestSECURITYEvents_isSecurityAlert(t *testing.T) {
	// 这些函数是内部的，但我们可以通过间接方式测试
	// isSecurityAlert 通过 GetEventSubCategory("SEC-ALERT-xxx") = "ALERT" 来验证
	assert.Equal(t, "ALERT", GetEventSubCategory("SEC-ALERT-001"))
	assert.Equal(t, "ALERT", GetEventSubCategory("SEC-ALERT-002"))
}

// TestSECURITYEvents_isSecurityBreach 测试安全突破检测
func TestSECURITYEvents_isSecurityBreach(t *testing.T) {
	// 通过 GetEventSubCategory 验证
	assert.Equal(t, "BREACH", GetEventSubCategory("SEC-BREACH-001"))
	assert.Equal(t, "BREACH", GetEventSubCategory("SEC-BREACH-002"))
}

// TestSECURITYEvents_GetSECURITYEvents_Complete 测试所有SECURITY事件
func TestSECURITYEvents_GetSECURITYEvents_Complete(t *testing.T) {
	events := GetSECURITYEvents()

	// 验证所有SECURITY事件
	assert.Contains(t, events, "INV-PKG-001")
	assert.Contains(t, events, "INV-PKG-002")
	assert.Contains(t, events, "INV-PKG-003")
	assert.Contains(t, events, "INV-SET-001")
	assert.Contains(t, events, "INV-SET-002")
	assert.Contains(t, events, "INV-SET-003")
	assert.Contains(t, events, "SEC-BREACH-001")
	assert.Contains(t, events, "SEC-BREACH-002")
	assert.Contains(t, events, "SEC-ALERT-001")
	assert.Contains(t, events, "SEC-ALERT-002")

	// 验证总数
	assert.Len(t, events, 10)
}