package model

import (
	"time"
)

// ==================== M-013: 凭证暴露事件详情 ====================

// CredentialExposureDetail M-013: 凭证暴露事件专用
type CredentialExposureDetail struct {
	EventID           string     `json:"event_id"`            // 事件ID（关联audit_events）
	ExposureType      string     `json:"exposure_type"`       // exposed_in_response/exposed_in_log/exposed_in_export
	ExposureLocation  string     `json:"exposure_location"`   // response_body/response_header/log_file/export_file
	ExposurePattern   string     `json:"exposure_pattern"`     // 匹配到的正则模式
	ExposedFragment   string     `json:"exposed_fragment"`    // 暴露的片段（已脱敏）
	ScanRuleID        string     `json:"scan_rule_id"`        // 触发扫描规则ID
	Resolved          bool       `json:"resolved"`            // 是否已解决
	ResolvedAt        *time.Time `json:"resolved_at"`         // 解决时间
	ResolvedBy        *int64     `json:"resolved_by"`        // 解决人
	ResolutionNotes   string     `json:"resolution_notes"`    // 解决备注
}

// NewCredentialExposureDetail 创建凭证暴露详情
func NewCredentialExposureDetail(
	exposureType string,
	exposureLocation string,
	exposurePattern string,
	exposedFragment string,
	scanRuleID string,
) *CredentialExposureDetail {
	return &CredentialExposureDetail{
		ExposureType:     exposureType,
		ExposureLocation: exposureLocation,
		ExposurePattern:  exposurePattern,
		ExposedFragment:  exposedFragment,
		ScanRuleID:       scanRuleID,
		Resolved:         false,
	}
}

// Resolve 标记为已解决
func (d *CredentialExposureDetail) Resolve(resolvedBy int64, notes string) {
	now := time.Now()
	d.Resolved = true
	d.ResolvedAt = &now
	d.ResolvedBy = &resolvedBy
	d.ResolutionNotes = notes
}

// ==================== M-014: 凭证入站事件详情 ====================

// CredentialIngressDetail M-014: 凭证入站类型专用
type CredentialIngressDetail struct {
	EventID                string     `json:"event_id"`                  // 事件ID
	RequestCredentialType  string     `json:"request_credential_type"`   // 请求中的凭证类型
	ExpectedCredentialType string     `json:"expected_credential_type"`  // 期望的凭证类型
	CoverageCompliant      bool       `json:"coverage_compliant"`       // 是否合规
	PlatformTokenPresent   bool       `json:"platform_token_present"`   // 平台Token是否存在
	UpstreamKeyPresent    bool       `json:"upstream_key_present"`     // 上游Key是否存在
	Reviewed               bool       `json:"reviewed"`                 // 是否已审核
	ReviewedAt             *time.Time `json:"reviewed_at"`              // 审核时间
	ReviewedBy             *int64     `json:"reviewed_by"`             // 审核人
}

// NewCredentialIngressDetail 创建凭证入站详情
func NewCredentialIngressDetail(
	requestCredentialType string,
	expectedCredentialType string,
	coverageCompliant bool,
	platformTokenPresent bool,
	upstreamKeyPresent bool,
) *CredentialIngressDetail {
	return &CredentialIngressDetail{
		RequestCredentialType:  requestCredentialType,
		ExpectedCredentialType: expectedCredentialType,
		CoverageCompliant:      coverageCompliant,
		PlatformTokenPresent:   platformTokenPresent,
		UpstreamKeyPresent:     upstreamKeyPresent,
		Reviewed:               false,
	}
}

// Review 标记为已审核
func (d *CredentialIngressDetail) Review(reviewedBy int64) {
	now := time.Now()
	d.Reviewed = true
	d.ReviewedAt = &now
	d.ReviewedBy = &reviewedBy
}

// ==================== M-015: 直连绕过事件详情 ====================

// DirectCallDetail M-015: 直连绕过专用
type DirectCallDetail struct {
	EventID         string     `json:"event_id"`          // 事件ID
	ConsumerID     int64      `json:"consumer_id"`       // 消费者ID
	SupplierID      int64      `json:"supplier_id"`      // 供应商ID
	DirectEndpoint  string     `json:"direct_endpoint"`   // 直连端点
	ViaPlatform     bool       `json:"via_platform"`      // 是否通过平台
	BypassType      string     `json:"bypass_type"`      // ip_bypass/proxy_bypass/config_bypass/dns_bypass
	DetectionMethod string     `json:"detection_method"`  // 检测方法
	Blocked         bool       `json:"blocked"`          // 是否被阻断
	BlockedAt       *time.Time `json:"blocked_at"`       // 阻断时间
	BlockReason     string     `json:"block_reason"`     // 阻断原因
}

