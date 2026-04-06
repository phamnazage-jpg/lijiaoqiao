package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/audit/model"
	"lijiaoqiao/supply-api/internal/audit/service"

	"github.com/stretchr/testify/assert"
)

// mockAlertStore 模拟告警存储
type mockAlertStore struct {
	alerts map[string]*model.Alert
}

func newMockAlertStore() *mockAlertStore {
	return &mockAlertStore{
		alerts: make(map[string]*model.Alert),
	}
}

func (m *mockAlertStore) Create(ctx context.Context, alert *model.Alert) error {
	if alert.AlertID == "" {
		alert.AlertID = "test-alert-id"
	}
	alert.CreatedAt = testTime
	alert.UpdatedAt = testTime
	m.alerts[alert.AlertID] = alert
	return nil
}

func (m *mockAlertStore) GetByID(ctx context.Context, alertID string) (*model.Alert, error) {
	if alert, ok := m.alerts[alertID]; ok {
		return alert, nil
	}
	return nil, service.ErrAlertNotFound
}

func (m *mockAlertStore) Update(ctx context.Context, alert *model.Alert) error {
	if _, ok := m.alerts[alert.AlertID]; !ok {
		return service.ErrAlertNotFound
	}
	alert.UpdatedAt = testTime
	m.alerts[alert.AlertID] = alert
	return nil
}

func (m *mockAlertStore) Delete(ctx context.Context, alertID string) error {
	if _, ok := m.alerts[alertID]; !ok {
		return service.ErrAlertNotFound
	}
	delete(m.alerts, alertID)
	return nil
}

func (m *mockAlertStore) List(ctx context.Context, filter *model.AlertFilter) ([]*model.Alert, int64, error) {
	var result []*model.Alert
	for _, alert := range m.alerts {
		if filter.TenantID > 0 && alert.TenantID != filter.TenantID {
			continue
		}
		if filter.Status != "" && alert.Status != filter.Status {
			continue
		}
		result = append(result, alert)
	}
	return result, int64(len(result)), nil
}

var testTime = time.Now()

// TestAlertHandler_CreateAlert_Success 测试创建告警成功
func TestAlertHandler_CreateAlert_Success(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	reqBody := CreateAlertRequest{
		AlertName:  "TEST_ALERT",
		AlertType:  "security",
		AlertLevel: "warning",
		TenantID:   2001,
		Title:      "Test Alert Title",
		Message:    "Test alert message",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/audit/alerts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateAlert(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var result AlertResponse
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, "Test Alert Title", result.Alert.Title)
	assert.Equal(t, "security", result.Alert.AlertType)
}

// TestAlertHandler_CreateAlert_MissingTitle 测试缺少标题
func TestAlertHandler_CreateAlert_MissingTitle(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	reqBody := CreateAlertRequest{
		AlertType:  "security",
		AlertLevel: "warning",
		TenantID:   2001,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/audit/alerts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateAlert(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAlertHandler_GetAlert_Success 测试获取告警成功
func TestAlertHandler_GetAlert_Success(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	// 先创建一个告警
	alert := &model.Alert{
		AlertID:    "test-alert-123",
		AlertName:  "TEST_ALERT",
		AlertType:  "security",
		AlertLevel: "warning",
		TenantID:   2001,
		Title:      "Test Alert",
		Message:    "Test message",
	}
	store.Create(context.Background(), alert)

	// 获取告警
	req := httptest.NewRequest("GET", "/api/v1/audit/alerts/test-alert-123", nil)
	w := httptest.NewRecorder()

	h.GetAlert(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result AlertResponse
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, "test-alert-123", result.Alert.AlertID)
}

// TestAlertHandler_GetAlert_NotFound 测试告警不存在
func TestAlertHandler_GetAlert_NotFound(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	req := httptest.NewRequest("GET", "/api/v1/audit/alerts/nonexistent", nil)
	w := httptest.NewRecorder()

	h.GetAlert(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestAlertHandler_ListAlerts_Success 测试列出告警成功
func TestAlertHandler_ListAlerts_Success(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	// 创建多个告警
	for i := 0; i < 3; i++ {
		alert := &model.Alert{
			AlertID:    "alert-" + string(rune('a'+i)),
			AlertType:  "security",
			AlertLevel: "warning",
			TenantID:   2001,
			Title:      "Test Alert",
		}
		store.Create(context.Background(), alert)
	}

	req := httptest.NewRequest("GET", "/api/v1/audit/alerts?tenant_id=2001", nil)
	w := httptest.NewRecorder()

	h.ListAlerts(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result AlertListResponse
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), result.Total)
}

// TestAlertHandler_UpdateAlert_Success 测试更新告警成功
func TestAlertHandler_UpdateAlert_Success(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	// 先创建一个告警
	alert := &model.Alert{
		AlertID:    "test-alert-123",
		AlertType:  "security",
		AlertLevel: "warning",
		TenantID:   2001,
		Title:      "Original Title",
	}
	store.Create(context.Background(), alert)

	// 更新告警
	reqBody := UpdateAlertRequest{
		Title: "Updated Title",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/audit/alerts/test-alert-123", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateAlert(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result AlertResponse
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, "Updated Title", result.Alert.Title)
}

// TestAlertHandler_DeleteAlert_Success 测试删除告警成功
func TestAlertHandler_DeleteAlert_Success(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	// 先创建一个告警
	alert := &model.Alert{
		AlertID:    "test-alert-123",
		AlertType:  "security",
		AlertLevel: "warning",
		TenantID:   2001,
	}
	store.Create(context.Background(), alert)

	// 删除告警
	req := httptest.NewRequest("DELETE", "/api/v1/audit/alerts/test-alert-123", nil)
	w := httptest.NewRecorder()

	h.DeleteAlert(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestAlertHandler_DeleteAlert_NotFound 测试删除不存在的告警
func TestAlertHandler_DeleteAlert_NotFound(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	req := httptest.NewRequest("DELETE", "/api/v1/audit/alerts/nonexistent", nil)
	w := httptest.NewRecorder()

	h.DeleteAlert(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestAlertHandler_ResolveAlert_Success 测试解决告警成功
func TestAlertHandler_ResolveAlert_Success(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	// 先创建一个告警
	alert := &model.Alert{
		AlertID:    "test-alert-123",
		AlertType:  "security",
		AlertLevel: "warning",
		TenantID:   2001,
		Status:     model.AlertStatusActive,
	}
	store.Create(context.Background(), alert)

	// 解决告警
	reqBody := ResolveAlertRequest{
		ResolvedBy: "admin",
		Note:       "Fixed the issue",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/audit/alerts/test-alert-123/resolve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ResolveAlert(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result AlertResponse
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, model.AlertStatusResolved, result.Alert.Status)
	assert.Equal(t, "admin", result.Alert.ResolvedBy)
}
