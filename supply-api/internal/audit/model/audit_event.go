package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// 事件类别常量
const (
	CategoryCRED    = "CRED"
	CategoryAUTH   = "AUTH"
	CategoryDATA   = "DATA"
	CategoryCONFIG = "CONFIG"
	CategorySECURITY = "SECURITY"
)

// 凭证事件子类别
const (
	SubCategoryCredExpose   = "EXPOSE"
	SubCategoryCredIngress  = "INGRESS"
	SubCategoryCredRotate   = "ROTATE"
	SubCategoryCredRevoke   = "REVOKE"
	SubCategoryCredValidate = "VALIDATE"
	SubCategoryCredDirect   = "DIRECT"
)

// 凭证类型
const (
	CredentialTypePlatformToken  = "platform_token"
	CredentialTypeQueryKey       = "query_key"
	CredentialTypeUpstreamAPIKey = "upstream_api_key"
	CredentialTypeNone           = "none"
)

// 操作者类型
const (
	OperatorTypeUser   = "user"
	OperatorTypeSystem = "system"
	OperatorTypeAdmin  = "admin"
)

// 租户类型
const (
	TenantTypeSupplier = "supplier"
	TenantTypeConsumer = "consumer"
	TenantTypePlatform = "platform"
)

// SecurityFlags 安全标记
type SecurityFlags struct {
	HasCredential      bool     `json:"has_credential"`        // 是否包含凭证
	CredentialExposed  bool     `json:"credential_exposed"`    // 凭证是否暴露
	Desensitized       bool     `json:"desensitized"`          // 是否已脱敏
	Scanned            bool     `json:"scanned"`              // 是否已扫描
	ScanPassed         bool     `json:"scan_passed"`          // 扫描是否通过
	ViolationTypes     []string `json:"violation_types"`      // 违规类型列表
}

// NewSecurityFlags 创建默认安全标记
func NewSecurityFlags() *SecurityFlags {
	return &SecurityFlags{
		HasCredential:      false,
		CredentialExposed:  false,
		Desensitized:       false,
		Scanned:            false,
		ScanPassed:         false,
		ViolationTypes:     []string{},
	}
}

// HasViolation 检查是否有违规
func (sf *SecurityFlags) HasViolation() bool {
	return len(sf.ViolationTypes) > 0
}

// HasViolationOfType 检查是否有指定类型的违规
func (sf *SecurityFlags) HasViolationOfType(violationType string) bool {
	for _, v := range sf.ViolationTypes {
		if v == violationType {
			return true
		}
	}
	return false
}

// AddViolationType 添加违规类型
func (sf *SecurityFlags) AddViolationType(violationType string) {
	sf.ViolationTypes = append(sf.ViolationTypes, violationType)
}

