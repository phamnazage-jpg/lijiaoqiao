package events

import (
	"fmt"
)

// SECURITY事件类别常量
const (
	CategorySECURITY = "SECURITY"
	SubCategoryVIOLATION = "VIOLATION"
	SubCategoryALERT = "ALERT"
	SubCategoryBREACH = "BREACH"
)

// SECURITY事件列表
var securityEvents = []string{
	// 不变量违反事件 (INVARIANT-VIOLATION)
	"INV-PKG-001", // 供应方资质过期
	"INV-PKG-002", // 供应方余额为负
	"INV-PKG-003", // 售价不得低于保护价
	"INV-SET-001", // processing/completed 不可撤销
	"INV-SET-002", // 提现金额不得超过可提现余额
	"INV-SET-003", // 结算单金额与余额流水必须平衡

	// 安全突破事件 (SECURITY-BREACH)
	"SEC-BREACH-001", // 凭证泄露突破
	"SEC-BREACH-002", // 权限绕过突破

	// 安全告警事件 (SECURITY-ALERT)
	"SEC-ALERT-001", // 可疑访问告警
	"SEC-ALERT-002", // 异常行为告警
}

// 不变量违反事件到结果码的映射
var invariantResultCodes = map[string]string{
	"INV-PKG-001": "SEC_INV_PKG_001",
	"INV-PKG-002": "SEC_INV_PKG_002",
	"INV-PKG-003": "SEC_INV_PKG_003",
	"INV-SET-001": "SEC_INV_SET_001",
	"INV-SET-002": "SEC_INV_SET_002",
	"INV-SET-003": "SEC_INV_SET_003",
}

// 事件描述映射
var securityEventDescriptions = map[string]string{
	"INV-PKG-001": "供应方资质过期，资质验证失败",
	"INV-PKG-002": "供应方余额为负，余额检查失败",
	"INV-PKG-003": "售价不得低于保护价，价格校验失败",
	"INV-SET-001": "结算单状态为processing/completed，不可撤销",
	"INV-SET-002": "提现金额不得超过可提现余额",
	"INV-SET-003": "结算单金额与余额流水不平衡",
	"SEC-BREACH-001": "检测到凭证泄露安全突破",
	"SEC-BREACH-002": "检测到权限绕过安全突破",
	"SEC-ALERT-001": "检测到可疑访问行为",
	"SEC-ALERT-002": "检测到异常行为",
}

// GetSECURITYEvents 返回所有SECURITY事件
func GetSECURITYEvents() []string {
	return securityEvents
}

// GetInvariantViolationEvents 返回所有不变量违反事件
func GetInvariantViolationEvents() []string {
	return []string{
		"INV-PKG-001",
		"INV-PKG-002",
		"INV-PKG-003",
		"INV-SET-001",
		"INV-SET-002",
		"INV-SET-003",
	}
}

// GetSecurityAlertEvents 返回所有安全告警事件
func GetSecurityAlertEvents() []string {
	return []string{
		"SEC-ALERT-001",
		"SEC-ALERT-002",
	}
}

// GetSecurityBreachEvents 返回所有安全突破事件
func GetSecurityBreachEvents() []string {
	return []string{
		"SEC-BREACH-001",
		"SEC-BREACH-002",
	}
}

// GetEventCategory 返回事件的类别
func GetEventCategory(eventName string) string {
	if isInvariantViolation(eventName) || isSecurityBreach(eventName) || isSecurityAlert(eventName) {
		return CategorySECURITY
	}
	return ""
}

// GetEventSubCategory 返回事件的子类别
func GetEventSubCategory(eventName string) string {
	if isInvariantViolation(eventName) {
		return SubCategoryVIOLATION
	}
	if isSecurityBreach(eventName) {
		return SubCategoryBREACH
	}
	if isSecurityAlert(eventName) {
		return SubCategoryALERT
	}
	return ""
}

// GetResultCode 返回事件对应的结果码
func GetResultCode(eventName string) string {
	if code, ok := invariantResultCodes[eventName]; ok {
		return code
	}
	return ""
}

// GetEventDescription 返回事件的描述
func GetEventDescription(eventName string) string {
	if desc, ok := securityEventDescriptions[eventName]; ok {
		return desc
	}
	return ""
}

// IsValidEvent 检查事件名称是否有效
func IsValidEvent(eventName string) bool {
	for _, e := range securityEvents {
		if e == eventName {
			return true
		}
	}
	return false
}

// isInvariantViolation 检查是否为不变量违反事件
func isInvariantViolation(eventName string) bool {
	for _, e := range getInvariantViolationEvents() {
		if e == eventName {
			return true
		}
	}
	return false
}

// getInvariantViolationEvents 返回不变量违反事件列表（内部使用）
func getInvariantViolationEvents() []string {
	return []string{
		"INV-PKG-001",
		"INV-PKG-002",
		"INV-PKG-003",
		"INV-SET-001",
		"INV-SET-002",
		"INV-SET-003",
	}
}

// isSecurityBreach 检查是否为安全突破事件
func isSecurityBreach(eventName string) bool {
	prefixes := []string{"SEC-BREACH"}
	for _, prefix := range prefixes {
		if len(eventName) >= len(prefix) && eventName[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// isSecurityAlert 检查是否为安全告警事件
func isSecurityAlert(eventName string) bool {
	prefixes := []string{"SEC-ALERT"}
	for _, prefix := range prefixes {
		if len(eventName) >= len(prefix) && eventName[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// FormatSECURITYEvent 格式化SECURITY事件
func FormatSECURITYEvent(eventName string, params map[string]string) string {
	desc := GetEventDescription(eventName)
	if desc == "" {
		return fmt.Sprintf("SECURITY event: %s", eventName)
	}

	// 如果有额外参数，追加到描述中
	if len(params) > 0 {
		return fmt.Sprintf("%s - %v", desc, params)
	}
	return desc
}