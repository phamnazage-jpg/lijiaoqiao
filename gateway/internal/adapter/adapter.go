package adapter

import (
	"context"
	"io"
)

// CompletionOptions 完成选项
type CompletionOptions struct {
	Temperature float64
	MaxTokens   int
	TopP        float64
	Stream      bool
	Stop        []string
}

// CompletionResponse 完成响应
type CompletionResponse struct {
	ID      string
	Object  string
	Created int64
	Model   string
	Choices []Choice
	Usage   Usage
}

// Choice 选择
type Choice struct {
	Index        int
	Message      *Message
	FinishReason string
}

// Message 消息
type Message struct {
	Role    string
	Content string
	Name    string
}

// Usage 使用量
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// StreamChunk 流式响应块
type StreamChunk struct {
	ID      string
	Object  string
	Created int64
	Model   string
	Choices []StreamChoice
}

// StreamChoice 流式选择
type StreamChoice struct {
	Index        int
	Delta        *Delta
	FinishReason string
}

// Delta 增量
type Delta struct {
	Role    string
	Content string
}

// ProviderAdapter 供应商适配器抽象基类
type ProviderAdapter interface {
	// ChatCompletion 发送聊天完成请求
	ChatCompletion(ctx context.Context, model string, messages []Message, options CompletionOptions) (*CompletionResponse, error)

	// ChatCompletionStream 流式聊天完成请求
	ChatCompletionStream(ctx context.Context, model string, messages []Message, options CompletionOptions) (<-chan *StreamChunk, error)

	// GetUsage 获取使用量
	GetUsage(response *CompletionResponse) Usage

	// MapError 错误码映射
	MapError(err error) ProviderError

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) bool

	// ProviderName 供应商名称
	ProviderName() string

	// SupportedModels 支持的模型列表
	SupportedModels() []string
}

// ProviderError 供应商错误
type ProviderError struct {
	Code       string
	Message    string
	HTTPStatus int
	Retryable  bool
}

// Error 实现error接口
func (e ProviderError) Error() string {
	return e.Code + ": " + e.Message
}

// IsRetryable 是否可重试
func (e ProviderError) IsRetryable() bool {
	return e.Retryable
}

// Router 路由器接口
type Router interface {
	// SelectProvider 选择最佳Provider
	SelectProvider(ctx context.Context, model string) (ProviderAdapter, error)

	// GetFallbackProviders 获取Fallback Providers
	GetFallbackProviders(ctx context.Context, model string) ([]ProviderAdapter, error)

	// RecordResult 记录调用结果用于负载均衡
	RecordResult(ctx context.Context, provider string, success bool, latencyMs int64)
}

// HealthChecker 健康检查器
type HealthChecker interface {
	// Check 检查服务健康状态
	Check(ctx context.Context) error

	// IsHealthy 是否健康
	IsHealthy() bool
}

// ReadCloser 带错误回调的io.ReadCloser
type ReadCloser struct {
	io.Reader
	OnClose func() error
}

func (r *ReadCloser) Close() error {
	if r.OnClose != nil {
		return r.OnClose()
	}
	return nil
}