// NewDirectCallDetail 创建直连详情
func NewDirectCallDetail(
	consumerID int64,
	supplierID int64,
	directEndpoint string,
	viaPlatform bool,
	bypassType string,
	detectionMethod string,
) *DirectCallDetail {
	return &DirectCallDetail{
		ConsumerID:     consumerID,
		SupplierID:     supplierID,
		DirectEndpoint: directEndpoint,
		ViaPlatform:    viaPlatform,
		BypassType:    bypassType,
		DetectionMethod: detectionMethod,
		Blocked:        false,
	}
}

// Block 标记为已阻断
func (d *DirectCallDetail) Block(reason string) {
	now := time.Now()
	d.Blocked = true
	d.BlockedAt = &now
	d.BlockReason = reason
}

// ==================== M-016: Query Key 拒绝事件详情 ====================

// QueryKeyRejectDetail M-016: query key 拒绝专用
type QueryKeyRejectDetail struct {
	EventID            string `json:"event_id"`             // 事件ID
	QueryKeyID        string `json:"query_key_id"`         // Query Key ID
	RequestedEndpoint string `json:"requested_endpoint"`   // 请求端点
	RejectReason      string `json:"reject_reason"`        // not_allowed/expired/malformed/revoked/rate_limited
	RejectCode        string `json:"reject_code"`          // 拒绝码
	FirstOccurrence   bool   `json:"first_occurrence"`     // 是否首次发生
	OccurrenceCount   int    `json:"occurrence_count"`    // 发生次数
}

// NewQueryKeyRejectDetail 创建Query Key拒绝详情
func NewQueryKeyRejectDetail(
	queryKeyID string,
	requestedEndpoint string,
	rejectReason string,
	rejectCode string,
) *QueryKeyRejectDetail {
	return &QueryKeyRejectDetail{
		QueryKeyID:        queryKeyID,
		RequestedEndpoint: requestedEndpoint,
		RejectReason:      rejectReason,
		RejectCode:        rejectCode,
		FirstOccurrence:   true,
		OccurrenceCount:   1,
	}
}

// RecordOccurrence 记录再次发生
func (d *QueryKeyRejectDetail) RecordOccurrence(firstOccurrence bool) {
	d.FirstOccurrence = firstOccurrence
	d.OccurrenceCount++
}

// ==================== 指标常量 ====================

// M-013 暴露类型常量
const (
	ExposureTypeResponse = "exposed_in_response"
	ExposureTypeLog      = "exposed_in_log"
	ExposureTypeExport   = "exposed_in_export"
)

// M-013 暴露位置常量
const (
	ExposureLocationResponseBody   = "response_body"
	ExposureLocationResponseHeader = "response_header"
	ExposureLocationLogFile        = "log_file"
	ExposureLocationExportFile     = "export_file"
)

// M-015 绕过类型常量
const (
	BypassTypeIPBypass      = "ip_bypass"
	BypassTypeProxyBypass   = "proxy_bypass"
	BypassTypeConfigBypass  = "config_bypass"
	BypassTypeDNSBypass     = "dns_bypass"
)

// M-015 检测方法常量
const (
	DetectionMethodUpstreamAPIPattern = "upstream_api_pattern_match"
	DetectionMethodDNSResolution      = "dns_resolution_check"
	DetectionMethodConnectionSource    = "connection_source_check"
	DetectionMethodIPWhitelist        = "ip_whitelist_check"
)

// M-016 拒绝原因常量
const (
	RejectReasonNotAllowed = "not_allowed"
	RejectReasonExpired     = "expired"
	RejectReasonMalformed   = "malformed"
	RejectReasonRevoked     = "revoked"
	RejectReasonRateLimited = "rate_limited"
)

// M-016 拒绝码常量
const (
	RejectCodeNotAllowed   = "QUERY_KEY_NOT_ALLOWED"
	RejectCodeExpired      = "QUERY_KEY_EXPIRED"
	RejectCodeMalformed    = "QUERY_KEY_MALFORMED"
	RejectCodeRevoked       = "QUERY_KEY_REVOKED"
	RejectCodeRateLimited  = "QUERY_KEY_RATE_LIMITED"
)