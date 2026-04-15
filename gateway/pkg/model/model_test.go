package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestChatCompletionRequestJSONOmitsZeroValueOptionalFields(t *testing.T) {
	req := ChatCompletionRequest{
		Model: "gpt-4o-mini",
		Messages: []ChatMessage{
			{Role: "user", Content: "ping"},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	got := string(data)
	for _, want := range []string{`"model":"gpt-4o-mini"`, `"messages":[{"role":"user","content":"ping"}]`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected JSON to contain %s, got %s", want, got)
		}
	}

	for _, unexpected := range []string{
		`"temperature"`,
		`"max_tokens"`,
		`"top_p"`,
		`"stream"`,
		`"stop"`,
		`"n"`,
		`"presence_penalty"`,
		`"frequency_penalty"`,
		`"user":`,
	} {
		if strings.Contains(got, unexpected) {
			t.Fatalf("expected JSON to omit %s, got %s", unexpected, got)
		}
	}
}

func TestStreamResponseJSONUsesDeltaEnvelope(t *testing.T) {
	resp := StreamResponse{
		ID:      "chatcmpl-1",
		Object:  "chat.completion.chunk",
		Created: 1710000000,
		Model:   "gpt-4o-mini",
		Choices: []Delta{
			{
				Index:        0,
				FinishReason: "stop",
			},
		},
	}
	resp.Choices[0].Delta.Role = "assistant"
	resp.Choices[0].Delta.Content = "pong"

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	choices, ok := decoded["choices"].([]any)
	if !ok || len(choices) != 1 {
		t.Fatalf("expected one choice, got %#v", decoded["choices"])
	}

	choice, ok := choices[0].(map[string]any)
	if !ok {
		t.Fatalf("expected choice object, got %#v", choices[0])
	}

	delta, ok := choice["delta"].(map[string]any)
	if !ok {
		t.Fatalf("expected delta object, got %#v", choice["delta"])
	}

	if delta["role"] != "assistant" || delta["content"] != "pong" {
		t.Fatalf("unexpected delta payload: %#v", delta)
	}
	if choice["finish_reason"] != "stop" {
		t.Fatalf("unexpected finish reason: %#v", choice["finish_reason"])
	}
}

func TestHealthStatusJSONRoundTripPreservesServices(t *testing.T) {
	want := HealthStatus{
		Status:    "ok",
		Timestamp: time.Unix(1710000000, 0).UTC(),
		Services: map[string]bool{
			"postgres": true,
			"redis":    false,
		},
	}

	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got HealthStatus
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.Status != want.Status {
		t.Fatalf("status = %q, want %q", got.Status, want.Status)
	}
	if !got.Timestamp.Equal(want.Timestamp) {
		t.Fatalf("timestamp = %s, want %s", got.Timestamp, want.Timestamp)
	}
	if len(got.Services) != len(want.Services) {
		t.Fatalf("services len = %d, want %d", len(got.Services), len(want.Services))
	}
	for name, expected := range want.Services {
		if got.Services[name] != expected {
			t.Fatalf("service %s = %v, want %v", name, got.Services[name], expected)
		}
	}
}
