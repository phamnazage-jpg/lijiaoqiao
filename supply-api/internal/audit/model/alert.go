package model

import (
	"time"

	"github.com/google/uuid"
)

// 告警级别常量
const (
	AlertLevelInfo     = "info"
	AlertLevelWarning  = "warning"
	AlertLevelError    = "error"
	AlertLevelCritical = "critical"
)

// 告警状态常量
const (
	AlertStatusActive    = "active"
	AlertStatusResolved  = "resolved"
	AlertStatusAcknowledged = "acknowledged"
	AlertStatusSuppressed = "suppressed"
)

// 告警类型常量
const (
	AlertTypeSecurity        = "security"
	AlertTypeInvariant       = "invariant"
	AlertTypeCredential      = "credential"
	AlertTypeAuthentication  = "authentication"
	AlertTypeAuthorization   = "authorization"
	AlertTypeQuota           = "quota"
)

// Alert 告警
type Alert struct {
	// 基础标识
	AlertID     string `json:"alert_id"`              // 告警唯一ID
	AlertName   string `json:"alert_name"`            // 告警名称
	AlertType   string `json:"alert_type"`            // 告警类型 (security/invariant/credential/etc.)
	AlertLevel  string `json:"alert_level"`           // 告警级别 (info/warning/error/critical)
	TenantID    int64  `json:"tenant_id"`              // 租户ID
	SupplierID  int64  `json:"supplier_id,omitempty"` // 供应商ID（可选）

	// 告警内容
	Title       string `json:"title"`                 // 告警标题
	Message     string `json:"message"`               // 告警消息
	Description string `json:"description,omitempty"`  // 详细描述

	// 关联事件
	EventID     string `json:"event_id,omitempty"`    // 关联的事件ID
	EventIDs    []string `json:"event_ids,omitempty"` // 关联的事件ID列表（多个）

	// 触发条件
	TriggerCondition string `json:"trigger_condition,omitempty"` // 触发条件
	Threshold        float64 `json:"threshold,omitempty"`      // 阈值
	CurrentValue     float64 `json:"current_value,omitempty"`   // 当前值

	// 状态
	Status     string `json:"status"`                // 状态 (active/resolved/acknowledged/suppressed)
	ResolvedAt *time.Time `json:"resolved_at,omitempty"` // 解决时间
	ResolvedBy string `json:"resolved_by,omitempty"`  // 解决人
	ResolveNote string `json:"resolve_note,omitempty"` // 解决备注

	// 通知
	NotifyEnabled bool     `json:"notify_enabled"`     // 是否启用通知
	NotifyChannels []string `json:"notify_channels,omitempty"` // 通知渠道 (email/sms/webhook/etc.)

	// 时间戳
	CreatedAt   time.Time `json:"created_at"`         // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`         // 更新时间
	FirstSeenAt time.Time `json:"first_seen_at"`      // 首次出现时间
	LastSeenAt  time.Time `json:"last_seen_at"`       // 最后出现时间

	// 元数据
	Metadata map[string]any `json:"metadata,omitempty"` // 扩展元数据
	Tags     []string       `json:"tags,omitempty"`     // 标签
}

// NewAlert 创建新告警
func NewAlert(alertName, alertType, alertLevel, tenantID string, title, message string) *Alert {
	now := time.Now()
	return &Alert{
		AlertID:        generateAlertID(),
		AlertName:      alertName,
		AlertType:      alertType,
		AlertLevel:     alertLevel,
		TenantID:       parseTenantID(tenantID),
		Title:          title,
		Message:        message,
		Status:         AlertStatusActive,
		NotifyEnabled:  true,
		CreatedAt:      now,
		UpdatedAt:      now,
		FirstSeenAt:    now,
		LastSeenAt:     now,
		Metadata:       make(map[string]any),
		Tags:           []string{},
	}
}

// generateAlertID 生成告警ID
func generateAlertID() string {
	return "ALT-" + uuid.New().String()[:8]
}

// parseTenantID 解析租户ID
func parseTenantID(tenantID string) int64 {
	var id int64
	for _, c := range tenantID {
		if c >= '0' && c <= '9' {
			id = id*10 + int64(c-'0')
		}
	}
	return id
}

// IsActive 检查告警是否处于活跃状态
func (a *Alert) IsActive() bool {
	return a.Status == AlertStatusActive
}

// IsResolved 检查告警是否已解决
func (a *Alert) IsResolved() bool {
	return a.Status == AlertStatusResolved
}

// Resolve 解决告警
func (a *Alert) Resolve(resolvedBy, note string) {
	now := time.Now()
	a.Status = AlertStatusResolved
	a.ResolvedAt = &now
	a.ResolvedBy = resolvedBy
	a.ResolveNote = note
	a.UpdatedAt = now
}

// Acknowledge 确认告警
func (a *Alert) Acknowledge() {
	a.Status = AlertStatusAcknowledged
	a.UpdatedAt = time.Now()
}

// Suppress 抑制告警
func (a *Alert) Suppress() {
	a.Status = AlertStatusSuppressed
	a.UpdatedAt = time.Now()
}

// UpdateLastSeen 更新最后出现时间
func (a *Alert) UpdateLastSeen() {
	a.LastSeenAt = time.Now()
	a.UpdatedAt = time.Now()
}

// AddEventID 添加关联事件ID
func (a *Alert) AddEventID(eventID string) {
	a.EventIDs = append(a.EventIDs, eventID)
	if a.EventID == "" {
		a.EventID = eventID
	}
	a.UpdateLastSeen()
}

// SetMetadata 设置元数据
func (a *Alert) SetMetadata(key string, value any) {
	if a.Metadata == nil {
		a.Metadata = make(map[string]any)
	}
	a.Metadata[key] = value
}

// AddTag 添加标签
func (a *Alert) AddTag(tag string) {
	for _, t := range a.Tags {
		if t == tag {
			return
		}
	}
	a.Tags = append(a.Tags, tag)
}

// AlertFilter 告警查询过滤器
type AlertFilter struct {
	TenantID    int64
	SupplierID  int64
	AlertType   string
	AlertLevel  string
	Status      string
	StartTime   time.Time
	EndTime     time.Time
	Keywords    string // 关键字搜索（标题/消息）
	Limit       int
	Offset      int
}
