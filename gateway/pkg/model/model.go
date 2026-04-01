package model

import "time"

// ChatCompletionRequest 聊天完成请求
type ChatCompletionRequest struct {
	Model       string                  `json:"model" binding:"required"`
	Messages    []ChatMessage           `json:"messages" binding:"required"`
	Temperature float64                 `json:"temperature,omitempty"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	TopP        float64                `json:"top_p,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	Stop        []string               `json:"stop,omitempty"`
	N           int                     `json:"n,omitempty"`
	PresencePenalty float64            `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64           `json:"frequency_penalty,omitempty"`
	User       string                  `json:"user,omitempty"`
}

// ChatMessage 聊天消息
type ChatMessage struct {
	Role    string `json:"role" binding:"required"`
	Content string `json:"content" binding:"required"`
	Name    string `json:"name,omitempty"`
}

// ChatCompletionResponse 聊天完成响应
type ChatCompletionResponse struct {
	ID      string          `json:"id"`
	Object  string          `json:"object"`
	Created int64          `json:"created"`
	Model   string          `json:"model"`
	Choices []Choice       `json:"choices"`
	Usage   Usage           `json:"usage"`
}

type Choice struct {
	Index        int        `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// CompletionRequest 完成请求
type CompletionRequest struct {
	Model       string  `json:"model" binding:"required"`
	Prompt      string  `json:"prompt" binding:"required"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	Stream      bool    `json:"stream,omitempty"`
	Stop        []string `json:"stop,omitempty"`
	N           int     `json:"n,omitempty"`
}

// CompletionResponse 完成响应
type CompletionResponse struct {
	ID      string     `json:"id"`
	Object  string     `json:"object"`
	Created int64     `json:"created"`
	Model   string     `json:"model"`
	Choices []Choice1 `json:"choices"`
	Usage   Usage     `json:"usage"`
}

type Choice1 struct {
	Text        string `json:"text"`
	Index       int    `json:"index"`
	FinishReason string `json:"finish_reason"`
}

// StreamResponse 流式响应
type StreamResponse struct {
	ID      string      `json:"id"`
	Object  string      `json:"object"`
	Created int64      `json:"created"`
	Model   string      `json:"model"`
	Choices []Delta    `json:"choices"`
}

type Delta struct {
	Delta struct {
		Content string `json:"content,omitempty"`
		Role    string `json:"role,omitempty"`
	} `json:"delta"`
	Index       int    `json:"index"`
	FinishReason string `json:"finish_reason,omitempty"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Message string                 `json:"message"`
	Type    string                 `json:"type"`
	Code    string                 `json:"code,omitempty"`
	Param   string                 `json:"param,omitempty"`
}

// HealthStatus 健康状态
type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]bool   `json:"services"`
}

// Tenant 租户
type Tenant struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Plan      string    `json:"plan"`
	CreatedAt time.Time `json:"created_at"`
}

// Budget 预算
type Budget struct {
	TenantID     int64   `json:"tenant_id"`
	MonthlyLimit float64 `json:"monthly_limit"`
	AlertThreshold float64 `json:"alert_threshold"`
	CurrentUsage float64 `json:"current_usage"`
}

// RouteRequest 路由请求
type RouteRequest struct {
	Model     string
	TenantID  int64
	RouteType string // "primary", "fallback"
}

// RouteResult 路由结果
type RouteResult struct {
	Provider  string
	Model     string
	LatencyMs int64
	Success   bool
	Error     error
}
