package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"lijiaoqiao/gateway/internal/adapter"
	"lijiaoqiao/gateway/internal/router"
	gwerror "lijiaoqiao/gateway/pkg/error"
	"lijiaoqiao/gateway/pkg/model"
)

// MaxRequestBytes 最大请求体大小 (1MB)
const MaxRequestBytes = 1 * 1024 * 1024

// maxBytesReader 限制读取字节数的reader
type maxBytesReader struct {
	reader    io.ReadCloser
	remaining int64
}

// Read 实现io.Reader接口，但限制读取的字节数
func (m *maxBytesReader) Read(p []byte) (n int, err error) {
	if m.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > m.remaining {
		p = p[:m.remaining]
	}
	n, err = m.reader.Read(p)
	m.remaining -= int64(n)
	return n, err
}

// Close 实现io.Closer接口
func (m *maxBytesReader) Close() error {
	return m.reader.Close()
}

// Handler API处理器
type Handler struct {
	router  *router.Router
	version string
}

// NewHandler 创建处理器
func NewHandler(r *router.Router) *Handler {
	return &Handler{
		router:  r,
		version: "v1",
	}
}

// ChatCompletionsHandle /v1/chat/completions endpoint
func (h *Handler) ChatCompletionsHandle(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	requestID := sanitizeRequestID(r.Header.Get("X-Request-ID"))
	if requestID == "" {
		requestID = generateRequestID()
	}

	ctx := context.WithValue(r.Context(), "request_id", requestID)
	ctx = context.WithValue(ctx, "start_time", startTime)

	// 解析请求 - 使用限制reader防止过大的请求体
	var req model.ChatCompletionRequest
	limitedBody := &maxBytesReader{reader: r.Body, remaining: MaxRequestBytes}
	if err := json.NewDecoder(limitedBody).Decode(&req); err != nil {
		// 检查是否是请求体过大的错误
		if err.Error() == "http: request body too large" || limitedBody.remaining <= 0 {
			h.writeError(w, r, gwerror.NewGatewayError(gwerror.COMMON_REQUEST_TOO_LARGE, "request body exceeds maximum size limit").WithRequestID(requestID))
			return
		}
		h.writeError(w, r, gwerror.NewGatewayError(gwerror.COMMON_INVALID_REQUEST, "invalid request body: "+err.Error()).WithRequestID(requestID))
		return
	}

	// 验证请求
	if len(req.Messages) == 0 {
		h.writeError(w, r, gwerror.NewGatewayError(gwerror.COMMON_INVALID_REQUEST, "messages is required").WithRequestID(requestID))
		return
	}

	// 选择Provider
	provider, err := h.router.SelectProvider(ctx, req.Model)
	if err != nil {
		h.writeError(w, r, err.(*gwerror.GatewayError).WithRequestID(requestID))
		return
	}

	// 转换消息格式
	messages := make([]adapter.Message, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = adapter.Message{
			Role:    m.Role,
			Content: m.Content,
			Name:    m.Name,
		}
	}

	// 构建选项
	options := adapter.CompletionOptions{
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		TopP:        req.TopP,
		Stream:      req.Stream,
		Stop:        req.Stop,
	}

	// 处理流式请求
	if req.Stream {
		h.handleStream(ctx, w, r, provider, req.Model, messages, options, requestID)
		return
	}

	// 处理非流式请求
	response, err := provider.ChatCompletion(ctx, req.Model, messages, options)
	if err != nil {
		// 记录失败
		h.router.RecordResult(ctx, provider.ProviderName(), false, time.Since(startTime).Milliseconds())
		h.writeError(w, r, err.(*gwerror.GatewayError).WithRequestID(requestID))
		return
	}

	// 记录成功
	h.router.RecordResult(ctx, provider.ProviderName(), true, time.Since(startTime).Milliseconds())

	// 转换响应
	chatResp := model.ChatCompletionResponse{
		ID:      response.ID,
		Object:  "chat.completion",
		Created: response.Created,
		Model:   response.Model,
		Choices: make([]model.Choice, len(response.Choices)),
	}

	for i, c := range response.Choices {
		chatResp.Choices[i] = model.Choice{
			Index: c.Index,
			Message: model.ChatMessage{
				Role:    c.Message.Role,
				Content: c.Message.Content,
			},
			FinishReason: c.FinishReason,
		}
	}

	chatResp.Usage = model.Usage{
		PromptTokens:     response.Usage.PromptTokens,
		CompletionTokens: response.Usage.CompletionTokens,
		TotalTokens:      response.Usage.TotalTokens,
	}

	h.writeJSON(w, http.StatusOK, chatResp, requestID)
}

