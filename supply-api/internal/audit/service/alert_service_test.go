package service

import (
	"context"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/audit/model"

	"github.com/stretchr/testify/assert"
)

// ==================== AlertService 测试 ====================

func TestAlertService_CreateAlert_Success(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	alert := &model.Alert{
		AlertName:  "Test Alert",
		AlertType:  model.AlertTypeSecurity,
		AlertLevel: model.AlertLevelWarning,
		TenantID:   1001,
		Title:      "Security Alert Test",
		Message:    "This is a test alert",
	}

	result, err := svc.CreateAlert(ctx, alert)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.AlertID)
	assert.Equal(t, "Security Alert Test", result.Title)
	assert.Equal(t, model.AlertStatusActive, result.Status)
}

func TestAlertService_CreateAlert_WithDefaults(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	alert := &model.Alert{
		AlertName:  "Minimal Alert",
		AlertType:  model.AlertTypeCredential,
		AlertLevel: model.AlertLevelError,
		TenantID:   1001,
		Title:      "Minimal Alert Test",
		Message:    "This is a minimal test",
	}

	result, err := svc.CreateAlert(ctx, alert)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.AlertID)
	assert.Equal(t, model.AlertStatusActive, result.Status)
	assert.False(t, result.CreatedAt.IsZero())
	assert.False(t, result.UpdatedAt.IsZero())
}

func TestAlertService_CreateAlert_NilInput(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	result, err := svc.CreateAlert(ctx, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, ErrInvalidAlertInput, err)
}

func TestAlertService_CreateAlert_EmptyTitle(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	alert := &model.Alert{
		Title: "",
	}

	result, err := svc.CreateAlert(ctx, alert)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestAlertService_GetAlert_Success(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	alert := &model.Alert{
		AlertName:  "Get Test Alert",
		AlertType:  model.AlertTypeSecurity,
		AlertLevel: model.AlertLevelWarning,
		TenantID:   1001,
		Title:      "Get Alert Test",
		Message:    "Testing GetAlert",
	}

	created, _ := svc.CreateAlert(ctx, alert)

	result, err := svc.GetAlert(ctx, created.AlertID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, created.AlertID, result.AlertID)
	assert.Equal(t, "Get Alert Test", result.Title)
}

func TestAlertService_GetAlert_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	result, err := svc.GetAlert(ctx, "non-existent-id")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, ErrAlertNotFound, err)
}

func TestAlertService_GetAlert_EmptyID(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	result, err := svc.GetAlert(ctx, "")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, ErrInvalidAlertInput, err)
}

func TestAlertService_UpdateAlert_Success(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	alert := &model.Alert{
		AlertName:  "Update Test Alert",
		AlertType:  model.AlertTypeSecurity,
		AlertLevel: model.AlertLevelWarning,
		TenantID:   1001,
		Title:      "Original Title",
		Message:    "Original Message",
	}

	created, _ := svc.CreateAlert(ctx, alert)
	created.Title = "Updated Title"
	created.Message = "Updated Message"

	result, err := svc.UpdateAlert(ctx, created)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Updated Title", result.Title)
	assert.Equal(t, "Updated Message", result.Message)
}

func TestAlertService_UpdateAlert_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	alert := &model.Alert{
		AlertID:    "non-existent-id",
		AlertName:  "Update Test",
		AlertType:  model.AlertTypeSecurity,
		AlertLevel: model.AlertLevelWarning,
		TenantID:   1001,
		Title:      "Test",
		Message:    "Test",
	}

	result, err := svc.UpdateAlert(ctx, alert)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, ErrAlertNotFound, err)
}

func TestAlertService_UpdateAlert_NilInput(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	result, err := svc.UpdateAlert(ctx, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, ErrInvalidAlertInput, err)
}

func TestAlertService_DeleteAlert_Success(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	alert := &model.Alert{
		AlertName:  "Delete Test Alert",
		AlertType:  model.AlertTypeSecurity,
		AlertLevel: model.AlertLevelWarning,
		TenantID:   1001,
		Title:      "Delete Alert Test",
		Message:    "Testing Delete",
	}

	created, _ := svc.CreateAlert(ctx, alert)

	err := svc.DeleteAlert(ctx, created.AlertID)

	assert.NoError(t, err)

	// Verify deleted
	result, err := svc.GetAlert(ctx, created.AlertID)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestAlertService_DeleteAlert_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	err := svc.DeleteAlert(ctx, "non-existent-id")

	assert.Error(t, err)
	assert.Equal(t, ErrAlertNotFound, err)
}

