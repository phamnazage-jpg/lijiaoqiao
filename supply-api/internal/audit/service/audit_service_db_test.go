package service

import (
	"context"
	"testing"

	"lijiaoqiao/supply-api/internal/audit/model"

	"github.com/stretchr/testify/assert"
)

// ==================== InMemoryAuditStore Additional Tests ====================

func TestInMemoryAuditStore_GetByEventID_Success(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAuditStore()

	event := &model.AuditEvent{
		EventID:   "test-001",
		EventName: "test.event",
		TenantID:  1001,
	}
	store.Emit(ctx, event)

	result, err := store.GetByEventID(ctx, "test-001")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-001", result.EventID)
}

func TestInMemoryAuditStore_GetByEventID_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAuditStore()

	result, err := store.GetByEventID(ctx, "non-existent")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, ErrEventNotFound, err)
}

func TestInMemoryAuditStore_CleanupOldEvents_ZeroCount(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAuditStore()

	// Add events
	for i := 0; i < 5; i++ {
		store.Emit(ctx, &model.AuditEvent{
			EventID:   "test-" + string(rune('0'+i)),
			EventName: "test.event",
			TenantID:  1001,
		})
	}

	// Test with zero count - should use default MaxEvents/10 = 10000
	// Since we have 5 events and 10000 >= 5, removeCount = 5-1 = 4
	// remaining = 5-4 = 1, but s.events = s.events[1:] keeps the last 4 events
	store.cleanupOldEvents(0)

	store.mu.RLock()
	defer store.mu.RUnlock()
	assert.Len(t, store.events, 4)
}

func TestInMemoryAuditStore_CleanupOldEvents_NegativeCount(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAuditStore()

	// Add events
	for i := 0; i < 5; i++ {
		store.Emit(ctx, &model.AuditEvent{
			EventID:   "test-" + string(rune('0'+i)),
			EventName: "test.event",
			TenantID:  1001,
		})
	}

	// Test with negative count - should use default MaxEvents/10 = 10000
	// Same logic as zero count - keeps 4 events
	store.cleanupOldEvents(-1)

	store.mu.RLock()
	defer store.mu.RUnlock()
	assert.Len(t, store.events, 4)
}

// ==================== AuditService Additional Tests ====================

func TestAuditService_GetEventByID_Success(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAuditStore()
	svc := NewAuditService(store)

	event := &model.AuditEvent{
		EventID:   "test-001",
		EventName: "test.event",
		TenantID:  1001,
	}
	svc.CreateEvent(ctx, event)

	result, err := svc.GetEventByID(ctx, "test-001")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-001", result.EventID)
}

func TestAuditService_GetEventByID_EmptyID(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAuditStore()
	svc := NewAuditService(store)

	result, err := svc.GetEventByID(ctx, "")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, ErrInvalidInput, err)
}

func TestAuditService_CreateEventsBatch_Empty(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAuditStore()
	svc := NewAuditService(store)

	result, err := svc.CreateEventsBatch(ctx, []*model.AuditEvent{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 0, result.FailCount)
}

func TestAuditService_CreateEventsBatch_Success(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAuditStore()
	svc := NewAuditService(store)

	events := []*model.AuditEvent{
		{EventID: "test-001", EventName: "event1", TenantID: 1001},
		{EventID: "test-002", EventName: "event2", TenantID: 1001},
	}

	result, err := svc.CreateEventsBatch(ctx, events)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, result.SuccessCount)
	assert.Equal(t, 0, result.FailCount)
}