// handleStream 处理流式请求
func (h *Handler) handleStream(ctx context.Context, w http.ResponseWriter, r *http.Request, provider adapter.ProviderAdapter, model string, messages []adapter.Message, options adapter.CompletionOptions, requestID string) {
	ch, err := provider.ChatCompletionStream(ctx, model, messages, options)
	if err != nil {
		h.writeError(w, r, err.(*gwerror.GatewayError).WithRequestID(requestID))
		return
	}

	// 设置SSE头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Request-ID", requestID)

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, r, gwerror.NewGatewayError(gwerror.COMMON_INTERNAL_ERROR, "streaming not supported").WithRequestID(requestID))
		return
	}

	// 流式发送响应
	for chunk := range ch {
		data := fmt.Sprintf("data: %s\n\n", marshalJSON(chunk))
		w.Write([]byte(data))
		flusher.Flush()
	}

	w.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}

// CompletionsHandle /v1/completions endpoint
func (h *Handler) CompletionsHandle(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	requestID := sanitizeRequestID(r.Header.Get("X-Request-ID"))
	if requestID == "" {
		requestID = generateRequestID()
	}

	ctx := context.WithValue(r.Context(), "request_id", requestID)
	ctx = context.WithValue(ctx, "start_time", startTime)

	// 解析请求 - 使用限制reader防止过大的请求体
	var req model.CompletionRequest
	limitedBody := &maxBytesReader{reader: r.Body, remaining: MaxRequestBytes}
	if err := json.NewDecoder(limitedBody).Decode(&req); err != nil {
		if err.Error() == "http: request body too large" || limitedBody.remaining <= 0 {
			h.writeError(w, r, gwerror.NewGatewayError(gwerror.COMMON_REQUEST_TOO_LARGE, "request body exceeds maximum size limit").WithRequestID(requestID))
			return
		}
		h.writeError(w, r, gwerror.NewGatewayError(gwerror.COMMON_INVALID_REQUEST, "invalid request body").WithRequestID(requestID))
		return
	}

	// 构造消息
	messages := []adapter.Message{{Role: "user", Content: req.Prompt}}

	provider, err := h.router.SelectProvider(ctx, req.Model)
	if err != nil {
		h.router.RecordResult(ctx, provider.ProviderName(), false, time.Since(startTime).Milliseconds())
		h.writeError(w, r, err.(*gwerror.GatewayError).WithRequestID(requestID))
		return
	}

	options := adapter.CompletionOptions{
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		TopP:        req.TopP,
		Stream:      req.Stream,
		Stop:        req.Stop,
	}

	if req.Stream {
		h.handleStream(ctx, w, r, provider, req.Model, messages, options, requestID)
		return
	}

	response, err := provider.ChatCompletion(ctx, req.Model, messages, options)
	if err != nil {
		h.router.RecordResult(ctx, provider.ProviderName(), false, time.Since(startTime).Milliseconds())
		h.writeError(w, r, err.(*gwerror.GatewayError).WithRequestID(requestID))
		return
	}

	h.router.RecordResult(ctx, provider.ProviderName(), true, time.Since(startTime).Milliseconds())

	// 转换响应为Completion格式
	compResp := model.CompletionResponse{
		ID:      response.ID,
		Object:  "text_completion",
		Created: response.Created,
		Model:   response.Model,
		Choices: make([]model.Choice1, len(response.Choices)),
	}

	for i, c := range response.Choices {
		compResp.Choices[i] = model.Choice1{
			Text:        c.Message.Content,
			Index:       i,
			FinishReason: c.FinishReason,
		}
	}

	compResp.Usage = model.Usage{
		PromptTokens:     response.Usage.PromptTokens,
		CompletionTokens: response.Usage.CompletionTokens,
		TotalTokens:      response.Usage.TotalTokens,
	}

	h.writeJSON(w, http.StatusOK, compResp, requestID)
}