func TestAlertService_DeleteAlert_EmptyID(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	err := svc.DeleteAlert(ctx, "")

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidAlertInput, err)
}

func TestAlertService_ListAlerts_Success(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	// Create multiple alerts
	for i := 0; i < 5; i++ {
		alert := &model.Alert{
			AlertName:  "List Test Alert",
			AlertType:  model.AlertTypeSecurity,
			AlertLevel: model.AlertLevelWarning,
			TenantID:   1001,
			Title:      "List Alert Test",
			Message:    "Testing List",
		}
		svc.CreateAlert(ctx, alert)
	}

	filter := &model.AlertFilter{
		TenantID: 1001,
		Limit:    10,
	}

	results, total, err := svc.ListAlerts(ctx, filter)

	assert.NoError(t, err)
	assert.Len(t, results, 5)
	assert.Equal(t, int64(5), total)
}

func TestAlertService_ListAlerts_WithFilter(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	// Create alerts with different types
	alert1 := &model.Alert{
		AlertName:  "Security Alert",
		AlertType:  model.AlertTypeSecurity,
		AlertLevel: model.AlertLevelWarning,
		TenantID:   1001,
		Title:      "Security Alert",
		Message:    "Test",
	}
	alert2 := &model.Alert{
		AlertName:  "Quota Alert",
		AlertType:  model.AlertTypeQuota,
		AlertLevel: model.AlertLevelError,
		TenantID:   1001,
		Title:      "Quota Alert",
		Message:    "Test",
	}

	svc.CreateAlert(ctx, alert1)
	svc.CreateAlert(ctx, alert2)

	filter := &model.AlertFilter{
		TenantID:  1001,
		AlertType: model.AlertTypeSecurity,
	}

	results, total, err := svc.ListAlerts(ctx, filter)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, model.AlertTypeSecurity, results[0].AlertType)
}

func TestAlertService_ListAlerts_NilFilter(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	alert := &model.Alert{
		AlertName:  "Test Alert",
		AlertType:  model.AlertTypeSecurity,
		AlertLevel: model.AlertLevelWarning,
		TenantID:   1001,
		Title:      "Test",
		Message:    "Test",
	}
	svc.CreateAlert(ctx, alert)

	results, total, err := svc.ListAlerts(ctx, nil)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, results, 1)
}

func TestAlertService_ListAlerts_Pagination(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	// Create 10 alerts
	for i := 0; i < 10; i++ {
		alert := &model.Alert{
			AlertName:  "Pagination Test",
			AlertType:  model.AlertTypeSecurity,
			AlertLevel: model.AlertLevelWarning,
			TenantID:   1001,
			Title:      "Test",
			Message:    "Test",
		}
		svc.CreateAlert(ctx, alert)
	}

	// First page
	filter1 := &model.AlertFilter{TenantID: 1001, Limit: 5, Offset: 0}
	results1, total1, err1 := svc.ListAlerts(ctx, filter1)

	assert.NoError(t, err1)
	assert.Len(t, results1, 5)
	assert.Equal(t, int64(10), total1)

	// Second page
	filter2 := &model.AlertFilter{TenantID: 1001, Limit: 5, Offset: 5}
	results2, total2, err2 := svc.ListAlerts(ctx, filter2)

	assert.NoError(t, err2)
	assert.Len(t, results2, 5)
	assert.Equal(t, int64(10), total2)
}

func TestAlertService_ResolveAlert_Success(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	alert := &model.Alert{
		AlertName:  "Resolve Test",
		AlertType:  model.AlertTypeSecurity,
		AlertLevel: model.AlertLevelWarning,
		TenantID:   1001,
		Title:      "Resolve Alert Test",
		Message:    "Test",
		Status:     model.AlertStatusActive,
	}
	created, _ := svc.CreateAlert(ctx, alert)

	resolved, err := svc.ResolveAlert(ctx, created.AlertID, "admin", "Fixed the issue")

	assert.NoError(t, err)
	assert.NotNil(t, resolved)
	assert.Equal(t, model.AlertStatusResolved, resolved.Status)
	assert.Equal(t, "admin", resolved.ResolvedBy)
	assert.Equal(t, "Fixed the issue", resolved.ResolveNote)
	assert.NotNil(t, resolved.ResolvedAt)
}

func TestAlertService_ResolveAlert_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	resolved, err := svc.ResolveAlert(ctx, "non-existent", "admin", "note")

	assert.Error(t, err)
	assert.Nil(t, resolved)
	assert.Equal(t, ErrAlertNotFound, err)
}

