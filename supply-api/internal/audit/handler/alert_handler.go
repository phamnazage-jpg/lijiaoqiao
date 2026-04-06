package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"lijiaoqiao/supply-api/internal/audit/model"
	"lijiaoqiao/supply-api/internal/audit/service"
)

// AlertHandler 告警HTTP处理器
type AlertHandler struct {
	svc *service.AlertService
}

// NewAlertHandler 创建告警处理器
func NewAlertHandler(svc *service.AlertService) *AlertHandler {
	return &AlertHandler{svc: svc}
}

// CreateAlertRequest 创建告警请求
type CreateAlertRequest struct {
	AlertName   string `json:"alert_name"`
	AlertType   string `json:"alert_type"`
	AlertLevel  string `json:"alert_level"`
	TenantID    int64  `json:"tenant_id"`
	SupplierID  int64  `json:"supplier_id,omitempty"`
	Title       string `json:"title"`
	Message     string `json:"message"`
	Description string `json:"description,omitempty"`
	EventID     string `json:"event_id,omitempty"`
	EventIDs    []string `json:"event_ids,omitempty"`
	NotifyEnabled bool   `json:"notify_enabled"`
	Tags        []string `json:"tags,omitempty"`
}

// UpdateAlertRequest 更新告警请求
type UpdateAlertRequest struct {
	Title       string   `json:"title,omitempty"`
	Message     string   `json:"message,omitempty"`
	Description string   `json:"description,omitempty"`
	AlertLevel  string   `json:"alert_level,omitempty"`
	Status      string   `json:"status,omitempty"`
	NotifyEnabled *bool  `json:"notify_enabled,omitempty"`
	NotifyChannels []string `json:"notify_channels,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ResolveAlertRequest 解决告警请求
type ResolveAlertRequest struct {
	ResolvedBy string `json:"resolved_by"`
	Note       string `json:"note"`
}

// AlertResponse 告警响应
type AlertResponse struct {
	Alert *model.Alert `json:"alert"`
}

// AlertListResponse 告警列表响应
type AlertListResponse struct {
	Alerts []*model.Alert `json:"alerts"`
	Total  int64         `json:"total"`
	Offset int            `json:"offset"`
	Limit  int            `json:"limit"`
}

// CreateAlert 处理 POST /api/v1/audit/alerts
func (h *AlertHandler) CreateAlert(w http.ResponseWriter, r *http.Request) {
	var req CreateAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAlertError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body: "+err.Error())
		return
	}

	// 验证必填字段
	if req.Title == "" {
		writeAlertError(w, http.StatusBadRequest, "MISSING_FIELD", "title is required")
		return
	}
	if req.AlertType == "" {
		writeAlertError(w, http.StatusBadRequest, "MISSING_FIELD", "alert_type is required")
		return
	}

	// 创建告警
	alert := &model.Alert{
		AlertName:   req.AlertName,
		AlertType:   req.AlertType,
		AlertLevel:  req.AlertLevel,
		TenantID:    req.TenantID,
		SupplierID:  req.SupplierID,
		Title:       req.Title,
		Message:     req.Message,
		Description: req.Description,
		EventID:     req.EventID,
		EventIDs:    req.EventIDs,
		NotifyEnabled: req.NotifyEnabled,
		Tags:        req.Tags,
	}

	result, err := h.svc.CreateAlert(r.Context(), alert)
	if err != nil {
		writeAlertError(w, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(AlertResponse{Alert: result})
}

// GetAlert 处理 GET /api/v1/audit/alerts/{alert_id}
func (h *AlertHandler) GetAlert(w http.ResponseWriter, r *http.Request) {
	alertID := extractAlertID(r)
	if alertID == "" {
		writeAlertError(w, http.StatusBadRequest, "MISSING_PARAM", "alert_id is required")
		return
	}

	alert, err := h.svc.GetAlert(r.Context(), alertID)
	if err != nil {
		if err == service.ErrAlertNotFound {
			writeAlertError(w, http.StatusNotFound, "NOT_FOUND", "alert not found")
			return
		}
		writeAlertError(w, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AlertResponse{Alert: alert})
}

// ListAlerts 处理 GET /api/v1/audit/alerts
func (h *AlertHandler) ListAlerts(w http.ResponseWriter, r *http.Request) {
	filter := &model.AlertFilter{}

	// 解析查询参数
	if tenantIDStr := r.URL.Query().Get("tenant_id"); tenantIDStr != "" {
		tenantID, err := strconv.ParseInt(tenantIDStr, 10, 64)
		if err == nil {
			filter.TenantID = tenantID
		}
	}

	if supplierIDStr := r.URL.Query().Get("supplier_id"); supplierIDStr != "" {
		supplierID, err := strconv.ParseInt(supplierIDStr, 10, 64)
		if err == nil {
			filter.SupplierID = supplierID
		}
	}

	if alertType := r.URL.Query().Get("alert_type"); alertType != "" {
		filter.AlertType = alertType
	}

	if alertLevel := r.URL.Query().Get("alert_level"); alertLevel != "" {
		filter.AlertLevel = alertLevel
	}

	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = status
	}

	if keywords := r.URL.Query().Get("keywords"); keywords != "" {
		filter.Keywords = keywords
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err == nil && limit > 0 && limit <= 1000 {
			filter.Limit = limit
		}
	}

	if filter.Limit == 0 {
		filter.Limit = 100
	}

	alerts, total, err := h.svc.ListAlerts(r.Context(), filter)
	if err != nil {
		writeAlertError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AlertListResponse{
		Alerts: alerts,
		Total:  total,
		Offset: filter.Offset,
		Limit:  filter.Limit,
	})
}

// UpdateAlert 处理 PUT /api/v1/audit/alerts/{alert_id}
func (h *AlertHandler) UpdateAlert(w http.ResponseWriter, r *http.Request) {
	alertID := extractAlertID(r)
	if alertID == "" {
		writeAlertError(w, http.StatusBadRequest, "MISSING_PARAM", "alert_id is required")
		return
	}

	// 获取现有告警
	alert, err := h.svc.GetAlert(r.Context(), alertID)
	if err != nil {
		if err == service.ErrAlertNotFound {
			writeAlertError(w, http.StatusNotFound, "NOT_FOUND", "alert not found")
			return
		}
		writeAlertError(w, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}

	var req UpdateAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAlertError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body: "+err.Error())
		return
	}

	// 更新字段
	if req.Title != "" {
		alert.Title = req.Title
	}
	if req.Message != "" {
		alert.Message = req.Message
	}
	if req.Description != "" {
		alert.Description = req.Description
	}
	if req.AlertLevel != "" {
		alert.AlertLevel = req.AlertLevel
	}
	if req.Status != "" {
		alert.Status = req.Status
	}
	if req.NotifyEnabled != nil {
		alert.NotifyEnabled = *req.NotifyEnabled
	}
	if len(req.NotifyChannels) > 0 {
		alert.NotifyChannels = req.NotifyChannels
	}
	if len(req.Tags) > 0 {
		alert.Tags = req.Tags
	}
	if req.Metadata != nil {
		alert.Metadata = req.Metadata
	}

	result, err := h.svc.UpdateAlert(r.Context(), alert)
	if err != nil {
		writeAlertError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AlertResponse{Alert: result})
}

// DeleteAlert 处理 DELETE /api/v1/audit/alerts/{alert_id}
func (h *AlertHandler) DeleteAlert(w http.ResponseWriter, r *http.Request) {
	alertID := extractAlertID(r)
	if alertID == "" {
		writeAlertError(w, http.StatusBadRequest, "MISSING_PARAM", "alert_id is required")
		return
	}

	err := h.svc.DeleteAlert(r.Context(), alertID)
	if err != nil {
		if err == service.ErrAlertNotFound {
			writeAlertError(w, http.StatusNotFound, "NOT_FOUND", "alert not found")
			return
		}
		writeAlertError(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ResolveAlert 处理 POST /api/v1/audit/alerts/{alert_id}/resolve
func (h *AlertHandler) ResolveAlert(w http.ResponseWriter, r *http.Request) {
	alertID := extractAlertID(r)
	if alertID == "" {
		writeAlertError(w, http.StatusBadRequest, "MISSING_PARAM", "alert_id is required")
		return
	}

	var req ResolveAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAlertError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body: "+err.Error())
		return
	}

	if req.ResolvedBy == "" {
		writeAlertError(w, http.StatusBadRequest, "MISSING_FIELD", "resolved_by is required")
		return
	}

	result, err := h.svc.ResolveAlert(r.Context(), alertID, req.ResolvedBy, req.Note)
	if err != nil {
		if err == service.ErrAlertNotFound {
			writeAlertError(w, http.StatusNotFound, "NOT_FOUND", "alert not found")
			return
		}
		writeAlertError(w, http.StatusInternalServerError, "RESOLVE_FAILED", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AlertResponse{Alert: result})
}

// extractAlertID 从请求中提取alert_id（优先从路径，其次从查询参数）
func extractAlertID(r *http.Request) string {
	// 先尝试从路径提取
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "audit" && parts[3] == "alerts" {
		if parts[4] != "" && parts[4] != "resolve" {
			return parts[4]
		}
	}
	// 再尝试从查询参数提取
	if alertID := r.URL.Query().Get("alert_id"); alertID != "" {
		return alertID
	}
	return ""
}

// writeAlertError 写入错误响应
func writeAlertError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   message,
		Code:    code,
		Details: "",
	})
}
