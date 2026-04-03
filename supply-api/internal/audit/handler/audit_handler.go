package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"lijiaoqiao/supply-api/internal/audit/model"
	"lijiaoqiao/supply-api/internal/audit/service"
)

// AuditHandler HTTP处理器
type AuditHandler struct {
	svc *service.AuditService
}

// NewAuditHandler 创建审计处理器
func NewAuditHandler(svc *service.AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// CreateEventRequest 创建事件请求
type CreateEventRequest struct {
	EventName        string `json:"event_name"`
	EventCategory    string `json:"event_category"`
	EventSubCategory string `json:"event_sub_category"`
	OperatorID       int64  `json:"operator_id"`
	TenantID         int64  `json:"tenant_id"`
	ObjectType       string `json:"object_type"`
	ObjectID         int64  `json:"object_id"`
	Action           string `json:"action"`
	IdempotencyKey  string `json:"idempotency_key,omitempty"`
	SourceIP         string `json:"source_ip,omitempty"`
	Success          bool   `json:"success"`
	ResultCode       string `json:"result_code,omitempty"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// ListEventsResponse 事件列表响应
type ListEventsResponse struct {
	Events []*model.AuditEvent `json:"events"`
	Total  int64              `json:"total"`
	Offset int                 `json:"offset"`
	Limit  int                 `json:"limit"`
}

// CreateEvent 处理POST /api/v1/audit/events
// @Summary 创建审计事件
// @Description 创建新的审计事件，支持幂等
// @Tags audit
// @Accept json
// @Produce json
// @Param event body CreateEventRequest true "事件信息"
// @Success 201 {object} service.CreateEventResult
// @Success 200 {object} service.CreateEventResult "幂等重复"
// @Success 409 {object} service.CreateEventResult "幂等冲突"
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/audit/events [post]
func (h *AuditHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var req CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body: "+err.Error())
		return
	}

	// 验证必填字段
	if req.EventName == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELD", "event_name is required")
		return
	}
	if req.EventCategory == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELD", "event_category is required")
		return
	}

	event := &model.AuditEvent{
		EventName:        req.EventName,
		EventCategory:    req.EventCategory,
		EventSubCategory: req.EventSubCategory,
		OperatorID:       req.OperatorID,
		TenantID:         req.TenantID,
		ObjectType:       req.ObjectType,
		ObjectID:         req.ObjectID,
		Action:           req.Action,
		IdempotencyKey:   req.IdempotencyKey,
		SourceIP:         req.SourceIP,
		Success:          req.Success,
		ResultCode:       req.ResultCode,
	}

	result, err := h.svc.CreateEvent(r.Context(), event)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(result.StatusCode)
	json.NewEncoder(w).Encode(result)
}

// ListEvents 处理GET /api/v1/audit/events
// @Summary 查询审计事件
// @Description 查询审计事件列表，支持分页和过滤
// @Tags audit
// @Produce json
// @Param tenant_id query int false "租户ID"
// @Param category query string false "事件类别"
// @Param event_name query string false "事件名称"
// @Param offset query int false "偏移量" default(0)
// @Param limit query int false "限制数量" default(100)
// @Success 200 {object} ListEventsResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/audit/events [get]
func (h *AuditHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	filter := &service.EventFilter{}

	// 解析查询参数
	if tenantIDStr := r.URL.Query().Get("tenant_id"); tenantIDStr != "" {
		tenantID, err := strconv.ParseInt(tenantIDStr, 10, 64)
		if err == nil {
			filter.TenantID = tenantID
		}
	}

	if category := r.URL.Query().Get("category"); category != "" {
		filter.Category = category
	}

	if eventName := r.URL.Query().Get("event_name"); eventName != "" {
		filter.EventName = eventName
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err == nil {
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

	events, total, err := h.svc.ListEventsWithFilter(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ListEventsResponse{
		Events: events,
		Total:  total,
		Offset: filter.Offset,
		Limit:  filter.Limit,
	})
}

// writeError 写入错误响应
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   message,
		Code:    code,
		Details: "",
	})
}