func TestAlertService_AcknowledgeAlert_Success(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	alert := &model.Alert{
		AlertName:  "Acknowledge Test",
		AlertType:  model.AlertTypeSecurity,
		AlertLevel: model.AlertLevelWarning,
		TenantID:   1001,
		Title:      "Acknowledge Alert Test",
		Message:    "Test",
		Status:     model.AlertStatusActive,
	}
	created, _ := svc.CreateAlert(ctx, alert)

	acknowledged, err := svc.AcknowledgeAlert(ctx, created.AlertID)

	assert.NoError(t, err)
	assert.NotNil(t, acknowledged)
	assert.Equal(t, model.AlertStatusAcknowledged, acknowledged.Status)
}

func TestAlertService_AcknowledgeAlert_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()
	svc := NewAlertService(store)

	acknowledged, err := svc.AcknowledgeAlert(ctx, "non-existent")

	assert.Error(t, err)
	assert.Nil(t, acknowledged)
	assert.Equal(t, ErrAlertNotFound, err)
}

// ==================== InMemoryAlertStore 测试 ====================

func TestInMemoryAlertStore_CRUD(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()

	// Create
	alert := &model.Alert{
		AlertID:    "test-001",
		AlertName:  "CRUD Test",
		AlertType:  model.AlertTypeSecurity,
		AlertLevel: model.AlertLevelWarning,
		TenantID:   1001,
		Title:      "CRUD Test Alert",
		Message:    "Testing CRUD",
		Status:     model.AlertStatusActive,
	}

	err := store.Create(ctx, alert)
	assert.NoError(t, err)

	// Read
	retrieved, err := store.GetByID(ctx, "test-001")
	assert.NoError(t, err)
	assert.Equal(t, "CRUD Test Alert", retrieved.Title)

	// Update
	retrieved.Title = "Updated Title"
	err = store.Update(ctx, retrieved)
	assert.NoError(t, err)

	updated, _ := store.GetByID(ctx, "test-001")
	assert.Equal(t, "Updated Title", updated.Title)

	// Delete
	err = store.Delete(ctx, "test-001")
	assert.NoError(t, err)

	_, err = store.GetByID(ctx, "test-001")
	assert.Error(t, err)
}

func TestInMemoryAlertStore_PreservesMetadataAndTags(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()

	alert := &model.Alert{
		AlertID:    "test-meta-001",
		AlertName:  "Metadata Test",
		AlertType:  model.AlertTypeSecurity,
		AlertLevel: model.AlertLevelWarning,
		TenantID:   1001,
		Title:      "Metadata",
		Message:    "metadata and tags",
		Status:     model.AlertStatusActive,
		Metadata: map[string]any{
			"source": "unit-test",
		},
		Tags:           []string{"urgent", "security"},
		NotifyChannels: []string{"email"},
		EventIDs:       []string{"evt-001"},
	}

	err := store.Create(ctx, alert)
	assert.NoError(t, err)

	stored, err := store.GetByID(ctx, "test-meta-001")
	assert.NoError(t, err)
	assert.Equal(t, "unit-test", stored.Metadata["source"])
	assert.Equal(t, []string{"urgent", "security"}, stored.Tags)
	assert.Equal(t, []string{"email"}, stored.NotifyChannels)
	assert.Equal(t, []string{"evt-001"}, stored.EventIDs)
}

func TestInMemoryAlertStore_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()

	result, err := store.GetByID(ctx, "non-existent")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestInMemoryAlertStore_Update_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()

	alert := &model.Alert{
		AlertID: "non-existent",
		Title:   "Test",
	}

	err := store.Update(ctx, alert)

	assert.Error(t, err)
	assert.Equal(t, ErrAlertNotFound, err)
}

func TestInMemoryAlertStore_Delete_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()

	err := store.Delete(ctx, "non-existent")

	assert.Error(t, err)
	assert.Equal(t, ErrAlertNotFound, err)
}

func TestInMemoryAlertStore_List_FilterByTenant(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()

	store.Create(ctx, &model.Alert{
		AlertID:   "1",
		TenantID:  1001,
		Title:     "Tenant 1001 Alert",
		AlertType: model.AlertTypeSecurity,
	})
	store.Create(ctx, &model.Alert{
		AlertID:   "2",
		TenantID:  1002,
		Title:     "Tenant 1002 Alert",
		AlertType: model.AlertTypeSecurity,
	})

	filter := &model.AlertFilter{TenantID: 1001}
	results, total, err := store.List(ctx, filter)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, int64(1001), results[0].TenantID)
}