func TestAuditService_ListEvents_WithFilter_AllFields(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAuditStore()
	svc := NewAuditService(store)

	// Create events
	for i := 0; i < 3; i++ {
		svc.CreateEvent(ctx, &model.AuditEvent{
			EventID:       "test-00" + string(rune('1'+i)),
			EventName:     "CRED-EXPOSE-RESPONSE",
			EventCategory: "CRED",
			OperatorID:    1001,
			TenantID:      2001,
			ObjectType:    "account",
			ObjectID:      int64(12345 + i),
			Action:        "create",
			CredentialType: "platform_token",
			SourceType:    "api",
			SourceIP:      "192.168.1.1",
			Success:       true,
			ResultCode:    "SEC_CRED_EXPOSED",
		})
	}

	filter := &EventFilter{
		TenantID:  2001,
		Category:  "CRED",
		EventName: "CRED-EXPOSE-RESPONSE",
		Limit:     10,
		Offset:    0,
	}

	events, total, err := svc.ListEventsWithFilter(ctx, filter)

	assert.NoError(t, err)
	assert.Len(t, events, 3)
	assert.Equal(t, int64(3), total)
}

// ==================== isSamePayload and compareExtensions Tests ====================

func TestIsSamePayload_AllFields(t *testing.T) {
	event1 := &model.AuditEvent{
		EventName:       "test.event",
		EventCategory:   "TEST",
		OperatorID:      1001,
		TenantID:        2001,
		ObjectType:      "account",
		ObjectID:        12345,
		Action:          "create",
		ActionDetail:    "detailed action",
		CredentialType:  "platform_token",
		SourceType:      "api",
		SourceIP:        "192.168.1.1",
		Success:         true,
		ResultCode:      "OK",
		ResultMessage:   "Success",
		Extensions:      map[string]any{"key": "value"},
	}

	event2 := &model.AuditEvent{
		EventName:       "test.event",
		EventCategory:   "TEST",
		OperatorID:      1001,
		TenantID:        2001,
		ObjectType:      "account",
		ObjectID:        12345,
		Action:          "create",
		ActionDetail:    "detailed action",
		CredentialType:  "platform_token",
		SourceType:      "api",
		SourceIP:        "192.168.1.1",
		Success:         true,
		ResultCode:      "OK",
		ResultMessage:   "Success",
		Extensions:      map[string]any{"key": "value"},
	}

	assert.True(t, isSamePayload(event1, event2))
}

func TestIsSamePayload_DifferentActionDetail(t *testing.T) {
	event1 := &model.AuditEvent{
		EventName:     "test.event",
		EventCategory: "TEST",
		OperatorID:    1001,
		TenantID:      2001,
		ObjectType:    "account",
		ObjectID:      12345,
		Action:        "create",
		ActionDetail:  "detail 1",
	}

	event2 := &model.AuditEvent{
		EventName:     "test.event",
		EventCategory: "TEST",
		OperatorID:    1001,
		TenantID:      2001,
		ObjectType:    "account",
		ObjectID:      12345,
		Action:        "create",
		ActionDetail:  "detail 2",
	}

	assert.False(t, isSamePayload(event1, event2))
}

func TestIsSamePayload_DifferentResultMessage(t *testing.T) {
	event1 := &model.AuditEvent{
		EventName:      "test.event",
		EventCategory:  "TEST",
		OperatorID:     1001,
		TenantID:       2001,
		ObjectType:     "account",
		ObjectID:       12345,
		Action:         "create",
		ResultMessage:  "message 1",
	}

	event2 := &model.AuditEvent{
		EventName:      "test.event",
		EventCategory:  "TEST",
		OperatorID:     1001,
		TenantID:       2001,
		ObjectType:     "account",
		ObjectID:       12345,
		Action:         "create",
		ResultMessage:  "message 2",
	}

	assert.False(t, isSamePayload(event1, event2))
}

func TestIsSamePayload_DifferentExtensions(t *testing.T) {
	event1 := &model.AuditEvent{
		EventName:    "test.event",
		EventCategory: "TEST",
		OperatorID:   1001,
		TenantID:     2001,
		ObjectType:   "account",
		ObjectID:     12345,
		Action:       "create",
		Extensions:   map[string]any{"key": "value1"},
	}

	event2 := &model.AuditEvent{
		EventName:    "test.event",
		EventCategory: "TEST",
		OperatorID:   1001,
		TenantID:     2001,
		ObjectType:   "account",
		ObjectID:     12345,
		Action:       "create",
		Extensions:   map[string]any{"key": "value2"},
	}

	assert.False(t, isSamePayload(event1, event2))
}

