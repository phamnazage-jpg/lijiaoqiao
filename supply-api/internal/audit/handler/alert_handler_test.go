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

// TestAlertHandler_CreateAlert_InvalidJSON 测试无效JSON
func TestAlertHandler_CreateAlert_InvalidJSON(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	req := httptest.NewRequest("POST", "/api/v1/audit/alerts", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateAlert(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAlertHandler_UpdateAlert_InvalidJSON 测试更新无效JSON
func TestAlertHandler_UpdateAlert_InvalidJSON(t *testing.T) {
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

	req := httptest.NewRequest("PUT", "/api/v1/audit/alerts/test-alert-123", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateAlert(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAlertHandler_UpdateAlert_NotFound 测试更新不存在的告警
func TestAlertHandler_UpdateAlert_NotFound(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	reqBody := UpdateAlertRequest{Title: "Updated"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/audit/alerts/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateAlert(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestAlertHandler_GetAlert_MissingID 测试缺少告警ID
func TestAlertHandler_GetAlert_MissingID(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	req := httptest.NewRequest("GET", "/api/v1/audit/alerts/", nil)
	w := httptest.NewRecorder()

	h.GetAlert(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAlertHandler_DeleteAlert_MissingID 测试缺少告警ID
func TestAlertHandler_DeleteAlert_MissingID(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	req := httptest.NewRequest("DELETE", "/api/v1/audit/alerts/", nil)
	w := httptest.NewRecorder()

	h.DeleteAlert(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAlertHandler_ResolveAlert_NotFound 测试解决不存在的告警
func TestAlertHandler_ResolveAlert_NotFound(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	reqBody := ResolveAlertRequest{ResolvedBy: "admin", Note: "Fixed"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/audit/alerts/nonexistent/resolve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ResolveAlert(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestAlertHandler_ResolveAlert_InvalidJSON 测试解决告警无效JSON
func TestAlertHandler_ResolveAlert_InvalidJSON(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	req := httptest.NewRequest("POST", "/api/v1/audit/alerts/test-alert-123/resolve", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ResolveAlert(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAlertHandler_ListAlerts_WithPagination 测试分页
func TestAlertHandler_ListAlerts_WithPagination(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	// 创建5个告警
	for i := 0; i < 5; i++ {
		alert := &model.Alert{
			AlertID:    "alert-" + string(rune('a'+i)),
			AlertType:  "security",
			AlertLevel: "warning",
			TenantID:   2001,
		}
		store.Create(context.Background(), alert)
	}

	req := httptest.NewRequest("GET", "/api/v1/audit/alerts?tenant_id=2001&offset=0&limit=2", nil)
	w := httptest.NewRecorder()

	h.ListAlerts(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result AlertListResponse
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, int64(5), result.Total)
	assert.Equal(t, 2, result.Limit)
}

// TestAlertHandler_ListAlerts_WithStatusFilter 测试状态过滤
func TestAlertHandler_ListAlerts_WithStatusFilter(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	// 创建不同状态的告警
	store.Create(context.Background(), &model.Alert{
		AlertID:   "alert-active",
		AlertType: "security",
		TenantID:  2001,
		Status:    model.AlertStatusActive,
	})
	store.Create(context.Background(), &model.Alert{
		AlertID:   "alert-resolved",
		AlertType: "security",
		TenantID:  2001,
		Status:    model.AlertStatusResolved,
	})

	req := httptest.NewRequest("GET", "/api/v1/audit/alerts?tenant_id=2001&status=active", nil)
	w := httptest.NewRecorder()

	h.ListAlerts(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAlertHandler_UpdateAlert_WithNotifyEnabled 测试更新通知设置
func TestAlertHandler_UpdateAlert_WithNotifyEnabled(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	notifyEnabled := false
	alert := &model.Alert{
		AlertID:        "test-alert-123",
		AlertType:     "security",
		AlertLevel:    "warning",
		TenantID:      2001,
		NotifyEnabled: true,
	}
	store.Create(context.Background(), alert)

	reqBody := UpdateAlertRequest{NotifyEnabled: &notifyEnabled}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/audit/alerts/test-alert-123", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateAlert(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAlertHandler_UpdateAlert_WithTags 测试更新标签
func TestAlertHandler_UpdateAlert_WithTags(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	alert := &model.Alert{
		AlertID:    "test-alert-123",
		AlertType: "security",
		TenantID:  2001,
	}
	store.Create(context.Background(), alert)

	reqBody := UpdateAlertRequest{Tags: []string{"tag1", "tag2"}}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/audit/alerts/test-alert-123", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateAlert(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAlertHandler_UpdateAlert_WithMetadata 测试更新元数据
func TestAlertHandler_UpdateAlert_WithMetadata(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	alert := &model.Alert{
		AlertID:    "test-alert-123",
		AlertType: "security",
		TenantID:  2001,
	}
	store.Create(context.Background(), alert)

	reqBody := UpdateAlertRequest{
		Metadata: map[string]any{"key": "value"},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/audit/alerts/test-alert-123", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateAlert(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAlertHandler_ResolveAlert_WithResolveSuffix 测试resolve路径后缀
func TestAlertHandler_ResolveAlert_WithResolveSuffix(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	// 创建告警
	alert := &model.Alert{
		AlertID:    "test-alert-resolve",
		AlertType:  "security",
		AlertLevel: "warning",
		TenantID:   2001,
		Status:     model.AlertStatusActive,
	}
	store.Create(context.Background(), alert)

	reqBody := ResolveAlertRequest{ResolvedBy: "admin", Note: "Done"}
	body, _ := json.Marshal(reqBody)
	// 使用带 /resolve 后缀的路径
	req := httptest.NewRequest("POST", "/api/v1/audit/alerts/test-alert-resolve/resolve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ResolveAlert(w, req)

	// 应该能正确提取 ID 并成功解决
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAlertHandler_GetAlert_WithQueryParam 测试使用查询参数获取告警
func TestAlertHandler_GetAlert_WithQueryParam(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	// 创建告警
	alert := &model.Alert{
		AlertID:    "test-alert-query",
		AlertType:  "security",
		AlertLevel: "warning",
		TenantID:   2001,
	}
	store.Create(context.Background(), alert)

	// 使用查询参数提供 alert_id
	req := httptest.NewRequest("GET", "/api/v1/audit/alerts?alert_id=test-alert-query", nil)
	w := httptest.NewRecorder()

	h.GetAlert(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAlertHandler_DeleteAlert_WithResolveSuffix 测试删除带resolve后缀的路径
func TestAlertHandler_DeleteAlert_WithResolveSuffix(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	// 创建告警
	alert := &model.Alert{
		AlertID:    "test-alert-delete",
		AlertType:  "security",
		AlertLevel: "warning",
		TenantID:   2001,
	}
	store.Create(context.Background(), alert)

	// 带 resolve 后缀的路径，alert ID 应该是 "test-alert-delete"
	req := httptest.NewRequest("DELETE", "/api/v1/audit/alerts/test-alert-delete/resolve", nil)
	w := httptest.NewRecorder()

	h.DeleteAlert(w, req)

	// extractAlertID 正确提取 parts[4]="test-alert-delete" 作为 ID
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestAlertHandler_UpdateAlert_WithAlertLevel 测试更新告警级别
func TestAlertHandler_UpdateAlert_WithAlertLevel(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	alert := &model.Alert{
		AlertID:    "test-alert-123",
		AlertType: "security",
		TenantID:  2001,
	}
	store.Create(context.Background(), alert)

	reqBody := UpdateAlertRequest{AlertLevel: "error"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/audit/alerts/test-alert-123", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateAlert(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAlertHandler_CreateAlert_WithAllFields 测试创建告警包含所有字段
func TestAlertHandler_CreateAlert_WithAllFields(t *testing.T) {
	store := newMockAlertStore()
	svc := service.NewAlertService(store)
	h := NewAlertHandler(svc)

	reqBody := CreateAlertRequest{
		AlertName:    "full-alert",
		AlertType:    "security",
		AlertLevel:   "critical",
		TenantID:     2001,
		SupplierID:   3001,
		Title:        "Full Test Alert",
		Message:      "Full message",
		Description:  "Description",
		EventID:      "evt-123",
		NotifyEnabled: true,
		Tags:         []string{"tag1", "tag2"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/audit/alerts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateAlert(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}