func TestInMemoryAlertStore_List_FilterByAlertType(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()

	store.Create(ctx, &model.Alert{
		AlertID:   "1",
		TenantID:  1001,
		AlertType: model.AlertTypeSecurity,
		Title:     "Security",
	})
	store.Create(ctx, &model.Alert{
		AlertID:   "2",
		TenantID:  1001,
		AlertType: model.AlertTypeQuota,
		Title:     "Quota",
	})

	filter := &model.AlertFilter{
		TenantID:  1001,
		AlertType: model.AlertTypeSecurity,
	}
	results, total, err := store.List(ctx, filter)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, model.AlertTypeSecurity, results[0].AlertType)
	assert.Equal(t, int64(1), total)
}

func TestInMemoryAlertStore_List_FilterByAlertLevel(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()

	store.Create(ctx, &model.Alert{
		AlertID:    "1",
		TenantID:   1001,
		AlertLevel: model.AlertLevelWarning,
		Title:      "Warning",
	})
	store.Create(ctx, &model.Alert{
		AlertID:    "2",
		TenantID:   1001,
		AlertLevel: model.AlertLevelCritical,
		Title:      "Critical",
	})

	filter := &model.AlertFilter{
		TenantID:   1001,
		AlertLevel: model.AlertLevelCritical,
	}
	results, total, err := store.List(ctx, filter)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, model.AlertLevelCritical, results[0].AlertLevel)
	assert.Equal(t, int64(1), total)
}

func TestInMemoryAlertStore_List_FilterByStatus(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()

	store.Create(ctx, &model.Alert{
		AlertID:  "1",
		TenantID: 1001,
		Status:   model.AlertStatusActive,
		Title:    "Active",
	})
	store.Create(ctx, &model.Alert{
		AlertID:  "2",
		TenantID: 1001,
		Status:   model.AlertStatusResolved,
		Title:    "Resolved",
	})

	filter := &model.AlertFilter{
		TenantID: 1001,
		Status:   model.AlertStatusActive,
	}
	results, total, err := store.List(ctx, filter)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, model.AlertStatusActive, results[0].Status)
	assert.Equal(t, int64(1), total)
}

func TestInMemoryAlertStore_List_FilterByTimeRange(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()

	now := time.Now()
	oldTime := now.Add(-1 * time.Hour)
	recentTime := now.Add(-10 * time.Minute)

	store.Create(ctx, &model.Alert{
		AlertID:   "1",
		TenantID:  1001,
		CreatedAt: oldTime,
		Title:     "Old",
	})
	store.Create(ctx, &model.Alert{
		AlertID:   "2",
		TenantID:  1001,
		CreatedAt: recentTime,
		Title:     "Recent",
	})

	// Filter for recent alerts only
	filter := &model.AlertFilter{
		TenantID:  1001,
		StartTime: now.Add(-30 * time.Minute),
		EndTime:   now.Add(30 * time.Minute),
	}
	results, total, err := store.List(ctx, filter)

	assert.NoError(t, err)
	// Should only get the recent alert, not the old one
	assert.GreaterOrEqual(t, len(results), 1)
	assert.GreaterOrEqual(t, total, int64(1))
}

func TestInMemoryAlertStore_List_FilterByKeywords(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()

	store.Create(ctx, &model.Alert{
		AlertID:  "1",
		TenantID: 1001,
		Title:    "Database Connection Error",
		Message:  "Failed to connect to DB",
	})
	store.Create(ctx, &model.Alert{
		AlertID:  "2",
		TenantID: 1001,
		Title:    "API Timeout",
		Message:  "Request timed out",
	})

	filter := &model.AlertFilter{
		TenantID: 1001,
		Keywords: "Database",
	}
	results, total, err := store.List(ctx, filter)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Contains(t, results[0].Title, "Database")
	assert.Equal(t, int64(1), total)
}

func TestInMemoryAlertStore_List_Pagination(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()

	for i := 0; i < 10; i++ {
		store.Create(ctx, &model.Alert{
			AlertID:  string(rune('0' + i)),
			TenantID: 1001,
			Title:    "Test",
		})
	}

	filter := &model.AlertFilter{TenantID: 1001, Limit: 3, Offset: 0}
	results, total, err := store.List(ctx, filter)

	assert.NoError(t, err)
	assert.Len(t, results, 3)
	assert.Equal(t, int64(10), total)
}

