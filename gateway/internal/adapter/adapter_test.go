package adapter

import (
	"context"
	"testing"
)

func TestProviderError_Error(t *testing.T) {
	err := ProviderError{
		Code:       "TEST_ERROR",
		Message:    "test error message",
		HTTPStatus: 500,
		Retryable:  true,
	}

	if err.Error() != "TEST_ERROR: test error message" {
		t.Errorf("unexpected error string: %s", err.Error())
	}
}

func TestProviderError_IsRetryable(t *testing.T) {
	t.Run("retryable true", func(t *testing.T) {
		err := ProviderError{
			Code:       "TEST_ERROR",
			Message:    "test",
			HTTPStatus: 500,
			Retryable:  true,
		}
		if !err.IsRetryable() {
			t.Error("expected IsRetryable to be true")
		}
	})

	t.Run("retryable false", func(t *testing.T) {
		err := ProviderError{
			Code:       "TEST_ERROR",
			Message:    "test",
			HTTPStatus: 400,
			Retryable:  false,
		}
		if err.IsRetryable() {
			t.Error("expected IsRetryable to be false")
		}
	})
}

func TestReadCloser_Close(t *testing.T) {
	t.Run("close with callback", func(t *testing.T) {
		called := false
		rc := &ReadCloser{
			OnClose: func() error {
				called = true
				return nil
			},
		}

		err := rc.Close()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !called {
			t.Error("OnClose was not called")
		}
	})

	t.Run("close without callback", func(t *testing.T) {
		rc := &ReadCloser{}
		err := rc.Close()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestCompletionOptions(t *testing.T) {
	opts := CompletionOptions{
		Temperature: 0.7,
		MaxTokens:   100,
		TopP:        0.9,
		Stream:      true,
		Stop:        []string{"stop"},
	}

	if opts.Temperature != 0.7 {
		t.Errorf("expected 0.7, got %f", opts.Temperature)
	}
	if opts.MaxTokens != 100 {
		t.Errorf("expected 100, got %d", opts.MaxTokens)
	}
	if opts.TopP != 0.9 {
		t.Errorf("expected 0.9, got %f", opts.TopP)
	}
	if !opts.Stream {
		t.Error("expected Stream to be true")
	}
	if len(opts.Stop) != 1 || opts.Stop[0] != "stop" {
		t.Error("unexpected Stop value")
	}
}

func TestCompletionResponse(t *testing.T) {
	resp := CompletionResponse{
		ID:      "test-id",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "gpt-4",
		Choices: []Choice{
			{
				Index: 0,
				Message: &Message{
					Role:    "assistant",
					Content: "Hello",
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	if resp.ID != "test-id" {
		t.Errorf("unexpected ID: %s", resp.ID)
	}
	if resp.Object != "chat.completion" {
		t.Errorf("unexpected Object: %s", resp.Object)
	}
	if len(resp.Choices) != 1 {
		t.Errorf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Hello" {
		t.Errorf("unexpected content: %s", resp.Choices[0].Message.Content)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("unexpected TotalTokens: %d", resp.Usage.TotalTokens)
	}
}

func TestStreamChunk(t *testing.T) {
	chunk := StreamChunk{
		ID:      "chunk-id",
		Object:  "chat.completion.chunk",
		Created: 1234567890,
		Model:   "gpt-4",
		Choices: []StreamChoice{
			{
				Index: 0,
				Delta: &Delta{
					Role:    "assistant",
					Content: "Hi",
				},
			},
		},
	}

	if chunk.ID != "chunk-id" {
		t.Errorf("unexpected ID: %s", chunk.ID)
	}
	if len(chunk.Choices) != 1 {
		t.Errorf("expected 1 choice, got %d", len(chunk.Choices))
	}
	if chunk.Choices[0].Delta.Content != "Hi" {
		t.Errorf("unexpected content: %s", chunk.Choices[0].Delta.Content)
	}
}

func TestMessage(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "test message",
		Name:    "John",
	}

	if msg.Role != "user" {
		t.Errorf("unexpected Role: %s", msg.Role)
	}
	if msg.Content != "test message" {
		t.Errorf("unexpected Content: %s", msg.Content)
	}
	if msg.Name != "John" {
		t.Errorf("unexpected Name: %s", msg.Name)
	}
}

func TestUsage(t *testing.T) {
	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	if usage.PromptTokens != 100 {
		t.Errorf("unexpected PromptTokens: %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 50 {
		t.Errorf("unexpected CompletionTokens: %d", usage.CompletionTokens)
	}
	if usage.TotalTokens != 150 {
		t.Errorf("unexpected TotalTokens: %d", usage.TotalTokens)
	}
}

func TestDelta(t *testing.T) {
	delta := Delta{
		Role:    "assistant",
		Content: "response",
	}

	if delta.Role != "assistant" {
		t.Errorf("unexpected Role: %s", delta.Role)
	}
	if delta.Content != "response" {
		t.Errorf("unexpected Content: %s", delta.Content)
	}
}

// MockProviderForTesting 用于测试的Mock Provider
type MockProviderForTesting struct {
	NameFunc             func() string
	SupportedModelsFunc  func() []string
	ChatCompletionFunc   func(ctx context.Context, model string, messages []Message, options CompletionOptions) (*CompletionResponse, error)
	HealthCheckFunc      func(ctx context.Context) bool
}

func (m *MockProviderForTesting) ChatCompletion(ctx context.Context, model string, messages []Message, options CompletionOptions) (*CompletionResponse, error) {
	if m.ChatCompletionFunc != nil {
		return m.ChatCompletionFunc(ctx, model, messages, options)
	}
	return nil, nil
}

func (m *MockProviderForTesting) ChatCompletionStream(ctx context.Context, model string, messages []Message, options CompletionOptions) (<-chan *StreamChunk, error) {
	return nil, nil
}

func (m *MockProviderForTesting) GetUsage(response *CompletionResponse) Usage {
	return Usage{}
}

func (m *MockProviderForTesting) MapError(err error) ProviderError {
	return ProviderError{}
}

func (m *MockProviderForTesting) HealthCheck(ctx context.Context) bool {
	if m.HealthCheckFunc != nil {
		return m.HealthCheckFunc(ctx)
	}
	return true
}

func (m *MockProviderForTesting) ProviderName() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock"
}

func (m *MockProviderForTesting) SupportedModels() []string {
	if m.SupportedModelsFunc != nil {
		return m.SupportedModelsFunc()
	}
	return []string{}
}

func TestMockProviderForTesting(t *testing.T) {
	called := false
	provider := &MockProviderForTesting{
		NameFunc: func() string {
			return "test-provider"
		},
		SupportedModelsFunc: func() []string {
			return []string{"gpt-4", "gpt-3.5"}
		},
		ChatCompletionFunc: func(ctx context.Context, model string, messages []Message, options CompletionOptions) (*CompletionResponse, error) {
			called = true
			return &CompletionResponse{ID: "test"}, nil
		},
		HealthCheckFunc: func(ctx context.Context) bool {
			return true
		},
	}

	if provider.ProviderName() != "test-provider" {
		t.Errorf("unexpected name: %s", provider.ProviderName())
	}

	models := provider.SupportedModels()
	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}

	resp, err := provider.ChatCompletion(context.Background(), "gpt-4", nil, CompletionOptions{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp.ID != "test" {
		t.Errorf("unexpected response ID: %s", resp.ID)
	}
	if !called {
		t.Error("ChatCompletionFunc was not called")
	}

	if !provider.HealthCheck(context.Background()) {
		t.Error("expected healthy")
	}
}
