package events

import (
	"strings"
)

// CRED事件类别常量
const (
	CategoryCRED         = "CRED"
	SubCategoryEXPOSE    = "EXPOSE"
	SubCategoryINGRESS   = "INGRESS"
	SubCategoryROTATE    = "ROTATE"
	SubCategoryREVOKE    = "REVOKE"
	SubCategoryVALIDATE  = "VALIDATE"
	SubCategoryDIRECT    = "DIRECT"
)

// CRED事件列表
var credEvents = []string{
	// 凭证暴露事件 (CRED-EXPOSE)
	"CRED-EXPOSE-RESPONSE", // 响应中暴露凭证
	"CRED-EXPOSE-LOG",      // 日志中暴露凭证
	"CRED-EXPOSE-EXPORT",   // 导出文件中暴露凭证

	// 凭证入站事件 (CRED-INGRESS)
	"CRED-INGRESS-PLATFORM",  // 平台凭证入站
	"CRED-INGRESS-SUPPLIER", // 供应商凭证入站

	// 凭证轮换事件 (CRED-ROTATE)
	"CRED-ROTATE",

	// 凭证吊销事件 (CRED-REVOKE)
	"CRED-REVOKE",

	// 凭证验证事件 (CRED-VALIDATE)
	"CRED-VALIDATE",

	// 直连绕过事件 (CRED-DIRECT)
	"CRED-DIRECT-SUPPLIER", // 直连供应商
	"CRED-DIRECT-BYPASS",   // 绕过直连
}

// CRED事件结果码映射
var credResultCodes = map[string]string{
	"CRED-EXPOSE-RESPONSE": "SEC_CRED_EXPOSED",
	"CRED-EXPOSE-LOG":      "SEC_CRED_EXPOSED",
	"CRED-EXPOSE-EXPORT":   "SEC_CRED_EXPOSED",
	"CRED-INGRESS-PLATFORM": "CRED_INGRESS_OK",
	"CRED-INGRESS-SUPPLIER": "CRED_INGRESS_OK",
	"CRED-DIRECT-SUPPLIER":  "SEC_DIRECT_BYPASS",
	"CRED-DIRECT-BYPASS":    "SEC_DIRECT_BYPASS",
	"CRED-ROTATE":           "CRED_ROTATE_OK",
	"CRED-REVOKE":           "CRED_REVOKE_OK",
	"CRED-VALIDATE":         "CRED_VALIDATE_OK",
}

// CRED指标名称映射
var credMetricNames = map[string]string{
	"CRED-EXPOSE-RESPONSE":    "supplier_credential_exposure_events",
	"CRED-EXPOSE-LOG":         "supplier_credential_exposure_events",
	"CRED-EXPOSE-EXPORT":      "supplier_credential_exposure_events",
	"CRED-INGRESS-PLATFORM":   "platform_credential_ingress_coverage_pct",
	"CRED-INGRESS-SUPPLIER":   "platform_credential_ingress_coverage_pct",
	"CRED-DIRECT-SUPPLIER":    "direct_supplier_call_by_consumer_events",
	"CRED-DIRECT-BYPASS":      "direct_supplier_call_by_consumer_events",
}

// GetCREDEvents 返回所有CRED事件
func GetCREDEvents() []string {
	return credEvents
}

// GetCREDExposeEvents 返回所有凭证暴露事件
func GetCREDExposeEvents() []string {
	return []string{
		"CRED-EXPOSE-RESPONSE",
		"CRED-EXPOSE-LOG",
		"CRED-EXPOSE-EXPORT",
	}
}

// GetCREDFngressEvents 返回所有凭证入站事件
func GetCREDFngressEvents() []string {
	return []string{
		"CRED-INGRESS-PLATFORM",
		"CRED-INGRESS-SUPPLIER",
	}
}

// GetCREDDnirectEvents 返回所有直连绕过事件
func GetCREDDnirectEvents() []string {
	return []string{
		"CRED-DIRECT-SUPPLIER",
		"CRED-DIRECT-BYPASS",
	}
}

// GetCREDEventCategory 返回CRED事件的类别
func GetCREDEventCategory(eventName string) string {
	if strings.HasPrefix(eventName, "CRED-") {
		return CategoryCRED
	}
	if eventName == "CRED-ROTATE" || eventName == "CRED-REVOKE" || eventName == "CRED-VALIDATE" {
		return CategoryCRED
	}
	return ""
}

// GetCREDEventSubCategory 返回CRED事件的子类别
func GetCREDEventSubCategory(eventName string) string {
	if strings.HasPrefix(eventName, "CRED-EXPOSE") {
		return SubCategoryEXPOSE
	}
	if strings.HasPrefix(eventName, "CRED-INGRESS") {
		return SubCategoryINGRESS
	}
	if strings.HasPrefix(eventName, "CRED-DIRECT") {
		return SubCategoryDIRECT
	}
	if strings.HasPrefix(eventName, "CRED-ROTATE") {
		return SubCategoryROTATE
	}
	if strings.HasPrefix(eventName, "CRED-REVOKE") {
		return SubCategoryREVOKE
	}
	if strings.HasPrefix(eventName, "CRED-VALIDATE") {
		return SubCategoryVALIDATE
	}
	return ""
}

// IsValidCREDEvent 检查事件名称是否为有效的CRED事件
func IsValidCREDEvent(eventName string) bool {
	for _, e := range credEvents {
		if e == eventName {
			return true
		}
	}
	return false
}

// IsCREDExposeEvent 检查是否为凭证暴露事件（M-013相关）
func IsCREDExposeEvent(eventName string) bool {
	return strings.HasPrefix(eventName, "CRED-EXPOSE")
}

// IsCREDFngressEvent 检查是否为凭证入站事件（M-014相关）
func IsCREDFngressEvent(eventName string) bool {
	return strings.HasPrefix(eventName, "CRED-INGRESS")
}

// IsCREDDnirectEvent 检查是否为直连绕过事件（M-015相关）
func IsCREDDnirectEvent(eventName string) bool {
	return strings.HasPrefix(eventName, "CRED-DIRECT")
}

// GetCREDMetricName 获取CRED事件对应的指标名称
func GetCREDMetricName(eventName string) string {
	if metric, ok := credMetricNames[eventName]; ok {
		return metric
	}
	return ""
}

// GetCREDEventResultCode 获取CRED事件对应的结果码
func GetCREDEventResultCode(eventName string) string {
	if code, ok := credResultCodes[eventName]; ok {
		return code
	}
	return ""
}

// IsCREDExposeEvent 检查是否为M-013事件（凭证暴露）
func IsM013RelatedEvent(eventName string) bool {
	return IsCREDExposeEvent(eventName)
}

// IsCREDFngressEvent 检查是否为M-014事件（凭证入站）
func IsM014RelatedEvent(eventName string) bool {
	return IsCREDFngressEvent(eventName)
}

// IsCREDDnirectEvent 检查是否为M-015事件（直连绕过）
func IsM015RelatedEvent(eventName string) bool {
	return IsCREDDnirectEvent(eventName)
}