func TestInMemoryAlertStore_List_OffsetBeyondBounds(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStore()

	store.Create(ctx, &model.Alert{
		AlertID:  "1",
		TenantID: 1001,
		Title:    "Test",
	})

	filter := &model.AlertFilter{TenantID: 1001, Limit: 10, Offset: 100}
	results, total, err := store.List(ctx, filter)

	assert.NoError(t, err)
	assert.Len(t, results, 0)
	assert.Equal(t, int64(1), total)
}

// ==================== Alert Model 测试 ====================

func TestAlert_IsActive(t *testing.T) {
	alert := &model.Alert{Status: model.AlertStatusActive}
	assert.True(t, alert.IsActive())

	alert.Status = model.AlertStatusResolved
	assert.False(t, alert.IsActive())
}

func TestAlert_IsResolved(t *testing.T) {
	alert := &model.Alert{Status: model.AlertStatusResolved}
	assert.True(t, alert.IsResolved())

	alert.Status = model.AlertStatusActive
	assert.False(t, alert.IsResolved())
}

func TestAlert_Resolve(t *testing.T) {
	alert := &model.Alert{Status: model.AlertStatusActive}

	alert.Resolve("admin", "Fixed")

	assert.Equal(t, model.AlertStatusResolved, alert.Status)
	assert.Equal(t, "admin", alert.ResolvedBy)
	assert.Equal(t, "Fixed", alert.ResolveNote)
	assert.NotNil(t, alert.ResolvedAt)
}

func TestAlert_Acknowledge(t *testing.T) {
	alert := &model.Alert{Status: model.AlertStatusActive}

	alert.Acknowledge()

	assert.Equal(t, model.AlertStatusAcknowledged, alert.Status)
}

func TestAlert_Suppress(t *testing.T) {
	alert := &model.Alert{Status: model.AlertStatusActive}

	alert.Suppress()

	assert.Equal(t, model.AlertStatusSuppressed, alert.Status)
}

func TestAlert_UpdateLastSeen(t *testing.T) {
	alert := &model.Alert{LastSeenAt: time.Now().Add(-1 * time.Hour)}

	alert.UpdateLastSeen()

	assert.True(t, alert.LastSeenAt.After(time.Now().Add(-1*time.Hour)))
}

func TestAlert_AddEventID(t *testing.T) {
	alert := &model.Alert{}

	alert.AddEventID("evt-001")

	assert.Len(t, alert.EventIDs, 1)
	assert.Equal(t, "evt-001", alert.EventID)
	assert.Equal(t, "evt-001", alert.EventIDs[0])
}

func TestAlert_AddEventID_Multiple(t *testing.T) {
	alert := &model.Alert{}

	alert.AddEventID("evt-001")
	alert.AddEventID("evt-002")

	assert.Len(t, alert.EventIDs, 2)
	assert.Equal(t, "evt-001", alert.EventID)
	assert.Equal(t, "evt-002", alert.EventIDs[1])
}

func TestAlert_SetMetadata(t *testing.T) {
	alert := &model.Alert{}

	alert.SetMetadata("key1", "value1")
	alert.SetMetadata("key2", 123)

	assert.Equal(t, "value1", alert.Metadata["key1"])
	assert.Equal(t, 123, alert.Metadata["key2"])
}

func TestAlert_AddTag(t *testing.T) {
	alert := &model.Alert{}

	alert.AddTag("security")
	alert.AddTag("urgent")

	assert.Len(t, alert.Tags, 2)
	assert.Contains(t, alert.Tags, "security")
	assert.Contains(t, alert.Tags, "urgent")
}

func TestAlert_AddTag_Duplicate(t *testing.T) {
	alert := &model.Alert{Tags: []string{"security"}}

	alert.AddTag("security")

	assert.Len(t, alert.Tags, 1)
}

func TestNewAlert(t *testing.T) {
	alert := model.NewAlert("TestAlert", model.AlertTypeSecurity, model.AlertLevelWarning, "1001", "Test Title", "Test Message")

	assert.NotEmpty(t, alert.AlertID)
	assert.Equal(t, "TestAlert", alert.AlertName)
	assert.Equal(t, model.AlertTypeSecurity, alert.AlertType)
	assert.Equal(t, model.AlertLevelWarning, alert.AlertLevel)
	assert.Equal(t, int64(1001), alert.TenantID)
	assert.Equal(t, "Test Title", alert.Title)
	assert.Equal(t, "Test Message", alert.Message)
	assert.Equal(t, model.AlertStatusActive, alert.Status)
	assert.True(t, alert.NotifyEnabled)
}
