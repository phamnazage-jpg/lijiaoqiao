package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"lijiaoqiao/gateway/internal/adapter"
	"lijiaoqiao/gateway/internal/router"
	"lijiaoqiao/gateway/pkg/error"
	"lijiaoqiao/gateway/pkg/model"
)

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

// ChatCompletionsHandle /v1/chat/completions端点
func (h *Handler) ChatCompletionsHandle(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = generateRequestID()
	}

	ctx := context.WithValue(r.Context(), "request_id", requestID)
	ctx = context.WithValue(ctx, "start_time", startTime)

	// 解析请求
	var req model.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, error.NewGatewayError(error.COMMON_INVALID_REQUEST, "invalid request body: "+err.Error()).WithRequestID(requestID))
		return
	}

	// 验证请求
	if len(req.Messages) == 0 {
		h.writeError(w, r, error.NewGatewayError(error.COMMON_INVALID_REQUEST, "messages is required").WithRequestID(requestID))
		return
	}

	// 选择Provider
	provider, err := h.router.SelectProvider(ctx, req.Model)
	if err != nil {
		h.writeError(w, r, err.(*error.GatewayError).WithRequestID(requestID))
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
		h.writeError(w, r, err.(*error.GatewayError).WithRequestID(requestID))
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
		h.writeError(w, r, err.(*error.GatewayError).WithRequestID(requestID))
		return
	}

	// 设置SSE头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Request-ID", requestID)

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, r, error.NewGatewayError(error.COMMON_INTERNAL_ERROR, "streaming not supported").WithRequestID(requestID))
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

// CompletionsHandle /v1/completions端点
func (h *Handler) CompletionsHandle(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = generateRequestID()
	}

	// 解析请求
	var req model.CompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, error.NewGatewayError(error.COMMON_INVALID_REQUEST, "invalid request body").WithRequestID(requestID))
		return
	}

	// 转换格式并调用ChatCompletions
	chatReq := model.ChatCompletionRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		TopP:        req.TopP,
		Stream:      req.Stream,
		Stop:        req.Stop,
		Messages: []model.ChatMessage{
			{Role: "user", Content: req.Prompt},
		},
	}

	// 复用ChatCompletions逻辑
	req.Method = "POST"
	req.URL.Path = "/v1/chat/completions"

	// 重新构造请求体并处理
	ctx := r.Context()
	messages := []adapter.Message{{Role: "user", Content: req.Prompt}}

	provider, err := h.router.SelectProvider(ctx, req.Model)
	if err != nil {
		h.writeError(w, r, err.(*error.GatewayError).WithRequestID(requestID))
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
		h.writeError(w, r, err.(*error.GatewayError).WithRequestID(requestID))
		return
	}

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

// ModelsHandle /v1/models端点
func (h *Handler) ModelsHandle(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = generateRequestID()
	}

	// 返回支持的模型列表
	models := []map[string]interface{}{
		{"id": "gpt-4", "object": "model", "created": 1687882411, "owned_by": "openai"},
		{"id": "gpt-3.5-turbo", "object": "model", "created": 1677610602, "owned_by": "openai"},
		{"id": "claude-3-opus", "object": "model", "created": 1709598254, "owned_by": "anthropic"},
		{"id": "claude-3-sonnet", "object": "model", "created": 1709598255, "owned_by": "anthropic"},
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"object": "list",
		"data":   models,
	}, requestID)
}

// HealthHandle /health端点
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

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err *error.GatewayError) {
	info := err.GetErrorInfo()
	w.Header().Set("Content-Type", "application/json")
	if err.RequestID != "" {
		w.Header().Set("X-Request-ID", err.RequestID)
	}
	w.WriteHeader(info.HTTPStatus)

	resp := model.ErrorResponse{
		Error: model.ErrorDetail{
			Message: err.Message,
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

// SSEReader 流式响应读取器
type SSEReader struct {
	reader *bufio.Reader
}

func NewSSEReader(r io.Reader) *SSEReader {
	return &SSEReader{reader: bufio.NewReader(r)}
}

func (s *SSEReader) ReadLine() (string, error) {
	line, err := s.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return line[:len(line)-1], nil
}

func parseSSEData(line string) string {
	if len(line) < 6 {
		return ""
	}
	if line[:5] != "data:" {
		return ""
	}
	return line[6:]
}

func getenv(key, defaultValue string) string {
	return defaultValue
}

func init() {
	getenv = func(key, defaultValue string) string {
		return defaultValue
	}
}
