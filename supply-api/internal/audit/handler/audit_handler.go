package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lijiaoqiao/supply-api/internal/audit/model"
	"lijiaoqiao/supply-api/internal/audit/service"
)

// AuditHandler HTTP处理器
type AuditHandler struct {
	svc        *service.AuditService
	metricsSvc *service.MetricsService
}

// NewAuditHandler 创建审计处理器
func NewAuditHandler(svc *service.AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// NewAuditHandlerWithMetrics 创建带指标服务的审计处理器
func NewAuditHandlerWithMetrics(svc *service.AuditService, metricsSvc *service.MetricsService) *AuditHandler {
	return &AuditHandler{
		svc:        svc,
		metricsSvc: metricsSvc,
	}
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

// GetEventResponse 单个事件响应
type GetEventResponse struct {
	Event *model.AuditEvent `json:"event"`
}

// GetEvent 处理 GET /api/v1/audit/events/{event_id}
// @Summary 获取单个审计事件
// @Description 根据事件ID获取审计事件详情
// @Tags audit
// @Produce json
// @Param event_id path string true "事件ID"
// @Success 200 {object} GetEventResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/audit/events/{event_id} [get]
func (h *AuditHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	// 从路径提取 event_id
	eventID := r.URL.Query().Get("event_id")
	if eventID == "" {
		// 尝试从路径参数获取
		pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/audit/events/"), "/")
		if len(pathParts) > 0 && pathParts[0] != "" {
			eventID = pathParts[0]
		}
	}

	if eventID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_PARAM", "event_id is required")
		return
	}

	event, err := h.svc.GetEventByID(r.Context(), eventID)
	if err != nil {
		if err == service.ErrEventNotFound {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "event not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetEventResponse{Event: event})
}

// GetMetrics 处理 GET /api/v1/audit/metrics/{metric_id}
// @Summary 获取审计指标
// @Description 获取M-013~M-016指标数据
// @Tags audit
// @Produce json
// @Param metric_id path string true "指标ID (m013/m014/m015/m016)"
// @Param start query string false "开始时间 ISO8601"
// @Param end query string false "结束时间 ISO8601"
// @Success 200 {object} service.Metric
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/audit/metrics/{metric_id} [get]
func (h *AuditHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	if h.metricsSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "METRICS_UNAVAILABLE", "metrics service not available")
		return
	}

	// 解析metric_id
	metricID := r.URL.Query().Get("metric_id")
	if metricID == "" {
		// 从路径中提取
		metricID = "m013" // 默认
	}

	// 解析时间范围
	now := time.Now()
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var start, end time.Time
	if startStr != "" {
		var err error
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			start = now.Add(-24 * time.Hour)
		}
	} else {
		start = now.Add(-24 * time.Hour)
	}

	if endStr != "" {
		var err error
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			end = now
		}
	} else {
		end = now
	}

	// 根据metric_id调用对应的计算方法
	var metric *service.Metric
	var err error

	switch metricID {
	case "m013", "M013", "m014", "M014", "m015", "M015", "m016", "M016":
		metric, err = h.calculateMetric(r.Context(), metricID, start, end)
	default:
		writeError(w, http.StatusBadRequest, "INVALID_METRIC", "invalid metric_id: "+metricID)
		return
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "METRICS_FAILED", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metric)
}

// CreateEventsBatchRequest 批量创建事件请求
type CreateEventsBatchRequest struct {
	Events []*CreateEventRequest `json:"events"`
}

// CreateEventsBatchResponse 批量创建事件响应
type CreateEventsBatchResponse struct {
	SuccessCount int      `json:"success_count"`
	FailCount    int      `json:"fail_count"`
	Errors       []string `json:"errors,omitempty"`
	EventIDs    []string `json:"event_ids,omitempty"`
}

// CreateEventsBatch 处理 POST /api/v1/audit/events/batch
// @Summary 批量创建审计事件
// @Description 批量创建审计事件，支持最多50条/批次
// @Tags audit
// @Accept json
// @Produce json
// @Param events body CreateEventsBatchRequest true "事件列表"
// @Success 200 {object} CreateEventsBatchResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/audit/events/batch [post]
func (h *AuditHandler) CreateEventsBatch(w http.ResponseWriter, r *http.Request) {
	var req CreateEventsBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body: "+err.Error())
		return
	}

	// 限制批次大小
	if len(req.Events) > 50 {
		writeError(w, http.StatusBadRequest, "BATCH_TOO_LARGE", "batch size cannot exceed 50")
		return
	}

	if len(req.Events) == 0 {
		writeError(w, http.StatusBadRequest, "EMPTY_BATCH", "batch cannot be empty")
		return
	}

	// 转换为 AuditEvent
	events := make([]*model.AuditEvent, 0, len(req.Events))
	for i, eventReq := range req.Events {
		// 验证必填字段
		if eventReq.EventName == "" {
			writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", "event["+strconv.Itoa(i)+"]: event_name is required")
			return
		}
		if eventReq.EventCategory == "" {
			writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", "event["+strconv.Itoa(i)+"]: event_category is required")
			return
		}

		event := &model.AuditEvent{
			EventName:        eventReq.EventName,
			EventCategory:    eventReq.EventCategory,
			EventSubCategory: eventReq.EventSubCategory,
			OperatorID:       eventReq.OperatorID,
			TenantID:         eventReq.TenantID,
			ObjectType:       eventReq.ObjectType,
			ObjectID:         eventReq.ObjectID,
			Action:           eventReq.Action,
			IdempotencyKey:   eventReq.IdempotencyKey,
			SourceIP:         eventReq.SourceIP,
			Success:          eventReq.Success,
			ResultCode:       eventReq.ResultCode,
		}
		events = append(events, event)
	}

	// 调用批量创建
	batchResult, err := h.svc.CreateEventsBatch(r.Context(), events)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "BATCH_CREATE_FAILED", err.Error())
		return
	}

	response := &CreateEventsBatchResponse{
		SuccessCount: batchResult.SuccessCount,
		FailCount:    batchResult.FailCount,
		Errors:       batchResult.Errors,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// calculateMetric 根据metric_id计算指标
func (h *AuditHandler) calculateMetric(ctx context.Context, metricID string, start, end time.Time) (*service.Metric, error) {
	switch metricID {
	case "m013", "M013":
		return h.metricsSvc.CalculateM013(ctx, start, end)
	case "m014", "M014":
		return h.metricsSvc.CalculateM014(ctx, start, end)
	case "m015", "M015":
		return h.metricsSvc.CalculateM015(ctx, start, end)
	case "m016", "M016":
		return h.metricsSvc.CalculateM016(ctx, start, end)
	default:
		return nil, nil
	}
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