// AuditEvent 统一审计事件
type AuditEvent struct {
	// 基础标识
	EventID       string `json:"event_id"`                // 事件唯一ID (UUID)
	EventName     string `json:"event_name"`              // 事件名称 (e.g., "CRED-EXPOSE")
	EventCategory string `json:"event_category"`          // 事件大类 (e.g., "CRED")
	EventSubCategory string `json:"event_sub_category"`    // 事件子类

	// 时间戳
	Timestamp    time.Time `json:"timestamp"`              // 事件发生时间
	TimestampMs  int64     `json:"timestamp_ms"`           // 毫秒时间戳

	// 请求上下文
	RequestID    string `json:"request_id"`              // 请求追踪ID
	TraceID      string `json:"trace_id"`               // 分布式追踪ID
	SpanID       string `json:"span_id"`               // Span ID

	// 幂等性
	IdempotencyKey string `json:"idempotency_key,omitempty"` // 幂等键

	// 操作者信息
	OperatorID   int64  `json:"operator_id"`            // 操作者ID
	OperatorType string `json:"operator_type"`          // 操作者类型 (user/system/admin)
	OperatorRole string `json:"operator_role"`          // 操作者角色

	// 租户信息
	TenantID   int64  `json:"tenant_id"`              // 租户ID
	TenantType string `json:"tenant_type"`            // 租户类型 (supplier/consumer/platform)

	// 对象信息
	ObjectType string `json:"object_type"`             // 对象类型 (account/package/settlement)
	ObjectID   int64  `json:"object_id"`              // 对象ID

	// 操作信息
	Action       string `json:"action"`                 // 操作类型 (create/update/delete)
	ActionDetail string `json:"action_detail"`          // 操作详情

	// 凭证信息 (M-013/M-014/M-015/M-016 关键)
	CredentialType       string `json:"credential_type"`         // 凭证类型 (platform_token/query_key/upstream_api_key/none)
	CredentialID         string `json:"credential_id,omitempty"`  // 凭证标识 (脱敏)
	CredentialFingerprint string `json:"credential_fingerprint,omitempty"` // 凭证指纹

	// 来源信息
	SourceType   string `json:"source_type"`            // 来源类型 (api/ui/cron/internal)
	SourceIP     string `json:"source_ip"`              // 来源IP
	SourceRegion string `json:"source_region"`          // 来源区域
	UserAgent    string `json:"user_agent,omitempty"`  // User Agent

	// 目标信息 (用于直连检测 M-015)
	TargetType     string `json:"target_type,omitempty"`    // 目标类型
	TargetEndpoint string `json:"target_endpoint,omitempty"` // 目标端点
	TargetDirect   bool   `json:"target_direct"`             // 是否直连

	// 结果信息
	ResultCode    string `json:"result_code"`            // 结果码
	ResultMessage string `json:"result_message,omitempty"` // 结果消息
	Success       bool   `json:"success"`               // 是否成功

	// 状态变更 (用于溯源)
	BeforeState map[string]any `json:"before_state,omitempty"` // 操作前状态
	AfterState  map[string]any `json:"after_state,omitempty"`  // 操作后状态

	// 安全标记 (M-013 关键)
	SecurityFlags SecurityFlags `json:"security_flags"`      // 安全标记
	RiskScore     int           `json:"risk_score"`          // 风险评分 0-100

	// 合规信息
	ComplianceTags []string `json:"compliance_tags,omitempty"` // 合规标签 (e.g., ["GDPR", "SOC2"])
	InvariantRule  string   `json:"invariant_rule,omitempty"` // 触发的不变量规则

	// 扩展字段
	Extensions map[string]any `json:"extensions,omitempty"` // 扩展数据

	// 元数据
	Version  int       `json:"version"`  // 事件版本
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// NewAuditEvent 创建审计事件
func NewAuditEvent(
	eventName string,
	eventCategory string,
	eventSubCategory string,
	metricName string,
	requestID string,
	traceID string,
	operatorID int64,
	operatorType string,
	operatorRole string,
	tenantID int64,
	tenantType string,
	objectType string,
	objectID int64,
	action string,
	credentialType string,
	sourceType string,
	sourceIP string,
	success bool,
	resultCode string,
	resultMessage string,
) *AuditEvent {
	now := time.Now()
	event := &AuditEvent{
		EventID:         uuid.New().String(),
		EventName:       eventName,
		EventCategory:   eventCategory,
		EventSubCategory: eventSubCategory,
		Timestamp:       now,
		TimestampMs:     now.UnixMilli(),
		RequestID:       requestID,
		TraceID:         traceID,
		OperatorID:      operatorID,
		OperatorType:    operatorType,
		OperatorRole:    operatorRole,
		TenantID:        tenantID,
		TenantType:      tenantType,
		ObjectType:      objectType,
		ObjectID:        objectID,
		Action:          action,
		CredentialType:  credentialType,
		SourceType:      sourceType,
		SourceIP:        sourceIP,
		Success:         success,
		ResultCode:      resultCode,
		ResultMessage:   resultMessage,
		Version:         1,
		CreatedAt:       now,
		SecurityFlags:   *NewSecurityFlags(),
		ComplianceTags:  []string{},
	}

	// 根据凭证类型设置安全标记
	if credentialType != CredentialTypeNone && credentialType != "" {
		event.SecurityFlags.HasCredential = true
	}

	// 根据事件名称设置凭证暴露标记（M-013）
	if IsM013Event(eventName) {
		event.SecurityFlags.CredentialExposed = true
	}

	// 根据事件名称设置指标名称到扩展字段
	if metricName != "" {
		if event.Extensions == nil {
			event.Extensions = make(map[string]any)
		}
		event.Extensions["metric_name"] = metricName
	}

	return event
}

// NewAuditEventWithSecurityFlags 创建带完整安全标记的审计事件
func NewAuditEventWithSecurityFlags(
	eventName string,
	eventCategory string,
	eventSubCategory string,
	metricName string,
	requestID string,
	traceID string,
	operatorID int64,
	operatorType string,
	operatorRole string,
	tenantID int64,
	tenantType string,
	objectType string,
	objectID int64,
	action string,
	credentialType string,
	sourceType string,
	sourceIP string,
	success bool,
	resultCode string,
	resultMessage string,
	securityFlags SecurityFlags,
	riskScore int,
) *AuditEvent {
	event := NewAuditEvent(
		eventName,
		eventCategory,
		eventSubCategory,
		metricName,
		requestID,
		traceID,
		operatorID,
		operatorType,
		operatorRole,
		tenantID,
		tenantType,
		objectType,
		objectID,
		action,
		credentialType,
		sourceType,
		sourceIP,
		success,
		resultCode,
		resultMessage,
	)
	event.SecurityFlags = securityFlags
	event.RiskScore = riskScore
	return event
}

// SetIdempotencyKey 设置幂等键
func (e *AuditEvent) SetIdempotencyKey(key string) {
	e.IdempotencyKey = key
}

// SetTarget 设置目标信息（用于M-015直连检测）
func (e *AuditEvent) SetTarget(targetType, targetEndpoint string, targetDirect bool) {
	e.TargetType = targetType
	e.TargetEndpoint = targetEndpoint
	e.TargetDirect = targetDirect
}

// SetInvariantRule 设置不变量规则（用于SECURITY事件）
func (e *AuditEvent) SetInvariantRule(rule string) {
	e.InvariantRule = rule
	// 添加合规标签
	e.ComplianceTags = append(e.ComplianceTags, "XR-001")
}

// GetMetricName 获取指标名称
func (e *AuditEvent) GetMetricName() string {
	if e.Extensions != nil {
		if metricName, ok := e.Extensions["metric_name"].(string); ok {
			return metricName
		}
	}

	// 根据事件名称推断指标
	switch e.EventName {
	case "CRED-EXPOSE-RESPONSE", "CRED-EXPOSE-LOG", "CRED-EXPOSE":
		return "supplier_credential_exposure_events"
	case "CRED-INGRESS-PLATFORM", "CRED-INGRESS":
		return "platform_credential_ingress_coverage_pct"
	case "CRED-DIRECT-SUPPLIER", "CRED-DIRECT":
		return "direct_supplier_call_by_consumer_events"
	case "token.query_key.rejected", "token.query_key":
		return "query_key_external_reject_rate_pct"
	default:
		return ""
	}
}

// IsM013Event 判断是否为M-013凭证暴露事件
func IsM013Event(eventName string) bool {
	return strings.HasPrefix(eventName, "CRED-EXPOSE")
}

// IsM014Event 判断是否为M-014凭证入站事件
func IsM014Event(eventName string) bool {
	return strings.HasPrefix(eventName, "CRED-INGRESS")
}

// IsM014EventByCategory 判断是否为M-014凭证入站事件（基于分类字段）
// 设计文档要求：event_category='CRED' AND event_sub_category='INGRESS'
func IsM014EventByCategory(e *AuditEvent) bool {
	return e.EventCategory == CategoryCRED && e.EventSubCategory == SubCategoryCredIngress
}

// IsM015Event 判断是否为M-015直连绕过事件
func IsM015Event(eventName string) bool {
	return strings.HasPrefix(eventName, "CRED-DIRECT")
}

// IsM015EventByTargetDirect 判断是否为M-015直连绕过事件（基于target_direct字段）
// 设计文档要求：target_direct = TRUE
func IsM015EventByTargetDirect(e *AuditEvent) bool {
	return e.TargetDirect
}

// IsM016Event 判断是否为M-016 query key相关事件
// 统一事件格式: token.query_key (请求), token.query_key.rejected (拒绝)
func IsM016Event(eventName string) bool {
	return strings.HasPrefix(eventName, "token.query_key")
}

// IsM016QueryKeyEvent 判断是否为M-016 query key请求事件（仅KEY，不含REJECT）
// M-016分母定义：所有 query key 请求（不含 rejected）
func IsM016QueryKeyEvent(eventName string) bool {
	return eventName == "token.query_key"
}

// IsM016QueryKeyRejectEvent 判断是否为M-016 query key拒绝事件
func IsM016QueryKeyRejectEvent(eventName string) bool {
	return eventName == "token.query_key.rejected"
}