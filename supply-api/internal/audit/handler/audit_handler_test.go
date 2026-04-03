package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"lijiaoqiao/supply-api/internal/audit/model"
	"lijiaoqiao/supply-api/internal/audit/service"

	"github.com/stretchr/testify/assert"
)

// mockAuditStore 模拟审计存储
type mockAuditStore struct {
	events          []*model.AuditEvent
	nextID          int64
	idempotencyKeys map[string]*model.AuditEvent
}

func newMockAuditStore() *mockAuditStore {
	return &mockAuditStore{
		events:          make([]*model.AuditEvent, 0),
		nextID:          1,
		idempotencyKeys: make(map[string]*model.AuditEvent),
	}
}

func (m *mockAuditStore) Emit(ctx context.Context, event *model.AuditEvent) error {
	if event.EventID == "" {
		event.EventID = "test-event-id"
	}
	m.events = append(m.events, event)
	if event.IdempotencyKey != "" {
		m.idempotencyKeys[event.IdempotencyKey] = event
	}
	return nil
}

func (m *mockAuditStore) Query(ctx context.Context, filter *service.EventFilter) ([]*model.AuditEvent, int64, error) {
	var result []*model.AuditEvent
	for _, e := range m.events {
		if filter.TenantID != 0 && e.TenantID != filter.TenantID {
			continue
		}
		if filter.Category != "" && e.EventCategory != filter.Category {
			continue
		}
		result = append(result, e)
	}
	return result, int64(len(result)), nil
}

func (m *mockAuditStore) GetByIdempotencyKey(ctx context.Context, key string) (*model.AuditEvent, error) {
	if e, ok := m.idempotencyKeys[key]; ok {
		return e, nil
	}
	return nil, nil
}

// TestAuditHandler_CreateEvent_Success 测试创建事件成功
func TestAuditHandler_CreateEvent_Success(t *testing.T) {
	store := newMockAuditStore()
	svc := service.NewAuditService(store)
	h := NewAuditHandler(svc)

	reqBody := CreateEventRequest{
		EventName:        "CRED-EXPOSE-RESPONSE",
		EventCategory:    "CRED",
		EventSubCategory: "EXPOSE",
		OperatorID:       1001,
		TenantID:         2001,
		ObjectType:       "account",
		ObjectID:         12345,
		Action:          "query",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/audit/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateEvent(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var result service.CreateEventResult
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, 201, result.StatusCode)
	assert.Equal(t, "created", result.Status)
}

// TestAuditHandler_CreateEvent_DuplicateIdempotencyKey 测试幂等键重复
func TestAuditHandler_CreateEvent_DuplicateIdempotencyKey(t *testing.T) {
	store := newMockAuditStore()
	svc := service.NewAuditService(store)
	h := NewAuditHandler(svc)

	reqBody := CreateEventRequest{
		EventName:        "CRED-EXPOSE-RESPONSE",
		EventCategory:    "CRED",
		EventSubCategory: "EXPOSE",
		OperatorID:       1001,
		TenantID:         2001,
		IdempotencyKey:  "test-idempotency-key",
	}

	body, _ := json.Marshal(reqBody)

	// 第一次请求
	req1 := httptest.NewRequest("POST", "/audit/events", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	h.CreateEvent(w1, req1)
	assert.Equal(t, http.StatusCreated, w1.Code)

	// 第二次请求（相同幂等键）
	req2 := httptest.NewRequest("POST", "/audit/events", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	h.CreateEvent(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code) // 应该返回200而非201
}

// TestAuditHandler_ListEvents_Success 测试查询事件成功
func TestAuditHandler_ListEvents_Success(t *testing.T) {
	store := newMockAuditStore()
	svc := service.NewAuditService(store)
	h := NewAuditHandler(svc)

	// 先创建一些事件
	events := []*model.AuditEvent{
		{EventName: "EVENT-1", TenantID: 2001, EventCategory: "CRED"},
		{EventName: "EVENT-2", TenantID: 2001, EventCategory: "CRED"},
		{EventName: "EVENT-3", TenantID: 2002, EventCategory: "AUTH"},
	}
	for _, e := range events {
		store.Emit(context.Background(), e)
	}

	// 查询
	req := httptest.NewRequest("GET", "/audit/events?tenant_id=2001", nil)
	w := httptest.NewRecorder()

	h.ListEvents(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result ListEventsResponse
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), result.Total) // 只有2个2001租户的事件
}

// TestAuditHandler_ListEvents_WithPagination 测试分页查询
func TestAuditHandler_ListEvents_WithPagination(t *testing.T) {
	store := newMockAuditStore()
	svc := service.NewAuditService(store)
	h := NewAuditHandler(svc)

	// 创建多个事件
	for i := 0; i < 5; i++ {
		store.Emit(context.Background(), &model.AuditEvent{
			EventName: "EVENT",
			TenantID:  2001,
		})
	}

	req := httptest.NewRequest("GET", "/audit/events?tenant_id=2001&offset=0&limit=2", nil)
	w := httptest.NewRecorder()

	h.ListEvents(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result ListEventsResponse
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, int64(5), result.Total)
	assert.Equal(t, 0, result.Offset)
	assert.Equal(t, 2, result.Limit)
}

// TestAuditHandler_InvalidRequest 测试无效请求
func TestAuditHandler_InvalidRequest(t *testing.T) {
	store := newMockAuditStore()
	svc := service.NewAuditService(store)
	h := NewAuditHandler(svc)

	req := httptest.NewRequest("POST", "/audit/events", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateEvent(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAuditHandler_MissingRequiredFields 测试缺少必填字段
func TestAuditHandler_MissingRequiredFields(t *testing.T) {
	store := newMockAuditStore()
	svc := service.NewAuditService(store)
	h := NewAuditHandler(svc)

	// 缺少EventName
	reqBody := CreateEventRequest{
		EventCategory: "CRED",
		OperatorID:    1001,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/audit/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateEvent(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