// ModelsHandle /v1/models endpoint
func (h *Handler) ModelsHandle(w http.ResponseWriter, r *http.Request) {
	requestID := sanitizeRequestID(r.Header.Get("X-Request-ID"))
	if requestID == "" {
		requestID = generateRequestID()
	}

	registeredModels := h.router.RegisteredModels()
	models := make([]map[string]interface{}, 0, len(registeredModels))
	for _, registeredModel := range registeredModels {
		models = append(models, map[string]interface{}{
			"id":       registeredModel.ID,
			"object":   "model",
			"created":  0,
			"owned_by": registeredModel.OwnedBy,
		})
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"object": "list",
		"data":   models,
	}, requestID)
}

// HealthHandle /health endpoint
func (h *Handler) HealthHandle(w http.ResponseWriter, r *http.Request) {
	healthStatus := h.router.GetHealthStatus()

	allHealthy := true
	services := make(map[string]bool)
	for name, health := range healthStatus {
		services[name] = health.Available
		if !health.Available {
			allHealthy = false
		}
	}

	status := "healthy"
	statusCode := http.StatusOK
	if !allHealthy {
		status = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	h.writeJSON(w, statusCode, model.HealthStatus{
		Status:    status,
		Timestamp: time.Now(),
		Services:  services,
	}, "")
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	if requestID != "" {
		w.Header().Set("X-Request-ID", requestID)
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// P1-7: writeError strips internal error details before sending to client
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err *gwerror.GatewayError) {
	info := err.GetErrorInfo()
	w.Header().Set("Content-Type", "application/json")
	if err.RequestID != "" {
		w.Header().Set("X-Request-ID", err.RequestID)
	}
	w.WriteHeader(info.HTTPStatus)

	// Strip internal details — only expose safe generic messages to clients
	safeMessage := err.Message
	switch err.Code {
	case gwerror.COMMON_INTERNAL_ERROR:
		safeMessage = "internal server error"
	case gwerror.COMMON_INVALID_REQUEST:
		// For validation errors, show which field was invalid (not the underlying reason)
		if strings.Contains(err.Message, "messages is required") {
			safeMessage = "messages is required"
		} else {
			safeMessage = "invalid request"
		}
	case gwerror.COMMON_REQUEST_TOO_LARGE:
		safeMessage = "request body too large"
	case gwerror.PROVIDER_ERROR:
		safeMessage = "upstream provider error"
	}

	resp := model.ErrorResponse{
		Error: model.ErrorDetail{
			Message: safeMessage,
			Type:    "gateway_error",
			Code:    string(err.Code),
		},
	}
	json.NewEncoder(w).Encode(resp)
}

func generateRequestID() string {
	return fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
}

func marshalJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

// sanitizeRequestID removes dangerous characters from client-provided X-Request-ID
// to prevent log injection attacks. Only allows safe alphanumeric, hyphens, underscores.
func sanitizeRequestID(rid string) string {
	if rid == "" {
		return ""
	}
	var result []byte
	for i := 0; i < len(rid) && i < 128; i++ {
		c := rid[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		}
	}
	if len(result) == 0 {
		return ""
	}
	return string(result)
}
