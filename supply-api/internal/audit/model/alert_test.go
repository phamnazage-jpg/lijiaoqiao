package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewAlert 测试创建告警
func TestNewAlert(t *testing.T) {
	alert := NewAlert("test-alert", AlertTypeSecurity, AlertLevelWarning, "12345", "Test Title", "Test Message")

	assert.NotEmpty(t, alert.AlertID)
	assert.Equal(t, "test-alert", alert.AlertName)
	assert.Equal(t, AlertTypeSecurity, alert.AlertType)
	assert.Equal(t, AlertLevelWarning, alert.AlertLevel)
	assert.Equal(t, int64(12345), alert.TenantID)
	assert.Equal(t, "Test Title", alert.Title)
	assert.Equal(t, "Test Message", alert.Message)
	assert.Equal(t, AlertStatusActive, alert.Status)
	assert.True(t, alert.NotifyEnabled)
	assert.NotZero(t, alert.CreatedAt)
	assert.NotNil(t, alert.Metadata)
	assert.NotNil(t, alert.Tags)
}

// TestParseTenantID 测试租户ID解析
func TestParseTenantID(t *testing.T) {
	testCases := []struct {
		input    string
		expected int64
	}{
		{"12345", 12345},
		{"0", 0},
		{"", 0},
		{"abc", 0},
		{"123abc456", 123456},
		{"000", 0},
		{"00123", 123},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := parseTenantID(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestAlert_IsActive 测试告警活跃状态检测
func TestAlert_IsActive(t *testing.T) {
	alert := &Alert{Status: AlertStatusActive}
	assert.True(t, alert.IsActive())

	alert.Status = AlertStatusResolved
	assert.False(t, alert.IsActive())

	alert.Status = AlertStatusAcknowledged
	assert.False(t, alert.IsActive())

	alert.Status = AlertStatusSuppressed
	assert.False(t, alert.IsActive())
}

// TestAlert_IsResolved 测试告警已解决状态检测
func TestAlert_IsResolved(t *testing.T) {
	alert := &Alert{Status: AlertStatusResolved}
	assert.True(t, alert.IsResolved())

	alert.Status = AlertStatusActive
	assert.False(t, alert.IsResolved())
}

// TestAlert_Resolve 测试解决告警
func TestAlert_Resolve(t *testing.T) {
	alert := &Alert{
		Status:     AlertStatusActive,
		UpdatedAt:  time.Now().Add(-time.Hour),
	}

	alert.Resolve("user123", "Fixed the issue")

	assert.Equal(t, AlertStatusResolved, alert.Status)
	assert.NotNil(t, alert.ResolvedAt)
	assert.Equal(t, "user123", alert.ResolvedBy)
	assert.Equal(t, "Fixed the issue", alert.ResolveNote)
	assert.True(t, alert.UpdatedAt.After(time.Now().Add(-time.Minute)))
}

// TestAlert_Acknowledge 测试确认告警
func TestAlert_Acknowledge(t *testing.T) {
	alert := &Alert{Status: AlertStatusActive}
	originalTime := alert.UpdatedAt

	alert.Acknowledge()

	assert.Equal(t, AlertStatusAcknowledged, alert.Status)
	assert.True(t, alert.UpdatedAt.After(originalTime) || alert.UpdatedAt.Equal(originalTime))
}

// TestAlert_Suppress 测试抑制告警
func TestAlert_Suppress(t *testing.T) {
	alert := &Alert{Status: AlertStatusActive}
	originalTime := alert.UpdatedAt

	alert.Suppress()

	assert.Equal(t, AlertStatusSuppressed, alert.Status)
	assert.True(t, alert.UpdatedAt.After(originalTime) || alert.UpdatedAt.Equal(originalTime))
}

// TestAlert_UpdateLastSeen 测试更新最后出现时间
func TestAlert_UpdateLastSeen(t *testing.T) {
	oldTime := time.Now().Add(-time.Hour)
	alert := &Alert{LastSeenAt: oldTime, UpdatedAt: oldTime}

	alert.UpdateLastSeen()

	assert.True(t, alert.LastSeenAt.After(oldTime))
	assert.True(t, alert.UpdatedAt.After(oldTime))
}

// TestAlert_AddEventID 测试添加关联事件ID
func TestAlert_AddEventID(t *testing.T) {
	alert := &Alert{}

	// First event ID should set both EventID and EventIDs[0]
	alert.AddEventID("evt-001")
	assert.Equal(t, "evt-001", alert.EventID)
	assert.Len(t, alert.EventIDs, 1)
	assert.Equal(t, "evt-001", alert.EventIDs[0])

	// Second event ID should only add to EventIDs
	alert.AddEventID("evt-002")
	assert.Equal(t, "evt-001", alert.EventID) // First one preserved
	assert.Len(t, alert.EventIDs, 2)
	assert.Equal(t, "evt-002", alert.EventIDs[1])
}

// TestAlert_SetMetadata 测试设置元数据
func TestAlert_SetMetadata(t *testing.T) {
	alert := &Alert{}

	// Set metadata when nil
	alert.SetMetadata("key1", "value1")
	assert.Equal(t, "value1", alert.Metadata["key1"])

	// Set another key
	alert.SetMetadata("key2", 123)
	assert.Equal(t, "value1", alert.Metadata["key1"])
	assert.Equal(t, 123, alert.Metadata["key2"])

	// Overwrite existing key
	alert.SetMetadata("key1", "newvalue")
	assert.Equal(t, "newvalue", alert.Metadata["key1"])
}

// TestAlert_AddTag 测试添加标签
func TestAlert_AddTag(t *testing.T) {
	alert := &Alert{}

	// Add first tag
	alert.AddTag("security")
	assert.Contains(t, alert.Tags, "security")

	// Add another tag
	alert.AddTag("urgent")
	assert.Contains(t, alert.Tags, "security")
	assert.Contains(t, alert.Tags, "urgent")

	// Add duplicate tag - should not add again
	alert.AddTag("security")
	assert.Equal(t, 2, len(alert.Tags))
}

// TestAlert_AddTag_NoDuplicate 测试添加重复标签
func TestAlert_AddTag_NoDuplicate(t *testing.T) {
	alert := &Alert{Tags: []string{"existing"}}

	alert.AddTag("existing")
	assert.Len(t, alert.Tags, 1)
}

// TestAlert_Constants 测试告警常量
func TestAlert_Constants(t *testing.T) {
	// Alert levels
	assert.Equal(t, "info", AlertLevelInfo)
	assert.Equal(t, "warning", AlertLevelWarning)
	assert.Equal(t, "error", AlertLevelError)
	assert.Equal(t, "critical", AlertLevelCritical)

	// Alert statuses
	assert.Equal(t, "active", AlertStatusActive)
	assert.Equal(t, "resolved", AlertStatusResolved)
	assert.Equal(t, "acknowledged", AlertStatusAcknowledged)
	assert.Equal(t, "suppressed", AlertStatusSuppressed)

	// Alert types
	assert.Equal(t, "security", AlertTypeSecurity)
	assert.Equal(t, "invariant", AlertTypeInvariant)
	assert.Equal(t, "credential", AlertTypeCredential)
	assert.Equal(t, "authentication", AlertTypeAuthentication)
	assert.Equal(t, "authorization", AlertTypeAuthorization)
	assert.Equal(t, "quota", AlertTypeQuota)
}

// TestGenerateAlertID 测试告警ID生成
func TestGenerateAlertID(t *testing.T) {
	id1 := generateAlertID()
	id2 := generateAlertID()

	// Each ID should be unique
	assert.NotEqual(t, id1, id2)

	// ID should start with ALT-
	assert.Contains(t, id1, "ALT-")
	assert.Contains(t, id2, "ALT-")
}