func TestIsSamePayload_DifferentExtensionKeys(t *testing.T) {
	event1 := &model.AuditEvent{
		EventName:    "test.event",
		EventCategory: "TEST",
		OperatorID:   1001,
		TenantID:     2001,
		ObjectType:   "account",
		ObjectID:     12345,
		Action:       "create",
		Extensions:   map[string]any{"key1": "value"},
	}

	event2 := &model.AuditEvent{
		EventName:    "test.event",
		EventCategory: "TEST",
		OperatorID:   1001,
		TenantID:     2001,
		ObjectType:   "account",
		ObjectID:     12345,
		Action:       "create",
		Extensions:   map[string]any{"key2": "value"},
	}

	assert.False(t, isSamePayload(event1, event2))
}

func TestIsSamePayload_NilExtensions(t *testing.T) {
	event1 := &model.AuditEvent{
		EventName:    "test.event",
		EventCategory: "TEST",
		OperatorID:   1001,
		TenantID:     2001,
		ObjectType:   "account",
		ObjectID:     12345,
		Action:       "create",
		Extensions:   nil,
	}

	event2 := &model.AuditEvent{
		EventName:    "test.event",
		EventCategory: "TEST",
		OperatorID:   1001,
		TenantID:     2001,
		ObjectType:   "account",
		ObjectID:     12345,
		Action:       "create",
		Extensions:   nil,
	}

	assert.True(t, isSamePayload(event1, event2))
}

func TestCompareExtensions_Equal(t *testing.T) {
	ext1 := map[string]any{"a": 1, "b": "test"}
	ext2 := map[string]any{"a": 1, "b": "test"}

	assert.True(t, compareExtensions(ext1, ext2))
}

func TestCompareExtensions_DifferentLengths(t *testing.T) {
	ext1 := map[string]any{"a": 1}
	ext2 := map[string]any{"a": 1, "b": 2}

	assert.False(t, compareExtensions(ext1, ext2))
}

func TestCompareExtensions_DifferentValues(t *testing.T) {
	ext1 := map[string]any{"a": 1, "b": 2}
	ext2 := map[string]any{"a": 1, "b": 3}

	assert.False(t, compareExtensions(ext1, ext2))
}

func TestCompareExtensions_NilMaps(t *testing.T) {
	assert.True(t, compareExtensions(nil, nil))
	assert.False(t, compareExtensions(nil, map[string]any{"a": 1}))
	assert.False(t, compareExtensions(map[string]any{"a": 1}, nil))
}

func TestCompareExtensions_MissingKey(t *testing.T) {
	ext1 := map[string]any{"a": 1, "b": 2}
	ext2 := map[string]any{"a": 1}

	assert.False(t, compareExtensions(ext1, ext2))
}

func TestCompareExtensions_EmptyMaps(t *testing.T) {
	assert.True(t, compareExtensions(map[string]any{}, map[string]any{}))
}

func TestCompareExtensions_IntValues(t *testing.T) {
	ext1 := map[string]any{"count": 42}
	ext2 := map[string]any{"count": 42}

	assert.True(t, compareExtensions(ext1, ext2))
}

func TestCompareExtensions_FloatValues(t *testing.T) {
	ext1 := map[string]any{"ratio": 3.14}
	ext2 := map[string]any{"ratio": 3.14}

	assert.True(t, compareExtensions(ext1, ext2))
}

func TestCompareExtensions_BoolValues(t *testing.T) {
	ext1 := map[string]any{"enabled": true}
	ext2 := map[string]any{"enabled": true}

	assert.True(t, compareExtensions(ext1, ext2))
}
